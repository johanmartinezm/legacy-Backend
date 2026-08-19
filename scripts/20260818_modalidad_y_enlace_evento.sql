-- 20260818_modalidad_y_enlace_evento.sql
--
-- Añade la modalidad del evento y su enlace de acceso.
--
-- Contexto: events.events solo tenía `location`, un texto libre, así que el
-- backend no podía saber si un evento es presencial o virtual. Consecuencia:
-- **se emite un QR de acceso para toda inscripción confirmada**, incluidas las
-- de una masterclass virtual, donde el QR no sirve para nada y lo que hace
-- falta es el enlace de la sesión. Reportado por el cliente el 2026-08-18
-- (punto 2.3 de reports/20260818_plan_ajustes.html).
--
-- Este hueco bloqueaba además el correo de confirmación de la inscripción: no
-- había dónde guardar el enlace que ese correo debe llevar.
--
-- `is_virtual` en vez de un enum de modalidad: hoy solo hay dos casos y son
-- excluyentes. Si algún día aparece el híbrido, un booleano se migra a enum sin
-- perder datos; al revés no.
--
-- DEFAULT false deja los eventos existentes como presenciales, que es lo que
-- son todos hoy: el Legacy Summit y las sesiones. No hay que tocar ninguna fila.
--
-- `access_url` queda NULL en los presenciales. No se valida el formato en la
-- base: lo pone un administrador desde el panel y puede ser un enlace de Zoom,
-- de Meet o de lo que contraten.
--
-- Afecta a:
--   internal/core/domain/event.go
--   internal/adapter/storage/postgres/event_repository.go
--   internal/handler/http/event_handler.go
--   Sitio-Administrativo/.../events (formulario de evento)
--   App-Movil/.../mi_credencial_screen.dart y participando_screen.dart

ALTER TABLE events.events
    ADD COLUMN IF NOT EXISTS is_virtual boolean NOT NULL DEFAULT false;

ALTER TABLE events.events
    ADD COLUMN IF NOT EXISTS access_url text;

COMMENT ON COLUMN events.events.is_virtual IS
    'true = masterclass virtual en vivo; false = evento presencial (Legacy Summit). Decide si la inscripción recibe QR o enlace de acceso.';

COMMENT ON COLUMN events.events.access_url IS
    'Enlace de la sesión para los eventos virtuales. NULL en los presenciales. Solo se entrega a inscripciones confirmadas.';
