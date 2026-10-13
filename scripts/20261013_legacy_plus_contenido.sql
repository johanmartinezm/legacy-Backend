-- 20261013_legacy_plus_contenido.sql
--
-- Hace administrable desde el panel el contenido de la pantalla Legacy+ de la
-- app: los textos fijos (banner, «Cómo accedo» y el subtítulo de beneficios) y
-- los puntos con check.
--
-- Contexto: el cliente reescribió esa pantalla el 2026-10-13 y pidió poder
-- cambiarla sin publicar versión. Los textos fijos van como filas de
-- core.secciones_inicio con `pantalla = 'legacy-plus'` (mismo mecanismo que las
-- tarjetas del Inicio y LSO; vacío = el texto de la app). Los puntos, que
-- pueden ser más o menos, van en una tabla propia con su grupo:
--   'incluye'    → bajo «Tiene acceso a:»
--   'beneficios' → bajo el subtítulo «BENEFICIOS»
--
-- Archivos que dependen de esto:
--   internal/core/domain/legacy_plus_item.go
--   internal/adapter/storage/postgres/legacy_plus_item_repository.go
--   internal/core/services/legacy_plus_item_service.go
--   internal/handler/http/legacy_plus_item_handler.go
--   cmd/server/main.go (GET /api/legacy-plus/items y CRUD en /api/admin/legacy-plus/items)
--
-- Requiere la columna `pantalla` de core.secciones_inicio
-- (20261013_textos_lso.sql).
--
-- Idempotente: CREATE IF NOT EXISTS; los puntos solo se siembran si la tabla
-- está vacía, para no resucitar los que el panel haya borrado.

CREATE TABLE IF NOT EXISTS core.legacy_plus_items (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    grupo          text NOT NULL DEFAULT 'incluye',
    titulo         text NOT NULL,
    texto          text NOT NULL DEFAULT '',
    orden          integer NOT NULL DEFAULT 0,
    activo         boolean NOT NULL DEFAULT true,
    creado_en      timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    actualizado_en timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO core.legacy_plus_items (grupo, titulo, texto, orden)
SELECT * FROM (VALUES
    ('incluye', 'Chats de sinergia',
     'Chats de sinergia entre pares: hay 4 chats para hablar de temas de Empresa, Patrimonio, Familia y otros entre miembros de otras familias empresarias.', 10),
    ('incluye', 'Legacy Board',
     'Red de gobierno corporativo que conecta a familias empresarias y empresas que necesitan crear o fortalecer sus órganos de gobierno (juntas directivas, consejos de familia, directorios, advisory boards y comités) con profesionales que quieren servir en ellos.', 20),
    ('incluye', 'Beneficios',
     'Son descuentos y servicios especiales de aliados o de Legacy Network.', 30),
    ('beneficios', 'Legacy Network',
     'Legacy Test con descuento. Legacy Test: trampas familiares bonificado. Descuento LSO 15% para cualquier programa, 10% de descuento en los libros en nuestra página con el código xxxx. 20% de descuento en el Legacy Summit.', 10),
    ('beneficios', 'Beneficios de aliados',
     'Vitalea, Olfabrand, Boconcept, Espectador, EUDE, JW Marriott.', 20)
) AS v(grupo, titulo, texto, orden)
WHERE NOT EXISTS (SELECT 1 FROM core.legacy_plus_items);

INSERT INTO core.secciones_inicio (slug, nombre, orden, pantalla) VALUES
    ('lp-banner',      'Banner',                 10, 'legacy-plus'),
    ('lp-como-accedo', 'Cómo accedo',            20, 'legacy-plus'),
    ('lp-beneficios',  'Subtítulo de beneficios', 30, 'legacy-plus')
ON CONFLICT (slug) DO NOTHING;
