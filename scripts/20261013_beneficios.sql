-- 20261013_beneficios.sql
--
-- Crea core.beneficios: el contenido de la sección Beneficios de la app.
--
-- Contexto: la fila «Beneficios» de la sección Legacy+ del Inicio abre una
-- sección propia, solo para usuarios Legacy+ (es_plus). El 2026-10-13 el
-- cliente aclaró que su contenido es distinto del de la pantalla Legacy+: un
-- slider de beneficios, cada uno con título, descripción, imagen y enlace, que
-- se administran desde el panel («Beneficios»). El mismo día pidió un segundo
-- slider de alianzas con la misma forma: `tipo` distingue «beneficio» y
-- «alianza», y el enlace es opcional en los dos.
--
-- Archivos que dependen de ella:
--   internal/core/domain/beneficio.go
--   internal/adapter/storage/postgres/beneficio_repository.go
--   internal/core/services/beneficio_service.go
--   internal/handler/http/beneficio_handler.go
--   cmd/server/main.go (GET /api/beneficios detrás de RequierePlus; CRUD en /api/admin/beneficios)
--
-- Nace vacía: el contenido lo carga el cliente. Idempotente: CREATE IF NOT EXISTS
-- y ADD COLUMN IF NOT EXISTS (para bases donde ya se creó sin `tipo`).

CREATE TABLE IF NOT EXISTS core.beneficios (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tipo           text NOT NULL DEFAULT 'beneficio',
    titulo         text NOT NULL,
    descripcion    text NOT NULL DEFAULT '',
    imagen_url     text NOT NULL DEFAULT '',
    enlace_url     text NOT NULL DEFAULT '',
    orden          integer NOT NULL DEFAULT 0,
    activo         boolean NOT NULL DEFAULT true,
    creado_en      timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    actualizado_en timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE core.beneficios
    ADD COLUMN IF NOT EXISTS tipo text NOT NULL DEFAULT 'beneficio';
