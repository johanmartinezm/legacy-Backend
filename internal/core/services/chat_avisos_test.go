package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/security"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// avisoRecibido es lo que llegó al servicio de notificaciones.
type avisoRecibido struct {
	userID string
	titulo string
	cuerpo string
	datos  map[string]string
}

// notificadorChat captura el aviso. El envío sale en su propia goroutine, así
// que el canal es lo que permite esperarlo sin dormir un tiempo fijo.
type notificadorChat struct {
	avisos chan avisoRecibido
	err    error
}

func nuevoNotificadorChat(err error) *notificadorChat {
	return &notificadorChat{avisos: make(chan avisoRecibido, 4), err: err}
}

func (n *notificadorChat) SendToUser(ctx context.Context, userID, title, body string, data map[string]string) error {
	n.avisos <- avisoRecibido{userID: userID, titulo: title, cuerpo: body, datos: data}
	return n.err
}

// El resto de la interfaz no interviene en el chat.
func (n *notificadorChat) RegisterToken(ctx context.Context, userID, token, deviceType string) error {
	return nil
}
func (n *notificadorChat) SendNotification(ctx context.Context, adminID, title, body, targetType, targetValue string, data map[string]string) error {
	return nil
}
func (n *notificadorChat) GetHistory(ctx context.Context, limit, offset int) ([]*domain.NotificationHistory, error) {
	return nil, nil
}
func (n *notificadorChat) SubscribeAllToTopic(ctx context.Context) (int, error) { return 0, nil }

// esperarAviso da tiempo a la goroutine del envío. Un fallo aquí significa que
// el mensaje se guardó y nadie se enteró, que es justo lo que se corrige.
func (n *notificadorChat) esperarAviso(t *testing.T) avisoRecibido {
	t.Helper()
	select {
	case a := <-n.avisos:
		return a
	case <-time.After(2 * time.Second):
		t.Fatal("no se envió ninguna notificación del mensaje")
		return avisoRecibido{}
	}
}

// repoUsuariosChat devuelve al remitente con el nombre cifrado, como está en la
// base: si el aviso no descifrara, el título de la notificación sería base64.
type repoUsuariosChat struct {
	usuario *domain.User
	err     error
}

func (r *repoUsuariosChat) FindByID(ctx context.Context, id string) (*domain.User, error) {
	return r.usuario, r.err
}
func (r *repoUsuariosChat) Create(ctx context.Context, u *domain.User) error { return nil }
func (r *repoUsuariosChat) FindByEmailBlindIndex(ctx context.Context, b string) (*domain.User, error) {
	return nil, nil
}
func (r *repoUsuariosChat) FindAll(ctx context.Context, limit, offset int) ([]*domain.User, error) { return nil, nil }
func (r *repoUsuariosChat) CountAll(ctx context.Context) (int, error) { return 0, nil }
func (r *repoUsuariosChat) FindBySocialID(ctx context.Context, provider, socialID string) (*domain.User, error) {
	return nil, errors.New("user not found")
}
func (r *repoUsuariosChat) LinkSocialID(ctx context.Context, userID, provider, socialID string) error {
	return nil
}
func (r *repoUsuariosChat) Update(ctx context.Context, u *domain.User) error             { return nil }
func (r *repoUsuariosChat) Delete(ctx context.Context, id string) error                  { return nil }
func (r *repoUsuariosChat) AnonymizeUser(ctx context.Context, id string) error           { return nil }
func (r *repoUsuariosChat) UpdatePassword(ctx context.Context, id, h string) error       { return nil }
func (r *repoUsuariosChat) UpdatePasswordByEmail(ctx context.Context, e, h string) error { return nil }
func (r *repoUsuariosChat) MarkEmailAsVerified(ctx context.Context, b string) error      { return nil }

// conversacionAceptada arma el escenario común: una conexión válida entre dos
// personas y un remitente con nombre.
func conversacionAceptada(t *testing.T, notificador *notificadorChat) (*ChatService, *security.CryptoService) {
	t.Helper()
	crypto, err := security.NewCryptoService("12345678901234567890123456789012")
	if err != nil {
		t.Fatalf("no se pudo crear el servicio de cifrado: %v", err)
	}

	nombre, _ := crypto.Encrypt("Juan")
	apellido, _ := crypto.Encrypt("Pérez")

	repoChat := &MockChatRepository{
		GetConnectionFunc: func(ctx context.Context, id string) (*domain.ChatConnection, error) {
			return &domain.ChatConnection{
				ID:          id,
				RequesterID: "quien-escribe",
				ReceiverID:  "quien-recibe",
				Status:      domain.StatusAccepted,
			}, nil
		},
		SaveMessageFunc: func(ctx context.Context, msg *domain.Message) error { return nil },
	}

	repoUsers := &repoUsuariosChat{
		usuario: &domain.User{ID: "quien-escribe", FirstName: nombre, LastName: apellido},
	}

	return NewChatService(repoChat, repoUsers, nuevoRepoBloqueos(), crypto, notificador), crypto
}

func TestSendMessage_AvisaAlDestinatario(t *testing.T) {
	notificador := nuevoNotificadorChat(nil)
	servicio, _ := conversacionAceptada(t, notificador)

	if _, err := servicio.SendMessage(context.Background(), "quien-escribe", "conexion-1", "¿Confirmamos la reunión del jueves?"); err != nil {
		t.Fatalf("error inesperado al enviar: %v", err)
	}

	aviso := notificador.esperarAviso(t)

	if aviso.userID != "quien-recibe" {
		t.Errorf("el aviso debe ir al otro lado de la conversación, fue a %q", aviso.userID)
	}
	if aviso.titulo != "Juan Pérez" {
		t.Errorf("el título debe ser el nombre descifrado de quien escribe, fue %q", aviso.titulo)
	}
	if aviso.cuerpo != "¿Confirmamos la reunión del jueves?" {
		t.Errorf("el cuerpo debe ser el mensaje, fue %q", aviso.cuerpo)
	}
	if aviso.datos["type"] != "chat" || aviso.datos["id"] != "conexion-1" {
		t.Errorf("sin type=chat e id de la conexión la app no puede abrir la conversación: %v", aviso.datos)
	}
	if aviso.datos["title"] != "Juan Pérez" {
		t.Errorf("el nombre viaja en los datos para pintar el encabezado, fue %q", aviso.datos["title"])
	}
}

func TestSendMessage_AvisaAlOtroLadoCuandoResponde(t *testing.T) {
	// La dirección no es fija: si quien escribe es el receptor de la invitación,
	// el aviso tiene que ir al que invitó. Invertirlo notificaría a la persona
	// equivocada, que además vería su propio mensaje.
	notificador := nuevoNotificadorChat(nil)
	servicio, _ := conversacionAceptada(t, notificador)

	if _, err := servicio.SendMessage(context.Background(), "quien-recibe", "conexion-1", "Confirmado"); err != nil {
		t.Fatalf("error inesperado al enviar: %v", err)
	}

	if aviso := notificador.esperarAviso(t); aviso.userID != "quien-escribe" {
		t.Errorf("el aviso debía ir a quien-escribe, fue a %q", aviso.userID)
	}
}

func TestSendMessage_ElMensajeSeEnviaAunqueFalleElAviso(t *testing.T) {
	// El aviso es un efecto secundario: con FCM caído, o en modo mock por falta
	// de credenciales, el mensaje tiene que guardarse y devolverse igual.
	notificador := nuevoNotificadorChat(errors.New("FCM no disponible"))
	servicio, _ := conversacionAceptada(t, notificador)

	msg, err := servicio.SendMessage(context.Background(), "quien-escribe", "conexion-1", "Hola")
	if err != nil {
		t.Fatalf("un fallo del aviso no puede tumbar el envío: %v", err)
	}
	if msg == nil || msg.ContentEncrypted != "Hola" {
		t.Error("el mensaje debe volver con su contenido en claro para quien lo escribió")
	}

	notificador.esperarAviso(t)
}

func TestSendMessage_SinNotificadorSigueFuncionando(t *testing.T) {
	// Es como quedan los tests y cualquier montaje sin notificaciones: el chat
	// no puede depender de que exista un servicio de push.
	crypto, _ := security.NewCryptoService("12345678901234567890123456789012")
	repoChat := &MockChatRepository{
		GetConnectionFunc: func(ctx context.Context, id string) (*domain.ChatConnection, error) {
			return &domain.ChatConnection{
				ID: id, RequesterID: "a", ReceiverID: "b", Status: domain.StatusAccepted,
			}, nil
		},
		SaveMessageFunc: func(ctx context.Context, msg *domain.Message) error { return nil },
	}

	servicio := NewChatService(repoChat, nil, nuevoRepoBloqueos(), crypto, nil)

	if _, err := servicio.SendMessage(context.Background(), "a", "conexion-1", "Hola"); err != nil {
		t.Fatalf("sin notificador el chat debe seguir funcionando: %v", err)
	}
}

func TestSendMessage_MensajeNoGuardadoNoAvisa(t *testing.T) {
	// Avisar de un mensaje que no llegó a guardarse mandaría a la otra persona a
	// abrir una conversación donde no hay nada.
	notificador := nuevoNotificadorChat(nil)
	servicio, _ := conversacionAceptada(t, notificador)
	servicio.repo.(*MockChatRepository).SaveMessageFunc = func(ctx context.Context, msg *domain.Message) error {
		return errors.New("la base no responde")
	}

	if _, err := servicio.SendMessage(context.Background(), "quien-escribe", "conexion-1", "Hola"); err == nil {
		t.Fatal("se esperaba error al guardar")
	}

	select {
	case aviso := <-notificador.avisos:
		t.Errorf("no debía avisarse de un mensaje que no se guardó: %v", aviso)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestRecortarMensaje(t *testing.T) {
	t.Run("un mensaje corto va entero", func(t *testing.T) {
		if got := recortarMensaje("  Hola  ", 140); got != "Hola" {
			t.Errorf("se esperaba %q, se obtuvo %q", "Hola", got)
		}
	})

	t.Run("uno largo se corta por el último espacio", func(t *testing.T) {
		got := recortarMensaje(strings.Repeat("palabra ", 40), 140)
		if !strings.HasSuffix(got, "…") {
			t.Errorf("el recorte debe indicarse con puntos suspensivos: %q", got)
		}
		if len([]rune(got)) > 141 {
			t.Errorf("el recorte no respetó el máximo: %d caracteres", len([]rune(got)))
		}
	})

	t.Run("no parte un carácter de varios bytes", func(t *testing.T) {
		// Contar bytes en vez de caracteres dejaba media eñe o medio emoji al
		// final, que el teléfono pinta como un rombo negro.
		got := recortarMensaje(strings.Repeat("ñ", 200), 140)
		if strings.ContainsRune(got, '�') {
			t.Errorf("el recorte partió un carácter: %q", got)
		}
		if len([]rune(got)) != 141 {
			t.Errorf("debían quedar 140 caracteres más los puntos, quedaron %d", len([]rune(got)))
		}
	})
}
