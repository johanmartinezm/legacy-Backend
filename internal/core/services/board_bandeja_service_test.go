package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
)

// Métodos de la bandeja del repositorio falso (la estructura está en
// board_formulario_service_test.go).
func (r *repoEnviosBoardFake) Listar(ctx context.Context, f domain.FiltroEnviosBoard, limit, offset int) ([]*domain.EnvioBoard, error) {
	r.filtroVisto = f
	var out []*domain.EnvioBoard
	for _, e := range r.guardados {
		out = append(out, e)
	}
	return out, nil
}

func (r *repoEnviosBoardFake) Contar(ctx context.Context, f domain.FiltroEnviosBoard) (int, error) {
	return len(r.guardados), nil
}

func (r *repoEnviosBoardFake) FindByID(ctx context.Context, id string) (*domain.EnvioBoard, error) {
	if e, ok := r.guardados[id]; ok {
		return e, nil
	}
	return nil, domain.ErrNotFound
}

func (r *repoEnviosBoardFake) CambiarEstado(ctx context.Context, id, estado, motivo, adminID string) error {
	e, ok := r.guardados[id]
	if !ok {
		return domain.ErrNotFound
	}
	e.Estado, e.EstadoMotivo = estado, motivo
	r.cambio = []string{id, estado, motivo, adminID}
	return nil
}

func (r *repoEnviosBoardFake) MarcarAvisoEnviado(ctx context.Context, id string) error {
	r.avisados = append(r.avisados, id)
	return nil
}

func (r *repoEnviosBoardFake) Destinatarios(ctx context.Context) ([]string, error) {
	return r.destinos, nil
}

func (r *repoEnviosBoardFake) ReemplazarDestinatarios(ctx context.Context, correos []string) error {
	r.destinos = correos
	return nil
}

// descifradorFake deshace lo que hace cifradorFake.
type descifradorFake struct{}

func (descifradorFake) Decrypt(s string) (string, error) {
	return strings.TrimSuffix(strings.TrimPrefix(s, "CIFRADO("), ")"), nil
}

type correoBoardFake struct {
	mu      sync.Mutex
	nuevos  []string
	estados []string
	fallaA  string
}

func (c *correoBoardFake) SendBoardEnvioNuevoEmail(to, titulo, nombre, correo string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if to == c.fallaA {
		return errors.New("smtp caído")
	}
	c.nuevos = append(c.nuevos, to+"|"+nombre+"|"+correo)
	return nil
}

func (c *correoBoardFake) SendBoardEstadoEmail(to, nombre, titulo, estado string) error {
	c.estados = append(c.estados, to+"|"+estado)
	return nil
}

type pushFake struct{ cuerpos, datos []string }

func (p *pushFake) SendToUser(ctx context.Context, userID, title, body string, data map[string]string) error {
	p.cuerpos = append(p.cuerpos, userID+"|"+body)
	p.datos = append(p.datos, data["type"]+"|"+data["id"])
	return nil
}

func bandejaPrueba(t *testing.T) (*BoardBandejaService, *repoEnviosBoardFake, *correoBoardFake, *pushFake) {
	t.Helper()
	datos, _ := json.Marshal(map[string]any{
		"nombre_completo": "Ana Pérez", "correo": "ana@empresa.co", "telefono": "+57 300 123 4567",
		"empresas_contextos": "Grupo X", "fecha_nacimiento": "1975-04-30",
	})
	repo := &repoEnviosBoardFake{guardados: map[string]*domain.EnvioBoard{
		"e1": {
			ID: "e1", Formulario: "A", UserID: "u1", Estado: "pendiente", FotoURL: "/api/images/x.jpg",
			DatosCifrados: "CIFRADO(" + string(datos) + ")",
			Respuestas: map[string]any{
				"sectores": []any{"sector:salud", "sector:borrada"}, "nivel_cargo": "rol:presidente",
				"fue_independiente": false, "anios_experiencia": float64(20),
			},
		},
	}}
	correo, push := &correoBoardFake{}, &pushFake{}
	s := NewBoardBandejaService(repo, opcionesA(), descifradorFake{}, correo, push, "contacto@legacy.co")
	s.enSegundoPlano = func(f func()) { f() }
	return s, repo, correo, push
}

func TestBandejaListarYDetalle(t *testing.T) {
	ctx := context.Background()
	s, repo, _, _ := bandejaPrueba(t)

	lista, total, err := s.Listar(ctx, domain.FiltroEnviosBoard{Formulario: "A", Campo: "sector", Opcion: "sector:salud"}, 25, 0)
	if err != nil || total != 1 || lista[0].Nombre != "Ana Pérez" || lista[0].Correo != "ana@empresa.co" {
		t.Fatalf("lista: %+v total=%d err=%v", lista, total, err)
	}
	if repo.filtroVisto.Campo != "sectores" || !repo.filtroVisto.Multiple {
		t.Fatalf("«sector» debía traducirse a «sectores», multiselección: %+v", repo.filtroVisto)
	}
	for _, f := range []domain.FiltroEnviosBoard{
		{Formulario: "Z"},
		{Formulario: "A", Estado: "borrada"},
		{Formulario: "A", Campo: "nombre_completo", Opcion: "x"},
	} {
		if _, _, err := s.Listar(ctx, f, 25, 0); err == nil {
			t.Errorf("filtro %+v debía rechazarse", f)
		}
	}

	d, err := s.Detalle(ctx, "e1")
	if err != nil {
		t.Fatal(err)
	}
	valores := map[string]domain.RespuestaLegibleBoard{}
	for _, sec := range d.Secciones {
		for _, r := range sec.Respuestas {
			valores[r.Clave] = r
		}
	}
	casos := map[string]string{
		"nombre_completo":    "Ana Pérez",
		"empresas_contextos": "Grupo X",
		"fecha_nacimiento":   "30/04/1975",
		"nivel_cargo":        "PRESIDENTE",
		"fue_independiente":  "No",
		"anios_experiencia":  "20",
		"sectores":           "SALUD, (opción borrada)",
		"linkedin":           "",
	}
	for clave, esperado := range casos {
		if valores[clave].Valor != esperado {
			t.Errorf("%s = %q, se esperaba %q", clave, valores[clave].Valor, esperado)
		}
	}
	if !valores["empresas_contextos"].Confidencial || valores["sectores"].Confidencial {
		t.Error("la marca de confidencial no corresponde")
	}
	if _, ok := valores["foto"]; ok || d.FotoURL != "/api/images/x.jpg" {
		t.Error("la foto debía ir en FotoURL y no entre las respuestas")
	}
	if _, err := s.Detalle(ctx, "nada"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("id inexistente: %v", err)
	}
}

func TestBandejaCambiarEstado(t *testing.T) {
	ctx := context.Background()

	t.Run("rechazo sin motivo, estado inventado y motivo largo", func(t *testing.T) {
		s, _, _, _ := bandejaPrueba(t)
		for estado, motivo := range map[string]string{"rechazada": "  ", "borrada": "x", "aprobada": strings.Repeat("a", 1001)} {
			if _, err := s.CambiarEstado(ctx, "e1", estado, motivo, "adm"); !EsErrorDeValidacionBandejaBoard(err) {
				t.Errorf("%s/%q: se esperaba error de validación, llegó %v", estado, motivo, err)
			}
		}
	})

	t.Run("rechazar guarda el motivo y avisa sin él", func(t *testing.T) {
		s, repo, correo, push := bandejaPrueba(t)
		d, err := s.CambiarEstado(ctx, "e1", "rechazada", " Perfil sin experiencia en juntas ", "adm")
		if err != nil {
			t.Fatal(err)
		}
		if repo.cambio[2] != "Perfil sin experiencia en juntas" || repo.cambio[3] != "adm" || d.Estado != "rechazada" {
			t.Fatalf("cambio guardado: %v / %+v", repo.cambio, d)
		}
		if len(push.cuerpos) != 1 || strings.Contains(push.cuerpos[0], "Perfil") ||
			!strings.Contains(push.cuerpos[0], "no fue aprobada") || push.datos[0] != "board_envio|A" {
			t.Fatalf("push: %v %v", push.cuerpos, push.datos)
		}
		if len(correo.estados) != 1 || correo.estados[0] != "ana@empresa.co|rechazada" {
			t.Fatalf("correo: %v", correo.estados)
		}
		if _, err := s.CambiarEstado(ctx, "e1", "rechazada", "otra vez", "adm"); !errors.Is(err, ErrEstadoBoardIgual) {
			t.Fatalf("repetir estado: %v", err)
		}
	})

	t.Run("aprobar no exige motivo", func(t *testing.T) {
		s, _, _, push := bandejaPrueba(t)
		if _, err := s.CambiarEstado(ctx, "e1", "aprobada", "", "adm"); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(push.cuerpos[0], "aprobó tu postulación como consejero") {
			t.Fatalf("push: %v", push.cuerpos)
		}
	})
}

func TestBandejaAvisoNuevoYDestinatarios(t *testing.T) {
	ctx := context.Background()

	t.Run("sin destinatarios va al buzón de contacto", func(t *testing.T) {
		s, repo, correo, _ := bandejaPrueba(t)
		s.AvisarEnvioNuevo(repo.guardados["e1"], "Ana Pérez", "ana@empresa.co")
		if len(correo.nuevos) != 1 || correo.nuevos[0] != "contacto@legacy.co|Ana Pérez|ana@empresa.co" || len(repo.avisados) != 1 {
			t.Fatalf("aviso: %v / marcados %v", correo.nuevos, repo.avisados)
		}
	})

	t.Run("si falla uno, no se marca como avisado", func(t *testing.T) {
		s, repo, correo, _ := bandejaPrueba(t)
		repo.destinos = []string{"a@x.co", "b@x.co"}
		correo.fallaA = "b@x.co"
		s.AvisarEnvioNuevo(repo.guardados["e1"], "Ana", "ana@x.co")
		if len(correo.nuevos) != 1 || len(repo.avisados) != 0 {
			t.Fatalf("aviso: %v / marcados %v", correo.nuevos, repo.avisados)
		}
	})

	t.Run("guardar normaliza, quita repetidos y valida", func(t *testing.T) {
		s, repo, _, _ := bandejaPrueba(t)
		got, err := s.GuardarDestinatarios(ctx, []string{" Paola@Legacy.co ", "paola@legacy.co", "", "comite@legacy.co"})
		if err != nil || strings.Join(got, ",") != "paola@legacy.co,comite@legacy.co" || len(repo.destinos) != 2 {
			t.Fatalf("guardados %v err %v", got, err)
		}
		for _, malo := range []string{"sin-arroba", "a@b", "Ana <a@b.co>"} {
			if _, err := s.GuardarDestinatarios(ctx, []string{malo}); !errors.Is(err, ErrDestinatarioBoard) {
				t.Errorf("%q debía rechazarse: %v", malo, err)
			}
		}
		muchos := make([]string, 21)
		for i := range muchos {
			muchos[i] = strings.Repeat("a", i+1) + "@x.co"
		}
		if _, err := s.GuardarDestinatarios(ctx, muchos); !errors.Is(err, ErrDestinatariosBoardMucho) {
			t.Errorf("21 destinatarios: %v", err)
		}
		if got, err := s.GuardarDestinatarios(ctx, nil); err != nil || len(got) != 0 {
			t.Errorf("vaciar la lista debía valer: %v %v", got, err)
		}
	})

	t.Run("el envío del formulario avisa al comité con nombre y correo", func(t *testing.T) {
		var visto []string
		f := servicioFormA(&repoEnviosBoardFake{}).ConAvisoNuevo(func(e *domain.EnvioBoard, nombre, correo string) {
			visto = []string{e.Formulario, nombre, correo}
		})
		if _, err := f.Enviar(ctx, "u1", "A", entrada(t, respuestasA(), true)); err != nil {
			t.Fatal(err)
		}
		if strings.Join(visto, "|") != "A|Ana Pérez|ana@empresa.co" {
			t.Fatalf("aviso: %v", visto)
		}
	})
}
