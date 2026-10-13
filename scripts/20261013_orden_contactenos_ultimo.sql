-- 20261013_orden_contactenos_ultimo.sql
--
-- Deja Contáctenos al final de la cuadrícula en core.secciones_inicio.
--
-- Contexto: el 2026-10-13 el cliente pidió, en tres pasos el mismo día, quitar
-- de «Explore libremente» las tarjetas Beneficios y Legacy+ y dejar Contáctenos
-- la última. La cuadrícula de la app queda: Legacy Knowledge, Eventos,
-- LSO · Escuela, Asesorías, Legacy Board, Contáctenos. Esta columna solo
-- replica ese orden para el panel («Tarjetas del Inicio»): con
-- 20261013_orden_legacy_board.sql Legacy Board había quedado en 95, detrás de
-- Contáctenos (90).
--
-- Legacy+ (80) se deja en la tabla: ya no se pinta en el Inicio, y el panel lo
-- indica en «Dónde se ve».
--
-- Idempotente: fija un valor, no lo incrementa.

UPDATE core.secciones_inicio SET orden = 85 WHERE slug = 'legacy-board';
