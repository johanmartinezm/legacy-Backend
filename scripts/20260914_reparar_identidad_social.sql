-- 20260914_reparar_identidad_social.sql
--
-- Contexto: `core.users.apple_id` y `google_id` debían guardar el claim `sub`
-- del proveedor (así lo dice el COMMENT que puso 20260812_identidad_social.sql),
-- pero el registro guardaba el **token de identidad entero**. La app lo manda
-- así —App-Movil/lib/presentation/screens/register_screen.dart:115 pasa el
-- `identityToken` como `apple_id`— y el controlador lo copiaba tal cual
-- (internal/handler/http/user_handler.go:92).
--
-- Qué corrige: que Sign in with Apple no funcione al VOLVER a entrar.
-- AuthService.SocialLogin busca por el `sub` del token, y un JWT de ~900
-- caracteres no coincide nunca. Quedaba el respaldo de buscar por correo, que
-- con Google funciona —su token siempre trae el correo— pero con Apple no:
-- Apple solo manda el correo en la PRIMERA autorización de cada Apple ID con la
-- app. A partir de la segunda no queda nada con que reconocer la cuenta, y la
-- respuesta es 404 "user not registered". Es el rechazo 2.1(a) de App Review
-- del 2026-09-14: «unable to use the core feature, Sign in with Apple».
--
-- Archivos afectados:
--   internal/core/services/auth_service.go   normalizarIdentidadSocial (nuevo)
--   internal/core/domain/errors.go           ErrIdentidadSocialInvalida
--   internal/handler/http/user_handler.go    400 en vez de 500
--
-- El código ya no vuelve a escribir un token, pero las filas escritas antes
-- siguen rotas: esta migración las repara extrayendo el `sub` de la carga útil
-- del JWT. No hace falta la clave de firma —solo se lee el contenido— y no se
-- toca ninguna fila cuyo valor ya sea un `sub`.
--
-- Idempotente: al aplicarla otra vez, `LIKE '%.%.%'` ya no encuentra nada.

BEGIN;

-- Decodifica la carga útil (segmento central) de un JWT.
--
-- base64url no es base64: cambia '+/' por '-_' y se come el relleno. Postgres
-- solo entiende el estándar, así que hay que deshacer las dos cosas antes de
-- decodificar, o `decode()` falla con "invalid symbol".
CREATE OR REPLACE FUNCTION pg_temp.sub_de_jwt(token text) RETURNS text AS $$
DECLARE
    carga text;
BEGIN
    carga := split_part(token, '.', 2);
    IF carga = '' THEN
        RETURN NULL;
    END IF;
    carga := translate(carga, '-_', '+/');
    carga := carga || repeat('=', (4 - length(carga) % 4) % 4);
    RETURN (convert_from(decode(carga, 'base64'), 'UTF8')::jsonb) ->> 'sub';
EXCEPTION WHEN others THEN
    -- Un valor que no sea un JWT legible se deja como está: mejor una cuenta
    -- que haya que reparar a mano que un NULL que borre el vínculo.
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Un `sub` de Apple no lleva puntos dobles ni firma; un JWT tiene tres
-- segmentos y es largo. El filtro de longitud evita tocar un `sub` de Google,
-- que es un número de 21 dígitos sin puntos.
UPDATE core.users
   SET apple_id = pg_temp.sub_de_jwt(apple_id),
       updated_at = now()
 WHERE apple_id IS NOT NULL
   AND apple_id LIKE '%.%.%'
   AND length(apple_id) > 100
   AND pg_temp.sub_de_jwt(apple_id) IS NOT NULL;

UPDATE core.users
   SET google_id = pg_temp.sub_de_jwt(google_id),
       updated_at = now()
 WHERE google_id IS NOT NULL
   AND google_id LIKE '%.%.%'
   AND length(google_id) > 100
   AND pg_temp.sub_de_jwt(google_id) IS NOT NULL;

-- Qué quedó sin reparar: filas con un valor que parece un token pero cuya carga
-- útil no trae `sub`. Se revisan a mano; entrar con el proveedor las volvería a
-- mandar al registro.
DO $$
DECLARE
    pendientes integer;
BEGIN
    SELECT count(*) INTO pendientes
      FROM core.users
     WHERE (apple_id LIKE '%.%.%' AND length(apple_id) > 100)
        OR (google_id LIKE '%.%.%' AND length(google_id) > 100);
    IF pendientes > 0 THEN
        RAISE WARNING 'quedan % filas con identidad social ilegible', pendientes;
    END IF;
END;
$$;

COMMIT;
