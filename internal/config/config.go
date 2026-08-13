package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Port string `yaml:"port"`
		Env  string `yaml:"env"`
	} `yaml:"server"`

	Database struct {
		DSN string `yaml:"dsn"`
	} `yaml:"database"`

	Security struct {
		EncryptionKey string `yaml:"encryption_key"`
		JWTSecret     string `yaml:"jwt_secret"`
	} `yaml:"security"`

	Email struct {
		SMTPHost             string `yaml:"smtp_host"`
		SMTPPort             int    `yaml:"smtp_port"`
		Username             string `yaml:"username"`
		Password             string `yaml:"password"`
		From                 string `yaml:"from"`
		GmailCredentialsFile string `yaml:"gmail_credentials_file"`
		GmailImpersonateUser string `yaml:"gmail_impersonate_user"`
	} `yaml:"email"`

	WebApp struct {
		ResetPasswordURL string `yaml:"reset_password_url"`
		VerifyEmailURL   string `yaml:"verify_email_url"`
	} `yaml:"web_app"`

	Firebase struct {
		CredentialsFile string `yaml:"credentials_file"`
		GoogleClientID  string `yaml:"google_client_id"`
	} `yaml:"firebase"`

	// Apple es lo necesario para verificar Sign in with Apple. El bundle_id es
	// la audiencia que debe traer el token: sin él no se puede distinguir un
	// token emitido para esta app del de cualquier otra.
	Apple struct {
		BundleID string `yaml:"bundle_id"`
	} `yaml:"apple"`

	CredibanCo struct {
		BaseURL  string `yaml:"base_url"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
		Terminal string `yaml:"terminal"`
		Merchant string `yaml:"merchant"`

		// Simulado sustituye la pasarela por una de mentira que no cobra nada y
		// deja elegir el desenlace. Existe porque CredibanCo lleva bloqueando
		// los pagos con "acceso denegado" desde el 2026-08-06 y sin esto no hay
		// forma de probar el flujo completo: intención, retorno a la app,
		// notificación e inscripción confirmada.
		//
		// **Jamás en producción.** Activarlo allí sería regalar inscripciones a
		// quien encuentre la URL. `main.go` se niega a arrancar si esto está
		// encendido apuntando a la pasarela real.
		Simulado bool `yaml:"simulado"`

		// SimuladoBaseURL es la dirección pública de ESTE backend, con la que se
		// construye el enlace de la pasarela de mentira. Vacío equivale a
		// http://localhost:8080. En un emulador Android tiene que ser
		// http://10.0.2.2:8080, y en un teléfono real la IP del equipo en la
		// red local: el navegador que abre el enlace no es el del servidor.
		SimuladoBaseURL string `yaml:"simulado_base_url"`
	} `yaml:"credibanco"`

	// Storage.UploadsDir es donde se guardan las imagenes que suben los foros.
	// Vacio equivale a "uploads" (relativo a Backend/). En produccion tiene que
	// ser un directorio montado como volumen: si vive dentro del contenedor, se
	// pierde entero en cada despliegue.
	Storage struct {
		UploadsDir string `yaml:"uploads_dir"`
	} `yaml:"storage"`

	BoardContacts map[string]string `yaml:"board_contacts"`
	AsesoriaEmail string            `yaml:"asesoria_email"`

	// ContactoEmail es el buzón de la pantalla "Contáctenos". Si falta, se cae
	// a board_contacts["default"] (ver BuzonDeContacto): así un despliegue con
	// una configuración anterior sigue entregando los mensajes en vez de
	// rechazarlos, que es lo que pasaría con el buzón vacío.
	ContactoEmail string `yaml:"contacto_email"`
}

// BuzonDeContacto resuelve a dónde van los mensajes de "Contáctenos".
func (c *Config) BuzonDeContacto() string {
	if c.ContactoEmail != "" {
		return c.ContactoEmail
	}
	return c.BoardContacts["default"]
}

func LoadConfig(path string) (*Config, error) {
	config := &Config{}

	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(file, config)
	if err != nil {
		return nil, err
	}

	return config, nil
}
