-- 20260806_borrado_de_cuenta.sql
--
-- Marca de cuenta eliminada, para poder ofrecer "eliminar mi cuenta" desde la
-- app.
--
-- Contexto: Apple lo exige desde junio de 2022 (directriz 5.1.1(v)) a toda app
-- que permita registrarse, y Google Play tambien. Sin esto, la app no puede
-- publicarse en ninguna de las dos tiendas. Hoy la app solo ofrece cerrar
-- sesion (profile_screen.dart:170).
--
-- Por que se ANONIMIZA en vez de borrar la fila: catorce tablas referencian
-- core.users con ON DELETE CASCADE —chat, foros, transacciones, encuestas,
-- sinergias, grupos—, asi que un DELETE real se llevaria por delante las
-- conversaciones de OTRAS personas (se perderia la mitad de cada dialogo) y las
-- transacciones de eventos ya cobrados. events.registrations ni siquiera tiene
-- clave foranea: sus filas quedarian huerfanas apuntando a un id inexistente.
--
-- Anonimizar conserva esos registros y elimina a la persona: es lo que pide el
-- RGPD y lo que aceptan ambas tiendas.
--
-- Archivos afectados:
--   internal/core/ports/interfaces.go              AnonymizeUser
--   internal/adapter/storage/postgres/user_repository.go
--   internal/core/services/auth_service.go         DeleteMyAccount
--   internal/handler/http/user_handler.go          DELETE /api/me
--   cmd/server/main.go                             registrar la ruta
--   App-Movil .../profile_screen.dart              opcion de eliminar
--
-- Idempotente: se puede aplicar dos veces sin error.

-- deleted_at distingue una cuenta anonimizada de una activa. Sin esta columna,
-- una cuenta eliminada seria indistinguible de un perfil incompleto, y no
-- habria forma de excluirlas de listados ni de auditar cuando se pidio el
-- borrado.
ALTER TABLE core.users
    ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

COMMENT ON COLUMN core.users.deleted_at IS
    'Fecha en que el usuario pidio eliminar su cuenta. La fila se conserva anonimizada porque catorce tablas dependen de ella en cascada.';

-- Las consultas que listan usuarios activos filtran por esta columna, y son
-- pocas filas frente al total: un indice parcial cuesta casi nada y evita
-- recorrer la tabla entera.
CREATE INDEX IF NOT EXISTS idx_users_deleted_at
    ON core.users (deleted_at)
    WHERE deleted_at IS NOT NULL;
