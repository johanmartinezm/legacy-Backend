package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"applegacy/backend/internal/security"
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"
)

// asuntoPorDefecto se usa cuando el usuario no escribe ninguno. Sin esto el
// correo llegaría con el asunto vacío y se pierde entre los demás.
const asuntoPorDefecto = "Consulta desde la app"

const (
	maximoAsunto  = 200
	maximoMensaje = 5000

	// Límite de frecuencia. No es contra un ataque —hace falta sesión válida—
	// sino contra el envío repetido por nervios o por un botón que se queda
	// pulsado: sin esto, una persona puede llenar el buzón.
	maximoPorVentana = 5
	ventanaDeEnvios  = time.Hour
)

// ErrDemasiadosMensajes lo distingue el handler para responder 429 en vez de
// 400: no es que el mensaje esté mal, es que hay que esperar.
var ErrDemasiadosMensajes = errors.New("has enviado varios mensajes seguidos; espera un momento antes de escribir otro")

type contactoService struct {
	repo         ports.ContactoRepository
	emailService ports.EmailService
	crypto       *security.CryptoService
	destinatario string
}

func NewContactoService(
	repo ports.ContactoRepository,
	emailService ports.EmailService,
	crypto *security.CryptoService,
	destinatario string,
) ports.ContactoService {
	return &contactoService{
		repo:         repo,
		emailService: emailService,
		crypto:       crypto,
		destinatario: destinatario,
	}
}

// EnviarMensaje guarda el mensaje y luego intenta el correo, en ese orden.
//
// **El orden es la razón de ser de esta tabla.** Antes solo se enviaba el
// correo: si el SMTP fallaba, el mensaje se perdía entero y quien lo escribió no
// tenía forma de saberlo salvo por el aviso en pantalla. Ahora un fallo de envío
// deja el mensaje guardado y marcado como no enviado, y la persona recibe
// confirmación porque su mensaje **sí llegó**: está en la bandeja del panel.
func (s *contactoService) EnviarMensaje(ctx context.Context, userID, asunto, remitenteNombre, remitenteEmail, mensaje string) error {
	if userID == "" {
		return errors.New("falta el remitente")
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

	recientes, err := s.repo.ContarDesde(ctx, userID, time.Now().Add(-ventanaDeEnvios))
	if err != nil {
		return err
	}
	if recientes >= maximoPorVentana {
		return ErrDemasiadosMensajes
	}

	// Cifrado en reposo, el mismo trato que los mensajes de chat: es texto libre
	// y puede contener cualquier cosa.
	asuntoCifrado, err := s.crypto.Encrypt(asunto)
	if err != nil {
		return fmt.Errorf("no se pudo cifrar el asunto: %w", err)
	}
	mensajeCifrado, err := s.crypto.Encrypt(mensaje)
	if err != nil {
		return fmt.Errorf("no se pudo cifrar el mensaje: %w", err)
	}

	id, err := s.repo.Guardar(ctx, &domain.MensajeDeContacto{
		UserID:  userID,
		Asunto:  asuntoCifrado,
		Mensaje: mensajeCifrado,
	})
	if err != nil {
		// Aquí sí se falla: sin guardar ni enviar, el mensaje se habría perdido.
		return err
	}

	if s.destinatario == "" {
		// El mensaje está guardado; lo que falta es a quién avisar. Se registra
		// para que se note en el arranque siguiente, pero no se pierde nada.
		log.Printf("contacto: mensaje %s guardado sin enviar, no hay buzón configurado", id)
		return nil
	}

	if err := s.emailService.SendContactoEmail(s.destinatario, asunto, remitenteNombre, remitenteEmail, mensaje); err != nil {
		log.Printf("contacto: mensaje %s guardado pero el correo no salió: %v", id, err)
		return nil
	}

	if err := s.repo.MarcarEnviado(ctx, id); err != nil {
		// El correo salió; que no se haya podido anotar no cambia nada para
		// quien escribió.
		log.Printf("contacto: mensaje %s enviado pero no se pudo marcar: %v", id, err)
	}
	return nil
}

// Listar devuelve la bandeja con todo descifrado y listo para el panel.
func (s *contactoService) Listar(ctx context.Context, estado string) ([]*domain.MensajeDeContacto, error) {
	if estado != "" && !slices.Contains(domain.EstadosDeContactoValidos, estado) {
		return nil, fmt.Errorf("estado no válido: %s", estado)
	}

	mensajes, err := s.repo.Listar(ctx, estado)
	if err != nil {
		return nil, err
	}

	for _, m := range mensajes {
		// Un valor que no descifra se deja como está en vez de romper la
		// bandeja entera: es preferible una fila rara a una pantalla vacía.
		m.Asunto = s.descifrar(m.Asunto)
		m.Mensaje = s.descifrar(m.Mensaje)
		m.RemitenteNombre = strings.TrimSpace(s.descifrar(m.RemitenteNombre) + " " + s.descifrar(m.RemitenteApellido))
		m.RemitenteApellido = ""
		m.RemitenteEmail = s.descifrar(m.RemitenteEmail)
	}
	return mensajes, nil
}

func (s *contactoService) CambiarEstado(ctx context.Context, id, estado string) error {
	if id == "" {
		return errors.New("falta el mensaje")
	}
	if !slices.Contains(domain.EstadosDeContactoValidos, estado) {
		return fmt.Errorf("estado no válido: %s", estado)
	}
	return s.repo.CambiarEstado(ctx, id, estado)
}

func (s *contactoService) descifrar(valor string) string {
	if valor == "" {
		return ""
	}
	claro, err := s.crypto.Decrypt(valor)
	if err != nil || claro == "" {
		return valor
	}
	return claro
}
