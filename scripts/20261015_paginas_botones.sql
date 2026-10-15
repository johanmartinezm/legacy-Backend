-- 20261015_paginas_botones.sql
--
-- Añade a core.paginas_informativas la pregunta y los textos de los dos botones
-- que la app pinta al pie de la página, y siembra los de Legacy Board con los
-- textos que hoy lleva escritos la app.
--
-- Contexto: el cliente pidió el 2026-10-15 poder editar desde el panel el
-- bloque «¿Cómo quiere participar?» de Legacy Board, con «Quiero ser parte» y
-- «Quiero buscar», que estaban fijos en
-- App-Movil/lib/presentation/widgets/botones_legacy_board.dart. Lo que hace
-- cada botón (abrir Contáctenos con su asunto) no cambia: solo el texto.
--
-- Archivos que dependen de ella:
--   internal/core/domain/pagina_informativa.go (PreguntaAcciones, BotonPrincipal, BotonSecundario)
--   internal/adapter/storage/postgres/pagina_repository.go
--   internal/core/services/pagina_service.go (límites de longitud)
--
-- Error que evita: aplicar el binario nuevo sin estas columnas deja Legacy
-- Board y la lista de páginas del panel en 500 —
--   ERROR: column "pregunta_acciones" does not exist (SQLSTATE 42703)
-- Va ANTES que el binario.
--
-- Vacío significa «el texto de siempre»: la app lo sustituye por el suyo, así
-- que una página que no tiene botones no se ve afectada.
--
-- Idempotente: ADD COLUMN IF NOT EXISTS, y la siembra solo rellena lo vacío.

ALTER TABLE core.paginas_informativas
    ADD COLUMN IF NOT EXISTS pregunta_acciones text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS boton_principal   text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS boton_secundario  text NOT NULL DEFAULT '';

UPDATE core.paginas_informativas
   SET pregunta_acciones = '¿Cómo quiere participar?'
 WHERE slug = 'legacy-board' AND pregunta_acciones = '';
UPDATE core.paginas_informativas
   SET boton_principal = 'Quiero ser parte'
 WHERE slug = 'legacy-board' AND boton_principal = '';
UPDATE core.paginas_informativas
   SET boton_secundario = 'Quiero buscar'
 WHERE slug = 'legacy-board' AND boton_secundario = '';

-- Comprobación:
--   SELECT slug, pregunta_acciones, boton_principal, boton_secundario
--     FROM core.paginas_informativas;
-- Legacy Board con los tres textos; el resto, vacíos.
