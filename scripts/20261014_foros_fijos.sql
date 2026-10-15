-- 20261014_foros_fijos.sql
--
-- Los foros pasan a ser cuatro fijos: Empresa, Patrimonio, Familia y Otros.
--
-- Contexto: decisión del cliente del 2026-10-14. Hasta ahora cualquier usuario
-- (Legacy+ desde el 13-10) podía crear un foro desde la app con
-- POST /api/forums; esa ruta se retira en el mismo cambio y solo el panel
-- (POST /api/admin/forums) puede crear foros. Los usuarios siguen entrando y
-- comentando.
--
-- Archivos afectados:
--   cmd/server/main.go                                   (se retira POST /api/forums)
--   internal/adapter/storage/postgres/forum_repository.go (ORDER BY f.orden, ...)
--
-- Qué hace:
--   1. Añade core.forums.orden. Sin esta columna el backend nuevo falla en
--      GET /api/forums y /api/admin/forums con
--      ERROR: column f.orden does not exist (SQLSTATE 42703): aplicar ANTES de
--      desplegar el binario.
--   2. Deja activos los cuatro foros, en ese orden. Si ya existe uno con el
--      mismo título (sin distinguir mayúsculas), se reutiliza el más antiguo
--      y conserva sus publicaciones; si no, se crea como foro de administración.
--   3. Retira el resto con borrado lógico (status = 'deleted', lo mismo que el
--      botón «Eliminar» del panel). Sus publicaciones NO se borran: siguen en
--      core.forum_posts y basta con volver el foro a 'active' para recuperarlo.
--
-- Idempotente: se puede ejecutar más de una vez.

BEGIN;

ALTER TABLE core.forums
    ADD COLUMN IF NOT EXISTS orden smallint NOT NULL DEFAULT 100;

DO $$
DECLARE
    fijos text[] := ARRAY['Empresa', 'Patrimonio', 'Familia', 'Otros'];
    i int;
    foro_id uuid;
    conservados uuid[] := '{}';
BEGIN
    FOR i IN 1 .. array_length(fijos, 1) LOOP
        SELECT id INTO foro_id
          FROM core.forums
         WHERE lower(trim(title)) = lower(fijos[i])
         ORDER BY (status = 'deleted'), created_at, id
         LIMIT 1;

        IF foro_id IS NULL THEN
            INSERT INTO core.forums (title, description, status, created_by_admin, orden)
            VALUES (fijos[i], '', 'active', true, i)
            RETURNING id INTO foro_id;
        ELSE
            UPDATE core.forums
               SET title = fijos[i], status = 'active', orden = i, updated_at = NOW()
             WHERE id = foro_id;
        END IF;

        conservados := conservados || foro_id;
    END LOOP;

    UPDATE core.forums
       SET status = 'deleted', updated_at = NOW()
     WHERE status <> 'deleted'
       AND NOT (id = ANY (conservados));
END $$;

COMMIT;

-- Comprobación:
--   SELECT title, status, orden FROM core.forums WHERE status <> 'deleted' ORDER BY orden;
-- debe devolver exactamente Empresa, Patrimonio, Familia y Otros, en ese orden.
