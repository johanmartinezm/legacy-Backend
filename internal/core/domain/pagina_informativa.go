package domain

import "time"

// PaginaInformativa es una página de solo lectura que el panel edita y la app
// pinta tal cual. No tiene formularios: sirve para contar algo que cambia con
// el tiempo sin tener que publicar una versión nueva de la app. Los botones del
// pie, si los hay, son de la app; el panel solo edita su texto.
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
	// Pregunta y textos de los dos botones del pie (hoy solo Legacy Board).
	// Vacío es «el texto de siempre»: lo pone la app.
	PreguntaAcciones string `json:"pregunta_acciones" db:"pregunta_acciones"`
	BotonPrincipal   string `json:"boton_principal" db:"boton_principal"`
	BotonSecundario  string `json:"boton_secundario" db:"boton_secundario"`
	ActualizadaEn time.Time `json:"actualizada_en" db:"actualizada_en"`
}
