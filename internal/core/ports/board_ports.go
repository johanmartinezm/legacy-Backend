package ports

import (
	"context"
)

type BoardService interface {
	NotifyContact(ctx context.Context, contactID, senderName, senderEmail, message string) error
}
