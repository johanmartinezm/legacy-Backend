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

// Motivos por los que un inicio de sesión no prospera.
//
// Viven en el dominio y no en el servicio para que el handler pueda distinguirlos
// sin importar el paquete de servicios: la dirección de las dependencias va de
// fuera hacia dentro, y un controlador que importa servicios la invierte.
var (
	// ErrCredencialesInvalidas cubre a la vez "no existe esa cuenta" y "la
	// contraseña no es". **Tiene que seguir siendo indistinguible**: separarlos
	// convierte el inicio de sesión en un comprobador de qué correos están
	// registrados.
	ErrCredencialesInvalidas = errors.New("invalid credentials")

	// ErrCorreoSinVerificar solo puede salir **después** de acertar la
	// contraseña. Antes de eso confirmaría que la cuenta existe.
	ErrCorreoSinVerificar = errors.New("email_not_verified")

	// ErrCuentaSocial es quien se registró con Google o Apple y prueba a entrar
	// con contraseña. Su cuenta no tiene ninguna.
	ErrCuentaSocial = errors.New("account uses social login")
)
