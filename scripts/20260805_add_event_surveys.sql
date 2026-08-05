-- 20260805_add_event_surveys.sql
--
-- Crea events.event_surveys: la encuesta general de un evento, una respuesta
-- por usuario y evento.
--
-- Contexto: hasta hoy lo único que podía calificar un usuario era una charla
-- suelta, en events.workshop_ratings. GetEventFeedback
-- (internal/adapter/storage/postgres/event_repository.go:238) no es lo que su
-- nombre sugiere: hace JOIN de workshop_ratings con workshops filtrando por
-- event_id, es decir, agrega las calificaciones DE LAS CHARLAS de ese evento, y
-- es de solo lectura para el panel. No existía ninguna vía por la que el usuario
-- opinara del evento completo, que es lo que pide el módulo de eventos en
-- docs/Grandes Grupos Funcionales V1.0.xlsx.
--
-- Archivos afectados (fase 3 del plan de eventos):
--   internal/core/domain/event.go                        EventSurvey, EventSurveySummary
--   internal/core/ports/event_ports.go                   métodos de repositorio y servicio
--   internal/adapter/storage/postgres/event_repository.go
--   internal/core/services/event_service.go              validación y regla de acceso
--   internal/handler/http/event_handler.go
--   cmd/server/main.go                                   POST /api/events/{id}/survey
--                                                        GET  /api/events/{id}/survey/me
--                                                        GET  /api/events/{id}/survey/summary
--
-- Idempotente: se puede aplicar dos veces sin error.

CREATE TABLE IF NOT EXISTS events.event_surveys (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id            uuid NOT NULL REFERENCES events.events(id) ON DELETE CASCADE,
    user_id             uuid NOT NULL REFERENCES core.users(id) ON DELETE CASCADE,
    overall_rating      integer NOT NULL,
    organization_rating integer,
    content_rating      integer,
    speakers_rating     integer,
    would_recommend     boolean,
    comment             text,
    created_at          timestamp with time zone DEFAULT CURRENT_TIMESTAMP,

    -- Una sola respuesta por usuario y evento. events.workshop_ratings NO tiene
    -- esta restricción: allí un doble toque en el botón deja dos filas y sesga el
    -- promedio. Aquí es deliberado, no una copia del precedente.
    CONSTRAINT event_surveys_event_user_key UNIQUE (event_id, user_id),

    CONSTRAINT event_surveys_overall_rating_check
        CHECK (overall_rating >= 1 AND overall_rating <= 5),
    CONSTRAINT event_surveys_organization_rating_check
        CHECK (organization_rating IS NULL OR (organization_rating >= 1 AND organization_rating <= 5)),
    CONSTRAINT event_surveys_content_rating_check
        CHECK (content_rating IS NULL OR (content_rating >= 1 AND content_rating <= 5)),
    CONSTRAINT event_surveys_speakers_rating_check
        CHECK (speakers_rating IS NULL OR (speakers_rating >= 1 AND speakers_rating <= 5))
);

-- El resumen del panel agrupa por evento; la consulta del usuario busca por
-- (event_id, user_id), que ya cubre el índice único de arriba.
CREATE INDEX IF NOT EXISTS idx_event_surveys_event_id
    ON events.event_surveys (event_id);
