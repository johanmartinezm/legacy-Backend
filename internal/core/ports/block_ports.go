package ports

import (
	"applegacy/backend/internal/core/domain"
	"context"
)

type BlockRepository interface {
	Block(ctx context.Context, blockerID, blockedID string) error
	Unblock(ctx context.Context, blockerID, blockedID string) error

	// ListBlocked devuelve a quién ha bloqueado esta persona, que es lo que se
	// muestra para poder desbloquear. No devuelve quién la ha bloqueado a ella:
	// saberlo permitiría deducir un bloqueo y buscar otra vía de contacto.
	ListBlocked(ctx context.Context, blockerID string) ([]*domain.BlockedUser, error)

	// AreBlocked responde si hay bloqueo entre dos personas en CUALQUIER
	// dirección. Es la pregunta que hacen los guardas antes de dejar escribir o
	// invitar.
	AreBlocked(ctx context.Context, userA, userB string) (bool, error)

	Report(ctx context.Context, report *domain.UserReport) error
	ListReports(ctx context.Context, status string) ([]*domain.UserReport, error)
	UpdateReportStatus(ctx context.Context, reportID, status string) error
}

type BlockService interface {
	BlockUser(ctx context.Context, blockerID, blockedID string) error
	UnblockUser(ctx context.Context, blockerID, blockedID string) error
	ListBlocked(ctx context.Context, blockerID string) ([]*domain.BlockedUser, error)
	ReportUser(ctx context.Context, reporterID, reportedID, reason string, messageID *string) error
	ListReports(ctx context.Context, status string) ([]*domain.UserReport, error)
	ResolveReport(ctx context.Context, reportID, status string) error
}
