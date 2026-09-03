package domain

import "strings"

// FilaImportacion es una fila del archivo de asistentes ya leída por el panel.
//
// El archivo es un .xls de 42 columnas que exporta la plataforma que vendió las
// entradas; **de esas 42 se importan 15** y el resto se descartan por decisión
// (reports/20260826_plan_carga_masiva.md §3). Aquí llegan ya con nombre propio:
// el backend no lee Excel, recibe JSON.
type FilaImportacion struct {
	// Fila es el número que la persona ve en su hoja de cálculo. Solo sirve
	// para los mensajes: «fila 34, tipo de documento no reconocido» es
	// accionable; «el archivo tiene errores» no.
	Fila int `json:"fila"`

	Nombres   string `json:"nombres"`
	Apellidos string `json:"apellidos"`
	Email     string `json:"email"`
	Telefono  string `json:"telefono"`
	Empresa   string `json:"empresa"`
	Cargo     string `json:"cargo"`

	TipoDocumento   string `json:"tipo_documento"`
	NumeroDocumento string `json:"numero_documento"`

	Pais            string `json:"pais"`
	Ciudad          string `json:"ciudad"`
	Departamento    string `json:"departamento"`
	Direccion       string `json:"direccion"`
	Sexo            string `json:"sexo"`
	FechaNacimiento string `json:"fecha_nacimiento"`

	AceptaTerminos bool `json:"acepta_terminos"`
}

// ProblemaDeFila señala qué fila y qué columna hay que corregir. Se corrige el
// archivo, no el código.
type ProblemaDeFila struct {
	Fila    int    `json:"fila"`
	Columna string `json:"columna"`
	Motivo  string `json:"motivo"`
}

// OpcionesImportacion es lo que distingue las dos entradas de la carga
// (reports/20260826_plan_carga_masiva.md §4).
//
// Por dentro hay **un solo importador**: lo único que cambia entre «Importar
// usuarios» e «Importar asistentes» es si hay un evento de por medio. Dos
// motores se separan al tercer arreglo; uno, no.
type OpcionesImportacion struct {
	// EventoID vacío significa que solo se crean cuentas. Con evento, además
	// se inscribe a todo el archivo a **ese** evento: lo fija la pantalla, no
	// el archivo, así que la columna «Ticket» no se lee.
	EventoID string `json:"evento_id"`

	// GenerarCredencial crea el código de acceso. Apagado —el valor por
	// defecto— la inscripción queda con qr_data en NULL y esa persona no pasa
	// el check-in hasta que se le genere desde la pantalla de inscritos.
	//
	// En un evento virtual da igual: no hay QR que mostrar, y el panel enseña
	// el interruptor apagado y deshabilitado con la razón al lado.
	GenerarCredencial bool `json:"generar_credencial"`

	// AvisarPorCorreo manda un correo por persona. Apagado por defecto: una
	// carga de trescientas filas que avisa sin que nadie lo pida es trescientos
	// correos. Qué correo sale depende del otro interruptor, y **nunca salen
	// los dos**.
	AvisarPorCorreo bool `json:"avisar_por_correo"`
}

// ConEvento dice si esta carga además inscribe.
func (o OpcionesImportacion) ConEvento() bool { return o.EventoID != "" }

// AvisoDeLaCarga traduce los dos interruptores al correo que toca, que es la
// tabla de §4.1: con credencial sale el de credencial —con el QR dibujado—; sin
// ella, el de inscripción de siempre; y sin avisar, ninguno.
func (o OpcionesImportacion) AvisoDeLaCarga() AvisoDeAlta {
	if !o.AvisarPorCorreo {
		return AvisoNinguno
	}
	if o.GenerarCredencial {
		return AvisoCredencial
	}
	return AvisoPorDefecto
}

// ResultadoImportacion es lo que devuelve tanto la simulación como la carga.
//
// La simulación no escribe nada: dice cuántas cuentas se crearían, cuántas ya
// existen y qué hay que arreglar. Es el paso que se le enseña a quien preparó
// el archivo.
type ResultadoImportacion struct {
	Simulacion bool `json:"simulacion"`
	Total      int  `json:"total"`
	Nuevas     int  `json:"nuevas"`
	YaExistian int  `json:"ya_existian"`
	Creadas    int  `json:"creadas"`

	// PorInscribir e Inscritas solo tienen sentido con evento. La primera la
	// calcula la simulación —cuántas filas quedarían inscritas—; la segunda es
	// lo que se inscribió de verdad.
	//
	// YaInscritas son las que ya estaban en ese evento: volver a pasar el mismo
	// archivo no duplica a nadie, y conviene que el informe lo diga en vez de
	// dejar creer que no se hizo nada.
	PorInscribir int `json:"por_inscribir"`
	Inscritas    int `json:"inscritas"`
	YaInscritas  int `json:"ya_inscritas"`

	Problemas []ProblemaDeFila `json:"problemas"`
}

// TieneProblemas indica si el archivo se puede aplicar.
func (r *ResultadoImportacion) TieneProblemas() bool { return len(r.Problemas) > 0 }

// TiposDeDocumentoImportados traduce lo que trae el archivo al catálogo de
// Legacy.
//
// El archivo usa dos etiquetas propias —«CC/TI/CE» y «Pasaporte u otro
// documento»— que **no existen** en el catálogo (`Cédula`, `Cédula de
// extranjería`, `Pasaporte`, `Tarjeta de identidad`, `NIT`…). Importarlas
// crudas repetiría el fallo del 25-08: el desplegable no encuentra el valor y
// el campo sale vacío en la app.
var TiposDeDocumentoImportados = map[string]string{
	"cc/ti/ce":                   "Cédula",
	"cc":                         "Cédula",
	"ti":                         "Tarjeta de identidad",
	"ce":                         "Cédula de extranjería",
	"pasaporte u otro documento": "Pasaporte",
	"pasaporte":                  "Pasaporte",
	"nit":                        "NIT",
}

// TraducirTipoDeDocumento devuelve el valor del catálogo y si se reconoció.
// Una cadena vacía se acepta: no todas las filas traen documento y solo es
// obligatorio porque de él sale la contraseña, cosa que valida el importador.
func TraducirTipoDeDocumento(valor string) (string, bool) {
	limpio := strings.TrimSpace(valor)
	if limpio == "" {
		return "", true
	}
	// El archivo trae puntuación pegada al final en algunas columnas.
	limpio = strings.Trim(limpio, " .,;:")
	if traducido, ok := TiposDeDocumentoImportados[strings.ToLower(limpio)]; ok {
		return traducido, true
	}
	return "", false
}
