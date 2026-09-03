package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"
	"testing"
)

// La entrada del evento de la carga masiva: fase 3 del plan
// (reports/20260826_plan_carga_masiva.md §4 A y §4.1).
//
// Lo que se prueba aquí no es que se creen cuentas —eso es la fase 1 y ya está
// en importacion_service_test.go— sino las tres cosas que esta entrada añade:
// que la gente queda inscrita al evento de la pantalla, que los dos
// interruptores hacen lo que dicen, y que volver a pasar el mismo archivo no
// duplica a nadie.

// importadorConEvento arma el motor con las dos dependencias.
func importadorConEvento(auth *authFalso, eventos *eventosFalsos) *ImportacionService {
	return NewImportacionService(auth, eventos)
}

// conEvento son las opciones de la entrada A: el evento lo fija la URL de la
// pantalla, no el archivo.
func conEvento(generarCredencial, avisar bool) domain.OpcionesImportacion {
	return domain.OpcionesImportacion{
		EventoID:          "evento-summit",
		GenerarCredencial: generarCredencial,
		AvisarPorCorreo:   avisar,
	}
}

func TestEntradaDeEvento_CreaLaCuentaYLaInscribe(t *testing.T) {
	auth := nuevoAuthFalso()
	eventos := nuevosEventosFalsos()
	s := importadorConEvento(auth, eventos)

	res, err := s.Aplicar(context.Background(), []domain.FilaImportacion{
		filaBuena(2, "ana@empresa.com"),
		filaBuena(3, "luis@empresa.com"),
	}, conEvento(false, false))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if res.TieneProblemas() {
		t.Fatalf("problemas inesperados: %+v", res.Problemas)
	}

	if res.Creadas != 2 || res.Inscritas != 2 {
		t.Errorf("se esperaban 2 creadas y 2 inscritas, llegó %+v", res)
	}
	if len(eventos.inscripciones) != 2 {
		t.Fatalf("inscripciones esperadas 2, hubo %d", len(eventos.inscripciones))
	}

	// El evento sale de las opciones, nunca del archivo: la columna «Ticket» no
	// se lee, y por eso esta entrada no tiene que resolverla a nada.
	for _, reg := range eventos.inscripciones {
		if reg.EventID != "evento-summit" {
			t.Errorf("inscrita al evento %q", reg.EventID)
		}
		// El id tiene que ser el que devolvió el alta de la cuenta: sin él se
		// inscribiría a nadie.
		if reg.UserID == "" {
			t.Error("se inscribió sin usuario")
		}
	}
}

func TestEntradaDeEvento_UnaCuentaQueYaExisteTambienQuedaInscrita(t *testing.T) {
	// Es la mitad que importa de esta entrada: alguien que ya es miembro y
	// además va al Summit. Si solo se inscribiera a las cuentas nuevas, media
	// lista se quedaría fuera sin avisar.
	auth := nuevoAuthFalso("ana@empresa.com")
	eventos := nuevosEventosFalsos()

	res, err := importadorConEvento(auth, eventos).Aplicar(context.Background(),
		[]domain.FilaImportacion{filaBuena(2, "ana@empresa.com")}, conEvento(false, false))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if res.Creadas != 0 || res.YaExistian != 1 {
		t.Errorf("no debía crear cuenta: %+v", res)
	}
	if res.Inscritas != 1 {
		t.Errorf("la cuenta que ya existía no quedó inscrita: %+v", res)
	}
	if len(auth.creados) != 0 {
		t.Errorf("se creó una cuenta que ya existía: %+v", auth.creados)
	}
}

func TestEntradaDeEvento_ReejecutarNoDuplicaInscripciones(t *testing.T) {
	// El mismo archivo dos veces. La segunda pasada no crea cuentas ni
	// inscripciones, y el informe lo dice en vez de dejar creer que no hizo
	// nada.
	auth := nuevoAuthFalso()
	eventos := nuevosEventosFalsos()
	s := importadorConEvento(auth, eventos)
	filas := []domain.FilaImportacion{filaBuena(2, "ana@empresa.com")}

	if _, err := s.Aplicar(context.Background(), filas, conEvento(false, false)); err != nil {
		t.Fatalf("primera pasada: %v", err)
	}

	res, err := s.Aplicar(context.Background(), filas, conEvento(false, false))
	if err != nil {
		t.Fatalf("segunda pasada: %v", err)
	}

	if res.Creadas != 0 {
		t.Errorf("la segunda pasada creó %d cuentas", res.Creadas)
	}
	if res.Inscritas != 0 || res.YaInscritas != 1 {
		t.Errorf("se esperaba 0 inscritas y 1 ya inscrita, llegó %+v", res)
	}
	if len(eventos.inscripciones) != 1 {
		t.Errorf("hay %d inscripciones para la misma persona", len(eventos.inscripciones))
	}
}

func TestEntradaDeEvento_LosDosInterruptores(t *testing.T) {
	// La tabla de §4.1, entera. Lo que se comprueba es lo que llega a
	// RegisterUser, que es quien decide si genera el código y qué correo manda.
	casos := []struct {
		nombre        string
		credencial    bool
		avisar        bool
		sinCredencial bool
		aviso         domain.AvisoDeAlta
	}{
		{
			nombre:        "generar y avisar: sale el correo de credencial, con el QR",
			credencial:    true,
			avisar:        true,
			sinCredencial: false,
			aviso:         domain.AvisoCredencial,
		},
		{
			nombre:        "generar sin avisar: la credencial ya se ve en la app",
			credencial:    true,
			avisar:        false,
			sinCredencial: false,
			aviso:         domain.AvisoNinguno,
		},
		{
			nombre:        "sin credencial y avisando: el correo de inscripción de siempre",
			credencial:    false,
			avisar:        true,
			sinCredencial: true,
			aviso:         domain.AvisoPorDefecto,
		},
		{
			nombre:        "los dos apagados, que es el valor por defecto: nada",
			credencial:    false,
			avisar:        false,
			sinCredencial: true,
			aviso:         domain.AvisoNinguno,
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			eventos := nuevosEventosFalsos()
			_, err := importadorConEvento(nuevoAuthFalso(), eventos).Aplicar(
				context.Background(),
				[]domain.FilaImportacion{filaBuena(2, "ana@empresa.com")},
				conEvento(c.credencial, c.avisar))
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}

			if len(eventos.inscripciones) != 1 {
				t.Fatalf("inscripciones: %d", len(eventos.inscripciones))
			}
			reg := eventos.inscripciones[0]

			if reg.Importacion.SinCredencial != c.sinCredencial {
				t.Errorf("SinCredencial=%v, se esperaba %v",
					reg.Importacion.SinCredencial, c.sinCredencial)
			}
			if reg.Importacion.Aviso != c.aviso {
				t.Errorf("Aviso=%q, se esperaba %q", reg.Importacion.Aviso, c.aviso)
			}
		})
	}
}

func TestEntradaDeEvento_LasCredencialesDeAccesoSoloVanEnUnaCuentaNueva(t *testing.T) {
	// El correo de credencial es el único sitio donde alguien recién importado
	// se entera de cómo entrar: su correo y su número de documento. A quien ya
	// tenía cuenta no se le puede decir eso, porque su contraseña es la suya.
	auth := nuevoAuthFalso("vieja@empresa.com")
	eventos := nuevosEventosFalsos()

	_, err := importadorConEvento(auth, eventos).Aplicar(context.Background(),
		[]domain.FilaImportacion{
			filaBuena(2, "nueva@empresa.com"),
			filaBuena(3, "vieja@empresa.com"),
		}, conEvento(true, true))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	porCorreo := map[string]domain.Registration{}
	for _, reg := range eventos.inscripciones {
		porCorreo[reg.Importacion.Usuario] = reg
	}

	nueva, ok := porCorreo["nueva@empresa.com"]
	if !ok {
		t.Fatalf("la cuenta nueva no lleva sus credenciales: %+v", eventos.inscripciones)
	}
	if nueva.Importacion.Contrasena != "1020304050" {
		t.Errorf("la contraseña tenía que ser el documento, llegó %q",
			nueva.Importacion.Contrasena)
	}

	// La que ya existía va con usuario y contraseña vacíos, y así la plantilla
	// omite ese bloque.
	if _, hayVacia := porCorreo[""]; !hayVacia {
		t.Errorf("a la cuenta que ya existía se le mandaron credenciales: %+v", eventos.inscripciones)
	}
}

func TestEntradaDeEvento_SimularNoInscribeYDiceCuantasSerian(t *testing.T) {
	auth := nuevoAuthFalso("vieja@empresa.com")
	eventos := nuevosEventosFalsos()

	res, err := importadorConEvento(auth, eventos).Simular(context.Background(),
		[]domain.FilaImportacion{
			filaBuena(2, "nueva@empresa.com"),
			filaBuena(3, "vieja@empresa.com"),
		}, conEvento(true, true))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if len(eventos.inscripciones) != 0 {
		t.Error("la simulación inscribió a alguien")
	}
	if len(auth.creados) != 0 {
		t.Error("la simulación creó cuentas")
	}
	// Las dos quedarían inscritas: la nueva y la que ya tenía cuenta.
	if res.PorInscribir != 2 {
		t.Errorf("PorInscribir=%d, se esperaba 2", res.PorInscribir)
	}
	if res.Inscritas != 0 {
		t.Errorf("una simulación no puede reportar inscritas: %+v", res)
	}
}

func TestEntradaDeEvento_UnaFilaMalaNoInscribeNiCreaNada(t *testing.T) {
	auth := nuevoAuthFalso()
	eventos := nuevosEventosFalsos()

	res, err := importadorConEvento(auth, eventos).Aplicar(context.Background(),
		[]domain.FilaImportacion{
			filaBuena(2, "ana@empresa.com"),
			filaBuena(3, "sin-arroba"),
		}, conEvento(true, true))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if !res.TieneProblemas() || !res.Simulacion {
		t.Errorf("el informe debería decir que no se aplicó nada: %+v", res)
	}
	if len(auth.creados) != 0 || len(eventos.inscripciones) != 0 {
		t.Errorf("se escribió algo pese al problema: cuentas=%d inscripciones=%d",
			len(auth.creados), len(eventos.inscripciones))
	}
}

func TestEntradaDeEvento_SiFallaLaInscripcionLoDiceConSuFila(t *testing.T) {
	// La cuenta ya quedó creada: el mensaje tiene que decirlo, porque volver a
	// pasar el archivo la saltará por existir y la persona seguiría sin estar
	// inscrita.
	auth := nuevoAuthFalso()
	eventos := nuevosEventosFalsos()
	eventos.err = errors.New("cupo agotado")

	res, err := importadorConEvento(auth, eventos).Aplicar(context.Background(),
		[]domain.FilaImportacion{filaBuena(9, "ana@empresa.com")}, conEvento(false, false))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if !res.TieneProblemas() {
		t.Fatalf("el fallo no se reportó: %+v", res)
	}
	p := res.Problemas[0]
	if p.Fila != 9 {
		t.Errorf("fila esperada 9, llegó %d", p.Fila)
	}
	if res.Creadas != 1 {
		t.Errorf("la cuenta sí se creó, el informe debería decirlo: %+v", res)
	}
}

func TestEntradaDeEvento_SinServicioDeEventosNoSeCreaNadaEnSilencio(t *testing.T) {
	// El fallo que no queremos: crear trescientas cuentas y olvidarse de
	// inscribirlas porque el cableado de main.go se quedó a medias.
	auth := nuevoAuthFalso()
	s := NewImportacionService(auth, nil)

	if _, err := s.Aplicar(context.Background(),
		[]domain.FilaImportacion{filaBuena(2, "ana@empresa.com")},
		conEvento(false, false)); !errors.Is(err, ErrSinServicioDeEventos) {
		t.Errorf("se esperaba ErrSinServicioDeEventos, llegó %v", err)
	}
	if len(auth.creados) != 0 {
		t.Error("se crearon cuentas antes de darse cuenta de que no puede inscribir")
	}

	if _, err := s.Simular(context.Background(),
		[]domain.FilaImportacion{filaBuena(2, "ana@empresa.com")},
		conEvento(false, false)); !errors.Is(err, ErrSinServicioDeEventos) {
		t.Errorf("la simulación también tiene que avisar, llegó %v", err)
	}
}

func TestEntradaGenerica_NoInscribeAunqueHayaServicioDeEventos(t *testing.T) {
	// «Importar usuarios» es solo cuentas. Es la mitad barata del trabajo y no
	// puede acabar inscribiendo a nadie a ningún evento.
	auth := nuevoAuthFalso()
	eventos := nuevosEventosFalsos()

	res, err := importadorConEvento(auth, eventos).Aplicar(context.Background(),
		[]domain.FilaImportacion{filaBuena(2, "ana@empresa.com")}, sinEvento)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if res.Creadas != 1 {
		t.Errorf("no creó la cuenta: %+v", res)
	}
	if len(eventos.inscripciones) != 0 {
		t.Errorf("inscribió a alguien sin evento: %+v", eventos.inscripciones)
	}
	if res.Inscritas != 0 || res.PorInscribir != 0 {
		t.Errorf("el informe habla de inscripciones que no hubo: %+v", res)
	}
}
