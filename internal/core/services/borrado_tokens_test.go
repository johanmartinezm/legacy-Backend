package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/infrastructure/firebase"
	"context"
	"errors"
	"fmt"
	"testing"
)

// enviadorFalso permite decidir con qué error falla cada envío, que es lo que
// determina si un token se borra o se conserva.
type enviadorFalso struct {
	errPorToken map[string]error
	enviados    []string
}

func (e *enviadorFalso) SendToToken(ctx context.Context, token, title, body string, data map[string]string) (string, error) {
	e.enviados = append(e.enviados, token)
	if err, hay := e.errPorToken[token]; hay {
		return "", err
	}
	return "msg-id", nil
}

func (e *enviadorFalso) SendToTopic(ctx context.Context, topic, title, body string, data map[string]string) (string, error) {
	return "msg-id", nil
}

func (e *enviadorFalso) SubscribeToTopic(ctx context.Context, tokens []string, topic string) (int, error) {
	return len(tokens), nil
}

// repoConBorrado registra qué tokens se mandaron borrar.
type repoConBorrado struct {
	repoNotificaciones
	borrados []string
}

func (r *repoConBorrado) GetTokensByUserID(ctx context.Context, userID string) ([]*domain.FCMToken, error) {
	return r.tokens, nil
}

func (r *repoConBorrado) DeleteToken(ctx context.Context, userID, token string) error {
	r.borrados = append(r.borrados, token)
	return nil
}

func servicioDeEnvio(errPorToken map[string]error, tokens ...string) (*NotificationService, *repoConBorrado) {
	filas := make([]*domain.FCMToken, 0, len(tokens))
	for _, t := range tokens {
		filas = append(filas, &domain.FCMToken{UserID: "user-1", FCMToken: t})
	}
	repo := &repoConBorrado{repoNotificaciones: repoNotificaciones{tokens: filas}}
	return NewNotificationService(repo, &enviadorFalso{errPorToken: errPorToken}), repo
}

func TestEnvio_UnFalloPasajeroNoBorraElToken(t *testing.T) {
	// El motivo del arreglo: un corte momentáneo con Google borraba
	// dispositivos buenos, y esos usuarios dejaban de recibir notificaciones
	// para siempre sin que nadie se enterara.
	svc, repo := servicioDeEnvio(map[string]error{
		"token-bueno": errors.New("servidor no disponible, intenta luego"),
	}, "token-bueno")

	_ = svc.SendNotification(context.Background(), "", "Hola", "Cuerpo", "user", "user-1", nil)

	if len(repo.borrados) != 0 {
		t.Errorf("un fallo pasajero no debe borrar el token, se borraron %v", repo.borrados)
	}
}

func TestEnvio_UnTokenMuertoSiSeBorra(t *testing.T) {
	svc, repo := servicioDeEnvio(map[string]error{
		"token-muerto": fmt.Errorf("%w: app desinstalada", firebase.ErrTokenInvalido),
	}, "token-muerto")

	_ = svc.SendNotification(context.Background(), "", "Hola", "Cuerpo", "user", "user-1", nil)

	if len(repo.borrados) != 1 || repo.borrados[0] != "token-muerto" {
		t.Errorf("el token muerto debe borrarse, se borraron %v", repo.borrados)
	}
}

func TestEnvio_SoloSeBorraElMuertoYElRestoSigue(t *testing.T) {
	// Un usuario con varios dispositivos: que uno esté muerto no puede afectar a
	// los demás, ni en el envío ni en el borrado.
	svc, repo := servicioDeEnvio(map[string]error{
		"token-muerto": fmt.Errorf("%w: app desinstalada", firebase.ErrTokenInvalido),
		"token-caido":  errors.New("timeout"),
	}, "token-vivo", "token-muerto", "token-caido")

	err := svc.SendNotification(context.Background(), "", "Hola", "Cuerpo", "user", "user-1", nil)

	// No fallaron todos, así que el envío en conjunto se da por bueno.
	if err != nil {
		t.Errorf("con al menos un envío correcto no debe devolverse error: %v", err)
	}
	if len(repo.borrados) != 1 || repo.borrados[0] != "token-muerto" {
		t.Errorf("solo debe borrarse el token muerto, se borraron %v", repo.borrados)
	}
}

func TestEnvio_SiTodosFallanSeDevuelveError(t *testing.T) {
	svc, _ := servicioDeEnvio(map[string]error{
		"t1": errors.New("timeout"),
		"t2": errors.New("timeout"),
	}, "t1", "t2")

	if err := svc.SendNotification(context.Background(), "", "Hola", "Cuerpo", "user", "user-1", nil); err == nil {
		t.Error("si fallan todos los envíos, el error debe propagarse")
	}
}
