package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"
	"testing"
	"time"
)

// El fallo que estos tests fijan no era de lógica sino de identidad: el
// repositorio de tokens hablaba del blind index del correo mientras la tabla
// guarda user_id. La consulta iba contra una columna inexistente y todo registro
// con correo y contraseña moría con SQLSTATE 42703 después de haber creado la
// cuenta. Lo que se comprueba aquí es que el identificador que sale de validar
// un token es exactamente el que se usa para marcar el correo como verificado.

type repoTokensVerificacion struct {
	guardadoPara string // a quién se le guardó el último token
	devuelve     string // qué id devuelve al validar
	err          error
}

func (r *repoTokensVerificacion) StoreToken(ctx context.Context, userID, token string, expiresAt time.Time) error {
	r.guardadoPara = userID
	return r.err
}

func (r *repoTokensVerificacion) ValidateToken(ctx context.Context, token string) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	return r.devuelve, nil
}

func (r *repoTokensVerificacion) DeleteToken(ctx context.Context, token string) error { return nil }

// repoUsuariosVerificacion captura a quién se marcó como verificado.
type repoUsuariosVerificacion struct {
	verificados []string
}

func (r *repoUsuariosVerificacion) MarkEmailAsVerified(ctx context.Context, userID string) error {
	r.verificados = append(r.verificados, userID)
	return nil
}

func (r *repoUsuariosVerificacion) Create(ctx context.Context, u *domain.User) error { return nil }
func (r *repoUsuariosVerificacion) FindByEmailBlindIndex(ctx context.Context, b string) (*domain.User, error) {
	return nil, nil
}
func (r *repoUsuariosVerificacion) FindAll(ctx context.Context, limit, offset int) ([]*domain.User, error) {
	return nil, nil
}
func (r *repoUsuariosVerificacion) CountAll(ctx context.Context) (int, error) { return 0, nil }
func (r *repoUsuariosVerificacion) FindByID(ctx context.Context, id string) (*domain.User, error) {
	return nil, nil
}
func (r *repoUsuariosVerificacion) Update(ctx context.Context, u *domain.User) error { return nil }
func (r *repoUsuariosVerificacion) Delete(ctx context.Context, id string) error      { return nil }
func (r *repoUsuariosVerificacion) UpdatePassword(ctx context.Context, id, h string) error {
	return nil
}
func (r *repoUsuariosVerificacion) UpdatePasswordByEmail(ctx context.Context, e, h string) error {
	return nil
}
func (r *repoUsuariosVerificacion) AnonymizeUser(ctx context.Context, id string) error { return nil }

func TestVerifyEmail_MarcaAQuienDiceElToken(t *testing.T) {
	tokens := &repoTokensVerificacion{devuelve: "user-42"}
	usuarios := &repoUsuariosVerificacion{}
	svc := &AuthService{repo: usuarios, verifyRepo: tokens}

	if err := svc.VerifyEmail(context.Background(), "un-token"); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if len(usuarios.verificados) != 1 || usuarios.verificados[0] != "user-42" {
		t.Errorf("debe marcarse la cuenta dueña del token, se marcó: %v", usuarios.verificados)
	}
}

func TestVerifyEmail_TokenInvalidoNoVerificaANadie(t *testing.T) {
	// Lo peor de un token inválido no es el error: es dar por verificado un
	// correo que nadie confirmó.
	tokens := &repoTokensVerificacion{err: errors.New("invalid or expired token")}
	usuarios := &repoUsuariosVerificacion{}
	svc := &AuthService{repo: usuarios, verifyRepo: tokens}

	if err := svc.VerifyEmail(context.Background(), "token-falso"); err == nil {
		t.Error("un token inválido debe devolver error")
	}

	if len(usuarios.verificados) != 0 {
		t.Errorf("no debe marcarse ninguna cuenta, se marcó: %v", usuarios.verificados)
	}
}

func (r *repoUsuariosVerificacion) FindBySocialID(ctx context.Context, provider, socialID string) (*domain.User, error) {
	return nil, errors.New("user not found")
}
func (r *repoUsuariosVerificacion) LinkSocialID(ctx context.Context, userID, provider, socialID string) error {
	return nil
}
