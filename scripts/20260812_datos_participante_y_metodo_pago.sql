-- 20260812_datos_participante_y_metodo_pago.sql
--
-- Contexto: la pantalla de pago de un evento pide "Datos del Participante"
-- —nombre, correo y teléfono— y los valida desde el 2026-08-05, pero **no
-- tenían dónde ir**: ni POST /api/events/{id}/register ni POST
-- /api/payments/intent los aceptaban, así que se validaban y se tiraban. El
-- organizador del evento no tiene a quién llamar si alguien no aparece.
--
-- Esa misma pantalla deja elegir entre tarjeta y PSE, y esa elección tampoco
-- viajaba a ninguna parte.
--
-- Archivos afectados:
--   internal/core/domain/event.go            campos del participante
--   internal/handler/http/event_handler.go   los acepta al inscribir
--   internal/core/services/event_service.go  los cifra al escribir
--   internal/adapter/storage/postgres/event_repository.go
--   internal/core/services/payment_service.go   guarda el método elegido
--   App-Movil  event_payment_screen.dart, event_service.dart, payment_service.dart
--
-- Qué corrige: un formulario que pedía datos y los descartaba, y un selector de
-- método de pago que no hacía absolutamente nada.
--
-- Idempotente: se puede aplicar varias veces sin efecto.

BEGIN;

-- Contacto de quien asiste, para ese evento concreto.
--
-- **Van cifrados**, como el resto de datos personales del proyecto: los escribe
-- y los lee event_service a través de CryptoService. Una consulta directa a
-- estas columnas devuelve texto cifrado, no el nombre.
--
-- No sustituyen a user_id ni permiten inscribir a otra persona: la entrada
-- sigue siendo de quien la compra. Son el contacto para ese evento, que puede
-- diferir del perfil —un correo de trabajo, otro teléfono—.
ALTER TABLE events.registrations
    ADD COLUMN IF NOT EXISTS participant_name  text,
    ADD COLUMN IF NOT EXISTS participant_email text,
    ADD COLUMN IF NOT EXISTS participant_phone text;

COMMENT ON COLUMN events.registrations.participant_name IS
    'Nombre de contacto para este evento, CIFRADO. Vacío = usar el del perfil.';
COMMENT ON COLUMN events.registrations.participant_email IS
    'Correo de contacto para este evento, CIFRADO.';
COMMENT ON COLUMN events.registrations.participant_phone IS
    'Teléfono de contacto para este evento, CIFRADO.';

-- Método elegido en la app: 'credit_card' o 'pse'.
--
-- **Es informativo.** La pasarela muestra sus propios medios de pago y decide
-- ella; esto solo deja constancia de lo que el usuario esperaba, que sirve para
-- soporte —"elegí PSE y me salió tarjeta"— y para saber si PSE se usa lo
-- suficiente como para justificar integrarlo de verdad.
ALTER TABLE core.transactions
    ADD COLUMN IF NOT EXISTS payment_method character varying(20);

COMMENT ON COLUMN core.transactions.payment_method IS
    'Medio de pago elegido en la app (credit_card | pse). Informativo: lo que decide es la pasarela.';

COMMIT;
