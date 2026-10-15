-- 20261015_asesoria_unidades_imagen.sql
--
-- Añade core.asesoria_unidades.imagen_url: un icono o imagen subido desde el
-- panel para cada unidad de Asesoría.
--
-- Contexto: el cliente pidió el 2026-10-15 poder poner a cada unidad su propio
-- icono o imagen (el logotipo de L&M, Aurum…) en lugar de elegir uno de los
-- iconos genéricos de la lista cerrada. Si hay imagen, la app la pinta; si no,
-- sigue pintando el icono de `icono`, que no se toca.
--
-- Archivos que dependen de ella:
--   internal/core/domain/asesoria_unidad.go (ImagenURL)
--   internal/adapter/storage/postgres/asesoria_unidad_repository.go
--   internal/core/services/asesoria_unidad_service.go (valida http/https)
--
-- Error que evita: aplicar el binario nuevo sin esta columna rompe la pantalla
-- entera, no solo la imagen —
--   ERROR: column "imagen_url" does not exist (SQLSTATE 42703)
-- en GET /api/asesoria/unidades y en todo el CRUD del panel. Va ANTES que el
-- binario.
--
-- Idempotente: ADD COLUMN IF NOT EXISTS, con '' por defecto para las unidades
-- que ya existen.

ALTER TABLE core.asesoria_unidades
    ADD COLUMN IF NOT EXISTS imagen_url text NOT NULL DEFAULT '';

-- Comprobación:
--   SELECT titulo, icono, imagen_url FROM core.asesoria_unidades ORDER BY orden;
-- debe listar las unidades con imagen_url vacío.
