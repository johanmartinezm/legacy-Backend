package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// El código de acceso y su correo: fase 3 del plan
// (reports/20260826_plan_carga_masiva.md §4.1 y §4.3).
//
// Son las dos mitades del mismo interruptor. Que la carga pueda no generar el
// código solo sirve si existe la vuelta —«Generar credenciales»—, y esa vuelta
// solo sirve si el correo llega a su dueño.

// correoDeCredencialEspia anota los correos de credencial que salen.
//
// Lleva mutex y canal por lo mismo que correoDePagoEspia: el envío ocurre en una
// goroutine con contexto propio, así que el test escribe desde un hilo y lee
// desde otro.
type correoDeCredencialEspia struct {
	mu           sync.Mutex
	credenciales []domain.CorreoCredencial
	inscripcion  []domain.CorreoInscripcion
	aviso        chan struct{}

	// dentro cuenta cuántos envíos están ocurriendo a la vez, y solapados
	// guarda el máximo que se llegó a ver. Con la cola tiene que ser 1.
	dentro    int
	solapados int
}

// entrando y saliendo envuelven un envío para medir el solapamiento. El envío
// tarda un poco a propósito: sin eso, dos goroutines simultáneas podrían no
// llegar a coincidir por casualidad y la prueba no diría nada.
func (c *correoDeCredencialEspia) entrando() {
	c.mu.Lock()
	c.dentro++
	if c.dentro > c.solapados {
		c.solapados = c.dentro
	}
	c.mu.Unlock()
	time.Sleep(2 * time.Millisecond)
}

func (c *correoDeCredencialEspia) saliendo() {
	c.mu.Lock()
	c.dentro--
	c.mu.Unlock()
}

func (c *correoDeCredencialEspia) maximoSolapados() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.solapados
}

func nuevoEspiaDeCredenciales() *correoDeCredencialEspia {
	return &correoDeCredencialEspia{aviso: make(chan struct{}, 128)}
}

func (c *correoDeCredencialEspia) SendEventCredentialEmail(d domain.CorreoCredencial) error {
	c.entrando()
	defer c.saliendo()

	c.mu.Lock()
	c.credenciales = append(c.credenciales, d)
	c.mu.Unlock()
	c.aviso <- struct{}{}
	return nil
}

func (c *correoDeCredencialEspia) SendEventRegistrationEmail(d domain.CorreoInscripcion) error {
	c.mu.Lock()
	c.inscripcion = append(c.inscripcion, d)
	c.mu.Unlock()
	c.aviso <- struct{}{}
	return nil
}

// esperarUnCorreo bloquea hasta que salga un correo, o se rinde. Sin esto la
// prueba leería la lista antes de que la goroutine del envío haya escrito.
func (c *correoDeCredencialEspia) esperarUnCorreo(t *testing.T) {
	t.Helper()
	select {
	case <-c.aviso:
	case <-time.After(2 * time.Second):
		t.Fatal("no salió ningún correo")
	}
}

// nadaEnUnRato comprueba que NO sale ningún correo. Es la mitad que importa de
// los interruptores apagados: importar trescientas personas no puede disparar
// trescientos correos.
func (c *correoDeCredencialEspia) nadaEnUnRato(t *testing.T) {
	t.Helper()
	select {
	case <-c.aviso:
		t.Fatal("salió un correo que nadie pidió")
	case <-time.After(150 * time.Millisecond):
	}
}

func (c *correoDeCredencialEspia) SendResetPasswordEmail(to, resetURL string) error { return nil }
func (c *correoDeCredencialEspia) SendBoardContactEmail(to, n, e, m string) error   { return nil }
func (c *correoDeCredencialEspia) SendAsesoriaEmail(to, n, e, cat, m string) error  { return nil }
func (c *correoDeCredencialEspia) SendContactoEmail(to, a, n, e, m string) error    { return nil }
func (c *correoDeCredencialEspia) SendWelcomeEmail(to, userName string) error       { return nil }
func (c *correoDeCredencialEspia) SendVerificationEmail(to, link string) error      { return nil }
func (c *correoDeCredencialEspia) SendEventPaymentEmail(d domain.CorreoPago) error  { return nil }

// eventoPresencialGratuito es el caso de toda inscripción importada mientras la
// pasarela siga bloqueada: gratuita y confirmada en el acto (§4.2).
func eventoPresencialGratuito() *domain.Event {
	lugar := "Bogotá, Ágora"
	return &domain.Event{
		ID:        "evento-summit",
		Title:     "Legacy Summit 2026",
		IsFree:    true,
		Location:  &lugar,
		StartDate: time.Now().Add(30 * 24 * time.Hour),
	}
}

// altaImportada monta el servicio y da de alta una inscripción con las opciones
// que se le pasen, devolviendo lo que se guardó y el espía del correo.
func altaImportada(t *testing.T, evento *domain.Event, alta domain.AltaImportada) (*domain.Registration, *correoDeCredencialEspia) {
	t.Helper()

	var guardada *domain.Registration
	repo := &MockEventRepository{
		GetEventByIDFunc: func(ctx context.Context, id string) (*domain.Event, error) {
			return evento, nil
		},
		CreateRegistrationFunc: func(ctx context.Context, r *domain.Registration) error {
			r.ID = "inscripcion-1"
			copia := *r
			guardada = &copia
			return nil
		},
	}

	espia := nuevoEspiaDeCredenciales()
	svc := NewEventService(repo, nil).ConCorreoDeInscripcion(&usuariosDePrueba{}, espia)

	reg := &domain.Registration{
		UserID:      "usuario-1",
		EventID:     evento.ID,
		Importacion: alta,
	}
	if err := svc.RegisterUser(context.Background(), reg); err != nil {
		t.Fatalf("RegisterUser: %v", err)
	}
	if guardada == nil {
		t.Fatal("no se guardó la inscripción")
	}
	return guardada, espia
}

func TestAlta_SinCredencialNoGeneraElCodigo(t *testing.T) {
	// «Un interruptor que dice "no crear" tiene que no crear.» El estado es la
	// propia fila con qr_data vacío, sin ninguna columna aparte que pueda decir
	// otra cosa.
	guardada, espia := altaImportada(t, eventoPresencialGratuito(),
		domain.AltaImportada{SinCredencial: true, Aviso: domain.AvisoNinguno})

	if guardada.QRData != "" {
		t.Errorf("se generó el código pese al interruptor: %q", guardada.QRData)
	}
	if guardada.RegistrationStatus != domain.RegistrationConfirmed {
		t.Errorf("la inscripción tenía que quedar confirmada: %q", guardada.RegistrationStatus)
	}
	espia.nadaEnUnRato(t)
}

func TestAlta_DesdeLaAppElCodigoSeSigueGenerando(t *testing.T) {
	// La app deja Importacion en su valor cero. Esto es lo que garantiza que la
	// fase 3 no cambia nada de lo que ya funcionaba.
	guardada, espia := altaImportada(t, eventoPresencialGratuito(), domain.AltaImportada{})

	if guardada.QRData == "" {
		t.Error("una inscripción normal se quedó sin código")
	}
	espia.esperarUnCorreo(t)
	if len(espia.inscripcion) != 1 {
		t.Errorf("se esperaba el correo de inscripción de siempre: %+v", espia)
	}
	if len(espia.credenciales) != 0 {
		t.Error("la app no puede disparar el correo con el QR")
	}
}

func TestAlta_ConCredencialYAvisoSaleElCorreoDelQRConLasClaves(t *testing.T) {
	guardada, espia := altaImportada(t, eventoPresencialGratuito(), domain.AltaImportada{
		Aviso:      domain.AvisoCredencial,
		Usuario:    "ana@empresa.com",
		Contrasena: "1020304050",
	})

	if guardada.QRData == "" {
		t.Fatal("se pidió generar la credencial y no hay código")
	}

	espia.esperarUnCorreo(t)
	if len(espia.credenciales) != 1 {
		t.Fatalf("se esperaba un correo de credencial: %+v", espia.credenciales)
	}
	if len(espia.inscripcion) != 0 {
		t.Error("salieron dos correos por persona")
	}

	c := espia.credenciales[0]
	if c.QRData != guardada.QRData {
		t.Errorf("el correo lleva otro código: %q frente a %q", c.QRData, guardada.QRData)
	}
	// Es el único sitio donde alguien recién importado se entera de cómo entrar.
	if c.Usuario != "ana@empresa.com" || c.Contrasena != "1020304050" {
		t.Errorf("el correo no dice cómo entrar: %+v", c)
	}
	if c.Evento != "Legacy Summit 2026" {
		t.Errorf("evento inesperado: %q", c.Evento)
	}
}

func TestAlta_SinCredencialPeroAvisandoSaleElCorreoDeSiempre(t *testing.T) {
	// Tercera fila de la tabla de §4.1: sin QR que mandar, se manda la
	// confirmación de siempre.
	guardada, espia := altaImportada(t, eventoPresencialGratuito(),
		domain.AltaImportada{SinCredencial: true, Aviso: domain.AvisoPorDefecto})

	if guardada.QRData != "" {
		t.Errorf("no debía generar código: %q", guardada.QRData)
	}
	espia.esperarUnCorreo(t)
	if len(espia.inscripcion) != 1 || len(espia.credenciales) != 0 {
		t.Errorf("correo equivocado: inscripción=%d credencial=%d",
			len(espia.inscripcion), len(espia.credenciales))
	}
}

// repoDeCredenciales sirve las inscripciones sin código y anota los UPDATE.
type repoDeCredenciales struct {
	MockEventRepository
	mu         sync.Mutex
	evento     *domain.Event
	pendientes []domain.Registration
	// pedidos guarda los ids que se le pasaron, para comprobar que la acción por
	// persona no barre el evento entero.
	pedidos  []string
	escritos map[string]string
	fallar   error
}

func (r *repoDeCredenciales) GetEventByID(ctx context.Context, id string) (*domain.Event, error) {
	return r.evento, nil
}

func (r *repoDeCredenciales) GetRegistrationsSinCredencial(ctx context.Context, eventID string, ids []string) ([]domain.Registration, error) {
	r.pedidos = append([]string{}, ids...)
	if len(ids) == 0 {
		return r.pendientes, nil
	}
	var fuera []domain.Registration
	for _, reg := range r.pendientes {
		for _, id := range ids {
			if reg.ID == id {
				fuera = append(fuera, reg)
			}
		}
	}
	return fuera, nil
}

func (r *repoDeCredenciales) SetRegistrationQR(ctx context.Context, regID, qrData string) error {
	if r.fallar != nil {
		return r.fallar
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.escritos == nil {
		r.escritos = map[string]string{}
	}
	r.escritos[regID] = qrData
	return nil
}

func repoConPendientes(evento *domain.Event, regs ...domain.Registration) *repoDeCredenciales {
	return &repoDeCredenciales{evento: evento, pendientes: regs}
}

func inscripcionSinCodigo(id, estado string) domain.Registration {
	return domain.Registration{
		ID:                 id,
		UserID:             "usuario-" + id,
		EventID:            "evento-summit",
		RegistrationStatus: estado,
		PaymentStatus:      "free",
	}
}

func TestGenerarCredenciales_EnBloqueRellenaLasQueFaltan(t *testing.T) {
	repo := repoConPendientes(eventoPresencialGratuito(),
		inscripcionSinCodigo("r1", domain.RegistrationConfirmed),
		inscripcionSinCodigo("r2", domain.RegistrationConfirmed),
	)
	espia := nuevoEspiaDeCredenciales()
	svc := NewEventService(repo, nil).ConCorreoDeInscripcion(&usuariosDePrueba{}, espia)

	generadas, err := svc.GenerarCredenciales(context.Background(), "evento-summit", nil, false)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if generadas != 2 {
		t.Errorf("generadas=%d, se esperaban 2", generadas)
	}
	if len(repo.escritos) != 2 {
		t.Errorf("escritos=%+v", repo.escritos)
	}
	// Aleatorio y distinto para cada persona: tener un código no puede permitir
	// fabricar el de otro.
	if repo.escritos["r1"] == repo.escritos["r2"] {
		t.Error("dos inscripciones recibieron el mismo código")
	}
	// Sin avisar, ni un correo.
	espia.nadaEnUnRato(t)
}

func TestGenerarCredenciales_PorPersonaSoloAlcanzaAEsa(t *testing.T) {
	repo := repoConPendientes(eventoPresencialGratuito(),
		inscripcionSinCodigo("r1", domain.RegistrationConfirmed),
		inscripcionSinCodigo("r2", domain.RegistrationConfirmed),
	)
	svc := NewEventService(repo, nil)

	generadas, err := svc.GenerarCredenciales(context.Background(), "evento-summit", []string{"r2"}, false)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if generadas != 1 {
		t.Errorf("generadas=%d, se esperaba 1", generadas)
	}
	if _, tocada := repo.escritos["r1"]; tocada {
		t.Error("se generó el código de quien no se pidió")
	}
}

func TestGenerarCredenciales_ConAvisoSaleElCorreoSinLasClaves(t *testing.T) {
	// Esta persona ya tiene su contraseña: decirle otra cosa sería mentirle. El
	// bloque de credenciales solo lo rellena la carga que acaba de crear la
	// cuenta.
	repo := repoConPendientes(eventoPresencialGratuito(),
		inscripcionSinCodigo("r1", domain.RegistrationConfirmed))
	espia := nuevoEspiaDeCredenciales()
	svc := NewEventService(repo, nil).ConCorreoDeInscripcion(&usuariosDePrueba{}, espia)

	if _, err := svc.GenerarCredenciales(context.Background(), "evento-summit", nil, true); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	espia.esperarUnCorreo(t)
	if len(espia.credenciales) != 1 {
		t.Fatalf("credenciales=%+v", espia.credenciales)
	}
	c := espia.credenciales[0]
	if c.QRData != repo.escritos["r1"] {
		t.Errorf("el correo lleva un código distinto al guardado: %q vs %q",
			c.QRData, repo.escritos["r1"])
	}
	if c.Usuario != "" || c.Contrasena != "" {
		t.Errorf("se le mandaron credenciales que no son suyas: %+v", c)
	}
}

func TestGenerarCredenciales_NoLeDaAccesoAQuienNoHaPagado(t *testing.T) {
	// CheckIn ya lo rechazaría en la puerta, pero el correo habría salido igual
	// y esa persona creería tener su entrada.
	repo := repoConPendientes(eventoPresencialGratuito(),
		inscripcionSinCodigo("r1", domain.RegistrationPendingPayment))
	espia := nuevoEspiaDeCredenciales()
	svc := NewEventService(repo, nil).ConCorreoDeInscripcion(&usuariosDePrueba{}, espia)

	generadas, err := svc.GenerarCredenciales(context.Background(), "evento-summit", nil, true)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if generadas != 0 {
		t.Errorf("se generó la credencial de una reserva sin pagar: %d", generadas)
	}
	if len(repo.escritos) != 0 {
		t.Errorf("se escribió un código: %+v", repo.escritos)
	}
	espia.nadaEnUnRato(t)
}

func TestGenerarCredenciales_EnUnEventoVirtualSeNiegaYLoExplica(t *testing.T) {
	// Allí el acceso es el enlace de la sesión y el QR no se muestra nunca, así
	// que generarlo sería escribir un valor que nadie va a mirar. El panel ya
	// deshabilita el interruptor; esto es la comprobación en el único punto por
	// el que pasan todos los caminos.
	enlace := "https://meet.example.com/summit"
	virtual := &domain.Event{
		ID:        "evento-summit",
		Title:     "Masterclass virtual",
		IsVirtual: true,
		IsFree:    true,
		AccessURL: &enlace,
		StartDate: time.Now().Add(48 * time.Hour),
	}
	repo := repoConPendientes(virtual, inscripcionSinCodigo("r1", domain.RegistrationConfirmed))
	svc := NewEventService(repo, nil)

	_, err := svc.GenerarCredenciales(context.Background(), "evento-summit", nil, true)
	if !errors.Is(err, ErrCredencialEnEventoVirtual) {
		t.Fatalf("se esperaba ErrCredencialEnEventoVirtual, llegó %v", err)
	}
	if len(repo.escritos) != 0 {
		t.Errorf("escribió códigos en un evento virtual: %+v", repo.escritos)
	}
}

func TestGenerarCredenciales_UnaCarreraNoLeCambiaElCodigoANadie(t *testing.T) {
	// Si dos personas pulsan el botón a la vez, la segunda encuentra la fila ya
	// rellena: el repositorio devuelve ErrNotFound y aquí eso no es un fallo,
	// es «esa ya tiene su código». Regenerarlo invalidaría el QR que la persona
	// lleva encima.
	repo := repoConPendientes(eventoPresencialGratuito(),
		inscripcionSinCodigo("r1", domain.RegistrationConfirmed))
	repo.fallar = domain.ErrNotFound
	espia := nuevoEspiaDeCredenciales()
	svc := NewEventService(repo, nil).ConCorreoDeInscripcion(&usuariosDePrueba{}, espia)

	generadas, err := svc.GenerarCredenciales(context.Background(), "evento-summit", nil, true)
	if err != nil {
		t.Fatalf("una carrera no es un error: %v", err)
	}
	if generadas != 0 {
		t.Errorf("generadas=%d, se esperaba 0", generadas)
	}
	// Y sobre todo: no se manda un correo con un código que no se escribió.
	espia.nadaEnUnRato(t)
}

func TestAlta_UnaCargaAUnEventoVirtualNuncaGeneraElCodigo(t *testing.T) {
	// Aunque lo pidan. Allí el acceso es el enlace de la sesión y la credencial
	// no se muestra jamás, así que el código sería un valor que nadie mira.
	//
	// Es la misma respuesta que da GenerarCredenciales, y tiene que serlo desde
	// los dos caminos: el panel deshabilita el interruptor, pero esto no puede
	// depender de que ningún cliente se acuerde.
	enlace := "https://meet.example.com/summit"
	virtual := &domain.Event{
		ID:        "evento-summit",
		Title:     "Masterclass virtual",
		IsVirtual: true,
		IsFree:    true,
		AccessURL: &enlace,
		StartDate: time.Now().Add(48 * time.Hour),
	}

	guardada, _ := altaImportada(t, virtual, domain.AltaImportada{
		EsCarga:       true,
		SinCredencial: false, // se pide expresamente, y aun así no se genera
		Aviso:         domain.AvisoNinguno,
	})

	if guardada.QRData != "" {
		t.Errorf("se generó un código en un evento virtual: %q", guardada.QRData)
	}
}

func TestAlta_DesdeLaAppUnEventoVirtualSigueComoEstaba(t *testing.T) {
	// La regla de arriba es solo para las cargas. Cambiar lo que hace la app
	// sería otro trabajo, y este no lo toca.
	enlace := "https://meet.example.com/summit"
	virtual := &domain.Event{
		ID:        "evento-summit",
		Title:     "Masterclass virtual",
		IsVirtual: true,
		IsFree:    true,
		AccessURL: &enlace,
		StartDate: time.Now().Add(48 * time.Hour),
	}

	guardada, _ := altaImportada(t, virtual, domain.AltaImportada{})

	if guardada.QRData == "" {
		t.Error("se cambió el comportamiento de la app sin pedirlo")
	}
}

func TestGenerarCredenciales_LosCorreosSalenDeUnoEnUno(t *testing.T) {
	// Hasta el 2026-09-03 cada correo salía en su propia goroutine, así que
	// generar credenciales para trescientas personas abría trescientas
	// conexiones contra Gmail a la vez. Gmail limita por tasa: lo previsible no
	// es que tarde, es que rechace una parte — y esos correos llevan la única
	// copia de la contraseña de alguien.
	//
	// Lo que se comprueba aquí no es la velocidad sino que **no hay dos envíos
	// solapados**: el espía cuenta cuántos hay dentro a la vez.
	const cuantas = 40

	var regs []domain.Registration
	for i := 0; i < cuantas; i++ {
		regs = append(regs, inscripcionSinCodigo(fmt.Sprintf("r%d", i), domain.RegistrationConfirmed))
	}

	repo := repoConPendientes(eventoPresencialGratuito(), regs...)
	espia := nuevoEspiaDeCredenciales()
	svc := NewEventService(repo, nil).ConCorreoDeInscripcion(&usuariosDePrueba{}, espia)

	if _, err := svc.GenerarCredenciales(context.Background(), "evento-summit", nil, true); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	for i := 0; i < cuantas; i++ {
		espia.esperarUnCorreo(t)
	}

	if n := espia.maximoSolapados(); n != 1 {
		t.Errorf("hubo %d envíos a la vez; la cola tiene que dejarlos de uno en uno", n)
	}
	if n := len(espia.credenciales); n != cuantas {
		t.Errorf("salieron %d correos de %d", n, cuantas)
	}
}
