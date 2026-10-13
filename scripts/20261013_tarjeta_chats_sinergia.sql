-- 20261013_tarjeta_chats_sinergia.sql
--
-- Añade a core.secciones_inicio la fila «Chats de sinergia» de la sección
-- Legacy+ del Inicio de la app.
--
-- Contexto: el cliente pidió el 2026-10-13 un ítem bajo Miembros, con el mismo
-- candado (solo clientes), que abre los foros anónimos. Primero se llamó
-- «Foro»; el mismo día pasó a «Chats de sinergia», sin cambiar de destino. La app lo
-- pinta aunque falte esta fila; la fila es para que el panel («Tarjetas del
-- Inicio») le pueda poner imagen, título y texto como al resto.
--
-- Archivos afectados:
--   App-Movil: lib/presentation/screens/home/home_content_screen.dart
--   Sitio-Administrativo: core/models/seccion-inicio.model.ts (texto de siempre)
--
-- Idempotente: ON CONFLICT DO NOTHING. Borra la fila «foro» que dejó la versión
-- anterior de esta migración en bases de desarrollo; en producción no llegó.

DELETE FROM core.secciones_inicio WHERE slug = 'foro';

INSERT INTO core.secciones_inicio (slug, nombre, orden, pantalla) VALUES
    ('chats-sinergia', 'Chats de sinergia', 105, 'inicio')
ON CONFLICT (slug) DO NOTHING;
