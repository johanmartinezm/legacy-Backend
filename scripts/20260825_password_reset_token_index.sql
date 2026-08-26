-- 20260825_password_reset_token_index.sql
--
-- Índice único sobre core.password_reset_tokens.token, para poder resolver el
-- correo a partir del token.
--
-- Contexto: internal/core/services/auth_service.go armaba el enlace de
-- recuperación como "<url>?token=<token>&email=<correo>", así que el correo de
-- la persona viajaba en la barra de direcciones. De ahí se filtra a sitios que
-- nadie controla: el historial del navegador, la cabecera Referer hacia
-- cualquier recurso de terceros que cargue esa página, los registros de proxies
-- y, si el enlace se reenvía o se pega en cualquier parte, a quien lo reciba.
-- Además iba sin escapar, de modo que un "+" en la dirección llegaba al panel
-- convertido en espacio.
--
-- El token ya identifica la solicitud por sí solo —32 bytes aleatorios— y la
-- tabla guarda el correo junto a él, así que el backend puede resolverlo sin que
-- el enlace lo lleve. Esta es la consulta que lo hace:
--
--     SELECT email FROM core.password_reset_tokens
--      WHERE token = $1 AND expires_at > now()
--
-- La tabla solo tenía clave primaria por email, de modo que esa búsqueda haría
-- un recorrido completo. Se crea único, no simple: dos filas con el mismo token
-- serían un fallo del generador, y el índice único garantiza que la consulta
-- devuelve como mucho una fila — que es la propiedad de la que depende la
-- seguridad del flujo.
--
-- Archivos afectados:
--   internal/adapter/storage/postgres/password_reset_repository.go (GetEmailByToken)
--   internal/core/ports/interfaces.go
--   internal/core/services/auth_service.go (RequestPasswordReset, ResetPassword)
--   internal/handler/http/user_handler.go (ResetPassword)
--   Sitio-Administrativo/src/app/features/auth/reset-password/
--
-- Los enlaces ya enviados siguen funcionando: llevan el token, que es lo único
-- que se usa ahora; el "&email=" sobrante se ignora.

-- Las filas caducadas no aportan nada y podrían chocar con el índice único si
-- alguna vez se repitiera un token. Se limpian antes de crearlo.
DELETE FROM core.password_reset_tokens
 WHERE expires_at <= now();

CREATE UNIQUE INDEX IF NOT EXISTS password_reset_tokens_token_key
    ON core.password_reset_tokens (token);
