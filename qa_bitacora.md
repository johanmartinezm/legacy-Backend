# Bitácora de QA - Proyecto Go [BACKEND]

Entrada de trabajo para validación de API.

---

### [2026-08-04]: Gestión de eventos restringida a rol admin
- **Alcance:**
  - `Rutas`: `cmd/server/main.go` — `POST /api/events`, `PUT /api/events/{id}`, `DELETE /api/events/{id}`, `GET /api/events/{id}/feedback` y `POST /api/events/check-in` pasan del grupo `AuthMiddleware` al grupo `AdminOnly`. Estaban bajo autenticación de usuario pese al comentario "Admin Event Management", así que cualquier cuenta de la app móvil con sesión iniciada podía crear, editar y borrar eventos, registrar asistencia por QR y leer las calificaciones. Verificado antes del cambio: un token de rol `familia` obtenía `204 No Content` en el borrado, es decir, se ejecutaba.
  - **Sin cambios en las rutas de usuario:** registro a evento, agenda de talleres y calificación siguen bajo `AuthMiddleware`.
  - **Sin cambios en los clientes:** las cinco rutas solo las consume el panel Angular, que autentica con token de rol `admin` (`internal/core/services/auth_service.go:301`). La app móvil no llama a ninguna (`App-Movil/lib/data/services/event_service.dart`).
- **Criterios de QA:**
  1. **Usuario sin privilegios:** con la sesión de un usuario normal de la app, las cinco rutas deben responder **403**. Antes respondían 2xx.
  2. **Panel administrativo:** iniciar sesión en el panel y comprobar que se puede **crear, editar y eliminar** un evento con normalidad, ver las calificaciones de un evento y registrar una asistencia escaneando un QR.
  3. **Sin regresión para el usuario final:** desde la app, inscribirse a un evento, añadir y quitar un taller de la agenda y calificar una charla deben seguir funcionando.
  4. **Rutas públicas:** `GET /api/events`, `GET /api/events/{id}` y `GET /api/categories` siguen respondiendo sin autenticación.

### [2026-08-04]: Login social alcanzable en producción, asuntos de correo y `cmd/` fuera de `.gitignore`
- **Alcance:**
  - `Rutas`: `cmd/server/main.go` — alias `POST /api/auth/social-login`, `/api/auth/verify-email`, `/api/auth/resend-verification` y `/api/verify-email`. En producción HAProxy solo enruta `/api/...` y una lista fija de paths al backend; `social-login`, `verify-email` y `resend-verification` quedaban fuera de esa lista y morían en nginx con 405. Las rutas antiguas en la raíz se mantienen.
  - `Diagnóstico`: `internal/handler/http/user_handler.go` — se registra el motivo real del rechazo en `SocialLogin`, que antes se perdía al aplanarlo en un 401 genérico.
  - `Correo`: `internal/infrastructure/email/mime_header.go` (nuevo), `gmail_service.go`, `smtp_service.go` — asuntos codificados en RFC 2047. Las cabeceras son ASCII: el `charset=UTF-8` del `Content-Type` solo describe el cuerpo, y los acentos llegaban como mojibake.
  - `Versionado`: `.gitignore` — los patrones de binarios se anclan a la raíz. Sin la barra inicial, `server` casaba con `cmd/server/` y `main.go` nunca había estado en el repositorio.
  - `Configuración (no versionada)`: `config.docker.yaml` en el servidor recibe `firebase.google_client_id` con el cliente web del proyecto Firebase.
- **Criterios de QA:**
  1. **Rutas nuevas alcanzables:** `POST https://legacy.intelyclick.com/api/auth/social-login` con cuerpo `{}` debe responder **401**, no 404 ni 405. Un 405 con cabecera `server: nginx` significa que la petición no llegó al backend.
  2. **Sin regresión en las antiguas:** `POST /login` y `POST /register` siguen respondiendo desde el backend (401 y 500 con cuerpo `{}` respectivamente), no desde nginx.
  3. **Panel administrativo:** el enlace de verificación de correo que abre el panel (`/api/verify-email`) responde 400 con cuerpo vacío en vez de 405.
  4. **Asunto del correo:** registrar un usuario y comprobar en la bandeja de entrada que el asunto se lee `¡Bienvenido a Legacy Network!` y no `Ã‚Â¡Bienvenido...`. Verificar también el correo de restablecimiento de contraseña.
  5. **Validación de audiencia:** con `firebase.google_client_id` presente, un `id_token` emitido para otra aplicación debe ser rechazado; el motivo concreto queda en `docker compose logs backend` como `[SocialLogin] rechazado`.
  6. **Integridad del repositorio:** `git ls-files cmd/server/main.go` debe listar el archivo, y `git check-ignore server_linux` debe seguir ignorando el binario compilado.

### [2026-07-26]: Módulo de Foros Anónimos (Fase 1 y Fase 2 - DB & Backend)
- **Alcance:**
  - `Base de Datos`: Migraciones para tablas `core.forums`, `core.forum_posts`, `core.forum_post_reports`, adición de `alias` en `core.users`. Actualización de `DATA_DICTIONARY.md`.
  - `Backend (Go)`: Creación de dominio, puertos, repositorio, servicio y handler para foros (`forum.go`, `forum_ports.go`, `forum_repository.go`, `forum_service.go`, `forum_handler.go`). Integración en `main.go`.
- **Criterios de QA:**
  1. **Validación de Anónimos:** Verificar que los endpoints públicos (`/api/forums` y `/api/forums/{id}/posts`) NO expongan datos personales (`email`, `name`, `profile_image_url`). Solo debe retornar `author_alias`.
  2. **Validación de Alias:** Confirmar que al intentar proponer un foro o publicar un post sin alias configurado, el sistema retorna error `422 Unprocessable Entity` con el mensaje `alias_required`.
  3. **Control Administrativo:** Asegurar que los endpoints de admin (`/api/admin/forums/*`) permitan ver foros pendientes, aprobarlos (`PATCH /approve`), ocultarlos y eliminarlos correctamente, así como eliminar posts y revisar posts reportados.
  4. **Subida de Imágenes:** Confirmar que la lógica combinada de usar el `image_handler` existente y pasar el `image_url` en el JSON de creación del post funcione para incluir imágenes de manera anónima.
### [2026-06-29]: Registro Móvil - Parámetros de Persona Natural
- **Alcance:**
  - `Base de Datos (Local & Remota)`: Tabla `core.users`
  - `Backend (Go)`: `internal/core/domain/user.go`, `internal/handler/http/user_handler.go`, `internal/adapter/storage/postgres/user_repository.go`
  - `Frontend (Flutter)`: `lib/presentation/screens/register_screen.dart`, `auth_service.dart`, `auth_provider.dart`

- **Criterios de QA:**
  1. **Verificar guardado de términos:** Al registrarse, asegurarse de que `terms_accepted` y `data_sharing_accepted` se guarden como `true` en la base de datos PostgreSQL.
  2. **Verificar Intereses:** Confirmar que al seleccionar intereses en la app, se guarden correctamente en la tabla `core.user_interests`.
  3. **Despliegue seguro:** Confirmar que el registro normal de usuarios existentes sigue funcionando sin excepciones por campos faltantes.

### [2026-02-26]: Gestión de Agenda y QR Check-in (Backend)
- **Alcance:** 
  - `internal/adapter/storage/postgres/event_repository.go`
  - `internal/core/domain/event.go`
  - `internal/core/ports/event_ports.go`
  - `internal/core/services/event_service.go`
  - `internal/handler/http/event_handler.go`
  - `cmd/server/main.go`
  - `scripts/20260226_create_attendance_logs.sql`

- **Funcionalidad Nueva/Actualizada:**
  - `POST /api/events/check-in`: Nuevo endpoint que valida un QR, confirma asistencia y retorna los workshops inscritos.
  - Auditoría: Tabla `events.attendance_logs` para registrar quién valida cada QR.
  - Robustez: `COALESCE` en campos opcionales y JOINs optimizados para el reporte de check-in.

- **Criterios de QA (Puntos a Validar):**
  1. **Check-in QR:** Validar que al enviar un `qrData` válido a `/api/events/check-in`, se retorne el JSON con los datos del usuario y sus workshops.
  2. **Doble Check-in:** Verificar que si se escanea el mismo QR dos veces, el sistema registre ambos logs en `attendance_logs` pero mantenga el estado `attendance_confirmed = true`.
  3. **Seguridad Staff:** Confirmar que solo usuarios con token válido puedan realizar el check-in.
  4. **Nulabilidad:** Comprobar que el check-in funcione aun si el workshop no tiene descripción o imagen definida.
### [2026-02-26]: Administración de Usuarios (Persistencia Real)
- **Alcance:** 
  - `backend/go/internal/adapter/storage/postgres/user_repository.go`
  - `backend/go/internal/core/ports/interfaces.go`
  - `backend/go/internal/core/services/auth_service.go`
  - `backend/go/internal/handler/http/user_handler.go`
  - `backend/go/cmd/server/main.go`
  - `angular/legacy-app/src/app/core/services/user.service.ts`

- **Funcionalidad Nueva/Actualizada:**
  - **Backend:** Implementación de CRUD completo para usuarios (`List`, `Update`, `Delete`) con desencriptación de PII para la vista administrativa.
  - **Frontend:** Conexión del servicio de Angular con la API de Go, eliminando la dependencia de `users.json`.
  - **Mapeo:** Implementación de mapeo DTO entre camelCase (Angular) y snake_case (Go).

- **Criterios de QA (Puntos a Validar):**
  1. **Listado Real:** Verificar que la lista de usuarios en Angular muestre los datos reales de la base de datos (PostgreSQL) y no los de `users.json`.
  2. **Persistencia de Edición:** Al editar un usuario desde el panel, refrescar la página y validar que el cambio se mantenga (confirmando guardado en DB).
  3. **Seguridad (Cifrado):** Validar en la base de datos que los campos PII (Email, Nombre, etc.) se guarden cifrados, mientras que en el frontend se vean correctamente (desencriptados por el backend).
  4. **Eliminación:** Confirmar que al borrar un usuario, desaparezca de la lista y se elimine físicamente de la base de datos.

### [2026-02-26]: Integración Flutter - Login y Perfil Real
- **Alcance:** 
  - `flutter/legacy_app/lib/data/config/api_constants.dart`
  - `flutter/legacy_app/lib/data/services/auth_service.dart`
  - `flutter/legacy_app/lib/presentation/screens/profile/profile_screen.dart`
  - `backend/go/internal/handler/http/user_handler.go`
  - `backend/go/cmd/server/main.go`

- **Funcionalidad Nueva/Actualizada:**
  - **Login:** Verificado que Flutter ya llamaba correctamente al backend de Go para autenticación.
  - **Perfil (Me):** Implementación de endpoints `/api/me` (GET y PUT) en Go para que el usuario gestione su propio perfil.
  - **Persistencia Flutter:** Se eliminó la dependencia de `user_profile.json` en Flutter. Ahora la pantalla de Mi Perfil carga y guarda datos reales mediante el token JWT.
  - **Cifrado:** Los datos editados desde Flutter se cifran automáticamente en el backend antes de guardarse en PostgreSQL.

- **Criterios de QA (Puntos a Validar):**
  1. **Persistencia de Sesión:** Tras loguearse en Flutter, navegar a "Mi Perfil" y validar que los datos mostrados coincidan con los de la DB.
  2. **Actualización de Perfil:** Editar el teléfono o la bio en Flutter, guardar, y verificar que al reabrir la app los cambios persistan (llamada real a la API).
  3. **Seguridad de Token:** Intentar acceder a la pantalla de perfil sin estar logueado; el sistema debe manejar el error o redirigir (validado mediante `AuthProvider`).
  4. **Integridad de Datos:** Asegurar que al actualizar el perfil no se pierdan o corrompan campos no editados (como el email que es read-only).

### [2026-02-26]: Gestión de Seguridad - Cambio de Contraseña
- **Alcance:** 
  - `backend/go/internal/core/ports/interfaces.go`
  - `backend/go/internal/adapter/storage/postgres/user_repository.go`
  - `backend/go/internal/core/services/auth_service.go`
  - `backend/go/internal/handler/http/user_handler.go`
  - `flutter/legacy_app/lib/presentation/screens/profile/profile_screen.dart`

- **Funcionalidad Nueva:**
  - **Seguridad en Backend:** Implementación del flujo completo de validación de contraseña actual antes de permitir el cambio. Uso de `bcrypt` para verificación y nuevo hasheo.
  - **Endpoint Seguro:** Nuevo endpoint `POST /api/me/change-password` protegido por JWT.
  - **Interfaz Flutter:** Diálogo interactivo en la pantalla de perfil para realizar el cambio de clave con validación de coincidencia de campos.

- **Criterios de QA (Puntos a Validar):**
  1. **Validación de Clave Actual:** Intentar cambiar la contraseña ingresando una clave actual incorrecta; el sistema debe rechazarlo con un error 400.
  2. **Coincidencia de Campos:** El diálogo en Flutter debe validar que la "Nueva Contraseña" y "Confirmar" sean idénticas antes de enviar la petición.
  3. **Persistencia:** Tras cambiar la contraseña, cerrar sesión e intentar reingresar con la clave vieja (debe fallar) y con la nueva (debe funcionar).
  4. **Seguridad:** Confirmar que no se puede cambiar la contraseña de otro usuario manipulando IDs (el ID se extrae directamente del token JWT en el backend).
### [2026-02-26]: Matriculación de Usuarios (Panel Admin -> Backend)
- **Alcance:** 
  - `backend/go/internal/core/ports/event_ports.go`
  - `backend/go/internal/core/services/event_service.go`
  - `backend/go/internal/handler/http/event_handler.go`
  - `backend/go/internal/adapter/storage/postgres/event_repository.go`
  - `angular/legacy-app/src/app/core/services/registration.service.ts`

- **Funcionalidad Nueva/Actualizada:**
  - **Matriculación Real:** Se eliminó el mock en Angular; ahora las matriculaciones se persisten en PostgreSQL a través del API de Go.
  - **Overriding de Pago:** El endpoint de registro ahora soporta que un administrador marque la matriculación como 'paid' directamente desde el panel.
  - **Soporte de Talleres:** Registro automático de la relación entre el usuario y sus talleres seleccionados en la tabla `events.registration_workshops`.
  - **Consistencia Flutter:** Al marcarse como 'paid' en el registro admin, la App de Flutter desbloquea inmediatamente el código QR.

- **Criterios de QA (Puntos a Validar):**
  1. **Persistencia en DB:** Verificar que tras registrar un usuario en Angular, aparezca la fila en `events.registrations` con the `user_id` y `event_id` correctos.
  2. **Estado de Pago:** Validar que el `payment_status` sea 'paid' cuando la solicitud viene del Panel Admin.
  3. **Visualización en Flutter:** Confirmar que el usuario registrado vea su QR de asistencia en la App sin errores de "Pago Pendiente".
  4. **Talleres Relacionados:** Comprobar que los registros en `events.registration_workshops` coincidan con la selección hecha en el asistente de Angular.

---

### [2026-02-27]: Sistema de Contenido de Valor Personalizado (Administrable)
- **Alcance:**
  - **Backend (Go):** `scripts/20260227_create_custom_content.sql`, `internal/core/domain/`, `internal/adapter/storage/postgres/`, `internal/handler/http/content_handler.go`, `cmd/server/main.go`.
  - **Panel Admin (Angular):** `src/app/core/services/content_admin.service.ts`, `src/app/features/admin/custom-content/`.
  - **App Móvil (Flutter):** `lib/domain/models/custom_content_model.dart`, `lib/data/services/custom_content_service.dart`, `lib/presentation/screens/informandote/informandote_screen.dart`.

- **Funcionalidad Nueva/Actualizada:**
  - **Gestión Híbrida:** Capacidad de administrar contenido propio (Texto/Video) desde el panel e integrarlo fluidamente con la información de WordPress en la App.
  - **Formularios Dinámicos:** El panel de administración adapta sus campos según el tipo de contenido seleccionado (Markdown para texto, campos de URL para video).
  - **Carga Unificada:** La App de Flutter realiza peticiones paralelas a GraphQL (WordPress) y REST (API Go) para presentar una biblioteca de conocimiento unificada.

- **Criterios de QA (Puntos a Validar):**
  1. **Admin Panel:** Listar, crear, editar y eliminar contenidos de valor. Validar que el tipo 'video' pida URL y el tipo 'texto' pida contenido.
  2. **App Móvil (Informándote):** Unificación de fuentes. Confirmar que aparecen artículos de WP y locales.
  3. **Navegación:** Al pulsar un video local, debe abrir `VideoDetailScreen`. Al pulsar un artículo local, debe abrir `ArticleDetailScreen`.
  4. **Filtros:** Validar que al filtrar por categoría (ej: Finanzas), se traigan resultados de ambas fuentes si existen.
### [2026-03-04]: Sistema de Estadísticas y Tracking de Contenido
- **Alcance:**
  - **Backend (Go):** `internal/core/domain/stats.go`, `internal/adapter/storage/postgres/stats_repository.go`, `internal/core/services/stats_service.go`, `internal/handler/http/stats_handler.go`, `internal/handler/http/middleware.go`.
  - **Panel Admin (Angular):** `src/app/core/services/admin/stats/stats.service.ts`, `src/app/features/admin/statistics/`.
  - **App Móvil (Flutter):** Verificación de envío de token en `recordView` (existente).

- **Funcionalidad Nueva/Actualizada:**
  - **Agregaciones SQL:** Consultas optimizadas para Top 10 artículos, Top 10 usuarios y agrupaciones por periodo (Semana/Mes/Año).
  - **Dashboard Premium:** Nueva sección en Angular con tres "Cards" de resumen y gráficas de tendencia utilizando Chart.js.
  - **Tracking Unificado:** Implementación de `OptionalAuthMiddleware` en Go para capturar el ID de usuario en visitas si están autenticados, sin bloquear el acceso anónimo.
  - **Rankings:** Visualización de tablas con los contenidos más consumidos y los lectores más activos.

- **Criterios de QA (Puntos a Validar):**
  1. **Generación de Datos:** Realizar varias visitas a artículos en Flutter (logueado y anónimo) y verificar que se reflejen en el dashboard de Angular.
  2. **Interatividad de Gráficas:** Cambiar el filtro de periodo (Semana/Mes/Año) en la gráfica de tendencia y validar que los datos cambien correctamente.
  3. **Identificación de Usuarios:** Validar que en el "Top Lectores" aparezcan nombres reales si el usuario está logueado en la App.
  4. **Seguridad Admin:** Confirmar que solo usuarios Administradores puedan acceder a la ruta `/admin/statistics` en Angular y al endpoint de `/stats` en el backend.
### [2026-03-10]: Script de Compilación Cruzada para Linux (Backend)
- **Alcance:**
  - `Backend/build-linux.sh`: Nuevo script Bash para automatizar la cross-compilation.
- **Funcionalidad Nueva:**
  - **Cross-Compilation (Go):** Permite generar binarios estáticos optimizados para Linux (amd64) desde entornos de desarrollo Mac OS sin dependencias externas (CGO_ENABLED=0).
- **Criterios de QA (Puntos a Validar):**
  1. **Generación de Binario:** Ejecutar `sh build-linux.sh` en el directorio `Backend` y verificar la creación del archivo `server_linux`.
  2. **Portabilidad:** Si se tiene acceso a un servidor Linux, verificar que el binario se ejecuta correctamente sin errores de librerías faltantes.

### [2026-03-11]: Rediseño de Pantalla de Asesorías (Selector de 4 áreas)
- **Alcance:** 
  - `App-Movil/lib/presentation/screens/asesoria/asesoria_screen.dart`
- **Funcionalidad Nueva/Actualizada:**
  - **Grid de Selección:** Nueva interfaz con 4 botones circulares para categorizar las asesorías: ORDENAR, PROTEGER, CRECER y FORMAR.
  - **Informativa:** Inclusión de subtítulos de marca (L&M, AURUM, etc.), descripciones de servicios y perfil de destinatario directamente en la selección.
  - **Flujo de Contacto:** Al elegir un área, se habilita una tarjeta de detalle animada con un formulario rápido para solicitar asesoría personalizada.
- **Criterios de QA (Puntos a Validar):**
  1. **Navegación:** Al entrar a "Asesoría" desde el menú principal, deben aparecer los 4 botones circulares con sus iconos correspondientes.
  2. **Precisión de Textos:** Los textos descriptivos y el "footer" (destinatario) deben ser legibles y coincidir con los lineamientos de la imagen de referencia.
  3. **Selección:** Validar que al tocar un botón, este se resalte (color naranja) y se actualice el contenido de la tarjeta inferior.
  4. **Funcionalidad de Envío:** Probar el formulario de la tarjeta de detalle para asegurar que el mensaje de éxito se muestre correctamente al "Enviar solicitud".

### [2026-03-11]: Detalle de Programas, Carrito y Arquitectura Multi-Fuente
- **Alcance:** 
  - `App-Movil/lib/presentation/screens/programs/program_detail_screen.dart`
  - `App-Movil/lib/domain/models/program_model.dart`
  - `App-Movil/lib/data/services/graphql_service.dart`
  - `App-Movil/lib/data/config/config_service.dart`
  - `App-Movil/assets/config/config.json`

- **Funcionalidad Nueva/Actualizada:**
  - **Pantalla de Detalle Premium**: Nueva interfaz con diseño inmersivo, gradientes, info badges y galería de imágenes para programas.
  - **Integración de Carrito**: Conexión con `CartProvider` para añadir programas al carrito de compras desde el detalle.
  - **Limpieza de Precios**: Lógica de conversión de strings de moneda WordPress ($1.100) a valores numéricos para cálculos de factura.
  - **Arquitectura Híbrida GraphQL**: Configuración de doble fuente para separar el ecosistema de venta (`lso.school`) del de contenido divulgativo (`legacynetworkco.com`).

- **Criterios de QA (Puntos a Validar):**
  1. **Detalle Completo**: Validar que en `ProgramDetailScreen` se visualice: Nombre, Modalidad, Duración, Descripción (limpia de HTML) y Precio.
  2.  **Añadir al Carrito**: 
      - Pulsar "Añadir al carrito" y confirmar aparición del SnackBar. 
      - Verificar que el botón "VER CARRITO" en el SnackBar funcione.
      - En la pantalla de Carrito, validar que el precio del programa sea correcto y se sume al total.
  3. **Integración WhatsApp**: Probar el botón de contacto directo y verificar que el mensaje predefinido incluya el nombre del programa actual.
  4. **Validación Multi-Fuente**:
      - Entrar a "Programas": Los datos deben venir de `lso.school`.
      - Entrar a "Contenido de Valor": Los artículos deben cargar de `legacynetworkco.com`. Validar que los filtros de categorías sigan funcionando.
  5. **Manejo de Imágenes**: Asegurar que las imágenes de ambas fuentes (Legacy y LSO) carguen correctamente, considerando que Legacy usa proxies por CORS.

---

### [2026-03-11]: Comité de Sinergias (Módulo Comunitario Full-Stack) - [COMPLETADA Y SUBIDA]
- **Alcance:** 
  - **Backend (Go):** `internal/core/domain/synergy.go`, `internal/adapter/storage/postgres/synergy_repository.go`, `internal/core/services/synergy_service.go`, `internal/handler/http/synergy_handler.go`.
  - **Base de Datos:** `Base-de-Datos/20260311_create_synergy_system.sql`.
  - **App Móvil (Flutter):** `lib/domain/models/synergy_model.dart`, `lib/data/services/synergy_service.dart`, `lib/presentation/screens/community/synergy_list_screen.dart`, `lib/presentation/screens/community/synergy_detail_screen.dart`, `lib/presentation/screens/community/synergy_create_screen.dart`.

- **Funcionalidad Nueva:**
  - **Foro de Sinergias:** Sistema de debate comunitario donde los usuarios proponen ideas de negocio o colaboración.
  - **Interacción Social:** Soporte para "Likes" (Toggle), visualizaciones y comentarios jerárquicos.
  - **Feedback de Expertos:** Flag `is_expert_feedback` en la DB que resalta visualmente los comentarios de mentores/expertos con una distinción dorada.
  - **Paginación:** Los listados de sinergias están optimizados con paginación desde el repositorio PostgreSQL.

- **Criterios de QA (Puntos a Validar):**
  1. **Publicar Propuesta**: Crear una sinergia desde la App (botón +) y verificar que aparezca en el listado global.
  2. **Interacción (Likes)**: Pulsar el corazón en una sinergia y validar que el contador se incremente/decremente correctamente (Toggle).
  3. **Comentarios**: Publicar una opinión en una sinergia y verificar que aparezca en el hilo de discusión.
  4. **Seguridad**: Intentar publicar sin estar logueado (la App debe redirigir al login o el botón debe estar bloqueado por el middleware Auth).
  5. **Integridad de Base de Datos**: Al borrar una sinergia de la DB, confirmar que sus comentarios y likes se borren automáticamente (Cascade).

### [2026-03-12]: Búsqueda, Filtros y Contadores en Tiempo Real (Comité de Sinergias)
- **Alcance:** 
  - **Backend (Go):** `synergy_ports.go`, `synergy_service.go`, `synergy_repository.go`, `synergy_handler.go`.
  - **Base de Datos:** Alteración de tabla `community.synergies` para `comments_count`.
  - **App Móvil (Flutter):** `synergy_service.dart`, `synergy_model.dart`, `synergy_list_screen.dart`, `synergy_detail_screen.dart`.

- **Funcionalidad Nueva/Actualizada:**
  - **Buscador Full-Text**: Búsqueda por título o descripción en el listado de sinergias (case-insensitive).
  - **Filtros Dinámicos**: Selector de categorías superior (`Negocios`, `Legal`, etc.) con actualización instantánea.
  - **Contadores Reales**: Denormalización de `comments_count` en la tabla principal para visualización rápida en el listado.
  - **Toggle Like Activo**: Funcionalidad de 'Me gusta' operativa en la pantalla de detalle con actualización de estado.

- **Criterios de QA (Puntos a Validar):**
  1. **Buscador**: Escribir una palabra clave en el buscador y verificar que solo aparezcan sinergias relacionadas.
  2. **Cambio de Categoría**: Seleccionar "Negocios" y luego "Todas" para validar que el filtrado por categoría funcione correctamente en conjunto con el buscador.
  3. **Contador de Comentarios**: Publicar un comentario en una sinergia, volver a la lista y verificar que el número de opiniones se haya incrementado en +1.
  4. **Interacción de Likes**: En la pantalla de detalle, pulsar en la estadística de "interesados" y validar que el contador cambie y persista al volver a cargar.
  5. **Compilación Backend**: Verificar que el servidor compile sin errores tras los cambios en las firmas de los métodos del repositorio.

---

### [2026-03-20]: Sistema de Notificaciones Full-Stack (Legacy Board & Asesorías) - [COMPLETADA]
- **Alcance:** 
  - **Backend (Go):** `config.yaml`, `internal/config/`, `internal/core/ports/`, `internal/core/services/`, `internal/handler/http/`, `internal/infrastructure/email/`.
  - **App Móvil (Flutter):** `lib/data/services/board_service.dart`, `lib/data/services/asesoria_service.dart`, `lib/presentation/screens/community/comunidad_screen.dart`, `lib/presentation/screens/asesoria/asesoria_screen.dart`.

- **Funcionalidad Nueva/Actualizada:**
  - **Notificaciones Legacy Board:** Integración E2E para contactar a directivos mediante SMTP (Google).
  - **Solicitudes de Asesoría:** Sistema centralizado de solicitudes con destinatario único y clasificación por categorías en el asunto (Subject).
  - **Identidad del Remitente:** Inclusión obligatoria del Nombre y Correo del usuario solicitante en el cuerpo de todos los emails generados.
  - **Seguridad:** Uso de perfiles reales del backend para garantizar la veracidad del remitente mediante token JWT.

- **Criterios de QA (Puntos a Validar):**
  1. **Envío Legacy Board:** Seleccionar a Gonzalo o Luis Carlos y validar que el backend reciba el ID correcto y despache el correo al destinatario mapeado.
  2. **Categorización Asesoría:** Validar que al solicitar "CRECER", el correo tenga el subject dinámico correspondiente.
  3. **Visibilidad del Remitente:** Verificar en el HTML recibido que el campo *"Remitente"* muestre claramente el formato `Nombre <email@domain.com>`.
  4. **Robustez UI:** Confirmar que los SnackBars informen correctamente sobre el estado del envío (Cargando -> Éxito/Error).
  5. **Clean Code:** Verificación de desacoplamiento total entre la lógica de negocio (Services) y la implementación de red (SMTP).

---

### [2026-03-20]: Navegación Interactiva de Artículos (WordPress) - [COMPLETADA]
- **Alcance:** 
  - **App Móvil (Flutter):** `lib/domain/models/graphql_post_model.dart`, `lib/data/services/graphql_service.dart`, `lib/presentation/screens/informandote/article_detail_screen.dart`, `lib/presentation/screens/informandote/video_detail_screen.dart`.
  - **Dependencies:** `flutter_widget_from_html_core`.

- **Funcionalidad Nueva/Actualizada:**
  - **Habilitación de Links:** Preservación de etiquetas HTML `<a>` en el contenido de los posts.
  - **Navegación In-App:** Interceptación de URLs locales (`legacynetworkco.com`) para evitar salir al navegador del sistema.
  - **Carga por Slug:** Implementación de búsqueda por Slug en GraphQL para cargar artículos relacionados bajo demanda.
  - **Estabilidad de UI:** Integración de `HtmlWidget` con soporte de fuentes Google Fonts.

- **Criterios de QA (Puntos a Validar):**
  1. **Interactividad:** Validar que el texto "Leer más" al final de un artículo sea ahora un enlace clickable.
  2. **Continuidad:** Al pulsar un enlace interno, verificar que la app cargue el nuevo artículo sin salir al navegador.
  3. **Fallback:** Pulsar un enlace externo (ej: link en descripción de video) y validar que se abra el navegador externo.
  4. **Performance:** Verificar el mensaje de carga ("Cargando artículo...") durante la transición entre posts.
  5. **Integridad:** Confirmar que no se pierdan estilos clave ni se rompa el layout al inyectar HTML.

---

### [2026-03-20]: ChatBot Interactivo con Deep Linking (Simple Implementation) - [COMPLETADA]
- **Alcance:** 
  - **App Móvil (Flutter):** `lib/presentation/screens/chat/chatbot_screen.dart`.
  - **Dependencies:** `flutter_widget_from_html_core`, `go_router`.

- **Funcionalidad Nueva/Actualizada:**
  - **Quick Action Chips:** Implementación de carrusel de botones (Asesoría, Programas, Contacto, Libros) para facilitar la interacción rápida.
  - **Burbujas con HTML:** Las respuestas del bot ahora soportan negritas y enlaces clickables internos (`internal:/`).
  - **Navegación In-App (Deep Linking):** Al pulsar un link dentro del chat (ej: "Ir a Asesoría"), la app navega a la sección correspondiente sin salir de la vista actual de charla (Smooth UX).
  - **Visual Polish:** Optimización de padding, sombras y estilos de fuentes Google Fonts en el flujo de conversación.

- **Criterios de QA (Puntos a Validar):**
  1. **Chips de Acción:** Pulsar el botón de "Asesoría" en la barra superior y verificar que se rellene el chat y el bot responda con el link de acción.
  2. **Navegación Interna:** Pulsar un vínculo generado por el bot (ej: "aquí") y confirmar que la app navegue satisfactoriamente a la sección solicitada.
  3. **Scroll Automático:** Verificar que la lista de mensajes se desplace automáticamente hacia abajo al recibir un nuevo mensaje o al usar un chip.
  4. **Experiencia de Usuario:** Validar el mensaje de "Bot escribiendo..." durante la simulación de respuesta.

---

### [2026-04-16]: Corrección de Compilación (Importación 'strings' no utilizada)
- **Alcance:** 
  - `Backend/internal/core/services/asesoria_service.go`

- **Funcionalidad Nueva/Actualizada:**
  - Se eliminó la importación del paquete `strings` que no estaba siendo utilizado, permitiendo que el proyecto compile exitosamente.

- **Criterios de QA (Puntos a Validar):**
  1. **Compilación:** Verificar que el servidor inicie correctamente al ejecutar `sh run.sh`.
  2. **Regresión:** Confirmar que la solicitud de asesorías sigue enviando correos electrónicos sin problemas (la lógica de este servicio no fue alterada).

### [2026-06-21]: Sistema de Notificaciones Push Administrativo (FCM)
- **Alcance:**
  - **Backend (Go):** `Backend/cmd/server/main.go`, `Backend/config.yaml`, `Backend/internal/config/config.go`, `Backend/internal/infrastructure/firebase/fcm.go`, `Backend/internal/core/domain/notification.go`, `Backend/internal/core/ports/notification_ports.go`, `Backend/internal/core/services/notification_service.go`, `Backend/internal/handler/http/notification_handler.go`, `Backend/internal/handler/http/admin_middleware.go`.
  - **Base de Datos:** `Base-de-Datos/20260621_create_notifications_tables.sql`, `docs/DATA_DICTIONARY.md`.
  - **Angular (Frontend):** `Sitio-Administrativo/src/app/core/services/notification.service.ts`, `Sitio-Administrativo/src/app/features/admin/push-notifications/`, `Sitio-Administrativo/src/app/app.routes.ts`, `Sitio-Administrativo/src/app/core/layout/main-layout/main-layout.component.html`.

- **Funcionalidad Nueva/Actualizada:**
  - **Despacho FCM:** Endpoints de backend para registrar tokens de dispositivos (`POST /api/me/fcm-token`) y despachar notificaciones (`POST /api/admin/notifications/send`).
  - **Historial:** Backend almacena historial de notificaciones enviadas y recupera logs (`GET /api/admin/notifications/history`).
  - **Panel Administrativo:** Interfaz Angular integrada con Angular Material para redactar y despachar notificaciones, con selector de audiencia (Todos, Grupo, Usuario) e historial integrado.

- **Criterios de QA (Puntos a Validar):**
  1. **Envío General:** Validar que al enviar a "Todos los usuarios" en el panel Angular, la notificación sea despachada y se cree el log en el historial.
  2. **Envío por Grupo:** Comprobar que al enviar a un grupo, el payload use el tópico dinámico de Firebase (`/topics/group_{grupo}`).
  3. **Registro de Tokens:** Validar la inserción de nuevos tokens de dispositivos usando `/api/me/fcm-token` desde dispositivos móviles.
  4. **Seguridad:** Verificar que sólo administradores con sesión activa puedan despachar notificaciones y ver el historial.

### [2026-06-21]: Interfaz de Selección y Agrupación de Usuarios para Notificaciones Push
- **Alcance:**
  - **Angular (Frontend):** `Sitio-Administrativo/src/app/features/admin/push-notifications/push-notifications.component.ts`, `Sitio-Administrativo/src/app/features/admin/push-notifications/push-notifications.component.html`, `Sitio-Administrativo/src/app/features/admin/push-notifications/push-notifications.component.scss`, `Sitio-Administrativo/angular.json`.

- **Funcionalidad Nueva/Actualizada:**
  - **Interfaz de Selección de Destinatarios:** Rediseño del panel para ofrecer tres modos claros de audiencia: "Todos los usuarios", "Por Grupo (Rol de Usuario)" y "Selección Manual de Usuarios".
  - **Selector por Grupo:** Carga y lista a los usuarios filtrados por rol (`profesional`, `familia`, `empresa`) con un buscador dinámico e interactivo, permitiendo marcar/desmarcar individualmente o en lote.
  - **Envío Masivo e Indicador de Progreso:** Ejecuta el despacho secuencial de notificaciones individuales por ID de usuario para evitar inconsistencias de temas en FCM, mostrando una barra de progreso animada en tiempo real.
  - **Ajuste de Presupuesto:** Se incrementó el límite de presupuesto de estilos por componente en `angular.json` para dar soporte a las nuevas vistas estilizadas.

- **Criterios de QA (Puntos a Validar):**
  - **Visualización de Grupos:** Seleccionar "Por Grupo" y elegir "Familia" o "Profesional". Validar que se carguen correctamente los usuarios que pertenecen a ese grupo.
  - **Buscador en Tiempo Real:** Escribir parte de un correo o nombre en la barra de búsqueda y confirmar que la lista de usuarios se filtre dinámicamente.
  - **Envío Masivo y Barra de Progreso:** Componer un mensaje, seleccionar múltiples usuarios y presionar "Despachar Notificación". Validar que se muestre el indicador con la cantidad procesada (ej: "1 de 3") y la barra de progreso se complete.
  - **Historial:** Confirmar que tras los envíos masivos, el historial se refresque mostrando los registros correspondientes.

### [2026-06-21]: Gestión de Grupos Personalizados (Custom Groups)
- **Alcance:**
  - **Base de Datos:** `Base-de-Datos/20260621_create_custom_groups_tables.sql`, `docs/DATA_DICTIONARY.md`.
  - **Backend (Go):** `Backend/internal/core/domain/group.go`, `Backend/internal/core/ports/interfaces.go`, `Backend/internal/adapter/storage/postgres/group_repository.go`, `Backend/internal/core/services/group_service.go`, `Backend/internal/handler/http/group_handler.go`, `Backend/cmd/server/main.go`.
  - **Angular (Frontend):** `Sitio-Administrativo/src/app/core/services/group.service.ts`, `Sitio-Administrativo/src/app/features/admin/groups/`, `Sitio-Administrativo/src/app/app.routes.ts`, `Sitio-Administrativo/src/app/core/layout/main-layout/main-layout.component.html`, `Sitio-Administrativo/src/app/features/admin/push-notifications/`.

- **Funcionalidad Nueva/Actualizada:**
  - **Estructura de Base de Datos:** Tablas `core.custom_groups` y `core.custom_group_members` creadas con relaciones y restricciones adecuadas.
  - **API de Grupos en Go:** Endpoints para listar, crear, eliminar grupos personalizados, así como para consultar y guardar el listado de miembros asociados.
  - **Panel de Gestión de Grupos (Angular):** Nueva pantalla `/admin/groups` que permite crear grupos con un panel interactivo de maestro-detalle, ver miembros asignados y actualizar la membresía de usuarios mediante casillas de verificación de la base completa de usuarios.
  - **Integración con Notificaciones Push:** El selector "Por Grupo Personalizado" en el formulario de notificaciones ahora consume dinámicamente estos grupos, listando y pre-seleccionando a sus integrantes correspondientes.

- **Criterios de QA (Puntos a Validar):**
  - **Creación de Grupos:** Crear un grupo en la interfaz (ej: "Socios Premium"), verificar que aparezca en el listado y se guarde en la BD.
  - **Asignación de Usuarios:** Seleccionar un grupo, buscar y marcar/desmarcar usuarios mediante el checklist, y presionar "Actualizar Miembros". Verificar en base de datos y al re-seleccionar que los miembros persistan.
  - **Eliminación de Grupos:** Borrar un grupo, verificar que desaparezca del listado y sus relaciones de pertenencia sean eliminadas (ON DELETE CASCADE), manteniendo intactos a los usuarios.
  - **Integración en Notificaciones:** Ir a "Notificaciones Push", elegir "Por Grupo Personalizado", seleccionar el grupo creado y confirmar que se listen y pre-seleccionen exactamente los usuarios asignados al grupo en el paso de gestión.

### [2026-06-27]: Dockerización de Backend y Base de Datos (Go & PostgreSQL)

- **Alcance:**
  - `Backend/Dockerfile` (file:///Volumes/Disco2T/desarrollo/Legacy/appLegaci/Backend/Dockerfile)
  - `Backend/docker-compose.yml` (file:///Volumes/Disco2T/desarrollo/Legacy/appLegaci/Backend/docker-compose.yml)
  - `Backend/config.docker.yaml` (file:///Volumes/Disco2T/desarrollo/Legacy/appLegaci/Backend/config.docker.yaml)
  - `Backend/.env` (file:///Volumes/Disco2T/desarrollo/Legacy/appLegaci/Backend/.env)
  - `.gitignore` (file:///Volumes/Disco2T/desarrollo/Legacy/appLegaci/.gitignore)

- **Funcionalidad Nueva/Actualizada:**
  - **Dockerización**: Creación de entorno Docker con PostgreSQL 15 (Alpine) y la aplicación Go.
  - **Configuración Dockerizada**: Archivo `config.docker.yaml` configurado para usar la base de datos apuntando al host del contenedor `db`.
  - **Uso de Binario Precompilado**: Dockerfile optimizado para ejecutar directamente el binario compilado `server_linux` en una imagen minimalista de alpine, evitando recompilaciones pesadas en el servidor de producción.
  - **Variables de Entorno**: Archivo `.env` en Backend conteniendo variables de conexión del servidor para despliegues.

- **Criterios de QA (Puntos a Validar):**
  1. **Inicialización**: Correr `docker compose up -d` y verificar que los servicios `legacy_db` y `legacy_backend` se levanten correctamente en segundo plano.
  2. **Conectividad**: Comprobar logs (`docker compose logs backend`) y verificar que se logre establecer la conexión con la base de datos ("Connected to Database").
  3. **Healthcheck**: Consultar el endpoint público de salud `http://143.198.179.55:8080/health` y validar que retorne status HTTP 200 OK con respuesta "OK".

### [2026-06-27]: Reverse Proxy Seguro con SSL (HAProxy & Let's Encrypt)
- **Alcance:**
  - `HAProxy/haproxy.cfg` (file:///Volumes/Disco2T/desarrollo/Legacy/appLegaci/HAProxy/haproxy.cfg)
  - `HAProxy/docker-compose.yml` (file:///Volumes/Disco2T/desarrollo/Legacy/appLegaci/HAProxy/docker-compose.yml)
  - `Backend/docker-compose.yml` (file:///Volumes/Disco2T/desarrollo/Legacy/appLegaci/Backend/docker-compose.yml)
  - `Sitio-Administrativo/docker-compose.yml` (file:///Volumes/Disco2T/desarrollo/Legacy/appLegaci/Sitio-Administrativo/docker-compose.yml)

- **Funcionalidad Nueva/Actualizada:**
  - **Reverse Proxy**: Servidor HAProxy integrado para canalizar y balancear peticiones públicas sobre los puertos 80/443.
  - **Certificado SSL Let's Encrypt**: Automatización de generación y renovación de certificados HTTPS para `legacy.intelyclick.com` usando un contenedor Certbot con validación standalone en el puerto 8888.
  - **Seguridad en Redes**: Backend y frontend de Angular enlazados a la red interna docker `proxy-net`, eliminando la exposición de puertos directos en el host y delegando la seguridad de entrada a HAProxy.

- **Criterios de QA (Puntos a Validar):**
  1. **HTTPS Seguro**: Acceder a `https://legacy.intelyclick.com` y verificar que el navegador muestre el candado indicando que la conexión es segura con el certificado válido de Let's Encrypt.
  2. **Enrutamiento**: Confirmar que la app de Angular cargue en la raíz `https://legacy.intelyclick.com` y las llamadas a la API `/api/...` o `/health` se enruten correctamente al backend de Go en segundo plano.
  3. **Redirección HTTP a HTTPS**: Escribir `http://legacy.intelyclick.com` en el navegador y validar que redirija automáticamente a la URL segura con HTTPS.

### [2026-07-15]: Lógica de Verificación de Correos
- **Alcance:**
  - Migración SQL (20260715_add_email_verification.sql) para añadir `email_verified` en `users` y crear la tabla de tokens `email_verification_tokens`.
  - Configuración y servicio Gmail (`gmail_service.go`) para despachar correos usando cuenta base.
  - Endpoints nuevos: `/api/verify-email` y `/api/resend-verification`.
  - Integración en `auth_service.go` bloqueando logins hasta que `email_verified` sea `true`.
- **Criterios de QA (Puntos a Validar):**
  1. **Recepción del correo:** Generar un registro por la app móvil o el endpoint directamente y confirmar la recepción del correo de validación con el link `https://legacy.intelyclick.com/verify-email?token=...`.
  2. **Bloqueo en Login:** Confirmar que al intentar autenticarse en el endpoint `/login` con las nuevas credenciales, el backend devuelva el error HTTP 401/403 con mensaje de "email_not_verified".
  3. **Validación Exitosa:** Realizar la petición a `/api/verify-email` con el token recibido, validando que el flag `email_verified` quede en true en base de datos.

### [2026-07-20]: Integración CredibanCo (Fase 1 - Backend)
- **Alcance:**
  - Migración SQL (`20260720_create_transactions_table.sql`).
  - Capa de dominio y puertos en Go (`domain/transaction.go`, `ports/payment_ports.go`).
  - Cliente de infraestructura para CredibanCo API (`infrastructure/credibanco/client.go`).
  - Lógica de negocio de pagos (`services/payment_service.go`).
  - Endpoints REST para app móvil y angular (`handler/http/payment_controller.go`).
- **Criterios de QA (Puntos a Validar):**
  1. **Inicio de Transacción:** Validar que un llamado a `/api/payments/intent` registre la orden como `PENDING` en BD y retorne el `formUrl` de CredibanCo correctamente generado.
  2. **Verificación de Transacción:** Validar que el endpoint `/api/payments/verify/:order_id` consuma adecuadamente la pasarela de CredibanCo y cambie el estado de la tabla `transactions` a `APPROVED` o `DECLINED` de acuerdo a la respuesta real.
  3. **Manejo de Errores (CredibanCo down):** Simular una indisponibilidad de la red hacia CredibanCo y comprobar que la aplicación no falle abruptamente, sino que retorne un HTTP 500 y registre la transacción en error.

### [2026-07-20]: Integración CredibanCo (Fase 2 - App Móvil Web Checkout)
- **Alcance:**
  - `event_payment_screen.dart` y `checkout_screen.dart`: Se reemplazaron los botones mockeados y los campos locales de tarjeta por la invocación del API `/api/payments/intent`.
  - `PaymentService` (`data/services/payment_service.dart`): Creación del servicio para consumir el backend desde Flutter.
- **Criterios de QA (Puntos a Validar):**
  1. **Apertura de Pasarela:** Desde la App Móvil, agregar un evento o producto al carrito y pulsar el botón "Proceder al Pago" o "Confirmar Pago". Validar que se lance exitosamente el navegador in-app (o externo) hacia la URL de la pasarela segura proporcionada por el backend.
  2. **Estados Visuales (Loading):** Comprobar que el botón muestre un spinner de carga y no se pueda pulsar dos veces mientras se está solicitando la URL al backend.
  3. **Ausencia de captura local:** Certificar que la aplicación Flutter ya no pide en ningún momento números de tarjeta (para dar cumplimiento al scope PCI sin captura local).

### [2026-07-20]: Integración CredibanCo (Fase 3 - Panel Administrativo Angular)
- **Alcance:**
  - `registration-wizard.component.ts`: Modificación del asistente de pagos.
  - `payment-callback.component.ts`: Nuevo componente para manejar la respuesta del banco.
  - Repositorio Postgres Backend (`transaction_repository.go`): Creado y enlazado en `main.go`.
- **Criterios de QA (Puntos a Validar):**
  1. **Asistente de Pagos Admin:** Realizar un registro desde el panel web de administrador, validar que "Proceder al Pago" redirija a la pasarela web de pruebas.
  2. **Retorno Exitoso Angular:** Confirmar que al realizar el pago, la url de retorno apunte a `/admin/payment-callback?order_id=...` y el componente de Angular verifique el estado consumiendo la API, mostrando "Pago Exitoso" en pantalla.
