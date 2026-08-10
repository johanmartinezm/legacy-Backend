-- 20260810_alias_vacio_a_null.sql
--
-- Normaliza core.users.alias: la cadena vacía pasa a NULL.
--
-- Contexto: users_alias_key es un índice UNIQUE sobre alias. En Postgres dos
-- NULL no colisionan, pero dos cadenas vacías SÍ. Como RegisterRequest no tenía
-- campo alias (internal/handler/http/user_handler.go), todo registro insertaba
-- alias = '', de modo que la SEGUNDA cuenta sin alias de cada base violaba la
-- restricción y el registro fallaba con:
--
--   ERROR: duplicate key value violates unique constraint "users_alias_key" (SQLSTATE 23505)
--
-- El repositorio además traducía cualquier 23505 a "alias_in_use", así que el
-- mensaje mandaba a cambiar un alias que nadie había escrito.
--
-- Arreglado en código: el registro ya acepta alias, y el INSERT lo guarda con
-- NULLIF($28, '') para que "sin alias" sea NULL y no cadena vacía. Esta
-- migración limpia lo que quedó de antes: mientras exista una fila con '', la
-- primera cuenta nueva que llegue sin alias volverá a chocar con ella.
--
-- El índice parcial idx_users_alias (WHERE alias IS NOT NULL) ya asumía NULL
-- como representación de "sin alias"; esto alinea los datos con ese supuesto.
--
-- Archivos afectados:
--   internal/handler/http/user_handler.go                (campo alias en el request)
--   internal/adapter/storage/postgres/user_repository.go (NULLIF y errores por restricción)
--
-- Idempotente: la segunda pasada no encuentra filas que cambiar.

UPDATE core.users
   SET alias = NULL
 WHERE alias = '';
