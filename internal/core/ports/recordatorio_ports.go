package ports

import (
	"applegacy/backend/internal/core/domain"
	"context"
)

// RecordatorioEventoRepository es lo que necesita el aviso de la mañana del
// evento (services/recordatorio_eventos.go).
type RecordatorioEventoRepository interface {
	// EventosSinRecordar devuelve los eventos activos que empiezan en `dia`
	// (AAAA-MM-DD) y todavía no tienen recordatorio para ese día.
	EventosSinRecordar(ctx context.Context, dia string) ([]*domain.EventoPorRecordar, error)

	// MarcarRecordado reserva el recordatorio de un evento para `dia`. Devuelve
	// false si ya estaba reservado: es lo que impide avisar dos veces.
	MarcarRecordado(ctx context.Context, eventID, dia string) (bool, error)

	// InscritosConfirmados devuelve, sin repetir, las personas con inscripción
	// confirmada en el evento.
	InscritosConfirmados(ctx context.Context, eventID string) ([]string, error)
}
