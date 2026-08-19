package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"net/url"
	"strings"

	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"github.com/google/uuid"
)

type paymentService struct {
	txRepo    ports.TransactionRepository
	gateway   ports.PaymentGateway
	eventRepo ports.EventRepository

	// Piezas del correo de pago aprobado. Las tres pueden ser nil: sin ellas el
	// cobro se verifica y la inscripción se confirma igual, y solo se omite el
	// correo. Se inyectan con ConCorreoDePago para no romper las llamadas
	// existentes ni los tests, que no lo necesitan.
	users  ports.UserRepository
	email  ports.EmailService
	crypto ports.CryptoService
}

// NewPaymentService recibe eventRepo para poder confirmar la inscripción cuando
// la pasarela aprueba un cobro de tipo EVENT. Admite nil: en ese caso el pago se
// verifica igual y solo se omite la confirmación de la inscripción.
func NewPaymentService(txRepo ports.TransactionRepository, gateway ports.PaymentGateway, eventRepo ports.EventRepository) ports.PaymentService {
	return &paymentService{
		txRepo:    txRepo,
		gateway:   gateway,
		eventRepo: eventRepo,
	}
}

// Errores del inicio de pago, para que el handler distinga 400, 404 y 409.
var (
	ErrPaymentEventNotFound  = errors.New("event not found")
	ErrPaymentEventIsFree    = errors.New("event is free, no payment needed")
	ErrPaymentAmountMismatch = errors.New("amount does not match the event price")
	// ErrPaymentGatewayUnavailable: la pasarela rechazó o no atendió la
	// petición. No es un fallo nuestro, y conviene distinguirlo de un 500 para
	// no mandar a buscar la avería en el sitio equivocado.
	ErrPaymentGatewayUnavailable = errors.New("payment gateway unavailable")
	// ErrPaymentNotificationEmpty: la notificación llegó sin ningún
	// identificador y no hay nada que mirar.
	ErrPaymentNotificationEmpty = errors.New("notification without a transaction reference")
	// ErrPaymentNotificationUnknown: el identificador no corresponde a ninguna
	// transacción nuestra. No es un error del servidor: puede ser una prueba de
	// la pasarela, un reenvío antiguo o alguien tanteando la URL.
	ErrPaymentNotificationUnknown = errors.New("notification for an unknown transaction")
)

// toleranciaImporte: los importes viajan como float y se guardan como
// numeric(10,2). Comparar con == daría falsos negativos por el redondeo, así
// que se admite menos de un centavo de diferencia.
const toleranciaImporte = 0.005

// metodoDePagoConocido filtra lo que llega del cliente. Es un dato informativo,
// así que un valor raro se descarta en silencio en vez de tumbar la compra:
// nadie debería quedarse sin poder pagar porque una versión futura de la app
// mande una etiqueta nueva.
func metodoDePagoConocido(metodo string) string {
	switch metodo {
	case "credit_card", "pse":
		return metodo
	default:
		return ""
	}
}

func (s *paymentService) InitiatePayment(ctx context.Context, userID uuid.UUID, refType domain.ReferenceType, refID uuid.UUID, amount float64, returnUrl string, paymentMethod string) (string, error) {
	// El importe lo decide el servidor, no el cliente.
	//
	// Antes se cobraba tal cual lo que llegaba en el cuerpo de la peticion, sin
	// contrastarlo nunca con el precio real —que el backend tiene a mano, porque
	// reference_id ES el id del evento—. Un {"amount": 1000} en lugar de 250000
	// se cobraba por mil pesos.
	if refType == domain.RefTypeEvent {
		precio, err := s.precioDelEvento(ctx, refID)
		if err != nil {
			return "", err
		}
		if math.Abs(amount-precio) > toleranciaImporte {
			// Se rechaza en vez de cobrar el precio correcto en silencio: si el
			// cliente traia otro importe es que su informacion esta obsoleta, y
			// cobrar algo distinto de lo que el usuario vio en pantalla es peor
			// que pedirle que lo revise.
			return "", fmt.Errorf("%w: el cliente envió %.2f y el evento cuesta %.2f",
				ErrPaymentAmountMismatch, amount, precio)
		}
		amount = precio
	}

	// Create transaction in pending state
	tx := &domain.Transaction{
		ID:            uuid.New(),
		UserID:        userID,
		ReferenceType: refType,
		ReferenceID:   refID,
		Amount:        amount,
		Status:        domain.TxStatusPending,
		PaymentMethod: metodoDePagoConocido(paymentMethod),
	}

	if err := s.txRepo.CreateTransaction(ctx, tx); err != nil {
		return "", fmt.Errorf("failed to create transaction: %w", err)
	}

	// El id de NUESTRA transacción viaja en la URL de retorno.
	//
	// Al volver de la pasarela, la app necesita saber qué verificar, y
	// /api/payments/verify espera este id, no el de CredibanCo. Depender de que
	// la pasarela añada un parámetro con un nombre concreto sería frágil: aquí
	// se garantiza que está, se llame como se llame lo que ella agregue.
	returnUrl = conParametro(returnUrl, "tx_id", tx.ID.String())

	// Call CredibanCo gateway
	orderId, formUrl, err := s.gateway.CreatePaymentIntent(ctx, amount, tx.ID.String(), returnUrl)
	if err != nil {
		// Update status to FAILED since gateway failed
		_ = s.txRepo.UpdateTransactionStatus(ctx, tx.ID, domain.TxStatusFailed, "")
		// El detalle va al log del servidor, no al usuario: puede traer códigos
		// y mensajes del banco que no le dicen nada y que tampoco conviene
		// exponer.
		log.Printf("[PAGO] la pasarela rechazo la transaccion %s (usuario %s, %.2f): %v",
			tx.ID, userID, amount, err)
		return "", fmt.Errorf("%w: %v", ErrPaymentGatewayUnavailable, err)
	}

	// Update with credibanco order id
	if err := s.txRepo.UpdateTransactionStatus(ctx, tx.ID, domain.TxStatusPending, orderId); err != nil {
		return "", fmt.Errorf("failed to update transaction with order id: %w", err)
	}

	return formUrl, nil
}

// conParametro añade una pareja clave=valor a la query de una URL, conservando
// lo que ya trajera. Si la URL no se puede interpretar se concatena a mano, para
// no perder el parámetro por un formato inesperado.
func conParametro(rawURL, clave, valor string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		separador := "?"
		if strings.Contains(rawURL, "?") {
			separador = "&"
		}
		return rawURL + separador + url.QueryEscape(clave) + "=" + url.QueryEscape(valor)
	}
	q := u.Query()
	q.Set(clave, valor)
	u.RawQuery = q.Encode()
	return u.String()
}

// precioDelEvento devuelve el precio que manda: el de la base de datos.
func (s *paymentService) precioDelEvento(ctx context.Context, eventID uuid.UUID) (float64, error) {
	if s.eventRepo == nil {
		return 0, fmt.Errorf("no hay repositorio de eventos para validar el importe")
	}

	event, err := s.eventRepo.GetEventByID(ctx, eventID.String())
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return 0, ErrPaymentEventNotFound
		}
		return 0, err
	}
	// Un evento gratuito no pasa por la pasarela: se entra por
	// POST /api/events/{id}/register, que lo deja confirmado en el acto.
	if event.IsFree || event.Price <= 0 {
		return 0, ErrPaymentEventIsFree
	}
	return event.Price, nil
}

// ProcessGatewayNotification atiende el aviso que CredibanCo manda cuando una
// operación cambia de estado. Existe porque hasta ahora la confirmación dependía
// de que el usuario volviera a la app: quien pagaba y cerraba el navegador
// dejaba el cobro hecho y la inscripción sin confirmar para siempre.
//
// **El contenido de la notificación no decide nada.** Solo se usa para saber qué
// transacción mirar; el estado se pregunta a la pasarela con nuestras
// credenciales, igual que hace VerifyPayment. Eso es lo que permite exponer la
// ruta sin autenticación —la pasarela no tiene un token nuestro— sin que sea un
// agujero: quien descubra la URL e invente una notificación no puede declarar
// nada aprobado, solo provocar que consultemos un estado que ya conocemos.
//
// La referencia puede ser el id de nuestra transacción (que viaja como
// orderNumber) o el que asigna CredibanCo (mdOrder). Se aceptan los dos porque
// no está confirmado cuál enviará.
func (s *paymentService) ProcessGatewayNotification(ctx context.Context, referencia string) (*domain.Transaction, error) {
	referencia = strings.TrimSpace(referencia)
	if referencia == "" {
		return nil, ErrPaymentNotificationEmpty
	}

	tx, err := s.transaccionDeLaNotificacion(ctx, referencia)
	if err != nil {
		return nil, err
	}

	// Una transacción ya resuelta no se vuelve a consultar a la pasarela: la
	// ruta es pública, y sin este corte cualquiera podría hacernos repetir
	// llamadas salientes indefinidamente con una referencia válida.
	if tx.Status == domain.TxStatusApproved {
		// Sí se reintenta la confirmación de la inscripción, que es local,
		// barata e idempotente: si la primera vez el pago se marcó aprobado pero
		// falló al tocar events.registrations, este reintento lo repara.
		if err := s.confirmarInscripcionSiProcede(ctx, tx); err != nil {
			return nil, err
		}
		return tx, nil
	}
	if tx.Status == domain.TxStatusDeclined {
		return tx, nil
	}

	return s.VerifyPayment(ctx, tx.ID)
}

// transaccionDeLaNotificacion resuelve la referencia recibida, probando primero
// como id nuestro y después como id de CredibanCo.
func (s *paymentService) transaccionDeLaNotificacion(ctx context.Context, referencia string) (*domain.Transaction, error) {
	if id, err := uuid.Parse(referencia); err == nil {
		tx, err := s.txRepo.GetTransactionByID(ctx, id)
		if err == nil && tx != nil {
			return tx, nil
		}
		// Un uuid que no es transacción nuestra todavía puede ser un mdOrder:
		// se sigue probando por el otro camino antes de darlo por desconocido.
	}

	tx, err := s.txRepo.GetTransactionByOrderID(ctx, referencia)
	if err != nil || tx == nil {
		return nil, fmt.Errorf("%w: %s", ErrPaymentNotificationUnknown, referencia)
	}
	return tx, nil
}

func (s *paymentService) VerifyPayment(ctx context.Context, txID uuid.UUID) (*domain.Transaction, error) {
	tx, err := s.txRepo.GetTransactionByID(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("transaction not found: %w", err)
	}

	if tx.CredibancoOrderID == "" {
		return nil, fmt.Errorf("transaction does not have a credibanco order id")
	}

	// Always verify against the bank
	status, err := s.gateway.GetPaymentStatus(ctx, tx.CredibancoOrderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway status: %w", err)
	}

	if status != tx.Status {
		if err := s.txRepo.UpdateTransactionStatus(ctx, tx.ID, status, tx.CredibancoOrderID); err != nil {
			return nil, fmt.Errorf("failed to update transaction status: %w", err)
		}
		tx.Status = status
	}

	// Un pago aprobado tiene que confirmar la inscripción: hasta hoy esto no
	// existía y quien pagaba de verdad se quedaba sin inscribir, con la
	// transacción en APPROVED y ninguna fila en events.registrations que lo
	// reflejara.
	//
	// Se ejecuta siempre que el estado sea APPROVED, no solo cuando acaba de
	// cambiar: verificar dos veces debe dejar el mismo resultado, y el UPDATE es
	// idempotente. Si la primera verificación confirmó el pago pero falló al
	// tocar la inscripción, la segunda lo arregla.
	if err := s.confirmarInscripcionSiProcede(ctx, tx); err != nil {
		return nil, err
	}

	return tx, nil
}

// confirmarInscripcionSiProcede pasa la inscripción a confirmada cuando el pago
// está aprobado y la transacción es de un evento.
//
// Se ejecuta siempre que el estado sea APPROVED, no solo cuando acaba de
// cambiar: verificar dos veces debe dejar el mismo resultado, y el UPDATE es
// idempotente. Si la primera verificación confirmó el pago pero falló al tocar
// la inscripción, la siguiente lo arregla.
func (s *paymentService) confirmarInscripcionSiProcede(ctx context.Context, tx *domain.Transaction) error {
	if tx.Status != domain.TxStatusApproved || tx.ReferenceType != domain.RefTypeEvent || s.eventRepo == nil {
		return nil
	}

	// Se mira el estado ANTES de confirmar para saber si esta verificación es la
	// que cambia las cosas. `ConfirmEventRegistration` es idempotente y devuelve
	// una fila afectada aunque ya estuviera confirmada, así que sin esto el
	// correo saldría otra vez en cada verificación —y la app verifica al volver
	// de la pasarela, y el webhook también—.
	yaEstabaConfirmada := false
	if previa, errPrevia := s.eventRepo.GetRegistrationByUserAndEvent(ctx, tx.UserID.String(), tx.ReferenceID.String()); errPrevia == nil && previa != nil {
		yaEstabaConfirmada = previa.RegistrationStatus == domain.RegistrationConfirmed
	}

	err := s.eventRepo.ConfirmEventRegistration(ctx, tx.UserID.String(), tx.ReferenceID.String())
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("payment approved but registration could not be confirmed: %w", err)
	}

	if err == nil && !yaEstabaConfirmada {
		s.enviarCorreoDePago(tx)
	}
	// ErrNotFound significa que se pagó sin inscripción previa. No se inventa
	// una aquí: no hay forma de saber qué talleres eligió ni con qué datos, y
	// crear una a medias sería peor que dejar constancia. Queda el pago aprobado
	// y visible para reconciliar a mano.
	if errors.Is(err, domain.ErrNotFound) {
		log.Printf("[PAGO] transaccion %s aprobada sin inscripcion previa (usuario %s, evento %s): revisar a mano",
			tx.ID, tx.UserID, tx.ReferenceID)
	}
	return nil
}
