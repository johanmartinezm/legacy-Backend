-- 20261021_event_reminders.sql
--
-- Registra qué eventos ya tuvieron su aviso push de la mañana del evento.
--
-- Contexto: hasta hoy ninguna notificación avisaba de que un evento empezaba;
-- solo salía «Nuevo evento» al crearlo. Por decisión del cliente (2026-10-21)
-- el recordatorio va a los inscritos confirmados, una vez, desde las 8:00 de
-- Bogotá del día del evento. Lo envía core/services/recordatorio_eventos.go,
-- que revisa cada 10 minutos.
--
-- Archivos afectados: internal/adapter/storage/postgres/recordatorio_evento_repository.go,
-- internal/core/services/recordatorio_eventos.go, cmd/server/main.go.
--
-- Por qué una tabla: sin ella, cada vuelta y cada reinicio del backend volvería
-- a avisar. La clave incluye la fecha y no solo el evento para que, si el
-- evento se mueve a otro día después del aviso, se avise también el día nuevo.
--
-- Sin esta tabla el backend arranca igual, pero el recordatorio falla en cada
-- vuelta con: relation "events.event_reminders" does not exist (SQLSTATE 42P01).

CREATE TABLE IF NOT EXISTS events.event_reminders (
    event_id uuid        NOT NULL REFERENCES events.events(id) ON DELETE CASCADE,
    fecha    date        NOT NULL,
    sent_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (event_id, fecha)
);
