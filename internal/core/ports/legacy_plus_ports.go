package ports

import (
	"applegacy/backend/internal/core/domain"
	"context"
)

// UsuarioPlusRepository y UsuarioPlusService: si un usuario es Legacy+
// (core.users.es_plus). Interfaz aparte de UserRepository para no obligar a
// los dobles de prueba que ya lo implementan.
type UsuarioPlusRepository interface {
	EsPlus(ctx context.Context, userID string) (bool, error)
	SetEsPlus(ctx context.Context, userID string, esPlus bool) error
}

type UsuarioPlusService interface {
	EsPlus(ctx context.Context, userID string) (bool, error)
	CambiarPlus(ctx context.Context, userID string, esPlus bool) error
}

// LegacyPlusItemRepository y LegacyPlusItemService: los puntos con check de la
// pantalla Legacy+, administrables desde el panel (20261013_legacy_plus_contenido.sql).
type LegacyPlusItemRepository interface {
	List(ctx context.Context, soloActivos bool) ([]*domain.LegacyPlusItem, error)
	Create(ctx context.Context, i *domain.LegacyPlusItem) error
	Update(ctx context.Context, i *domain.LegacyPlusItem) error
	Delete(ctx context.Context, id string) error
}

type LegacyPlusItemService interface {
	ListActivos(ctx context.Context) ([]*domain.LegacyPlusItem, error)
	ListTodos(ctx context.Context) ([]*domain.LegacyPlusItem, error)
	Crear(ctx context.Context, i *domain.LegacyPlusItem) error
	Actualizar(ctx context.Context, i *domain.LegacyPlusItem) error
	Borrar(ctx context.Context, id string) error
}
