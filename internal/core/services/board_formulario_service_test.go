package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

type repoEnviosBoardFake struct {
	actual *domain.EnvioBoard
	creado *domain.EnvioBoard
	choque bool
}

func (r *repoEnviosBoardFake) Create(ctx context.Context, e *domain.EnvioBoard) error {
	if r.choque {
		return fmt.Errorf("%w: uq_envios_un_formulario_a", domain.ErrUniqueViolation)
	}
	e.ID = "nuevo"
	e.Estado = domain.EstadoEnvioPendiente
	r.creado = e
	return nil
}

func (r *repoEnviosBoardFake) Actual(ctx context.Context, userID, formulario string) (*domain.EnvioBoard, error) {
	if r.actual == nil {
		return nil, domain.ErrNotFound
	}
	return r.actual, nil
}

// cifradorFake marca el texto para poder comprobar qué se cifró.
type cifradorFake struct{}

func (cifradorFake) Encrypt(s string) (string, error) { return "CIFRADO(" + s + ")", nil }

// opcionesA: una o dos opciones por cada lista que usa el Formulario A. El id
// es «catalogo:codigo» para que las pruebas se lean solas.
func opcionesA() *repoOpcionesBoardFake {
	pares := [][2]string{
		{"tipo_documento", "cc"}, {"pais", "colombia"}, {"pais", "mexico"}, {"pais", "remoto"}, {"pais", "otro"},
		{"cliente_alumni", "lso"}, {"rol", "presidente"}, {"sector", "salud"}, {"sector", "agro"},
		{"idioma", "es"}, {"tipo_organo", "junta_directiva"}, {"tipo_organo", "no_aplica"},
		{"expertise", "sucesion"}, {"dinastia", "cultura"}, {"disponibilidad", "1_2"},
		{"modalidad", "virtual"}, {"honorarios", "r1"},
	}
	repo := &repoOpcionesBoardFake{}
	for _, p := range pares {
		repo.opciones = append(repo.opciones, &domain.OpcionBoard{
			ID: p[0] + ":" + p[1], Catalogo: p[0], Codigo: p[1], Etiqueta: strings.ToUpper(p[1]), Activo: true,
		})
	}
	repo.opciones = append(repo.opciones, &domain.OpcionBoard{ID: "sector:vieja", Catalogo: "sector", Codigo: "vieja", Activo: false})
	return repo
}

// respuestasA es un Formulario A válido, con todos los obligatorios.
func respuestasA() map[string]any {
	return map[string]any{
		"nombre_completo":     "  Ana   Pérez ",
		"correo":              "ana@empresa.co",
		"telefono":            "+57 300 123 4567",
		"pais_residencia":     "pais:colombia",
		"ciudad_residencia":   "Bogotá",
		"foto":                "/api/images/0f8fad5b-d9cb-469f-a165-70867728950e_foto.jpg",
		"cliente_alumni":      "cliente_alumni:lso",
		"formacion":           "Administración de empresas.\nMBA.",
		"nivel_cargo":         "rol:presidente",
		"cargo_actual":        "Presidente de junta",
		"anios_experiencia":   20,
		"sectores":            []string{"sector:salud", "sector:agro"},
		"idiomas":             []string{"idioma:es"},
		"tipos_organo":        []string{"tipo_organo:junta_directiva"},
		"num_juntas":          2,
		"num_comites":         0,
		"fue_independiente":   true,
		"empresas_contextos":  "Grupo X (confidencial)",
		"programas_formacion": "Programa de juntas directivas, 2019",
		"programas_areas":     []string{"expertise:sucesion"},
		"expertise":           []string{"expertise:sucesion"},
		"aporte":              "Experiencia en sucesión.",
		"disponibilidad":      "disponibilidad:1_2",
		"modalidad":           []string{"modalidad:virtual"},
		"regiones":            []string{"pais:colombia", "pais:remoto"},
		"honorarios":          "honorarios:r1",
	}
}

func entrada(t *testing.T, respuestas map[string]any, terminos bool) ports.EntradaEnvioBoard {
	t.Helper()
	in := ports.EntradaEnvioBoard{Respuestas: map[string]json.RawMessage{}, AceptaTerminos: terminos}
	for k, v := range respuestas {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		in.Respuestas[k] = b
	}
	return in
}

func servicioFormA(envios *repoEnviosBoardFake) *BoardFormularioService {
	s := NewBoardFormularioService(opcionesA(), envios, cifradorFake{}, func(nombre string) bool {
		return strings.HasPrefix(nombre, "0f8fad5b-")
	})
	s.ahora = func() time.Time { return time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC) }
	return s
}

func TestFormularioATiene32Campos(t *testing.T) {
	n := 0
	for _, s := range domain.FormularioA.Secciones {
		n += len(s.Campos)
	}
	// 31 campos más la casilla de términos, que va aparte (acepta_terminos):
	// los 31 del 2026-11-23 y las áreas de los programas del 2026-11-25.
	if n+1 != 32 {
		t.Fatalf("el Formulario A tiene %d campos más los términos; el plan dice 32", n)
	}
}

func TestEnviarFormularioAValido(t *testing.T) {
	envios := &repoEnviosBoardFake{}
	envio, err := servicioFormA(envios).Enviar(context.Background(), "u1", "A", entrada(t, respuestasA(), true))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if envio.UserID != "u1" || envio.Formulario != "A" || envio.FotoURL == "" {
		t.Fatalf("envío incompleto: %+v", envio)
	}
	r := envio.Respuestas
	if r["anios_experiencia"] != 20 || r["fue_independiente"] != true || r["cargo_actual"] != "Presidente de junta" {
		t.Fatalf("respuestas en claro incorrectas: %+v", r)
	}
	if r["formacion"] != "Administración de empresas.\nMBA." {
		t.Fatalf("el texto largo perdió los saltos de línea: %q", r["formacion"])
	}
	// Lo personal y lo confidencial no puede quedar en claro.
	for _, clave := range []string{"nombre_completo", "correo", "telefono", "empresas_contextos", "foto"} {
		if _, ok := r[clave]; ok {
			t.Errorf("%s quedó en las respuestas en claro", clave)
		}
	}
	if !strings.HasPrefix(envio.DatosCifrados, "CIFRADO(") ||
		!strings.Contains(envio.DatosCifrados, `"nombre_completo":"Ana Pérez"`) ||
		!strings.Contains(envio.DatosCifrados, `"empresas_contextos"`) {
		t.Fatalf("datos cifrados incorrectos: %s", envio.DatosCifrados)
	}
	// Los opcionales que no vinieron no aparecen.
	if _, ok := r["dinastia"]; ok {
		t.Error("dinastia no vino y aparece")
	}
}

func TestEnviarFormularioAErrores(t *testing.T) {
	casos := []struct {
		nombre string
		cambia func(map[string]any)
		campo  string
	}{
		{"falta un obligatorio", func(r map[string]any) { delete(r, "aporte") }, "aporte"},
		{"obligatorio en blanco", func(r map[string]any) { r["cargo_actual"] = "   " }, "cargo_actual"},
		{"lista obligatoria vacía", func(r map[string]any) { r["sectores"] = []string{} }, "sectores"},
		{"aporte de más de 600", func(r map[string]any) { r["aporte"] = strings.Repeat("a", 601) }, "aporte"},
		{"correo inválido", func(r map[string]any) { r["correo"] = "ana@" }, "correo"},
		{"correo con nombre", func(r map[string]any) { r["correo"] = "Ana <ana@x.co>" }, "correo"},
		{"teléfono corto", func(r map[string]any) { r["telefono"] = "123" }, "telefono"},
		{"teléfono con letras", func(r map[string]any) { r["telefono"] = "+57 300 ABC 4567" }, "telefono"},
		{"años fuera de rango", func(r map[string]any) { r["anios_experiencia"] = 61 }, "anios_experiencia"},
		{"años con decimales", func(r map[string]any) { r["anios_experiencia"] = 2.5 }, "anios_experiencia"},
		{"juntas como texto", func(r map[string]any) { r["num_juntas"] = "2" }, "num_juntas"},
		{"sí/no como texto", func(r map[string]any) { r["fue_independiente"] = "sí" }, "fue_independiente"},
		{"opción de otra lista", func(r map[string]any) { r["nivel_cargo"] = "sector:salud" }, "nivel_cargo"},
		{"opción desactivada", func(r map[string]any) { r["sectores"] = []string{"sector:vieja"} }, "sectores"},
		{"opción inventada", func(r map[string]any) { r["honorarios"] = "honorarios:r9" }, "honorarios"},
		{"remoto como residencia", func(r map[string]any) { r["pais_residencia"] = "pais:remoto" }, "pais_residencia"},
		{"otro como región", func(r map[string]any) { r["regiones"] = []string{"pais:otro"} }, "regiones"},
		{"opción repetida", func(r map[string]any) { r["idiomas"] = []string{"idioma:es", "idioma:es"} }, "idiomas"},
		{"áreas de programas de otra lista", func(r map[string]any) { r["programas_areas"] = []string{"sector:salud"} }, "programas_areas"},
		{"faltan las áreas de los programas", func(r map[string]any) { delete(r, "programas_areas") }, "programas_areas"},
		{"no aplica con otras", func(r map[string]any) {
			r["tipos_organo"] = []string{"tipo_organo:no_aplica", "tipo_organo:junta_directiva"}
		}, "tipos_organo"},
		{"linkedin de otro dominio", func(r map[string]any) { r["linkedin"] = "https://linkedin.com.evil.co/in/ana" }, "linkedin"},
		{"linkedin sin esquema", func(r map[string]any) { r["linkedin"] = "linkedin.com/in/ana" }, "linkedin"},
		{"fecha futura", func(r map[string]any) { r["fecha_nacimiento"] = "2030-01-01" }, "fecha_nacimiento"},
		{"fecha mal escrita", func(r map[string]any) { r["fecha_nacimiento"] = "01/02/1980" }, "fecha_nacimiento"},
		{"documento con símbolos", func(r map[string]any) { r["numero_documento"] = "123;DROP" }, "numero_documento"},
		{"foto de otro sitio", func(r map[string]any) { r["foto"] = "https://evil.co/x.jpg" }, "foto"},
		{"foto que no se subió", func(r map[string]any) {
			r["foto"] = "/api/images/aaaaaaaa-d9cb-469f-a165-70867728950e_x.jpg"
		}, "foto"},
		{"foto con ruta", func(r map[string]any) { r["foto"] = "/api/images/../config.yaml" }, "foto"},
		{"campo desconocido", func(r map[string]any) { r["es_plus"] = true }, "es_plus"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			r := respuestasA()
			c.cambia(r)
			_, err := servicioFormA(&repoEnviosBoardFake{}).Enviar(context.Background(), "u1", "A", entrada(t, r, true))
			var ec *ErrCampoBoard
			if !errors.As(err, &ec) {
				t.Fatalf("se esperaba un error de campo, llegó %v", err)
			}
			if ec.Campo != c.campo {
				t.Fatalf("error en %q (%s), se esperaba en %q", ec.Campo, ec.Mensaje, c.campo)
			}
			if !EsErrorDeValidacionFormularioBoard(err) {
				t.Fatal("debía contar como error de validación")
			}
		})
	}
}

func TestEnviarFormularioAReglas(t *testing.T) {
	ctx := context.Background()

	t.Run("opcionales válidos se guardan", func(t *testing.T) {
		r := respuestasA()
		r["fecha_nacimiento"] = "1975-04-30"
		r["linkedin"] = "https://www.linkedin.com/in/ana"
		r["tipo_documento"] = "tipo_documento:cc"
		r["numero_documento"] = "1.020.304"
		r["dinastia"] = []string{"dinastia:cultura"}
		r["tipos_organo"] = []string{"tipo_organo:no_aplica"}
		envios := &repoEnviosBoardFake{}
		if _, err := servicioFormA(envios).Enviar(ctx, "u1", "A", entrada(t, r, true)); err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if envios.creado.Respuestas["linkedin"] != "https://www.linkedin.com/in/ana" ||
			!strings.Contains(envios.creado.DatosCifrados, "1975-04-30") {
			t.Fatalf("faltan opcionales: %+v / %s", envios.creado.Respuestas, envios.creado.DatosCifrados)
		}
	})

	t.Run("sin aceptar términos", func(t *testing.T) {
		_, err := servicioFormA(&repoEnviosBoardFake{}).Enviar(ctx, "u1", "A", entrada(t, respuestasA(), false))
		if !errors.Is(err, ErrFormularioBoardTerminos) {
			t.Fatalf("error %v, se esperaba el de términos", err)
		}
	})

	t.Run("ya enviado", func(t *testing.T) {
		envios := &repoEnviosBoardFake{actual: &domain.EnvioBoard{Estado: "pendiente"}}
		_, err := servicioFormA(envios).Enviar(ctx, "u1", "A", entrada(t, respuestasA(), true))
		if !errors.Is(err, ErrFormularioBoardEnviado) {
			t.Fatalf("error %v, se esperaba ya enviado", err)
		}
	})

	t.Run("dos envíos a la vez", func(t *testing.T) {
		_, err := servicioFormA(&repoEnviosBoardFake{choque: true}).Enviar(ctx, "u1", "A", entrada(t, respuestasA(), true))
		if !errors.Is(err, ErrFormularioBoardEnviado) {
			t.Fatalf("error %v, se esperaba ya enviado", err)
		}
	})

	t.Run("formulario inexistente", func(t *testing.T) {
		_, err := servicioFormA(&repoEnviosBoardFake{}).Enviar(ctx, "u1", "Z", entrada(t, respuestasA(), true))
		if !errors.Is(err, ErrFormularioBoardNoExiste) {
			t.Fatalf("error %v", err)
		}
	})
}

func TestServirFormularioA(t *testing.T) {
	envios := &repoEnviosBoardFake{}
	f, err := servicioFormA(envios).Formulario(context.Background(), "u1", "A")
	if err != nil {
		t.Fatal(err)
	}
	campos := map[string]domain.CampoBoard{}
	for _, s := range f.Secciones {
		for _, c := range s.Campos {
			campos[c.Clave] = c
		}
	}
	codigos := func(c domain.CampoBoard) []string {
		var out []string
		for _, o := range c.Opciones {
			out = append(out, o.Codigo)
		}
		return out
	}
	if got := codigos(campos["pais_residencia"]); strings.Contains(strings.Join(got, ","), "remoto") {
		t.Fatalf("el país de residencia ofrece «remoto»: %v", got)
	}
	if got := strings.Join(codigos(campos["regiones"]), ","); !strings.Contains(got, "remoto") || strings.Contains(got, "otro") {
		t.Fatalf("regiones debía ofrecer «remoto» y no «otro»: %v", got)
	}
	if got := codigos(campos["sectores"]); len(got) != 2 {
		t.Fatalf("sectores debía traer solo las 2 activas: %v", got)
	}
	if f.EnvioActual != nil {
		t.Fatal("sin envío, envio_actual debía ser null")
	}
	// Servir no puede tocar la configuración global.
	if domain.FormularioA.Secciones[0].Campos[6].Opciones != nil {
		t.Fatal("Formulario modificó domain.FormularioA")
	}

	envios.actual = &domain.EnvioBoard{ID: "e1", Estado: "aprobada"}
	f, _ = servicioFormA(envios).Formulario(context.Background(), "u1", "A")
	if f.EnvioActual == nil || f.EnvioActual.Estado != "aprobada" {
		t.Fatalf("debía traer el envío actual: %+v", f.EnvioActual)
	}
}
