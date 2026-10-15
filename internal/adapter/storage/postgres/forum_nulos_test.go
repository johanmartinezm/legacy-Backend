package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

// Un foro con description y cover_url en NULL tiene que listarse igual. El
// 2026-10-14 la migración de los foros fijos los creó sin cover_url y
// GET /api/forums respondió 500 («cannot scan NULL into *string») en producción.
func TestListForumsConColumnasNulas(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := pgxpool.Connect(ctx, urlDeLaBaseDePruebas())
	if err == nil {
		err = db.Ping(ctx)
	}
	if err != nil {
		t.Skipf("sin base de datos de pruebas: %v", err)
	}
	defer db.Close()

	var id string
	if err := db.QueryRow(ctx, `INSERT INTO core.forums (title, description, cover_url, status)
		VALUES ('foro de prueba con nulos', NULL, NULL, 'active') RETURNING id`).Scan(&id); err != nil {
		t.Fatalf("no se pudo crear el foro de prueba: %v", err)
	}
	defer db.Exec(context.Background(), `DELETE FROM core.forums WHERE id = $1`, id)

	repo := NewForumRepository(db)
	forums, err := repo.ListForums(ctx, false)
	if err != nil {
		t.Fatalf("ListForums: %v", err)
	}
	encontrado := false
	for _, f := range forums {
		if f.ID == id {
			encontrado = true
		}
	}
	if !encontrado {
		t.Fatal("el foro con columnas nulas no aparece en la lista")
	}
	if _, err := repo.GetForumByID(ctx, id); err != nil {
		t.Fatalf("GetForumByID: %v", err)
	}
}
