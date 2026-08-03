package services

import (
	"applegacy/backend/internal/core/ports"
	"context"
	"errors"
	"fmt"
)

type asesoriaService struct {
	emailService  ports.EmailService
	asesoriaEmail string
}

func NewAsesoriaService(emailService ports.EmailService, email string) ports.AsesoriaService {
	return &asesoriaService{
		emailService:  emailService,
		asesoriaEmail: email,
	}
}

func (s *asesoriaService) RequestAsesoria(ctx context.Context, category, senderName, senderEmail, message string) error {
	if s.asesoriaEmail == "" {
		return errors.New("no destination email configured for asesoria")
	}

	fmt.Printf("Sending asesoria request to %s for category %s\n", s.asesoriaEmail, category)
	return s.emailService.SendAsesoriaEmail(s.asesoriaEmail, senderName, senderEmail, category, message)
}
