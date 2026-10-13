-- 20261013_texto_asesoria.sql
--
-- Hace editable desde el panel el texto de bienvenida de la pantalla Asesoría
-- de la app, reutilizando core.secciones_inicio con `pantalla = 'asesoria'`.
--
-- Contexto: el cliente pidió el 2026-10-13 quitar el título «Asesoría abierta a
-- todos» del primer recuadro de Asesoría y poder cambiar su texto desde el
-- panel. Es el mismo mecanismo que 20261013_textos_lso.sql: vacío = el texto
-- de siempre de la app. El título de la fila no se usa: la app ya no lo pinta
-- y el panel no lo ofrece.
--
-- Requiere la columna `pantalla`, que crea 20261013_textos_lso.sql: aplicar
-- esa antes que esta.
--
-- Archivos afectados:
--   Sitio-Administrativo: features/admin/inicio (módulo «Texto de Asesoría»)
--   App-Movil: lib/presentation/screens/asesoria/asesoria_screen.dart
--
-- Idempotente: ON CONFLICT DO NOTHING.

INSERT INTO core.secciones_inicio (slug, nombre, orden, pantalla) VALUES
    ('asesoria-banner', 'Texto de bienvenida', 10, 'asesoria')
ON CONFLICT (slug) DO NOTHING;
