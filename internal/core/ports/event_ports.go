package ports

import (
	"applegacy/backend/internal/core/domain"
	"context"
)

type EventRepository interface {
	GetEvents(ctx context.Context) ([]domain.Event, error)
	GetEventByID(ctx context.Context, id string) (*domain.Event, error)
	GetWorkshopsByEventID(ctx context.Context, eventID string) ([]domain.Workshop, error)
	ListCategories(ctx context.Context) ([]domain.EventCategory, error)
	CreateEvent(ctx context.Context, event *domain.Event) error
	UpdateEvent(ctx context.Context, event *domain.Event) error
	DeleteEvent(ctx context.Context, id string) error
	CreateWorkshop(ctx context.Context, workshop *domain.Workshop) error
	DeleteWorkshopsByEventID(ctx context.Context, eventID string) error
	CreateRegistration(ctx context.Context, registration *domain.Registration) error
	AddRegistrationWorkshops(ctx context.Context, registrationID string, workshopIDs []string) error
	GetRegistrationByUserAndEvent(ctx context.Context, userID, eventID string) (*domain.Registration, error)
	CreateWorkshopRating(ctx context.Context, rating *domain.WorkshopRating) error
	GetRatingsByEventID(ctx context.Context, eventID string) ([]domain.WorkshopRating, error)
	GetAgenda(ctx context.Context, userID string) ([]domain.Workshop, error)
	AddToAgenda(ctx context.Context, userID, workshopID string) error
	RemoveFromAgenda(ctx context.Context, userID, workshopID string) error

	// QR & Attendance
	GetRegistrationByQR(ctx context.Context, qrData string) (*domain.Registration, *domain.CheckInResponse, error)
	RecordAttendance(ctx context.Context, registrationID, staffID string) error
	GetWorkshopsByRegistrationID(ctx context.Context, registrationID string) ([]domain.Workshop, error)
}

type EventService interface {
	ListEvents(ctx context.Context) ([]domain.Event, error)
	GetEventDetails(ctx context.Context, id string) (*domain.Event, error)
	ListCategories(ctx context.Context) ([]domain.EventCategory, error)
	CreateEvent(ctx context.Context, event *domain.Event) error
	UpdateEvent(ctx context.Context, event *domain.Event) error
	DeleteEvent(ctx context.Context, id string) error
	RegisterUser(ctx context.Context, reg *domain.Registration) error
	SubmitWorkshopRating(ctx context.Context, rating *domain.WorkshopRating) error
	GetEventFeedback(ctx context.Context, eventID string) ([]domain.WorkshopRating, error)
	GetAgenda(ctx context.Context, userID string) ([]domain.Workshop, error)
	AddToAgenda(ctx context.Context, userID, workshopID string) error
	RemoveFromAgenda(ctx context.Context, userID, workshopID string) error

	// QR & Attendance
	CheckIn(ctx context.Context, qrData, staffID string) (*domain.CheckInResponse, error)
}
