package postgres

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"testing"
)

// La imagen de una unidad tiene que ir y volver por las tres consultas
// (alta, edición y listado), y poder quitarse: vaciarla al editar la borra,
// no conserva la anterior. Necesita la migración 20261015_asesoria_unidades_imagen.sql.
func TestAsesoriaUnidadImagen(t *testing.T) {
	db := baseDePruebas(t)
	ctx := context.Background()
	repo := NewAsesoriaUnidadRepository(db)

	u := &domain.AsesoriaUnidad{
		Titulo: "unidad de prueba con imagen", Icono: "negocio", Necesidad: "Otro",
		ImagenURL: "https://example.com/logo.png", Orden: 999,
	}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("no se pudo crear: %v", err)
	}
	defer repo.Delete(context.Background(), u.ID)

	if u.ImagenURL != "https://example.com/logo.png" {
		t.Fatalf("al crear, imagen = %q", u.ImagenURL)
	}

	u.ImagenURL = ""
	if err := repo.Update(ctx, u); err != nil {
		t.Fatalf("no se pudo editar: %v", err)
	}

	unidades, err := repo.List(ctx, false)
	if err != nil {
		t.Fatalf("no se pudo listar: %v", err)
	}
	for _, x := range unidades {
		if x.ID == u.ID {
			if x.ImagenURL != "" {
				t.Fatalf("vaciar la imagen no la quitó: %q", x.ImagenURL)
			}
			return
		}
	}
	t.Fatal("la unidad no aparece en el listado")
}
