package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
	"log"
	"strings"
	"time"
)

// Aviso de la mañana del evento.
//
// Hasta el 2026-10-21 nada avisaba de que un evento empezaba: solo salía el
// aviso de «Nuevo evento» al crearlo, a veces semanas antes. Por decisión del
// cliente ese día, el recordatorio va **solo a los inscritos confirmados**, a
// sus dispositivos (SendToUser, sin pasar por el historial del panel), **una
// vez, la mañana del evento**.
//
// Es por fecha y no por hora porque events.events.start_date es un `date`: el
// evento no guarda a qué hora empieza. Avisar «empieza en una hora» exigiría
// añadir ese campo al modelo y al panel.

var (
	// zonaColombia es fija a propósito: Colombia no cambia de hora, y así no se
	// depende de que la imagen traiga tzdata. Las fechas del panel se piensan en
	// hora de Bogotá; en UTC, de 7 p. m. a medianoche ya es el día siguiente.
	zonaColombia = time.FixedZone("America/Bogota", -5*60*60)
)

const (
	// horaRecordatorio es la hora local desde la que se avisa del evento del día.
	horaRecordatorio = 8

	// intervaloRecordatorio es cada cuánto se revisa. Acota el retraso del aviso
	// tras las 8:00 o tras un reinicio; revisar es una consulta pequeña.
	intervaloRecordatorio = 10 * time.Minute

	// tiempoMaximoRecordatorio acota una vuelta completa, con todos sus envíos.
	tiempoMaximoRecordatorio = 2 * time.Minute
)

type RecordatorioEventos struct {
	repo     ports.RecordatorioEventoRepository
	notifier ports.NotificationService
	ahora    func() time.Time
}

func NewRecordatorioEventos(repo ports.RecordatorioEventoRepository, notifier ports.NotificationService) *RecordatorioEventos {
	return &RecordatorioEventos{repo: repo, notifier: notifier, ahora: time.Now}
}

// Iniciar revisa al arrancar y luego cada intervaloRecordatorio, hasta que se
// cancele ctx. Pensado para `go recordatorio.Iniciar(ctx)` en main.go.
func (s *RecordatorioEventos) Iniciar(ctx context.Context) {
	ticker := time.NewTicker(intervaloRecordatorio)
	defer ticker.Stop()
	for {
		s.revisarConLimite(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *RecordatorioEventos) revisarConLimite(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, tiempoMaximoRecordatorio)
	defer cancel()
	s.Revisar(ctx)
}

// Revisar envía los recordatorios pendientes de hoy. **Nunca devuelve error**:
// es un proceso de fondo y lo que falle queda en el log para la próxima vuelta.
func (s *RecordatorioEventos) Revisar(ctx context.Context) {
	if s.notifier == nil {
		return
	}
	ahora := s.ahora().In(zonaColombia)
	if ahora.Hour() < horaRecordatorio {
		return
	}
	dia := ahora.Format("2006-01-02")

	eventos, err := s.repo.EventosSinRecordar(ctx, dia)
	if err != nil {
		log.Printf("[RECORDATORIO] no se pudieron leer los eventos de %s: %v", dia, err)
		return
	}

	for _, evento := range eventos {
		// Se marca antes de enviar: si el servidor cae a mitad del envío, es
		// preferible que alguien se quede sin el aviso a que todos lo reciban
		// dos veces en cada reinicio.
		reservado, err := s.repo.MarcarRecordado(ctx, evento.ID, dia)
		if err != nil {
			log.Printf("[RECORDATORIO] no se pudo marcar el evento %s: %v", evento.ID, err)
			continue
		}
		if !reservado {
			continue
		}

		usuarios, err := s.repo.InscritosConfirmados(ctx, evento.ID)
		if err != nil {
			log.Printf("[RECORDATORIO] no se pudieron leer los inscritos del evento %s: %v", evento.ID, err)
			continue
		}

		titulo, cuerpo := textoRecordatorio(evento)
		// `type` e `id` son los que la app traduce a la pantalla del evento
		// (domain/utils/notificacion_destino.dart), igual que «Nuevo evento».
		datos := map[string]string{"type": "event", "id": evento.ID}

		enviados := 0
		for _, userID := range usuarios {
			if err := s.notifier.SendToUser(ctx, userID, titulo, cuerpo, datos); err != nil {
				// Sin dispositivos registrados es lo normal para quien nunca
				// abrió la app en un teléfono.
				log.Printf("[RECORDATORIO] evento %s, usuario %s: %v", evento.ID, userID, err)
				continue
			}
			enviados++
		}
		log.Printf("[RECORDATORIO] evento %s (%s): avisados %d de %d inscritos", evento.ID, dia, enviados, len(usuarios))
	}
}

// textoRecordatorio arma el aviso. El cuerpo nunca va vacío: FCM lo rechaza.
func textoRecordatorio(e *domain.EventoPorRecordar) (titulo, cuerpo string) {
	titulo = "Hoy: " + strings.TrimSpace(e.Title)
	switch {
	case e.IsVirtual:
		cuerpo = "Tu evento es hoy. Entra a la app para conectarte."
	case e.Location != nil && strings.TrimSpace(*e.Location) != "":
		cuerpo = recortarMensaje("Tu evento es hoy. Te esperamos en "+strings.TrimSpace(*e.Location)+".", maxCuerpoMensaje)
	default:
		cuerpo = "Tu evento es hoy. Revisa los detalles en la app."
	}
	return titulo, cuerpo
}
