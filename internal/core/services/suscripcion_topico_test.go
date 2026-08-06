package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/infrastructure/firebase"
	"context"
	"errors"
	"testing"
)

// repoNotificaciones es un doble en memoria del repositorio de notificaciones.
type repoNotificaciones struct {
	tokens     []*domain.FCMToken
	guardados  []*domain.FCMToken
	errGuardar error
	errLeer    error
}

func (r *repoNotificaciones) SaveToken(ctx context.Context, t *domain.FCMToken) error {
	if r.errGuardar != nil {
		return r.errGuardar
	}
	r.guardados = append(r.guardados, t)
	return nil
}

func (r *repoNotificaciones) GetTokensByUserID(ctx context.Context, userID string) ([]*domain.FCMToken, error) {
	return nil, nil
}

func (r *repoNotificaciones) GetAllTokens(ctx context.Context) ([]*domain.FCMToken, error) {
	if r.errLeer != nil {
		return nil, r.errLeer
	}
	return r.tokens, nil
}

func (r *repoNotificaciones) DeleteToken(ctx context.Context, userID, token string) error { return nil }
func (r *repoNotificaciones) SaveHistory(ctx context.Context, h *domain.NotificationHistory) error {
	return nil
}
func (r *repoNotificaciones) GetHistory(ctx context.Context, limit, offset int) ([]*domain.NotificationHistory, error) {
	return nil, nil
}

// clienteMock: un FCMClient sin credenciales entra en modo mock, que es
// justamente lo que permite ejercitar el servicio sin hablar con Google. El
// nombre de archivo inexistente es deliberado.
func clienteMock(t *testing.T) *firebase.FCMClient {
	t.Helper()
	c, err := firebase.NewFCMClient("no-existe-este-archivo.json")
	if err != nil {
		t.Fatalf("no se pudo crear el cliente mock: %v", err)
	}
	return c
}

func TestRegisterToken_SuscribeAlTopicoGeneral(t *testing.T) {
	// La app registra el token y no se suscribe a ningún tópico, así que si el
	// servidor no lo hace, el dispositivo queda guardado pero no recibe los
	// envíos a "todos".
	repo := &repoNotificaciones{}
	svc := NewNotificationService(repo, clienteMock(t))

	if err := svc.RegisterToken(context.Background(), "user-1", "token-abc", "android"); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(repo.guardados) != 1 {
		t.Fatalf("el token debe guardarse, hubo %d", len(repo.guardados))
	}
	if repo.guardados[0].FCMToken != "token-abc" {
		t.Errorf("token esperado token-abc, llegó %q", repo.guardados[0].FCMToken)
	}
}

func TestRegisterToken_ExigeLosTresDatos(t *testing.T) {
	svc := NewNotificationService(&repoNotificaciones{}, clienteMock(t))

	casos := []struct{ userID, token, tipo string }{
		{"", "token", "android"},
		{"user-1", "", "android"},
		{"user-1", "token", ""},
	}
	for _, c := range casos {
		if err := svc.RegisterToken(context.Background(), c.userID, c.token, c.tipo); err == nil {
			t.Errorf("se esperaba error con (%q,%q,%q)", c.userID, c.token, c.tipo)
		}
	}
}

func TestRegisterToken_SiFallaElGuardadoNoSigue(t *testing.T) {
	repo := &repoNotificaciones{errGuardar: errors.New("base caida")}
	svc := NewNotificationService(repo, clienteMock(t))

	if err := svc.RegisterToken(context.Background(), "user-1", "token-abc", "android"); err == nil {
		t.Error("un fallo al guardar el token sí debe propagarse")
	}
}

func TestSubscribeAllToTopic_SuscribeLosTokensExistentes(t *testing.T) {
	// El caso que motiva todo: los dispositivos registrados antes de que
	// existiera la suscripción automática no se suscribirían nunca solos.
	repo := &repoNotificaciones{tokens: []*domain.FCMToken{
		{UserID: "u1", FCMToken: "t1"},
		{UserID: "u2", FCMToken: "t2"},
		{UserID: "u3", FCMToken: "t3"},
	}}
	svc := NewNotificationService(repo, clienteMock(t))

	suscritos, err := svc.SubscribeAllToTopic(context.Background())

	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if suscritos != 3 {
		t.Errorf("se esperaban 3 suscritos, llegaron %d", suscritos)
	}
}

func TestSubscribeAllToTopic_SinTokens(t *testing.T) {
	svc := NewNotificationService(&repoNotificaciones{}, clienteMock(t))

	suscritos, err := svc.SubscribeAllToTopic(context.Background())

	if err != nil {
		t.Fatalf("no tener tokens no es un error: %v", err)
	}
	if suscritos != 0 {
		t.Errorf("se esperaban 0, llegaron %d", suscritos)
	}
}

func TestSubscribeAllToTopic_DescartaTokensVacios(t *testing.T) {
	// Una fila con el token en blanco haría fallar la llamada al Admin SDK y
	// dejaría sin suscribir a todos los demás del lote.
	repo := &repoNotificaciones{tokens: []*domain.FCMToken{
		{UserID: "u1", FCMToken: "t1"},
		{UserID: "u2", FCMToken: ""},
	}}
	svc := NewNotificationService(repo, clienteMock(t))

	suscritos, err := svc.SubscribeAllToTopic(context.Background())

	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if suscritos != 1 {
		t.Errorf("solo debe contarse el token válido, llegaron %d", suscritos)
	}
}

func TestSubscribeAllToTopic_ErrorDeLectura(t *testing.T) {
	repo := &repoNotificaciones{errLeer: errors.New("base caida")}
	svc := NewNotificationService(repo, clienteMock(t))

	if _, err := svc.SubscribeAllToTopic(context.Background()); err == nil {
		t.Error("el error de lectura debe propagarse")
	}
}
