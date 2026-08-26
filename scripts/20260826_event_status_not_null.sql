-- 20260826_event_status_not_null.sql
--
-- events.events.status pasa a NOT NULL con CHECK sobre los dos valores
-- conocidos.
--
-- Contexto: desde el 2026-08-25 el listado público filtra por
-- `WHERE e.status = 'active'` (internal/adapter/storage/postgres/
-- event_repository.go:66), así que esa columna decide si un evento se ve en la
-- app. La columna existía desde el esquema inicial con DEFAULT 'active', pero
-- **sin NOT NULL y sin CHECK**.
--
-- El error concreto que corrige: un NULL —o cualquier cadena escrita a mano,
-- 'activo', 'Active', ''— no cumple `= 'active'`, de modo que el evento
-- desaparece de la app sin que nadie lo haya ocultado y sin que ninguna
-- pantalla muestre nada raro. El síntoma es un evento que "ya no sale" y una
-- fila que a simple vista parece correcta.
--
-- Archivos afectados en el mismo cambio:
--   internal/core/domain/event.go            (campo Status y EstadoDeEventoValido)
--   internal/core/ports/event_ports.go       (UpdateEventStatus en los dos puertos)
--   internal/adapter/storage/postgres/event_repository.go
--   internal/core/services/event_service.go
--   internal/handler/http/event_handler.go
--   cmd/server/main.go                       (PUT /api/events/{id}/status, AdminOnly)
--
-- Es idempotente: se puede correr dos veces. Aplicar antes de desplegar el
-- binario nuevo no es obligatorio —el código valida por su cuenta—, pero es lo
-- que impide que un UPDATE hecho a mano vuelva a dejar la columna en un estado
-- que nadie atrapa.

BEGIN;

-- 1. Las filas que ya estuvieran en NULL. Se dan por activas: es el valor por
--    defecto de la columna y el comportamiento que tenían antes del filtro,
--    cuando todo evento salía en el listado sin importar su estado.
UPDATE events.events
   SET status = 'active'
 WHERE status IS NULL;

-- 2. Cualquier otro valor fuera de los dos conocidos también se da por activo:
--    hoy no debería haber ninguno, y dejar el evento visible es el lado seguro
--    del error —lo contrario oculta un evento real sin avisar—.
UPDATE events.events
   SET status = 'active'
 WHERE status NOT IN ('active', 'inactive');

ALTER TABLE events.events
    ALTER COLUMN status SET DEFAULT 'active',
    ALTER COLUMN status SET NOT NULL;

-- 3. El CHECK. Se borra antes por si la migración ya se corrió: ADD CONSTRAINT
--    no admite IF NOT EXISTS.
ALTER TABLE events.events
    DROP CONSTRAINT IF EXISTS events_status_check;

ALTER TABLE events.events
    ADD CONSTRAINT events_status_check CHECK (status IN ('active', 'inactive'));

COMMIT;
