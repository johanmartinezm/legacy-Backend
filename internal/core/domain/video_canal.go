package domain

import "time"

// VideoDeCanal es un video publicado en un canal de YouTube, ya normalizado para
// la sección de contenido de la app.
//
// No es un CustomContent: aquellos son filas que alguien crea desde el panel y
// se pueden editar y despublicar. Estos vienen de fuera, no se guardan y no se
// administran desde aquí, así que se modelan aparte en vez de forzarlos dentro
// de una estructura que promete cosas que no cumplen (id propio, `is_published`,
// categoría).
//
// Canal es el nombre del canal y **es el autor** que se muestra en la app. Hasta
// el 2026-08-18 todo el contenido salía firmado como "Autor desconocido"; para
// los videos de YouTube la firma correcta es el canal, no una persona.
type VideoDeCanal struct {
	// ID es el identificador del video en YouTube, no un uuid propio. Sirve
	// para deduplicar si un mismo video apareciera en dos canales.
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	VideoURL     string    `json:"video_url"`
	ThumbnailURL string    `json:"thumbnail_url"`
	Channel      string    `json:"channel"`
	PublishedAt  time.Time `json:"published_at"`
}
