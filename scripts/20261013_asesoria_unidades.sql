-- 20261013_asesoria_unidades.sql
--
-- Crea core.asesoria_unidades y siembra las cinco unidades que la pantalla
-- Asesoría de la app tenía escritas en su código.
--
-- Contexto: el cliente pidió el 2026-10-13 que las unidades de Asesoría (L&M
-- Consultoría, Aurum, Legacy Legal, LSO, Network en Gobierno Corporativo) se
-- administren desde el panel: crear, editar, ordenar, ocultar y borrar, sin
-- publicar una versión nueva de la app.
--
-- Archivos que dependen de ella:
--   internal/core/domain/asesoria_unidad.go
--   internal/adapter/storage/postgres/asesoria_unidad_repository.go
--   internal/core/services/asesoria_unidad_service.go
--   internal/handler/http/asesoria_unidad_handler.go
--   cmd/server/main.go (GET /api/asesoria/unidades y CRUD en /api/admin/asesoria/unidades)
--
-- `necesidad` es la opción de «¿Qué necesita?» que el formulario marca al
-- entrar por esa unidad; `icono` es una clave de una lista cerrada que la app
-- traduce a un icono. Las dos se validan en el servicio.
--
-- Idempotente: CREATE IF NOT EXISTS, y la siembra solo si la tabla está vacía,
-- para no resucitar unidades que el panel haya borrado.

CREATE TABLE IF NOT EXISTS core.asesoria_unidades (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    titulo         text NOT NULL,
    subtitulo      text NOT NULL DEFAULT '',
    icono          text NOT NULL DEFAULT 'negocio',
    necesidad      text NOT NULL DEFAULT 'Otro',
    orden          integer NOT NULL DEFAULT 0,
    activa         boolean NOT NULL DEFAULT true,
    creada_en      timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    actualizada_en timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO core.asesoria_unidades (titulo, subtitulo, icono, necesidad, orden)
SELECT * FROM (VALUES
    ('L&M Consultoría',                 'Gobierno corporativo, empresa familiar y patrimonio', 'escudo',      'Gobierno',         10),
    ('Aurum Legacy Advisors',           'Finanzas estratégicas y crecimiento patrimonial',     'crecimiento', 'Patrimonio',       20),
    ('Legacy Legal',                    'Estructuración jurídica, tributaria y patrimonial',   'balanza',     'Legal/Tributario', 30),
    ('LSO',                             'Formación y certificación para propietarios',          'formacion',   'Certificación',    40),
    ('Network en Gobierno Corporativo', 'Consejeros y miembros de junta',                       'grupo',       'Gobierno',         50)
) AS v(titulo, subtitulo, icono, necesidad, orden)
WHERE NOT EXISTS (SELECT 1 FROM core.asesoria_unidades);
