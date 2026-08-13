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
| `security.jwt_secret` | secreto propio, no el de ejemplo (rotado el 2026-08-13) |
| `firebase.google_client_id` | el cliente **web**; presente y verificado |
| `apple.bundle_id` | `co.legacynetwork.legacyapp`; sin él, Sign in with Apple rechaza a todo el mundo |
| `storage.uploads_dir` | **`/data/uploads`**, que es el volumen `legacy_uploads` del `docker-compose.yml` |

**`storage.uploads_dir` tiene que apuntar al volumen.** Ahí se guardan las imágenes que suben los
foros. Si apunta a cualquier ruta interna del contenedor —`/tmp`, o vacío, que equivale a `uploads`
junto al binario—, las imágenes **se pierden enteras en el siguiente despliegue**, porque el
contenedor se recrea. No hay aviso: las subidas siguen respondiendo 200 y las imágenes viejas pasan
a dar 404.

**Cambiar `encryption_key` inutiliza todos los datos ya cifrados** (usuarios, mensajes de chat,
sinergias): quedan ilegibles y no hay forma de recuperarlos. No la toques sin migrar antes.

### ⚠️ La copia local de este archivo se desincroniza sola — comprobado el 2026-08-13

Este archivo **se edita en los dos sitios**: en el servidor cuando hay prisa, y en local cuando se
prepara un despliegue. Como no está versionado, nada avisa de que difieran.

Pasó entre el 11 y el 13 de agosto: se añadió `apple.bundle_id` directamente en el servidor y la
copia local se quedó sin él. El `scp` del paso 3 sube el local, así que **el siguiente despliegue
habría borrado esa clave y dejado a todo el mundo fuera de Sign in with Apple**, sin tocar una sola
línea de código y sin ningún error visible en el arranque.

**Antes de cualquier `scp`, compara:**

```bash
ssh "$SSH_USER@$SERVER_IP" 'sha256sum /docker/legacy/config.docker.yaml'
sha256sum config.docker.yaml
```

Si no coinciden, **baja primero el del servidor** (`scp` en sentido contrario) y aplica encima tus
cambios. El servidor es la copia buena: es la que está corriendo.

### `google_client_id`: presente desde el 2026-08-13

La advertencia anterior —que `config.docker.yaml` no lo definía y que por eso `idtoken.Validate`
omitía la comprobación del `aud`— **ya no aplica**. Verificado en el servidor: la clave está, con el
cliente **web** (`client_type: 3`) de `google-services.json`, el mismo que la app pasa como
`serverClientId`. Si algún día se reescribe el archivo desde cero, esa clave tiene que seguir ahí.

### Rotar el `jwt_secret`

Hecho el 2026-08-13, porque producción firmaba con `super-secret-jwt-key-change-me`. Con ese valor
—un placeholder que está en cualquier tutorial— **cualquiera podía firmarse un token de rol `admin`
y entrar a las rutas de administración**; comprobado contra producción antes de rotarlo.

```bash
cd /docker/legacy
cp config.docker.yaml config.docker.yaml.bak.$(date +%Y%m%d_%H%M)
NUEVO=$(openssl rand -hex 48)          # 96 caracteres, sin '$' (ver la regla del .env)
# sustituir el valor de jwt_secret por "$NUEVO"
docker compose up -d --build backend   # el Dockerfile COPIA el config: reiniciar NO basta
```

**Rotarlo invalida todas las sesiones abiertas**: la app y el panel piden iniciar sesión otra vez.
Eso es todo el impacto; no toca datos.

**El paso que se olvida es el `--build`.** El `config.docker.yaml` viaja dentro de la imagen
(`COPY config.docker.yaml /app/config.yaml`), así que editarlo en `/docker/legacy` y hacer
`restart` deja el contenedor con el valor viejo y la sensación de haber rotado nada.

Verificación de que surtió efecto, sin necesidad de credenciales: firma un token con el secreto
**viejo** y pídele una ruta de administración. Antes daba 200; después tiene que dar 401.

### Rotar la `encryption_key` — pendiente, y NO es un cambio de configuración

Producción sigue con la clave de ejemplo (`0123456789…`). Cambiarla en el YAML y reconstruir **deja
la base ilegible**: `gcm.Open` falla con la clave nueva y no hay forma de volver atrás si además se
perdió la vieja. Nadie podría iniciar sesión por correo, ni leer un chat, ni ver un nombre.

Hay que hacerlo con un comando de un solo uso —`cmd/recifrar`, aún sin escribir— que lea con la
clave vieja y escriba con la nueva, **dentro de una transacción**, sobre este inventario:

| Tabla | Columnas |
|---|---|
| `core.users` | `email_encrypted`, `first_name`, `last_name`, `phone`, `location`, `bio`, `company_name`, `job_title`, `identification_number` |
| `core.users` | `email_blind_index` — **no se descifra: se recalcula** |
| `chat.messages` | `content_encrypted` |
| `events.registrations` | `participant_name`, `participant_email`, `participant_phone` |

**El `email_blind_index` es la trampa.** No es un cifrado sino un `HMAC-SHA256` con esa misma clave
(`internal/security/crypto.go:85`), y es **por donde se busca al usuario al iniciar sesión**
(`auth_service.go:325`). Si se re-cifran los datos y no se recalcula el índice, la base queda intacta
y legible, todo parece haber salido bien, y **nadie vuelve a poder entrar con correo y contraseña**:
la búsqueda no encuentra a nadie y responde credenciales inválidas.

Las sinergias no guardan nada cifrado propio: muestran campos de `core.users`, así que se arreglan
solas al arreglar esa tabla.

Orden y precauciones:

1. **Respaldo completo de la base** y comprobar que se restaura, no solo que se creó el `.gz`.
2. **Parar el backend** (`docker compose stop backend`). Escribir mientras acepta peticiones deja
   filas nuevas cifradas con la clave vieja detrás del cursor de la migración.
3. Ejecutar el comando con las dos claves, la vieja y la nueva.
4. Cambiar `encryption_key` en `config.docker.yaml` y `docker compose up -d --build backend`.
5. Verificar **antes de dar por buena la ventana**: iniciar sesión con correo y contraseña, abrir un
   chat con historial y ver un nombre de usuario en el panel. Si algo de eso falla, restaurar el
   respaldo: es más rápido que diagnosticar.

Una fila que no descifre con la clave vieja debe **abortar** la migración, no saltarse: un valor que
no se puede leer hoy tampoco se recupera mañana, y saltarlo lo convierte en pérdida silenciosa.

## 3. Subir los artefactos

Ningún script automatiza este paso. Los datos de conexión están en `.env` (no versionado):
`SERVER_IP`, `SSH_USER`, `SSH_PASS`, `DEPLOY_DIR`.

```bash
set -a; source .env; set +a

scp server_linux config.docker.yaml Dockerfile \
    google-mailer-service-account.json firebase-service-account.json \
    "$SSH_USER@$SERVER_IP:$DEPLOY_DIR"
```

### ⚠️ NO subas `docker-compose.yml` — comprobado el 2026-08-11

**El `docker-compose.yml` versionado es el de desarrollo local y NO sirve para producción.** Difiere
en dos cosas del que vive en el servidor:

| | Repositorio (desarrollo) | Servidor (producción) |
|---|---|---|
| `POSTGRES_PASSWORD` | `"123"` | contraseña real, fuera de git |
| Resto | igual | igual |

Subirlo sobrescribe la contraseña de Postgres con `"123"` en el archivo. **Pasó el 2026-08-11**: el
daño fue limitado porque Postgres **ignora `POSTGRES_PASSWORD` cuando el volumen de datos ya está
inicializado**, así que la contraseña efectiva de la base no cambió y el backend siguió conectando.
Pero el archivo quedó mintiendo, y si alguien recreara el volumen la base nacería con `"123"`.

**Si hay que cambiar el compose de producción** —por ejemplo para añadir un volumen—, edítalo
**en el servidor** partiendo del que ya está, o copia solo el fragmento nuevo. Haz antes
`cp docker-compose.yml docker-compose.yml.bak.$(date +%Y%m%d_%H%M)` y compara después:

```bash
diff docker-compose.yml.bak.AAAAMMDD_HHMM docker-compose.yml
```

### Postgres ya no publica el 5432 a Internet — cerrado el 2026-08-12

Estuvo abierto en `0.0.0.0:5432` (y en IPv6) con `ufw` inactivo, comprobado desde fuera el
2026-08-11 y de nuevo el 2026-08-12. Venía del `ports: - "5432:5432"` del compose de producción.

**Se ató a la interfaz local** en vez de retirar la publicación entera:

```yaml
    ports:
      - "127.0.0.1:5432:5432"
```

Así deja de ser alcanzable desde Internet —comprobado: la conexión da *Connection refused*— pero
**pgAdmin, DBeaver y `psql` siguen sirviendo a través de un túnel SSH**, que es lo que se habría
perdido quitando el `ports` del todo:

```bash
ssh -L 5432:127.0.0.1:5432 <usuario>@<servidor>   # y conectar a localhost:5432
```

Ojo con `ufw`: **no habría bastado**. Docker escribe sus reglas en la cadena `DOCKER` de iptables,
que `ufw` no gobierna, así que un puerto publicado sigue abierto aunque el firewall esté activo. El
cierre tiene que venir del binding, como aquí.

El backend no se vio afectado en ningún momento: se conecta por `proxy-net` con el host `db`, no
por el puerto publicado. Recrear `legacy_db` corta las conexiones abiertas del pool y pgx
reconecta solo.

### Pasarela de pago simulada — solo para desarrollo

Mientras CredibanCo siga devolviendo «acceso denegado», el flujo de pago se puede probar entero con
una pasarela de mentira que no cobra nada:

```yaml
credibanco:
  base_url: "https://ecouat.credibanco.com/payment/rest/"   # UAT o localhost
  simulado: true
  simulado_base_url: ""     # la dirección de ESTE backend vista por el teléfono
```

`simulado_base_url` importa más de lo que parece: **quien abre ese enlace es el navegador del
teléfono, no el servidor**. Vacío vale para web y para el simulador de iOS; en el emulador de
Android es `http://10.0.2.2:8080`, y en un teléfono real, la IP del equipo en la red local.

Con eso, `/api/payments/intent` devuelve una pantalla propia con tres botones —aprobar, rechazar,
dejar pendiente— que redirige al `returnUrl` igual que haría el banco. Sirve para comprobar el
recorrido completo: intención, salida al navegador, vuelta por el deep link, notificación e
inscripción confirmada.

**No valida el comportamiento real del banco**: ni su redirección, ni el formato de su notificación,
ni si el importe se interpreta ×100 o en pesos enteros. Eso solo lo cierra una transacción real de
importe mínimo.

**El backend se niega a arrancar** si `simulado` está activo con una `base_url` que no sea de
pruebas: la comprobación es por lista blanca —`ecouat`, `localhost`, `127.0.0.1`, `sandbox`,
`test`— y ocurre antes de abrir la base de datos, así que no hay camino por el que una configuración
así llegue a servir peticiones. Las rutas `/api/payments/simulado/...` **solo se registran** con el
modo encendido.

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
