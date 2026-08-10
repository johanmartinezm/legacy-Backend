package domain

// Versiones vigentes de los textos legales que se aceptan al registrarse.
//
// Se guardan junto al consentimiento (core.users.terms_version y
// data_sharing_version) para poder probar QUÉ texto aceptó cada persona, no solo
// que aceptó algo. El formato es la fecha de entrada en vigor del documento.
//
// ACTUALIZAR ESTAS CONSTANTES cada vez que se publique una redacción nueva. Si
// el texto cambia y la constante no, todas las personas quedan registradas
// aceptando una versión que ya no es la que leyeron, que es exactamente el
// problema que estas columnas vienen a resolver.
//
// La versión la fija el servidor y nunca el cliente: una app antigua —o
// manipulada— no debe poder declarar que se aceptó un texto distinto del
// vigente.
const (
	// TermsVersionVigente es la versión de los Términos y Condiciones de la APP.
	// El documento declara en su cláusula 2 que entra en vigor el 01/04/2026.
	//
	// Ojo: a fecha de hoy esos T&C no están publicados ni en la web ni dentro de
	// la app, y el aviso legal que la app muestra al registrarse es un texto
	// distinto y más corto. Está encargada la corrección; cuando se publique,
	// esta constante debe subir a la fecha del texto nuevo.
	TermsVersionVigente = "2026-04-01"

	// PrivacyVersionVigente es la versión de la política de tratamiento de datos
	// publicada en https://legacynetworkco.com/politica-de-privacidad/, fechada
	// en el propio documento el 02/06/2026.
	PrivacyVersionVigente = "2026-06-02"
)
