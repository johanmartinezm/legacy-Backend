-- 20260805_registration_status_pendiente_pago.sql
--
-- Da uso real a events.registrations.registration_status, que hasta hoy era una
-- columna muerta: existe en la tabla con DEFAULT 'confirmed' y no aparece en
-- NINGÚN archivo Go (ni en domain.Registration, ni en las consultas del
-- repositorio), así que todas las inscripciones valían 'confirmed' pasara lo
-- que pasara.
--
-- A partir de ahora distingue las dos situaciones que pidió el cliente:
--
--   confirmed        el evento es gratuito, o el de pago ya está pagado
--   pending_payment  el evento es de pago y todavía no se ha pagado
--
-- Contexto: hasta hoy, en un evento de pago la app iba directa a la pasarela
-- (event_purchase_detail_screen.dart:263 bifurca en isFree) y NUNCA creaba la
-- inscripción, así que no había ni rastro de quién había intentado comprar. En
-- los gratuitos sí se creaba, con payment_status='free'.
--
-- Archivos afectados:
--   internal/core/domain/event.go                        RegistrationStatus y sus constantes
--   internal/core/ports/event_ports.go                   ConfirmEventRegistration
--   internal/adapter/storage/postgres/event_repository.go INSERT/SELECT y el UPDATE de confirmación
--   internal/core/services/event_service.go              RegisterUser fija el estado según is_free
--   internal/core/services/payment_service.go            confirma la inscripción al aprobarse el pago
--   cmd/server/main.go                                   cablea eventRepo en el servicio de pagos
--   App-Movil .../event_payment_screen.dart              crea la inscripción antes de ir a la pasarela
--
-- Idempotente: se puede aplicar dos veces sin error.

-- 1. Alinear los datos existentes ANTES de poner la restricción, o el ALTER
--    falla con las filas que no cumplan. Hoy en producción hay una sola
--    inscripción, en 'confirmed' y 'paid', así que no cambia nada; el UPDATE
--    está por si se aplica sobre una base con más historial.
UPDATE events.registrations
   SET registration_status = CASE
         WHEN payment_status IN ('free', 'paid') THEN 'confirmed'
         ELSE 'pending_payment'
       END
 WHERE registration_status IS NULL
    OR registration_status NOT IN ('confirmed', 'pending_payment');

-- 2. Ningún valor nulo: una inscripción sin estado no significa nada.
ALTER TABLE events.registrations
    ALTER COLUMN registration_status SET DEFAULT 'confirmed';

UPDATE events.registrations
   SET registration_status = 'confirmed'
 WHERE registration_status IS NULL;

ALTER TABLE events.registrations
    ALTER COLUMN registration_status SET NOT NULL;

-- 3. Restringir a los dos valores válidos. Sin esto la columna es un
--    varchar(20) libre donde cabe 'PAID', 'pagado' o cualquier errata, que es
--    justo lo que hace hoy inservibles a payment_status y registration_status.
ALTER TABLE events.registrations
    DROP CONSTRAINT IF EXISTS registrations_registration_status_check;

ALTER TABLE events.registrations
    ADD CONSTRAINT registrations_registration_status_check
    CHECK (registration_status IN ('confirmed', 'pending_payment'));

-- 4. El panel y la app preguntan "qué inscripciones de este evento están
--    pendientes de pago"; sin índice es un recorrido completo de la tabla.
CREATE INDEX IF NOT EXISTS idx_registrations_event_status
    ON events.registrations (event_id, registration_status);
