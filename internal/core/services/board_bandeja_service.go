package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/mail"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// La bandeja del panel para los formularios de Legacy Board (fase 5 del plan
// reports/20261002_plan_legacy_board_formularios.md): la lista, el detalle
// descifrado, el cambio de estado con sus avisos y los destinatarios del aviso
// de cada envío nuevo.

const (
	maxMotivoBoard        = 1000
	maxDestinatariosBoard = 20
	// TipoAvisoBoard es el `type` del push; la app lo traduce a una pantalla en
	// domain/utils/notificacion_destino.dart.
	TipoAvisoBoard = "board_envio"
)

var (
	ErrEstadoBoardInvalido     = errors.New("el estado debe ser «aprobada» o «rechazada»")
	ErrEstadoBoardSinMotivo    = errors.New("para rechazar hay que escribir el motivo")
	ErrEstadoBoardMotivoLargo  = fmt.Errorf("el motivo admite hasta %d caracteres", maxMotivoBoard)
	ErrEstadoBoardIgual        = errors.New("el envío ya está en ese estado")
	ErrFiltroBoardInvalido     = errors.New("filtro no válido para este formulario")
	ErrDestinatarioBoard       = errors.New("hay un correo con formato no válido")
	ErrDestinatariosBoardMucho = fmt.Errorf("se admiten hasta %d destinatarios", maxDestinatariosBoard)
	// ErrMatchingNoDisponible: el matching es de solicitudes (B) aprobadas
	// (decisión 2 del plan).
	ErrMatchingNoDisponible = errors.New("los candidatos se calculan solo para solicitudes de empresarios aprobadas")
)

// EsErrorDeValidacionBandejaBoard dice si el error es de la petición (400).
func EsErrorDeValidacionBandejaBoard(err error) bool {
	for _, e := range []error{ErrEstadoBoardInvalido, ErrEstadoBoardSinMotivo, ErrEstadoBoardMotivoLargo,
		ErrFiltroBoardInvalido, ErrDestinatarioBoard, ErrDestinatariosBoardMucho, ErrMatchingNoDisponible} {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}

// correoBoard es lo que la bandeja usa del servicio de correo. Va aparte de
// ports.EmailService para no obligar a cada implementación falsa de las pruebas
// a implementar estos dos métodos.
type correoBoard interface {
	SendBoardEnvioNuevoEmail(to, titulo, nombre, correo string) error
	SendBoardEstadoEmail(to, nombre, titulo, estado string) error
}

type descifradorBoard interface {
	Decrypt(texto string) (string, error)
}

type avisadorBoard interface {
	SendToUser(ctx context.Context, userID, title, body string, data map[string]string) error
}

type BoardBandejaService struct {
	envios   ports.BoardEnvioRepository
	opciones ports.BoardOpcionRepository
	crypto   descifradorBoard
	correo   correoBoard
	push     avisadorBoard
	// buzon recibe el aviso de un envío nuevo cuando el panel no tiene ningún
	// destinatario cargado, para que ninguno se pierda.
	buzon string
	// enSegundoPlano corre los avisos; las pruebas lo cambian para esperarlos.
	enSegundoPlano func(func())
}

func NewBoardBandejaService(envios ports.BoardEnvioRepository, opciones ports.BoardOpcionRepository,
	crypto descifradorBoard, correo correoBoard, push avisadorBoard, buzon string) *BoardBandejaService {
	return &BoardBandejaService{envios: envios, opciones: opciones, crypto: crypto, correo: correo, push: push,
		buzon: buzon, enSegundoPlano: func(f func()) { go f() }}
}

// datos descifra el JSON de datos_cifrados. Si falla devuelve un mapa vacío:
// el panel muestra el envío con esos campos en blanco en vez de no mostrarlo.
func (s *BoardBandejaService) datos(e *domain.EnvioBoard) map[string]any {
	datos := map[string]any{}
	if e.DatosCifrados == "" {
		return datos
	}
	claro, err := s.crypto.Decrypt(e.DatosCifrados)
	if err != nil {
		log.Printf("board bandeja: no se pudo descifrar el envío %s: %v", e.ID, err)
		return datos
	}
	if err := json.Unmarshal([]byte(claro), &datos); err != nil {
		log.Printf("board bandeja: datos del envío %s ilegibles: %v", e.ID, err)
	}
	return datos
}

func texto(v any) string {
	s, _ := v.(string)
	return s
}

func (s *BoardBandejaService) resumen(e *domain.EnvioBoard, datos map[string]any) *domain.ResumenEnvioBoard {
	return &domain.ResumenEnvioBoard{
		ID: e.ID, Formulario: e.Formulario, Estado: e.Estado, AvisoEnviado: e.AvisoEnviado,
		CreadoEn: e.CreadoEn, EstadoCambiadoEn: e.EstadoCambiadoEn,
		Nombre: texto(datos["nombre_completo"]), Correo: texto(datos["correo"]),
	}
}

// Listar traduce los filtros «sector» y «expertise» al campo del formulario.
func (s *BoardBandejaService) Listar(ctx context.Context, f domain.FiltroEnviosBoard, limit, offset int) ([]*domain.ResumenEnvioBoard, int, error) {
	form, ok := domain.FormulariosBoard[f.Formulario]
	if !ok {
		return nil, 0, ErrFormularioBoardNoExiste
	}
	if f.Estado != "" && !slices.Contains([]string{domain.EstadoEnvioPendiente, domain.EstadoEnvioAprobada, domain.EstadoEnvioRechazada}, f.Estado) {
		return nil, 0, ErrFiltroBoardInvalido
	}
	if f.Campo != "" {
		campo, ok := domain.CamposFiltroBoard[f.Formulario][f.Campo]
		if !ok {
			return nil, 0, ErrFiltroBoardInvalido
		}
		f.Campo = campo
		f.Multiple = campoDe(form, campo).Tipo == domain.CampoMultiseleccion
	}
	envios, err := s.envios.Listar(ctx, f, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.envios.Contar(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	lista := make([]*domain.ResumenEnvioBoard, 0, len(envios))
	for _, e := range envios {
		lista = append(lista, s.resumen(e, s.datos(e)))
	}
	return lista, total, nil
}

func campoDe(f *domain.FormularioBoard, clave string) domain.CampoBoard {
	for _, sec := range f.Secciones {
		for _, c := range sec.Campos {
			if c.Clave == clave {
				return c
			}
		}
	}
	return domain.CampoBoard{}
}

// Detalle traduce cada respuesta a lo que se lee: etiquetas de opción (también
// de las desactivadas, que siguen valiendo en lo ya enviado), «Sí»/«No» y fechas.
func (s *BoardBandejaService) Detalle(ctx context.Context, id string) (*domain.DetalleEnvioBoard, error) {
	e, err := s.envios.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	form, ok := domain.FormulariosBoard[e.Formulario]
	if !ok {
		return nil, ErrFormularioBoardNoExiste
	}
	todas, err := s.opciones.List(ctx, "", false)
	if err != nil {
		return nil, err
	}
	etiquetas := make(map[string]string, len(todas))
	for _, o := range todas {
		etiquetas[o.ID] = o.Etiqueta
	}
	datos := s.datos(e)

	d := &domain.DetalleEnvioBoard{
		ResumenEnvioBoard: *s.resumen(e, datos),
		Titulo:            form.Titulo,
		UserID:            e.UserID,
		FotoURL:           e.FotoURL,
		EstadoMotivo:      e.EstadoMotivo,
		EstadoCambiadoPor: e.EstadoCambiadoPorStr,
	}
	for _, sec := range form.Secciones {
		legible := domain.SeccionLegibleBoard{Titulo: sec.Titulo}
		for _, c := range sec.Campos {
			if c.Tipo == domain.CampoFoto {
				continue // va en FotoURL
			}
			valor := e.Respuestas[c.Clave]
			if c.Cifrado {
				valor = datos[c.Clave]
			}
			r := domain.RespuestaLegibleBoard{Clave: c.Clave, Etiqueta: c.Etiqueta, Tipo: c.Tipo, Confidencial: c.Cifrado}
			switch c.Tipo {
			case domain.CampoMultiseleccion:
				lista, _ := valor.([]any)
				for _, v := range lista {
					r.Valores = append(r.Valores, etiquetaDe(etiquetas, texto(v)))
				}
				r.Valor = strings.Join(r.Valores, ", ")
			case domain.CampoSeleccion:
				if id := texto(valor); id != "" {
					r.Valor = etiquetaDe(etiquetas, id)
				}
			case domain.CampoSiNo:
				if b, ok := valor.(bool); ok {
					r.Valor = map[bool]string{true: "Sí", false: "No"}[b]
				}
			case domain.CampoNumero:
				if n, ok := valor.(float64); ok {
					r.Valor = fmt.Sprintf("%d", int(n))
				}
			case domain.CampoFecha:
				if f, err := time.Parse("2006-01-02", texto(valor)); err == nil {
					r.Valor = f.Format("02/01/2006")
				}
			default:
				r.Valor = texto(valor)
			}
			legible.Respuestas = append(legible.Respuestas, r)
		}
		d.Secciones = append(d.Secciones, legible)
	}
	return d, nil
}

func etiquetaDe(etiquetas map[string]string, id string) string {
	if e, ok := etiquetas[id]; ok {
		return e
	}
	return "(opción borrada)"
}

// CambiarEstado aprueba o rechaza, guarda quién y cuándo, y avisa a la persona
// por push y por correo. El motivo es la nota del comité y NO va en el aviso:
// decidido el 2026-11-25, la persona solo sabe que no fue aprobada.
func (s *BoardBandejaService) CambiarEstado(ctx context.Context, id, estado, motivo, adminID string) (*domain.DetalleEnvioBoard, error) {
	if estado != domain.EstadoEnvioAprobada && estado != domain.EstadoEnvioRechazada {
		return nil, ErrEstadoBoardInvalido
	}
	motivo = strings.TrimSpace(motivo)
	if estado == domain.EstadoEnvioRechazada && motivo == "" {
		return nil, ErrEstadoBoardSinMotivo
	}
	if utf8.RuneCountInString(motivo) > maxMotivoBoard {
		return nil, ErrEstadoBoardMotivoLargo
	}
	actual, err := s.envios.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if actual.Estado == estado {
		return nil, ErrEstadoBoardIgual
	}
	if err := s.envios.CambiarEstado(ctx, id, estado, motivo, adminID); err != nil {
		return nil, err
	}
	detalle, err := s.Detalle(ctx, id)
	if err != nil {
		return nil, err
	}
	s.avisarEstado(detalle)
	return detalle, nil
}

// frase pasa «Postulación como consejero» a «tu postulación como consejero».
func frase(titulo string) string {
	if titulo == "" {
		return "tu envío"
	}
	r, n := utf8.DecodeRuneInString(titulo)
	return "tu " + strings.ToLower(string(r)) + titulo[n:]
}

func (s *BoardBandejaService) avisarEstado(d *domain.DetalleEnvioBoard) {
	cuerpo := fmt.Sprintf("El comité aprobó %s.", frase(d.Titulo))
	if d.Estado == domain.EstadoEnvioRechazada {
		cuerpo = fmt.Sprintf("El comité revisó %s y esta vez no fue aprobada.", frase(d.Titulo))
	}
	s.enSegundoPlano(func() {
		ctx := context.Background()
		if s.push != nil {
			data := map[string]string{"type": TipoAvisoBoard, "id": d.Formulario}
			if err := s.push.SendToUser(ctx, d.UserID, "Legacy Board", cuerpo, data); err != nil {
				log.Printf("board bandeja: push del envío %s falló: %v", d.ID, err)
			}
		}
		if s.correo != nil && d.Correo != "" {
			if err := s.correo.SendBoardEstadoEmail(d.Correo, d.Nombre, d.Titulo, d.Estado); err != nil {
				log.Printf("board bandeja: correo de estado del envío %s falló: %v", d.ID, err)
			}
		}
	})
}

// AvisarEnvioNuevo manda el correo a los destinatarios del panel (o al buzón de
// contacto si no hay ninguno) y marca el envío si salió a todos. Lo llama el
// servicio de formularios después de guardar: el envío ya está a salvo aunque
// el correo falle, y el panel destaca los que no avisaron.
func (s *BoardBandejaService) AvisarEnvioNuevo(e *domain.EnvioBoard, nombre, correo string) {
	form, ok := domain.FormulariosBoard[e.Formulario]
	if !ok || s.correo == nil {
		return
	}
	s.enSegundoPlano(func() {
		ctx := context.Background()
		destinos, err := s.envios.Destinatarios(ctx)
		if err != nil {
			log.Printf("board bandeja: no se pudieron leer los destinatarios: %v", err)
		}
		if len(destinos) == 0 && s.buzon != "" {
			destinos = []string{s.buzon}
		}
		todos := len(destinos) > 0
		for _, d := range destinos {
			if err := s.correo.SendBoardEnvioNuevoEmail(d, form.Titulo, nombre, correo); err != nil {
				todos = false
				log.Printf("board bandeja: aviso del envío %s a %s falló: %v", e.ID, d, err)
			}
		}
		if todos {
			if err := s.envios.MarcarAvisoEnviado(ctx, e.ID); err != nil {
				log.Printf("board bandeja: no se pudo marcar el aviso del envío %s: %v", e.ID, err)
			}
		}
	})
}

func (s *BoardBandejaService) Destinatarios(ctx context.Context) ([]string, error) {
	return s.envios.Destinatarios(ctx)
}

// GuardarDestinatarios deja exactamente la lista recibida: en minúsculas, sin
// espacios y sin repetidos.
func (s *BoardBandejaService) GuardarDestinatarios(ctx context.Context, correos []string) ([]string, error) {
	limpios := []string{}
	for _, c := range correos {
		c = strings.ToLower(strings.TrimSpace(c))
		if c == "" || slices.Contains(limpios, c) {
			continue
		}
		dir, err := mail.ParseAddress(c)
		if err != nil || dir.Address != c || !strings.Contains(c[strings.LastIndex(c, "@"):], ".") {
			return nil, fmt.Errorf("%w: %s", ErrDestinatarioBoard, c)
		}
		limpios = append(limpios, c)
	}
	if len(limpios) > maxDestinatariosBoard {
		return nil, ErrDestinatariosBoardMucho
	}
	if err := s.envios.ReemplazarDestinatarios(ctx, limpios); err != nil {
		return nil, err
	}
	return limpios, nil
}

// maxCandidatosBoard: cuántos candidatos se devuelven como mucho, los de más
// porcentaje. El comité lee la cabeza de la lista, no la cola.
const maxCandidatosBoard = 50

// Candidatos cruza una solicitud aprobada del B con todos los consejeros
// aprobados del A y los devuelve de mayor a menor porcentaje. Los que no
// coinciden en nada (0 %) no salen. Empate: el que se postuló antes, y luego el
// id, para que el orden sea siempre el mismo.
func (s *BoardBandejaService) Candidatos(ctx context.Context, solicitudID string) ([]*domain.CandidatoBoard, error) {
	sol, err := s.envios.FindByID(ctx, solicitudID)
	if err != nil {
		return nil, err
	}
	if sol.Formulario != "B" || sol.Estado != domain.EstadoEnvioAprobada {
		return nil, ErrMatchingNoDisponible
	}
	todas, err := s.opciones.List(ctx, "", false)
	if err != nil {
		return nil, err
	}
	opciones := make(map[string]*domain.OpcionBoard, len(todas))
	for _, o := range todas {
		opciones[o.ID] = o
	}
	consejeros, err := s.envios.Aprobados(ctx, "A")
	if err != nil {
		return nil, err
	}

	type fila struct {
		c     *domain.CandidatoBoard
		envio *domain.EnvioBoard
	}
	var filas []fila
	for _, e := range consejeros {
		// Quien pide consejo no se recomienda a sí mismo.
		if e.UserID == sol.UserID {
			continue
		}
		r := CruzarBoard(sol.Respuestas, e.Respuestas, opciones)
		if r.Porcentaje == 0 {
			continue
		}
		datos := s.datos(e)
		c := &domain.CandidatoBoard{
			EnvioID: e.ID, Nombre: texto(datos["nombre_completo"]), FotoURL: e.FotoURL,
			CargoActual: texto(e.Respuestas["cargo_actual"]), Porcentaje: r.Porcentaje, Razones: r.Razones,
		}
		if id := texto(e.Respuestas["honorarios"]); id != "" {
			c.Honorarios = etiquetaOpcion(opciones, id)
		}
		if c.Razones == nil {
			c.Razones = []string{}
		}
		filas = append(filas, fila{c, e})
	}
	slices.SortStableFunc(filas, func(a, b fila) int {
		if a.c.Porcentaje != b.c.Porcentaje {
			return b.c.Porcentaje - a.c.Porcentaje
		}
		if !a.envio.CreadoEn.Equal(b.envio.CreadoEn) {
			return a.envio.CreadoEn.Compare(b.envio.CreadoEn)
		}
		return strings.Compare(a.envio.ID, b.envio.ID)
	})
	candidatos := make([]*domain.CandidatoBoard, 0, min(len(filas), maxCandidatosBoard))
	for i, f := range filas {
		if i == maxCandidatosBoard {
			break
		}
		candidatos = append(candidatos, f.c)
	}
	return candidatos, nil
}

func etiquetaOpcion(opciones map[string]*domain.OpcionBoard, id string) string {
	if o, ok := opciones[id]; ok {
		return o.Etiqueta
	}
	return "(opción borrada)"
}
