package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"
	"strings"
	"testing"
)

// authFalso implementa solo lo que el importador usa de ports.AuthService.
// No se embebe la interfaz entera a propósito: si mañana el importador empieza
// a llamar otro método, esto deja de compilar y hay que decidirlo, en vez de
// reventar en producción con un nil.
type authFalso struct {
	existentes map[string]bool
	creados    []domain.User
	contrasena map[string]string
	errCrear   error
}

func nuevoAuthFalso(existentes ...string) *authFalso {
	m := map[string]bool{}
	for _, c := range existentes {
		m[c] = true
	}
	return &authFalso{existentes: m, contrasena: map[string]string{}}
}

func (a *authFalso) ExisteCuentaConCorreo(ctx context.Context, email string) (bool, error) {
	return a.existentes[email], nil
}

func (a *authFalso) RegistrarImportado(ctx context.Context, user *domain.User, password string) error {
	if a.errCrear != nil {
		return a.errCrear
	}
	a.creados = append(a.creados, *user)
	a.contrasena[user.Email] = password
	a.existentes[user.Email] = true
	return nil
}

// filaBuena es una fila completa y correcta; cada prueba estropea lo suyo.
func filaBuena(n int, correo string) domain.FilaImportacion {
	return domain.FilaImportacion{
		Fila:            n,
		Nombres:         "Ana María",
		Apellidos:       "Restrepo Uribe",
		Email:           correo,
		Telefono:        "3001234567",
		Empresa:         "Agroandina S.A.S.",
		Cargo:           "Gerente",
		TipoDocumento:   "CC/TI/CE",
		NumeroDocumento: "1020304050",
		Pais:            "Colombia",
		Ciudad:          "Medellín",
		Departamento:    "Antioquia",
		Direccion:       "Calle 10 # 43-21",
		Sexo:            "Femenino",
		FechaNacimiento: "15/01/1990",
		AceptaTerminos:  true,
	}
}

func importadorCon(auth *authFalso) *ImportacionService {
	return NewImportacionService(auth)
}

func TestSimular_NoEscribeNada(t *testing.T) {
	auth := nuevoAuthFalso()
	s := importadorCon(auth)

	res, err := s.Simular(context.Background(), []domain.FilaImportacion{filaBuena(2, "ana@empresa.com")})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if !res.Simulacion || res.Total != 1 || res.Nuevas != 1 || res.Creadas != 0 {
		t.Errorf("informe inesperado: %+v", res)
	}
	if len(auth.creados) != 0 {
		t.Error("la simulación creó cuentas")
	}
}

func TestAplicar_CreaLaCuentaConLoQueDiceElPlan(t *testing.T) {
	auth := nuevoAuthFalso()
	s := importadorCon(auth)

	res, err := s.Aplicar(context.Background(), []domain.FilaImportacion{filaBuena(2, "  Ana@Empresa.COM ")})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if res.Creadas != 1 || res.TieneProblemas() {
		t.Fatalf("no se creó la cuenta: %+v", res)
	}

	creado := auth.creados[0]
	// El correo se normaliza: si no, "Ana@X.com" y "ana@x.com" serían dos
	// cuentas para el archivo y una sola para el índice ciego.
	if creado.Email != "ana@empresa.com" {
		t.Errorf("correo sin normalizar: %q", creado.Email)
	}
	// El rol se fija; el DEFAULT de la tabla es 'familia' y dejarlo al azar
	// metería a todo el mundo mal.
	if creado.Role != RolDeCuentaImportada {
		t.Errorf("rol esperado %q, llegó %q", RolDeCuentaImportada, creado.Role)
	}
	// El tipo se traduce al catálogo de Legacy: "CC/TI/CE" no existe en él.
	if creado.IdentificationType != "Cédula" {
		t.Errorf("tipo de documento sin traducir: %q", creado.IdentificationType)
	}
	if !creado.TermsAccepted || !creado.DataSharingAccepted {
		t.Errorf("consentimientos mal: terms=%v data=%v", creado.TermsAccepted, creado.DataSharingAccepted)
	}
	if creado.BirthDate == nil || creado.BirthDate.Year() != 1990 {
		t.Errorf("fecha de nacimiento mal: %v", creado.BirthDate)
	}
	if creado.Departamento != "Antioquia" || creado.Direccion == "" || creado.Sexo != "Femenino" {
		t.Errorf("campos nuevos mal: %+v", creado)
	}
	// La contraseña es el número de documento (§2.2).
	if auth.contrasena["ana@empresa.com"] != "1020304050" {
		t.Errorf("contraseña esperada el documento, llegó %q", auth.contrasena["ana@empresa.com"])
	}
}

func TestAplicar_UnaCuentaQueYaExisteNoSeDuplica(t *testing.T) {
	auth := nuevoAuthFalso("ana@empresa.com")
	s := importadorCon(auth)

	res, err := s.Aplicar(context.Background(), []domain.FilaImportacion{
		filaBuena(2, "ana@empresa.com"),
		filaBuena(3, "otra@empresa.com"),
	})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if res.YaExistian != 1 || res.Nuevas != 1 || res.Creadas != 1 {
		t.Errorf("informe inesperado: %+v", res)
	}
	if len(auth.creados) != 1 || auth.creados[0].Email != "otra@empresa.com" {
		t.Errorf("se creó lo que no tocaba: %+v", auth.creados)
	}
}

func TestValidacion_ProblemasPorFilaYColumna(t *testing.T) {
	casos := []struct {
		nombre  string
		ajuste  func(*domain.FilaImportacion)
		columna string
		motivo  string
	}{
		{
			nombre:  "sin correo no hay identidad ni acceso",
			ajuste:  func(f *domain.FilaImportacion) { f.Email = "   " },
			columna: "E-mail",
			motivo:  "no trae correo",
		},
		{
			nombre:  "un correo sin arroba",
			ajuste:  func(f *domain.FilaImportacion) { f.Email = "ana.empresa.com" },
			columna: "E-mail",
			motivo:  "no parece un correo",
		},
		{
			nombre:  "sin documento no hay contraseña",
			ajuste:  func(f *domain.FilaImportacion) { f.NumeroDocumento = "" },
			columna: "CC/TI/CE",
			motivo:  "contraseña",
		},
		{
			// La trampa que anota el plan: una cédula pasa el mínimo de seis,
			// un pasaporte corto no.
			nombre:  "un documento demasiado corto para ser contraseña",
			ajuste:  func(f *domain.FilaImportacion) { f.NumeroDocumento = "A123" },
			columna: "CC/TI/CE",
			motivo:  "al menos 6",
		},
		{
			nombre:  "un tipo de documento fuera del catálogo",
			ajuste:  func(f *domain.FilaImportacion) { f.TipoDocumento = "Licencia de conducción" },
			columna: "Tipo",
			motivo:  "catálogo",
		},
		{
			nombre:  "una fecha ilegible",
			ajuste:  func(f *domain.FilaImportacion) { f.FechaNacimiento = "el año pasado" },
			columna: "Fecha De Nacimiento",
			motivo:  "no es una fecha",
		},
		{
			nombre:  "sin nombre",
			ajuste:  func(f *domain.FilaImportacion) { f.Nombres = "" },
			columna: "Nombres",
			motivo:  "vacío",
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			fila := filaBuena(34, "ana@empresa.com")
			c.ajuste(&fila)

			auth := nuevoAuthFalso()
			res, err := importadorCon(auth).Aplicar(context.Background(), []domain.FilaImportacion{fila})
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}

			if !res.TieneProblemas() {
				t.Fatalf("se esperaba un problema y no lo hubo: %+v", res)
			}
			p := res.Problemas[0]
			// El número de fila es lo que hace accionable el informe: quien
			// preparó el archivo va a esa fila y corrige.
			if p.Fila != 34 {
				t.Errorf("fila esperada 34, llegó %d", p.Fila)
			}
			if p.Columna != c.columna {
				t.Errorf("columna esperada %q, llegó %q", c.columna, p.Columna)
			}
			if !strings.Contains(p.Motivo, c.motivo) {
				t.Errorf("motivo %q no menciona %q", p.Motivo, c.motivo)
			}
			if len(auth.creados) != 0 {
				t.Error("se creó una cuenta pese al problema")
			}
		})
	}
}

func TestValidacion_CorreoRepetidoDentroDelArchivo(t *testing.T) {
	// Sin esta comprobación, la primera fila crearía la cuenta y la segunda
	// fallaría contra el índice único a mitad de la carga.
	auth := nuevoAuthFalso()
	res, err := importadorCon(auth).Simular(context.Background(), []domain.FilaImportacion{
		filaBuena(2, "ana@empresa.com"),
		filaBuena(7, "ANA@empresa.com"),
	})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if len(res.Problemas) != 1 {
		t.Fatalf("se esperaba un problema, hubo %d: %+v", len(res.Problemas), res.Problemas)
	}
	p := res.Problemas[0]
	if p.Fila != 7 || !strings.Contains(p.Motivo, "fila 2") {
		t.Errorf("el aviso no señala la fila anterior: %+v", p)
	}
}

func TestAplicar_UnaFilaMalaNoAplicaNingunaOtra(t *testing.T) {
	// "Un archivo con una fila mala no se aplica a medias" (§5, fase 1).
	auth := nuevoAuthFalso()
	filaMala := filaBuena(3, "sin-arroba")
	res, err := importadorCon(auth).Aplicar(context.Background(), []domain.FilaImportacion{
		filaBuena(2, "ana@empresa.com"),
		filaMala,
		filaBuena(4, "otra@empresa.com"),
	})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if len(auth.creados) != 0 {
		t.Errorf("se crearon %d cuentas pese al problema", len(auth.creados))
	}
	if res.Creadas != 0 || !res.Simulacion {
		t.Errorf("el informe debería decir que no se aplicó nada: %+v", res)
	}
}

func TestAplicar_SiLaBaseFallaLoDiceConSuFila(t *testing.T) {
	auth := nuevoAuthFalso()
	auth.errCrear = errors.New("connection refused")

	res, err := importadorCon(auth).Aplicar(context.Background(), []domain.FilaImportacion{filaBuena(9, "ana@empresa.com")})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if !res.TieneProblemas() || res.Problemas[0].Fila != 9 {
		t.Errorf("el fallo no se reportó con su fila: %+v", res.Problemas)
	}
}

func TestTraducirTipoDeDocumento(t *testing.T) {
	casos := map[string]string{
		"CC/TI/CE":                   "Cédula",
		"cc/ti/ce":                   "Cédula",
		"Pasaporte u otro documento": "Pasaporte",
		// El archivo trae puntuación pegada en algunas columnas.
		"Pasaporte,": "Pasaporte",
		"  NIT  ":    "NIT",
		"":           "",
	}
	for entrada, esperado := range casos {
		valor, ok := domain.TraducirTipoDeDocumento(entrada)
		if !ok {
			t.Errorf("%q se dio por no reconocido", entrada)
		}
		if valor != esperado {
			t.Errorf("%q → %q, se esperaba %q", entrada, valor, esperado)
		}
	}

	if _, ok := domain.TraducirTipoDeDocumento("Licencia"); ok {
		t.Error("un tipo inventado se dio por bueno")
	}
}
