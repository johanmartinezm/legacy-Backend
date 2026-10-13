package ports

import (
	"applegacy/backend/internal/core/domain"
	"context"
)

// BeneficioRepository y BeneficioService: el slider de la sección Beneficios
// de la app (20261013_beneficios.sql).
type BeneficioRepository interface {
	List(ctx context.Context, soloActivos bool) ([]*domain.Beneficio, error)
	Create(ctx context.Context, b *domain.Beneficio) error
	Update(ctx context.Context, b *domain.Beneficio) error
	Delete(ctx context.Context, id string) error
}

type BeneficioService interface {
	ListActivos(ctx context.Context) ([]*domain.Beneficio, error)
	ListTodos(ctx context.Context) ([]*domain.Beneficio, error)
	Crear(ctx context.Context, b *domain.Beneficio) error
	Actualizar(ctx context.Context, b *domain.Beneficio) error
	Borrar(ctx context.Context, id string) error
}
