package domain

import "time"

// PaginaInformativa es una página de solo lectura que el panel edita y la app
// pinta tal cual. No tiene acciones ni formularios: sirve para contar algo que
// cambia con el tiempo sin tener que publicar una versión nueva de la app.
//
// La identifica el `slug`, no un uuid: la app pide la página por su nombre
// («legacy-board») y ese nombre está escrito en su código, así que no puede
// depender de un identificador que cambia entre bases.
type PaginaInformativa struct {
	Slug          string    `json:"slug" db:"slug"`
	Titulo        string    `json:"titulo" db:"titulo"`
	Subtitulo     string    `json:"subtitulo" db:"subtitulo"`
	ImagenURL     string    `json:"imagen_url" db:"imagen_url"`
	Cuerpo        string    `json:"cuerpo" db:"cuerpo"`
	Publicada     bool      `json:"publicada" db:"publicada"`
	ActualizadaEn time.Time `json:"actualizada_en" db:"actualizada_en"`
}
