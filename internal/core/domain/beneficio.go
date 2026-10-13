package domain

import "time"

// Beneficio es una tarjeta de la sección Beneficios de la app, solo para
// usuarios Legacy+: un beneficio o una alianza, cada tipo en su propio slider.
// Se administra desde el panel desde el 2026-10-13.
type Beneficio struct {
	ID string `json:"id" db:"id"`
	// Tipo es uno de TiposBeneficio: en qué slider sale.
	Tipo        string `json:"tipo" db:"tipo"`
	Titulo      string `json:"titulo" db:"titulo"`
	Descripcion string `json:"descripcion" db:"descripcion"`
	// ImagenURL y EnlaceURL vacíos: la tarjeta va sin imagen o sin botón.
	ImagenURL     string    `json:"imagen_url" db:"imagen_url"`
	EnlaceURL     string    `json:"enlace_url" db:"enlace_url"`
	Orden         int       `json:"orden" db:"orden"`
	Activo        bool      `json:"activo" db:"activo"`
	CreadoEn      time.Time `json:"creado_en" db:"creado_en"`
	ActualizadoEn time.Time `json:"actualizado_en" db:"actualizado_en"`
}

// TiposBeneficio: los dos sliders de la sección Beneficios.
var TiposBeneficio = []string{"beneficio", "alianza"}
