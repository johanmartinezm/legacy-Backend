package services

import (
	"applegacy/backend/internal/core/domain"
	"strings"
	"testing"
)

// catalogoMatch: ids legibles («sector:salud») para que las pruebas se lean solas.
func catalogoMatch() map[string]*domain.OpcionBoard {
	pares := [][3]string{
		{"expertise", "sucesion", "Sucesión"}, {"expertise", "gobierno", "Gobierno corporativo"}, {"expertise", "esg", "ESG"},
		{"sector", "salud", "Salud"}, {"sector", "agro", "Agro"}, {"sector", "otro", "Otro"},
		{"tipo_organo", "junta", "Junta directiva"}, {"tipo_organo", "familia", "Consejo de familia"},
		{"dinastia", "cultura", "Cultura"}, {"dinastia", "gobierno", "Gobierno"},
		{"idioma", "es", "Español"}, {"idioma", "en", "Inglés"},
		{"modalidad", "presencial", "Presencial"}, {"modalidad", "virtual", "Virtual"},
		{"pais", "colombia", "Colombia"}, {"pais", "mexico", "México"}, {"pais", "remoto", "Remoto"}, {"pais", "otro", "Otro"},
	}
	m := map[string]*domain.OpcionBoard{}
	for _, p := range pares {
		id := p[0] + ":" + p[1]
		m[id] = &domain.OpcionBoard{ID: id, Catalogo: p[0], Codigo: p[1], Etiqueta: p[2]}
	}
	return m
}

func lista(ids ...string) []any {
	out := make([]any, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}

// solicitudMatch pide de todo, incluida la independencia y DINASTÍA.
func solicitudMatch() map[string]any {
	return map[string]any{
		"expertise_requerida":    lista("expertise:sucesion", "expertise:gobierno"),
		"sector_empresa":         "sector:agro",
		"organo_fortalecer":      "tipo_organo:junta",
		"requiere_independiente": true,
		"dinastia_prioritaria":   lista("dinastia:cultura"),
		"idiomas_requeridos":     lista("idioma:es", "idioma:en"),
		"modalidad_esperada":     lista("modalidad:virtual"),
		"pais_empresa":           "pais:mexico",
	}
}

// consejeroPerfecto cubre todo lo que pide solicitudMatch.
func consejeroPerfecto() map[string]any {
	return map[string]any{
		"expertise":         lista("expertise:sucesion", "expertise:gobierno"),
		"sectores":          lista("sector:agro", "sector:salud"),
		"tipos_organo":      lista("tipo_organo:junta"),
		"fue_independiente": true,
		"dinastia":          lista("dinastia:cultura"),
		"idiomas":           lista("idioma:es", "idioma:en"),
		"modalidad":         lista("modalidad:virtual", "modalidad:presencial"),
		"regiones":          lista("pais:mexico"),
	}
}

func TestPesosBoardSuman100(t *testing.T) {
	total := PesoExpertiseBoard + PesoSectorBoard + PesoOrganoBoard + PesoIndependenciaBoard +
		PesoDinastiaBoard + PesoIdiomasBoard + PesoModalidadBoard + PesoRegionBoard
	if total != 100 || PesoOrganoBoard != 20 || PesoIndependenciaBoard != 10 {
		t.Fatalf("los pesos suman %d; los del 2026-11-23 son 20/20/20/10/10/10/5/5", total)
	}
}

func TestCruzarBoard(t *testing.T) {
	op := catalogoMatch()

	t.Run("cubre todo: 100 y una razón por criterio", func(t *testing.T) {
		r := CruzarBoard(solicitudMatch(), consejeroPerfecto(), op)
		if r.Porcentaje != 100 || len(r.Razones) != 8 {
			t.Fatalf("%d %% con razones %v", r.Porcentaje, r.Razones)
		}
		if r.Razones[0] != "Expertise en Sucesión, Gobierno corporativo" || r.Razones[2] != "Ha servido en Junta directiva" ||
			r.Razones[3] != "Ha sido independiente, como se requiere" || r.Razones[7] != "Puede servir en México" {
			t.Fatalf("razones: %v", r.Razones)
		}
	})

	t.Run("no tiene nada: 0 y sin razones", func(t *testing.T) {
		r := CruzarBoard(solicitudMatch(), map[string]any{}, op)
		if r.Porcentaje != 0 || len(r.Razones) != 0 {
			t.Fatalf("%d %% con razones %v", r.Porcentaje, r.Razones)
		}
	})

	casos := []struct {
		nombre   string
		cambia   func(sol, con map[string]any)
		esperado int
	}{
		// 2 de 3 áreas: el criterio vale 2/3 de 20 = 13,33; el total, 93,33.
		{"cubre 2 de 3 áreas de expertise", func(sol, con map[string]any) {
			sol["expertise_requerida"] = lista("expertise:sucesion", "expertise:gobierno", "expertise:esg")
		}, 93},
		{"sin el sector pierde 20", func(sol, con map[string]any) { con["sectores"] = lista("sector:salud") }, 80},
		{"sin el órgano pierde 20", func(sol, con map[string]any) { con["tipos_organo"] = lista("tipo_organo:familia") }, 80},
		{"no fue independiente y se exige: pierde 10", func(sol, con map[string]any) { con["fue_independiente"] = false }, 90},
		// Errata F: si no se exige, no cuenta; con lo demás perfecto sigue en 100.
		{"no se exige independencia: no cuenta", func(sol, con map[string]any) {
			sol["requiere_independiente"] = false
			con["fue_independiente"] = false
		}, 100},
		// Sin DINASTÍA pedida, el denominador es 90 y el sector ausente pesa 20/90.
		{"sin DINASTÍA pedida se reescala", func(sol, con map[string]any) {
			delete(sol, "dinastia_prioritaria")
			con["sectores"] = lista("sector:salud")
		}, 78},
		{"habla uno de dos idiomas", func(sol, con map[string]any) { con["idiomas"] = lista("idioma:es") }, 95},
		{"otra modalidad", func(sol, con map[string]any) { con["modalidad"] = lista("modalidad:presencial") }, 95},
		{"otro país y no remoto", func(sol, con map[string]any) { con["regiones"] = lista("pais:colombia") }, 95},
		// Errata E: remoto vale si la empresa acepta virtual.
		{"remoto y la empresa acepta virtual", func(sol, con map[string]any) { con["regiones"] = lista("pais:remoto") }, 100},
		{"remoto pero la empresa solo quiere presencial", func(sol, con map[string]any) {
			con["regiones"] = lista("pais:remoto")
			sol["modalidad_esperada"] = lista("modalidad:presencial")
		}, 95},
		// Opción b del 2026-11-25: las áreas de los programas cuentan como expertise.
		{"la expertise que falta viene de sus programas", func(sol, con map[string]any) {
			con["expertise"] = lista("expertise:sucesion")
			con["programas_areas"] = lista("expertise:gobierno")
		}, 100},
		// Errata G: «Otro» con «Otro» no suma; pedido solo «Otro», el criterio no cuenta.
		{"empresa de sector Otro: el criterio no cuenta", func(sol, con map[string]any) {
			sol["sector_empresa"] = "sector:otro"
			con["sectores"] = lista("sector:otro")
		}, 100},
		{"empresa en país Otro: la región no cuenta", func(sol, con map[string]any) {
			sol["pais_empresa"] = "pais:otro"
			con["regiones"] = lista("pais:colombia")
		}, 100},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			sol, con := solicitudMatch(), consejeroPerfecto()
			c.cambia(sol, con)
			if r := CruzarBoard(sol, con, op); r.Porcentaje != c.esperado {
				t.Fatalf("%d %%, se esperaba %d (razones %v)", r.Porcentaje, c.esperado, r.Razones)
			}
		})
	}

	t.Run("la razón dice cuándo el área viene de los programas", func(t *testing.T) {
		con := consejeroPerfecto()
		con["expertise"] = lista("expertise:sucesion")
		con["programas_areas"] = lista("expertise:gobierno")
		r := CruzarBoard(solicitudMatch(), con, op)
		if !strings.Contains(r.Razones[0], "Gobierno corporativo (por sus programas de formación)") {
			t.Fatalf("razón: %v", r.Razones[0])
		}
	})

	t.Run("es determinista", func(t *testing.T) {
		a := CruzarBoard(solicitudMatch(), consejeroPerfecto(), op)
		b := CruzarBoard(solicitudMatch(), consejeroPerfecto(), op)
		if a.Porcentaje != b.Porcentaje || strings.Join(a.Razones, "|") != strings.Join(b.Razones, "|") {
			t.Fatal("dos cálculos iguales dieron resultados distintos")
		}
	})
}
