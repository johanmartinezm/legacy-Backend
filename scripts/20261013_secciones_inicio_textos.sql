-- 20261013_secciones_inicio_textos.sql
--
-- Añade a core.secciones_inicio el título y el texto de cada tarjeta del Inicio.
--
-- Contexto: 20261012_secciones_inicio.sql dejó en el panel solo la imagen; el
-- título y el texto seguían escritos en la app. El cliente pidió cambiarlos
-- también desde el panel, junto a la imagen.
--
-- Vacío significa «el de siempre»: la app pinta el título y el texto que ya
-- trae en su código. Así las filas existentes no cambian nada al aplicar esto,
-- y una app instalada que no conoce estos campos los ignora sin romperse.
--
-- Archivos afectados:
--   internal/core/domain/seccion_inicio.go
--   internal/adapter/storage/postgres/seccion_inicio_repository.go
--   internal/core/services/seccion_inicio_service.go
--   internal/handler/http/seccion_inicio_handler.go
--
-- Idempotente: ADD COLUMN IF NOT EXISTS.

ALTER TABLE core.secciones_inicio
    ADD COLUMN IF NOT EXISTS titulo text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS texto  text NOT NULL DEFAULT '';
