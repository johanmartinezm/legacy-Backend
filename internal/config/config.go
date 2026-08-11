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

	CredibanCo struct {
		BaseURL  string `yaml:"base_url"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
		Terminal string `yaml:"terminal"`
		Merchant string `yaml:"merchant"`
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
