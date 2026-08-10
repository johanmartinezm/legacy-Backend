-- 20260810_version_consentimiento.sql
--
-- Registra QUÉ versión de los textos legales aceptó cada persona y CUÁNDO.
--
-- Contexto: core.users guarda hoy el consentimiento en dos booleanos
-- (terms_accepted, data_sharing_accepted, líneas 573-574 de schema.sql). Un
-- booleano dice que se aceptó algo, pero no qué texto ni en qué fecha. La Ley
-- 1581 de 2012 y su Decreto 1377 de 2013 exigen al responsable conservar prueba
-- del modo, la fecha y el CONTENIDO de la autorización.
--
-- Urgencia: los textos legales se están reescribiendo para poder publicar en las
-- tiendas (ver reports/20260810_documentacion_privacidad_contenido.md). En cuanto
-- se publique la redacción nueva, quien la acepte será indistinguible de quien
-- aceptó la anterior, y ese corte no se puede reconstruir hacia atrás. Por eso
-- esta migración va ANTES de que los textos cambien, no después.
--
-- Archivos afectados:
--   internal/core/domain/legal.go              (constantes de versión vigente)
--   internal/core/domain/user.go               (campos nuevos)
--   internal/core/services/auth_service.go     (sella el consentimiento al registrar)
--   internal/adapter/storage/postgres/user_repository.go (INSERT)
--
-- Idempotente: se puede aplicar dos veces sin efecto.

ALTER TABLE core.users
    ADD COLUMN IF NOT EXISTS terms_version            text,
    ADD COLUMN IF NOT EXISTS terms_accepted_at        timestamp with time zone,
    ADD COLUMN IF NOT EXISTS data_sharing_version     text,
    ADD COLUMN IF NOT EXISTS data_sharing_accepted_at timestamp with time zone;

COMMENT ON COLUMN core.users.terms_version IS
    'Versión de los T&C aceptada, por fecha de entrada en vigor del documento. NULL = aceptado antes de que se registraran versiones: consta que aceptó, no qué texto.';

COMMENT ON COLUMN core.users.data_sharing_version IS
    'Versión de la política de tratamiento de datos aceptada. NULL con el mismo significado que terms_version.';

-- Backfill. De las cuentas anteriores sabemos CUÁNDO aceptaron —al registrarse—
-- pero no QUÉ texto vieron: la app ha mostrado su propio aviso legal embebido,
-- que no coincide con los T&C que la empresa considera vigentes.
--
-- Se rellena la fecha y se deja la versión en NULL a propósito. Escribir una
-- versión supuesta sería peor que no tener ninguna: convertiría una laguna
-- conocida en un dato falso que parece prueba.
UPDATE core.users
   SET terms_accepted_at = created_at
 WHERE terms_accepted = true
   AND terms_accepted_at IS NULL;

UPDATE core.users
   SET data_sharing_accepted_at = created_at
 WHERE data_sharing_accepted = true
   AND data_sharing_accepted_at IS NULL;
