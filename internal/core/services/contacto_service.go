package services

import (
	"applegacy/backend/internal/core/ports"
	"context"
	"errors"
	"strings"
)

// asuntoPorDefecto se usa cuando el usuario no escribe ninguno. Sin esto el
// correo llegaría con el asunto vacío y se pierde entre los demás.
const asuntoPorDefecto = "Consulta desde la app"

const (
	maximoAsunto  = 200
	maximoMensaje = 5000
)

type contactoService struct {
	emailService ports.EmailService
	destinatario string
}

func NewContactoService(emailService ports.EmailService, destinatario string) ports.ContactoService {
	return &contactoService{
		emailService: emailService,
		destinatario: destinatario,
	}
}

func (s *contactoService) EnviarMensaje(ctx context.Context, asunto, remitenteNombre, remitenteEmail, mensaje string) error {
	if s.destinatario == "" {
		return errors.New("no hay buzón de contacto configurado")
	}

	mensaje = strings.TrimSpace(mensaje)
	if mensaje == "" {
		return errors.New("el mensaje no puede estar vacío")
	}

	asunto = strings.TrimSpace(asunto)
	if asunto == "" {
		asunto = asuntoPorDefecto
	}

	// Los límites se aplican aquí y no solo en el cliente: quien llama a la API
	// no tiene por qué ser la app.
	if len(asunto) > maximoAsunto {
		return errors.New("el asunto es demasiado largo")
	}
	if len(mensaje) > maximoMensaje {
		return errors.New("el mensaje es demasiado largo")
	}

	return s.emailService.SendContactoEmail(s.destinatario, asunto, remitenteNombre, remitenteEmail, mensaje)
}
