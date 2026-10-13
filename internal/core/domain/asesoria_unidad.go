package domain

import "time"

// AsesoriaUnidad es una de las unidades que la pantalla Asesoría de la app
// lista bajo «NUESTRAS UNIDADES» (L&M Consultoría, Aurum…). Se administran
// desde el panel desde el 2026-10-13; antes estaban escritas en la app.
type AsesoriaUnidad struct {
	ID        string `json:"id" db:"id"`
	Titulo    string `json:"titulo" db:"titulo"`
	Subtitulo string `json:"subtitulo" db:"subtitulo"`
	// Icono es una clave de IconosAsesoria; la app la traduce a un icono.
	Icono string `json:"icono" db:"icono"`
	// Necesidad es la opción de «¿Qué necesita?» que el formulario marca al
	// entrar por esta unidad. Una de NecesidadesAsesoria.
	Necesidad     string    `json:"necesidad" db:"necesidad"`
	Orden         int       `json:"orden" db:"orden"`
	Activa        bool      `json:"activa" db:"activa"`
	CreadaEn      time.Time `json:"creada_en" db:"creada_en"`
	ActualizadaEn time.Time `json:"actualizada_en" db:"actualizada_en"`
}

// NecesidadesAsesoria son las opciones de «¿Qué necesita?» del formulario de
// la app (asesoria_screen.dart). Si la app añade una, hay que añadirla aquí.
var NecesidadesAsesoria = []string{
	"Gobierno", "Sucesión", "Patrimonio", "Legal/Tributario", "Certificación", "Otro",
}

// IconosAsesoria son las claves de icono que la app sabe pintar. Una clave
// desconocida la app la pinta con el icono genérico, pero el servicio no la
// deja guardar.
var IconosAsesoria = []string{
	"escudo", "crecimiento", "balanza", "formacion", "grupo",
	"negocio", "familia", "banco", "apreton", "mazo",
}
