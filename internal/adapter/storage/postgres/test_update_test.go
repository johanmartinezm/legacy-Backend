package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"applegacy/backend/internal/core/domain"

	"github.com/jackc/pgx/v4/pgxpool"
)

// Prueba de integración del repositorio de usuarios: comprueba que `Update`
// escribe **todos** los campos del perfil y que `FindByID` los devuelve.
//
// Necesita una base de datos de verdad, y por eso es la única del repositorio
// que no corre sola. Hasta el 2026-09-03 arrastraba tres problemas, y los tres
// tenían la misma raíz: se escribió para ejecutarse a mano una vez y se quedó.
//
//  1. **La cadena de conexión iba escrita a mano y con el usuario equivocado**
//     (`postgres:postgres`, cuando el contenedor de desarrollo usa `dba`). Como
//     además fallaba con `t.Fatalf`, dejaba `go test ./...` **en rojo
//     permanente** en cualquier máquina, y un rojo de fondo esconde el siguiente.
//     Ahora sale de una variable de entorno, con el valor del entorno local por
//     defecto, y **se salta** si no hay base a la que conectarse.
//
//  2. 🔴 **Machacaba a un usuario real.** Cogía el primero de `FindAll` y le
//     sobreescribía nombre, teléfono, país y documento con basura de prueba, sin
//     restaurarlo nunca. Es decir: "arreglar" solo la cadena de conexión lo
//     habría puesto en verde **y habría empezado a destrozar una cuenta en cada
//     ejecución**. Ahora crea la suya y la borra al terminar.
//
//  3. **Escribía en claro** en columnas que el resto del sistema guarda
//     cifradas. El cifrado vive en el servicio, no en el repositorio, así que
//     esta prueba lo esquivaba y dejaba una fila que la app ya no podía
//     descifrar. Con una cuenta propia y desechable eso deja de importar.
//
// Para correrla hace falta la base local levantada (`.\levantar.ps1`):
//
//	go test ./internal/adapter/storage/postgres -run TestExhaustiveUserUpdate -v
//
// Contra otra base, con la variable:
//
//	LEGACY_TEST_DB_URL=postgres://usuario:clave@host:5432/base?sslmode=disable

// urlDeLaBaseDePruebas devuelve contra qué base correr.
//
// El valor por defecto es el del contenedor de desarrollo que levanta
// `levantar.ps1`, y son las mismas credenciales que ya están escritas en
// CLAUDE.md: no es un secreto de producción, es la base de juguete de cada
// máquina. Cualquier otra cosa se pasa por la variable.
func urlDeLaBaseDePruebas() string {
	if url := os.Getenv("LEGACY_TEST_DB_URL"); url != "" {
		return url
	}
	return "postgres://dba:123@localhost:5432/applegacy?sslmode=disable"
}

// baseDePruebas conecta, o **salta la prueba** si no hay base.
//
// Saltar y no fallar es la diferencia entre una suite que sirve de puerta y una
// que no: `go test ./...` tiene que poder correr en una máquina sin Postgres sin
// dar un rojo que no significa nada.
func baseDePruebas(t *testing.T) *pgxpool.Pool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.Connect(ctx, urlDeLaBaseDePruebas())
	if err != nil {
		t.Skipf("sin base de datos, se salta esta prueba de integración: %v\n"+
			"levanta la base con .\\levantar.ps1, o apunta a otra con LEGACY_TEST_DB_URL", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// usuarioDesechable crea una cuenta solo para esta prueba y la borra al acabar.
//
// **No se toca ninguna cuenta existente**, que era el peor de los defectos de
// esta prueba. El borrado va en `t.Cleanup` para que también ocurra si la
// comprobación falla a mitad.
func usuarioDesechable(t *testing.T, repo *UserRepository) *domain.User {
	t.Helper()
	ctx := context.Background()

	marca := fmt.Sprintf("prueba-update-%d", time.Now().UnixNano())
	user := &domain.User{
		// El índice ciego es único: la marca de tiempo evita chocar con otra
		// ejecución que se quedara a medias.
		EmailBlindIndex: marca,
		EmailEncrypted:  marca + "@prueba.test",
		PasswordHash:    "no-se-usa-en-esta-prueba",
		FirstName:       "Nombre",
		LastName:        "Apellido",
		Role:            "profesional",
	}

	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("no se pudo crear el usuario de prueba: %v", err)
	}
	t.Cleanup(func() {
		if err := repo.Delete(context.Background(), user.ID); err != nil {
			t.Errorf("quedó sin borrar el usuario de prueba %s: %v", user.ID, err)
		}
	})

	return user
}

func TestExhaustiveUserUpdate(t *testing.T) {
	pool := baseDePruebas(t)
	repo := NewUserRepository(pool)
	ctx := context.Background()

	user := usuarioDesechable(t, repo)

	// Una marca distinta por ejecución, para que un valor que no se escribiera
	// no pueda confundirse con el que ya estaba.
	salt := fmt.Sprintf("%d", time.Now().UnixNano())

	user.FirstName = "TestFirst_" + salt
	user.LastName = "TestLast_" + salt
	user.Industry = "Servicios"
	user.Phone = "12345678"
	user.Country = "Colombia"
	user.IdentificationType = "CC"
	user.IdentificationNumber = "ID_" + salt
	user.CustomerStatus = "Ya soy cliente"
	user.Generation = "Segunda"
	user.IsPublicProfile = !user.IsPublicProfile
	user.AllowMessagesFromStrangers = !user.AllowMessagesFromStrangers
	user.ShowActivity = !user.ShowActivity

	// Los tres campos que añadió la carga masiva (2026-09-02). Estaban fuera de
	// esta prueba y es justo lo que comprueba: que el UPDATE no se deje ninguno.
	user.Sexo = "Femenino"
	user.Departamento = "Antioquia"
	user.Direccion = "Calle 10 # 43-21_" + salt

	if err := repo.Update(ctx, user); err != nil {
		t.Fatalf("Update falló: %v", err)
	}

	guardado, err := repo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID falló: %v", err)
	}

	// Se comparan campo a campo y con el nombre delante: «no coincide» a secas
	// obliga a volver a leer el test para saber cuál.
	comparar := map[string][2]string{
		"FirstName":            {guardado.FirstName, user.FirstName},
		"LastName":             {guardado.LastName, user.LastName},
		"Industry":             {guardado.Industry, user.Industry},
		"Phone":                {guardado.Phone, user.Phone},
		"Country":              {guardado.Country, user.Country},
		"IdentificationType":   {guardado.IdentificationType, user.IdentificationType},
		"IdentificationNumber": {guardado.IdentificationNumber, user.IdentificationNumber},
		"CustomerStatus":       {guardado.CustomerStatus, user.CustomerStatus},
		"Generation":           {guardado.Generation, user.Generation},
		"Sexo":                 {guardado.Sexo, user.Sexo},
		"Departamento":         {guardado.Departamento, user.Departamento},
		"Direccion":            {guardado.Direccion, user.Direccion},
	}
	for campo, valores := range comparar {
		if valores[0] != valores[1] {
			t.Errorf("%s: se guardó %q y se esperaba %q", campo, valores[0], valores[1])
		}
	}

	compararBool := map[string][2]bool{
		"IsPublicProfile":            {guardado.IsPublicProfile, user.IsPublicProfile},
		"AllowMessagesFromStrangers": {guardado.AllowMessagesFromStrangers, user.AllowMessagesFromStrangers},
		"ShowActivity":               {guardado.ShowActivity, user.ShowActivity},
	}
	for campo, valores := range compararBool {
		if valores[0] != valores[1] {
			t.Errorf("%s: se guardó %v y se esperaba %v", campo, valores[0], valores[1])
		}
	}
}
