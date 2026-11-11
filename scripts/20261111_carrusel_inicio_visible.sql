-- 20261111_carrusel_inicio_visible.sql
--
-- Permite encender y apagar desde el panel el carrusel de banners del Inicio
-- de la app.
--
-- Contexto: desde el 2026-11-11 la app pinta en el Inicio los banners con
-- ubicación «Inicio» (App-Movil, home_content_screen.dart). El cliente pidió
-- poder ocultar el carrusel entero sin desactivar uno por uno los banners.
-- Se reutiliza core.secciones_inicio, que la app ya consulta al abrir el
-- Inicio (GET /api/inicio/secciones), en vez de crear otra tabla y otra
-- petición.
--
-- Cambios:
--   * Columna `visible` (por defecto true): hoy solo la lee la app para la
--     fila «carrusel-inicio». Las demás filas no cambian de comportamiento.
--   * Fila «carrusel-inicio» con pantalla «banners»: el módulo Inicio del panel
--     lista por pantalla y no la muestra; su interruptor está en la página de
--     Banners.
--
-- Archivos que dependen de ella:
--   internal/core/domain/seccion_inicio.go (Visible, CambiosSeccionInicio.Visible)
--   internal/adapter/storage/postgres/seccion_inicio_repository.go
--   Sitio-Administrativo: banner-list (interruptor)
--   App-Movil: secciones_inicio_service.dart, home_content_screen.dart
--
-- Orden de despliegue: esta migración antes que el binario, que ya selecciona
-- la columna `visible` y fallaría sin ella.
--
-- Idempotente: ADD COLUMN IF NOT EXISTS y ON CONFLICT DO NOTHING.

ALTER TABLE core.secciones_inicio
    ADD COLUMN IF NOT EXISTS visible boolean NOT NULL DEFAULT true;

INSERT INTO core.secciones_inicio (slug, nombre, orden, pantalla)
VALUES ('carrusel-inicio', 'Carrusel de banners del Inicio', 5, 'banners')
ON CONFLICT (slug) DO NOTHING;
