package email

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type GmailService struct {
	svc  *gmail.Service
	from string
}

func NewGmailService(credentialsFile, impersonateUser string) (*GmailService, error) {
	ctx := context.Background()

	b, err := ioutil.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("error leyendo json de credenciales: %v", err)
	}

	config, err := google.JWTConfigFromJSON(b, gmail.GmailSendScope)
	if err != nil {
		return nil, fmt.Errorf("error configurando jwt: %v", err)
	}
	config.Subject = impersonateUser

	client := config.Client(ctx)

	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("error inicializando cliente gmail: %v", err)
	}

	return &GmailService{
		svc:  srv,
		from: impersonateUser,
	}, nil
}

func (s *GmailService) sendMessage(to, subject, body string) error {
	message := []byte("To: " + to + "\r\n" +
		"From: " + s.from + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: text/html; charset=UTF-8\r\n\r\n" +
		body)

	msg := &gmail.Message{
		Raw: base64.URLEncoding.EncodeToString(message),
	}

	_, err := s.svc.Users.Messages.Send("me", msg).Do()
	return err
}

func (s *GmailService) SendResetPasswordEmail(to, resetURL string) error {
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

	return s.sendMessage(to, subject, body)
}

func (s *GmailService) SendBoardContactEmail(to, senderName, senderEmail, messageText string) error {
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

	return s.sendMessage(to, subject, body)
}

func (s *GmailService) SendAsesoriaEmail(to, senderName, senderEmail, category, messageText string) error {
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

	return s.sendMessage(to, subject, body)
}

func (s *GmailService) SendWelcomeEmail(to, userName string) error {
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

	return s.sendMessage(to, subject, body)
}

func (s *GmailService) SendVerificationEmail(to, verifyLink string) error {
	subject := "Verifica tu cuenta en Legacy Network"
	body := fmt.Sprintf(`
		<div style="font-family: Arial, sans-serif; max-width: 600px; border: 1px solid #eee; padding: 20px;">
			<h2 style="color: #162540;">Verificación de Correo Electrónico</h2>
			<p>¡Bienvenido a <strong>Legacy Network</strong>!</p>
			<p>Para activar tu cuenta y poder iniciar sesión en la aplicación móvil, por favor haz clic en el siguiente enlace para verificar tu correo:</p>
			<br>
			<a href="%s" style="display: inline-block; padding: 12px 24px; background-color: #162540; color: #ffffff; text-decoration: none; border-radius: 4px; font-weight: bold;">Verificar mi correo</a>
			<br><br>
			<p>Si el botón no funciona, copia y pega el siguiente enlace en tu navegador:</p>
			<p style="word-break: break-all; color: #555;">%s</p>
			<hr style="border: none; border-top: 1px solid #eee;">
			<p style="font-size: 12px; color: #777;">Legacy Network - Conectando líderes.</p>
		</div>
	`, verifyLink, verifyLink)

	return s.sendMessage(to, subject, body)
}
