package domain

import "errors"

// Errores de almacenamiento que los adaptadores traducen desde el motor de base
// de datos, para que los servicios no tengan que conocer códigos de Postgres ni
// buscar subcadenas dentro del mensaje de error.
var (
	// ErrUniqueViolation corresponde al SQLSTATE 23505. El adaptador lo devuelve
	// envuelto; comprobar con errors.Is.
	ErrUniqueViolation = errors.New("unique constraint violation")

	// ErrNotFound es "la consulta no devolvió ninguna fila". Evita que los
	// servicios tengan que importar pgx para reconocer pgx.ErrNoRows.
	ErrNotFound = errors.New("not found")
)
