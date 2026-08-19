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

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), tiempoMaximoCorreo)
		defer cancel()
		_ = ctx // el puerto de correo no recibe contexto; el timeout acota la goroutine

		if err := s.email.SendEventRegistrationEmail(datos); err != nil {
			log.Printf("[correo inscripción] %s: %v", reg.ID, err)
		}
	}()
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
