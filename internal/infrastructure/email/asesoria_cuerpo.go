package email

import (
	"fmt"
	"html"
)

// cuerpoCorreoAsesoria arma el HTML del aviso de una solicitud de asesoría.
// Lo comparten GmailService y EmailService (SMTP), que lo tenían duplicado.
//
// Todo lo que viene del usuario se escapa. Antes se insertaba tal cual, y
// desde el 2026-10-13 el formulario de la app incluye un detalle libre de
// hasta 500 palabras: un «<a href=…>» escrito ahí habría llegado como enlace
// real al buzón del equipo, con apariencia de correo del sistema.
func cuerpoCorreoAsesoria(senderName, senderEmail, category, messageText string) string {
	return fmt.Sprintf(`
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
	`, html.EscapeString(category), html.EscapeString(senderName), html.EscapeString(senderEmail), html.EscapeString(messageText))
}
