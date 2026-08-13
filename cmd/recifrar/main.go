// Comando de un solo uso para rotar la clave de cifrado en reposo
// (security.encryption_key). Lee cada valor con la clave vieja y lo vuelve a
// escribir con la nueva, dentro de UNA transaccion: o se rota todo, o no se
// toca nada.
//
// Lo que hace distinto a este comando de un simple "descifrar y cifrar":
// core.users.email_blind_index NO es un cifrado sino un HMAC con la misma
// clave (internal/security/crypto.go), y es por donde Login busca al usuario
// (auth_service.go:325). Si se recifran los datos y no se recalcula el indice,
// la base queda intacta y legible y NADIE vuelve a poder iniciar sesion por
// correo: la busqueda no encuentra a nadie.
//
// Uso:
//
//	export RECIFRAR_DSN='postgres://...'
//	export RECIFRAR_CLAVE_VIEJA='...'    # 32 caracteres exactos
//	export RECIFRAR_CLAVE_NUEVA='...'    # 32 caracteres exactos
//	./recifrar                # simulacro: hace todo y deshace al final
//	./recifrar -aplicar       # confirma la transaccion
//
// El backend tiene que estar PARADO. Si sigue aceptando peticiones, las filas
// que escriba mientras corre esto quedan cifradas con la clave vieja detras del
// cursor de la migracion, y se pierden al cambiar la configuracion.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"applegacy/backend/internal/security"

	"github.com/jackc/pgx/v4"
)

// politicaTextoPlano decide que hacer con un valor que no descifra con la clave
// vieja. Pasa de verdad: hay filas anteriores a que existiera el cifrado.
type politicaTextoPlano int

const (
	// abortar es el valor por defecto a proposito. Un valor que no descifra
	// puede ser texto plano heredado, pero tambien un dato cifrado con OTRA
	// clave; cifrar ese segundo caso lo convierte en basura irrecuperable.
	// Que el operador lo mire antes de decidir.
	abortar politicaTextoPlano = iota
	// cifrar asume que el valor es texto plano y lo cifra con la clave nueva.
	// Ademas de rotar, arregla esas filas: hoy se muestran vacias, porque al
	// leerlas Decrypt falla y auth_service.go:433 asigna el resultado igual.
	cifrar
)

type contadores struct {
	Recifrados   int
	DesdePlano   int
	Vacios       int
	IndicesNuevo int
}

func (c *contadores) String() string {
	return fmt.Sprintf("recifrados=%d  cifrados_desde_texto_plano=%d  vacios_intactos=%d",
		c.Recifrados, c.DesdePlano, c.Vacios)
}

// columnasUsuario son las nueve columnas cifradas de core.users, en el mismo
// orden en el que se leen y se escriben. email_encrypted va primero porque su
// texto claro se necesita ademas para recalcular el indice ciego.
var columnasUsuario = []string{
	"email_encrypted",
	"first_name",
	"last_name",
	"phone",
	"location",
	"bio",
	"company_name",
	"job_title",
	"identification_number",
}

func main() {
	aplicar := flag.Bool("aplicar", false, "confirma la transaccion; sin esto es un simulacro que deshace al final")
	textoPlano := flag.String("texto-plano", "abortar", "que hacer con un valor que no descifra: abortar | cifrar")
	flag.Parse()

	politica := abortar
	switch *textoPlano {
	case "abortar":
	case "cifrar":
		politica = cifrar
	default:
		salir(fmt.Errorf("-texto-plano solo acepta 'abortar' o 'cifrar', no %q", *textoPlano))
	}

	dsn := os.Getenv("RECIFRAR_DSN")
	claveVieja := os.Getenv("RECIFRAR_CLAVE_VIEJA")
	claveNueva := os.Getenv("RECIFRAR_CLAVE_NUEVA")
	if dsn == "" || claveVieja == "" || claveNueva == "" {
		salir(errors.New("faltan RECIFRAR_DSN, RECIFRAR_CLAVE_VIEJA o RECIFRAR_CLAVE_NUEVA"))
	}
	if claveVieja == claveNueva {
		salir(errors.New("la clave nueva es identica a la vieja"))
	}

	vieja, err := security.NewCryptoService(claveVieja)
	if err != nil {
		salir(fmt.Errorf("clave vieja: %w", err))
	}
	nueva, err := security.NewCryptoService(claveNueva)
	if err != nil {
		salir(fmt.Errorf("clave nueva: %w", err))
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		salir(fmt.Errorf("no se pudo conectar: %w", err))
	}
	defer conn.Close(ctx)

	tx, err := conn.Begin(ctx)
	if err != nil {
		salir(err)
	}
	// El rollback tardio es inofensivo si ya se hizo Commit, y es la red de
	// seguridad si algo entra en panico a mitad.
	defer tx.Rollback(ctx)

	if err := recifrarUsuarios(ctx, tx, vieja, nueva, politica); err != nil {
		salir(err)
	}
	if err := recifrarMensajes(ctx, tx, vieja, nueva, politica); err != nil {
		salir(err)
	}
	if err := recifrarInscripciones(ctx, tx, vieja, nueva, politica); err != nil {
		salir(err)
	}

	// Verificar ANTES de confirmar. Dentro de la transaccion todavia se puede
	// deshacer; despues del Commit ya no, y un fallo aqui significa datos
	// ilegibles para siempre.
	if err := verificar(ctx, tx, nueva); err != nil {
		salir(fmt.Errorf("la verificacion fallo, no se confirma nada: %w", err))
	}
	fmt.Println("verificacion: todos los valores descifran con la clave nueva y los indices cuadran")

	if !*aplicar {
		fmt.Println("\nSIMULACRO: no se confirma nada. Repite con -aplicar cuando el resultado te cuadre.")
		return
	}
	if err := tx.Commit(ctx); err != nil {
		salir(fmt.Errorf("no se pudo confirmar: %w", err))
	}
	fmt.Println("\nCONFIRMADO. Ahora cambia security.encryption_key en config.docker.yaml y reconstruye la imagen.")
}

// convertir aplica la rotacion a un valor suelto. Devuelve ademas el texto
// claro, que hace falta para el indice ciego del correo.
func convertir(v *string, vieja, nueva *security.CryptoService, p politicaTextoPlano, cuenta *contadores, quien string) (nuevoValor *string, claro string, err error) {
	if v == nil || *v == "" {
		// Vacio se queda vacio: cifrar la cadena vacia la convertiria en un
		// valor que parece dato, y el codigo distingue los dos casos.
		cuenta.Vacios++
		return v, "", nil
	}

	claro, errDesc := vieja.Decrypt(*v)
	if errDesc != nil || claro == "" {
		if p != cifrar {
			return nil, "", fmt.Errorf("%s no descifra con la clave vieja; revisalo y decide, "+
				"o repite con -texto-plano=cifrar si son datos heredados sin cifrar", quien)
		}
		claro = *v
		cuenta.DesdePlano++
	} else {
		cuenta.Recifrados++
	}

	cifrado, err := nueva.Encrypt(claro)
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", quien, err)
	}
	return &cifrado, claro, nil
}

func recifrarUsuarios(ctx context.Context, tx pgx.Tx, vieja, nueva *security.CryptoService, p politicaTextoPlano) error {
	type fila struct {
		id     string
		campos []*string
	}

	sql := "SELECT id, " + unir(columnasUsuario) + " FROM core.users ORDER BY id"
	rows, err := tx.Query(ctx, sql)
	if err != nil {
		return err
	}

	// Se lee todo a memoria antes de escribir: pgx no admite ejecutar UPDATEs
	// sobre la misma conexion con un cursor abierto.
	var filas []fila
	for rows.Next() {
		f := fila{campos: make([]*string, len(columnasUsuario))}
		destinos := make([]any, 0, len(columnasUsuario)+1)
		destinos = append(destinos, &f.id)
		for i := range f.campos {
			destinos = append(destinos, &f.campos[i])
		}
		if err := rows.Scan(destinos...); err != nil {
			rows.Close()
			return err
		}
		filas = append(filas, f)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	var cuenta contadores
	for _, f := range filas {
		nuevos := make([]*string, len(f.campos))
		var correoClaro string

		for i, col := range columnasUsuario {
			nuevo, claro, err := convertir(f.campos[i], vieja, nueva, p, &cuenta, fmt.Sprintf("core.users.%s (id %s)", col, f.id))
			if err != nil {
				return err
			}
			nuevos[i] = nuevo
			if col == "email_encrypted" {
				correoClaro = claro
			}
		}

		// Sin correo legible no hay indice ciego posible, y sin indice ese
		// usuario no puede volver a iniciar sesion. Es el unico campo cuyo
		// fallo no admite continuar.
		if correoClaro == "" {
			return fmt.Errorf("core.users id %s: el correo quedo vacio, no se puede recalcular el indice ciego", f.id)
		}
		// El indice se calcula sobre el correo TAL CUAL, sin ToLower ni
		// TrimSpace: Login tampoco normaliza (auth_service.go:325), asi que
		// normalizar aqui dejaria fuera a quien se registro con una mayuscula.
		indice := nueva.BlindIndex(correoClaro)
		cuenta.IndicesNuevo++

		args := make([]any, 0, len(nuevos)+2)
		for _, n := range nuevos {
			args = append(args, n)
		}
		args = append(args, indice, f.id)

		update := "UPDATE core.users SET " + asignaciones(columnasUsuario) +
			fmt.Sprintf(", email_blind_index = $%d WHERE id = $%d", len(nuevos)+1, len(nuevos)+2)
		if _, err := tx.Exec(ctx, update, args...); err != nil {
			return fmt.Errorf("core.users id %s: %w", f.id, err)
		}
	}

	fmt.Printf("core.users            %d filas   %s   indices_recalculados=%d\n", len(filas), cuenta.String(), cuenta.IndicesNuevo)
	return nil
}

func recifrarMensajes(ctx context.Context, tx pgx.Tx, vieja, nueva *security.CryptoService, p politicaTextoPlano) error {
	return recifrarTablaSimple(ctx, tx, vieja, nueva, p, "chat.messages", []string{"content_encrypted"})
}

func recifrarInscripciones(ctx context.Context, tx pgx.Tx, vieja, nueva *security.CryptoService, p politicaTextoPlano) error {
	return recifrarTablaSimple(ctx, tx, vieja, nueva, p, "events.registrations",
		[]string{"participant_name", "participant_email", "participant_phone"})
}

// recifrarTablaSimple sirve para las tablas sin indice ciego que mantener.
func recifrarTablaSimple(ctx context.Context, tx pgx.Tx, vieja, nueva *security.CryptoService, p politicaTextoPlano, tabla string, columnas []string) error {
	type fila struct {
		id     string
		campos []*string
	}

	rows, err := tx.Query(ctx, "SELECT id, "+unir(columnas)+" FROM "+tabla+" ORDER BY id")
	if err != nil {
		return err
	}
	var filas []fila
	for rows.Next() {
		f := fila{campos: make([]*string, len(columnas))}
		destinos := make([]any, 0, len(columnas)+1)
		destinos = append(destinos, &f.id)
		for i := range f.campos {
			destinos = append(destinos, &f.campos[i])
		}
		if err := rows.Scan(destinos...); err != nil {
			rows.Close()
			return err
		}
		filas = append(filas, f)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	var cuenta contadores
	for _, f := range filas {
		args := make([]any, 0, len(columnas)+1)
		for i, col := range columnas {
			nuevo, _, err := convertir(f.campos[i], vieja, nueva, p, &cuenta, fmt.Sprintf("%s.%s (id %s)", tabla, col, f.id))
			if err != nil {
				return err
			}
			args = append(args, nuevo)
		}
		args = append(args, f.id)

		update := "UPDATE " + tabla + " SET " + asignaciones(columnas) +
			fmt.Sprintf(" WHERE id = $%d", len(columnas)+1)
		if _, err := tx.Exec(ctx, update, args...); err != nil {
			return fmt.Errorf("%s id %s: %w", tabla, f.id, err)
		}
	}

	fmt.Printf("%-21s %d filas   %s\n", tabla, len(filas), cuenta.String())
	return nil
}

// verificar relee lo escrito y comprueba que todo descifra con la clave nueva y
// que cada indice ciego corresponde a su correo. Corre dentro de la misma
// transaccion, que es el unico momento en el que un fallo todavia se puede
// deshacer.
func verificar(ctx context.Context, tx pgx.Tx, nueva *security.CryptoService) error {
	rows, err := tx.Query(ctx, "SELECT id, email_blind_index, "+unir(columnasUsuario)+" FROM core.users ORDER BY id")
	if err != nil {
		return err
	}
	type fila struct {
		id     string
		indice string
		campos []*string
	}
	var filas []fila
	for rows.Next() {
		f := fila{campos: make([]*string, len(columnasUsuario))}
		destinos := []any{&f.id, &f.indice}
		for i := range f.campos {
			destinos = append(destinos, &f.campos[i])
		}
		if err := rows.Scan(destinos...); err != nil {
			rows.Close()
			return err
		}
		filas = append(filas, f)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, f := range filas {
		var correo string
		for i, col := range columnasUsuario {
			if f.campos[i] == nil || *f.campos[i] == "" {
				continue
			}
			claro, err := nueva.Decrypt(*f.campos[i])
			if err != nil {
				return fmt.Errorf("core.users.%s (id %s) no descifra con la clave nueva: %w", col, f.id, err)
			}
			if col == "email_encrypted" {
				correo = claro
			}
		}
		if correo == "" {
			return fmt.Errorf("core.users id %s se quedo sin correo legible", f.id)
		}
		if f.indice != nueva.BlindIndex(correo) {
			return fmt.Errorf("core.users id %s: el indice ciego no corresponde a su correo; ese usuario no podria iniciar sesion", f.id)
		}
	}

	if err := verificarTabla(ctx, tx, nueva, "chat.messages", []string{"content_encrypted"}); err != nil {
		return err
	}
	return verificarTabla(ctx, tx, nueva, "events.registrations",
		[]string{"participant_name", "participant_email", "participant_phone"})
}

func verificarTabla(ctx context.Context, tx pgx.Tx, nueva *security.CryptoService, tabla string, columnas []string) error {
	rows, err := tx.Query(ctx, "SELECT id, "+unir(columnas)+" FROM "+tabla+" ORDER BY id")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		campos := make([]*string, len(columnas))
		destinos := []any{&id}
		for i := range campos {
			destinos = append(destinos, &campos[i])
		}
		if err := rows.Scan(destinos...); err != nil {
			return err
		}
		for i, col := range columnas {
			if campos[i] == nil || *campos[i] == "" {
				continue
			}
			if _, err := nueva.Decrypt(*campos[i]); err != nil {
				return fmt.Errorf("%s.%s (id %s) no descifra con la clave nueva: %w", tabla, col, id, err)
			}
		}
	}
	return rows.Err()
}

func unir(columnas []string) string {
	return strings.Join(columnas, ", ")
}

func asignaciones(columnas []string) string {
	partes := make([]string, len(columnas))
	for i, c := range columnas {
		partes[i] = fmt.Sprintf("%s = $%d", c, i+1)
	}
	return strings.Join(partes, ", ")
}

func salir(err error) {
	fmt.Fprintln(os.Stderr, "ERROR:", err)
	os.Exit(1)
}
