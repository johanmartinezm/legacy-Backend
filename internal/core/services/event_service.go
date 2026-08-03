package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
	"fmt"
)

type EventService struct {
	repo ports.EventRepository
}

func NewEventService(repo ports.EventRepository) *EventService {
	return &EventService{repo: repo}
}

func (s *EventService) ListCategories(ctx context.Context) ([]domain.EventCategory, error) {
	return s.repo.ListCategories(ctx)
}

func (s *EventService) ListEvents(ctx context.Context) ([]domain.Event, error) {
	events, err := s.repo.GetEvents(ctx)
	if err != nil {
		return nil, err
	}
	for i := range events {
		s.populatePriceLabel(&events[i])
		workshops, err := s.repo.GetWorkshopsByEventID(ctx, events[i].ID)
		if err == nil {
			events[i].Workshops = workshops
		}
	}
	return events, nil
}

func (s *EventService) GetEventDetails(ctx context.Context, id string) (*domain.Event, error) {
	event, err := s.repo.GetEventByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.populatePriceLabel(event)

	workshops, err := s.repo.GetWorkshopsByEventID(ctx, id)
	if err != nil {
		return nil, err
	}
	event.Workshops = workshops

	return event, nil
}

func (s *EventService) CreateEvent(ctx context.Context, e *domain.Event) error {
	err := s.repo.CreateEvent(ctx, e)
	if err != nil {
		return err
	}

	for i := range e.Workshops {
		e.Workshops[i].EventID = e.ID
		err = s.repo.CreateWorkshop(ctx, &e.Workshops[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *EventService) UpdateEvent(ctx context.Context, e *domain.Event) error {
	err := s.repo.UpdateEvent(ctx, e)
	if err != nil {
		return err
	}

	// Simple workshop sync: delete and recreate
	err = s.repo.DeleteWorkshopsByEventID(ctx, e.ID)
	if err != nil {
		return err
	}

	for i := range e.Workshops {
		e.Workshops[i].EventID = e.ID
		err = s.repo.CreateWorkshop(ctx, &e.Workshops[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *EventService) DeleteEvent(ctx context.Context, id string) error {
	// Workshops will be deleted via cascade if set up, or explicitly here
	_ = s.repo.DeleteWorkshopsByEventID(ctx, id)
	return s.repo.DeleteEvent(ctx, id)
}

func (s *EventService) RegisterUser(ctx context.Context, reg *domain.Registration) error {
	// 1. Check if already registered
	existing, _ := s.repo.GetRegistrationByUserAndEvent(ctx, reg.UserID, reg.EventID)
	if existing != nil {
		reg.ID = existing.ID
		reg.PaymentStatus = existing.PaymentStatus
		reg.RegistrationDate = existing.RegistrationDate
		reg.QRData = existing.QRData
		reg.TotalPaid = existing.TotalPaid
		reg.AttendanceConfirmed = existing.AttendanceConfirmed
		return nil
	}

	// 2. Fetch event to check price and free status
	event, err := s.repo.GetEventByID(ctx, reg.EventID)
	if err != nil {
		return err
	}

	// 3. Set payment status and total if not already set (e.g., from an admin override)
	if reg.PaymentStatus == "" {
		reg.PaymentStatus = "pending"
		if event.IsFree {
			reg.PaymentStatus = "free"
		}
	}

	if reg.TotalPaid == 0 && !event.IsFree {
		reg.TotalPaid = event.Price
	}

	if reg.QRData == "" {
		reg.QRData = fmt.Sprintf("REG-%s-%s", reg.UserID, reg.EventID)
	}

	// 4. Create Registration in repository
	err = s.repo.CreateRegistration(ctx, reg)
	if err != nil {
		return err
	}

	// 5. Add workshops if any
	if len(reg.Workshops) > 0 {
		var workshopIDs []string
		for _, w := range reg.Workshops {
			workshopIDs = append(workshopIDs, w.ID)
		}
		return s.repo.AddRegistrationWorkshops(ctx, reg.ID, workshopIDs)
	}

	return nil
}

func (s *EventService) SubmitWorkshopRating(ctx context.Context, rating *domain.WorkshopRating) error {
	return s.repo.CreateWorkshopRating(ctx, rating)
}

func (s *EventService) GetEventFeedback(ctx context.Context, eventID string) ([]domain.WorkshopRating, error) {
	return s.repo.GetRatingsByEventID(ctx, eventID)
}

func (s *EventService) populatePriceLabel(e *domain.Event) {
	if e.IsFree {
		e.PriceLabel = "GRATIS"
	} else {
		e.PriceLabel = fmt.Sprintf("$%.0f.000.000", e.Price)
	}
}

func (s *EventService) GetAgenda(ctx context.Context, userID string) ([]domain.Workshop, error) {
	return s.repo.GetAgenda(ctx, userID)
}

func (s *EventService) AddToAgenda(ctx context.Context, userID, workshopID string) error {
	return s.repo.AddToAgenda(ctx, userID, workshopID)
}

func (s *EventService) RemoveFromAgenda(ctx context.Context, userID, workshopID string) error {
	return s.repo.RemoveFromAgenda(ctx, userID, workshopID)
}

func (s *EventService) CheckIn(ctx context.Context, qrData, staffID string) (*domain.CheckInResponse, error) {
	// 1. Find registration by QR
	reg, resp, err := s.repo.GetRegistrationByQR(ctx, qrData)
	if err != nil {
		return nil, fmt.Errorf("invalid QR data or registration not found")
	}

	// 2. Record attendance (updates confirmation and logs)
	err = s.repo.RecordAttendance(ctx, reg.ID, staffID)
	if err != nil {
		return nil, fmt.Errorf("failed to record attendance: %v", err)
	}

	// 3. Get workshops for this registration
	workshops, err := s.repo.GetWorkshopsByRegistrationID(ctx, reg.ID)
	if err == nil {
		resp.Workshops = workshops
	}

	return resp, nil
}
