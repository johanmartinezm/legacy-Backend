-- 20261012_add_event_model.sql
--
-- Añade el campo opcional «Modelo» a events.events.
--
-- Contexto: la ficha del detalle de un evento en la app
-- (App-Movil/lib/presentation/screens/eventos/event_purchase_detail_screen.dart)
-- mostraba «Modelo · 7 dimensiones DINASTÍA» como texto fijo, primero en todos
-- los eventos y desde el 2026-10-12 solo en los de categoría summit. No había
-- forma de cambiarlo sin publicar una versión nueva de la app.
--
-- Archivos afectados: internal/core/domain/event.go (Event.Model, JSON "model"),
-- internal/adapter/storage/postgres/event_repository.go (GetEvents,
-- GetEventByID, CreateEvent, UpdateEvent), el formulario de eventos del panel y
-- la ficha de la app, que solo pinta la fila si el campo trae texto.
--
-- ⚠ UpdateEvent escribe la columna con lo que mande el panel: el backend y el
-- panel con el campo nuevo tienen que desplegarse juntos, o cada edición desde
-- un panel viejo dejaría el modelo en NULL.

ALTER TABLE events.events
    ADD COLUMN IF NOT EXISTS model text;

-- Backfill: los eventos de categoría summit conservan lo que la app ya les
-- mostraba. Solo donde todavía no hay valor, para poder repetir el script.
UPDATE events.events e
   SET model = '7 dimensiones DINASTÍA'
  FROM events.categories c
 WHERE e.category_id = c.id
   AND lower(c.name) = 'summit'
   AND e.model IS NULL;
