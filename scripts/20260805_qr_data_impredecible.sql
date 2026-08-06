-- 20260805_qr_data_impredecible.sql
--
-- El QR de acceso a un evento dejaba de ser un secreto: se generaba como
--
--     REG-{user_id}-{event_id}          (event_service.go:147)
--
-- es decir, la concatenacion de dos uuid que el propio usuario conoce. Cualquiera
-- que supiera el id de otra persona y el del evento podia fabricar su codigo, y
-- CheckIn lo daria por bueno: GetRegistrationByQR busca por qr_data y no
-- comprueba ni el pago ni el estado de la inscripcion.
--
-- A partir de ahora el valor es aleatorio (uuid v4, generado con crypto/rand en
-- el servicio). Esta migracion regenera los que ya existen y anade el UNIQUE que
-- faltaba: qr_data es la CLAVE DE BUSQUEDA del control de acceso y admitia
-- duplicados, de modo que dos filas con el mismo codigo dejaban el resultado del
-- escaneo a merced del orden que devolviera Postgres.
--
-- Los codigos ya emitidos cambian. No hay riesgo: la imagen del QR no se guarda
-- en ninguna parte, se dibuja en el cliente a partir de este valor cada vez que
-- se abre la pantalla, asi que nadie tiene una copia impresa que vaya a dejar de
-- funcionar.
--
-- Archivos afectados:
--   internal/core/services/event_service.go   genera el valor aleatorio
--
-- Idempotente: se puede aplicar dos veces sin error.

-- 1. Regenerar los predecibles. Se reconocen porque empiezan por 'REG-' seguido
--    del uuid del usuario; los nuevos tambien empiezan por 'REG-' pero con un
--    uuid que no corresponde a nadie, de ahi la comparacion con user_id.
UPDATE events.registrations
   SET qr_data = 'REG-' || gen_random_uuid()::text
 WHERE qr_data IS NULL
    OR qr_data LIKE 'REG-' || user_id::text || '%';

-- 2. Un codigo por inscripcion. Sin esto, el control de acceso busca por una
--    columna que admite repetidos.
ALTER TABLE events.registrations
    DROP CONSTRAINT IF EXISTS registrations_qr_data_key;

ALTER TABLE events.registrations
    ADD CONSTRAINT registrations_qr_data_key UNIQUE (qr_data);
