package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"log"
	"strings"
	"time"
)

// Aviso push al recibir un mensaje de chat.
//
// Hasta ahora un mensaje no avisaba a nadie: crear un evento y publicar
// contenido sí mandaban push al tópico "all" (`handler/http/avisos.go`), pero el
// chat no, así que la conversación solo avanzaba si la otra persona abría la app
// por su cuenta. Es la única pieza que le faltaba al módulo de notificaciones.
//
// A diferencia de las novedades, este aviso **no va al tópico "all"** sino a los
// dispositivos del destinatario: un mensaje privado no le interesa a la
// comunidad, y mandarlo por tópico lo entregaría a todos los teléfonos.

const (
	// maxCuerpoMensaje recorta la vista previa. Android e iOS truncan de todas
	// formas, y el mensaje entero solo llena el centro de notificaciones.
	maxCuerpoMensaje = 140

	// tiempoMaximoAviso acota el envío. Corre fuera de la petición HTTP, así que
	// sin límite una llamada colgada a FCM dejaría la goroutine viva para
	// siempre.
	tiempoMaximoAviso = 15 * time.Second

	// tituloPorDefecto se usa cuando no se puede resolver el nombre de quien
	// escribe. Vale más un aviso sin nombre que ningún aviso.
	tituloPorDefecto = "Nuevo mensaje"

	// cuerpoPorDefecto cubre el mensaje sin texto: FCM rechaza un cuerpo vacío,
	// así que sin esto ese envío fallaría entero.
	cuerpoPorDefecto = "Te envió un mensaje"
)

// avisarMensajeNuevo notifica al otro lado de la conversación.
//
// Sale en su propia goroutine y **nunca devuelve error**, por las mismas dos
// razones que los avisos de novedades: el envío tarda lo que tarde FCM y quien
// escribe no tiene por qué esperarlo, y si Firebase está caído —o corre en modo
// mock por falta de `firebase-service-account.json`— el mensaje tiene que
// guardarse y entregarse igual. Un fallo aquí que tumbara el envío convertiría
// una molestia en una avería.
//
// Lleva contexto propio a propósito: el de la petición HTTP se cancela en cuanto
// se responde al remitente, y con él se cancelaría el envío a medias.
func (s *ChatService) avisarMensajeNuevo(conn *domain.ChatConnection, connectionID, senderID, contenido string) {
	if s.notifier == nil {
		return
	}

	destinatario := conn.ReceiverID
	if senderID == conn.ReceiverID {
		destinatario = conn.RequesterID
	}
	if destinatario == "" {
		return
	}

	cuerpo := recortarMensaje(contenido, maxCuerpoMensaje)
	if cuerpo == "" {
		cuerpo = cuerpoPorDefecto
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), tiempoMaximoAviso)
		defer cancel()

		titulo := s.nombreDe(ctx, senderID)

		// `type` e `id` son los que lee la app para abrir la conversación al
		// tocar la notificación; `title` le ahorra una consulta para pintar el
		// encabezado, que la ruta /individual-chat/:id espera como parámetro.
		datos := map[string]string{
			"type":  "chat",
			"id":    connectionID,
			"title": titulo,
		}

		if err := s.notifier.SendToUser(ctx, destinatario, titulo, cuerpo, datos); err != nil {
			// Que el destinatario no tenga ningún dispositivo registrado es lo
			// normal si nunca abrió la app en un teléfono; no es una avería.
			log.Printf("[CHAT] no se pudo avisar del mensaje en la conexion %s: %v", connectionID, err)
		}
	}()
}

// nombreDe resuelve el nombre de quien escribe, que va como título del aviso.
//
// Los nombres están cifrados en reposo, así que hay que descifrarlos: leerlos
// del repositorio sin pasar por aquí pondría base64 en la notificación.
func (s *ChatService) nombreDe(ctx context.Context, userID string) string {
	if s.userRepo == nil {
		return tituloPorDefecto
	}

	usuario, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || usuario == nil {
		return tituloPorDefecto
	}

	nombre, err := s.crypto.Decrypt(usuario.FirstName)
	if err != nil {
		nombre = ""
	}
	apellido, err := s.crypto.Decrypt(usuario.LastName)
	if err != nil {
		apellido = ""
	}

	completo := strings.TrimSpace(strings.TrimSpace(nombre) + " " + strings.TrimSpace(apellido))
	if completo == "" {
		return tituloPorDefecto
	}
	return completo
}

// recortarMensaje deja el texto en un largo razonable, cortando por el último
// espacio para no partir una palabra.
//
// Cuenta **caracteres, no bytes**, a diferencia de su gemela `recortar` en
// `handler/http/avisos.go`: en un chat abundan las tildes, las eñes y los
// emojis, y cortar a mitad de un carácter de varios bytes produce un texto
// inválido que el teléfono pinta como un rombo negro. Se duplica en vez de
// compartirse porque el núcleo no puede importar la capa de handlers sin
// invertir la dependencia.
func recortarMensaje(texto string, maximo int) string {
	texto = strings.TrimSpace(texto)
	runas := []rune(texto)
	if len(runas) <= maximo {
		return texto
	}
	corte := string(runas[:maximo])
	if i := strings.LastIndex(corte, " "); i > len(corte)/2 {
		corte = corte[:i]
	}
	return strings.TrimSpace(corte) + "…"
}
