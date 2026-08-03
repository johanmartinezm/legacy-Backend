package domain

import (
	"time"
)

type EventCategory struct {
	ID          string    `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	OrderIndex  int       `json:"orderIndex" db:"order_index"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

type Event struct {
	ID             string     `json:"id" db:"id"`
	CategoryID     string     `json:"category_id" db:"category_id"`
	Category       string     `json:"category" db:"-"` // Added for frontend convenience
	Title          string     `json:"title" db:"title"`
	Description    *string    `json:"description" db:"description"`
	ImageUrl       *string    `json:"imageUrl" db:"image_url"`
	Location       *string    `json:"location" db:"location"`
	SpeakerMain    *string    `json:"speaker" db:"speaker_main"`
	StartDate      time.Time  `json:"date" db:"start_date"`
	EndDate        *time.Time `json:"end_date" db:"end_date"`
	Price          float64    `json:"price" db:"price"`
	PriceLabel     string     `json:"priceLabel" db:"-"`
	IsFree         bool       `json:"isFree" db:"is_free"`
	ActionStatus   string     `json:"actionStatus" db:"action_status"`
	ButtonText     string     `json:"buttonText" db:"button_text"`
	AttendeesLimit *int       `json:"attendees_limit" db:"attendees_limit"`
	Includes       *string    `json:"includes" db:"includes"`
	CategoryOrder  int        `json:"categoryOrder" db:"-"`
	Workshops      []Workshop `json:"workshops" db:"-"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}

type Workshop struct {
	ID            string    `json:"id" db:"id"`
	EventID       string    `json:"eventId" db:"event_id"`
	EventTitle    string    `json:"eventTitle" db:"-"`
	Name          string    `json:"name" db:"name"`
	Description   string    `json:"description" db:"description"`
	Room          string    `json:"room" db:"room"`
	Speaker       string    `json:"speaker" db:"speaker"`
	ImageUrl      string    `json:"imageUrl" db:"image_url"`
	StartDateTime time.Time `json:"startDateTime" db:"start_date_time"`
	EndDateTime   time.Time `json:"endDateTime" db:"end_date_time"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

type Registration struct {
	ID                  string     `json:"id" db:"id"`
	UserID              string     `json:"user_id" db:"user_id"`
	EventID             string     `json:"event_id" db:"event_id"`
	PaymentStatus       string     `json:"payment_status" db:"payment_status"`
	RegistrationDate    time.Time  `json:"registration_date" db:"registration_date"`
	QRData              string     `json:"qr_data" db:"qr_data"`
	TotalPaid           float64    `json:"total_paid" db:"total_paid"`
	AttendanceConfirmed bool       `json:"attendance_confirmed" db:"attendance_confirmed"`
	Workshops           []Workshop `json:"workshops" db:"-"`
}

type WorkshopRating struct {
	ID           string    `json:"id" db:"id"`
	WorkshopID   string    `json:"workshopId" db:"workshop_id"`
	WorkshopName string    `json:"workshopName" db:"-"`
	UserID       string    `json:"userId" db:"user_id"`
	Rating       int       `json:"rating" db:"rating"`
	Comment      string    `json:"comment" db:"comment"`
	CreatedAt    time.Time `json:"createdAt" db:"created_at"`
}

type CheckInResponse struct {
	RegistrationID string     `json:"registrationId"`
	UserName       string     `json:"userName"`
	UserEmail      string     `json:"userEmail"`
	EventTitle     string     `json:"eventTitle"`
	CheckInTime    time.Time  `json:"checkInTime"`
	Workshops      []Workshop `json:"workshops"`
}
