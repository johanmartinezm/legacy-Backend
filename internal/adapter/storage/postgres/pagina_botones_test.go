package postgres

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"testing"
)

// Los textos de los botones tienen que ir y volver por la edición, la lectura
// y el listado. Edita Legacy Board de verdad y la deja como estaba. Necesita la
// migración 20261015_paginas_botones.sql.
func TestPaginaBotones(t *testing.T) {
	db := baseDePruebas(t)
	ctx := context.Background()
	repo := NewPaginaRepository(db)

	original, err := repo.GetBySlug(ctx, "legacy-board")
	if err != nil {
		t.Skipf("no hay página legacy-board en esta base: %v", err)
	}
	copia := *original
	defer repo.Update(context.Background(), &copia)

	editada := *original
	editada.PreguntaAcciones = "¿Prueba?"
	editada.BotonPrincipal = "Uno"
	editada.BotonSecundario = ""
	if err := repo.Update(ctx, &editada); err != nil {
		t.Fatalf("no se pudo editar: %v", err)
	}

	leida, err := repo.GetBySlug(ctx, "legacy-board")
	if err != nil {
		t.Fatalf("no se pudo leer: %v", err)
	}
	if leida.PreguntaAcciones != "¿Prueba?" || leida.BotonPrincipal != "Uno" || leida.BotonSecundario != "" {
		t.Fatalf("no volvió lo guardado: %+v", leida)
	}

	paginas, err := repo.ListAll(ctx)
	if err != nil {
		t.Fatalf("no se pudo listar: %v", err)
	}
	var enLista *domain.PaginaInformativa
	for _, p := range paginas {
		if p.Slug == "legacy-board" {
			enLista = p
		}
	}
	if enLista == nil || enLista.BotonPrincipal != "Uno" {
		t.Fatalf("el listado no trae los botones: %+v", enLista)
	}
}
