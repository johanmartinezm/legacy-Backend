-- 20260812_identidad_social.sql
--
-- Contexto: `core.users` **no tiene** dónde guardar la identidad del proveedor.
-- `domain.User` declara GoogleID y AppleID y la API los devuelve como null,
-- pero las columnas nunca existieron: en el código quedó incluso un
-- `// Update DB with google ID (dummy update here)`.
--
-- Sin esto, entrar con Google o con Apple solo puede resolverse por correo, y
-- con Apple eso **no funciona**: Apple envía el email únicamente en el primer
-- inicio de sesión, y puede ser una dirección de retransmisión privada
-- (@privaterelay.appleid.com) distinta de la real. El único identificador
-- estable es el `sub` del token.
--
-- Archivos afectados:
--   internal/core/services/auth_service.go        valida y enlaza
--   internal/infrastructure/apple/validator.go    verifica el token de Apple
--   internal/adapter/storage/postgres/user_repository.go
--
-- Qué corrige: que Sign in with Apple no funcione en absoluto —el backend
-- aceptaba cualquier token sin validarlo y devolvía siempre el mismo correo
-- ficticio, user_apple@example.com— y que el vínculo con el proveedor se
-- perdiera en cada inicio de sesión.
--
-- Idempotente: se puede aplicar varias veces sin efecto.

BEGIN;

ALTER TABLE core.users
    ADD COLUMN IF NOT EXISTS google_id text,
    ADD COLUMN IF NOT EXISTS apple_id  text;

COMMENT ON COLUMN core.users.google_id IS
    'Identificador estable de Google (claim "sub"). NULL si nunca entró con Google.';
COMMENT ON COLUMN core.users.apple_id IS
    'Identificador estable de Apple (claim "sub"). Es el único fiable: Apple solo manda el correo en el primer inicio de sesión y puede ser de retransmisión privada.';

-- Únicos, para que dos cuentas no puedan reclamar la misma identidad social.
-- Parciales: los NULL no compiten entre sí, y la inmensa mayoría de cuentas no
-- usa acceso social.
CREATE UNIQUE INDEX IF NOT EXISTS users_google_id_key
    ON core.users (google_id) WHERE google_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS users_apple_id_key
    ON core.users (apple_id) WHERE apple_id IS NOT NULL;

COMMIT;
