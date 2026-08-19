// Comando de un solo uso: recalcula core.users.email_blind_index con el correo
// normalizado (sin espacios y en minúsculas).
//
// Por qué hace falta. Hasta el 2026-08-18 `BlindIndex` aplicaba el HMAC al correo
// tal como llegaba. Consecuencias comprobadas ese día contra la base local:
//
//   - Iniciar sesión escribiendo el correo con otra caja fallaba con
//     "Credenciales inválidas". En un teclado móvil la primera letra sale en
//     mayúscula, así que no es un caso raro.
//   - El mismo correo se podía registrar **dos veces**: `dup@…` y `DUP@…`
//     devolvieron 201 los dos y quedaron dos cuentas distintas.
//
// Arreglar `BlindIndex` no basta: los índices ya guardados se calcularon sobre el
// texto sin normalizar, así que el servidor nuevo no encontraría a quien se
// registró con mayúsculas y lo dejaría fuera de su propia cuenta. Este comando
// pone la base de acuerdo con el código.
//
// 🔴 Los choques se detectan ANTES de escribir. Si dos cuentas normalizan al
// mismo correo, sus índices colisionarían contra la restricción UNIQUE
// `users_email_key` y el UPDATE fallaría a medias. El comando aborta y los lista
// por id, sin exponer correos: qué hacer con esas cuentas —fusionarlas, borrar
// una, renombrar— es una decisión de negocio, no de este comando.
//
// Uso, con el backend parado para que nadie escriba detrás del cursor:
//
//	export REINDEXAR_DSN=...          # el mismo dsn del config, con host alcanzable
//	export REINDEXAR_CLAVE=...        # encryption_key en uso, 32 caracteres
//	./reindexar            # simulacro: hace todo el trabajo y lo deshace
//	./reindexar -aplicar   # confirma
package main

import (
	"applegacy/backend/internal/security"
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/jackc/pgx/v4"
)

type usuario struct {
	id            string
	correoClaro   string
	indiceViejo   string
	indiceNuevo   string
	correoNormal  string
	necesitaTocar bool
}

func main() {
	aplicar := flag.Bool("aplicar", false, "confirma la transaccion; sin esto es un simulacro que deshace al final")
	flag.Parse()

	dsn := os.Getenv("REINDEXAR_DSN")
	clave := os.Getenv("REINDEXAR_CLAVE")
	if dsn == "" || clave == "" {
		salir(fmt.Errorf("faltan REINDEXAR_DSN o REINDEXAR_CLAVE"))
	}

	crypto, err := security.NewCryptoService(clave)
	if err != nil {
		salir(fmt.Errorf("clave invalida: %w", err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		salir(fmt.Errorf("no se pudo conectar: %w", err))
	}
	defer conn.Close(ctx)

	tx, err := conn.Begin(ctx)
	if err != nil {
		salir(err)
	}
	// Inofensivo si ya se hizo Commit, y es la red de seguridad del simulacro.
	defer tx.Rollback(ctx)

	usuarios, err := leerUsuarios(ctx, tx, crypto)
	if err != nil {
		salir(err)
	}
	fmt.Printf("usuarios leidos: %d\n", len(usuarios))

	// 1. Choques ANTES de escribir nada.
	if choques := detectarChoques(usuarios); len(choques) > 0 {
		fmt.Println()
		fmt.Println("ABORTADO: hay cuentas que normalizan al mismo correo.")
		fmt.Println("Sus indices chocarian contra la restriccion UNIQUE users_email_key.")
		fmt.Println("Decide que hacer con ellas antes de reindexar:")
		for _, grupo := range choques {
			fmt.Printf("  %d cuentas comparten un correo normalizado: %v\n", len(grupo), grupo)
		}
		os.Exit(1)
	}

	// 2. Reindexar.
	tocados := 0
	for _, u := range usuarios {
		if !u.necesitaTocar {
			continue
		}
		_, err := tx.Exec(ctx,
			"UPDATE core.users SET email_blind_index = $1 WHERE id = $2",
			u.indiceNuevo, u.id)
		if err != nil {
			salir(fmt.Errorf("actualizando %s: %w", u.id, err))
		}
		tocados++
	}
	fmt.Printf("indices actualizados: %d (los demas ya estaban normalizados)\n", tocados)

	// 3. Verificar DENTRO de la transaccion: releer y comprobar que el indice
	// guardado es el que el servidor calculara al iniciar sesion. Es el ultimo
	// momento en el que un fallo todavia se puede deshacer.
	if err := verificar(ctx, tx, crypto); err != nil {
		salir(fmt.Errorf("verificacion fallida, no se confirma nada: %w", err))
	}
	fmt.Println("verificacion correcta: cada indice corresponde a su correo normalizado")

	if !*aplicar {
		fmt.Println()
		fmt.Println("SIMULACRO: no se confirma nada. Repite con -aplicar cuando estes conforme.")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		salir(err)
	}
	fmt.Println()
	fmt.Println("APLICADO.")
}

func leerUsuarios(ctx context.Context, tx pgx.Tx, crypto *security.CryptoService) ([]usuario, error) {
	filas, err := tx.Query(ctx, "SELECT id, email_encrypted, email_blind_index FROM core.users")
	if err != nil {
		return nil, err
	}
	defer filas.Close()

	var out []usuario
	for filas.Next() {
		var u usuario
		var cifrado string
		if err := filas.Scan(&u.id, &cifrado, &u.indiceViejo); err != nil {
			return nil, err
		}

		claro, err := crypto.Decrypt(cifrado)
		if err != nil {
			// Un correo que no descifra con la clave en uso no se puede
			// reindexar sin inventarse el valor. Se aborta: es preferible a
			// dejar una cuenta con un indice que no corresponde a nadie.
			return nil, fmt.Errorf("el correo de %s no descifra con esta clave: %w", u.id, err)
		}

		u.correoClaro = claro
		u.correoNormal = security.NormalizarCorreo(claro)
		u.indiceNuevo = crypto.BlindIndex(claro)
		u.necesitaTocar = u.indiceNuevo != u.indiceViejo
		out = append(out, u)
	}
	return out, filas.Err()
}

// detectarChoques agrupa por correo normalizado y devuelve los grupos con mas de
// una cuenta. Devuelve ids, nunca correos: este comando se ejecuta en produccion
// y su salida acaba en un log.
func detectarChoques(usuarios []usuario) [][]string {
	porCorreo := map[string][]string{}
	for _, u := range usuarios {
		porCorreo[u.correoNormal] = append(porCorreo[u.correoNormal], u.id)
	}

	var choques [][]string
	for _, ids := range porCorreo {
		if len(ids) > 1 {
			sort.Strings(ids)
			choques = append(choques, ids)
		}
	}
	sort.Slice(choques, func(i, j int) bool { return choques[i][0] < choques[j][0] })
	return choques
}

// verificar relee la tabla ya actualizada y comprueba que el indice guardado es
// exactamente el que `BlindIndex` produce para ese correo. Es la comprobacion de
// que el inicio de sesion va a encontrar a la persona.
func verificar(ctx context.Context, tx pgx.Tx, crypto *security.CryptoService) error {
	filas, err := tx.Query(ctx, "SELECT id, email_encrypted, email_blind_index FROM core.users")
	if err != nil {
		return err
	}
	defer filas.Close()

	revisados := 0
	for filas.Next() {
		var id, cifrado, indice string
		if err := filas.Scan(&id, &cifrado, &indice); err != nil {
			return err
		}
		claro, err := crypto.Decrypt(cifrado)
		if err != nil {
			return fmt.Errorf("%s: %w", id, err)
		}
		if esperado := crypto.BlindIndex(claro); esperado != indice {
			return fmt.Errorf("%s: el indice guardado no corresponde a su correo", id)
		}
		revisados++
	}
	if err := filas.Err(); err != nil {
		return err
	}
	fmt.Printf("verificados: %d\n", revisados)
	return nil
}

func salir(err error) {
	fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
	os.Exit(1)
}
