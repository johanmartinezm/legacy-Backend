package domain

import "time"

// OpcionBoard es una opción de una de las listas de los formularios A
// (consejero) y B (empresario) de Legacy Board, en board.opciones
// (scripts/20261125_legacy_board_formularios.sql). Las respuestas guardan su
// ID, nunca el texto: por eso la etiqueta se puede renombrar desde el panel.
// No se borran: se desactivan.
type OpcionBoard struct {
	ID       string `json:"id" db:"id"`
	Catalogo string `json:"catalogo" db:"catalogo"`
	// Codigo es la clave estable con la que el código se refiere a la opción
	// («otro», que nunca puntúa en el matching; «remoto»; las que un
	// formulario oculta). Lo pone la siembra o el servicio al crearla, y no
	// cambia nunca.
	Codigo        string    `json:"codigo" db:"codigo"`
	Etiqueta      string    `json:"etiqueta" db:"etiqueta"`
	Orden         int       `json:"orden" db:"orden"`
	Activo        bool      `json:"activo" db:"activo"`
	CreadoEn      time.Time `json:"creado_en" db:"creado_en"`
	ActualizadoEn time.Time `json:"actualizado_en" db:"actualizado_en"`
}

// CatalogoBoard describe una de las listas. Vive en código y no en la base:
// qué catálogos existen y cuáles alimentan el matching lo decide el código que
// los usa, no el panel.
type CatalogoBoard struct {
	Clave  string `json:"clave"`
	Nombre string `json:"nombre"`
	// Formularios en los que aparece: "A", "B" o "AB".
	Formularios string `json:"formularios"`
	// Matching: el panel lo avisa al editarlo, porque cambiarlo afecta al
	// cruce entre consejeros y empresarios (§8 del documento del cliente).
	Matching bool `json:"matching"`
	// Total y Activas los rellena el servicio para el panel.
	Total   int `json:"total"`
	Activas int `json:"activas"`
}

// CatalogosBoard son los 18 catálogos sembrados por la migración, en el orden
// en que el panel los ofrece.
var CatalogosBoard = []CatalogoBoard{
	{Clave: "tipo_documento", Nombre: "Tipo de documento", Formularios: "AB"},
	{Clave: "pais", Nombre: "País (y regiones del consejero)", Formularios: "AB", Matching: true},
	{Clave: "cliente_alumni", Nombre: "Cliente o alumni de Legacy Network", Formularios: "AB"},
	{Clave: "rol", Nombre: "Rol o nivel de cargo", Formularios: "AB"},
	{Clave: "sector", Nombre: "Sector", Formularios: "AB", Matching: true},
	{Clave: "idioma", Nombre: "Idioma", Formularios: "AB", Matching: true},
	{Clave: "tipo_organo", Nombre: "Tipo de órgano de gobierno", Formularios: "AB", Matching: true},
	{Clave: "expertise", Nombre: "Áreas de expertise", Formularios: "AB", Matching: true},
	{Clave: "dinastia", Nombre: "Dimensiones DINASTÍA", Formularios: "AB", Matching: true},
	{Clave: "modalidad", Nombre: "Modalidad", Formularios: "AB", Matching: true},
	{Clave: "honorarios", Nombre: "Rango de honorarios", Formularios: "AB"},
	{Clave: "disponibilidad", Nombre: "Disponibilidad (juntas al mes)", Formularios: "A"},
	{Clave: "tipo_empresa", Nombre: "Tipo de empresa", Formularios: "B"},
	{Clave: "tamano_empresa", Nombre: "Tamaño de la empresa", Formularios: "B"},
	{Clave: "generacion", Nombre: "Generación de la familia", Formularios: "B"},
	{Clave: "estado_junta", Nombre: "¿Tiene junta directiva hoy?", Formularios: "B"},
	{Clave: "reto", Nombre: "Reto principal", Formularios: "B"},
	{Clave: "urgencia", Nombre: "Urgencia", Formularios: "B"},
}

// CatalogoBoardPorClave devuelve el catálogo y si existe.
func CatalogoBoardPorClave(clave string) (CatalogoBoard, bool) {
	for _, c := range CatalogosBoard {
		if c.Clave == clave {
			return c, true
		}
	}
	return CatalogoBoard{}, false
}
