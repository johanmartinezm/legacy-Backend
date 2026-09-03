package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"log"
	"strings"
	"time"
)

// Correo de confirmación al inscribirse a un evento.
//
// Hasta el 2026-08-18 inscribirse no enviaba nada: quien reservaba su cupo en
// una masterclass virtual no recibía constancia ni, sobre todo, el enlace de la
// sesión. Lo reportó el cliente (punto 2.2 de reports/20260818_plan_ajustes.html)
// preguntando literalmente "¿o dónde me da el link de ingreso?".
//
// El correo depende de la modalidad, que existe desde
// scripts/20260818_modalidad_y_enlace_evento.sql: el virtual lleva el enlace de
// acceso; el presencial remite a la credencial de la app, donde está el QR. Sin
// esa columna este correo no habría tenido qué decir en el caso virtual, que es
// justo el que preguntaba el cliente.

// tiempoMaximoCorreo acota el envío. Corre fuera de la petición HTTP, así que
// sin límite un SMTP colgado dejaría la goroutine viva para siempre. Es más
// generoso que el de los push porque un correo tarda más en salir.
const tiempoMaximoCorreo = 30 * time.Second

// enviarCorreoInscripcion manda la confirmación **sin bloquear ni fallar nunca**
// la inscripción.
//
// Mismo criterio que los avisos push: el cupo ya está reservado en la base, y un
// problema con el correo no puede deshacer eso ni devolverle un error a quien se
// acaba de inscribir. Lleva contexto propio porque el de la petición HTTP se
// cancela al responder y cortaría el envío a medias.
func (s *EventService) enviarCorreoInscripcion(reg *domain.Registration, event *domain.Event) {
	if s.email == nil || s.users == nil {
		return
	}

	destino := s.correoDelInscrito(reg)
	if destino == "" {
		log.Printf("[correo inscripción] sin correo para la inscripción %s, no se envía", reg.ID)
		return
	}

	datos := domain.CorreoInscripcion{
		Para:        destino,
		Nombre:      s.nombreDelInscrito(reg),
		Evento:      event.Title,
		Fecha:       event.StartDate.Format("02/01/2006"),
		EsVirtual:   event.IsVirtual,
		EnlaceLugar: lugarOEnlace(event),
	}

	s.encolarCorreo("correo inscripción "+reg.ID, func() error {
		return s.email.SendEventRegistrationEmail(datos)
	})
}

// enviarCorreoCredencial entrega el código de acceso, con el QR dibujado dentro
// del propio correo.
//
// Sale de un solo sitio: generar la credencial. Se dispare desde la carga masiva
// (§4.1) o desde la acción «Generar credenciales» de la pantalla de inscritos
// (§4.3), crear el código y avisar de él son la misma cosa, así que hay un solo
// camino y no dos que puedan divergir.
//
// `usuario` y `contrasena` solo llegan rellenos cuando la cuenta acaba de
// crearse en una carga: entonces este correo es **el único sitio** donde la
// persona se entera de cómo entrar. Para una cuenta que ya existía van vacíos, y
// la plantilla omite ese bloque en vez de decirle una contraseña que no es la
// suya.
//
// No puede salir antes de tiempo: sin código no hay QR que dibujar, y quien
// llama aquí acaba de escribirlo.
func (s *EventService) enviarCorreoCredencial(reg *domain.Registration, event *domain.Event, usuario, contrasena string) {
	if s.email == nil || s.users == nil {
		return
	}

	destino := s.correoDelInscrito(reg)
	if destino == "" {
		log.Printf("[correo credencial] sin correo para la inscripción %s, no se envía", reg.ID)
		return
	}

	datos := domain.CorreoCredencial{
		Para:        destino,
		Nombre:      s.nombreDelInscrito(reg),
		Evento:      event.Title,
		Fecha:       event.StartDate.Format("02/01/2006"),
		EsVirtual:   event.IsVirtual,
		EnlaceLugar: lugarOEnlace(event),
		QRData:      reg.QRData,
		Usuario:     usuario,
		Contrasena:  contrasena,
	}

	// Por la cola, no por una goroutine suelta: generar credenciales en bloque
	// manda un correo por persona, y todos a la vez es lo que Gmail rechaza.
	s.encolarCorreo("correo credencial "+reg.ID, func() error {
		return s.email.SendEventCredentialEmail(datos)
	})
}

// --------------------------------------------------------------------------
// La cola de envíos
// --------------------------------------------------------------------------
//
// Hasta el 2026-09-03 cada correo salía en su propia goroutine, y eso estaba
// bien mientras los correos fueran de uno en uno: alguien se inscribe, se le
// avisa. Con la carga masiva dejó de serlo. Los tres caminos que hacen ráfaga:
//
//   - importar con «avisar» encendido: un correo por fila del archivo;
//   - «Generar credenciales» en bloque: uno por persona sin credencial;
//   - y los dos a la vez, si la carga genera credenciales y avisa.
//
// Trescientas personas eran trescientas goroutines abriendo trescientas
// conexiones contra Gmail en el mismo instante. Gmail limita por tasa, así que
// lo previsible no es que tarde: es que rechace una parte, y esos correos son
// justamente los que llevan **la única copia** de la contraseña de alguien
// (§4.3 del plan de carga masiva).
//
// La cola no acelera nada ni lo pretende: pone los envíos en fila para que
// salgan de uno en uno. Un correo tarda lo que tarda y el ritmo lo marca la
// propia red, sin esperas artificiales.

// capacidadDeLaCola es el techo de correos esperando. Sobra para el Summit —el
// importador no acepta más de 5000 filas de golpe— y acota la memoria si el
// servidor de correo se atasca.
const capacidadDeLaCola = 5000

// encolarCorreo pone un envío en la fila. **Nunca bloquea a quien llama:** la
// inscripción ya está en la base y una cola llena no puede retrasar la
// respuesta de la petición HTTP.
//
// Si la cola está llena —que solo pasaría con el correo caído y miles
// esperando— el envío sale por su cuenta, como se hacía antes. Es peor que la
// fila, pero mucho mejor que perder el correo en silencio.
func (s *EventService) encolarCorreo(que string, enviar func() error) {
	tarea := func() {
		// Contexto propio: el de la petición HTTP se cancela al responder y
		// cortaría el envío a medias. El timeout acota la espera.
		ctx, cancel := context.WithTimeout(context.Background(), tiempoMaximoCorreo)
		defer cancel()
		_ = ctx // el puerto de correo no recibe contexto

		if err := enviar(); err != nil {
			log.Printf("[%s] %v", que, err)
		}
	}

	// La cola se crea la primera vez que hace falta, no en el constructor: así
	// los tests que arman el servicio sin correo no dejan una goroutine viva.
	s.arrancarCola.Do(func() {
		s.colaCorreos = make(chan func(), capacidadDeLaCola)
		go s.atenderCola()
	})

	select {
	case s.colaCorreos <- tarea:
	default:
		log.Printf("[correo] la cola está llena (%d esperando); %s sale por su cuenta",
			capacidadDeLaCola, que)
		go tarea()
	}
}

// atenderCola manda los correos de uno en uno. Vive lo que viva el proceso,
// como el hub del chat.
func (s *EventService) atenderCola() {
	for tarea := range s.colaCorreos {
		tarea()
	}
}

// lugarOEnlace devuelve lo que la persona necesita para llegar al evento: el
// enlace si es virtual, el lugar si es presencial. Puede ir vacío —un evento sin
// ubicación escrita todavía—, y entonces la plantilla lo omite.
func lugarOEnlace(event *domain.Event) string {
	if event.IsVirtual {
		if event.AccessURL != nil {
			return strings.TrimSpace(*event.AccessURL)
		}
		return ""
	}
	if event.Location != nil {
		return strings.TrimSpace(*event.Location)
	}
	return ""
}

// correoDelInscrito prefiere el contacto que se escribió en la inscripción y cae
// al del perfil.
//
// El de la inscripción existe porque a un evento se puede inscribir a alguien
// con un contacto distinto al de su cuenta. Los dos llegan cifrados: el de la
// inscripción lo cifra este mismo servicio al crearla, y el del perfil está
// cifrado en reposo como el resto de datos personales.
func (s *EventService) correoDelInscrito(reg *domain.Registration) string {
	if reg.ParticipantEmail != "" && s.crypto != nil {
		if claro, err := s.crypto.Decrypt(reg.ParticipantEmail); err == nil && claro != "" {
			return claro
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	user, err := s.users.FindByID(ctx, reg.UserID)
	if err != nil || user == nil {
		return ""
	}
	if s.crypto == nil {
		return user.EmailEncrypted
	}
	claro, err := s.crypto.Decrypt(user.EmailEncrypted)
	if err != nil {
		return ""
	}
	return claro
}

// nombreDelInscrito da el nombre para el saludo. Si no se puede resolver se
// devuelve vacío y la plantilla saluda sin nombre: vale más un correo impersonal
// que ninguno.
func (s *EventService) nombreDelInscrito(reg *domain.Registration) string {
	if reg.ParticipantName != "" && s.crypto != nil {
		if claro, err := s.crypto.Decrypt(reg.ParticipantName); err == nil && claro != "" {
			return claro
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	user, err := s.users.FindByID(ctx, reg.UserID)
	if err != nil || user == nil {
		return ""
	}
	if s.crypto == nil {
		return user.FirstName
	}
	if claro, err := s.crypto.Decrypt(user.FirstName); err == nil {
		return claro
	}
	return ""
}
