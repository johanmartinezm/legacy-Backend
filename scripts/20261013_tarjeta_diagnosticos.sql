-- 20261013_tarjeta_diagnosticos.sql
--
-- Añade a core.secciones_inicio la tarjeta «DIAGNÓSTICOS» de la cuadrícula del
-- Inicio de la app.
--
-- Contexto: el cliente pidió el 2026-10-13 sustituir en la cuadrícula la
-- tarjeta Legacy Board por «DIAGNÓSTICOS», que abre una pantalla nueva con dos
-- enlaces (diagnóstico para familias empresarias y para empresas). La app la
-- pinta aunque falte esta fila; la fila es para que el panel («Tarjetas del
-- Inicio») le pueda poner imagen, título y texto como al resto. La fila
-- legacy-board se conserva: ya no se pinta, y el panel lo indica.
--
-- Orden: el de Legacy Board (85), justo antes de Contáctenos (90).
--
-- Idempotente: ON CONFLICT DO NOTHING.

INSERT INTO core.secciones_inicio (slug, nombre, orden, pantalla) VALUES
    ('diagnosticos', 'Diagnósticos', 85, 'inicio')
ON CONFLICT (slug) DO NOTHING;
