-- 20261105_pago_por_link_y_rechazo.sql
--
-- Eventos de pago con link externo y aprobación manual (parte B, §7 del plan
-- reports/20261002_plan_material_talleres.md, en la raíz del monorepo, no
-- versionado).
--
-- Contexto: CredibanCo sigue bloqueado (errorCode 5), así que el pago pasa a un
-- link externo por evento y el administrador aprueba o rechaza cada inscripción
-- pendiente desde el panel (event_handler.go: AprobarInscripcion y
-- RechazarInscripcion).
--
--   1. events.events.payment_url: el link de pago. El servicio lo exige, https,
--      en todo evento que no sea gratuito (validarLinkDePago en
--      event_service.go). Puede quedar NULL en los gratuitos y en los de pago
--      que ya existían: el panel lo pedirá la próxima vez que se guarden.
--   2. El CHECK de events.registrations.registration_status admite 'rejected'.
--      Hasta ahora solo admitía 'confirmed' y 'pending_payment', y rechazar
--      habría fallado con: new row for relation "registrations" violates check
--      constraint "registrations_registration_status_check" (SQLSTATE 23514).
--
-- Aditiva e idempotente (§8 del plan): ADD COLUMN IF NOT EXISTS, y el CHECK se
-- reemplaza por uno que admite lo mismo que antes más 'rejected', así que
-- ninguna fila existente puede incumplirlo. Sin DELETE ni UPDATE de datos.
-- Se puede correr dos veces sin efecto.

BEGIN;

ALTER TABLE events.events
    ADD COLUMN IF NOT EXISTS payment_url text;

ALTER TABLE events.registrations
    DROP CONSTRAINT IF EXISTS registrations_registration_status_check;
ALTER TABLE events.registrations
    ADD CONSTRAINT registrations_registration_status_check
    CHECK (registration_status IN ('confirmed', 'pending_payment', 'rejected'));

COMMIT;
