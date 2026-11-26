package services

import (
	"applegacy/backend/internal/core/domain"
	"math"
	"slices"
	"strings"
)

// El matching de Legacy Board (§8 del documento del cliente, fase 6 del plan
// reports/20261002_plan_legacy_board_formularios.md): cruza una solicitud del
// empresario (Formulario B) con la postulación de un consejero (Formulario A).
//
// Es determinista: misma solicitud y mismos consejeros, mismo resultado. No
// interviene ninguna IA (la §9 está fuera del alcance).
//
// Reglas, todas cerradas con el cliente:
//   - Pesos (2026-11-23): expertise 20, sector 20, órgano 20, independencia 10,
//     DINASTÍA 10, idiomas 10, modalidad 5 y región 5.
//   - En una lista, cuenta la fracción de lo pedido que cubre el consejero: si
//     se piden 3 áreas y tiene 2, el criterio vale 2/3 de su peso.
//   - Errata F: un criterio que el empresario no exige no cuenta, y el resto se
//     reescala a 100.
//   - Errata G: «Otro» nunca suma. Si el empresario pide justo «Otro», el
//     criterio no se puede evaluar y se trata como no exigido.
//   - Errata E: modalidad cuenta si coincide al menos una; región, si el país de
//     la empresa está entre las regiones del consejero, o si este marcó
//     «Remoto» y la empresa acepta trabajo virtual.
//   - 2026-11-25 (opción b): la expertise del consejero es la unión de «Áreas
//     de expertise» y «Áreas que cubrieron esos programas».
//   - Errata I: los honorarios no puntúan; se muestran junto al candidato.

const (
	PesoExpertiseBoard     = 20
	PesoSectorBoard        = 20
	PesoOrganoBoard        = 20
	PesoIndependenciaBoard = 10
	PesoDinastiaBoard      = 10
	PesoIdiomasBoard       = 10
	PesoModalidadBoard     = 5
	PesoRegionBoard        = 5
	codigoOtroBoard        = "otro"
	codigoRemotoBoard      = "remoto"
	codigoVirtualBoard     = "virtual"
)

// ResultadoMatchBoard es el cruce de una solicitud con un consejero.
type ResultadoMatchBoard struct {
	// Porcentaje de 0 a 100, redondeado.
	Porcentaje int
	// Razones: lo que coincide, en frases cortas para el comité.
	Razones []string
}

// idsDe lee una respuesta como lista de ids: una multiselección ([]any) o una
// selección suelta (string).
func idsDe(v any) []string {
	switch x := v.(type) {
	case string:
		if x == "" {
			return nil
		}
		return []string{x}
	case []any:
		ids := make([]string, 0, len(x))
		for _, e := range x {
			if s, ok := e.(string); ok && s != "" {
				ids = append(ids, s)
			}
		}
		return ids
	case []string:
		return x
	}
	return nil
}

type cruceBoard struct {
	opciones map[string]*domain.OpcionBoard
	puntos   float64
	pesos    float64
	razones  []string
}

func (c *cruceBoard) codigo(id string) string {
	if o, ok := c.opciones[id]; ok {
		return o.Codigo
	}
	return ""
}

func (c *cruceBoard) etiqueta(id string) string {
	if o, ok := c.opciones[id]; ok {
		return o.Etiqueta
	}
	return "?"
}

// sinOtro quita «Otro» de una lista: nunca suma (errata G).
func (c *cruceBoard) sinOtro(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if c.codigo(id) != codigoOtroBoard && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// sumar añade un criterio que sí se exige: su peso entra al denominador y la
// fracción cubierta, al numerador.
func (c *cruceBoard) sumar(peso int, fraccion float64, razon string) {
	c.pesos += float64(peso)
	c.puntos += float64(peso) * fraccion
	if fraccion > 0 && razon != "" {
		c.razones = append(c.razones, razon)
	}
}

// cubrir: qué parte de lo pedido tiene el consejero. Sin nada que pedir
// (vacío o solo «Otro»), el criterio no cuenta.
func (c *cruceBoard) cubrir(pedidos, tiene []string) (fraccion float64, cubiertos []string, cuenta bool) {
	pedidos = c.sinOtro(pedidos)
	if len(pedidos) == 0 {
		return 0, nil, false
	}
	tiene = c.sinOtro(tiene)
	for _, id := range pedidos {
		if slices.Contains(tiene, id) {
			cubiertos = append(cubiertos, id)
		}
	}
	return float64(len(cubiertos)) / float64(len(pedidos)), cubiertos, true
}

func (c *cruceBoard) etiquetas(ids []string) string {
	nombres := make([]string, len(ids))
	for i, id := range ids {
		nombres[i] = c.etiqueta(id)
	}
	return strings.Join(nombres, ", ")
}

// CruzarBoard calcula el match de una solicitud (respuestas del B) con un
// consejero (respuestas del A). opciones son todas las del catálogo, también
// las inactivas: lo ya enviado puede usarlas.
func CruzarBoard(solicitud, consejero map[string]any, opciones map[string]*domain.OpcionBoard) ResultadoMatchBoard {
	c := &cruceBoard{opciones: opciones}

	// Expertise: la del consejero más la de sus programas de formación.
	propias := idsDe(consejero["expertise"])
	deProgramas := idsDe(consejero["programas_areas"])
	tiene := append(slices.Clone(propias), deProgramas...)
	if f, cubiertos, ok := c.cubrir(idsDe(solicitud["expertise_requerida"]), tiene); ok {
		partes := make([]string, len(cubiertos))
		for i, id := range cubiertos {
			partes[i] = c.etiqueta(id)
			if !slices.Contains(propias, id) {
				partes[i] += " (por sus programas de formación)"
			}
		}
		c.sumar(PesoExpertiseBoard, f, "Expertise en "+strings.Join(partes, ", "))
	}

	if f, cubiertos, ok := c.cubrir(idsDe(solicitud["sector_empresa"]), idsDe(consejero["sectores"])); ok {
		c.sumar(PesoSectorBoard, f, "Ha trabajado en "+c.etiquetas(cubiertos))
	}

	if f, cubiertos, ok := c.cubrir(idsDe(solicitud["organo_fortalecer"]), idsDe(consejero["tipos_organo"])); ok {
		c.sumar(PesoOrganoBoard, f, "Ha servido en "+c.etiquetas(cubiertos))
	}

	// Independencia: solo cuenta si el empresario la exige.
	if requiere, _ := solicitud["requiere_independiente"].(bool); requiere {
		fue, _ := consejero["fue_independiente"].(bool)
		f := 0.0
		if fue {
			f = 1
		}
		c.sumar(PesoIndependenciaBoard, f, "Ha sido independiente, como se requiere")
	}

	if f, cubiertos, ok := c.cubrir(idsDe(solicitud["dinastia_prioritaria"]), idsDe(consejero["dinastia"])); ok {
		c.sumar(PesoDinastiaBoard, f, "DINASTÍA: "+c.etiquetas(cubiertos))
	}

	if f, cubiertos, ok := c.cubrir(idsDe(solicitud["idiomas_requeridos"]), idsDe(consejero["idiomas"])); ok {
		c.sumar(PesoIdiomasBoard, f, "Habla "+c.etiquetas(cubiertos))
	}

	// Modalidad: basta con que coincida una.
	modalidades := c.sinOtro(idsDe(solicitud["modalidad_esperada"]))
	if len(modalidades) > 0 {
		_, comunes, _ := c.cubrir(modalidades, idsDe(consejero["modalidad"]))
		f := 0.0
		if len(comunes) > 0 {
			f = 1
		}
		c.sumar(PesoModalidadBoard, f, "Modalidad "+c.etiquetas(comunes))
	}

	// Región: el país de la empresa entre sus regiones, o remoto + virtual.
	pais := c.sinOtro(idsDe(solicitud["pais_empresa"]))
	if len(pais) > 0 {
		regiones := idsDe(consejero["regiones"])
		f, razon := 0.0, ""
		if slices.Contains(regiones, pais[0]) {
			f, razon = 1, "Puede servir en "+c.etiqueta(pais[0])
		} else if slices.ContainsFunc(regiones, func(id string) bool { return c.codigo(id) == codigoRemotoBoard }) &&
			slices.ContainsFunc(modalidades, func(id string) bool { return c.codigo(id) == codigoVirtualBoard }) {
			f, razon = 1, "Trabaja en remoto y la empresa acepta virtual"
		}
		c.sumar(PesoRegionBoard, f, razon)
	}

	if c.pesos == 0 {
		return ResultadoMatchBoard{Porcentaje: 0}
	}
	return ResultadoMatchBoard{
		Porcentaje: int(math.Round(100 * c.puntos / c.pesos)),
		Razones:    c.razones,
	}
}
