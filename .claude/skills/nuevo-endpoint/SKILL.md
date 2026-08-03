---
name: nuevo-endpoint
description: Genera el corte vertical completo de un endpoint nuevo en el backend Go, recorriendo las seis capas de la arquitectura hexagonal en orden y registrando la ruta en main.go. Usar al añadir cualquier funcionalidad que exponga un endpoint HTTP nuevo.
---

# Añadir un endpoint al backend

Este backend usa arquitectura hexagonal con un **único punto de cableado**: `cmd/server/main.go`.
Añadir una funcionalidad toca seis archivos y hay que recorrerlos en orden, porque cada capa
depende de la anterior.

**El paso que más se olvida es el sexto.** `ImageHandler.UploadImage` está implementado, tiene
tests que pasan, y lleva meses sin funcionar porque nunca se registró su ruta. Un handler sin
entrada en `main.go` es código muerto que compila.

## Las seis capas, en orden

### 1. `internal/core/domain/<entidad>.go`

La entidad, con etiquetas `json` y `db`. Sigue el patrón de `event.go`: los campos opcionales son
punteros (`*string`, `*time.Time`), y los calculados que no vienen de la base llevan `db:"-"`.

### 2. `internal/core/ports/<entidad>_ports.go`

La interfaz del repositorio. Define solo lo que el servicio necesita, no un CRUD por defecto.

### 3. `internal/adapter/storage/postgres/<entidad>_repository.go`

La implementación con `pgxpool`. Reglas no negociables:

- **Siempre consultas parametrizadas** (`$1`, `$2`). No concatenar cadenas: es lo que mantiene el
  proyecto libre de inyección SQL.
- **Incluye `LIMIT` y `OFFSET`** en cualquier listado. Ningún repositorio existente lo hace y es
  deuda conocida; no la aumentes.
- Si la tabla o columna es nueva, **crea también la migración** en `scripts/AAAAMMDD_descripcion.sql`.
  `scripts/schema.sql` es un dump de `pg_dump` y no se edita a mano. Ha habido dos derivas entre
  código y esquema por saltarse esto.

### 4. `internal/core/services/<entidad>_service.go`

La lógica de negocio. Si maneja datos personales, mensajes o cualquier cosa sensible, el servicio
recibe `*security.CryptoService` y **cifra al escribir y descifra al leer**. Mira `chat_service.go`
o `synergy_service.go` como referencia. Una consulta que devuelva esos campos sin pasar por el
servicio entrega texto cifrado.

### 5. `internal/handler/http/<entidad>_handler.go`

El controlador. Obtén el usuario con `r.Context().Value(UserIDKey).(string)`.

**No devuelvas `err.Error()` al cliente.** El proyecto tiene ~75 sitios que lo hacen y filtran
nombres de columnas y códigos de PostgreSQL. Registra el detalle y devuelve un mensaje genérico.

### 6. `cmd/server/main.go` — instanciar y enrutar

Dos cosas separadas, y ambas hacen falta:

```go
// a) cablear el repositorio, el servicio y el handler
xRepo := postgres.NewXRepository(dbPool)
xService := services.NewXService(xRepo)
xHandler := handler.NewXHandler(xService)

// b) registrar la ruta en el grupo correcto
```

## Elegir el grupo de rutas

Es una decisión de seguridad, no de organización. Hay cuatro grupos:

| Grupo | Middleware | Para qué |
|---|---|---|
| Público | ninguno | Registro, login, recuperación de contraseña |
| Público con auth opcional | `OptionalAuthMiddleware` | Lecturas que enriquecen la respuesta si hay sesión |
| Privado | `AuthMiddleware` | Requiere usuario autenticado |
| Admin | `AdminOnly` | Solo administradores |

**Si la funcionalidad es administrativa, la ruta va dentro del grupo `AdminOnly`. No basta con
escribirlo en un comentario.** Las rutas de gestión de eventos están comentadas como "Admin Event
Management" y viven en el grupo privado: se verificó que un usuario de rol `familia` crea eventos
(HTTP 201) y los borra (HTTP 204).

## Antes de darlo por terminado

```bash
go build ./... && go vet ./... && go test ./...
```

Y comprueba la ruta de verdad, desde `Backend/` con el entorno levantado:

```bash
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/api/<ruta>
```

Ejecuta siempre desde `Backend/`: `main.go` carga `config.yaml` con ruta relativa y muere al
arrancar desde otro directorio.

Si el endpoint lo va a consumir la app móvil o el panel, añade la constante en
`App-Movil/lib/data/config/api_constants.dart` o el método en el servicio Angular correspondiente,
y verifica que la URL coincida exactamente.
