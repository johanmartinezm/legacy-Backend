package ports

import "context"

// ContactoService atiende el "Contáctenos" general de la app: un mensaje
// libre que llega al buzón de soporte.
//
// No se confunde con los otros dos canales que ya existían, que van a
// destinatarios distintos y por eso siguen separados:
//
//	AsesoriaService -> asesoria_email, y clasifica por categoría
//	BoardService    -> board_contacts, escribe a un miembro de junta concreto
type ContactoService interface {
	EnviarMensaje(ctx context.Context, asunto, remitenteNombre, remitenteEmail, mensaje string) error
}
