package email

import (
	"fmt"
	"net/smtp"
)

type EmailService struct {
	host     string
	port     int
	username string
	password string
	from     string
}

func NewEmailService(host string, port int, username, password, from string) *EmailService {
	return &EmailService{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
	}
}

func (s *EmailService) SendResetPasswordEmail(to, resetURL string) error {
	subject := "Restablecer tu contraseña - Legacy App"
	body := fmt.Sprintf(`
		<h1>Restablecer tu contraseña</h1>
		<p>Hola,</p>
		<p>Has solicitado restablecer tu contraseña en Legacy App. Haz clic en el siguiente enlace para continuar:</p>
		<p><a href="%s">%s</a></p>
		<p>Si no solicitaste este cambio, puedes ignorar este correo.</p>
		<p>Este enlace expirará en 1 hora.</p>
		<p>Atentamente,<br>El equipo de Legacy App</p>
	`, resetURL, resetURL)

	message := []byte(fmt.Sprintf("To: %s\r\n"+
		"Subject: %s\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n"+
		"%s\r\n", to, encodeHeader(subject), body))

	auth := smtp.PlainAuth("", s.username, s.password, s.host)
	addr := fmt.Sprintf("%s:%d", s.host, s.port)

	return smtp.SendMail(addr, auth, s.from, []string{to}, message)
}

func (s *EmailService) SendBoardContactEmail(to, senderName, senderEmail, messageText string) error {
	subject := fmt.Sprintf("Nuevo mensaje para el Board de %s", senderName)
	body := fmt.Sprintf(`
		<div style="font-family: Arial, sans-serif; max-width: 600px; border: 1px solid #eee; padding: 20px;">
			<h2 style="color: #162540;">Nueva Solicitud de Legacy Board</h2>
			<p>Hola,</p>
			<p>Has recibido un nuevo mensaje de contacto a través de la plataforma Legacy App:</p>
			<div style="background-color: #f5f7f9; padding: 20px; border-left: 4px solid #306c9e; margin: 20px 0;">
				<p><strong>Remitente:</strong> %s &lt;%s&gt;</p>
				<p><strong>Mensaje:</strong></p>
				<p style="white-space: pre-wrap;">%s</p>
			</div>
			<p>Este es un correo automático generado por el sistema.</p>
			<hr style="border: none; border-top: 1px solid #eee;">
			<p style="font-size: 12px; color: #777;">Legacy Network - Connectando líderes.</p>
		</div>
	`, senderName, senderEmail, messageText)

	message := []byte(fmt.Sprintf("To: %s\r\n"+
		"Subject: %s\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n"+
		"%s\r\n", to, encodeHeader(subject), body))

	auth := smtp.PlainAuth("", s.username, s.password, s.host)
	addr := fmt.Sprintf("%s:%d", s.host, s.port)

	return smtp.SendMail(addr, auth, s.from, []string{to}, message)
}

func (s *EmailService) SendAsesoriaEmail(to, senderName, senderEmail, category, messageText string) error {
	subject := fmt.Sprintf("Nueva Solicitud de Asesoría (%s) - %s", category, senderName)
	body := fmt.Sprintf(`
		<div style="font-family: Arial, sans-serif; max-width: 600px; border: 1px solid #eee; padding: 20px;">
			<h2 style="color: #c98a1a;">Nueva Solicitud de Asesoría</h2>
			<p>Hola,</p>
			<p>Has recibido una nueva solicitud de asesoría a través de la plataforma Legacy App:</p>
			<div style="background-color: #fcf9f2; padding: 20px; border-left: 4px solid #c98a1a; margin: 20px 0;">
				<p><strong>Categoría:</strong> %s</p>
				<p><strong>Remitente:</strong> %s &lt;%s&gt;</p>
				<p><strong>Mensaje Adicional:</strong></p>
				<p style="white-space: pre-wrap;">%s</p>
			</div>
			<p>Este es un correo automático generado por el sistema.</p>
			<hr style="border: none; border-top: 1px solid #eee;">
			<p style="font-size: 12px; color: #777;">Legacy Network - Connectando líderes.</p>
		</div>
	`, category, senderName, senderEmail, messageText)

	message := []byte(fmt.Sprintf("To: %s\r\n"+
		"Subject: %s\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n"+
		"%s\r\n", to, encodeHeader(subject), body))

	auth := smtp.PlainAuth("", s.username, s.password, s.host)
	addr := fmt.Sprintf("%s:%d", s.host, s.port)

	return smtp.SendMail(addr, auth, s.from, []string{to}, message)
}

// SendContactoEmail entrega al buzón de soporte un mensaje libre escrito desde
// la pantalla "Contáctenos". El asunto lleva el nombre de quien escribe para
// poder distinguirlo en la bandeja sin abrirlo.
func (s *EmailService) SendContactoEmail(to, asunto, senderName, senderEmail, messageText string) error {
	subject := fmt.Sprintf("Contacto desde la app: %s - %s", asunto, senderName)
	body := fmt.Sprintf(`
		<div style="font-family: Arial, sans-serif; max-width: 600px; border: 1px solid #eee; padding: 20px;">
			<h2 style="color: #c98a1a;">Nuevo mensaje desde la app</h2>
			<p>Hola,</p>
			<p>Un usuario de Legacy App ha escrito desde la pantalla de Contáctenos:</p>
			<div style="background-color: #fcf9f2; padding: 20px; border-left: 4px solid #c98a1a; margin: 20px 0;">
				<p><strong>Asunto:</strong> %s</p>
				<p><strong>Remitente:</strong> %s &lt;%s&gt;</p>
				<p><strong>Mensaje:</strong></p>
				<p style="white-space: pre-wrap;">%s</p>
			</div>
			<p>Puedes responder directamente a esta persona en el correo de arriba.</p>
			<hr style="border: none; border-top: 1px solid #eee;">
			<p style="font-size: 12px; color: #777;">Legacy Network - Connectando líderes.</p>
		</div>
	`, asunto, senderName, senderEmail, messageText)

	message := []byte(fmt.Sprintf("To: %s\r\n"+
		"Subject: %s\r\n"+
		"Reply-To: %s\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n"+
		"%s\r\n", to, encodeHeader(subject), senderEmail, body))

	auth := smtp.PlainAuth("", s.username, s.password, s.host)
	addr := fmt.Sprintf("%s:%d", s.host, s.port)

	return smtp.SendMail(addr, auth, s.from, []string{to}, message)
}

func (s *EmailService) SendWelcomeEmail(to, userName string) error {
	subject := "¡Bienvenido a Legacy Network!"
	body := fmt.Sprintf(`
		<div style="font-family: Arial, sans-serif; max-width: 600px; border: 1px solid #eee; padding: 20px;">
			<h2 style="color: #162540;">¡Bienvenido, %s!</h2>
			<p>Nos emociona darte la bienvenida a <strong>Legacy Network</strong>, la red de líderes y empresas de familia.</p>
			<p>Tu cuenta ha sido creada exitosamente. Ya puedes iniciar sesión en la aplicación móvil para conectar con otros miembros, acceder a contenido exclusivo y gestionar tu perfil.</p>
			<br>
			<p>Si tienes alguna pregunta o necesitas ayuda, no dudes en contactarnos.</p>
			<hr style="border: none; border-top: 1px solid #eee;">
			<p style="font-size: 12px; color: #777;">Legacy Network - Conectando líderes.</p>
		</div>
	`, userName)

	message := []byte(fmt.Sprintf("To: %s\r\n"+
		"Subject: %s\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n"+
		"%s\r\n", to, encodeHeader(subject), body))

	auth := smtp.PlainAuth("", s.username, s.password, s.host)
	addr := fmt.Sprintf("%s:%d", s.host, s.port)

	return smtp.SendMail(addr, auth, s.from, []string{to}, message)
}
