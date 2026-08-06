package firebase

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

type FCMClient struct {
	app *firebase.App
}

func NewFCMClient(credentialsPath string) (*FCMClient, error) {
	ctx := context.Background()
	var app *firebase.App
	var err error

	if credentialsPath == "" {
		credentialsPath = "firebase-service-account.json"
	}

	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		log.Printf("[FCM] Advertencia: Archivo de credenciales %s no encontrado. El envío de notificaciones push estará desactivado (Mock activo).", credentialsPath)
		return &FCMClient{app: nil}, nil
	}

	opt := option.WithCredentialsFile(credentialsPath)
	app, err = firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return nil, fmt.Errorf("error al inicializar firebase app: %v", err)
	}

	return &FCMClient{app: app}, nil
}

func (c *FCMClient) SendToToken(ctx context.Context, token, title, body string, data map[string]string) (string, error) {
	if c.app == nil {
		log.Printf("[FCM MOCK] Enviando notificación a Token: %s. Título: %s, Mensaje: %s", token, title, body)
		return "mock-message-id-token", nil
	}

	client, err := c.app.Messaging(ctx)
	if err != nil {
		return "", fmt.Errorf("error al obtener cliente messaging: %v", err)
	}

	message := &messaging.Message{
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data:  data,
		Token: token,
	}

	msgID, err := client.Send(ctx, message)
	if err != nil {
		return "", fmt.Errorf("error al enviar mensaje: %v", err)
	}

	return msgID, nil
}

// maxTokensPorLote es el límite que impone el Admin SDK en una sola llamada de
// suscripción. Hoy hay una decena de tokens, pero trocear cuesta poco y evita
// que esto falle en silencio el día que sean miles.
const maxTokensPorLote = 1000

// SubscribeToTopic suscribe dispositivos a un tópico desde el servidor.
//
// Existe porque la app **no llama a subscribeToTopic en ningún sitio**: registra
// su token en /api/me/fcm-token y nada más. Sin esto, `SendToTopic(..., "all")`
// —que es como el panel envía a "todos" y como salen los avisos automáticos— no
// llega a ningún teléfono, aunque haya tokens guardados en la base.
//
// Suscribir desde el servidor tiene una ventaja decisiva sobre hacerlo en
// Flutter: alcanza también a los dispositivos ya registrados, sin esperar a que
// se publique una versión nueva de la app.
//
// Devuelve cuántos tokens quedaron suscritos. Un token caducado hace fallar solo
// su parte del lote, no la llamada entera.
func (c *FCMClient) SubscribeToTopic(ctx context.Context, tokens []string, topic string) (int, error) {
	if topic == "" {
		return 0, errors.New("el tópico no puede estar vacío")
	}
	if len(tokens) == 0 {
		return 0, nil
	}

	if c.app == nil {
		log.Printf("[FCM MOCK] Suscribiendo %d token(s) al tópico %s", len(tokens), topic)
		return len(tokens), nil
	}

	client, err := c.app.Messaging(ctx)
	if err != nil {
		return 0, fmt.Errorf("error al obtener cliente messaging: %v", err)
	}

	suscritos := 0
	for inicio := 0; inicio < len(tokens); inicio += maxTokensPorLote {
		fin := inicio + maxTokensPorLote
		if fin > len(tokens) {
			fin = len(tokens)
		}

		resp, err := client.SubscribeToTopic(ctx, tokens[inicio:fin], topic)
		if err != nil {
			return suscritos, fmt.Errorf("error al suscribir al tópico %s: %v", topic, err)
		}
		suscritos += resp.SuccessCount
		if resp.FailureCount > 0 {
			// Lo habitual es un token caducado, de una app desinstalada. No es
			// motivo para dar por fallida la operación, pero conviene verlo.
			log.Printf("[FCM] %d token(s) no se pudieron suscribir a %s (probablemente caducados)",
				resp.FailureCount, topic)
		}
	}

	return suscritos, nil
}

func (c *FCMClient) SendToTopic(ctx context.Context, topic, title, body string, data map[string]string) (string, error) {
	if topic == "" {
		return "", errors.New("el tópico no puede estar vacío")
	}

	if c.app == nil {
		log.Printf("[FCM MOCK] Enviando notificación a Tópico: %s. Título: %s, Mensaje: %s", topic, title, body)
		return "mock-message-id-topic", nil
	}

	client, err := c.app.Messaging(ctx)
	if err != nil {
		return "", fmt.Errorf("error al obtener cliente messaging: %v", err)
	}

	message := &messaging.Message{
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data:  data,
		Topic: topic,
	}

	msgID, err := client.Send(ctx, message)
	if err != nil {
		return "", fmt.Errorf("error al enviar mensaje a tópico %s: %v", topic, err)
	}

	return msgID, nil
}
