-- 20261013_textos_lso.sql
--
-- Hace editables desde el panel el encabezado y el banner principal de la
-- pantalla LSO · Escuela de la app, reutilizando core.secciones_inicio.
--
-- Contexto: el cliente pidió el 2026-10-13 cambiar desde el panel el título y
-- el texto de esa pantalla, igual que ya hace con las tarjetas del Inicio. En
-- vez de una tabla nueva se añade la columna `pantalla` («inicio» o «lso») y
-- dos filas: el mecanismo es el mismo —título y texto, vacíos = los de la app—
-- y así comparten servicio, endpoints y validaciones.
--
-- Archivos afectados:
--   internal/core/domain/seccion_inicio.go (campo Pantalla)
--   internal/adapter/storage/postgres/seccion_inicio_repository.go
--   Sitio-Administrativo: features/admin/inicio (módulo «Textos de LSO»)
--   App-Movil: lib/presentation/screens/programs/programs_screen.dart
--
-- Las dos filas nacen vacías: la app sigue mostrando sus textos hasta que el
-- panel escriba otros. GET /api/inicio/secciones devuelve también estas filas;
-- el Inicio de la app las ignora porque busca sus tarjetas por slug.
--
-- Idempotente: ADD COLUMN IF NOT EXISTS y ON CONFLICT DO NOTHING.

ALTER TABLE core.secciones_inicio
    ADD COLUMN IF NOT EXISTS pantalla text NOT NULL DEFAULT 'inicio';

INSERT INTO core.secciones_inicio (slug, nombre, orden, pantalla) VALUES
    ('lso-encabezado', 'Encabezado',        10, 'lso'),
    ('lso-banner',     'Banner principal',  20, 'lso')
ON CONFLICT (slug) DO NOTHING;
