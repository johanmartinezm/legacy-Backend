# Reporte Técnico: Implementación de Verificación de Correo (Backend Go)

**Fecha:** 2026-07-15
**Autor:** Antigravity / IA Arquitecto

## Cambios Realizados
1. **Dominio:** 
   - Se añadió el campo booleano `EmailVerified` a la estructura `domain.User`.
2. **Repositorio (PostgreSQL):**
   - Se actualizó `user_repository.go` para insertar el campo por defecto en false, y leerlo en consultas.
   - Se implementó `email_verification_repository.go` gestionando la tabla de tokens temporales (`email_verification_tokens`).
3. **Servicios Core:**
   - **GmailService:** Se añadió la función `SendVerificationEmail(email, verifyLink string) error` usando autenticación base de SMTP.
   - **AuthService:** 
     - Se adaptó `Register` para generar el token de 32 bytes y despachar el correo de forma asíncrona mediante Go routines (`go func()`).
     - Se bloquea la autenticación en `Login` si `EmailVerified` es `false`, retornando el error `email_not_verified`.
     - Nuevas funciones `VerifyEmail` y `ResendVerificationEmail` agregadas para validar el token y reenviar respectivamente.
4. **Capa HTTP & Rutas:**
   - Se agregaron los handlers `VerifyEmail` y `ResendVerificationEmail` en `user_handler.go`.
   - Se registraron las rutas POST en `/api/verify-email` y `/api/resend-verification` en `main.go`.

## Validaciones y Riesgos
- **Seguridad:** Los tokens son estocásticos (32 bytes base) y expiran en 24 horas, vinculados mediante el `blind_index` del correo electrónico para preservar el cifrado PII.
- **Retrocompatibilidad:** Usuarios de Social Login (Google/Apple) quedan validados automáticamente (`EmailVerified = true`).
