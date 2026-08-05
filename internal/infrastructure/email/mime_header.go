package email

import "mime"

// encodeHeader codifica un valor de cabecera segun el RFC 2047 cuando contiene
// caracteres fuera de ASCII, y lo devuelve tal cual cuando ya es ASCII puro.
//
// Las cabeceras de un correo son ASCII (RFC 5322): el "charset=UTF-8" del
// Content-Type describe unicamente el cuerpo. Al escribir bytes UTF-8 crudos en
// Subject, cada cliente adivinaba el charset y los acentos llegaban como
// mojibake ("¡Bienvenido" se leia como "Ã‚Â¡Bienvenido").
func encodeHeader(v string) string {
	return mime.QEncoding.Encode("UTF-8", v)
}
