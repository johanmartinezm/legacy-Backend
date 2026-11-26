package domain

import "time"

// Los formularios de Legacy Board no se programan uno por uno: son
// configuraciones de un mismo motor (§3.4 del plan
// reports/20261002_plan_legacy_board_formularios.md). La configuración vive
// aquí, la app la pide por GET /api/board/formularios/{clave} para pintarlo, y
// el servicio la usa para validar el envío. Así lo que la app muestra y lo que
// el backend acepta no pueden separarse.

// Tipos de campo que entiende el motor, en la app y en el backend.
const (
	CampoTexto          = "texto"
	CampoTextoLargo     = "texto_largo"
	CampoEmail          = "email"
	CampoTelefono       = "telefono"
	CampoNumero         = "numero"
	CampoFecha          = "fecha"
	CampoURL            = "url"
	CampoSeleccion      = "seleccion"
	CampoMultiseleccion = "multiseleccion"
	CampoSiNo           = "sino"
	CampoFoto           = "foto"
)

// CampoBoard es un campo de un formulario.
type CampoBoard struct {
	Clave       string `json:"clave"`
	Etiqueta    string `json:"etiqueta"`
	Tipo        string `json:"tipo"`
	Obligatorio bool   `json:"obligatorio"`
	Ayuda       string `json:"ayuda,omitempty"`
	// Catalogo: de qué lista salen las opciones (seleccion y multiseleccion).
	Catalogo string `json:"catalogo,omitempty"`
	// Excluir: códigos de opción del catálogo que este campo no ofrece (p. ej.
	// «remoto» en el país de residencia). No viaja a la app: el servicio ya
	// manda las opciones filtradas.
	Excluir []string `json:"-"`
	// Exclusiva: código de una opción que no se puede marcar junto a otras
	// («No aplica» en los órganos de gobierno).
	Exclusiva string `json:"exclusiva,omitempty"`
	// Min y Max: rango de los campos numéricos.
	Min *int `json:"min,omitempty"`
	Max *int `json:"max,omitempty"`
	// MaxLargo: caracteres como máximo de los campos de texto.
	MaxLargo int `json:"max_largo,omitempty"`
	// Patron: expresión regular que debe cumplir un texto (Go, sintaxis RE2;
	// la app usa la misma, que es compatible con la de Dart en estos casos).
	Patron string `json:"patron,omitempty"`
	// Dominio: el host que debe tener una URL (linkedin.com).
	Dominio string `json:"dominio,omitempty"`
	// Cifrado: el valor va a datos_cifrados y no a respuestas. Son los datos
	// personales y los textos confidenciales (§3.5 del plan).
	Cifrado bool `json:"cifrado"`
	// Prellenar: dato del perfil con el que la app llena el campo al abrirlo:
	// "nombre", "correo", "telefono" o "foto".
	Prellenar string `json:"prellenar,omitempty"`
	// Opciones las rellena el servicio al servir el formulario.
	Opciones []OpcionFormulario `json:"opciones,omitempty"`
}

// OpcionFormulario es lo que la app necesita de una opción para pintarla.
type OpcionFormulario struct {
	ID       string `json:"id"`
	Codigo   string `json:"codigo"`
	Etiqueta string `json:"etiqueta"`
}

type SeccionBoard struct {
	Titulo string       `json:"titulo"`
	Campos []CampoBoard `json:"campos"`
}

// FormularioBoard es la configuración completa de un formulario.
type FormularioBoard struct {
	Clave       string         `json:"clave"`
	Titulo      string         `json:"titulo"`
	Descripcion string         `json:"descripcion"`
	Secciones   []SeccionBoard `json:"secciones"`
	// TextoTerminos acompaña a la casilla obligatoria del final.
	TextoTerminos string `json:"texto_terminos"`
	// TextoConfirmacion es lo que la app muestra al enviarlo.
	TextoConfirmacion string `json:"texto_confirmacion"`
	// EnvioActual lo rellena el servicio: el envío del usuario que manda sobre
	// el formulario (el A ya enviado, o el B pendiente). Si viene, la app
	// muestra su estado en vez del formulario.
	EnvioActual *EnvioBoard `json:"envio_actual"`
}

// Estados de un envío.
const (
	EstadoEnvioPendiente = "pendiente"
	EstadoEnvioAprobada  = "aprobada"
	EstadoEnvioRechazada = "rechazada"
)

// EnvioBoard es un formulario enviado (board.envios).
type EnvioBoard struct {
	ID         string         `json:"id"`
	Formulario string         `json:"formulario"`
	UserID     string         `json:"-"`
	Respuestas map[string]any `json:"-"`
	// DatosCifrados: JSON de los campos con Cifrado, ya cifrado. Nunca sale tal
	// cual: el panel recibe los datos descifrados (fase 5).
	DatosCifrados  string `json:"-"`
	FotoURL        string `json:"-"`
	AceptaTerminos bool   `json:"-"`
	Estado         string `json:"estado"`
	// EstadoMotivo no se manda a la app: es la nota del comité. Si el usuario
	// la recibe o no se decide con los avisos (fase 5).
	EstadoMotivo     string     `json:"-"`
	EstadoCambiadoEn *time.Time `json:"estado_cambiado_en,omitempty"`
	CreadoEn         time.Time  `json:"creado_en"`
}

func entero(n int) *int { return &n }

// FormularioA es la postulación del consejero: §7.1 del documento del cliente,
// con las erratas resueltas el 2026-10-02, el campo de programas de formación
// añadido el 2026-11-23 y sus áreas, añadidas el 2026-11-25 para el matching. La fecha de nacimiento y el documento son opcionales
// (decisión 1: Apple ya rechazó la app por exigirlos).
var FormularioA = FormularioBoard{
	Clave:  "A",
	Titulo: "Postulación como consejero",
	Descripcion: "Cuéntanos tu trayectoria y tu experiencia en órganos de gobierno. El comité de " +
		"curaduría revisa cada postulación y te avisará de su decisión.",
	TextoTerminos: "Acepto los términos y condiciones y autorizo el tratamiento de mis datos " +
		"personales para evaluar mi postulación a Legacy Board.",
	TextoConfirmacion: "Recibimos tu postulación. El comité de curaduría la revisará y te " +
		"avisaremos de su decisión.",
	Secciones: []SeccionBoard{
		{Titulo: "Identificación", Campos: []CampoBoard{
			{Clave: "nombre_completo", Etiqueta: "Nombre completo", Tipo: CampoTexto, Obligatorio: true, MaxLargo: 120, Cifrado: true, Prellenar: "nombre"},
			{Clave: "tipo_documento", Etiqueta: "Tipo de documento", Tipo: CampoSeleccion, Catalogo: "tipo_documento", Cifrado: true},
			{Clave: "numero_documento", Etiqueta: "Número de documento", Tipo: CampoTexto, MaxLargo: 30, Patron: `^[A-Za-z0-9.\- ]+$`, Cifrado: true},
			{Clave: "fecha_nacimiento", Etiqueta: "Fecha de nacimiento", Tipo: CampoFecha, Cifrado: true},
			{Clave: "correo", Etiqueta: "Correo profesional", Tipo: CampoEmail, Obligatorio: true, Cifrado: true, Prellenar: "correo"},
			{Clave: "telefono", Etiqueta: "Teléfono / WhatsApp", Tipo: CampoTelefono, Obligatorio: true, Ayuda: "Con el código de país, p. ej. +57 300 000 0000", Cifrado: true, Prellenar: "telefono"},
			{Clave: "pais_residencia", Etiqueta: "País de residencia", Tipo: CampoSeleccion, Obligatorio: true, Catalogo: "pais", Excluir: []string{"remoto"}},
			{Clave: "ciudad_residencia", Etiqueta: "Ciudad de residencia", Tipo: CampoTexto, Obligatorio: true, MaxLargo: 80},
			{Clave: "foto", Etiqueta: "Fotografía ejecutiva", Tipo: CampoFoto, Obligatorio: true, Ayuda: "Fondo neutro, hasta 5 MB", Prellenar: "foto"},
			{Clave: "linkedin", Etiqueta: "Perfil de LinkedIn", Tipo: CampoURL, Dominio: "linkedin.com", MaxLargo: 200, Ayuda: "https://www.linkedin.com/in/..."},
			{Clave: "cliente_alumni", Etiqueta: "¿Es cliente o alumni de Legacy Network?", Tipo: CampoSeleccion, Obligatorio: true, Catalogo: "cliente_alumni"},
		}},
		{Titulo: "Trayectoria profesional", Campos: []CampoBoard{
			{Clave: "formacion", Etiqueta: "Formación", Tipo: CampoTextoLargo, Obligatorio: true, MaxLargo: 1000},
			{Clave: "nivel_cargo", Etiqueta: "Nivel de cargo / rol actual", Tipo: CampoSeleccion, Obligatorio: true, Catalogo: "rol"},
			{Clave: "cargo_actual", Etiqueta: "Cargo / rol actual", Tipo: CampoTexto, Obligatorio: true, MaxLargo: 120, Ayuda: "Ej. Presidente de junta"},
			{Clave: "anios_experiencia", Etiqueta: "Años de experiencia directiva", Tipo: CampoNumero, Obligatorio: true, Min: entero(0), Max: entero(60)},
			{Clave: "sectores", Etiqueta: "Sectores donde ha trabajado", Tipo: CampoMultiseleccion, Obligatorio: true, Catalogo: "sector"},
			{Clave: "idiomas", Etiqueta: "Idiomas", Tipo: CampoMultiseleccion, Obligatorio: true, Catalogo: "idioma"},
		}},
		{Titulo: "Experiencia en órganos de gobierno", Campos: []CampoBoard{
			{Clave: "tipos_organo", Etiqueta: "Tipos de órgano en los que ha participado", Tipo: CampoMultiseleccion, Obligatorio: true, Catalogo: "tipo_organo", Exclusiva: "no_aplica"},
			{Clave: "num_juntas", Etiqueta: "Número de juntas en las que sirve hoy", Tipo: CampoNumero, Obligatorio: true, Min: entero(0), Max: entero(9)},
			{Clave: "num_comites", Etiqueta: "Número de comités a los que sirve hoy", Tipo: CampoNumero, Obligatorio: true, Min: entero(0), Max: entero(9)},
			{Clave: "fue_independiente", Etiqueta: "¿Ha sido miembro independiente?", Tipo: CampoSiNo, Obligatorio: true},
			{Clave: "empresas_contextos", Etiqueta: "Empresas / contextos (confidencial)", Tipo: CampoTextoLargo, Obligatorio: true, MaxLargo: 2000, Ayuda: "No se muestra públicamente: solo lo lee el comité", Cifrado: true},
			{Clave: "programas_formacion", Etiqueta: "Mencione los programas de formación que ha cursado dirigidos a miembros de órganos de gobierno (juntas directivas, consejos de administración, consejos de familia u otros)", Tipo: CampoTextoLargo, Obligatorio: true, MaxLargo: 2000},
			// Decidido el 2026-11-25 (opción b del §2 ter del plan): el texto de
			// arriba es informativo, y lo que cuenta para el matching son estas
			// áreas, de la misma lista que «Áreas de expertise». En la fase 6 un
			// área suma dentro del 20 % de expertise si el consejero la marcó en
			// cualquiera de los dos campos.
			{Clave: "programas_areas", Etiqueta: "Áreas que cubrieron esos programas", Tipo: CampoMultiseleccion, Obligatorio: true, Catalogo: "expertise"},
		}},
		{Titulo: "Expertise y aporte de valor", Campos: []CampoBoard{
			{Clave: "expertise", Etiqueta: "Áreas de expertise", Tipo: CampoMultiseleccion, Obligatorio: true, Catalogo: "expertise"},
			{Clave: "dinastia", Etiqueta: "Especialización (DINASTÍA)", Tipo: CampoMultiseleccion, Catalogo: "dinastia"},
			{Clave: "aporte", Etiqueta: "¿Qué aporta a una junta?", Tipo: CampoTextoLargo, Obligatorio: true, MaxLargo: 600},
		}},
		{Titulo: "Disponibilidad y condiciones", Campos: []CampoBoard{
			{Clave: "disponibilidad", Etiqueta: "Disponibilidad (juntas al mes)", Tipo: CampoSeleccion, Obligatorio: true, Catalogo: "disponibilidad"},
			{Clave: "modalidad", Etiqueta: "Modalidad", Tipo: CampoMultiseleccion, Obligatorio: true, Catalogo: "modalidad"},
			{Clave: "regiones", Etiqueta: "Regiones donde puede servir", Tipo: CampoMultiseleccion, Obligatorio: true, Catalogo: "pais", Excluir: []string{"otro"}},
			{Clave: "honorarios", Etiqueta: "Expectativa de honorarios (rango)", Tipo: CampoSeleccion, Obligatorio: true, Catalogo: "honorarios"},
		}},
	},
}

// FormulariosBoard: los formularios que existen, por clave. El B se añade en
// la fase 4.
var FormulariosBoard = map[string]*FormularioBoard{
	"A": &FormularioA,
}
