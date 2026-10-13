package ports

import (
	"applegacy/backend/internal/core/domain"
	"context"
)

type AsesoriaService interface {
	RequestAsesoria(ctx context.Context, category, senderName, senderEmail, message string) error
}

// AsesoriaUnidadRepository y AsesoriaUnidadService: las unidades de la
// pantalla Asesoría, administrables desde el panel (20261013_asesoria_unidades.sql).
type AsesoriaUnidadRepository interface {
	List(ctx context.Context, soloActivas bool) ([]*domain.AsesoriaUnidad, error)
	Create(ctx context.Context, u *domain.AsesoriaUnidad) error
	Update(ctx context.Context, u *domain.AsesoriaUnidad) error
	Delete(ctx context.Context, id string) error
}

type AsesoriaUnidadService interface {
	ListActivas(ctx context.Context) ([]*domain.AsesoriaUnidad, error)
	ListTodas(ctx context.Context) ([]*domain.AsesoriaUnidad, error)
	Crear(ctx context.Context, u *domain.AsesoriaUnidad) error
	Actualizar(ctx context.Context, u *domain.AsesoriaUnidad) error
	Borrar(ctx context.Context, id string) error
}
