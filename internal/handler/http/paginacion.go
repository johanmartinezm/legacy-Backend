package http

import (
	"net/http"
	"strconv"
)

// LimiteMaximoDePagina es el techo de `?limit=`, se pida lo que se pida.
//
// Sin techo, `?limit=1000000` devuelve la tabla entera y la paginación no
// protege de nada: cualquiera con un token de administrador —o un panel con un
// error de cálculo— vuelve a traerse todo de una vez. Es justo lo que se está
// arreglando.
const LimiteMaximoDePagina = 200

// CabeceraTotal lleva cuántas filas hay en total, para que quien pagina sepa
// cuántas páginas existen.
//
// Va en una cabecera y no dentro del cuerpo a propósito: los listados de esta
// API responden un array plano y hay clientes publicados —la app instalada en
// teléfonos que no se actualizan solos— que lo recorren directamente. Envolver
// la respuesta en `{items, total}` los rompería a todos a la vez.
const CabeceraTotal = "X-Total-Count"

// Paginacion lee `?limit=` y `?offset=` de la petición.
//
// `porDefecto` es el tamaño de página cuando no se pide ninguno. Cada endpoint
// elige el suyo: no es lo mismo una lista de inscritos que se lee en pantalla
// que un volcado para exportar.
//
// Valores raros —negativos, cero, texto— caen al valor por defecto en vez de
// dar error: un `?limit=abc` no debería tumbar la pantalla, y un offset
// negativo en el SQL sí sería un error de Postgres.
func Paginacion(r *http.Request, porDefecto int) (limit, offset int) {
	limit = porDefecto
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
		limit = v
	}
	if limit > LimiteMaximoDePagina {
		limit = LimiteMaximoDePagina
	}

	if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && v > 0 {
		offset = v
	}
	return limit, offset
}

// EscribirTotal publica el total en la cabecera.
//
// También publica `Access-Control-Expose-Headers`: sin eso el navegador **no
// deja al panel leerla**, porque es una petición entre orígenes distintos
// (el panel vive en el dominio y llama a /api del mismo dominio, pero en
// desarrollo son localhost:4200 y localhost:8080). Sin esta línea la cabecera
// llega y el JavaScript la ve vacía, que es de los fallos más difíciles de
// entender mirando solo el servidor.
func EscribirTotal(w http.ResponseWriter, total int) {
	w.Header().Set(CabeceraTotal, strconv.Itoa(total))
	w.Header().Set("Access-Control-Expose-Headers", CabeceraTotal)
}
