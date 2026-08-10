package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"applegacy/backend/internal/security"
	"context"
	"errors"
	"strings"
)

type BlockService struct {
	repo   ports.BlockRepository
	crypto *security.CryptoService
}

func NewBlockService(repo ports.BlockRepository, crypto *security.CryptoService) *BlockService {
	return &BlockService{repo: repo, crypto: crypto}
}

func (s *BlockService) BlockUser(ctx context.Context, blockerID, blockedID string) error {
	if blockerID == "" || blockedID == "" {
		return errors.New("faltan datos para bloquear")
	}
	// Bloquearse a uno mismo dejaría a esa cuenta invisible para sí misma en
	// todas las consultas que filtran por bloqueo.
	if blockerID == blockedID {
		return errors.New("no puedes bloquearte a ti mismo")
	}
	return s.repo.Block(ctx, blockerID, blockedID)
}

func (s *BlockService) UnblockUser(ctx context.Context, blockerID, blockedID string) error {
	if blockerID == "" || blockedID == "" {
		return errors.New("faltan datos para desbloquear")
	}
	return s.repo.Unblock(ctx, blockerID, blockedID)
}

func (s *BlockService) ListBlocked(ctx context.Context, blockerID string) ([]*domain.BlockedUser, error) {
	bloqueados, err := s.repo.ListBlocked(ctx, blockerID)
	if err != nil {
		return nil, err
	}
	// Los nombres se guardan cifrados. Se descifran aquí con el patrón del
	// proyecto: si falla, se conserva el valor tal cual en vez de romper la
	// lista entera por un registro.
	for _, b := range bloqueados {
		if dec, err := s.crypto.Decrypt(b.FirstName); err == nil {
			b.FirstName = dec
		}
		if dec, err := s.crypto.Decrypt(b.LastName); err == nil {
			b.LastName = dec
		}
	}
	return bloqueados, nil
}

func (s *BlockService) ReportUser(ctx context.Context, reporterID, reportedID, reason string, messageID *string) error {
	if reporterID == "" || reportedID == "" {
		return errors.New("faltan datos para reportar")
	}
	if reporterID == reportedID {
		return errors.New("no puedes reportarte a ti mismo")
	}
	// Un reporte sin motivo no le sirve a quien tiene que revisarlo.
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return errors.New("el motivo del reporte es obligatorio")
	}

	report := &domain.UserReport{
		ReporterID: reporterID,
		ReportedID: reportedID,
		MessageID:  messageID,
		Reason:     reason,
	}
	return s.repo.Report(ctx, report)
}

func (s *BlockService) ListReports(ctx context.Context, status string) ([]*domain.UserReport, error) {
	reportes, err := s.repo.ListReports(ctx, status)
	if err != nil {
		return nil, err
	}
	for _, r := range reportes {
		r.ReporterName = s.nombreDe(r.ReporterFirstName, r.ReporterLastName)
		r.ReportedName = s.nombreDe(r.ReportedFirstName, r.ReportedLastName)
	}
	return reportes, nil
}

// nombreDe descifra y compone. Si el descifrado falla se conserva el valor, que
// es el patrón del proyecto; si no queda nada legible se devuelve un texto
// neutro en vez de una fila en blanco que nadie sabría identificar.
func (s *BlockService) nombreDe(nombre, apellido string) string {
	if s.crypto != nil {
		if dec, err := s.crypto.Decrypt(nombre); err == nil {
			nombre = dec
		}
		if dec, err := s.crypto.Decrypt(apellido); err == nil {
			apellido = dec
		}
	}
	completo := strings.TrimSpace(nombre + " " + apellido)
	if completo == "" {
		return "Usuario"
	}
	return completo
}

func (s *BlockService) ResolveReport(ctx context.Context, reportID, status string) error {
	if reportID == "" {
		return errors.New("falta el reporte")
	}
	switch status {
	case domain.ReportStatusPending, domain.ReportStatusReviewed, domain.ReportStatusDismissed:
	default:
		return errors.New("estado de reporte no válido")
	}
	return s.repo.UpdateReportStatus(ctx, reportID, status)
}
