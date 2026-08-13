package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/security"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// claveDePrueba tiene los 32 bytes exactos que exige NewCryptoService.
const claveDePrueba = "01234567890123456789012345678901"

// correoDePrueba registra lo ultimo que se le pidio enviar, para poder
// comprobar que recibe el buzon sin mandar nada de verdad.
type correoDePrueba struct {
	llamado      bool
	destinatario string
	asunto       string
	nombre       string
	email        string
	mensaje      string
	fallar       error
}

func (c *correoDePrueba) SendResetPasswordEmail(to, resetURL string) error { return nil }
func (c *correoDePrueba) SendBoardContactEmail(to, senderName, senderEmail, messageText string) error {
	return nil
}
func (c *correoDePrueba) SendAsesoriaEmail(to, senderName, senderEmail, category, messageText string) error {
	return nil
}
func (c *correoDePrueba) SendWelcomeEmail(to, userName string) error  { return nil }
func (c *correoDePrueba) SendVerificationEmail(to, link string) error { return nil }

func (c *correoDePrueba) SendContactoEmail(to, asunto, senderName, senderEmail, messageText string) error {
	c.llamado = true
	c.destinatario, c.asunto, c.nombre, c.email, c.mensaje = to, asunto, senderName, senderEmail, messageText
	return c.fallar
}

// repoDePrueba guarda en memoria lo que le llega, tal cual: si el servicio no
// cifro antes de llamarlo, aqui se ve en claro.
type repoDePrueba struct {
	guardados []*domain.MensajeDeContacto
	marcados  []string
	recientes int
	fallar    error
	estados   map[string]string
}

func nuevoRepo() *repoDePrueba {
	return &repoDePrueba{estados: map[string]string{}}
}

func (r *repoDePrueba) Guardar(ctx context.Context, m *domain.MensajeDeContacto) (string, error) {
	if r.fallar != nil {
		return "", r.fallar
	}
	r.guardados = append(r.guardados, m)
	return "id-1", nil
}

func (r *repoDePrueba) MarcarEnviado(ctx context.Context, id string) error {
	r.marcados = append(r.marcados, id)
	return nil
}

func (r *repoDePrueba) Listar(ctx context.Context, estado string) ([]*domain.MensajeDeContacto, error) {
	return r.guardados, nil
}

func (r *repoDePrueba) CambiarEstado(ctx context.Context, id, estado string) error {
	r.estados[id] = estado
	return nil
}

func (r *repoDePrueba) ContarDesde(ctx context.Context, userID string, desde time.Time) (int, error) {
	return r.recientes, nil
}

func servicioDePrueba(repo *repoDePrueba, correo *correoDePrueba, destinatario string) (*contactoService, *security.CryptoService) {
	crypto, _ := security.NewCryptoService(claveDePrueba)
	s := NewContactoService(repo, correo, crypto, destinatario)
	return s.(*contactoService), crypto
}

func TestContactoGuardaYEnvia(t *testing.T) {
	repo, correo := nuevoRepo(), &correoDePrueba{}
	s, crypto := servicioDePrueba(repo, correo, "soporte@ejemplo.com")

	err := s.EnviarMensaje(context.Background(), "user-1", "Duda con un evento", "Ana Ruiz", "ana@ejemplo.com", "No puedo inscribirme")
	if err != nil {
		t.Fatalf("no debia fallar: %v", err)
	}

	if len(repo.guardados) != 1 {
		t.Fatalf("se guardaron %d mensajes, se esperaba 1", len(repo.guardados))
	}
	if repo.guardados[0].UserID != "user-1" {
		t.Errorf("user_id = %q", repo.guardados[0].UserID)
	}
	if len(repo.marcados) != 1 || repo.marcados[0] != "id-1" {
		t.Error("el mensaje enviado no se marco como enviado")
	}

	// Al buzon llega en claro; a la base, cifrado.
	if correo.mensaje != "No puedo inscribirme" || correo.destinatario != "soporte@ejemplo.com" {
		t.Errorf("correo = %q a %q", correo.mensaje, correo.destinatario)
	}
	if repo.guardados[0].Mensaje == "No puedo inscribirme" {
		t.Error("el mensaje se guardo SIN cifrar")
	}
	descifrado, err := crypto.Decrypt(repo.guardados[0].Mensaje)
	if err != nil || descifrado != "No puedo inscribirme" {
		t.Errorf("lo guardado no descifra al original: %q (%v)", descifrado, err)
	}
}

func TestContactoNoPierdeElMensajeSiFallaElCorreo(t *testing.T) {
	repo, correo := nuevoRepo(), &correoDePrueba{fallar: errors.New("smtp caido")}
	s, _ := servicioDePrueba(repo, correo, "soporte@ejemplo.com")

	// Antes de que existiera la tabla, esto devolvia error y el mensaje se
	// perdia entero. Ahora esta guardado, asi que para quien escribe SI llego:
	// el equipo lo ve en la bandeja del panel.
	err := s.EnviarMensaje(context.Background(), "user-1", "Asunto", "Ana", "ana@ejemplo.com", "Hola")
	if err != nil {
		t.Fatalf("con el mensaje guardado no debia fallar: %v", err)
	}
	if len(repo.guardados) != 1 {
		t.Fatal("el mensaje no se guardo")
	}
	if len(repo.marcados) != 0 {
		t.Error("se marco como enviado un correo que fallo")
	}
}

func TestContactoFallaSiNoSePuedeGuardar(t *testing.T) {
	repo, correo := nuevoRepo(), &correoDePrueba{}
	repo.fallar = errors.New("base caida")
	s, _ := servicioDePrueba(repo, correo, "soporte@ejemplo.com")

	// Aqui si hay que avisar: sin guardar ni enviar, el mensaje se pierde.
	if err := s.EnviarMensaje(context.Background(), "user-1", "Asunto", "Ana", "ana@ejemplo.com", "Hola"); err == nil {
		t.Fatal("debia fallar si no se puede guardar")
	}
	if correo.llamado {
		t.Error("no debia intentar el correo si no se guardo")
	}
}

func TestContactoLimitaLosEnviosSeguidos(t *testing.T) {
	repo, correo := nuevoRepo(), &correoDePrueba{}
	repo.recientes = maximoPorVentana
	s, _ := servicioDePrueba(repo, correo, "soporte@ejemplo.com")

	err := s.EnviarMensaje(context.Background(), "user-1", "Asunto", "Ana", "ana@ejemplo.com", "Hola")
	if !errors.Is(err, ErrDemasiadosMensajes) {
		t.Fatalf("se esperaba ErrDemasiadosMensajes, llego: %v", err)
	}
	if len(repo.guardados) != 0 || correo.llamado {
		t.Error("no debia guardar ni enviar por encima del limite")
	}
}

func TestContactoRellenaElAsuntoVacio(t *testing.T) {
	repo, correo := nuevoRepo(), &correoDePrueba{}
	s, _ := servicioDePrueba(repo, correo, "soporte@ejemplo.com")

	if err := s.EnviarMensaje(context.Background(), "user-1", "   ", "Ana", "ana@ejemplo.com", "Hola"); err != nil {
		t.Fatalf("no debia fallar: %v", err)
	}
	if correo.asunto != asuntoPorDefecto {
		t.Errorf("asunto = %q, se esperaba %q", correo.asunto, asuntoPorDefecto)
	}
}

func TestContactoRechazaMensajeVacio(t *testing.T) {
	repo, correo := nuevoRepo(), &correoDePrueba{}
	s, _ := servicioDePrueba(repo, correo, "soporte@ejemplo.com")

	if err := s.EnviarMensaje(context.Background(), "user-1", "Asunto", "Ana", "ana@ejemplo.com", "   \n  "); err == nil {
		t.Fatal("un mensaje en blanco debia rechazarse")
	}
	if len(repo.guardados) != 0 || correo.llamado {
		t.Error("no debia guardar ni enviar nada")
	}
}

func TestContactoRechazaMensajeDemasiadoLargo(t *testing.T) {
	repo, correo := nuevoRepo(), &correoDePrueba{}
	s, _ := servicioDePrueba(repo, correo, "soporte@ejemplo.com")

	err := s.EnviarMensaje(context.Background(), "user-1", "Asunto", "Ana", "ana@ejemplo.com", strings.Repeat("a", maximoMensaje+1))
	if err == nil {
		t.Fatal("un mensaje pasado de largo debia rechazarse")
	}
	if len(repo.guardados) != 0 {
		t.Error("no debia guardarse")
	}
}

func TestContactoGuardaAunqueNoHayaBuzon(t *testing.T) {
	repo, correo := nuevoRepo(), &correoDePrueba{}
	s, _ := servicioDePrueba(repo, correo, "")

	// Sin buzon configurado no hay a quien avisar, pero el mensaje no se tira:
	// queda en la bandeja.
	if err := s.EnviarMensaje(context.Background(), "user-1", "Asunto", "Ana", "ana@ejemplo.com", "Hola"); err != nil {
		t.Fatalf("no debia fallar: %v", err)
	}
	if len(repo.guardados) != 1 {
		t.Error("el mensaje debia guardarse igualmente")
	}
	if correo.llamado {
		t.Error("no habia buzon: no debia intentar enviar")
	}
}

func TestContactoListarDescifra(t *testing.T) {
	repo, correo := nuevoRepo(), &correoDePrueba{}
	s, crypto := servicioDePrueba(repo, correo, "soporte@ejemplo.com")

	asunto, _ := crypto.Encrypt("Duda")
	cuerpo, _ := crypto.Encrypt("Mi mensaje")
	nombre, _ := crypto.Encrypt("Ana")
	apellido, _ := crypto.Encrypt("Ruiz")
	email, _ := crypto.Encrypt("ana@ejemplo.com")
	repo.guardados = []*domain.MensajeDeContacto{{
		ID: "id-1", UserID: "user-1", Asunto: asunto, Mensaje: cuerpo,
		RemitenteNombre: nombre, RemitenteApellido: apellido, RemitenteEmail: email,
	}}

	mensajes, err := s.Listar(context.Background(), "")
	if err != nil {
		t.Fatalf("no debia fallar: %v", err)
	}
	m := mensajes[0]
	if m.Asunto != "Duda" || m.Mensaje != "Mi mensaje" {
		t.Errorf("no se descifro el contenido: %q / %q", m.Asunto, m.Mensaje)
	}
	// El nombre y el apellido se guardan cifrados por separado y se unen ya
	// descifrados: unirlos antes daria una cadena imposible de leer.
	if m.RemitenteNombre != "Ana Ruiz" || m.RemitenteEmail != "ana@ejemplo.com" {
		t.Errorf("remitente = %q <%s>", m.RemitenteNombre, m.RemitenteEmail)
	}
}

func TestContactoRechazaEstadosInventados(t *testing.T) {
	repo, correo := nuevoRepo(), &correoDePrueba{}
	s, _ := servicioDePrueba(repo, correo, "soporte@ejemplo.com")

	// Un estado fuera de la lista dejaria el mensaje fuera de todos los filtros
	// de la bandeja, que es una forma silenciosa de perderlo.
	if err := s.CambiarEstado(context.Background(), "id-1", "archivado"); err == nil {
		t.Fatal("un estado inventado debia rechazarse")
	}
	if err := s.CambiarEstado(context.Background(), "id-1", domain.ContactoRespondido); err != nil {
		t.Fatalf("un estado valido no debia fallar: %v", err)
	}
	if repo.estados["id-1"] != domain.ContactoRespondido {
		t.Error("no se guardo el estado nuevo")
	}
	if _, err := s.Listar(context.Background(), "inventado"); err == nil {
		t.Error("filtrar por un estado inventado debia rechazarse")
	}
}
