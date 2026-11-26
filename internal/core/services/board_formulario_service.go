package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/mail"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrFormularioBoardNoExiste = errors.New("el formulario no existe")
	ErrFormularioBoardEnviado  = errors.New("ya enviaste este formulario; el comité lo está revisando o ya respondió")
	ErrFormularioBoardTerminos = errors.New("debes aceptar los términos y el tratamiento de datos para enviar")
)

// ErrCampoBoard es un error de validación de un campo concreto. El handler lo
// devuelve con la clave del campo para que la app lo señale.
type ErrCampoBoard struct {
	Campo   string
	Mensaje string
}

func (e *ErrCampoBoard) Error() string { return e.Mensaje }

func errCampo(c domain.CampoBoard, formato string, args ...any) error {
	return &ErrCampoBoard{Campo: c.Clave, Mensaje: c.Etiqueta + ": " + fmt.Sprintf(formato, args...)}
}

// EsErrorDeValidacionFormularioBoard dice si el error es del formulario (400).
func EsErrorDeValidacionFormularioBoard(err error) bool {
	var ec *ErrCampoBoard
	return errors.As(err, &ec) || errors.Is(err, ErrFormularioBoardTerminos)
}

// cifradorBoard es lo que el servicio usa de security.CryptoService.
type cifradorBoard interface {
	Encrypt(plaintext string) (string, error)
}

const maxTextoBoardPorDefecto = 200

var (
	reFotoBoard     = regexp.MustCompile(`^/api/images/[0-9a-f-]{36}_[A-Za-z0-9._-]+$`)
	reTelefonoBoard = regexp.MustCompile(`^\+?[0-9 ().-]+$`)
)

type BoardFormularioService struct {
	opciones ports.BoardOpcionRepository
	envios   ports.BoardEnvioRepository
	cifrador cifradorBoard
	// fotoExiste comprueba que la foto se subió de verdad a este servidor
	// (recibe el nombre del archivo, sin /api/images/).
	fotoExiste func(nombre string) bool
	ahora      func() time.Time
	// avisoNuevo avisa al comité de un envío ya guardado (BoardBandejaService).
	// Puede ser nil: entonces no avisa a nadie.
	avisoNuevo func(e *domain.EnvioBoard, nombre, correo string)
}

// ConAvisoNuevo conecta el aviso al comité. Va aparte del constructor porque
// la bandeja se crea después y depende de los mismos repositorios.
func (s *BoardFormularioService) ConAvisoNuevo(f func(e *domain.EnvioBoard, nombre, correo string)) *BoardFormularioService {
	s.avisoNuevo = f
	return s
}

func NewBoardFormularioService(opciones ports.BoardOpcionRepository, envios ports.BoardEnvioRepository,
	cifrador cifradorBoard, fotoExiste func(string) bool) *BoardFormularioService {
	return &BoardFormularioService{opciones: opciones, envios: envios, cifrador: cifrador, fotoExiste: fotoExiste, ahora: time.Now}
}

// Formulario devuelve una copia de la configuración con las opciones activas
// de cada lista y el envío actual del usuario, si lo hay.
func (s *BoardFormularioService) Formulario(ctx context.Context, userID, clave string) (*domain.FormularioBoard, error) {
	base, ok := domain.FormulariosBoard[clave]
	if !ok {
		return nil, ErrFormularioBoardNoExiste
	}
	activas, err := s.opciones.List(ctx, "", true)
	if err != nil {
		return nil, err
	}
	porCatalogo := map[string][]*domain.OpcionBoard{}
	for _, o := range activas {
		porCatalogo[o.Catalogo] = append(porCatalogo[o.Catalogo], o)
	}

	f := *base
	f.Secciones = make([]domain.SeccionBoard, len(base.Secciones))
	for i, sec := range base.Secciones {
		campos := make([]domain.CampoBoard, len(sec.Campos))
		for j, c := range sec.Campos {
			if c.Catalogo != "" {
				c.Opciones = []domain.OpcionFormulario{}
				for _, o := range porCatalogo[c.Catalogo] {
					if !slices.Contains(c.Excluir, o.Codigo) {
						c.Opciones = append(c.Opciones, domain.OpcionFormulario{ID: o.ID, Codigo: o.Codigo, Etiqueta: o.Etiqueta})
					}
				}
			}
			campos[j] = c
		}
		f.Secciones[i] = domain.SeccionBoard{Titulo: sec.Titulo, Campos: campos}
	}

	// Se muestra el último en cualquier estado: del B, el que ya tiene
	// decisión también, para que la persona se entere de ella (si solo se
	// mostrara el pendiente, tras la decisión vería el formulario vacío).
	actual, err := s.envios.Ultimo(ctx, userID, clave)
	switch {
	case err == nil:
		f.EnvioActual = actual
	case !errors.Is(err, domain.ErrNotFound):
		return nil, err
	}
	return &f, nil
}

// Enviar valida las respuestas contra la configuración y las opciones activas,
// cifra los datos personales y guarda el envío en estado pendiente.
func (s *BoardFormularioService) Enviar(ctx context.Context, userID, clave string, in ports.EntradaEnvioBoard) (*domain.EnvioBoard, error) {
	f, ok := domain.FormulariosBoard[clave]
	if !ok {
		return nil, ErrFormularioBoardNoExiste
	}
	// Antes de validar: a quien ya lo envió no le sirve saber que un campo está mal.
	if _, err := s.envios.Actual(ctx, userID, clave); err == nil {
		return nil, ErrFormularioBoardEnviado
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	activas, err := s.opciones.List(ctx, "", true)
	if err != nil {
		return nil, err
	}
	opciones := make(map[string]*domain.OpcionBoard, len(activas))
	for _, o := range activas {
		opciones[o.ID] = o
	}

	envio, cifrados, err := s.validar(f, in, opciones)
	if err != nil {
		return nil, err
	}
	envio.UserID = userID
	envio.Formulario = clave
	if err := s.envios.Create(ctx, envio); err != nil {
		// Dos envíos a la vez del mismo usuario: el segundo choca con el índice único.
		if errors.Is(err, domain.ErrUniqueViolation) {
			return nil, ErrFormularioBoardEnviado
		}
		return nil, err
	}
	if s.avisoNuevo != nil {
		nombre, _ := cifrados["nombre_completo"].(string)
		correo, _ := cifrados["correo"].(string)
		s.avisoNuevo(envio, nombre, correo)
	}
	return envio, nil
}

// validar devuelve el envío listo para guardar y, aparte, los datos en claro
// que acaba de cifrar (el aviso al comité necesita el nombre y el correo).
func (s *BoardFormularioService) validar(f *domain.FormularioBoard, in ports.EntradaEnvioBoard,
	opciones map[string]*domain.OpcionBoard) (*domain.EnvioBoard, map[string]any, error) {
	conocidos := map[string]bool{}
	respuestas := map[string]any{}
	cifrados := map[string]any{}
	envio := &domain.EnvioBoard{AceptaTerminos: in.AceptaTerminos}

	for _, sec := range f.Secciones {
		for _, c := range sec.Campos {
			conocidos[c.Clave] = true
			valor, presente, err := s.validarCampo(c, in.Respuestas[c.Clave], opciones)
			if err != nil {
				return nil, nil, err
			}
			if !presente {
				if c.Obligatorio {
					return nil, nil, errCampo(c, "es obligatorio")
				}
				continue
			}
			switch {
			case c.Tipo == domain.CampoFoto:
				envio.FotoURL = valor.(string)
			case c.Cifrado:
				cifrados[c.Clave] = valor
			default:
				respuestas[c.Clave] = valor
			}
		}
	}
	// Una clave que el formulario no tiene es un cliente desalineado: mejor
	// decirlo que guardar algo que nadie va a leer.
	for clave := range in.Respuestas {
		if !conocidos[clave] {
			return nil, nil, &ErrCampoBoard{Campo: clave, Mensaje: "el formulario no tiene el campo «" + clave + "»"}
		}
	}
	if !in.AceptaTerminos {
		return nil, nil, ErrFormularioBoardTerminos
	}

	plano, err := json.Marshal(cifrados)
	if err != nil {
		return nil, nil, err
	}
	envio.DatosCifrados, err = s.cifrador.Encrypt(string(plano))
	if err != nil {
		return nil, nil, fmt.Errorf("cifrando el envío: %w", err)
	}
	envio.Respuestas = respuestas
	return envio, cifrados, nil
}

// validarCampo devuelve el valor ya normalizado y si vino. Un texto vacío, un
// null o una lista vacía cuentan como «no vino».
func (s *BoardFormularioService) validarCampo(c domain.CampoBoard, crudo json.RawMessage,
	opciones map[string]*domain.OpcionBoard) (any, bool, error) {
	if len(crudo) == 0 || string(crudo) == "null" {
		return nil, false, nil
	}

	switch c.Tipo {
	case domain.CampoNumero:
		var n float64
		if json.Unmarshal(crudo, &n) != nil || n != math.Trunc(n) {
			return nil, false, errCampo(c, "debe ser un número entero")
		}
		v := int(n)
		if (c.Min != nil && v < *c.Min) || (c.Max != nil && v > *c.Max) {
			return nil, false, errCampo(c, "debe estar entre %d y %d", *c.Min, *c.Max)
		}
		return v, true, nil

	case domain.CampoSiNo:
		var b bool
		if json.Unmarshal(crudo, &b) != nil {
			return nil, false, errCampo(c, "debe ser sí o no")
		}
		return b, true, nil

	case domain.CampoMultiseleccion:
		var ids []string
		if json.Unmarshal(crudo, &ids) != nil {
			return nil, false, errCampo(c, "debe ser una lista de opciones")
		}
		if len(ids) == 0 {
			return nil, false, nil
		}
		vistos := map[string]bool{}
		exclusiva := false
		for _, id := range ids {
			o, err := opcionDelCampo(c, id, opciones)
			if err != nil {
				return nil, false, err
			}
			if vistos[id] {
				return nil, false, errCampo(c, "hay una opción repetida")
			}
			vistos[id] = true
			if c.Exclusiva != "" && o.Codigo == c.Exclusiva {
				exclusiva = true
			}
		}
		if exclusiva && len(ids) > 1 {
			return nil, false, errCampo(c, "«%s» no se puede marcar junto con otras opciones", opciones[idDeCodigo(ids, opciones, c.Exclusiva)].Etiqueta)
		}
		return ids, true, nil
	}

	// El resto de tipos son textos.
	var texto string
	if json.Unmarshal(crudo, &texto) != nil {
		return nil, false, errCampo(c, "debe ser un texto")
	}
	if c.Tipo == domain.CampoTextoLargo {
		// En los largos se conservan los saltos de línea; solo se recortan los extremos.
		texto = strings.TrimSpace(texto)
	} else {
		texto = strings.Join(strings.Fields(texto), " ")
	}
	if texto == "" {
		return nil, false, nil
	}
	if strings.ContainsRune(texto, utf8.RuneError) {
		return nil, false, errCampo(c, "trae caracteres inválidos")
	}
	max := c.MaxLargo
	if max == 0 {
		max = maxTextoBoardPorDefecto
	}
	if utf8.RuneCountInString(texto) > max {
		return nil, false, errCampo(c, "admite hasta %d caracteres", max)
	}

	switch c.Tipo {
	case domain.CampoTexto, domain.CampoTextoLargo:
		if c.Patron != "" && !regexp.MustCompile(c.Patron).MatchString(texto) {
			return nil, false, errCampo(c, "tiene caracteres no permitidos")
		}
	case domain.CampoEmail:
		dir, err := mail.ParseAddress(texto)
		if err != nil || dir.Address != texto || utf8.RuneCountInString(texto) > 254 {
			return nil, false, errCampo(c, "no es un correo válido")
		}
		texto = strings.ToLower(texto)
	case domain.CampoTelefono:
		digitos := 0
		for _, r := range texto {
			if r >= '0' && r <= '9' {
				digitos++
			}
		}
		if !reTelefonoBoard.MatchString(texto) || digitos < 7 || digitos > 15 {
			return nil, false, errCampo(c, "no es un teléfono válido")
		}
	case domain.CampoFecha:
		fecha, err := time.Parse("2006-01-02", texto)
		if err != nil || fecha.Year() < 1900 || fecha.After(s.ahora()) {
			return nil, false, errCampo(c, "no es una fecha válida")
		}
	case domain.CampoURL:
		u, err := url.Parse(texto)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return nil, false, errCampo(c, "debe empezar por https://")
		}
		host := strings.ToLower(u.Hostname())
		if c.Dominio != "" && host != c.Dominio && !strings.HasSuffix(host, "."+c.Dominio) {
			return nil, false, errCampo(c, "debe ser un enlace de %s", c.Dominio)
		}
	case domain.CampoSeleccion:
		if _, err := opcionDelCampo(c, texto, opciones); err != nil {
			return nil, false, err
		}
	case domain.CampoFoto:
		if !reFotoBoard.MatchString(texto) || !s.fotoExiste(strings.TrimPrefix(texto, "/api/images/")) {
			return nil, false, errCampo(c, "vuelve a subir la foto")
		}
	default:
		return nil, false, fmt.Errorf("tipo de campo desconocido %q en %s", c.Tipo, c.Clave)
	}
	return texto, true, nil
}

// opcionDelCampo comprueba que el id sea una opción activa de la lista del
// campo y que el campo la ofrezca.
func opcionDelCampo(c domain.CampoBoard, id string, opciones map[string]*domain.OpcionBoard) (*domain.OpcionBoard, error) {
	o, ok := opciones[id]
	if !ok || o.Catalogo != c.Catalogo || slices.Contains(c.Excluir, o.Codigo) {
		return nil, errCampo(c, "una de las opciones ya no está disponible; vuelve a elegir")
	}
	return o, nil
}

func idDeCodigo(ids []string, opciones map[string]*domain.OpcionBoard, codigo string) string {
	for _, id := range ids {
		if opciones[id].Codigo == codigo {
			return id
		}
	}
	return ""
}
