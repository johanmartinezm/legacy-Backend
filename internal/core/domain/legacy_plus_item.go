package domain

import "time"

// LegacyPlusItem es un punto con check de la pantalla Legacy+ de la app.
// Administrable desde el panel desde el 2026-10-13.
type LegacyPlusItem struct {
	ID string `json:"id" db:"id"`
	// Grupo es uno de GruposLegacyPlus: dónde va el punto en la pantalla.
	Grupo         string    `json:"grupo" db:"grupo"`
	Titulo        string    `json:"titulo" db:"titulo"`
	Texto         string    `json:"texto" db:"texto"`
	Orden         int       `json:"orden" db:"orden"`
	Activo        bool      `json:"activo" db:"activo"`
	CreadoEn      time.Time `json:"creado_en" db:"creado_en"`
	ActualizadoEn time.Time `json:"actualizado_en" db:"actualizado_en"`
}

// GruposLegacyPlus: «incluye» va bajo «Tiene acceso a:» y «beneficios» bajo el
// subtítulo de beneficios.
var GruposLegacyPlus = []string{"incluye", "beneficios"}
