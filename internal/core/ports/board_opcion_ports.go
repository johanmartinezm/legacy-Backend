package ports

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"encoding/json"
)

// BoardOpcionRepository y BoardOpcionService: las listas administrables de los
// formularios de Legacy Board (20261125_legacy_board_formularios.sql).
type BoardOpcionRepository interface {
	// List devuelve las opciones de un catálogo, o de todos si catalogo es "".
	List(ctx context.Context, catalogo string, soloActivas bool) ([]*domain.OpcionBoard, error)
	FindByID(ctx context.Context, id string) (*domain.OpcionBoard, error)
	Create(ctx context.Context, o *domain.OpcionBoard) error
	// Update escribe etiqueta, orden y activo. Ni el catálogo ni el código
	// cambian nunca.
	Update(ctx context.Context, o *domain.OpcionBoard) error
}

type BoardOpcionService interface {
	// Activas devuelve las opciones activas agrupadas por catálogo, para la app.
	Activas(ctx context.Context) (map[string][]*domain.OpcionBoard, error)
	// Catalogos devuelve los 18 catálogos con su recuento, para el panel.
	Catalogos(ctx context.Context) ([]domain.CatalogoBoard, error)
	// DeCatalogo devuelve todas las opciones de un catálogo, también las inactivas.
	DeCatalogo(ctx context.Context, catalogo string) ([]*domain.OpcionBoard, error)
	Crear(ctx context.Context, o *domain.OpcionBoard) error
	Actualizar(ctx context.Context, o *domain.OpcionBoard) error
}

// BoardEnvioRepository guarda los formularios enviados (board.envios).
type BoardEnvioRepository interface {
	// Create devuelve un error que envuelve domain.ErrUniqueViolation si el
	// usuario ya tiene el A, o ya tiene un B pendiente.
	Create(ctx context.Context, e *domain.EnvioBoard) error
	// Actual devuelve el envío que manda sobre el formulario: el A del usuario,
	// o su B pendiente. domain.ErrNotFound si no hay.
	Actual(ctx context.Context, userID, formulario string) (*domain.EnvioBoard, error)
}

// BoardFormularioService sirve los formularios a la app y recibe los envíos.
type BoardFormularioService interface {
	Formulario(ctx context.Context, userID, clave string) (*domain.FormularioBoard, error)
	Enviar(ctx context.Context, userID, clave string, entrada EntradaEnvioBoard) (*domain.EnvioBoard, error)
}

// EntradaEnvioBoard es el cuerpo de POST /api/board/formularios/{clave}: las
// respuestas por clave de campo, sin interpretar todavía.
type EntradaEnvioBoard struct {
	Respuestas     map[string]json.RawMessage `json:"respuestas"`
	AceptaTerminos bool                       `json:"acepta_terminos"`
}
