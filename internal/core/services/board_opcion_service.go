package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	maxEtiquetaOpcionBoard = 120
	maxOrdenOpcionBoard    = 100000
	maxCodigoOpcionBoard   = 40
)

var (
	ErrOpcionBoardCatalogo      = errors.New("el catálogo no existe")
	ErrOpcionBoardSinEtiqueta   = errors.New("el nombre de la opción es obligatorio")
	ErrOpcionBoardEtiquetaLarga = fmt.Errorf("el nombre admite hasta %d caracteres", maxEtiquetaOpcionBoard)
	ErrOpcionBoardOrden         = fmt.Errorf("el orden debe ser un número entre 0 y %d", maxOrdenOpcionBoard)
	ErrOpcionBoardDuplicada     = errors.New("ya hay una opción con ese nombre en este catálogo")
	ErrOpcionBoardCodificacion  = errors.New("el nombre trae caracteres inválidos; vuelve a escribirlo")
)

// EsErrorDeValidacionOpcionBoard dice si el error es del formulario (400).
func EsErrorDeValidacionOpcionBoard(err error) bool {
	for _, e := range []error{ErrOpcionBoardCatalogo, ErrOpcionBoardSinEtiqueta, ErrOpcionBoardEtiquetaLarga,
		ErrOpcionBoardOrden, ErrOpcionBoardDuplicada, ErrOpcionBoardCodificacion} {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}

type BoardOpcionService struct {
	repo ports.BoardOpcionRepository
}

func NewBoardOpcionService(repo ports.BoardOpcionRepository) *BoardOpcionService {
	return &BoardOpcionService{repo: repo}
}

// Activas agrupa por catálogo. Todos los catálogos conocidos salen en el mapa,
// aunque estén vacíos, para que la app no tenga que distinguir «no existe» de
// «no tiene opciones».
func (s *BoardOpcionService) Activas(ctx context.Context) (map[string][]*domain.OpcionBoard, error) {
	opciones, err := s.repo.List(ctx, "", true)
	if err != nil {
		return nil, err
	}
	agrupadas := make(map[string][]*domain.OpcionBoard, len(domain.CatalogosBoard))
	for _, c := range domain.CatalogosBoard {
		agrupadas[c.Clave] = []*domain.OpcionBoard{}
	}
	for _, o := range opciones {
		// Una fila de un catálogo que el código no conoce no sale: ningún
		// formulario la podría usar.
		if _, ok := agrupadas[o.Catalogo]; ok {
			agrupadas[o.Catalogo] = append(agrupadas[o.Catalogo], o)
		}
	}
	return agrupadas, nil
}

func (s *BoardOpcionService) Catalogos(ctx context.Context) ([]domain.CatalogoBoard, error) {
	opciones, err := s.repo.List(ctx, "", false)
	if err != nil {
		return nil, err
	}
	catalogos := make([]domain.CatalogoBoard, len(domain.CatalogosBoard))
	copy(catalogos, domain.CatalogosBoard)
	indice := make(map[string]int, len(catalogos))
	for i, c := range catalogos {
		indice[c.Clave] = i
	}
	for _, o := range opciones {
		if i, ok := indice[o.Catalogo]; ok {
			catalogos[i].Total++
			if o.Activo {
				catalogos[i].Activas++
			}
		}
	}
	return catalogos, nil
}

func (s *BoardOpcionService) DeCatalogo(ctx context.Context, catalogo string) ([]*domain.OpcionBoard, error) {
	if _, ok := domain.CatalogoBoardPorClave(catalogo); !ok {
		return nil, ErrOpcionBoardCatalogo
	}
	return s.repo.List(ctx, catalogo, false)
}

func (s *BoardOpcionService) Crear(ctx context.Context, o *domain.OpcionBoard) error {
	if _, ok := domain.CatalogoBoardPorClave(o.Catalogo); !ok {
		return ErrOpcionBoardCatalogo
	}
	if err := normalizarOpcionBoard(o); err != nil {
		return err
	}
	existentes, err := s.repo.List(ctx, o.Catalogo, false)
	if err != nil {
		return err
	}
	if etiquetaRepetida(existentes, o.Etiqueta, "") {
		return ErrOpcionBoardDuplicada
	}
	o.Codigo = codigoLibre(existentes, o.Etiqueta)
	return s.repo.Create(ctx, o)
}

// Actualizar cambia nombre, orden y si está activa. El catálogo y el código se
// toman de la base aunque el cuerpo traiga otros: las respuestas ya enviadas y
// el código que busca «otro» dependen de ellos.
func (s *BoardOpcionService) Actualizar(ctx context.Context, o *domain.OpcionBoard) error {
	actual, err := s.repo.FindByID(ctx, o.ID)
	if err != nil {
		return err
	}
	o.Catalogo = actual.Catalogo
	o.Codigo = actual.Codigo
	if err := normalizarOpcionBoard(o); err != nil {
		return err
	}
	existentes, err := s.repo.List(ctx, o.Catalogo, false)
	if err != nil {
		return err
	}
	if etiquetaRepetida(existentes, o.Etiqueta, o.ID) {
		return ErrOpcionBoardDuplicada
	}
	return s.repo.Update(ctx, o)
}

func normalizarOpcionBoard(o *domain.OpcionBoard) error {
	o.Etiqueta = strings.Join(strings.Fields(o.Etiqueta), " ")
	switch {
	case o.Etiqueta == "":
		return ErrOpcionBoardSinEtiqueta
	// encoding/json cambia los bytes que no son UTF-8 por U+FFFD sin avisar: un
	// cliente que mande otra codificación guardaría «Energ�a» en la app.
	case strings.ContainsRune(o.Etiqueta, utf8.RuneError):
		return ErrOpcionBoardCodificacion
	case utf8.RuneCountInString(o.Etiqueta) > maxEtiquetaOpcionBoard:
		return ErrOpcionBoardEtiquetaLarga
	case o.Orden < 0 || o.Orden > maxOrdenOpcionBoard:
		return ErrOpcionBoardOrden
	}
	return nil
}

// etiquetaRepetida compara sin distinguir mayúsculas, también contra las
// opciones inactivas: reactivar la vieja es mejor que tener dos iguales, que en
// la app se verían como una sola y en el matching serían dos.
func etiquetaRepetida(existentes []*domain.OpcionBoard, etiqueta, excepto string) bool {
	for _, e := range existentes {
		if e.ID != excepto && strings.EqualFold(e.Etiqueta, etiqueta) {
			return true
		}
	}
	return false
}

var sinTildes = strings.NewReplacer(
	"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
	"à", "a", "è", "e", "ì", "i", "ò", "o", "ù", "u", "ç", "c", "&", "y",
)

// codigoDesdeEtiqueta convierte «Tecnología / IA» en «tecnologia_ia»: lo que
// acepta la restricción opciones_codigo_formato (^[a-z0-9_]+$).
func codigoDesdeEtiqueta(etiqueta string) string {
	s := sinTildes.Replace(strings.ToLower(etiqueta))
	var b strings.Builder
	guion := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			guion = false
		} else if !guion && b.Len() > 0 {
			b.WriteByte('_')
			guion = true
		}
	}
	codigo := strings.TrimRight(b.String(), "_")
	if len(codigo) > maxCodigoOpcionBoard {
		codigo = strings.TrimRight(codigo[:maxCodigoOpcionBoard], "_")
	}
	if codigo == "" {
		codigo = "opcion"
	}
	return codigo
}

// codigoLibre añade _2, _3… si el código ya lo usa otra opción del catálogo,
// p. ej. una inactiva que se llamaba distinto pero daba el mismo código.
func codigoLibre(existentes []*domain.OpcionBoard, etiqueta string) string {
	usados := make(map[string]bool, len(existentes))
	for _, e := range existentes {
		usados[e.Codigo] = true
	}
	base := codigoDesdeEtiqueta(etiqueta)
	codigo := base
	for n := 2; usados[codigo]; n++ {
		codigo = base + "_" + strconv.Itoa(n)
	}
	return codigo
}
