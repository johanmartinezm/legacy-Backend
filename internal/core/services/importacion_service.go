package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
	"errors"
	"fmt"
	"strings"
)

// ImportacionService es el motor de la carga masiva de asistentes.
//
// Plan: reports/20260826_plan_carga_masiva.md §5, fase 1.
//
// Dos pasos separados a propósito:
//
//   - **Simular** no escribe nada. Informa de cuántas cuentas se crearían,
//     cuántas ya existen y en qué fila y columna está cada problema. Es lo que
//     se le enseña a quien preparó el archivo, que corrige **el archivo**.
//   - **Aplicar** solo entra si no hay ningún problema: un archivo con una fila
//     mala no se aplica a medias.
//
// Reejecutable: la identidad es el correo, y una fila cuya cuenta ya existe se
// cuenta y se deja como está. Volver a pasar el mismo archivo no duplica a
// nadie.
//
// **Nada de SQL directo.** Once campos se guardan cifrados y el correo lleva
// además el índice ciego del que depende el inicio de sesión: un INSERT a mano
// deja cuentas que la app no puede descifrar y que no pueden entrar.
type ImportacionService struct {
	auth ports.CuentasImportadas
	// eventos solo hace falta en la entrada del evento. Puede ser nil: entonces
	// una carga con evento se rechaza con ErrSinServicioDeEventos en vez de
	// crear las cuentas y olvidarse de inscribirlas, que es el fallo silencioso
	// que no queremos.
	eventos ports.InscripcionesImportadas
}

func NewImportacionService(auth ports.CuentasImportadas, eventos ports.InscripcionesImportadas) *ImportacionService {
	return &ImportacionService{auth: auth, eventos: eventos}
}

// ErrSinServicioDeEventos salta si se pide inscribir a un evento y el servicio
// se cableo sin el de eventos. Es un error de arranque, no de datos.
var ErrSinServicioDeEventos = errors.New("la importacion no puede inscribir a un evento: falta el servicio de eventos")

// RolDeCuentaImportada: las cuentas nacen `profesional`, fijado aquí y no
// heredado del DEFAULT de la tabla, que es `familia` (§2.3).
const RolDeCuentaImportada = "profesional"

// Simular recorre el archivo sin escribir nada.
func (s *ImportacionService) Simular(ctx context.Context, filas []domain.FilaImportacion, opciones domain.OpcionesImportacion) (*domain.ResultadoImportacion, error) {
	if opciones.ConEvento() && s.eventos == nil {
		return nil, ErrSinServicioDeEventos
	}

	resultado, _, err := s.analizar(ctx, filas)
	if err != nil {
		return nil, err
	}
	resultado.Simulacion = true

	// Con evento, **todas** las filas buenas quedan inscritas: las que crean
	// cuenta y las que ya la tenian. Es lo que hace util a esta entrada, y lo
	// que hay que decir en el informe para que no parezca que solo se crean
	// cuentas.
	//
	// Cuantas ya estaban inscritas no se sabe sin consultar el evento fila por
	// fila; RegisterUser lo resuelve solo -vuelve sobre la existente y no
	// duplica-, asi que la simulacion da el techo y la carga da el reparto.
	if opciones.ConEvento() {
		resultado.PorInscribir = resultado.Nuevas + resultado.YaExistian
	}

	return resultado, nil
}

// Aplicar valida primero y, solo si no hay ningún problema, crea las cuentas
// que faltan.
//
// No es transaccional: cada cuenta se crea por separado. Por eso la validación
// va antes y es completa, así lo que puede fallar a mitad de camino es un error
// de la base y no una fila mal escrita. Si aun así falla una, se devuelve el
// problema con su número de fila y las anteriores quedan creadas; volver a
// pasar el mismo archivo las salta por existir.
func (s *ImportacionService) Aplicar(ctx context.Context, filas []domain.FilaImportacion, opciones domain.OpcionesImportacion) (*domain.ResultadoImportacion, error) {
	if opciones.ConEvento() && s.eventos == nil {
		return nil, ErrSinServicioDeEventos
	}

	resultado, preparadas, err := s.analizar(ctx, filas)
	if err != nil {
		return nil, err
	}
	if resultado.TieneProblemas() {
		// Se devuelve el mismo informe que la simulación: nada se escribió.
		resultado.Simulacion = true
		return resultado, nil
	}

	for _, p := range preparadas {
		usuarioID := p.usuarioID

		if usuarioID == "" {
			usuario := p.usuario
			if err := s.auth.RegistrarImportado(ctx, &usuario, p.contrasena); err != nil {
				resultado.Problemas = append(resultado.Problemas, domain.ProblemaDeFila{
					Fila:    p.fila,
					Columna: "E-mail",
					Motivo:  fmt.Sprintf("no se pudo crear la cuenta: %v", err),
				})
				return resultado, nil
			}
			usuarioID = usuario.ID
			resultado.Creadas++
		}

		if !opciones.ConEvento() {
			continue
		}

		// La inscripción va después de la cuenta y con el id que acaba de salir
		// de crearla. Si el alta falló, arriba ya se devolvió: nunca se intenta
		// inscribir a alguien que no existe.
		if err := s.inscribir(ctx, usuarioID, p, opciones, resultado); err != nil {
			return resultado, nil
		}
	}

	return resultado, nil
}

// inscribir deja la fila inscrita al evento de las opciones.
//
// Reutiliza EventService.RegisterUser, que es lo que ya sabe resolver el estado
// de pago, el estado de la inscripción, el código de acceso y el correo. Se
// llama al servicio directamente y no dando la vuelta por HTTP: la ruta pública
// exige un token de admin y compone la mitad de esto a partir del contexto de
// la petición.
//
// **No duplica.** RegisterUser vuelve sobre la inscripción existente y la
// devuelve tal cual, así que volver a pasar el mismo archivo cuenta esa fila
// como «ya inscrita» y no crea una segunda.
func (s *ImportacionService) inscribir(
	ctx context.Context,
	usuarioID string,
	p filaPreparada,
	opciones domain.OpcionesImportacion,
	resultado *domain.ResultadoImportacion,
) error {
	reg := &domain.Registration{
		UserID:  usuarioID,
		EventID: opciones.EventoID,
		Importacion: domain.AltaImportada{
			EsCarga:       true,
			SinCredencial: !opciones.GenerarCredencial,
			Aviso:         opciones.AvisoDeLaCarga(),
		},
	}

	// Las credenciales de acceso solo se escriben en el correo de una cuenta
	// **recién creada**: su contraseña es su número de documento y no hay otro
	// sitio donde se entere. A quien ya tenía cuenta no se le puede decir eso,
	// porque su contraseña es la suya.
	if p.usuarioID == "" {
		reg.Importacion.Usuario = p.usuario.Email
		reg.Importacion.Contrasena = p.contrasena
	}

	if err := s.eventos.RegisterUser(ctx, reg); err != nil {
		resultado.Problemas = append(resultado.Problemas, domain.ProblemaDeFila{
			Fila:    p.fila,
			Columna: "E-mail",
			Motivo:  fmt.Sprintf("la cuenta quedó creada pero no se pudo inscribir al evento: %v", err),
		})
		return err
	}

	if reg.YaEstabaInscrito {
		resultado.YaInscritas++
	} else {
		resultado.Inscritas++
	}
	return nil
}

// filaPreparada es una fila ya validada y convertida en usuario, lista para
// crearse. Se guarda para no repetir el trabajo entre validar y aplicar.
type filaPreparada struct {
	fila       int
	usuario    domain.User
	contrasena string
	// usuarioID es el id de la cuenta que ya existía, o cadena vacía si hay que
	// crearla. Guarda el id y no un «ya existe» porque la entrada del evento
	// necesita a quién inscribir, y volver a preguntarlo sería una consulta más
	// por fila.
	usuarioID string
}

func (s *ImportacionService) analizar(ctx context.Context, filas []domain.FilaImportacion) (*domain.ResultadoImportacion, []filaPreparada, error) {
	resultado := &domain.ResultadoImportacion{
		Total:     len(filas),
		Problemas: []domain.ProblemaDeFila{},
	}
	preparadas := make([]filaPreparada, 0, len(filas))

	// Un correo repetido dentro del propio archivo se avisa antes de aplicar:
	// si no, la primera fila crearía la cuenta y la segunda fallaría contra el
	// índice único a mitad de la carga.
	vistos := map[string]int{}

	for _, fila := range filas {
		problemas, usuario, contrasena := s.validarFila(fila)

		correo := normalizarCorreo(fila.Email)
		if correo != "" {
			if anterior, repetido := vistos[correo]; repetido {
				problemas = append(problemas, domain.ProblemaDeFila{
					Fila:    fila.Fila,
					Columna: "E-mail",
					Motivo:  fmt.Sprintf("repetido: ya aparece en la fila %d", anterior),
				})
			} else {
				vistos[correo] = fila.Fila
			}
		}

		if len(problemas) > 0 {
			resultado.Problemas = append(resultado.Problemas, problemas...)
			continue
		}

		usuarioID, err := s.auth.IDDeCuentaConCorreo(ctx, usuario.Email)
		if err != nil {
			return nil, nil, err
		}
		if usuarioID != "" {
			resultado.YaExistian++
		} else {
			resultado.Nuevas++
		}

		preparadas = append(preparadas, filaPreparada{
			fila:       fila.Fila,
			usuario:    usuario,
			contrasena: contrasena,
			usuarioID:  usuarioID,
		})
	}

	return resultado, preparadas, nil
}

// validarFila aplica las reglas del plan y devuelve el usuario ya armado.
//
// Las reglas, todas de §2.2 y §2.3:
//   - sin correo, la fila no se importa: sin él no hay identidad ni acceso;
//   - la contraseña es el número de documento, que tiene que pasar el mínimo
//     de seis caracteres. Una cédula lo pasa; un pasaporte corto, no;
//   - el tipo de documento se traduce al catálogo de Legacy;
//   - el rol es siempre `profesional`.
func (s *ImportacionService) validarFila(fila domain.FilaImportacion) ([]domain.ProblemaDeFila, domain.User, string) {
	var problemas []domain.ProblemaDeFila
	problema := func(columna, motivo string) {
		problemas = append(problemas, domain.ProblemaDeFila{
			Fila:    fila.Fila,
			Columna: columna,
			Motivo:  motivo,
		})
	}

	correo := normalizarCorreo(fila.Email)
	if correo == "" {
		problema("E-mail", "la fila no trae correo, así que no se puede crear la cuenta")
	} else if !strings.Contains(correo, "@") || strings.HasSuffix(correo, "@") || strings.HasPrefix(correo, "@") {
		problema("E-mail", "no parece un correo válido")
	}

	nombres := strings.TrimSpace(fila.Nombres)
	if nombres == "" {
		problema("Nombres", "está vacío")
	}

	documento := strings.TrimSpace(fila.NumeroDocumento)
	if documento == "" {
		problema("CC/TI/CE", "está vacío, y de ahí sale la contraseña de la persona")
	} else if err := domain.ValidarContrasena(documento); err != nil {
		problema("CC/TI/CE", fmt.Sprintf(
			"tiene %d caracteres y la contraseña necesita al menos %d",
			len([]rune(documento)), domain.LongitudMinimaContrasena))
	}

	tipoDocumento, reconocido := domain.TraducirTipoDeDocumento(fila.TipoDocumento)
	if !reconocido {
		problema("Tipo", fmt.Sprintf("%q no está en el catálogo de Legacy",
			strings.TrimSpace(fila.TipoDocumento)))
	}

	usuario := domain.User{
		Email:                correo,
		FirstName:            nombres,
		LastName:             strings.TrimSpace(fila.Apellidos),
		Phone:                strings.TrimSpace(fila.Telefono),
		CompanyName:          strings.TrimSpace(fila.Empresa),
		JobTitle:             strings.TrimSpace(fila.Cargo),
		IdentificationType:   tipoDocumento,
		IdentificationNumber: documento,
		Country:              strings.TrimSpace(fila.Pais),
		Location:             strings.TrimSpace(fila.Ciudad),
		Departamento:         strings.TrimSpace(fila.Departamento),
		Direccion:            strings.TrimSpace(fila.Direccion),
		Sexo:                 strings.TrimSpace(fila.Sexo),
		Role:                 RolDeCuentaImportada,
		TermsAccepted:        fila.AceptaTerminos,
		// El archivo trae una sola casilla, la de los términos del evento.
		// `data_sharing_accepted` se da por aceptado en una carga masiva, con su
		// versión y su fecha selladas como en el registro normal (§3.2).
		DataSharingAccepted: true,
	}

	if fecha := strings.TrimSpace(fila.FechaNacimiento); fecha != "" {
		if t, err := domain.ParsearFechaDeNacimiento(fecha); err == nil {
			usuario.BirthDate = &t
		} else {
			problema("Fecha De Nacimiento", fmt.Sprintf(
				"%q no es una fecha reconocible (AAAA-MM-DD o DD/MM/AAAA)", fecha))
		}
	}

	return problemas, usuario, documento
}

// normalizarCorreo deja el correo como lo espera el índice ciego: sin espacios
// y en minúsculas. Sin esto, "Ana@X.com" y "ana@x.com" serían dos cuentas
// distintas para el archivo y la misma para la base.
func normalizarCorreo(correo string) string {
	return strings.ToLower(strings.TrimSpace(correo))
}
