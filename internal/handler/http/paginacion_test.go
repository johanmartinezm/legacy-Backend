package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Ningun listado del panel tenia tope: `GET /api/users` traia la tabla entera y
// **descifraba cada fila**, y los inscritos de un evento igual. El coste no era
// el tamaño de la respuesta sino el trabajo por fila.

func peticionConQuery(query string) *http.Request {
	return httptest.NewRequest(http.MethodGet, "/api/cualquiera"+query, nil)
}

func TestPaginacion_SinParametrosUsaElValorPorDefecto(t *testing.T) {
	limit, offset := Paginacion(peticionConQuery(""), 50)
	if limit != 50 || offset != 0 {
		t.Errorf("llegó limit=%d offset=%d; se esperaba 50/0", limit, offset)
	}
}

func TestPaginacion_RespetaLoQuePideElCliente(t *testing.T) {
	limit, offset := Paginacion(peticionConQuery("?limit=10&offset=30"), 50)
	if limit != 10 || offset != 30 {
		t.Errorf("llegó limit=%d offset=%d; se esperaba 10/30", limit, offset)
	}
}

// Sin techo, la paginacion no protege de nada: basta pedir limit=1000000 para
// volver a traerse la tabla entera, que es justo lo que se esta arreglando.
func TestPaginacion_NuncaPasaDelTecho(t *testing.T) {
	for _, pedido := range []string{"?limit=201", "?limit=1000000", "?limit=99999999999999999999"} {
		limit, _ := Paginacion(peticionConQuery(pedido), 50)
		if limit > LimiteMaximoDePagina {
			t.Errorf("con %s se coló limit=%d, por encima del techo %d", pedido, limit, LimiteMaximoDePagina)
		}
	}
}

// Un `?limit=abc` no deberia tumbar la pantalla, y un offset negativo llegaria
// al SQL como un error de Postgres, no como una lista vacia.
func TestPaginacion_ValoresRarosCaenAlValorPorDefecto(t *testing.T) {
	casos := []struct {
		query         string
		limit, offset int
	}{
		{"?limit=abc", 50, 0},
		{"?limit=", 50, 0},
		{"?limit=0", 50, 0},
		{"?limit=-5", 50, 0},
		{"?offset=-5", 50, 0},
		{"?offset=hola", 50, 0},
		{"?limit=-1&offset=-1", 50, 0},
	}
	for _, c := range casos {
		limit, offset := Paginacion(peticionConQuery(c.query), 50)
		if limit != c.limit || offset != c.offset {
			t.Errorf("con %s llegó limit=%d offset=%d; se esperaba %d/%d", c.query, limit, offset, c.limit, c.offset)
		}
	}
}

func TestEscribirTotal_PublicaLaCabecera(t *testing.T) {
	rec := httptest.NewRecorder()
	EscribirTotal(rec, 1234)

	if got := rec.Header().Get(CabeceraTotal); got != "1234" {
		t.Errorf("la cabecera del total llegó como %q", got)
	}
}

// Sin Access-Control-Expose-Headers el navegador recibe la cabecera pero **no
// deja al JavaScript leerla**, asi que el panel pintaria el paginador con total
// cero sin que nada falle en el servidor. Es de los fallos mas dificiles de
// diagnosticar mirando solo el backend.
func TestEscribirTotal_DejaQueElNavegadorLaLea(t *testing.T) {
	rec := httptest.NewRecorder()
	EscribirTotal(rec, 7)

	if got := rec.Header().Get("Access-Control-Expose-Headers"); got != CabeceraTotal {
		t.Errorf("no se expuso la cabecera al navegador: %q", got)
	}
}
