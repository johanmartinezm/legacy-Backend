-- 20260731_add_synergies_comments_count.sql
--
-- Añade el contador denormalizado de comentarios a community.synergies.
--
-- Contexto: internal/adapter/storage/postgres/synergy_repository.go consulta
-- s.comments_count (líneas 31 y 55) e incrementa la columna al crear un
-- comentario (línea 124). La columna NO existe en la base de producción, por lo
-- que GET /api/synergies, GET /api/synergies/{id} y POST /api/synergies/{id}/comments
-- fallan con: ERROR: column s.comments_count does not exist (SQLSTATE 42703).
--
-- El informe reports/qa_synergy_search_report.md (2026-03-12) da por verificada
-- esta columna, pero la migración nunca se aplicó al servidor.

ALTER TABLE community.synergies
    ADD COLUMN IF NOT EXISTS comments_count integer NOT NULL DEFAULT 0;

-- Backfill: alinea el contador con los comentarios ya existentes.
UPDATE community.synergies s
   SET comments_count = c.total
  FROM (
        SELECT synergy_id, count(*) AS total
          FROM community.synergy_comments
         GROUP BY synergy_id
       ) c
 WHERE c.synergy_id = s.id
   AND s.comments_count IS DISTINCT FROM c.total;
