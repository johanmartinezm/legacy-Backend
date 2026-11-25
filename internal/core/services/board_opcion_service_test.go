package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"
	"strings"
	"testing"
)

type repoOpcionesBoardFake struct {
	opciones []*domain.OpcionBoard
	creada   *domain.OpcionBoard
	editada  *domain.OpcionBoard
}

func (r *repoOpcionesBoardFake) List(ctx context.Context, catalogo string, soloActivas bool) ([]*domain.OpcionBoard, error) {
	var out []*domain.OpcionBoard
	for _, o := range r.opciones {
		if (catalogo == "" || o.Catalogo == catalogo) && (!soloActivas || o.Activo) {
			out = append(out, o)
		}
	}
	return out, nil
}

func (r *repoOpcionesBoardFake) FindByID(ctx context.Context, id string) (*domain.OpcionBoard, error) {
	for _, o := range r.opciones {
		if o.ID == id {
			copia := *o
			return &copia, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *repoOpcionesBoardFake) Create(ctx context.Context, o *domain.OpcionBoard) error {
	r.creada = o
	return nil
}

func (r *repoOpcionesBoardFake) Update(ctx context.Context, o *domain.OpcionBoard) error {
	r.editada = o
	return nil
}

func sectoresFake() *repoOpcionesBoardFake {
	return &repoOpcionesBoardFake{opciones: []*domain.OpcionBoard{
		{ID: "1", Catalogo: "sector", Codigo: "salud", Etiqueta: "Salud", Activo: true},
		{ID: "2", Catalogo: "sector", Codigo: "otro", Etiqueta: "Otro", Activo: true},
		{ID: "3", Catalogo: "sector", Codigo: "mineria", Etiqueta: "Minería antigua", Activo: false},
		{ID: "4", Catalogo: "idioma", Codigo: "es", Etiqueta: "Español", Activo: true},
	}}
}

func TestCodigoDesdeEtiqueta(t *testing.T) {
	casos := map[string]string{
		"Tecnología / IA":         "tecnologia_ia",
		"  M&A  ":                 "mya",
		"Más de 200":              "mas_de_200",
		"Ñandú – Último":          "nandu_ultimo",
		"¿?¡!":                    "opcion",
		strings.Repeat("ab ", 30): "ab_ab_ab_ab_ab_ab_ab_ab_ab_ab_ab_ab_ab_a",
	}
	for etiqueta, esperado := range casos {
		if got := codigoDesdeEtiqueta(etiqueta); got != esperado {
			t.Errorf("codigoDesdeEtiqueta(%q) = %q, se esperaba %q", etiqueta, got, esperado)
		}
	}
}

func TestCrearOpcionBoard(t *testing.T) {
	ctx := context.Background()

	t.Run("normaliza, genera el código y la deja como venga", func(t *testing.T) {
		repo := sectoresFake()
		o := &domain.OpcionBoard{Catalogo: "sector", Etiqueta: "  Energía   y minería ", Orden: 85, Activo: true}
		if err := NewBoardOpcionService(repo).Crear(ctx, o); err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if repo.creada.Etiqueta != "Energía y minería" || repo.creada.Codigo != "energia_y_mineria" {
			t.Fatalf("no normalizó: %+v", repo.creada)
		}
	})

	t.Run("el código choca con una inactiva y recibe sufijo", func(t *testing.T) {
		repo := sectoresFake()
		repo.opciones[2].Etiqueta = "Minería (vieja)"
		o := &domain.OpcionBoard{Catalogo: "sector", Etiqueta: "Minería"}
		if err := NewBoardOpcionService(repo).Crear(ctx, o); err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if repo.creada.Codigo != "mineria_2" {
			t.Fatalf("código %q, se esperaba mineria_2", repo.creada.Codigo)
		}
	})

	casos := []struct {
		nombre string
		o      domain.OpcionBoard
		err    error
	}{
		{"catálogo inventado", domain.OpcionBoard{Catalogo: "fintech", Etiqueta: "X"}, ErrOpcionBoardCatalogo},
		{"sin nombre", domain.OpcionBoard{Catalogo: "sector", Etiqueta: "   "}, ErrOpcionBoardSinEtiqueta},
		{"nombre largo", domain.OpcionBoard{Catalogo: "sector", Etiqueta: strings.Repeat("a", 121)}, ErrOpcionBoardEtiquetaLarga},
		{"bytes que no eran UTF-8", domain.OpcionBoard{Catalogo: "sector", Etiqueta: "Energ�a"}, ErrOpcionBoardCodificacion},
		{"orden negativo", domain.OpcionBoard{Catalogo: "sector", Etiqueta: "X", Orden: -1}, ErrOpcionBoardOrden},
		{"repetida sin distinguir mayúsculas", domain.OpcionBoard{Catalogo: "sector", Etiqueta: "SALUD"}, ErrOpcionBoardDuplicada},
		{"repetida con una inactiva", domain.OpcionBoard{Catalogo: "sector", Etiqueta: "minería antigua"}, ErrOpcionBoardDuplicada},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			o := c.o
			err := NewBoardOpcionService(sectoresFake()).Crear(ctx, &o)
			if !errors.Is(err, c.err) {
				t.Fatalf("error %v, se esperaba %v", err, c.err)
			}
			if !EsErrorDeValidacionOpcionBoard(err) {
				t.Fatalf("%v debía ser de validación", err)
			}
		})
	}

	t.Run("el mismo nombre en otro catálogo sí vale", func(t *testing.T) {
		o := &domain.OpcionBoard{Catalogo: "idioma", Etiqueta: "Salud"}
		if err := NewBoardOpcionService(sectoresFake()).Crear(ctx, o); err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
	})
}

func TestActualizarOpcionBoard(t *testing.T) {
	ctx := context.Background()

	t.Run("catálogo y código salen de la base, no del cuerpo", func(t *testing.T) {
		repo := sectoresFake()
		o := &domain.OpcionBoard{ID: "2", Catalogo: "idioma", Codigo: "hackeado", Etiqueta: "Otro sector", Orden: 999, Activo: false}
		if err := NewBoardOpcionService(repo).Actualizar(ctx, o); err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if repo.editada.Catalogo != "sector" || repo.editada.Codigo != "otro" || repo.editada.Activo {
			t.Fatalf("cambió lo que no debía o no aplicó activo: %+v", repo.editada)
		}
	})

	t.Run("conservar su propio nombre no es duplicado", func(t *testing.T) {
		o := &domain.OpcionBoard{ID: "1", Etiqueta: "salud", Activo: true}
		if err := NewBoardOpcionService(sectoresFake()).Actualizar(ctx, o); err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
	})

	t.Run("tomar el nombre de otra es duplicado", func(t *testing.T) {
		o := &domain.OpcionBoard{ID: "1", Etiqueta: "Otro", Activo: true}
		if err := NewBoardOpcionService(sectoresFake()).Actualizar(ctx, o); !errors.Is(err, ErrOpcionBoardDuplicada) {
			t.Fatalf("error %v, se esperaba duplicada", err)
		}
	})

	t.Run("id inexistente", func(t *testing.T) {
		o := &domain.OpcionBoard{ID: "nada", Etiqueta: "X"}
		if err := NewBoardOpcionService(sectoresFake()).Actualizar(ctx, o); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("error %v, se esperaba ErrNotFound", err)
		}
	})
}

func TestActivasYCatalogosBoard(t *testing.T) {
	ctx := context.Background()
	s := NewBoardOpcionService(sectoresFake())

	activas, err := s.Activas(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(activas) != len(domain.CatalogosBoard) {
		t.Fatalf("salen %d catálogos, se esperaban los %d", len(activas), len(domain.CatalogosBoard))
	}
	if len(activas["sector"]) != 2 || len(activas["urgencia"]) != 0 || activas["urgencia"] == nil {
		t.Fatalf("agrupado incorrecto: sector=%d urgencia=%v", len(activas["sector"]), activas["urgencia"])
	}

	catalogos, err := s.Catalogos(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range catalogos {
		if c.Clave == "sector" && (c.Total != 3 || c.Activas != 2 || !c.Matching) {
			t.Fatalf("recuento de sector: %+v", c)
		}
	}
	if domain.CatalogosBoard[4].Total != 0 {
		t.Fatal("Catalogos modificó la lista global en vez de una copia")
	}
}
