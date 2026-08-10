-- 20260810_bloqueo_y_reporte_usuarios.sql
--
-- Bloquear y reportar a otra persona. Requisito de la directriz 1.2 de Apple.
--
-- Contexto: la app tiene chat 1:1 (chat.connections), foros y publicaciones, y
-- hasta hoy no había ninguna forma de que una persona bloqueara a otra. Apple
-- exige, para toda app con contenido generado por usuarios, que se pueda
-- reportar contenido Y bloquear a quien abusa, desde la propia app. Mensajería
-- directa entre desconocidos sin bloqueo es uno de los rechazos más frecuentes.
--
-- Lo que había: reportar publicaciones de foro (core.forum_post_reports). No
-- existía nada para el chat ni para las personas.
--
-- Archivos afectados:
--   internal/core/domain/block.go                        (entidades)
--   internal/core/ports/block_ports.go                   (interfaces)
--   internal/adapter/storage/postgres/block_repository.go
--   internal/adapter/storage/postgres/chat_repository.go (filtrado)
--   internal/core/services/block_service.go
--   internal/handler/http/block_handler.go
--   cmd/server/main.go                                   (rutas)
--
-- Idempotente: se puede aplicar dos veces.

-- Bloqueos. Dirigido: que A bloquee a B no implica lo contrario, pero el
-- filtrado sí es simétrico —ninguno de los dos ve al otro— porque un bloqueo
-- que dejara al bloqueado seguir escribiendo no protegería de nada.
CREATE TABLE IF NOT EXISTS core.user_blocks (
    id         uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    blocker_id uuid NOT NULL,
    blocked_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT user_blocks_pkey PRIMARY KEY (id),
    -- Bloquear dos veces a la misma persona es la misma acción, no dos.
    CONSTRAINT user_blocks_unique UNIQUE (blocker_id, blocked_id),
    -- Bloquearse a uno mismo dejaría a esa cuenta invisible para sí misma.
    CONSTRAINT user_blocks_no_auto CHECK (blocker_id <> blocked_id),
    CONSTRAINT user_blocks_blocker_fkey FOREIGN KEY (blocker_id)
        REFERENCES core.users(id) ON DELETE CASCADE,
    CONSTRAINT user_blocks_blocked_fkey FOREIGN KEY (blocked_id)
        REFERENCES core.users(id) ON DELETE CASCADE
);

-- El filtrado pregunta por las dos direcciones en cada consulta, así que hace
-- falta índice por ambas columnas. blocker_id ya lo cubre el UNIQUE.
CREATE INDEX IF NOT EXISTS idx_user_blocks_blocked ON core.user_blocks (blocked_id);

-- Reportes de personas. Separado de core.forum_post_reports a propósito: aquél
-- reporta una publicación concreta de un foro; este reporta a alguien, y puede
-- venir de un chat, del directorio de miembros o de un perfil.
CREATE TABLE IF NOT EXISTS core.user_reports (
    id          uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    reporter_id uuid NOT NULL,
    reported_id uuid NOT NULL,
    -- Mensaje concreto que motivó el reporte, si lo hubo. Opcional: también se
    -- puede reportar a alguien sin señalar un mensaje.
    message_id  uuid,
    reason      text NOT NULL,
    status      character varying(20) DEFAULT 'pending' NOT NULL,
    created_at  timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT user_reports_pkey PRIMARY KEY (id),
    CONSTRAINT user_reports_no_auto CHECK (reporter_id <> reported_id),
    CONSTRAINT user_reports_reporter_fkey FOREIGN KEY (reporter_id)
        REFERENCES core.users(id) ON DELETE CASCADE,
    CONSTRAINT user_reports_reported_fkey FOREIGN KEY (reported_id)
        REFERENCES core.users(id) ON DELETE CASCADE,
    -- ON DELETE SET NULL y no CASCADE: si el mensaje se borra, el reporte debe
    -- sobrevivir. Borrar la prueba no debería borrar la denuncia.
    CONSTRAINT user_reports_message_fkey FOREIGN KEY (message_id)
        REFERENCES chat.messages(id) ON DELETE SET NULL
);

-- La bandeja del panel administrativo lista lo pendiente, lo más reciente
-- primero.
CREATE INDEX IF NOT EXISTS idx_user_reports_status ON core.user_reports (status, created_at DESC);
