package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
	"errors"
	"log"
	"net/url"
	"strings"
)

// Eventos de pago con link externo y aprobación manual (parte B, §7 del plan de
// workshops; decisiones cerradas el 2026-10-28). CredibanCo sigue bloqueado:
// quien se inscribe a un evento de pago queda pendiente, paga por el link del
// evento, y el administrador aprueba o rechaza desde el panel tras cruzar el
// pago por fuera del sistema.

var (
	// ErrCupoLleno: el evento llegó a su límite de asistentes. Cuentan las
	// confirmadas y las pendientes; solo frena a la app (§7.5, decisión 8).
	ErrCupoLleno = errors.New("ya no quedan cupos para este evento")
	// ErrInscripcionNoPendiente: solo se aprueba o se rechaza una inscripción
	// pendiente de pago.
	ErrInscripcionNoPendiente = errors.New("esta inscripción ya no está pendiente de pago")
)

// ErrorEvento es un evento que no se puede guardar. Motivo va tal cual al
// panel, en español.
type ErrorEvento struct {
	Motivo string
}

func (e *ErrorEvento) Error() string { return e.Motivo }

// ConAvisos habilita el aviso push al aprobar y al rechazar una inscripción.
// Aparte del constructor, como ConCorreoDeInscripcion: sin él, las dos acciones
// funcionan igual y solo se omite el aviso.
func (s *EventService) ConAvisos(notifier ports.NotificationService) *EventService {
	s.notifier = notifier
	return s
}

// validarLinkDePago: todo evento que no es gratuito lleva un link de pago
// https (§7.1). La regla vive aquí y no solo en el panel.
func validarLinkDePago(e *domain.Event) error {
	if e.PaymentURL != nil {
		limpio := strings.TrimSpace(*e.PaymentURL)
		if limpio == "" {
			e.PaymentURL = nil
		} else {
			e.PaymentURL = &limpio
		}
	}
	if e.PaymentURL == nil {
		if e.IsFree {
			return nil
		}
		return &ErrorEvento{Motivo: "un evento de pago necesita el link de pago"}
	}
	u, err := url.Parse(*e.PaymentURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return &ErrorEvento{Motivo: "el link de pago tiene que ser una dirección https://"}
	}
	return nil
}

// comprobarCupo frena a la app cuando el evento tiene límite y ya está lleno.
// Sin límite cargado no hay tope (§7.5, decisión 7).
func (s *EventService) comprobarCupo(ctx context.Context, evento *domain.Event) error {
	if evento.AttendeesLimit == nil || *evento.AttendeesLimit <= 0 {
		return nil
	}
	ocupados, err := s.repo.CountActiveRegistrations(ctx, evento.ID)
	if err != nil {
		return err
	}
	if ocupados >= *evento.AttendeesLimit {
		return ErrCupoLleno
	}
	return nil
}

// AprobarInscripcion confirma una inscripción pendiente tras confirmar el pago:
// queda confirmada y pagada, recibe su credencial si el evento es presencial,
// el correo de confirmación y un aviso push. Aprobar una ya confirmada no
// repite nada (idempotente); una rechazada no se aprueba.
func (s *EventService) AprobarInscripcion(ctx context.Context, registrationID string) (*domain.Registration, error) {
	reg, err := s.repo.GetRegistrationByID(ctx, registrationID)
	if err != nil {
		return nil, err
	}
	switch reg.RegistrationStatus {
	case domain.RegistrationConfirmed:
		return reg, nil
	case domain.RegistrationPendingPayment:
	default:
		return nil, ErrInscripcionNoPendiente
	}

	if err := s.repo.ConfirmEventRegistration(ctx, reg.UserID, reg.EventID); err != nil {
		return nil, err
	}
	reg.RegistrationStatus = domain.RegistrationConfirmed
	reg.PaymentStatus = "paid"

	evento, err := s.repo.GetEventByID(ctx, reg.EventID)
	if err != nil {
		return nil, err
	}
	// La reserva ya trae QR, porque se genera al reservar; esto solo rellena
	// el de una inscripción que por algún camino llegara sin él.
	if !evento.IsVirtual {
		if _, err := s.GenerarCredenciales(ctx, reg.EventID, []string{reg.ID}, false); err != nil {
			log.Printf("[PAGO] credencial de la inscripción %s: %v", reg.ID, err)
		}
	}
	s.enviarCorreoInscripcion(reg, evento)
	s.avisarInscripcion(reg.UserID, reg.EventID, "Pago confirmado",
		"Tu inscripción a «"+evento.Title+"» está confirmada. Ya puedes ver tu credencial.",
		"inscripcion_aprobada")
	return reg, nil
}

// RechazarInscripcion marca como rechazada una inscripción pendiente: libera
// el cupo, no da acceso, y la persona puede volver a reservar. Avisa por push.
func (s *EventService) RechazarInscripcion(ctx context.Context, registrationID string) (*domain.Registration, error) {
	reg, err := s.repo.GetRegistrationByID(ctx, registrationID)
	if err != nil {
		return nil, err
	}
	if reg.RegistrationStatus == domain.RegistrationRejected {
		return reg, nil
	}
	if reg.RegistrationStatus != domain.RegistrationPendingPayment {
		return nil, ErrInscripcionNoPendiente
	}
	if err := s.repo.RejectRegistration(ctx, reg.ID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// Alguien la aprobó entre la lectura y este UPDATE.
			return nil, ErrInscripcionNoPendiente
		}
		return nil, err
	}
	reg.RegistrationStatus = domain.RegistrationRejected

	titulo := "el evento"
	if evento, err := s.repo.GetEventByID(ctx, reg.EventID); err == nil {
		titulo = "«" + evento.Title + "»"
	}
	s.avisarInscripcion(reg.UserID, reg.EventID, "Pago no confirmado",
		"No pudimos confirmar el pago de tu inscripción a "+titulo+". Puedes volver a reservar desde el evento.",
		"inscripcion_rechazada")
	return reg, nil
}

// avisarInscripcion manda el push a los dispositivos de esa persona, en segundo
// plano: que no tenga ninguno registrado es lo normal si nunca abrió la app en
// un teléfono, y no debe tumbar la aprobación. La app traduce {type, id} a una
// pantalla en notificacion_destino.dart.
func (s *EventService) avisarInscripcion(userID, eventID, titulo, cuerpo, tipo string) {
	if s.notifier == nil {
		return
	}
	go func() {
		datos := map[string]string{"type": tipo, "id": eventID}
		if err := s.notifier.SendToUser(context.Background(), userID, titulo, cuerpo, datos); err != nil {
			log.Printf("[PAGO] no se pudo avisar a %s (%s): %v", userID, tipo, err)
		}
	}()
}

// reactivarInscripcion vuelve a poner pendiente una inscripción rechazada, con
// la elección de workshops nueva. Si la vuelve a inscribir un administrador ya
// pagada, queda confirmada como cualquier inscripción manual.
func (s *EventService) reactivarInscripcion(ctx context.Context, reg, previa *domain.Registration, evento *domain.Event, workshopIDs []string) error {
	pagada := reg.PaymentStatus == "paid"
	reg.ID = previa.ID
	reg.QRData = previa.QRData
	reg.RegistrationDate = previa.RegistrationDate
	if err := s.repo.ReactivateRegistration(ctx, reg, workshopIDs); err != nil {
		return err
	}
	reg.RegistrationStatus = domain.RegistrationPendingPayment
	reg.PaymentStatus = "pending"

	if pagada || evento.IsFree {
		if err := s.repo.ConfirmEventRegistration(ctx, reg.UserID, reg.EventID); err != nil {
			return err
		}
		reg.RegistrationStatus = domain.RegistrationConfirmed
		reg.PaymentStatus = "paid"
	}
	return nil
}
