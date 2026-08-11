#!/bin/bash
#
# Respaldo diario de la base de producción.
#
# Se instala en el servidor como /usr/local/sbin/legacy-db-backup y lo lanza el
# cron de root. Ver "Respaldos automáticos" en DESPLIEGUE.md.
#
# Antes del 2026-08-10 no había ningún respaldo automático: solo dumps a mano
# antes de cada migración. Entre uno y otro podían pasar semanas, y lo que se
# perdía en un fallo del volumen era todo lo ocurrido desde el último.

set -euo pipefail

CONTENEDOR="legacy_db"
USUARIO="dba"
BASE="applegacy"
DESTINO="/var/backups/legacy"
DIAS_A_CONSERVAR=7
LOG="/var/log/legacy-db-backup.log"

# Un dump válido de esta base ronda los 20 KB comprimidos. El umbral existe para
# no dar por bueno un archivo truncado: si pg_dump falla a medias, gzip devuelve
# un .gz correcto pero con la base incompleta, y la rotación borraría los buenos
# creyendo que hay uno nuevo.
MINIMO_BYTES=5000

fecha="$(date +%Y%m%d_%H%M%S)"
archivo="$DESTINO/applegacy_$fecha.sql.gz"
temporal="$archivo.parcial"

log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" >> "$LOG"; }

# Si algo falla, no se deja un archivo a medias que parezca un respaldo bueno.
limpiar_si_falla() {
    rm -f "$temporal"
    log "ERROR: el respaldo falló y no se ha rotado nada"
}
trap limpiar_si_falla ERR

mkdir -p "$DESTINO"

if ! docker ps --format '{{.Names}}' | grep -qx "$CONTENEDOR"; then
    log "ERROR: el contenedor $CONTENEDOR no está corriendo"
    exit 1
fi

# Se escribe a .parcial y solo se renombra al final: así nunca existe un archivo
# con nombre definitivo que esté a medio escribir.
docker exec "$CONTENEDOR" pg_dump -U "$USUARIO" "$BASE" | gzip > "$temporal"

tamano=$(stat -c%s "$temporal")
if [ "$tamano" -lt "$MINIMO_BYTES" ]; then
    log "ERROR: el dump ocupa $tamano bytes, por debajo del mínimo de $MINIMO_BYTES"
    exit 1
fi

# gzip -t detecta un archivo truncado o corrupto antes de fiarse de él.
if ! gzip -t "$temporal" 2>/dev/null; then
    log "ERROR: el archivo comprimido no supera la comprobación de integridad"
    exit 1
fi

mv "$temporal" "$archivo"
trap - ERR

# La rotación va DESPUÉS de confirmar que el nuevo es bueno. Al revés, un fallo
# dejaría la carpeta vacía.
borrados=$(find "$DESTINO" -name 'applegacy_*.sql.gz' -mtime "+$DIAS_A_CONSERVAR" -print -delete | wc -l)

log "OK: $archivo ($tamano bytes); retirados $borrados respaldos de más de $DIAS_A_CONSERVAR días"
