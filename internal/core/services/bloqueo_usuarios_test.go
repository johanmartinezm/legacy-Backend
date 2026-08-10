package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"testing"
)

// Lo que se prueba aquí es que el bloqueo CORTA, no solo que se guarda. Un
// bloqueo que se registrara pero dejara pasar los mensajes no protegería de
// nada, y es exactamente lo que la directriz 1.2 de Apple mira.

type repoBloqueos struct {
	bloqueados map[string]bool // clave "a|b"
	bloqueoDe  []string        // a quién se pidió bloquear
	reportes   []*domain.UserReport
}

func nuevoRepoBloqueos() *repoBloqueos {
	return &repoBloqueos{bloqueados: map[string]bool{}}
}

func (r *repoBloqueos) Block(ctx context.Context, blockerID, blockedID string) error {
	r.bloqueoDe = append(r.bloqueoDe, blockerID+"->"+blockedID)
	r.bloqueados[blockerID+"|"+blockedID] = true
	return nil
}
func (r *repoBloqueos) Unblock(ctx context.Context, blockerID, blockedID string) error {
	delete(r.bloqueados, blockerID+"|"+blockedID)
	return nil
}
func (r *repoBloqueos) ListBlocked(ctx context.Context, blockerID string) ([]*domain.BlockedUser, error) {
	return nil, nil
}
func (r *repoBloqueos) AreBlocked(ctx context.Context, a, b string) (bool, error) {
	return r.bloqueados[a+"|"+b] || r.bloqueados[b+"|"+a], nil
}
func (r *repoBloqueos) Report(ctx context.Context, rep *domain.UserReport) error {
	r.reportes = append(r.reportes, rep)
	return nil
}
func (r *repoBloqueos) ListReports(ctx context.Context, status string) ([]*domain.UserReport, error) {
	return r.reportes, nil
}
func (r *repoBloqueos) UpdateReportStatus(ctx context.Context, reportID, status string) error {
	return nil
}

// repoChatMinimo solo responde lo que necesitan los guardas.
type repoChatMinimo struct {
	conexion   *domain.ChatConnection
	creadas    int
	guardados  int
	existente  *domain.ChatConnection
}

func (r *repoChatMinimo) CreateConnection(ctx context.Context, requesterID, receiverID string) error {
	r.creadas++
	return nil
}
func (r *repoChatMinimo) UpdateConnectionStatus(ctx context.Context, id string, s domain.ConnectionStatus) error {
	return nil
}
func (r *repoChatMinimo) GetConnection(ctx context.Context, id string) (*domain.ChatConnection, error) {
	return r.conexion, nil
}
func (r *repoChatMinimo) FindConnectionBetweenUsers(ctx context.Context, a, b string) (*domain.ChatConnection, error) {
	return r.existente, nil
}
func (r *repoChatMinimo) ListConnections(ctx context.Context, userID string) ([]*domain.ChatConnection, error) {
	return nil, nil
}
func (r *repoChatMinimo) SaveMessage(ctx context.Context, m *domain.Message) error {
	r.guardados++
	return nil
}
func (r *repoChatMinimo) GetMessages(ctx context.Context, id string, limit, offset int) ([]*domain.Message, error) {
	return []*domain.Message{}, nil
}
func (r *repoChatMinimo) MarkAsRead(ctx context.Context, id, userID string) error { return nil }
func (r *repoChatMinimo) ListMembers(ctx context.Context, viewerID string) ([]*domain.User, error) {
	return nil, nil
}

func conexionAceptada() *domain.ChatConnection {
	return &domain.ChatConnection{
		ID:          "conn-1",
		RequesterID: "ana",
		ReceiverID:  "beto",
		Status:      domain.StatusAccepted,
	}
}

func TestBloqueo_NoSePuedeEnviarMensaje(t *testing.T) {
	bloqueos := nuevoRepoBloqueos()
	_ = bloqueos.Block(context.Background(), "ana", "beto")
	chat := &repoChatMinimo{conexion: conexionAceptada()}
	svc := &ChatService{repo: chat, blockRepo: bloqueos}

	_, err := svc.SendMessage(context.Background(), "beto", "conn-1", "hola")
	if err == nil {
		t.Fatal("quien está bloqueado no debe poder escribir")
	}
	if chat.guardados != 0 {
		t.Errorf("no debe guardarse ningún mensaje, se guardaron %d", chat.guardados)
	}
}

func TestBloqueo_TampocoPuedeEscribirQuienBloqueo(t *testing.T) {
	// El bloqueo es simétrico en sus efectos: quien bloquea tampoco sigue
	// escribiendo. Si no, la conversación quedaría medio viva y confundiría a
	// las dos partes.
	bloqueos := nuevoRepoBloqueos()
	_ = bloqueos.Block(context.Background(), "ana", "beto")
	chat := &repoChatMinimo{conexion: conexionAceptada()}
	svc := &ChatService{repo: chat, blockRepo: bloqueos}

	if _, err := svc.SendMessage(context.Background(), "ana", "conn-1", "hola"); err == nil {
		t.Error("quien bloqueó tampoco debe poder escribir")
	}
}

func TestBloqueo_NoSePuedeLeerElHistorial(t *testing.T) {
	// Esconder la conversación de la lista no basta: el connectionID sigue
	// siendo válido y se podría pedir directamente.
	bloqueos := nuevoRepoBloqueos()
	_ = bloqueos.Block(context.Background(), "ana", "beto")
	chat := &repoChatMinimo{conexion: conexionAceptada()}
	svc := &ChatService{repo: chat, blockRepo: bloqueos}

	if _, err := svc.GetChatHistory(context.Background(), "conn-1", "beto", 50, 0); err == nil {
		t.Error("no debe poder leerse el historial de una conversación bloqueada")
	}
}

func TestBloqueo_NoSePuedeInvitar(t *testing.T) {
	bloqueos := nuevoRepoBloqueos()
	_ = bloqueos.Block(context.Background(), "ana", "beto")
	chat := &repoChatMinimo{}
	svc := &ChatService{repo: chat, blockRepo: bloqueos}

	if err := svc.SendInvite(context.Background(), "beto", "ana"); err == nil {
		t.Error("no debe poder invitarse a alguien con quien hay bloqueo")
	}
	if chat.creadas != 0 {
		t.Errorf("no debe crearse ninguna conexión, se crearon %d", chat.creadas)
	}
}

func TestSinBloqueo_ElChatFuncionaComoSiempre(t *testing.T) {
	// La otra mitad: el filtro no debe romper el uso normal.
	bloqueos := nuevoRepoBloqueos()
	chat := &repoChatMinimo{}
	svc := &ChatService{repo: chat, blockRepo: bloqueos}

	if err := svc.SendInvite(context.Background(), "ana", "beto"); err != nil {
		t.Errorf("sin bloqueo la invitación debe pasar: %v", err)
	}
	if chat.creadas != 1 {
		t.Errorf("debe crearse la conexión, se crearon %d", chat.creadas)
	}
}

func TestBloqueo_NoSePuedeUnoBloquearseASiMismo(t *testing.T) {
	// Bloquearse a uno mismo dejaría esa cuenta invisible para sí misma en
	// todas las consultas que filtran por bloqueo.
	bloqueos := nuevoRepoBloqueos()
	svc := NewBlockService(bloqueos, nil)

	if err := svc.BlockUser(context.Background(), "ana", "ana"); err == nil {
		t.Error("debe rechazarse el autobloqueo")
	}
	if len(bloqueos.bloqueoDe) != 0 {
		t.Errorf("no debe llegar al repositorio: %v", bloqueos.bloqueoDe)
	}
}

func TestReporte_ExigeMotivo(t *testing.T) {
	// Un reporte sin motivo no le sirve a quien tiene que revisarlo.
	bloqueos := nuevoRepoBloqueos()
	svc := NewBlockService(bloqueos, nil)

	if err := svc.ReportUser(context.Background(), "ana", "beto", "   ", nil); err == nil {
		t.Error("un motivo en blanco debe rechazarse")
	}
	if len(bloqueos.reportes) != 0 {
		t.Error("no debe guardarse un reporte sin motivo")
	}
}

func TestReporte_SeGuardaConSuMotivo(t *testing.T) {
	bloqueos := nuevoRepoBloqueos()
	svc := NewBlockService(bloqueos, nil)

	if err := svc.ReportUser(context.Background(), "ana", "beto", "  mensajes ofensivos  ", nil); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(bloqueos.reportes) != 1 {
		t.Fatalf("debe guardarse un reporte, hay %d", len(bloqueos.reportes))
	}
	rep := bloqueos.reportes[0]
	if rep.Reason != "mensajes ofensivos" {
		t.Errorf("el motivo debe llegar sin espacios sobrantes, llegó %q", rep.Reason)
	}
	if rep.ReporterID != "ana" || rep.ReportedID != "beto" {
		t.Errorf("reportante y reportado mal asignados: %s -> %s", rep.ReporterID, rep.ReportedID)
	}
}

func TestReporte_EstadoInvalidoSeRechaza(t *testing.T) {
	bloqueos := nuevoRepoBloqueos()
	svc := NewBlockService(bloqueos, nil)

	if err := svc.ResolveReport(context.Background(), "rep-1", "inventado"); err == nil {
		t.Error("un estado fuera de los previstos debe rechazarse")
	}
	if err := svc.ResolveReport(context.Background(), "rep-1", domain.ReportStatusReviewed); err != nil {
		t.Errorf("un estado válido debe aceptarse: %v", err)
	}
}
