-- 20260902_usuarios_debe_cambiar_contrasena.sql
--
-- Bandera que obliga a cambiar la contraseña en el primer ingreso. La pone en
-- true la carga masiva, porque a esas cuentas se les asigna como contraseña su
-- número de documento, que no es un secreto.
--
-- Plan: reports/20260826_plan_carga_masiva.md §2.5 (decidido el 2026-08-28).
--
-- DEFAULT false a propósito: ninguna de las cuentas que ya existen queda
-- obligada a nada. NOT NULL para que la app no tenga que distinguir entre
-- «false» y «sin dato».
--
-- Dos reglas que viven en el código y conviene no perder de vista:
--   - Solo ChangePassword la baja (user_repository.go, al guardar la contraseña
--     nueva). El UPDATE general de usuarios NO la toca, así que nadie puede
--     quitársela desde el PUT del perfil.
--   - No viaja en la respuesta del login, que es un map[string]string con solo
--     el token: sale por GET /api/me, dentro del usuario.
--
-- Es idempotente.

ALTER TABLE core.users
    ADD COLUMN IF NOT EXISTS debe_cambiar_contrasena boolean NOT NULL DEFAULT false;
