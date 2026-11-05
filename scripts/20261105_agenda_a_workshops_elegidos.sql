-- 20261105_agenda_a_workshops_elegidos.sql
--
-- Copia «Mi agenda» (events.user_agenda) a la elección de workshops de cada
-- inscripción (events.registration_workshops).
--
-- Contexto: §10 del plan reports/20261002_plan_material_talleres.md (raíz del
-- monorepo, no versionado). Hasta ahora había dos listas sin conexión:
--   - user_agenda, que la app llenaba con «Añadir a mi agenda» sin ninguna
--     regla (sin exigir inscripción ni fecha), y que la puerta no veía;
--   - registration_workshops, la que muestra el escáner de la puerta, que la
--     app nunca llenaba.
-- Desde este cambio GET /api/events/agenda lee de registration_workshops
-- (event_repository.go, GetAgenda) y la elección se hace al inscribirse
-- (workshops_eleccion.go). Sin esta copia, quien ya tenía workshops en su agenda
-- vería la agenda vacía.
--
-- Solo se copia cuando la persona está inscrita a ese evento: una entrada de
-- agenda sin inscripción no tiene a qué inscripción pegarse.
--
-- Es aditiva e idempotente (§8 del plan): ON CONFLICT DO NOTHING sobre la clave
-- (registration_id, workshop_id), sin DELETE ni UPDATE, y no toca
-- events.workshops. user_agenda se conserva tal cual, sin uso. Se puede correr
-- dos veces sin efecto.
--
-- No comprueba la regla de uno por franja: copia lo que la persona marcó. Si
-- quedan dos de la misma franja, la app lo mostrará y podrá corregirlo hasta el
-- día anterior al evento.

BEGIN;

INSERT INTO events.registration_workshops (registration_id, workshop_id)
SELECT reg.id, ua.workshop_id
  FROM events.user_agenda ua
  JOIN events.workshops w      ON w.id = ua.workshop_id
  JOIN events.registrations reg ON reg.user_id = ua.user_id
                               AND reg.event_id = w.event_id
ON CONFLICT DO NOTHING;

COMMIT;
