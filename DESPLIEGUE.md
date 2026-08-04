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

`.env` guarda hoy la contraseña de root en texto plano. Cambiar a clave SSH (`ssh-copy-id` y
`PasswordAuthentication no`) elimina el secreto más sensible del proyecto.

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
