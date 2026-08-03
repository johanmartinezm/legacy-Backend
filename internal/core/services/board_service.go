package services

import (
	"applegacy/backend/internal/core/ports"
	"context"
	"errors"
	"fmt"
)

type boardService struct {
	emailService  ports.EmailService
	boardContacts map[string]string
}

func NewBoardService(emailService ports.EmailService, contacts map[string]string) ports.BoardService {
	return &boardService{
		emailService:  emailService,
		boardContacts: contacts,
	}
}

func (s *boardService) NotifyContact(ctx context.Context, contactID, senderName, senderEmail, message string) error {
	email, ok := s.boardContacts[contactID]
	if !ok {
		// Try default
		email, ok = s.boardContacts["default"]
		if !ok {
			return errors.New("no destination email configured for this contact")
		}
	}

	fmt.Printf("Sending board notification to %s for contactID %s\n", email, contactID)
	return s.emailService.SendBoardContactEmail(email, senderName, senderEmail, message)
}
