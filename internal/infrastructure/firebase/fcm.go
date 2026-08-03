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
