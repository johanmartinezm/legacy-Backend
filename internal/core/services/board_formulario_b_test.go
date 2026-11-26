package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"
	"strings"
	"testing"
)

// respuestasB es un Formulario B válido, con todos los obligatorios.
func respuestasB() map[string]any {
	return map[string]any{
		"nombre_completo":        "Carlos Gómez",
		"correo":                 "carlos@familia.co",
		"telefono":               "+57 310 555 1234",
		"rol_solicitante":        "rol:presidente",
		"cliente_alumni":         "cliente_alumni:lso",
		"nombre_empresa":         "Grupo Gómez S.A.S.",
		"sector_empresa":         "sector:agro",
		"tipo_empresa":           "tipo_empresa:familiar",
		"pais_empresa":           "pais:mexico",
		"ciudad_empresa":         "Monterrey",
		"estado_junta":           "estado_junta:activa",
		"organo_fortalecer":      "tipo_organo:junta_directiva",
		"expertise_requerida":    []string{"expertise:sucesion"},
		"idiomas_requeridos":     []string{"idioma:es"},
		"reto_principal":         "reto:sucesion",
		"descripcion_contexto":   "La segunda generación entra a la junta el próximo año.",
		"modalidad_esperada":     []string{"modalidad:virtual"},
		"presupuesto_honorarios": "honorarios:r1",
	}
}

func TestFormularioBTiene27Campos(t *testing.T) {
	n := 0
	for _, s := range domain.FormularioB.Secciones {
		n += len(s.Campos)
	}
	if n != 27 || !domain.FormularioB.PermiteNuevo || domain.FormularioA.PermiteNuevo {
		t.Fatalf("B: %d campos (se esperaban 27 más los términos); permite nuevo A=%v B=%v",
			n, domain.FormularioA.PermiteNuevo, domain.FormularioB.PermiteNuevo)
	}
}

func TestEnviarFormularioB(t *testing.T) {
	ctx := context.Background()

	t.Run("válido, con lo confidencial cifrado", func(t *testing.T) {
		envios := &repoEnviosBoardFake{}
		r := respuestasB()
		r["num_consejeros"] = 2
		r["requiere_independiente"] = true
		r["dinastia_prioritaria"] = []string{"dinastia:cultura"}
		r["urgencia"] = "urgencia:trimestre"
		if _, err := servicioFormA(envios).Enviar(ctx, "u2", "B", entrada(t, r, true)); err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		e := envios.creado
		if e.Formulario != "B" || e.FotoURL != "" || e.Respuestas["sector_empresa"] != "sector:agro" ||
			e.Respuestas["requiere_independiente"] != true || e.Respuestas["num_consejeros"] != 2 {
			t.Fatalf("envío: %+v", e)
		}
		for _, clave := range []string{"nombre_empresa", "descripcion_contexto", "correo", "nombre_completo"} {
			if _, ok := e.Respuestas[clave]; ok || !strings.Contains(e.DatosCifrados, clave) {
				t.Errorf("%s debía ir cifrado y no en claro", clave)
			}
		}
	})

	casos := []struct {
		nombre string
		cambia func(map[string]any)
		campo  string
	}{
		{"«No aplica» como órgano a fortalecer", func(r map[string]any) { r["organo_fortalecer"] = "tipo_organo:no_aplica" }, "organo_fortalecer"},
		{"«Otro» como idioma requerido", func(r map[string]any) { r["idiomas_requeridos"] = []string{"idioma:otro"} }, "idiomas_requeridos"},
		{"«Remoto» como país de la empresa", func(r map[string]any) { r["pais_empresa"] = "pais:remoto" }, "pais_empresa"},
		{"seis consejeros", func(r map[string]any) { r["num_consejeros"] = 6 }, "num_consejeros"},
		{"sector como lista", func(r map[string]any) { r["sector_empresa"] = []string{"sector:agro"} }, "sector_empresa"},
		{"sin descripción del contexto", func(r map[string]any) { delete(r, "descripcion_contexto") }, "descripcion_contexto"},
		{"un campo del Formulario A", func(r map[string]any) { r["foto"] = "/api/images/x.jpg" }, "foto"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			r := respuestasB()
			c.cambia(r)
			_, err := servicioFormA(&repoEnviosBoardFake{}).Enviar(ctx, "u2", "B", entrada(t, r, true))
			var ec *ErrCampoBoard
			if !errors.As(err, &ec) || ec.Campo != c.campo {
				t.Fatalf("se esperaba error en %q, llegó %v", c.campo, err)
			}
		})
	}

	t.Run("con una pendiente no deja enviar otra", func(t *testing.T) {
		envios := &repoEnviosBoardFake{actual: &domain.EnvioBoard{Estado: "pendiente"}}
		if _, err := servicioFormA(envios).Enviar(ctx, "u2", "B", entrada(t, respuestasB(), true)); !errors.Is(err, ErrFormularioBoardEnviado) {
			t.Fatalf("error %v", err)
		}
	})

	t.Run("con la anterior ya decidida sí deja, y la app la ve", func(t *testing.T) {
		decidida := &domain.EnvioBoard{ID: "b1", Formulario: "B", Estado: "aprobada"}
		envios := &repoEnviosBoardFake{ultimo: decidida}
		f, err := servicioFormA(envios).Formulario(ctx, "u2", "B")
		if err != nil || f.EnvioActual == nil || f.EnvioActual.Estado != "aprobada" || !f.PermiteNuevo {
			t.Fatalf("formulario: %+v err %v", f, err)
		}
		if _, err := servicioFormA(envios).Enviar(ctx, "u2", "B", entrada(t, respuestasB(), true)); err != nil {
			t.Fatalf("una solicitud nueva tras la decisión debía valer: %v", err)
		}
	})
}

func TestBandejaFiltroDelB(t *testing.T) {
	s, repo, _, _ := bandejaPrueba(t)
	if _, _, err := s.Listar(context.Background(), domain.FiltroEnviosBoard{Formulario: "B", Campo: "sector", Opcion: "sector:agro"}, 25, 0); err != nil {
		t.Fatal(err)
	}
	if repo.filtroVisto.Campo != "sector_empresa" || repo.filtroVisto.Multiple {
		t.Fatalf("en el B el sector es «sector_empresa» y de selección simple: %+v", repo.filtroVisto)
	}
	if _, _, err := s.Listar(context.Background(), domain.FiltroEnviosBoard{Formulario: "B", Campo: "expertise", Opcion: "x"}, 25, 0); err != nil ||
		repo.filtroVisto.Campo != "expertise_requerida" || !repo.filtroVisto.Multiple {
		t.Fatalf("expertise del B: %+v %v", repo.filtroVisto, err)
	}
}
