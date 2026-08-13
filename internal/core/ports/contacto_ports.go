package ports

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"time"
)

// ContactoService atiende el "Contáctenos" general de la app: un mensaje
// libre que se guarda y llega al buzón de soporte.
//
// No se confunde con los otros dos canales, que van a destinatarios distintos
// y siguen sin guardar nada:
//
//	AsesoriaService -> asesoria_email, y clasifica por categoría
//	BoardService    -> board_contacts, escribe a un miembro de junta concreto
type ContactoService interface {
	EnviarMensaje(ctx context.Context, userID, asunto, remitenteNombre, remitenteEmail, mensaje string) error
	Listar(ctx context.Context, estado string) ([]*domain.MensajeDeContacto, error)
	CambiarEstado(ctx context.Context, id, estado string) error
}

type ContactoRepository interface {
	Guardar(ctx context.Context, mensaje *domain.MensajeDeContacto) (string, error)
	// MarcarEnviado separa "se guardó" de "salió el correo": lo primero ocurre
	// siempre, lo segundo puede fallar.
	MarcarEnviado(ctx context.Context, id string) error
	Listar(ctx context.Context, estado string) ([]*domain.MensajeDeContacto, error)
	CambiarEstado(ctx context.Context, id, estado string) error
	// ContarDesde cuenta los mensajes de una persona a partir de un momento,
	// para el límite de frecuencia.
	ContarDesde(ctx context.Context, userID string, desde time.Time) (int, error)
}
