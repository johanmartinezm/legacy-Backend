# Despliegue del backend

Guía para publicar la API Go en el servidor de producción (`https://legacy.intelyclick.com`).

> Este repositorio es **público**. No escribas aquí ni en ningún archivo versionado la IP del
> servidor, el usuario SSH, contraseñas ni claves. Todos esos valores viven en `.env` y
> `config.docker.yaml`, que están excluidos por `.gitignore`.

## Cómo está montado producción

```
Internet ──► HAProxy (80/443, Let's Encrypt)
                │
                ├──►  legacy_frontend   nginx:alpine   panel Angular en la raíz
                └──►  legacy_backend    alpine + binario Go   rutas /api/... y /health
                             │
                             └──►  legacy_db   postgres:17-alpine   volumen legacy_db_data
```

Los tres contenedores se comunican por la red Docker **externa** `proxy-net` y **no publican
puertos en el host**: la única entrada pública es HAProxy.

HAProxy y el certbot viven en un proyecto aparte (`HAProxy/haproxy.cfg` y su `docker-compose.yml`)
que **no forma parte de este repositorio**. Si hay que tocar el enrutamiento o el certificado, es
allí, no aquí.

## Requisitos en el servidor

```bash
docker --version && docker compose version
docker network ls | grep proxy-net     # si no existe: docker network create proxy-net
```

## 1. Compilar el binario en local

El `Dockerfile` **no compila**: copia un binario ya construido. Compilar en el servidor exigiría
instalar Go y ~340 MB de toolchain, y por eso se descartó.

```bash
./build-linux.sh          # CGO_ENABLED=0 GOOS=linux GOARCH=amd64 → ./server_linux
```

El binario resultante es estático: no depende de glibc y por eso funciona sobre `alpine`.

## 2. Preparar `config.docker.yaml`

No está versionado. Antes de subirlo, revisa que:

| Clave | Valor esperado en producción |
|---|---|
| `database.dsn` | host **`db`** (nombre del servicio en `docker-compose.yml`), no `localhost` |
| `web_app.reset_password_url` / `verify_email_url` | el dominio público, no `localhost:4200` |
| `credibanco.base_url` | pasarela de **producción**, no la de pruebas (`ecouat`) |
| `security.encryption_key` | **32 caracteres exactos** (AES-256) y el mismo con el que se cifraron los datos |
| `security.jwt_secret` | secreto propio, no el de ejemplo |
| `firebase.google_client_id` | ver la advertencia de abajo |
| `storage.uploads_dir` | **`/data/uploads`**, que es el volumen `legacy_uploads` del `docker-compose.yml` |

**`storage.uploads_dir` tiene que apuntar al volumen.** Ahí se guardan las imágenes que suben los
foros. Si apunta a cualquier ruta interna del contenedor —`/tmp`, o vacío, que equivale a `uploads`
junto al binario—, las imágenes **se pierden enteras en el siguiente despliegue**, porque el
contenedor se recrea. No hay aviso: las subidas siguen respondiendo 200 y las imágenes viejas pasan
a dar 404.

**Cambiar `encryption_key` inutiliza todos los datos ya cifrados** (usuarios, mensajes de chat,
sinergias): quedan ilegibles y no hay forma de recuperarlos. No la toques sin migrar antes.

### Advertencia verificada: falta `google_client_id`

`config.docker.yaml` **no define** `firebase.google_client_id`, aunque `config.yaml` sí lo trae.
`cmd/server/main.go:68` pasa ese valor a `AuthService`, que lo usa en
`internal/core/services/auth_service.go:195`:

```go
payload, err := idtoken.Validate(ctx, idToken, s.googleClientID)
```

Con la audiencia vacía, `idtoken.Validate` **omite la comprobación del campo `aud`**: el token se
verifica como token legítimo de Google, pero no que haya sido emitido para esta aplicación.
Añade la clave con el cliente **web** (`client_type: 3`) de `google-services.json` — el mismo que
la app móvil pasa como `serverClientId`.

## 3. Subir los artefactos

Ningún script automatiza este paso. Los datos de conexión están en `.env` (no versionado):
`SERVER_IP`, `SSH_USER`, `SSH_PASS`, `DEPLOY_DIR`.

```bash
set -a; source .env; set +a

scp server_linux config.docker.yaml Dockerfile docker-compose.yml \
    google-mailer-service-account.json firebase-service-account.json \
    "$SSH_USER@$SERVER_IP:$DEPLOY_DIR"
```

`firebase-service-account.json` es opcional en el `Dockerfile` (se copia con comodín). **Sin él,
FCM arranca en modo mock**: las notificaciones se escriben en la consola del contenedor y nadie
las recibe.

### `SSH_PASS` — retirada el 2026-08-10

**La clave SSH está autorizada y funciona.** Comprobado el 2026-08-06 y de nuevo el 2026-08-10:
todos los despliegues se hacen con `ssh -o BatchMode=yes`, que desactiva la autenticación por
contraseña, y `SSH_PASS` no aparecía en ningún script.

**Ya no está en ningún `.env`, ni el local ni el del servidor, y la contraseña de root se rotó.**
Se hicieron las dos cosas porque el valor anterior contenía un `$`: Docker Compose intentaba
expandirlo como variable y **escupía parte de la contraseña en un aviso por cada servicio**, en
todos los `docker compose` del despliegue. Quien viera esos logs veía el fragmento.

De ahí una regla para cualquier valor que acabe en un `.env` leído por Compose: **sin `$`**, o
escapado como `$$`. La contraseña nueva es alfanumérica por ese motivo.

La copia anterior del archivo quedó en el servidor como `.env.bak.20260810`; bórrala cuando
confirmes que todo va bien.

### Acceso por contraseña: cerrado el 2026-08-10

**Hecho y verificado.** El servidor ya no ofrece autenticación por contraseña:

```
antes:   Permission denied (publickey,password)
después: Permission denied (publickey)
```

Se entra solo por clave. La contraseña de root sigue existiendo como vía de rescate por la consola
del proveedor, que no pasa por sshd.

Queda documentado abajo cómo se hizo, porque el procedimiento evidente **no funciona en este
servidor** y volverá a hacer falta si alguien reinstala o cloud-init regenera su configuración.

**No basta con editar `/etc/ssh/sshd_config`, y este es el error que hay que evitar.** Estado
comprobado el 2026-08-10 en el servidor (Ubuntu 24.04.3):

```
/etc/ssh/sshd_config:12   Include /etc/ssh/sshd_config.d/*.conf
   50-cloud-init.conf:1      PasswordAuthentication yes    ← este gana
   60-cloudimg-settings.conf:1  PasswordAuthentication no
```

**En sshd manda la PRIMERA aparición de cada opción, no la última**, y los archivos incluidos se
procesan en orden alfabético. Por eso gana el `yes` de `50-` sobre el `no` de `60-`, y por eso
añadir la línea al final de `sshd_config` no cambiaría nada: el `Include` de la línea 12 ya la ha
fijado antes. Se editaría el archivo, se recargaría el servicio y **todo seguiría igual, con la
sensación de haberlo arreglado**.

La forma que funciona es un archivo que se procese antes que `50-`. Se llama `10-` a propósito, y
así tampoco lo pisa cloud-init si regenera el suyo:

```bash
# 1. Crear la configuración
cat > /etc/ssh/sshd_config.d/10-hardening.conf << 'EOF'
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitRootLogin prohibit-password
EOF

# 2. Validar la sintaxis ANTES de recargar. Si esto falla, NO sigas.
sshd -t && echo "sintaxis correcta"

# 3. Comprobar el valor efectivo que quedará
sshd -T | grep -E "passwordauthentication|permitrootlogin"
#   esperado: passwordauthentication no / permitrootlogin prohibit-password

# 4. Recargar (no corta las sesiones abiertas)
systemctl reload ssh
```

`PermitRootLogin prohibit-password` deja entrar a root **solo por clave**, que es como ya se entra.
Hoy está en `yes`.

**Hazlo con una segunda sesión SSH abierta.** `reload` no cierra las sesiones existentes, así que
si algo va mal esa sesión sigue viva y permite revertir. Ten a mano el acceso a la consola del
proveedor antes de empezar.

#### Verificar de verdad

`sshd -T` dice lo que *se aplicaría*, no siempre lo que el proceso vivo está usando. La prueba
buena es intentar entrar por contraseña **desde otra máquina**, forzando que no use la clave:

```bash
ssh -o PubkeyAuthentication=no -o PreferredAuthentications=password root@<servidor>
```

Debe responder `Permission denied (publickey)` **sin llegar a pedir contraseña**. Si la pide, el
cambio no ha surtido efecto.

Y confirma que lo de siempre sigue funcionando, desde la sesión que ya tenías abierta:

```bash
ssh -o BatchMode=yes root@<servidor> "echo ok"
```

#### Revertir

Desde la segunda sesión SSH que dejaste abierta:

```bash
rm /etc/ssh/sshd_config.d/10-hardening.conf
sshd -t && systemctl reload ssh
sshd -T | grep passwordauthentication      # vuelve a decir "yes"
```

**Si te quedaste fuera del todo**, esa vía no existe: entra por la consola web del proveedor —que
pide la contraseña de root, la rotada el 2026-08-10, guardada fuera de git— y ejecuta ahí el mismo
`rm` y el `reload`.

Por eso el paso 2 no es opcional: un `sshd_config` inválido y un `reload` dejan el servicio sin
arrancar, y ahí la única entrada es la consola del proveedor.

## 4. Levantar

```bash
cd "$DEPLOY_DIR"
docker compose up -d --build
```

`--build` es necesario: la imagen del backend se reconstruye para incorporar el binario nuevo.
El `restart: always` hace que los contenedores vuelvan solos tras un reinicio del servidor.

## 5. Verificar

```bash
docker compose ps                       # legacy_db y legacy_backend en Up
docker compose logs backend | tail -30  # debe aparecer "Connected to Database"
curl -s -o /dev/null -w "%{http_code}\n" https://legacy.intelyclick.com/health   # 200
```

Un 200 en `/health` a través del dominio confirma las tres capas a la vez: HAProxy enruta, el
contenedor responde y el certificado es válido.

## Base de datos

**Primera instalación.** `scripts/schema.sql` es un dump de `pg_dump` y **no es idempotente**
(empieza con `CREATE SCHEMA chat;` sin `IF NOT EXISTS`): solo sirve sobre una base vacía.

```bash
docker compose exec -T db psql -U dba -d applegacy < scripts/schema.sql
```

**Actualizaciones.** Cada cambio posterior va como migración fechada en
`scripts/AAAAMMDD_descripcion.sql` y se aplica a mano, en orden:

```bash
docker compose exec -T db psql -U dba -d applegacy < scripts/20260731_add_synergies_comments_count.sql
```

No hay herramienta de migraciones ni control de cuáles se aplicaron: llevar la cuenta es manual.

**Respaldo antes de cualquier migración:**

```bash
docker compose exec -T db pg_dump -U dba applegacy | gzip > backup_$(date +%Y%m%d).sql.gz
```

### Respaldos automáticos

Desde el **2026-08-10** hay un respaldo diario. Antes no había ninguno: solo estos dumps a mano
antes de cada migración, con semanas de diferencia entre uno y otro. Lo que se perdía en un fallo
del volumen era todo lo ocurrido desde el último.

| | |
|---|---|
| Script | `/usr/local/sbin/legacy-db-backup` (fuente en `scripts/respaldo-diario.sh`) |
| Cuándo | Cron de root, **03:30** — media hora después de la renovación del certificado |
| Dónde | `/var/backups/legacy/applegacy_AAAAMMDD_HHMMSS.sql.gz` |
| Retención | 7 días |
| Registro | `/var/log/legacy-db-backup.log` |

El script **verifica antes de rotar**: comprueba que el contenedor está vivo, que el dump supera un
tamaño mínimo y que el `.gz` pasa `gzip -t`. Solo entonces borra los antiguos. Al revés —rotar
primero— un `pg_dump` fallido dejaría la carpeta vacía, y un archivo truncado se vería como un
respaldo bueno hasta el día que hiciera falta.

Escribe a `.parcial` y renombra al final, para que nunca exista un archivo con nombre definitivo a
medio escribir.

**Comprobar que sigue vivo:**

```bash
tail -3 /var/log/legacy-db-backup.log      # una línea "OK:" por día
ls -lh /var/backups/legacy/                # el más reciente, de hoy o ayer
```

**Restaurar** (⚠️ sobrescribe los datos actuales):

```bash
zcat /var/backups/legacy/applegacy_AAAAMMDD_HHMMSS.sql.gz \
  | docker exec -i legacy_db psql -U dba -d applegacy
```

**Los respaldos viven en el mismo servidor que la base.** Cubren el borrado accidental y una
migración que salga mal, que es lo que ocurre a menudo; **no cubren la pérdida del servidor**. Para
eso hay que copiarlos fuera —otro host, un bucket, lo que sea—, y eso sigue sin montarse.

⚠️ `docker compose down -v` **borra el volumen `legacy_db_data` y con él toda la base**. Para
apagar sin perder datos: `docker compose down` (sin `-v`) o `docker compose stop`.

## Rollback

No hay versionado de imágenes ni etiquetas. La vuelta atrás práctica es conservar el
`server_linux` anterior en el servidor antes de sobrescribirlo:

```bash
cp server_linux server_linux.bak     # en el servidor, ANTES del scp
# revertir:
cp server_linux.bak server_linux && docker compose up -d --build
```

## Límites conocidos de esta arquitectura

- **Una sola réplica del backend.** El hub de chat vive en memoria
  (`infrastructure/websocket`, arrancado con `go chatHub.Run()`). Dos instancias significan que
  los usuarios conectados a una no ven los mensajes de la otra. No escalar horizontalmente sin
  mover el hub a Redis o similar.
- **CORS abierto.** `AllowedOrigins: "*"` con `AllowCredentials: true`. Restringirlo al dominio
  del panel y al de la app antes de considerar la API endurecida.
- **Sin paginación.** Ningún repositorio usa `LIMIT`/`OFFSET`; los listados crecen sin tope y la
  respuesta se degrada con el volumen de datos.
- **`server.env`** aparece en la configuración pero ningún archivo Go lo lee: cambiarlo a
  `production` no altera el comportamiento.

## Ejecución directa, sin Docker

Si alguna vez se despliega el binario suelto (systemd, por ejemplo): `main.go` carga `config.yaml`
con **ruta relativa**, así que el proceso debe arrancar con el directorio de trabajo puesto en la
carpeta que contiene el archivo. Desde cualquier otro sitio muere al arrancar.
