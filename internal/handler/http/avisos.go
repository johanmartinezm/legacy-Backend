package http

import (
	"context"
	"log"
	"strings"

	"applegacy/backend/internal/core/ports"
)

// Avisos automáticos a los usuarios de la app.
//
// Hasta ahora las notificaciones push solo salían a mano desde el panel: crear
// un evento o publicar contenido no avisaba a nadie, así que la novedad solo se
// veía si el usuario abría la app por su cuenta.
//
// El aviso se manda al topic "all", el mismo que usa el envío manual del panel.
// No se segmenta por grupos: los `core.custom_groups` existen para eso, pero un
// evento nuevo interesa a toda la comunidad y elegir a quién avisar sería una
// decisión de producto, no una que convenga inventar aquí.

// maxCuerpoAviso recorta el texto que se muestra en la notificación. Android e
// iOS truncan de todos modos, y mandar el artículo entero solo gasta ancho de
// banda y llena el historial de notificaciones con ruido.
const maxCuerpoAviso = 140

// notificarNovedad envía el aviso y **nunca devuelve error**.
//
// Es deliberado: el aviso es un efecto secundario. Si FCM está caído, o falta el
// firebase-service-account.json y el cliente corre en modo mock, crear el evento
// tiene que funcionar igual. Un fallo aquí que tumbara la creación convertiría
// una molestia en una avería.
func notificarNovedad(ctx context.Context, notifier ports.NotificationService, adminID, titulo, cuerpo string, datos map[string]string) {
	if notifier == nil {
		return
	}
	titulo = strings.TrimSpace(titulo)
	if titulo == "" {
		// Sin título no hay notificación posible: SendNotification lo rechaza, y
		// avisar de algo sin nombre tampoco le sirve a nadie.
		log.Printf("[AVISO] no se envia: la novedad no tiene titulo (datos=%v)", datos)
		return
	}

	if err := notifier.SendNotification(ctx, adminID, titulo, recortar(cuerpo, maxCuerpoAviso), "all", "", datos); err != nil {
		log.Printf("[AVISO] fallo el envio de la novedad %q: %v", titulo, err)
	}
}

// textoDe desreferencia un campo opcional. Varios campos del dominio son
// *string —Event.Description entre ellos— y aquí un nulo es simplemente un
// aviso sin cuerpo, no un error.
func textoDe(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

// recortar deja el texto en un largo razonable, cortando por el último espacio
// para no partir una palabra por la mitad.
func recortar(texto string, maximo int) string {
	texto = strings.TrimSpace(texto)
	if len(texto) <= maximo {
		return texto
	}
	corte := texto[:maximo]
	if i := strings.LastIndex(corte, " "); i > maximo/2 {
		corte = corte[:i]
	}
	return strings.TrimSpace(corte) + "…"
}
