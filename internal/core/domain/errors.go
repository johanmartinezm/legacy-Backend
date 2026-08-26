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

// Motivos por los que no se puede escribir en una conversación.
//
// Los dos son culpa de quien pide, no del servidor: hasta el 2026-08-18 salían
// como **500**, que además de mentir sobre de quién es el problema hacía que
// cualquier panel de errores los contara como caídas del backend.
var (
	// ErrConexionNoAceptada es escribir en una invitación que la otra persona
	// todavía no ha aceptado.
	ErrConexionNoAceptada = errors.New("cannot send messages to an unaccepted connection")

	// ErrNoEsDeLaConversacion es escribir en una conversación ajena.
	ErrNoEsDeLaConversacion = errors.New("unauthorized to send messages to this connection")

	// ErrBloqueado cubre las dos direcciones del bloqueo con el mismo mensaje, a
	// propósito: decir "te han bloqueado" revelaría una decisión de la otra
	// persona, y quien busca acosar sabría que debe cambiar de cuenta.
	ErrBloqueado = errors.New("no es posible contactar con esta persona")
)

// Motivo por el que no se acepta una contraseña.
var (
	// ErrContrasenaCorta es una contraseña por debajo de LongitudMinimaContrasena.
	//
	// El mensaje está en español porque los controladores de restablecer y de
	// cambiar contraseña devuelven err.Error() tal cual al cliente.
	ErrContrasenaCorta = errors.New("la contraseña debe tener al menos 6 caracteres")

	// ErrEstadoDeEventoInvalido es un `status` distinto de "active"/"inactive".
	//
	// En español y por la misma razón: el controlador lo devuelve tal cual.
	ErrEstadoDeEventoInvalido = errors.New("el estado del evento debe ser 'active' o 'inactive'")
)
