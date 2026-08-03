package ports

import (
	"context"
)

type AsesoriaService interface {
	RequestAsesoria(ctx context.Context, category, senderName, senderEmail, message string) error
}
