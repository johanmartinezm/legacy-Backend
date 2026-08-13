package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"
	"testing"
)

// repoUsuarios registra a quién se pidió anonimizar. Lo que importa de este
// flujo no es el resultado que devuelve, sino sobre QUÉ cuenta actúa: si
// pudiera actuar sobre otra, cualquiera podría borrar la cuenta ajena.
type repoUsuarios struct {
	anonimizados []string
	err          error
}

func (r *repoUsuarios) AnonymizeUser(ctx context.Context, id string) error {
	r.anonimizados = append(r.anonimizados, id)
	return r.err
}

// El resto de la interfaz no interviene en este flujo.
func (r *repoUsuarios) Create(ctx context.Context, u *domain.User) error { return nil }
func (r *repoUsuarios) FindByEmailBlindIndex(ctx context.Context, b string) (*domain.User, error) {
	return nil, nil
}
func (r *repoUsuarios) FindAll(ctx context.Context) ([]*domain.User, error) { return nil, nil }
func (r *repoUsuarios) FindBySocialID(ctx context.Context, provider, socialID string) (*domain.User, error) {
	return nil, errors.New("user not found")
}
func (r *repoUsuarios) LinkSocialID(ctx context.Context, userID, provider, socialID string) error {
	return nil
}
func (r *repoUsuarios) FindByID(ctx context.Context, id string) (*domain.User, error) {
	return nil, nil
}
func (r *repoUsuarios) Update(ctx context.Context, u *domain.User) error             { return nil }
func (r *repoUsuarios) Delete(ctx context.Context, id string) error                  { return nil }
func (r *repoUsuarios) UpdatePassword(ctx context.Context, id, h string) error       { return nil }
func (r *repoUsuarios) UpdatePasswordByEmail(ctx context.Context, e, h string) error { return nil }
func (r *repoUsuarios) MarkEmailAsVerified(ctx context.Context, b string) error      { return nil }

func servicioDeCuentas(err error) (*AuthService, *repoUsuarios) {
	repo := &repoUsuarios{err: err}
	return &AuthService{repo: repo}, repo
}

func TestDeleteMyAccount_AnonimizaSoloAQuienLoPide(t *testing.T) {
	svc, repo := servicioDeCuentas(nil)

	if err := svc.DeleteMyAccount(context.Background(), "user-1"); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if len(repo.anonimizados) != 1 || repo.anonimizados[0] != "user-1" {
		t.Errorf("debe anonimizarse exactamente la cuenta que lo pide, se pidió: %v", repo.anonimizados)
	}
}

func TestDeleteMyAccount_SinUsuarioNoHaceNada(t *testing.T) {
	// Si el identificador llegara vacío —por ejemplo, un token sin `sub`—, lo
	// peor sería lanzar un UPDATE sin condición fiable. Se corta antes.
	svc, repo := servicioDeCuentas(nil)

	if err := svc.DeleteMyAccount(context.Background(), ""); err == nil {
		t.Error("se esperaba error con el usuario vacío")
	}
	if len(repo.anonimizados) != 0 {
		t.Errorf("no debe tocarse ninguna cuenta, se tocaron: %v", repo.anonimizados)
	}
}

func TestDeleteMyAccount_CuentaInexistenteSePropaga(t *testing.T) {
	// El repositorio devuelve ErrNotFound cuando la cuenta no existe o ya estaba
	// eliminada. El handler lo traduce a 404 en vez de responder 204 sobre algo
	// que no ha ocurrido.
	svc, _ := servicioDeCuentas(domain.ErrNotFound)

	err := svc.DeleteMyAccount(context.Background(), "user-1")

	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("se esperaba ErrNotFound, llegó %v", err)
	}
}
