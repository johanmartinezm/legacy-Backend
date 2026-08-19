package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
	"log"
	"strings"
	"time"
)

// Correo de confirmación cuando la pasarela aprueba el cobro de un evento.
//
// Hasta el 2026-08-18 el flujo de pago no enviaba nada: `VerifyPayment` confirmaba
// la inscripción y ahí terminaba. Quien pagaba de verdad no recibía constancia
// del cobro ni forma de entrar al evento sin abrir la app.
//
// Es distinto del correo de inscripción gratuita (`event_correo.go`): este lleva
// además lo que se pagó y **el código de acceso dibujado como QR dentro del
// propio correo**.

// tiempoMaximoCorreoPago es más generoso que el de la inscripción gratuita: este
// correo dibuja un QR y viaja con la imagen dentro, así que pesa más.
const tiempoMaximoCorreoPago = 45 * time.Second

// ConCorreoDePago habilita el correo de pago aprobado sobre un servicio ya
// construido, y devuelve el mismo para poder encadenarlo en main.go.
//
// Es una función del paquete y no un método porque `NewPaymentService` devuelve
// la interfaz `ports.PaymentService`, y añadir ahí un método de cableado
// ensuciaría el puerto con algo que no es parte del contrato.
//
// Va aparte del constructor para no tocar las llamadas existentes ni obligar a
// los tests de pagos —que son varios y no lo usan— a inyectar tres dependencias
// más. Con un servicio de otro tipo no hace nada: el correo simplemente no sale.
func ConCorreoDePago(svc ports.PaymentService, users ports.UserRepository, email ports.EmailService, crypto ports.CryptoService) ports.PaymentService {
	if ps, ok := svc.(*paymentService); ok {
		ps.users = users
		ps.email = email
		ps.crypto = crypto
	}
	return svc
}

// enviarCorreoDePago arma y manda la confirmación **sin bloquear ni fallar
// nunca** la verificación del pago.
//
// El cobro ya está aprobado en la pasarela y la inscripción confirmada en la
// base: un problema con el correo no puede deshacer eso ni devolverle un error a
// quien acaba de pagar. Lleva contexto propio porque el de la petición HTTP se
// cancela al responder y cortaría el envío a medias.
func (s *paymentService) enviarCorreoDePago(tx *domain.Transaction) {
	if s.email == nil || s.users == nil || s.eventRepo == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), tiempoMaximoCorreoPago)
		defer cancel()

		datos, err := s.armarCorreoDePago(ctx, tx)
		if err != nil {
			log.Printf("[correo pago] transaccion %s: %v", tx.ID, err)
			return
		}
		if datos.Para == "" {
			log.Printf("[correo pago] transaccion %s: sin correo al que enviar", tx.ID)
			return
		}

		if err := s.email.SendEventPaymentEmail(*datos); err != nil {
			log.Printf("[correo pago] transaccion %s: %v", tx.ID, err)
		}
	}()
}

func (s *paymentService) armarCorreoDePago(ctx context.Context, tx *domain.Transaction) (*domain.CorreoPago, error) {
	evento, err := s.eventRepo.GetEventByID(ctx, tx.ReferenceID.String())
	if err != nil {
		return nil, err
	}

	datos := &domain.CorreoPago{
		Evento:     evento.Title,
		Fecha:      evento.StartDate.Format("02/01/2006"),
		Importe:    tx.Amount,
		Moneda:     "COP",
		Referencia: tx.CredibancoOrderID,
		Metodo:     tx.PaymentMethod,
		PagadoEl:   tx.UpdatedAt.Format("02/01/2006 15:04"),
		EsVirtual:  evento.IsVirtual,
	}

	if evento.IsVirtual {
		if evento.AccessURL != nil {
			datos.EnlaceLugar = strings.TrimSpace(*evento.AccessURL)
		}
	} else if evento.Location != nil {
		datos.EnlaceLugar = strings.TrimSpace(*evento.Location)
	}

	// El código de acceso solo se manda en los presenciales: en un evento
	// virtual el QR no abre ninguna puerta y lo que hace falta es el enlace.
	registro, err := s.eventRepo.GetRegistrationByUserAndEvent(ctx, tx.UserID.String(), tx.ReferenceID.String())
	if err == nil && registro != nil && !evento.IsVirtual {
		datos.QRData = registro.QRData
	}

	// Destinatario y nombre: primero el contacto escrito en la inscripción, que
	// puede ser distinto al de la cuenta; si no, el del perfil. Los dos llegan
	// cifrados.
	if registro != nil && s.crypto != nil {
		if claro, e := s.crypto.Decrypt(registro.ParticipantEmail); e == nil && claro != "" {
			datos.Para = claro
		}
		if claro, e := s.crypto.Decrypt(registro.ParticipantName); e == nil && claro != "" {
			datos.Nombre = claro
		}
	}

	if datos.Para == "" || datos.Nombre == "" {
		if user, e := s.users.FindByID(ctx, tx.UserID.String()); e == nil && user != nil {
			if datos.Para == "" {
				datos.Para = s.descifrar(user.EmailEncrypted)
			}
			if datos.Nombre == "" {
				datos.Nombre = s.descifrar(user.FirstName)
			}
		}
	}

	return datos, nil
}

// descifrar abre un valor cifrado y devuelve cadena vacía si no se puede. Sin
// CryptoService devuelve el valor tal cual: las filas anteriores al cifrado
// están en claro y perderlas sería peor que mostrarlas.
func (s *paymentService) descifrar(valor string) string {
	if valor == "" {
		return ""
	}
	if s.crypto == nil {
		return valor
	}
	claro, err := s.crypto.Decrypt(valor)
	if err != nil {
		return ""
	}
	return claro
}
