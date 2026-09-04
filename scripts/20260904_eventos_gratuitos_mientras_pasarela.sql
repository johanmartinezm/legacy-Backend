-- 20260904_eventos_gratuitos_mientras_pasarela.sql
--
-- Deja todos los eventos como gratuitos en la app, y corrige el precio del
-- Legacy Summit 2026.
--
-- Contexto: la pasarela de CredibanCo devuelve errorCode 5 ("Acceso denegado")
-- desde el 2026-08-06 y sigue sin respuesta. El 2026-08-28 se decidio que los
-- eventos con pago quedaran inactivos mientras tanto, pero esa decision nunca
-- se aplico al Summit, que quedo en produccion con is_free = false, activo y
-- visible.
--
-- Por que importa ahora: en el detalle de un evento la app decide el boton por
-- is_free (event_action_button.dart:197). Con is_free = false abre
-- EventPaymentScreen, que crea la inscripcion como pendiente y **lanza la
-- pasarela** en el navegador externo. Es decir, hoy cualquiera que toque el
-- Summit termina con una inscripcion sin pagar y un error de CredibanCo.
--
-- Y contradice lo que se le declaro a Apple en el envio del 2026-09-04:
-- "the app does not sell digital content and does not process any payments in
-- this version". Un revisor que toque ese boton ve lo contrario.
--
-- El precio NO se borra: se guarda el valor correcto para cuando la pasarela
-- vuelva. El Summit estaba en 3150000 —escrito en pesos— y son 499 dolares,
-- confirmado por el cliente el 2026-09-04. Mientras is_free = true, la app
-- muestra "GRATIS" y no lee el precio (event_model.dart:125).
--
-- Archivos relacionados:
--   Sitio-Administrativo/src/app/features/admin/event-form-dialog/ — hasta hoy
--   guardaba `isFree: price === 0`, asi que reabrir y guardar el Summit lo
--   habria vuelto de pago en silencio. Ya es una casilla propia.
--
-- COMO REVERTIR, cuando CredibanCo responda: devolver el DEFAULT a false y
-- desmarcar "Evento gratuito" en el panel evento por evento —no en bloque, que
-- los gratuitos de verdad deben seguir gratuitos—.

-- 1. El precio real del Summit, en dolares.
UPDATE events.events
   SET price = 499,
       updated_at = CURRENT_TIMESTAMP
 WHERE id = '92a7513e-3d13-42e5-bc2d-21cf560a5bc1';

-- 2. Todo evento existente pasa a gratuito en la app.
UPDATE events.events
   SET is_free = true,
       updated_at = CURRENT_TIMESTAMP
 WHERE is_free IS DISTINCT FROM true;

-- 3. Y los nuevos nacen gratuitos. El panel ya lo manda explicitamente; esto
--    cubre a quien cree un evento por API sin el campo.
ALTER TABLE events.events
    ALTER COLUMN is_free SET DEFAULT true;

-- Comprobacion.
SELECT title, price, is_free, status, start_date
  FROM events.events
 ORDER BY start_date;
