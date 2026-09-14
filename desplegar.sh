#!/bin/bash
#
# Despliegue del backend a producción, en el orden que exige DESPLIEGUE.md.
#
# Se escribió el 2026-09-14 para el arreglo de Sign in with Apple, donde el
# orden importa: la migración repara las cuentas ya guardadas mal y tiene que
# correr ANTES que el binario nuevo. Hacerlo al revés deja esas cuentas rotas
# hasta que alguien se acuerde del SQL.
#
# No inventa nada: son los pasos 1-5 de DESPLIEGUE.md con sus comprobaciones,
# incluidas las tres trampas que esa guía documenta —el config que se
# desincroniza, el docker-compose.yml que NO se sube y el --build que se olvida.
#
#   ./desplegar.sh              simulacro: comprueba y no cambia nada
#   ./desplegar.sh -aplicar     despliega
#
set -uo pipefail

APLICAR=0
[ "${1:-}" = "-aplicar" ] && APLICAR=1

MIGRACION="scripts/20260914_reparar_identidad_social.sql"

cd "$(dirname "$0")" || exit 1
set -a; source .env; set +a
SSH="ssh -o BatchMode=yes -o ConnectTimeout=20 $SSH_USER@$SERVER_IP"

paso()  { echo; echo "=== $* ==="; }
morir() { echo "ABORTA: $*" >&2; exit 1; }

# ---------------------------------------------------------------- 0. acceso
paso "0. Acceso al servidor"
$SSH 'echo conectado' >/dev/null 2>&1 || morir "no hay SSH al servidor (puerto 22)"
echo "SSH: ok"
curl -s -o /dev/null -w "health antes: %{http_code}\n" --max-time 20 \
    https://legacy.intelyclick.com/health

# ------------------------------------------------------- 1. binario al día
paso "1. Binario"
[ -f server_linux ] || morir "falta server_linux: corre ./build-linux.sh"
if [ -n "$(find . -name '*.go' -newer server_linux -not -path './tmp/*' 2>/dev/null | head -1)" ]; then
    morir "hay .go más nuevos que server_linux: vuelve a correr ./build-linux.sh"
fi
echo "server_linux: $(sha256sum server_linux | cut -c1-16)…  $(stat -c%s server_linux) bytes"

# --------------------------------------------- 2. el config no se pisa solo
#
# La copia del servidor es la buena: es la que está corriendo. Ya pasó una vez
# que apple.bundle_id existía solo allí y el scp lo habría borrado.
paso "2. config.docker.yaml — local contra servidor"
LOCAL=$(sha256sum config.docker.yaml | cut -d' ' -f1)
REMOTO=$($SSH "sha256sum $DEPLOY_DIR/config.docker.yaml 2>/dev/null | cut -d' ' -f1")
if [ "$LOCAL" != "$REMOTO" ]; then
    echo "local:    ${LOCAL:0:16}…"
    echo "servidor: ${REMOTO:0:16}…"
    morir "difieren. Baja el del servidor, aplica tus cambios encima y repite. NO subas el local a ciegas."
fi
echo "idénticos: ${LOCAL:0:16}…"

# apple.bundle_id tiene que estar, o Sign in with Apple rechaza a todo el mundo.
$SSH "grep -q 'bundle_id' $DEPLOY_DIR/config.docker.yaml" \
    || morir "el config del servidor no declara apple.bundle_id"
echo "apple.bundle_id: presente"

# ------------------------------------------------------------- 3. respaldo
paso "3. Respaldo de la base"
if [ "$APLICAR" = 1 ]; then
    RESPALDO="backup_$(date +%Y%m%d_%H%M)_pre_identidad_social.sql.gz"
    $SSH "cd $DEPLOY_DIR && docker exec legacy_db pg_dump -U dba applegacy | gzip > $RESPALDO && gunzip -t $RESPALDO && ls -lh $RESPALDO" \
        || morir "el respaldo no se creó o el .gz está corrupto"
    echo "respaldo verificado: $RESPALDO"
else
    echo "(simulacro: no se toma respaldo)"
fi

# ---------------------------------------- 4. migración ANTES que el binario
paso "4. Migración $MIGRACION"
[ -f "$MIGRACION" ] || morir "falta $MIGRACION"

ROTAS=$($SSH "docker exec legacy_db psql -U dba -d applegacy -tAc \"
  select count(*) from core.users
   where (apple_id like '%.%.%' and length(apple_id) > 100)
      or (google_id like '%.%.%' and length(google_id) > 100);\"")
echo "filas con el token guardado en vez del sub: $ROTAS"

if [ "$APLICAR" = 1 ]; then
    $SSH "cat > /tmp/reparar_identidad_social.sql" < "$MIGRACION" || morir "no se pudo subir la migración"
    $SSH "docker exec -i legacy_db psql -U dba -d applegacy -v ON_ERROR_STOP=1 < /tmp/reparar_identidad_social.sql" \
        || morir "la migración falló"
    QUEDAN=$($SSH "docker exec legacy_db psql -U dba -d applegacy -tAc \"
      select count(*) from core.users
       where (apple_id like '%.%.%' and length(apple_id) > 100)
          or (google_id like '%.%.%' and length(google_id) > 100);\"")
    echo "quedan sin reparar: $QUEDAN"
    [ "$QUEDAN" = "0" ] || echo "AVISO: quedan $QUEDAN filas ilegibles; revísalas a mano"
    $SSH "rm -f /tmp/reparar_identidad_social.sql"
else
    echo "(simulacro: no se aplica)"
fi

# ------------------------------------------------------------ 5. artefactos
#
# docker-compose.yml NO se sube: el versionado es el de desarrollo.
# firebase-service-account.json no suele estar en local y el bueno ya está en
# el servidor; por eso se copia aparte y su ausencia no aborta nada.
paso "5. Subir artefactos"
if [ "$APLICAR" = 1 ]; then
    scp -o BatchMode=yes server_linux config.docker.yaml Dockerfile \
        "$SSH_USER@$SERVER_IP:$DEPLOY_DIR" || morir "falló el scp"
    for extra in google-mailer-service-account.json firebase-service-account.json; do
        if [ -f "$extra" ]; then
            scp -o BatchMode=yes "$extra" "$SSH_USER@$SERVER_IP:$DEPLOY_DIR" \
                && echo "subido: $extra"
        else
            echo "no está en local (se conserva el del servidor): $extra"
        fi
    done
else
    echo "(simulacro: no se sube nada)"
fi

# --------------------------------------------------------------- 6. levantar
paso "6. Levantar"
if [ "$APLICAR" = 1 ]; then
    # --build, no restart: el config viaja DENTRO de la imagen.
    $SSH "cd $DEPLOY_DIR && docker compose up -d --build backend" || morir "falló el up"
    sleep 8
else
    echo "(simulacro: no se levanta)"
fi

# --------------------------------------------------------------- 7. verificar
paso "7. Verificar"
$SSH "cd $DEPLOY_DIR && docker compose ps --format '{{.Name}}\t{{.State}}'"
echo "--- últimas líneas del backend ---"
$SSH "cd $DEPLOY_DIR && docker compose logs backend --tail 25"
curl -s -o /dev/null -w "health después: %{http_code}\n" --max-time 20 \
    https://legacy.intelyclick.com/health

# Un token social inventado tiene que dar 401, no 500 ni un 201.
CODIGO=$(curl -s -o /dev/null -w "%{http_code}" --max-time 20 \
    -X POST https://legacy.intelyclick.com/api/auth/social-login \
    -H 'Content-Type: application/json' \
    -d '{"provider":"apple","id_token":"no-es-un-token"}')
echo "social-login con token inventado: $CODIGO (se espera 401)"

echo
if [ "$APLICAR" = 1 ]; then
    echo "Desplegado."
else
    echo "Simulacro terminado. Para desplegar de verdad: ./desplegar.sh -aplicar"
fi
