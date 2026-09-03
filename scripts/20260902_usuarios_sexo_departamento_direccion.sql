-- 20260902_usuarios_sexo_departamento_direccion.sql
--
-- Tres campos nuevos en core.users para la carga masiva del Legacy Summit 2026:
-- sexo, departamento y dirección. Vienen en el archivo de asistentes y son datos
-- de la persona, no de la inscripción, así que van en la cuenta.
--
-- Plan: reports/20260826_plan_carga_masiva.md §3.1 (decidido el 2026-08-28).
--
-- Cifrado, y por qué no es igual para los tres:
--   sexo y direccion  -> se guardan CIFRADOS por el servicio, como location y
--                        phone. Por eso son text: el cifrado es una cadena
--                        base64, no cabe en un tipo más estrecho ni admite
--                        CHECK de valores.
--   departamento      -> en claro, como country.
-- Consecuencia conocida: no se puede filtrar ni ordenar por sexo ni por
-- direccion en SQL. Hoy nadie lo necesita.
--
-- Las tres son anulables y sin DEFAULT: las cuentas que ya existen se quedan en
-- NULL, que es «no lo sabemos», y el repositorio ya las lee con COALESCE.
--
-- Archivos que dependen de esta migración:
--   internal/core/domain/user.go
--   internal/adapter/storage/postgres/user_repository.go
--   internal/core/services/auth_service.go (cifra sexo y direccion)
--   App-Movil/lib/presentation/screens/profile/profile_edit_screen.dart
--
-- Es idempotente.

ALTER TABLE core.users
    ADD COLUMN IF NOT EXISTS sexo         text,
    ADD COLUMN IF NOT EXISTS departamento text,
    ADD COLUMN IF NOT EXISTS direccion    text;
