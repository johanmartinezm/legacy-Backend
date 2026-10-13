package services

import (
	"applegacy/backend/internal/core/ports"
	"context"
)

// UsuarioPlusService decide quién es Legacy+ y lo consulta. El dato vive en
// core.users.es_plus (20261013_usuario_plus.sql) y solo lo cambia el panel.
type UsuarioPlusService struct {
	repo ports.UsuarioPlusRepository
}

func NewUsuarioPlusService(repo ports.UsuarioPlusRepository) *UsuarioPlusService {
	return &UsuarioPlusService{repo: repo}
}

// EsPlus se consulta en la base en cada petición protegida, no en el token:
// así quitar el plus desde el panel surte efecto al momento.
func (s *UsuarioPlusService) EsPlus(ctx context.Context, userID string) (bool, error) {
	return s.repo.EsPlus(ctx, userID)
}

// CambiarPlus devuelve domain.ErrNotFound si el usuario no existe.
func (s *UsuarioPlusService) CambiarPlus(ctx context.Context, userID string, esPlus bool) error {
	return s.repo.SetEsPlus(ctx, userID, esPlus)
}
