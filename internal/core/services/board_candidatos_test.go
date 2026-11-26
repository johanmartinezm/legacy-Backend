package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCandidatosBoard(t *testing.T) {
	ctx := context.Background()
	t0 := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

	consejero := func(id, usuario, nombre string, dias int, respuestas map[string]any) *domain.EnvioBoard {
		return &domain.EnvioBoard{
			ID: id, Formulario: "A", UserID: usuario, Estado: "aprobada", CreadoEn: t0.AddDate(0, 0, dias),
			DatosCifrados: `CIFRADO({"nombre_completo":"` + nombre + `"})`, Respuestas: respuestas,
		}
	}
	perfecto := consejeroPerfecto()
	perfecto["honorarios"] = "honorarios:r1"
	perfecto["cargo_actual"] = "Presidenta de junta"
	sinSector := consejeroPerfecto()
	sinSector["sectores"] = lista("sector:salud")

	preparar := func() (*BoardBandejaService, *repoEnviosBoardFake) {
		s, repo, _, _ := bandejaPrueba(t)
		op := opcionesA()
		for _, o := range catalogoMatch() {
			op.opciones = append(op.opciones, o)
		}
		s.opciones = op
		repo.guardados = map[string]*domain.EnvioBoard{
			"sol": {ID: "sol", Formulario: "B", UserID: "empresario", Estado: "aprobada", Respuestas: solicitudMatch()},
			// Empatan en 100: va primero quien se postuló antes.
			"c-tarde":    consejero("c-tarde", "u2", "Beatriz", 5, perfecto),
			"c-temprano": consejero("c-temprano", "u1", "Ana", 1, perfecto),
			"c-80":       consejero("c-80", "u3", "Carla", 2, sinSector),
			"c-0":        consejero("c-0", "u4", "Diana", 3, map[string]any{}),
			// El mismo empresario es consejero aprobado: no se recomienda a sí mismo.
			"c-yo": consejero("c-yo", "empresario", "Yo", 0, perfecto),
			// Pendiente: no entra.
			"c-pend": {ID: "c-pend", Formulario: "A", UserID: "u5", Estado: "pendiente", Respuestas: perfecto},
		}
		return s, repo
	}

	t.Run("ordenados, sin el 0 %, sin sí mismo y sin pendientes", func(t *testing.T) {
		s, _ := preparar()
		lista, err := s.Candidatos(ctx, "sol")
		if err != nil {
			t.Fatal(err)
		}
		var orden []string
		for _, c := range lista {
			orden = append(orden, c.Nombre+":"+strconv.Itoa(c.Porcentaje))
		}
		if strings.Join(orden, " ") != "Ana:100 Beatriz:100 Carla:80" {
			t.Fatalf("orden: %v", orden)
		}
		if lista[0].Honorarios != "R1" || lista[0].CargoActual != "Presidenta de junta" || len(lista[0].Razones) != 8 {
			t.Fatalf("primer candidato: %+v", lista[0])
		}
	})

	t.Run("solo para solicitudes aprobadas", func(t *testing.T) {
		s, repo := preparar()
		repo.guardados["sol"].Estado = "pendiente"
		if _, err := s.Candidatos(ctx, "sol"); !errors.Is(err, ErrMatchingNoDisponible) {
			t.Fatalf("pendiente: %v", err)
		}
		if _, err := s.Candidatos(ctx, "c-temprano"); !errors.Is(err, ErrMatchingNoDisponible) {
			t.Fatalf("una postulación A: %v", err)
		}
		if _, err := s.Candidatos(ctx, "nada"); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("inexistente: %v", err)
		}
	})
}
