-- 20260813_contacto_mensajes.sql
--
-- Guarda los mensajes de la pantalla "Contáctenos".
--
-- Contexto: la pantalla se añadió hoy y enviaba el mensaje por correo sin
-- guardarlo, igual que los otros dos canales (asesorías y contacto con la
-- junta). Eso deja tres agujeros: si el SMTP falla el mensaje se pierde del
-- todo, nadie puede ver en el panel qué se ha preguntado ni si quedó sin
-- responder, y no hay forma de frenar a quien escriba en bucle porque no se
-- sabe cuántos mensajes lleva.
--
-- El mensaje se guarda ANTES de intentar el correo, que es lo que hace que un
-- fallo de SMTP deje de perder datos.
--
-- El asunto y el cuerpo van CIFRADOS con security.CryptoService, el mismo
-- tratamiento que los mensajes de chat: es texto libre escrito por una persona
-- y puede contener cualquier cosa. Quien lea esta tabla por fuera del servicio
-- verá base64, no el mensaje.
--
-- El remitente NO se copia aquí: se guarda user_id y el nombre y el correo se
-- leen de core.users, donde ya están cifrados. Duplicarlos sería tener el mismo
-- dato personal en dos sitios y que uno de los dos quedara viejo.
--
-- Archivos afectados:
--   internal/core/domain/contacto.go                        (entidad)
--   internal/core/ports/contacto_ports.go                   (interfaces)
--   internal/adapter/storage/postgres/contacto_repository.go
--   internal/core/services/contacto_service.go
--   internal/handler/http/contacto_handler.go
--   cmd/server/main.go                                      (rutas)
--   Sitio-Administrativo: features/admin/contacto           (bandeja)
--
-- Idempotente: se puede aplicar dos veces.

CREATE TABLE IF NOT EXISTS core.contact_messages (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       uuid NOT NULL REFERENCES core.users(id) ON DELETE CASCADE,
    subject       text NOT NULL,
    body          text NOT NULL,
    -- nuevo -> leido -> respondido. Sin ENUM a propósito: añadir un estado a un
    -- tipo enumerado obliga a un ALTER TYPE que no es idempotente.
    status        varchar(20) NOT NULL DEFAULT 'nuevo',
    -- Si el correo no salió, el mensaje sigue aquí y el panel lo destaca: es la
    -- única señal de que alguien escribió y nadie recibió el aviso.
    email_enviado boolean NOT NULL DEFAULT false,
    created_at    timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- La bandeja se ordena por fecha y se filtra por estado.
CREATE INDEX IF NOT EXISTS idx_contact_messages_status_created
    ON core.contact_messages (status, created_at DESC);

-- Para contar en un rato lo que lleva escrito una persona (límite de frecuencia).
CREATE INDEX IF NOT EXISTS idx_contact_messages_user_created
    ON core.contact_messages (user_id, created_at DESC);

COMMENT ON TABLE core.contact_messages IS
    'Mensajes de la pantalla Contactenos. subject y body van cifrados (AES-256): leerlos por fuera del servicio devuelve base64.';
