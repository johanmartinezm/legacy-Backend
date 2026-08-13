package domain

import "time"

// UserBlock es un bloqueo de una persona sobre otra.
//
// El bloqueo se guarda dirigido —quién bloqueó a quién— pero se aplica en las
// dos direcciones: si A bloquea a B, ninguno ve al otro ni puede escribirle. Un
// bloqueo que dejara al bloqueado seguir enviando mensajes no protegería de
// nada, y uno que solo escondiera a A de B dejaría a A leyendo a quien acaba de
// bloquear.
type UserBlock struct {
	ID        string    `json:"id"`
	BlockerID string    `json:"blocker_id"`
	BlockedID string    `json:"blocked_id"`
	CreatedAt time.Time `json:"created_at"`
}

// BlockedUser es una entrada de la lista de bloqueados, con lo justo para
// mostrarla y poder desbloquear.
type BlockedUser struct {
	UserID          string    `json:"user_id"`
	FirstName       string    `json:"first_name"`
	LastName        string    `json:"last_name"`
	Alias           *string   `json:"alias"`
	ProfileImageUrl string    `json:"profile_image_url"`
	BlockedAt       time.Time `json:"blocked_at"`
}

// UserReport es la denuncia de una persona sobre otra. MessageID es opcional:
// se puede reportar desde un chat señalando el mensaje, o desde un perfil sin
// señalar nada.
type UserReport struct {
	ID         string    `json:"id"`
	ReporterID string    `json:"reporter_id"`
	ReportedID string    `json:"reported_id"`
	MessageID  *string   `json:"message_id"`
	Reason     string    `json:"reason"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`

	// Nombres de las dos personas, para la bandeja del panel administrativo.
	// Van aquí y no los resuelve el panel porque están cifrados en la base: solo
	// el backend tiene la clave. Una bandeja que mostrara UUIDs no serviría para
	// decidir nada.
	ReporterName string `json:"reporter_name"`
	ReportedName string `json:"reported_name"`

	// Nombre y apellido llegan cifrados por separado del repositorio y no se
	// pueden concatenar antes de descifrar. El servicio los descifra y compone
	// los dos campos de arriba; no salen en el JSON.
	ReporterFirstName string `json:"-"`
	ReporterLastName  string `json:"-"`
	ReportedFirstName string `json:"-"`
	ReportedLastName  string `json:"-"`
}

// Estados de un reporte en la bandeja del panel administrativo.
const (
	ReportStatusPending   = "pending"
	ReportStatusReviewed  = "reviewed"
	ReportStatusDismissed = "dismissed"
)
