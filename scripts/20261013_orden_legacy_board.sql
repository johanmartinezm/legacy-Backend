-- 20261013_orden_legacy_board.sql
--
-- Pasa «Legacy Board» del primer al último lugar de la cuadrícula en
-- core.secciones_inicio.
--
-- Contexto: el cliente pidió el 2026-10-13 que Legacy Board deje de abrir la
-- cuadrícula «Explore libremente» del Inicio de la app y quede la última,
-- detrás de Contáctenos. El orden de la pantalla lo decide la app
-- (home_content_screen.dart); esta columna solo replica ese orden para que el
-- panel («Tarjetas del Inicio») liste las tarjetas igual que se ven. Sin esto
-- el panel la seguiría mostrando arriba del todo.
--
-- Orden resultante: ..., Legacy+ 80, Contáctenos 90, Legacy Board 95,
-- Miembros 100, Legacy Test 110.
--
-- Idempotente: fija un valor, no lo incrementa.

UPDATE core.secciones_inicio SET orden = 95 WHERE slug = 'legacy-board';
