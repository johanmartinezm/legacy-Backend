package email

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/skip2/go-qrcode"
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

// sendMessageConImagen manda un correo con una imagen incrustada en el cuerpo.
//
// Va como `multipart/related` con la imagen referenciada por `cid:`. La
// alternativa evidente —incrustarla como `data:` en el `src` del `<img>`— **no
// sirve**: Gmail y Outlook descartan esas imágenes, así que el correo llegaría
// con un hueco justo donde está el código de acceso.
//
// El nombre del recurso lo fija quien llama y tiene que coincidir con el `cid:`
// que use en el HTML.
func (s *GmailService) sendMessageConImagen(to, subject, body, cid string, imagen []byte) error {
	// Separador fijo y suficientemente improbable: no puede aparecer dentro del
	// cuerpo o rompería el mensaje. El HTML lo generamos nosotros y el base64 de
	// la imagen solo tiene el alfabeto de base64.
	const frontera = "----legacy-frontera-9f2c1a7e"

	var b strings.Builder
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("From: " + s.from + "\r\n")
	b.WriteString("Subject: " + encodeHeader(subject) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/related; boundary=\"" + frontera + "\"\r\n\r\n")

	b.WriteString("--" + frontera + "\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	b.WriteString(body)
	b.WriteString("\r\n")

	b.WriteString("--" + frontera + "\r\n")
	b.WriteString("Content-Type: image/png\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n")
	b.WriteString("Content-ID: <" + cid + ">\r\n")
	b.WriteString("Content-Disposition: inline; filename=\"acceso.png\"\r\n\r\n")

	// En líneas de 76 caracteres: el estándar MIME lo pide y algunos servidores
	// rechazan o parten mal las líneas muy largas.
	codificada := base64.StdEncoding.EncodeToString(imagen)
	for i := 0; i < len(codificada); i += 76 {
		fin := i + 76
		if fin > len(codificada) {
			fin = len(codificada)
		}
		b.WriteString(codificada[i:fin] + "\r\n")
	}

	b.WriteString("--" + frontera + "--\r\n")

	msg := &gmail.Message{
		Raw: base64.URLEncoding.EncodeToString([]byte(b.String())),
	}
	_, err := s.svc.Users.Messages.Send("me", msg).Do()
	return err
}

func (s *GmailService) sendMessage(to, subject, body string) error {
	message := []byte("To: " + to + "\r\n" +
		"From: " + s.from + "\r\n" +
		"Subject: " + encodeHeader(subject) + "\r\n" +
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

// SendContactoEmail entrega al buzón de soporte un mensaje libre escrito desde
// la pantalla "Contáctenos". El asunto lleva el nombre de quien escribe para
// poder distinguirlo en la bandeja sin abrirlo.
func (s *GmailService) SendContactoEmail(to, asunto, senderName, senderEmail, messageText string) error {
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

// SendEventPaymentEmail confirma un pago aprobado y entrega el acceso.
//
// Lleva el código de acceso dibujado como QR dentro del propio correo. Es lo
// que hace cualquier venta de entradas —del avión al concierto— y evita que
// quien pagó dependa de abrir la app para entrar. El QR no es una credencial
// eterna: el check-in marca la asistencia, así que reutilizarlo no cuela dos
// veces por la puerta.
//
// En un evento virtual no hay QR: lo que se entrega es el enlace de la sesión.
func (s *GmailService) SendEventPaymentEmail(datos domain.CorreoPago) error {
	saludo := "Hola"
	if datos.Nombre != "" {
		saludo = fmt.Sprintf("Hola, %s", datos.Nombre)
	}

	moneda := datos.Moneda
	if moneda == "" {
		moneda = "COP"
	}

	detallePago := fmt.Sprintf(`
		<table style="border-collapse: collapse; margin: 8px 0 18px;">
			<tr><td style="padding: 4px 16px 4px 0; color: #555;">Evento</td><td style="padding: 4px 0;"><strong>%s</strong></td></tr>
			<tr><td style="padding: 4px 16px 4px 0; color: #555;">Fecha del evento</td><td style="padding: 4px 0;">%s</td></tr>
			<tr><td style="padding: 4px 16px 4px 0; color: #555;">Importe pagado</td><td style="padding: 4px 0;"><strong>%s %s</strong></td></tr>
			<tr><td style="padding: 4px 16px 4px 0; color: #555;">Referencia</td><td style="padding: 4px 0;">%s</td></tr>
			<tr><td style="padding: 4px 16px 4px 0; color: #555;">Fecha del pago</td><td style="padding: 4px 0;">%s</td></tr>
		</table>
	`, datos.Evento, datos.Fecha, formatearImporte(datos.Importe), moneda, datos.Referencia, datos.PagadoEl)

	// Evento virtual: no hay puerta que cruzar, así que se entrega el enlace.
	if datos.EsVirtual || datos.QRData == "" {
		acceso := `<p>Tu inscripción quedó confirmada. El acceso está en la app, en <strong>Mi credencial</strong>.</p>`
		if datos.EsVirtual && datos.EnlaceLugar != "" {
			acceso = fmt.Sprintf(`
				<p>Es una <strong>masterclass virtual en vivo</strong>. Entra desde aquí el día de la sesión:</p>
				<a href="%s" style="display: inline-block; padding: 12px 24px; background-color: #162540; color: #ffffff; text-decoration: none; border-radius: 4px; font-weight: bold;">Entrar a la sesión</a>
				<br><br>
				<p style="word-break: break-all; color: #555;">%s</p>
			`, datos.EnlaceLugar, datos.EnlaceLugar)
		}
		return s.sendMessage(datos.Para, fmt.Sprintf("Pago confirmado: %s", datos.Evento),
			cuerpoPago(saludo, detallePago, acceso))
	}

	png, err := qrcode.Encode(datos.QRData, qrcode.Medium, 320)
	if err != nil {
		// Sin QR el correo sigue valiendo: lleva la constancia del pago y remite
		// a la app. Vale más eso que no mandar nada porque falló el dibujo.
		acceso := `<p>Tu código de acceso está en la app, en <strong>Mi credencial</strong>. Preséntalo en la entrada.</p>`
		return s.sendMessage(datos.Para, fmt.Sprintf("Pago confirmado: %s", datos.Evento),
			cuerpoPago(saludo, detallePago, acceso))
	}

	const cid = "acceso-qr"
	acceso := fmt.Sprintf(`
		<p><strong>Este es tu código de acceso.</strong> Preséntalo en la entrada del evento:</p>
		<img src="cid:%s" alt="Código de acceso" width="320" height="320" style="display: block; margin: 12px 0;">
		<p style="color: #777; font-size: 12px;">Si no ves la imagen, el mismo código está en la app, en <strong>Mi credencial</strong>.</p>
	`, cid)

	return s.sendMessageConImagen(datos.Para, fmt.Sprintf("Pago confirmado: %s", datos.Evento),
		cuerpoPago(saludo, detallePago, acceso), cid, png)
}

func cuerpoPago(saludo, detallePago, acceso string) string {
	return fmt.Sprintf(`
		<div style="font-family: Arial, sans-serif; max-width: 600px; border: 1px solid #eee; padding: 20px;">
			<h2 style="color: #162540;">Pago confirmado</h2>
			<p>%s,</p>
			<p>Recibimos tu pago y tu inscripción quedó confirmada.</p>
			%s
			%s
			<hr style="border: none; border-top: 1px solid #eee;">
			<p style="font-size: 12px; color: #777;">Legacy Network - Conectando líderes.</p>
		</div>
	`, saludo, detallePago, acceso)
}

// formatearImporte escribe el importe con separador de miles y sin decimales
// cuando no los tiene, que es como se escriben los pesos.
func formatearImporte(v float64) string {
	entero := int64(v)
	if v != float64(entero) {
		return fmt.Sprintf("%.2f", v)
	}

	s := fmt.Sprintf("%d", entero)
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, c)
	}
	return string(out)
}

// SendEventCredentialEmail entrega la credencial a quien ya está inscrito.
//
// Lo nuevo frente al correo de pago no es el QR —ese ya se dibujaba— sino el
// bloque de acceso: en una carga masiva la contraseña es el número de documento
// y **este correo es el único sitio donde la persona se entera**. Si no viene
// relleno, no se pinta: una cuenta que ya existía tiene su propia contraseña.
func (s *GmailService) SendEventCredentialEmail(datos domain.CorreoCredencial) error {
	saludo := "Hola"
	if datos.Nombre != "" {
		saludo = fmt.Sprintf("Hola, %s", datos.Nombre)
	}

	asunto := fmt.Sprintf("Tu acceso a %s", datos.Evento)

	// Evento virtual: no hay puerta, así que lo que se entrega es el enlace. La
	// rama existe por si alguien llega aquí con un evento virtual; el servicio
	// ya lo rechaza antes, y el panel enseña el interruptor deshabilitado.
	if datos.EsVirtual || datos.QRData == "" {
		acceso := `<p>Tu inscripción está confirmada. El acceso está en la app, en <strong>Mi credencial</strong>.</p>`
		if datos.EsVirtual && datos.EnlaceLugar != "" {
			acceso = fmt.Sprintf(`
				<p>Es una <strong>masterclass virtual en vivo</strong>. Entra desde aquí el día de la sesión:</p>
				<a href="%s" style="display: inline-block; padding: 12px 24px; background-color: #162540; color: #ffffff; text-decoration: none; border-radius: 4px; font-weight: bold;">Entrar a la sesión</a>
				<br><br>
				<p style="word-break: break-all; color: #555;">%s</p>
			`, datos.EnlaceLugar, datos.EnlaceLugar)
		}
		return s.sendMessage(datos.Para, asunto,
			cuerpoCredencial(saludo, datos, acceso))
	}

	png, err := qrcode.Encode(datos.QRData, qrcode.Medium, 320)
	if err != nil {
		// Salida airosa, la misma del correo de pago: sin QR el correo sigue
		// valiendo —lleva las credenciales de acceso, que es lo que no está en
		// ningún otro sitio— y remite a la app para el código.
		acceso := `<p>Tu código de acceso está en la app, en <strong>Mi credencial</strong>. Preséntalo en la entrada.</p>`
		return s.sendMessage(datos.Para, asunto,
			cuerpoCredencial(saludo, datos, acceso))
	}

	const cid = "acceso-qr"
	acceso := fmt.Sprintf(`
		<p><strong>Este es tu código de acceso.</strong> Preséntalo en la entrada del evento:</p>
		<img src="cid:%s" alt="Código de acceso" width="320" height="320" style="display: block; margin: 12px 0;">
		<p style="color: #777; font-size: 12px;">Si no ves la imagen, el mismo código está en la app, en <strong>Mi credencial</strong>.</p>
	`, cid)

	return s.sendMessageConImagen(datos.Para, asunto,
		cuerpoCredencial(saludo, datos, acceso), cid, png)
}

// cuerpoCredencial arma el correo de la credencial. El bloque de acceso a la app
// va **antes** del QR a propósito: sin poder entrar, el QR no le sirve de nada a
// quien acaba de ser importado.
func cuerpoCredencial(saludo string, datos domain.CorreoCredencial, acceso string) string {
	var credenciales string
	if datos.Usuario != "" && datos.Contrasena != "" {
		credenciales = fmt.Sprintf(`
			<div style="background-color: #f5f7fa; border-left: 4px solid #162540; padding: 12px 16px; margin: 18px 0;">
				<p style="margin: 0 0 8px;"><strong>Tu cuenta en la app Legacy Network</strong></p>
				<table style="border-collapse: collapse;">
					<tr><td style="padding: 4px 16px 4px 0; color: #555;">Usuario</td><td style="padding: 4px 0;"><strong>%s</strong></td></tr>
					<tr><td style="padding: 4px 16px 4px 0; color: #555;">Contraseña</td><td style="padding: 4px 0;"><strong>%s</strong></td></tr>
				</table>
				<p style="margin: 8px 0 0; color: #777; font-size: 12px;">
					Es tu número de documento. La app te pedirá cambiarla la primera vez que entres.
				</p>
			</div>
		`, datos.Usuario, datos.Contrasena)
	}

	return fmt.Sprintf(`
		<div style="font-family: Arial, sans-serif; max-width: 600px; border: 1px solid #eee; padding: 20px;">
			<h2 style="color: #162540;">Tu acceso a %s</h2>
			<p>%s,</p>
			<p>Tu inscripción a <strong>%s</strong> está confirmada.</p>
			<p><strong>Fecha:</strong> %s</p>
			%s
			%s
			<hr style="border: none; border-top: 1px solid #eee;">
			<p style="font-size: 12px; color: #777;">Legacy Network - Conectando líderes.</p>
		</div>
	`, datos.Evento, saludo, datos.Evento, datos.Fecha, credenciales, acceso)
}

// SendEventRegistrationEmail confirma la inscripción a un evento.
//
// El bloque de acceso cambia con la modalidad: el virtual lleva el botón con el
// enlace de la sesión —que es lo que el cliente echaba en falta—; el presencial
// remite a "Mi credencial" en la app, donde está el QR. Nunca se manda el QR por
// correo: es lo que da derecho a entrar y un buzón reenviado lo repartiría.
func (s *GmailService) SendEventRegistrationEmail(datos domain.CorreoInscripcion) error {
	saludo := "Hola"
	if datos.Nombre != "" {
		saludo = fmt.Sprintf("Hola, %s", datos.Nombre)
	}

	var acceso string
	switch {
	case datos.EsVirtual && datos.EnlaceLugar != "":
		acceso = fmt.Sprintf(`
			<p>Es una <strong>masterclass virtual en vivo</strong>. Entra desde aquí el día de la sesión:</p>
			<a href="%s" style="display: inline-block; padding: 12px 24px; background-color: #162540; color: #ffffff; text-decoration: none; border-radius: 4px; font-weight: bold;">Entrar a la sesión</a>
			<br><br>
			<p>Si el botón no funciona, copia y pega este enlace en tu navegador:</p>
			<p style="word-break: break-all; color: #555;">%s</p>
		`, datos.EnlaceLugar, datos.EnlaceLugar)
	case datos.EsVirtual:
		// Virtual sin enlace cargado todavía en el panel. Se avisa en vez de
		// dejar el correo sin decir cómo entrar.
		acceso = `<p>Es una <strong>masterclass virtual en vivo</strong>. Te enviaremos el enlace de acceso antes de la sesión.</p>`
	case datos.EnlaceLugar != "":
		acceso = fmt.Sprintf(`
			<p>Es un <strong>evento presencial</strong> en %s.</p>
			<p>Tu código de acceso está en la app, en <strong>Mi credencial</strong>. Preséntalo en la entrada.</p>
		`, datos.EnlaceLugar)
	default:
		acceso = `<p>Tu código de acceso está en la app, en <strong>Mi credencial</strong>. Preséntalo en la entrada.</p>`
	}

	subject := fmt.Sprintf("Inscripción confirmada: %s", datos.Evento)
	body := fmt.Sprintf(`
		<div style="font-family: Arial, sans-serif; max-width: 600px; border: 1px solid #eee; padding: 20px;">
			<h2 style="color: #162540;">Inscripción confirmada</h2>
			<p>%s,</p>
			<p>Tu inscripción a <strong>%s</strong> quedó confirmada.</p>
			<p><strong>Fecha:</strong> %s</p>
			<br>
			%s
			<hr style="border: none; border-top: 1px solid #eee;">
			<p style="font-size: 12px; color: #777;">Legacy Network - Conectando líderes.</p>
		</div>
	`, saludo, datos.Evento, datos.Fecha, acceso)

	return s.sendMessage(datos.Para, subject, body)
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
