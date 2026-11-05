-- 20261105_material_de_workshops.sql
--
-- Material de cada workshop: una imagen, un archivo descargable y un video de
-- YouTube o Vimeo (parte A del plan reports/20261002_plan_material_talleres.md,
-- en la raíz del monorepo, no versionado).
--
-- Contexto: el material se carga desde el panel, en la pantalla de workshops
-- del evento, con su propia ruta (PUT /api/workshops/{id}/material, en
-- event_handler.go). Va en columnas aparte de las del workshop a propósito: el
-- UPDATE de PUT /api/workshops/{id} escribe todas las columnas que recibe, y si
-- el material estuviera ahí cada edición del workshop lo borraría.
--
-- La imagen del material es OTRA imagen, distinta de image_url (la del workshop,
-- que ya pinta el cronograma de la app): decidido el 2026-10-28.
--
-- Columnas:
--   material_image_url          imagen del material, como image_url
--   material_file_name          nombre del archivo en disco (UUID_nombre), se
--                               sirve por GET /api/documents/{nombre}
--   material_file_original_name nombre con el que se subió, para mostrarlo
--   material_file_size          tamaño en bytes (hasta 25 MB)
--   material_video_url          enlace de YouTube o de Vimeo
--
-- Aditiva e idempotente (§8 del plan): solo ADD COLUMN IF NOT EXISTS, sin
-- DELETE ni UPDATE sobre los workshops que ya existen. Se puede correr dos
-- veces sin efecto.

BEGIN;

ALTER TABLE events.workshops
    ADD COLUMN IF NOT EXISTS material_image_url          text,
    ADD COLUMN IF NOT EXISTS material_file_name          text,
    ADD COLUMN IF NOT EXISTS material_file_original_name text,
    ADD COLUMN IF NOT EXISTS material_file_size          bigint,
    ADD COLUMN IF NOT EXISTS material_video_url          text;

COMMIT;
