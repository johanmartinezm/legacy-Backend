-- 20261013_usuario_plus.sql
--
-- Añade core.users.es_plus: si el usuario es Legacy+.
--
-- Contexto: el cliente pidió el 2026-10-13 que las tres filas de la sección
-- Legacy+ del Inicio de la app (Chats de sinergia, Legacy Board, Beneficios)
-- solo se abran a usuarios Legacy+, y que eso se valide de verdad. Hasta hoy no
-- existía ningún campo así: lo más parecido era customer_status, que elige la
-- propia persona y se podía cambiar ella misma por la API. El plan está en
-- reports/20261013_plan_usuario_plus.md (repositorio raíz).
--
-- Solo el panel cambia esta columna (PUT /api/admin/users/{id}/plus). El UPDATE
-- general del usuario no la escribe, y PUT /api/me la conserva.
--
-- Archivos que dependen de ella:
--   internal/core/domain/user.go (EsPlus)
--   internal/adapter/storage/postgres/user_repository.go (FindAll, FindByID, SetEsPlus, EsPlus)
--   internal/core/services/usuario_plus_service.go
--   internal/handler/http/usuario_plus.go (RequierePlus y el endpoint del panel)
--   cmd/server/main.go (foros y beneficios detrás de RequierePlus)
--
-- Nadie es plus al aplicarla: se marcan a mano en el panel (decisión del
-- cliente). Idempotente: ADD COLUMN IF NOT EXISTS.

ALTER TABLE core.users
    ADD COLUMN IF NOT EXISTS es_plus boolean NOT NULL DEFAULT false;
