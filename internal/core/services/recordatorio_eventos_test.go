package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"testing"
	"time"
)

// repoRecordatorio simula events.event_reminders: `marcados` hace de clave
// primaria (evento + día).
type repoRecordatorio struct {
	eventos   map[string][]*domain.EventoPorRecordar // por día
	inscritos map[string][]string                    // por evento
	marcados  map[string]bool
	diasLeido []string
}

func nuevoRepoRecordatorio() *repoRecordatorio {
	return &repoRecordatorio{
		eventos:   map[string][]*domain.EventoPorRecordar{},
		inscritos: map[string][]string{},
		marcados:  map[string]bool{},
	}
}

func (r *repoRecordatorio) EventosSinRecordar(ctx context.Context, dia string) ([]*domain.EventoPorRecordar, error) {
	r.diasLeido = append(r.diasLeido, dia)
	var pendientes []*domain.EventoPorRecordar
	for _, e := range r.eventos[dia] {
		if !r.marcados[e.ID+"|"+dia] {
			pendientes = append(pendientes, e)
		}
	}
	return pendientes, nil
}

func (r *repoRecordatorio) MarcarRecordado(ctx context.Context, eventID, dia string) (bool, error) {
	clave := eventID + "|" + dia
	if r.marcados[clave] {
		return false, nil
	}
	r.marcados[clave] = true
	return true, nil
}

func (r *repoRecordatorio) InscritosConfirmados(ctx context.Context, eventID string) ([]string, error) {
	return r.inscritos[eventID], nil
}

// recordatorioA arma el servicio con el reloj fijo en una hora de Bogotá.
func recordatorioA(repo *repoRecordatorio, notificador *notificadorChat, anio int, mes time.Month, dia, hora, minuto int) *RecordatorioEventos {
	s := NewRecordatorioEventos(repo, notificador)
	s.ahora = func() time.Time { return time.Date(anio, mes, dia, hora, minuto, 0, 0, zonaColombia) }
	return s
}

func avisosPendientes(n *notificadorChat) []avisoRecibido {
	var avisos []avisoRecibido
	for {
		select {
		case a := <-n.avisos:
			avisos = append(avisos, a)
		default:
			return avisos
		}
	}
}

func TestRecordatorio_AntesDeLasOchoNoAvisa(t *testing.T) {
	repo := nuevoRepoRecordatorio()
	repo.eventos["2026-10-22"] = []*domain.EventoPorRecordar{{ID: "ev-1", Title: "Masterclass"}}
	repo.inscritos["ev-1"] = []string{"u-1"}
	notificador := nuevoNotificadorChat(nil)

	recordatorioA(repo, notificador, 2026, 10, 22, 7, 59).Revisar(context.Background())

	if avisos := avisosPendientes(notificador); len(avisos) != 0 {
		t.Fatalf("a las 7:59 no debía avisar, salieron %d avisos", len(avisos))
	}
	if repo.marcados["ev-1|2026-10-22"] {
		t.Fatal("antes de la hora no debía marcar el evento como recordado")
	}
}

func TestRecordatorio_AvisaALosInscritosUnaSolaVez(t *testing.T) {
	repo := nuevoRepoRecordatorio()
	sede := "Hotel Movich, Bogotá"
	repo.eventos["2026-10-22"] = []*domain.EventoPorRecordar{{ID: "ev-1", Title: "Masterclass", Location: &sede}}
	repo.inscritos["ev-1"] = []string{"u-1", "u-2"}
	notificador := nuevoNotificadorChat(nil)
	s := recordatorioA(repo, notificador, 2026, 10, 22, 8, 0)

	s.Revisar(context.Background())
	avisos := avisosPendientes(notificador)
	if len(avisos) != 2 {
		t.Fatalf("esperaba un aviso por inscrito (2), salieron %d", len(avisos))
	}
	a := avisos[0]
	if a.titulo != "Hoy: Masterclass" {
		t.Errorf("título inesperado: %q", a.titulo)
	}
	if a.cuerpo != "Tu evento es hoy. Te esperamos en Hotel Movich, Bogotá." {
		t.Errorf("cuerpo inesperado: %q", a.cuerpo)
	}
	if a.datos["type"] != "event" || a.datos["id"] != "ev-1" {
		t.Errorf("la app no podría abrir el evento con estos datos: %v", a.datos)
	}

	// La siguiente vuelta —o un reinicio— no repite el aviso.
	s.Revisar(context.Background())
	if avisos := avisosPendientes(notificador); len(avisos) != 0 {
		t.Fatalf("la segunda vuelta no debía avisar, salieron %d avisos", len(avisos))
	}
}

func TestRecordatorio_UsaElDiaDeBogotaNoElDeUTC(t *testing.T) {
	// 21-10 a las 20:00 en Bogotá ya es 22-10 en UTC: el evento del 22 no
	// debe avisarse la noche anterior.
	repo := nuevoRepoRecordatorio()
	notificador := nuevoNotificadorChat(nil)

	recordatorioA(repo, notificador, 2026, 10, 21, 20, 0).Revisar(context.Background())

	if len(repo.diasLeido) != 1 || repo.diasLeido[0] != "2026-10-21" {
		t.Fatalf("debía buscar los eventos del 2026-10-21 (Bogotá), buscó %v", repo.diasLeido)
	}
}

func TestRecordatorio_TextoSegunElTipoDeEvento(t *testing.T) {
	vacia := "  "
	casos := []struct {
		evento *domain.EventoPorRecordar
		cuerpo string
	}{
		{&domain.EventoPorRecordar{Title: "Webinar", IsVirtual: true}, "Tu evento es hoy. Entra a la app para conectarte."},
		{&domain.EventoPorRecordar{Title: "Sin sede", Location: &vacia}, "Tu evento es hoy. Revisa los detalles en la app."},
		{&domain.EventoPorRecordar{Title: "Sin sede"}, "Tu evento es hoy. Revisa los detalles en la app."},
	}
	for _, c := range casos {
		if _, cuerpo := textoRecordatorio(c.evento); cuerpo != c.cuerpo {
			t.Errorf("%s: cuerpo %q, esperaba %q", c.evento.Title, cuerpo, c.cuerpo)
		}
	}
}
