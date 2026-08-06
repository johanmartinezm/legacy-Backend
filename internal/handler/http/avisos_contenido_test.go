package http

import (
	"applegacy/backend/internal/core/domain"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// stubContentService implementa lo justo de ports.ContentService. `anterior` es
// lo que devuelve GetContentByID, es decir cómo estaba el contenido antes de
// guardar: de ahí sale la decisión de avisar o no.
type stubContentService struct {
	anterior    *domain.CustomContent
	errAnterior error
}

func (s *stubContentService) ListCategories(ctx context.Context) ([]*domain.ContentCategory, error) {
	return nil, nil
}
func (s *stubContentService) CreateCategory(ctx context.Context, cat *domain.ContentCategory) error {
	return nil
}
func (s *stubContentService) UpdateCategory(ctx context.Context, cat *domain.ContentCategory) error {
	return nil
}
func (s *stubContentService) DeleteCategory(ctx context.Context, id string) error { return nil }
func (s *stubContentService) ListContent(ctx context.Context, slug string, soloPublicados bool) ([]*domain.CustomContent, error) {
	return nil, nil
}
func (s *stubContentService) GetContentByID(ctx context.Context, id string) (*domain.CustomContent, error) {
	if s.errAnterior != nil {
		return nil, s.errAnterior
	}
	return s.anterior, nil
}
func (s *stubContentService) CreateContent(ctx context.Context, c *domain.CustomContent) error {
	c.ID = "contenido-nuevo"
	return nil
}
func (s *stubContentService) UpdateContent(ctx context.Context, c *domain.CustomContent) error {
	return nil
}
func (s *stubContentService) DeleteContent(ctx context.Context, id string) error { return nil }

func peticionContenido(metodo, id string, cuerpo map[string]any) (*http.Request, *httptest.ResponseRecorder) {
	body, _ := json.Marshal(cuerpo)
	req := httptest.NewRequest(metodo, "/api/admin/content", bytes.NewReader(body))

	rctx := chi.NewRouteContext()
	if id != "" {
		rctx.URLParams.Add("id", id)
	}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, UserIDKey, "admin-1")
	return req.WithContext(ctx), httptest.NewRecorder()
}

func TestCrearContenido_AvisaSoloSiEstaPublicado(t *testing.T) {
	casos := []struct {
		nombre    string
		publicado bool
		avisos    int
	}{
		{"publicado avisa", true, 1},
		// Un borrador no interesa a nadie todavía, y avisar de él mandaría a la
		// app a una pantalla vacía.
		{"borrador no avisa", false, 0},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			notificador := &notificadorFalso{}
			h := NewContentHandler(&stubContentService{}, notificador)

			req, rec := peticionContenido(http.MethodPost, "", map[string]any{
				"title": "Cinco claves para invertir", "excerpt": "Un resumen breve",
				"is_published": c.publicado,
			})
			h.AdminCreateContent(rec, req)

			if rec.Code != http.StatusCreated {
				t.Fatalf("se esperaba 201, llegó %d", rec.Code)
			}
			if len(notificador.envios) != c.avisos {
				t.Fatalf("se esperaban %d avisos, hubo %d", c.avisos, len(notificador.envios))
			}
			if c.avisos > 0 {
				e := notificador.envios[0]
				if e.titulo != "Cinco claves para invertir" {
					t.Errorf("título inesperado: %q", e.titulo)
				}
				if e.datos["type"] != "content" {
					t.Errorf("los datos deben identificar el tipo, llegó %v", e.datos)
				}
			}
		})
	}
}

func TestActualizarContenido_AvisaSoloAlPublicarPorPrimeraVez(t *testing.T) {
	casos := []struct {
		nombre          string
		estabaPublicado bool
		quedaPublicado  bool
		avisos          int
	}{
		{"de borrador a publicado avisa", false, true, 1},
		// El caso que más fácil se rompe: sin mirar el estado anterior, corregir
		// una errata en un artículo publicado volvería a notificar a todos.
		{"editar uno ya publicado no avisa", true, true, 0},
		{"despublicar no avisa", true, false, 0},
		{"seguir en borrador no avisa", false, false, 0},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			notificador := &notificadorFalso{}
			svc := &stubContentService{
				anterior: &domain.CustomContent{ID: "c-1", IsPublished: c.estabaPublicado},
			}
			h := NewContentHandler(svc, notificador)

			req, rec := peticionContenido(http.MethodPut, "c-1", map[string]any{
				"title": "Cinco claves para invertir", "is_published": c.quedaPublicado,
			})
			h.AdminUpdateContent(rec, req)

			if len(notificador.envios) != c.avisos {
				t.Errorf("se esperaban %d avisos, hubo %d", c.avisos, len(notificador.envios))
			}
		})
	}
}

func TestActualizarContenido_SiNoSePuedeLeerElEstadoAnteriorNoAvisa(t *testing.T) {
	// Es preferible perder un aviso a repetirlo: un aviso duplicado llega a
	// todos los usuarios y no se puede retirar.
	notificador := &notificadorFalso{}
	svc := &stubContentService{errAnterior: errors.New("base caida")}
	h := NewContentHandler(svc, notificador)

	req, rec := peticionContenido(http.MethodPut, "c-1", map[string]any{
		"title": "Cinco claves", "is_published": true,
	})
	h.AdminUpdateContent(rec, req)

	if len(notificador.envios) != 0 {
		t.Errorf("no debe avisarse si no se conoce el estado anterior, hubo %d", len(notificador.envios))
	}
	// Y el guardado debe seguir funcionando: el aviso no puede bloquearlo.
	if rec.Code != http.StatusOK {
		t.Errorf("el contenido debe guardarse igual, llegó %d", rec.Code)
	}
}
