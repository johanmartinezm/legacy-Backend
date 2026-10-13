package domain

import "time"

// SeccionInicio es una tarjeta de la pantalla de Inicio de la app con la imagen,
// el título y el texto que el panel le asigna. A dónde lleva sigue en el código
// de la app, que es quien sabe qué hace cada tarjeta.
//
// La identifica el `slug` («legacy-board», «eventos»…), igual que a las páginas
// informativas: la app busca su tarjeta por ese nombre, escrito en su código.
type SeccionInicio struct {
	Slug string `json:"slug" db:"slug"`
	// Nombre es solo para el panel: cómo se llama la tarjeta en la app.
	Nombre string `json:"nombre" db:"nombre"`
	// Orden replica el de la pantalla, para que el panel las liste igual.
	Orden int `json:"orden" db:"orden"`
	// Pantalla de la app donde se ve: «inicio» (tarjetas del Inicio), «lso»
	// (encabezado y banner de LSO · Escuela, desde 20261013_textos_lso.sql) o
	// «asesoria» (texto de bienvenida, desde 20261013_texto_asesoria.sql; ya no
	// se pinta) o «legacy-plus» (textos fijos de la pantalla Legacy+, desde
	// 20261013_legacy_plus_contenido.sql).
	Pantalla string `json:"pantalla" db:"pantalla"`
	// ImagenURL vacía significa «sin imagen»: la app pinta la tarjeta como
	// siempre, con su icono.
	ImagenURL string `json:"imagen_url" db:"imagen_url"`
	// Titulo y Texto vacíos significan «los de siempre»: la app usa los que
	// trae escritos en su código.
	Titulo        string    `json:"titulo" db:"titulo"`
	Texto         string    `json:"texto" db:"texto"`
	ActualizadaEn time.Time `json:"actualizada_en" db:"actualizada_en"`
}

// CambiosSeccionInicio son los campos que el panel puede cambiar. Un puntero
// nil deja el valor guardado como está: así un cliente que solo manda la
// imagen, como el panel anterior, no borra el título ni el texto.
type CambiosSeccionInicio struct {
	ImagenURL *string `json:"imagen_url"`
	Titulo    *string `json:"titulo"`
	Texto     *string `json:"texto"`
}
