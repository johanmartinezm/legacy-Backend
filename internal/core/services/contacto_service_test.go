package services

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// correoDePrueba registra lo último que se le pidió enviar, para poder
// comprobar qué recibe el buzón sin mandar nada de verdad.
type correoDePrueba struct {
	llamado      bool
	destinatario string
	asunto       string
	nombre       string
	email        string
	mensaje      string
	fallar       error
}

func (c *correoDePrueba) SendResetPasswordEmail(to, resetURL string) error { return nil }
func (c *correoDePrueba) SendBoardContactEmail(to, senderName, senderEmail, messageText string) error {
	return nil
}
func (c *correoDePrueba) SendAsesoriaEmail(to, senderName, senderEmail, category, messageText string) error {
	return nil
}
func (c *correoDePrueba) SendWelcomeEmail(to, userName string) error  { return nil }
func (c *correoDePrueba) SendVerificationEmail(to, link string) error { return nil }

func (c *correoDePrueba) SendContactoEmail(to, asunto, senderName, senderEmail, messageText string) error {
	c.llamado = true
	c.destinatario, c.asunto, c.nombre, c.email, c.mensaje = to, asunto, senderName, senderEmail, messageText
	return c.fallar
}

func TestContactoEnviaAlBuzonConfigurado(t *testing.T) {
	correo := &correoDePrueba{}
	s := NewContactoService(correo, "soporte@ejemplo.com")

	err := s.EnviarMensaje(context.Background(), "Duda con un evento", "Ana Ruiz", "ana@ejemplo.com", "No puedo inscribirme")
	if err != nil {
		t.Fatalf("no debia fallar: %v", err)
	}
	if !correo.llamado {
		t.Fatal("no se envio ningun correo")
	}
	if correo.destinatario != "soporte@ejemplo.com" {
		t.Errorf("destinatario = %q", correo.destinatario)
	}
	if correo.asunto != "Duda con un evento" || correo.mensaje != "No puedo inscribirme" {
		t.Errorf("asunto = %q, mensaje = %q", correo.asunto, correo.mensaje)
	}
	// El remitente lo pone el handler desde el perfil autenticado, no el cliente.
	if correo.nombre != "Ana Ruiz" || correo.email != "ana@ejemplo.com" {
		t.Errorf("remitente = %q <%s>", correo.nombre, correo.email)
	}
}

func TestContactoRellenaElAsuntoVacio(t *testing.T) {
	correo := &correoDePrueba{}
	s := NewContactoService(correo, "soporte@ejemplo.com")

	// Un asunto en blanco llegaria vacio a la bandeja y se perderia entre los
	// demas: se sustituye por uno por defecto.
	if err := s.EnviarMensaje(context.Background(), "   ", "Ana", "ana@ejemplo.com", "Hola"); err != nil {
		t.Fatalf("no debia fallar: %v", err)
	}
	if correo.asunto != asuntoPorDefecto {
		t.Errorf("asunto = %q, se esperaba %q", correo.asunto, asuntoPorDefecto)
	}
}

func TestContactoRechazaMensajeVacio(t *testing.T) {
	correo := &correoDePrueba{}
	s := NewContactoService(correo, "soporte@ejemplo.com")

	// Solo espacios cuenta como vacio: si no, el buzon recibe correos en blanco.
	if err := s.EnviarMensaje(context.Background(), "Asunto", "Ana", "ana@ejemplo.com", "   \n  "); err == nil {
		t.Fatal("un mensaje en blanco debia rechazarse")
	}
	if correo.llamado {
		t.Error("no debia intentar enviar nada")
	}
}

func TestContactoRechazaMensajeDemasiadoLargo(t *testing.T) {
	correo := &correoDePrueba{}
	s := NewContactoService(correo, "soporte@ejemplo.com")

	// El limite se aplica en el servidor porque quien llama a la API no tiene
	// por que ser la app.
	err := s.EnviarMensaje(context.Background(), "Asunto", "Ana", "ana@ejemplo.com", strings.Repeat("a", maximoMensaje+1))
	if err == nil {
		t.Fatal("un mensaje pasado de largo debia rechazarse")
	}
	if correo.llamado {
		t.Error("no debia intentar enviar nada")
	}
}

func TestContactoSinBuzonConfigurado(t *testing.T) {
	correo := &correoDePrueba{}
	s := NewContactoService(correo, "")

	if err := s.EnviarMensaje(context.Background(), "Asunto", "Ana", "ana@ejemplo.com", "Hola"); err == nil {
		t.Fatal("sin buzon configurado debia fallar en vez de tragarse el mensaje")
	}
}

func TestContactoPropagaElFalloDelCorreo(t *testing.T) {
	correo := &correoDePrueba{fallar: errors.New("smtp caido")}
	s := NewContactoService(correo, "soporte@ejemplo.com")

	// Si el correo no sale, el usuario tiene que enterarse: aqui no vale
	// devolver exito y perder el mensaje.
	if err := s.EnviarMensaje(context.Background(), "Asunto", "Ana", "ana@ejemplo.com", "Hola"); err == nil {
		t.Fatal("el fallo del envio debia propagarse")
	}
}
