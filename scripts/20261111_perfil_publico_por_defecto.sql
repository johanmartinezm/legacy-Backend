-- 20261111_perfil_publico_por_defecto.sql
--
-- Enciende el perfil público de las cuentas activas que lo tenían apagado sin
-- haberlo elegido, antes de que el directorio empiece a respetar ese campo.
--
-- Contexto: core.users.is_public_profile existía pero nada lo leía: el
-- directorio (ListMembers en internal/adapter/storage/postgres/chat_repository.go)
-- mostraba a todo el mundo, y la app no tenía interruptor para cambiarlo. Desde
-- esta fecha el directorio solo muestra a quien lo tiene encendido y la app lo
-- deja apagar en editar perfil.
--
-- Sin esta migración, aplicar el filtro escondería de golpe a cinco cuentas que
-- nadie apagó a propósito. Consultado en producción el 2026-11-11 (36 activas):
-- cuatro creadas entre febrero y julio, sin términos aceptados y con los tres
-- ajustes de privacidad en false —el valor que dejaba el código de entonces— y
-- la cuenta de revisión de Apple.
--
-- La de Apple (reports/20260812_cuenta_demo_apple.md) se deja apagada a
-- propósito: es una cuenta de prueba y no debe salir en el directorio.
--
-- Las cuentas eliminadas tampoco se tocan: la anonimización las deja en false
-- y el directorio ya las excluye por deleted_at.
--
-- Archivos del mismo cambio: chat_repository.go (filtro), importacion_service.go
-- (la carga masiva fija el valor en vez de dejar el cero de Go) y, en la app,
-- profile_edit_screen.dart (interruptor).
--
-- Tampoco se tocan las cuentas de la carga masiva que aún no han entrado
-- (debe_cambiar_contrasena): nacen ocultas a propósito
-- (PerfilPublicoDeCuentaImportada) mientras el cliente decide.
--
-- 🔴 Aplicada en producción el 2026-11-11 SIN esa condición: encendió 157
-- cuentas en vez de 4, porque esa mañana habían entrado 153 asistentes por la
-- carga masiva. Se corrigió a mano minutos después, antes de desplegar el
-- filtro, devolviendo a false esas 153 (las que tenían debe_cambiar_contrasena,
-- creadas ese día y con el updated_at de la migración). Respaldo previo:
-- backup_20261111_1108_pre_perfil_publico.sql.gz en el directorio de despliegue.
--
-- Idempotente: volver a aplicarla no cambia nada.

UPDATE core.users
   SET is_public_profile = true,
       updated_at = now()
 WHERE deleted_at IS NULL
   AND is_public_profile IS NOT TRUE
   AND debe_cambiar_contrasena IS NOT TRUE
   AND id <> '4ac874cb-bfae-4c06-a1ab-d2eed855e061';
