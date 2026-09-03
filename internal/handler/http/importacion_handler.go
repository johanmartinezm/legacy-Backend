package http

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"encoding/json"
	"log"
	"net/http"
)

// ImportacionHandler atiende la carga masiva de asistentes.
//
// El archivo no llega aquí: lo lee el panel con SheetJS y manda las filas ya en
// JSON (reports/20260826_plan_carga_masiva.md §1). Así el backend no necesita
// ningún lector de Excel y esta ruta se puede probar sin un .xls de por medio.
type ImportacionHandler struct {
	servicio ports.ImportacionService
}

func NewImportacionHandler(servicio ports.ImportacionService) *ImportacionHandler {
	return &ImportacionHandler{servicio: servicio}
}

// MaximoFilasPorImportacion es un techo de sensatez, no una regla de negocio:
// el Summit son cientos de personas, no decenas de miles, y una petición con
// cien mil filas sería un error o un abuso.
const MaximoFilasPorImportacion = 5000

type peticionImportacion struct {
	Filas []domain.FilaImportacion `json:"filas"`
}

// ImportarUsuarios atiende POST /api/admin/importaciones/usuarios.
//
// Con `?simular=true` no escribe nada y devuelve el informe. Sin él, aplica
// —pero solo si el informe sale limpio: un archivo con una fila mala no se
// aplica a medias, y en ese caso la respuesta es el mismo informe con
// `simulacion: true`, para que quede claro que no se creó nada—.
func (h *ImportacionHandler) ImportarUsuarios(w http.ResponseWriter, r *http.Request) {
	var peticion peticionImportacion
	if err := json.NewDecoder(r.Body).Decode(&peticion); err != nil {
		http.Error(w, "no se pudo leer el archivo enviado", http.StatusBadRequest)
		return
	}

	if len(peticion.Filas) == 0 {
		http.Error(w, "el archivo no trae ninguna fila", http.StatusBadRequest)
		return
	}
	if len(peticion.Filas) > MaximoFilasPorImportacion {
		http.Error(w, "el archivo trae demasiadas filas para una sola carga", http.StatusBadRequest)
		return
	}

	simular := r.URL.Query().Get("simular") == "true"

	var resultado *domain.ResultadoImportacion
	var err error
	if simular {
		resultado, err = h.servicio.Simular(r.Context(), peticion.Filas)
	} else {
		resultado, err = h.servicio.Aplicar(r.Context(), peticion.Filas)
	}
	if err != nil {
		log.Printf("importacion: error procesando %d filas (simular=%v): %v",
			len(peticion.Filas), simular, err)
		http.Error(w, "no se pudo procesar el archivo", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resultado)
}
