package domain

import "time"

// Estados por los que pasa un mensaje de "Contáctenos" en la bandeja del panel.
const (
	ContactoNuevo      = "nuevo"
	ContactoLeido      = "leido"
	ContactoRespondido = "respondido"
)

// EstadosDeContactoValidos son los únicos que acepta el servicio. Un estado
// inventado dejaría el mensaje fuera de todos los filtros de la bandeja, que es
// una forma silenciosa de perderlo.
var EstadosDeContactoValidos = []string{ContactoNuevo, ContactoLeido, ContactoRespondido}

// MensajeDeContacto es lo que escribe una persona desde la app.
//
// Asunto y Mensaje viajan descifrados en esta estructura: el cifrado ocurre al
// escribir en la base y el descifrado al leerla, ambos dentro del servicio.
type MensajeDeContacto struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Asunto       string    `json:"asunto"`
	Mensaje      string    `json:"mensaje"`
	Estado       string    `json:"estado"`
	EmailEnviado bool      `json:"email_enviado"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Datos de quien escribió, para que la bandeja no tenga que pedirlos aparte.
	// Se rellenan al listar, desde core.users.
	//
	// Nombre y apellido van separados porque en core.users están cifrados por
	// separado: unirlos antes de descifrar produce una cadena que ya no se puede
	// descifrar.
	RemitenteNombre   string `json:"remitente_nombre,omitempty"`
	RemitenteApellido string `json:"remitente_apellido,omitempty"`
	RemitenteEmail    string `json:"remitente_email,omitempty"`
}
