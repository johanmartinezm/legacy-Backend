-- 20260819_attendance_logs_una_por_inscripcion.sql
--
-- Una inscripción, una entrada: events.attendance_logs deja de admitir filas
-- repetidas para la misma inscripción.
--
-- Contexto: internal/adapter/storage/postgres/event_repository.go
-- (RecordAttendance) insertaba en events.attendance_logs sin comprobar si esa
-- inscripción ya había entrado. Pasar dos veces el mismo QR por el escáner del
-- panel dejaba DOS filas, y las dos respuestas de POST /api/events/check-in
-- eran idénticas: quien está en la puerta no tenía forma de saber que ese
-- código ya se había usado. Encontrado al ejecutar F12.8 del plan de pruebas
-- (reports/20260818_plan_pruebas.html) el 2026-08-19.
--
-- Efecto colateral del defecto: el recuento de asistentes de un evento sube una
-- unidad por cada relectura, así que las cifras del panel estaban infladas en
-- los eventos donde alguien escaneó dos veces.
--
-- Archivos que acompañan a esta migración:
--   internal/adapter/storage/postgres/event_repository.go  (ON CONFLICT DO NOTHING)
--   internal/core/ports/event_ports.go                     (firma de RecordAttendance)
--   internal/core/services/event_service.go                (marca la relectura)
--   internal/core/domain/event.go                          (alreadyCheckedIn)
--   Sitio-Administrativo/.../attendance-scanner            (lo muestra en pantalla)
--
-- ORDEN IMPORTANTE: primero se limpian los duplicados que ya existan y después
-- se crea la restricción. Al revés, la restricción no se puede crear en
-- cualquier base donde alguien haya escaneado dos veces.

-- 1. De cada grupo de repetidas se conserva la PRIMERA entrada —la que de
--    verdad ocurrió— y se descartan las relecturas posteriores.
DELETE FROM events.attendance_logs a
      USING events.attendance_logs b
      WHERE a.registration_id = b.registration_id
        AND (b.check_in_time, b.id) < (a.check_in_time, a.id);

-- 2. A partir de aquí la base lo impide por sí misma, sin depender de que el
--    código se acuerde de comprobarlo.
ALTER TABLE events.attendance_logs
      DROP CONSTRAINT IF EXISTS attendance_logs_registration_id_key;

ALTER TABLE events.attendance_logs
      ADD CONSTRAINT attendance_logs_registration_id_key UNIQUE (registration_id);
