package email

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"log"
)

// Servicio es todo lo que main.go usa del correo: ports.EmailService más los
// avisos de Legacy Board, que van aparte para no obligar a cada implementación
// falsa de las pruebas a implementarlos.
type Servicio interface {
	ports.EmailService
	SendBoardEnvioNuevoEmail(to, titulo, nombre, correo string) error
	SendBoardEstadoEmail(to, nombre, titulo, estado string) error
}

// Simulado no envía nada: escribe en el registro a quién y qué habría mandado.
// Se activa con LEGACY_CORREO_SIMULADO=1, para probar en local los flujos que
// escriben a alguien sin que salga un correo de verdad desde la cuenta del
// proyecto. Es lo mismo que hace FCM cuando falta su credencial.
type Simulado struct{}

var _ Servicio = Simulado{}

func simular(que, para string) error {
	log.Printf("[CORREO SIMULADO] %s → %s (no se envió)", que, para)
	return nil
}

func (Simulado) SendResetPasswordEmail(to, resetURL string) error {
	return simular("restablecer contraseña", to)
}
func (Simulado) SendBoardContactEmail(to, senderName, senderEmail, messageText string) error {
	return simular("contacto Legacy Board", to)
}
func (Simulado) SendAsesoriaEmail(to, senderName, senderEmail, category, messageText string) error {
	return simular("asesoría", to)
}
func (Simulado) SendContactoEmail(to, asunto, senderName, senderEmail, messageText string) error {
	return simular("Contáctenos", to)
}
func (Simulado) SendWelcomeEmail(to, userName string) error { return simular("bienvenida", to) }
func (Simulado) SendVerificationEmail(to, link string) error {
	return simular("verificación de correo", to)
}
func (Simulado) SendEventRegistrationEmail(datos domain.CorreoInscripcion) error {
	return simular("inscripción a evento", "(destinatario de la inscripción)")
}
func (Simulado) SendEventPaymentEmail(datos domain.CorreoPago) error {
	return simular("pago de evento", "(destinatario del pago)")
}
func (Simulado) SendEventCredentialEmail(datos domain.CorreoCredencial) error {
	return simular("credencial de evento", "(destinatario de la credencial)")
}
func (Simulado) SendBoardEnvioNuevoEmail(to, titulo, nombre, correo string) error {
	return simular("Legacy Board: nueva "+titulo+" de "+nombre, to)
}
func (Simulado) SendBoardEstadoEmail(to, nombre, titulo, estado string) error {
	return simular("Legacy Board: "+titulo+" "+estado, to)
}
