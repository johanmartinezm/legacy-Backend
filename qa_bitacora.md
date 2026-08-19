# Bitácora de QA - Proyecto Go [BACKEND]

Entrada de trabajo para validación de API.

---

### [2026-08-18]: Los videos de los canales de YouTube llegan a la app

Punto 1.4 de `reports/20260818_plan_ajustes.html`. El cliente encontró **un solo video** en Contenido
de Valor y pidió que salieran todos los de los canales de Legacy Network y LSO.

- **El problema:** no existía ninguna integración con YouTube. Los "videos" eran las entradas que
  alguien hubiera marcado con tipo `video` en WordPress o en el contenido propio, y había una.
- **Alcance:**
  - `internal/config/config.go` — sección `youtube` (clave, canales, tope por canal).
  - `internal/core/domain/video_canal.go` — nuevo. `VideoDeCanal`.
  - `internal/core/ports/interfaces.go` — `CanalDeVideos` y `VideoService`.
  - `internal/infrastructure/youtube/client.go` — nuevo. Cliente de la API v3.
  - `internal/core/services/video_service.go` — nuevo. Reúne canales y cachea.
  - `internal/handler/http/video_handler.go` — nuevo.
  - `cmd/server/main.go` — **ruta registrada**: `GET /api/content/videos`, pública.
- **La llamada la hace el backend y no la app, a propósito.** Los cuatro repositorios son públicos y
  una clave embarcada en el binario de Flutter se extrae sin dificultad. Además así la cuota se gasta
  una vez para todos en lugar de una por cada persona que abre la pantalla.
- 🔴 **`channels.list` + `playlistItems.list`, nunca `search.list`.** Las dos primeras cuestan **1
  unidad** por llamada; `search.list` cuesta **100**, y con una cuota diaria de 10.000 la agotaría con
  un puñado de usuarios. Quien toque esto que no lo "simplifique" con una búsqueda.
- **Se usa `forHandle` y no el identificador `UC…`.** El handle es lo que aparece en la URL del canal;
  la API lo acepta desde 2023. La página pública del canal **no** expone el identificador —se pinta
  por JavaScript—, así que sacarlo de ahí no era opción.
- **La caché es la razón de ser del servicio, no un adorno.** Una hora de TTL: un canal de esta
  comunidad publica cada pocos días, así que el retraso no se nota y el gasto baja a dos llamadas por
  canal y hora. Vive en el proceso, igual que el hub de chat, así que no añade una limitación de
  escalado que no existiera ya.
- **Si fallan todos los canales se devuelve la caché aunque esté caducada.** Material viejo es mejor
  que una sección vacía.
- **El endpoint siempre responde 200 con una lista.** La app une esta fuente con otras dos; un error
  aquí dejaría la pantalla entera en blanco por un problema de un tercero.
- **Sin clave configurada el servicio se queda a nil** y el endpoint devuelve lista vacía. Es lo que
  pasa hoy en cualquier entorno donde no se haya puesto.
- **Se descartan los videos sin id utilizable** —borrados o privados siguen apareciendo en la lista—
  porque enlazarlos llevaría a una página de error.
- **Verificado contra YouTube de verdad**, no con dobles: **87 videos**, 37 de Legacy Network y 50 de
  LSO (el tope configurado), los 87 con id único, miniatura, enlace y canal. La primera llamada tardó
  **1,48 s** y las siguientes **0,003 s**, lo que confirma que la caché evita la cuota.
- ⚠️ **La clave va en `config.yaml`**, cubierto por el `.gitignore`. Falta llevarla a las otras dos
  copias: el `config.docker.yaml` del servidor y el repositorio privado de secretos.
- **Criterios de QA:**
  1. **`GET /api/content/videos`** responde 200 con una lista no vacía.
  2. **Los videos traen `channel`** con el nombre del canal, y aparecen los dos canales.
  3. **Segunda llamada seguida:** responde en milisegundos (sale de caché).
  4. **Con la clave borrada del config:** responde 200 con `[]` y el arranque lo avisa en el log.
  5. **Con un canal inexistente en la lista:** los demás siguen llegando y el fallo va al log.
  6. **En la app**, la sección de contenido con el filtro «Videos» muestra decenas, no uno.
  7. **Abrir uno:** reproduce y la firma es el nombre del canal.

---

### [2026-08-18]: Los eventos virtuales dejan de dar QR, y la inscripción avisa por correo

Puntos 2.3 y 2.2 de `reports/20260818_plan_ajustes.html`. Van juntos porque el segundo depende del
primero: sin saber si un evento es virtual no había enlace que meter en el correo.

- **El problema:** `events.events` solo tenía `location`, un texto libre, así que **se emitía QR para
  toda inscripción confirmada**, también para una masterclass virtual, donde no abre ninguna puerta.
  Y **inscribirse no enviaba ningún correo**: el cliente preguntó literalmente "¿o dónde me da el link
  de ingreso?".
- **Alcance:**
  - `scripts/20260818_modalidad_y_enlace_evento.sql` — nuevo. `is_virtual` y `access_url`.
  - `internal/core/domain/event.go` — los dos campos, `EventIsVirtual`/`AccessURL` en la inscripción
    y el tipo `CorreoInscripcion`.
  - `internal/adapter/storage/postgres/event_repository.go` — cuatro consultas.
  - `internal/core/services/event_service.go` — regla de qué se entrega según modalidad; el servicio
    acepta repositorio de usuarios y servicio de correo.
  - `internal/core/services/event_correo.go` — nuevo. Arma y manda la confirmación.
  - `internal/core/ports/interfaces.go` e `internal/infrastructure/email/gmail_service.go` —
    `SendEventRegistrationEmail`.
  - `cmd/server/main.go` — el servicio de eventos recibe las dos dependencias. **No hay rutas nuevas.**
- **`is_virtual` booleano en vez de un enum de modalidad.** Hoy solo hay dos casos y son excluyentes.
  Si aparece el híbrido, un booleano se migra a enum sin perder datos; al revés no.
- **`DEFAULT false` deja los eventos existentes como presenciales**, que es lo que son todos. La
  migración no toca ninguna fila.
- **La regla de qué se entrega vive en el servicio, no en la consulta**, para que esté escrita en un
  solo sitio: pendiente de pago no recibe nada; virtual recibe enlace y no QR; presencial al revés.
- **El QR se sigue generando también en los virtuales.** No cuesta nada y deja el caso de un evento
  que cambia de modalidad resuelto solo: si un virtual pasa a presencial, su credencial ya existe.
- **El correo nunca bloquea ni falla la inscripción**, igual que los avisos push: sale en su propia
  goroutine, con contexto propio de 30 s —el de la petición HTTP se cancela al responder— y un fallo
  solo se registra en el log. El cupo ya está reservado en la base.
- **El correo cambia con la modalidad:** el virtual lleva el botón con el enlace; el presencial remite
  a "Mi credencial" en la app. **El QR nunca viaja por correo**: es lo que da derecho a entrar y un
  buzón reenviado lo repartiría.
- **Un virtual sin enlace cargado todavía no deja el correo mudo:** dice que el enlace llegará antes
  de la sesión, en vez de omitir el bloque.
- **El destinatario sale del contacto de la inscripción y, si viene vacío, del perfil.** Los dos están
  cifrados; se descifran aquí.
- **`ConCorreoDeInscripcion` va aparte del constructor** para no romper las llamadas existentes ni
  obligar a los tests a pasar dos dependencias que no usan.
- ⚠️ **El correo del flujo de pago no está.** Una inscripción de pago nace pendiente y se confirma en
  `paymentService.VerifyPayment`; ahí falta la misma llamada. No se añadió porque la pasarela sigue
  bloqueada y no se puede probar.
- ⚠️ **La base local estaba once migraciones por detrás.** `levantar.ps1` solo las aplica en una base
  vacía, así que faltaba desde `20260731`. Se aplicaron las trece en orden y **todas pasaron limpias**,
  lo que confirma de paso que son reejecutables.
- **Verificado:** `go build`, `go vet` y `go test ./internal/core/...` en verde, y el flujo probado
  contra la base local con tres eventos de prueba:

  | Evento | Modalidad | Estado | qrData | accessUrl |
  |---|---|---|---|---|
  | Legacy Summit QA | presencial | confirmed | `REG-…` | vacío |
  | Masterclass Virtual QA | virtual | confirmed | vacío | el enlace |
  | Masterclass Virtual QA de pago | virtual | pending_payment | vacío | vacío |

- **Criterios de QA:**
  1. **Aplicar la migración** y comprobar que `events.events` tiene `is_virtual` y `access_url`.
  2. **Crear un evento virtual con enlace** desde el panel e inscribirse: la respuesta de
     `GET /api/me/registrations` trae `accessUrl` y **`qrData` vacío**.
  3. **Crear uno presencial** e inscribirse: trae `qrData` y **`accessUrl` vacío**.
  4. **Inscribirse a uno de pago sin pagar:** ni `qrData` ni `accessUrl`.
  5. **Los eventos anteriores a la migración** siguen comportándose como presenciales.
  6. **Al inscribirse a un evento gratuito llega el correo**, con el enlace si es virtual y con la
     remisión a "Mi credencial" si es presencial.
  7. **Con el servicio de correo caído**, la inscripción se crea igual y el fallo solo aparece en el
     log del servidor.
  8. **Un evento virtual sin enlace cargado:** el correo dice que el enlace llegará antes de la sesión.

---

### [2026-08-18]: El perfil "miembro de junta" ya se puede registrar

- **El problema:** el onboarding de la app ofrece tres perfiles y el tercero, "Quiero ser miembro de
  junta o consejo", manda `role=junta`. El enum `core.user_role` solo tenía `familia`, `empresa` y
  `profesional`, así que ese registro moría en el INSERT con
  `invalid input value for enum core.user_role: "junta" (SQLSTATE 22P02)`. Visto en un iPhone el
  2026-08-18 (`docs/ios/error_18-08-2026.jpeg`, en el repo de la app).
- **Un tercio del onboarding no podía crear cuenta.** Las otras dos opciones sí funcionaban, que es
  por lo que esto sobrevivió sin que nadie lo notara.
- **Alcance:**
  - `scripts/20260818_add_junta_user_role.sql` — nuevo. `ALTER TYPE core.user_role ADD VALUE`.
  - `internal/core/domain/user.go` — `UserRoles`, `RoleDefault` e `IsValidRole`.
  - `internal/handler/http/user_handler.go` — valida el rol en `Register` y en `performUpdate`;
    `Register` deja de devolver `err.Error()`.
  - **No hay rutas nuevas.**
- **Se añade el valor en vez de mapear `junta` a `profesional`,** que era la alternativa barata y no
  habría necesitado migración. La app ya trata `junta` como un perfil propio: el saludo de la home y
  el texto del destacado semanal tienen su rama (`home_content_screen.dart`, líneas 40 y 299).
  Mapearlo habría dejado esas dos ramas muertas sin que nada avisara.
- **`profesional` se conserva** aunque no lo use ni la app ni el backend: el panel lo ofrece en el
  desplegable y puede haber cuentas creadas con él. Quitarlo de un enum obliga a recrear el tipo.
- **La validación cubre los dos caminos, no solo el registro.** `performUpdate` vuelca el JSON del
  cliente sobre el usuario ya cargado, así que un `PUT /api/users/{id}` con un rol inventado llegaba
  al mismo 22P02.
- **`Register` ya no devuelve `err.Error()` al cliente.** Ese `500` con el texto crudo de pgx es lo
  que le enseñó el SQLSTATE al usuario en la captura; ahora el detalle va al log del servidor y la
  app recibe un mensaje genérico. El `409` de usuario ya existente se mantiene igual.
- 🔒 **`performUpdate` deja de volcar el usuario entero en el log.** La línea `// Debug logging` con
  `%+v` escribía en cada edición de perfil el `password_hash` de bcrypt, el correo cifrado y el
  `email_blind_index` —el HMAC del que depende el inicio de sesión por correo, el mismo que hubo que
  recalcular al rotar la `encryption_key`—. Se vio en el log al probar el criterio 6. Ahora solo
  registra el id. Es anterior a este cambio, no una regresión suya. Revisados los demás `log.Printf`
  del backend: no hay otro que vuelque un struct.
- ⚠️ **La migración no se ha aplicado en ningún sitio todavía.** Docker no estaba levantado al hacer
  el cambio, así que no se probó contra una base real. Hay que aplicarla en local y en producción
  **antes** de desplegar el backend: si sale el binario nuevo sin la migración, `junta` sigue
  fallando, solo que ahora con un 400 en vez del SQLSTATE.
- ⚠️ **`ALTER TYPE ... ADD VALUE` no admite transacción** en PostgreSQL anterior a 12, y en 12+ el
  valor nuevo no se puede usar en la misma transacción que lo crea. Aplicar con psql directamente,
  sin envolver.
- **Verificado:** `go build ./...`, `go vet ./...` y `go test ./internal/core/...` en verde.
- **Criterios de QA:**
  1. **Aplicar la migración** y comprobar el enum:
     `select enumlabel from pg_enum e join pg_type t on t.oid=e.enumtypid where t.typname='user_role'`
     devuelve cuatro valores, con `junta` entre ellos.
  2. **Registrarse desde la app** por "Quiero ser miembro de junta o consejo": la cuenta se crea y no
     aparece ningún error de conexión.
  3. **Iniciar sesión con esa cuenta:** la home saluda con "Su perfil de gobierno crece." y el
     destacado semanal menciona la masterclass del consejero independiente.
  4. **`POST /api/users/register` con `"role": "inventado"`** responde **400** con "Perfil de usuario
     no válido", no 500 ni un texto de Postgres.
  5. **`POST /api/users/register` sin `role`** sigue creando la cuenta como `familia`.
  6. **`PUT /api/users/{id}` con un rol inventado** responde 400; con `junta`, 200.
  7. **En el panel**, editar un usuario: el desplegable "Rol" ofrece "Miembro de junta o consejo" y
     al guardar se conserva.
  8. **Las cuentas ya existentes** (`familia`, `empresa`) siguen entrando y editándose sin cambios.
  9. **Editar el perfil y mirar la ventana del servidor:** la traza dice `Updating user <id>` y nada
     más. No debe aparecer `PasswordHash`, `EmailEncrypted` ni `EmailBlindIndex`.

---

### [2026-08-14]: Un mensaje de chat ya avisa por push (y llega en vivo)

- **El problema:** el chat era el único módulo que no notificaba nada. Crear un evento y publicar
  contenido avisaban al tópico `all` desde el 6 de agosto, pero un mensaje no avisaba a nadie: la
  conversación solo avanzaba si la otra persona abría la app por su cuenta. Era lo único que le
  faltaba al módulo de notificaciones frente al documento de alcance.
- **Y había un segundo agujero, encontrado al tocarlo:** la app envía los mensajes por REST y el
  handler HTTP **nunca los repartía por WebSocket** —solo lo hacía el camino WS, que la app usa solo
  para recibir—. El destinatario no veía nada hasta recargar, aunque tuviera el chat abierto delante.
- **Alcance:**
  - `internal/core/services/chat_avisos.go` — nuevo. Arma y manda el aviso.
  - `internal/core/services/chat_service.go` — `SendMessage` dispara el aviso; el constructor recibe
    el servicio de notificaciones.
  - `internal/core/services/notification_service.go` y `internal/core/ports/notification_ports.go` —
    `SendToUser`, envío directo a los dispositivos de una persona.
  - `internal/handler/http/chat_handler.go` — reparte por el hub tras guardar.
  - `internal/infrastructure/websocket/hub.go` — entrega no bloqueante.
  - `cmd/server/main.go` — el chat recibe el servicio de notificaciones. **No hay rutas nuevas.**
- **El aviso va a los dispositivos del destinatario, no al tópico `all`.** Un mensaje privado no le
  interesa a la comunidad, y por tópico habría llegado a todos los teléfonos.
- **No escribe en el historial de notificaciones**, que es la bitácora de lo que manda un
  administrador desde el panel. Un mensaje de chat no lo manda nadie del panel: anotarlo sepultaría
  los envíos reales bajo miles de filas y exigiría un `admin_id` que no existe. Por eso `SendToUser`
  es un método aparte y no un `targetType` más de `SendNotification`.
- **El aviso sale después de guardar, en su propia goroutine y sin devolver error nunca**, igual que
  los avisos de novedades. Si FCM está caído —o corre en modo mock por falta de
  `firebase-service-account.json`— el mensaje se guarda y se entrega igual. Lleva contexto propio con
  15 s de límite: el de la petición HTTP se cancela al responder al remitente y cortaría el envío a
  medias.
- **El título es el nombre de quien escribe, descifrado**; el cuerpo, el mensaje recortado a 140
  caracteres. **El recorte cuenta caracteres, no bytes**: en un chat abundan tildes y emojis, y
  cortar a mitad de un carácter deja un rombo negro en la notificación.
- **La entrega por el hub ya no puede tumbar la petición:** antes escribía directo en el canal del
  cliente, que bloquea si la cola está llena y **hace panic si el cliente acaba de desconectarse**
  —con el proceso entero detrás—. Ahora es un envío no bloqueante con recuperación: perder la entrega
  en vivo no importa, el mensaje está guardado y la push avisa igual.
- **Sin migración de base de datos.**
- **Verificado:** `go build ./...`, `go vet ./...` y `go test ./...` en verde salvo
  `TestExhaustiveUserUpdate`, que falla desde siempre por su cadena de conexión escrita a mano.
  **9 tests nuevos** en `chat_avisos_test.go`.
- **Criterios de QA:**
  1. **Con dos cuentas y la app cerrada en la segunda:** escribir desde la primera. En el otro
     teléfono llega una notificación con el **nombre de quien escribe** como título y el texto del
     mensaje debajo.
  2. **Tocarla** abre esa conversación, no la bandeja, y el historial se ve completo.
  3. **Responder desde ahí:** la notificación llega en sentido contrario, al que escribió primero.
  4. **Con la conversación abierta en pantalla:** el mensaje aparece solo, sin recargar —esto es lo
     que arregla el reparto por WebSocket— y **no** salta ningún aviso encima.
  5. **Con la app abierta en otra pantalla:** aparece el aviso con el botón **Ver**, que lleva a la
     conversación.
  6. **Con un mensaje largo** (más de 140 caracteres) y con emojis: la notificación lo corta con
     puntos suspensivos y no muestra ningún carácter roto.
  7. **Un mensaje a alguien que nunca abrió la app en un teléfono** no falla: el mensaje se envía y
     en el log queda que no había dispositivos registrados.
  8. **Con FCM en modo mock** (sin `firebase-service-account.json`): el chat funciona igual y el
     aviso aparece en consola.

### [2026-08-13]: Los mensajes de Contáctenos ya no se pierden — bandeja en el panel

- **El problema:** la pantalla se estrenó hoy enviando solo un correo, como los otros dos canales.
  Eso dejaba tres agujeros: **si el SMTP fallaba el mensaje se perdía entero**, nadie podía ver en el
  panel qué se había preguntado ni si algo quedó sin responder, y no había forma de frenar a quien
  escribiera en bucle porque no se sabía cuántos mensajes llevaba.
- **Migración:** `scripts/20260813_contacto_mensajes.sql`, idempotente —comprobado aplicándola dos
  veces seguidas—. Tabla `core.contact_messages` con dos índices: uno para la bandeja y otro para
  contar los envíos recientes de una persona.
- **Alcance:**
  - `internal/core/domain/contacto.go`, `internal/core/ports/contacto_ports.go`,
    `internal/adapter/storage/postgres/contacto_repository.go` — nuevos.
  - `internal/core/services/contacto_service.go` — guarda, cifra y limita la frecuencia.
  - `internal/handler/http/contacto_handler.go` — `Listar` y `CambiarEstado`.
  - `cmd/server/main.go` — `GET /api/admin/contacto` y `PATCH /api/admin/contacto/{id}`, ambas bajo
    `AdminOnly`.
  - Panel: `core/models/contacto.model.ts`, `core/services/contacto.service.ts`,
    `features/admin/contacto/` y la entrada "Mensajes de Contacto" en el menú.
- **El orden importa y es la razón de la tabla: se guarda ANTES de intentar el correo.** Si el envío
  falla, el mensaje queda con `email_enviado = false` y la bandeja lo destaca en rojo; **la persona
  recibe confirmación porque su mensaje sí llegó**. Antes ese caso devolvía error y perdía el texto.
  Si lo que falla es guardar, entonces sí se avisa: sin base ni correo, el mensaje se perdería.
- **Asunto y cuerpo van cifrados** (AES-256), el mismo trato que los mensajes de chat: es texto libre
  y puede contener cualquier cosa. **Verificado en producción**: la bandeja los muestra en claro y un
  `SELECT` directo devuelve base64.
- **El remitente no se copia a la tabla:** se guarda `user_id` y el nombre y el correo se leen de
  `core.users`. Duplicar datos personales sería tener el mismo dato en dos sitios y que uno envejezca.
  **Nombre y apellido se devuelven separados** porque están cifrados por separado: unirlos antes de
  descifrar produce una cadena que ya no se puede descifrar.
- **Límite de frecuencia:** 5 mensajes por hora y persona, con **429** en vez de 400 para que la app
  pueda decir "espera un momento". No es contra un ataque —hace falta sesión— sino contra el envío
  repetido por nervios o por un botón que se queda pulsado.
- **Verificado en producción, circuito completo:** enviar desde la API guarda y entrega el correo
  (`email_enviado: true`), la bandeja lo devuelve descifrado con su remitente, la base lo guarda
  cifrado, y el panel responde 200 en `/admin/contacto` recargando la subruta. El mensaje de prueba
  se borró después: la bandeja queda en 0.
- **Respaldos previos:** `backup_20260813_precontactomensajes.sql.gz`,
  `server_linux.bak.20260813_precontactobandeja` y `dist.bak.20260813_contacto` en el frontend.
- **10 tests** en el backend y **6** en el panel.
- **Criterios de QA:**
  1. **Enviar un mensaje desde la app** y verlo aparecer en *Panel → Mensajes de Contacto* como Nuevo.
  2. **Abrirlo**: se despliega el texto y pasa solo a Leído.
  3. **Responder por correo**: abre el cliente con el destinatario y "Re: asunto", y queda Respondido.
  4. Cambiar entre los filtros Nuevos / Leídos / Respondidos / Todos.
  5. **Enviar 6 mensajes seguidos**: el sexto avisa de que hay que esperar, y solo se guardan 5.
  6. Comprobar que el correo sigue llegando a `soporte@legacynetworkco.com` y que al responderlo va
     al remitente (viaja en `Reply-To`).

### [2026-08-13]: Contáctenos — buzón de soporte desde la app

- **Por qué:** "Contactenos" es una de las seis pantallas del módulo de Autenticación en *Grandes
  Grupos Funcionales V1.0* y era la única del módulo sin implementación. Existían
  `POST /api/board/contact` y `POST /api/asesoria/request`, pero van a destinatarios distintos —un
  miembro de junta y el buzón de asesorías—, así que no servían como contacto general.
- **Alcance** (corte vertical completo, en el orden del CLAUDE.md):
  - `internal/core/ports/contacto_ports.go` — nuevo.
  - `internal/core/services/contacto_service.go` — nuevo. Valida y delega en el correo.
  - `internal/infrastructure/email/{smtp,gmail}_service.go` — `SendContactoEmail` en **los dos**
    implementadores. El que corre en producción es `GmailService`; olvidar el segundo rompe la
    compilación, que es la forma buena de enterarse.
  - `internal/handler/http/contacto_handler.go` — nuevo.
  - `internal/config/config.go` — `contacto_email` y `BuzonDeContacto()`.
  - **`cmd/server/main.go` — la ruta `POST /api/contacto` registrada.** Sin esto el handler existe y
    no se puede llamar, que es el fallo que ya ocurrió con `UploadImage`.
- **El remitente no viaja en el cuerpo**, se saca del perfil del token. Aceptarlo del cliente
  permitiría escribir al buzón de soporte haciéndose pasar por cualquiera.
- **`contacto_email` cae a `board_contacts["default"]` si falta.** Así un despliegue con una
  configuración anterior sigue entregando los mensajes en vez de rechazarlos con el buzón vacío.
- **Verificado en producción, sin enviar un solo correo real:** las tres validaciones responden 400
  con su motivo —mensaje vacío, mensaje demasiado largo, asunto demasiado largo— y sin token da 401.
  Se usó una cuenta semilla de pruebas, no la de una persona.
- **Corregido tras la primera verificación:** el handler consultaba el perfil antes de validar el
  mensaje, así que un mensaje en blanco gastaba una consulta a la base, y si el perfil fallaba
  devolvía `failed to fetch sender profile`, un texto interno en inglés. Ahora valida primero y el
  error dice qué hacer.
- **6 tests nuevos** en `contacto_service_test.go`: buzón correcto, asunto vacío sustituido por uno
  por defecto, mensaje en blanco y pasado de largo rechazados, falta de configuración, y que el fallo
  del envío se propaga en vez de tragarse el mensaje.
- **Criterios de QA** (necesita un build de la app):
  1. **Perfil → Contáctenos**: escribir asunto y mensaje, enviar. Llega a `soporte@legacynetworkco.com`
     con el nombre y correo de quien escribe, y se puede **responder directamente** (va en `Reply-To`).
  2. **Home → tarjeta Contáctenos**: abre la misma pantalla y se puede volver.
  3. **Enviar con el mensaje vacío**: avisa sin llamar al servidor.
  4. **Sin conexión**: muestra el error y **conserva lo escrito**, ofreciendo "Escribir por correo".
  5. Tocar **Correo** abre la app de correo con asunto y mensaje ya puestos.

### [2026-08-13]: El algoritmo de firma del JWT lo fijamos nosotros, no el token

- **El problema:** los tres middlewares llamaban a `jwt.Parse` sin `WithValidMethods`, así que el
  algoritmo con el que se verificaba la firma lo elegía el propio token. Es la puerta del clásico
  `alg: none` y de la confusión de algoritmo.
- **No era explotable, y conviene decirlo con precisión:** la `keyfunc` devuelve `[]byte`, y con eso
  la librería ya rechazaba tanto `none` como RS256. Pero esa protección era un detalle de
  implementación de la dependencia, no una decisión nuestra: una actualización que cambiara ese
  comportamiento lo habría abierto sin que nadie tocara este código.
- **Alcance:** `internal/handler/http/middleware.go` (`AuthMiddleware` y `OptionalAuthMiddleware`) y
  `internal/handler/http/admin_middleware.go` (`AdminOnly`), los tres con
  `jwt.WithValidMethods([]string{"HS256"})`. El validador de Apple ya lo hacía así desde el día 12.
- **Verificado en producción** tras desplegar, con tokens firmados dentro del servidor:
  - HS256 legítimo → **200** (no se rompió el camino bueno, que es el riesgo real de este cambio)
  - `alg: none` sin firma → **401**
  - `alg: RS256` firmado con HMAC → **401**
  - `/health`, `/api/events` y el panel → 200
- `go test ./internal/handler/http` **no se puede ejecutar en este equipo**: el Control de
  aplicaciones de Windows bloquea el binario de test. `go vet` y `go build` sí pasan, y el resto de
  la suite está en verde. Es un problema del entorno, no del paquete.
- **Limpieza del servidor, en la misma sesión:** se liberaron **223 MB** en `/docker/legacy`
  —tres `server_linux.bak` superados, el binario huérfano `legacy_backend` de julio y cinco
  `config.docker.yaml.bak` que ya no abren nada—. El disco pasó del 75% al 72%. **Se conservaron a
  propósito** los siete respaldos de la base, `server_linux.bak.20260813_middleware` (para revertir
  este despliegue) y `config.docker.yaml.bak.20260813_prerecifrado`, que es el único archivo capaz de
  leer `backup_20260813_prerecifrado.sql.gz`.
- **Criterios de QA:** los de siempre; este cambio no altera ningún flujo visible. Basta con
  comprobar que se puede **iniciar sesión en la app y en el panel**, que es lo que ejercita los tres
  middlewares.

### [2026-08-13]: Rotada la clave de cifrado en reposo, con recifrado de la base

- **Por qué:** `encryption_key` seguía siendo la clave de ejemplo, la misma desde el primer día y
  débil por construcción. A diferencia del `jwt_secret`, rotarla no es editar un YAML: los datos
  cifrados dejarían de leerse y **no habría vuelta atrás**.
- **Alcance:**
  - `cmd/recifrar/` — nuevo, comando de un solo uso. Lee con la clave vieja y escribe con la nueva
    **en una sola transacción**: o se rota todo, o no se toca nada. Cubre las nueve columnas cifradas
    de `core.users`, `chat.messages.content_encrypted` y los tres campos de contacto de
    `events.registrations`.
  - `.gitignore` — `/recifrar_linux`, para que el binario no acabe en el repositorio.
  - `DESPLIEGUE.md` — el procedimiento completo, ya ejecutado.
- **Lo que hace distinto a este comando de "descifrar y volver a cifrar":** `email_blind_index` no es
  un cifrado sino un HMAC con esa misma clave, y es **por donde `Login` busca al usuario**. Sin
  recalcularlo, la base habría quedado intacta y legible y **nadie habría podido iniciar sesión por
  correo**: la búsqueda no encuentra a nadie y responde credenciales inválidas. El índice se calcula
  sobre el correo **tal cual**, sin `ToLower` ni `TrimSpace`, porque `Login` tampoco normaliza
  (`auth_service.go:325`); normalizarlo habría dejado fuera a quien se registró con una mayúscula.
- **Ocho valores estaban en texto plano**, de dos cuentas semilla del 27 de febrero. Se comprobó que
  lo eran **sin exponerlos**, sustituyendo letras por `x` y dígitos por `9` en SQL: la forma
  `xxxxxxxxx_9999999999999999999` no puede ser base64. Cifrarlos **arregló un fallo de paso**:
  `auth_service.go:433` descarta el error de `Decrypt` pero asigna el resultado igual, y `Decrypt`
  devuelve cadena vacía al fallar, así que esos dos usuarios **se mostraban sin nombre en la app**.
- **Ejecución:** respaldo verificado con `gunzip -t` (36 tablas, 13 usuarios), backend parado,
  simulacro, y aplicación. Movidos **74 valores recifrados + 8 desde texto plano + 13 índices
  ciegos**; 35 campos vacíos se dejaron vacíos a propósito. Las 5 inscripciones no tenían datos de
  contacto todavía.
- **Verificado en tres niveles:** el comando relee todo con la clave nueva y comprueba cada índice
  **antes** de confirmar; el backend arranca; y `GET /api/users` con un token firmado dentro del
  servidor devuelve **13 usuarios, 13 correos con `@` y ningún campo ilegible** — es decir, el
  circuito entero, no solo la base.
- **Para revertir:** `backup_20260813_prerecifrado.sql.gz` y
  `config.docker.yaml.bak.20260813_prerecifrado`, ambos en `/docker/legacy`. Hacen falta **los dos**:
  el respaldo sin la clave vieja no sirve de nada.
- **Criterios de QA:**
  1. **Iniciar sesión en la app** con correo y contraseña: entra. Es la prueba del índice ciego.
  2. **Abrir un chat con historial**: los mensajes anteriores se leen.
  3. **Ver el perfil propio y el de otro miembro**: nombre, empresa y cargo legibles.
  4. **En el panel**, el listado de usuarios muestra nombres y correos, no cadenas de letras.
  5. **Registrar una cuenta nueva** y entrar con ella: cubre el camino de escritura con la clave
     nueva, no solo el de lectura.
  6. **Editar el perfil** y volver a abrirlo: lo guardado se relee bien.

### [2026-08-13]: Producción firmaba los JWT con el secreto de ejemplo — 🔴 cualquiera era admin

- **El problema:** `/docker/legacy/config.docker.yaml` tenía `jwt_secret:
  "super-secret-jwt-key-change-me"`, el placeholder que trae el repositorio. El secreto es lo único
  que separa un token legítimo de uno inventado, así que **cualquiera que conociera ese valor podía
  firmarse un token con `role: admin` y entrar a las rutas de administración**. No hacía falta
  ninguna credencial ni ningún fallo adicional.
- **Comprobado contra producción antes de rotar:** un token HS256 firmado a mano con ese secreto,
  con `sub` inventado y `role: admin`, obtuvo **200 en `GET /api/admin/stats/dashboard`**. La misma
  petición sin token daba 401.
- **Alcance:** solo configuración; no se tocó código.
  - `config.docker.yaml` (servidor y copia local) — `jwt_secret` nuevo, 96 caracteres de
    `openssl rand -hex 48`, generado en el servidor para que no pasara por ningún registro.
  - Imagen `legacy-backend` reconstruida. **El `--build` no es opcional:** el `Dockerfile` hace
    `COPY config.docker.yaml /app/config.yaml`, así que el archivo viaja dentro de la imagen y un
    `restart` habría dejado el secreto viejo en marcha.
  - Respaldo previo: `config.docker.yaml.bak.20260813_prerotacion`.
- **Efecto para los usuarios:** todas las sesiones abiertas quedan invalidadas. La app y el panel
  piden iniciar sesión otra vez, una sola vez. No se tocó ningún dato.
- **Verificado tras el despliegue:** el token forjado con el secreto viejo pasa a **401**; `/health`
  200, `GET /api/events` 200, el panel 200, y `POST /api/admin/login` con credenciales malas
  responde 401 (no 500, que delataría un arranque a medias).
- **Segundo hallazgo, sin relación con lo anterior:** la copia local de `config.docker.yaml` estaba
  **desincronizada** del servidor —le faltaba la sección `apple:`, añadida directamente allí el día
  12—. Como el despliegue sube el archivo local por `scp`, **el siguiente despliegue habría borrado
  `apple.bundle_id` y dejado a todo el mundo fuera de Sign in with Apple**, sin cambiar una línea de
  código y sin error visible en el arranque. Ambas copias quedan idénticas
  (`sha256 38681c06…`) y en `DESPLIEGUE.md` queda el paso de comparar hashes antes de subir.
- ⚠️ **`encryption_key` sigue con el valor de ejemplo.** Rotarla es otra tarea, no un cambio de
  configuración: hay que descifrar y volver a cifrar nueve columnas de `core.users`, los mensajes de
  chat y el contacto de los inscritos, y **recalcular `email_blind_index`**, que es un
  HMAC con esa misma clave y del que depende el inicio de sesión por correo. Sin ese recálculo nadie
  puede entrar. Queda pendiente por decisión, con el procedimiento documentado.
- **Criterios de QA:**
  1. **Iniciar sesión en el panel** con un administrador real: entra, y el listado de usuarios carga.
  2. **Abrir la app con la sesión que ya estaba iniciada**: debe pedir iniciar sesión de nuevo —eso
     es el efecto esperado, no un fallo— y funcionar con normalidad tras entrar.
  3. **Chat y notificaciones** después de volver a entrar: siguen operando.
  4. Repetir el punto 2 en un segundo dispositivo para confirmar que la expulsión fue general.

### [2026-08-12]: Sign in with Apple no validaba nada — 🔴 agujero de autenticación

- **El problema:** `SocialLogin` no comprobaba el token de Apple. El código lo decía sin rodeos —
  `// Mock for now until Apple verification logic is set`— y devolvía siempre
  `user_apple@example.com` / "Apple User", **ignorando el `id_token`**.
- **Comprobado contra producción antes del arreglo:** un `POST /api/auth/social-login` con
  `{"provider":"apple","id_token":"token-inventado-de-diagnostico"}` respondía con esa identidad. Las
  consecuencias, en orden: **(1)** nadie podía entrar con su cuenta de Apple, porque todos colapsaban
  en el mismo correo ficticio; **(2)** la app mandaba a registro con ese correo; **(3)** en cuanto
  alguien completara ese registro, cualquiera con cualquier cadena habría obtenido un JWT válido de
  esa cuenta. La cuenta no existía —el 404 lo confirmó—, así que el agujero **no llegó a abrirse**.
- **También estaba a medias el vínculo social:** `core.users` no tenía `google_id` ni `apple_id`. El
  dominio los declaraba y la API los devolvía como `null`, pero no existían; en el código quedaba un
  `// Update DB with google ID (dummy update here)`.
- **Migración:** `scripts/20260812_identidad_social.sql`, idempotente. Dos columnas y sus índices
  únicos parciales, para que dos cuentas no puedan reclamar la misma identidad.
- **Alcance:**
  - `internal/infrastructure/apple/validator.go` — nuevo. Verifica firma contra las claves públicas
    de Apple (JWKS con caché de 6 h), emisor, audiencia y caducidad; **exige** las tres. Solo acepta
    RS256: admitir otro algoritmo abriría la puerta al clásico `alg: none`.
  - `internal/core/ports/interfaces.go` — `ValidadorDeApple`, `FindBySocialID`, `LinkSocialID`.
  - `internal/core/services/auth_service.go` — valida de verdad y busca **por el `sub`**, no por
    correo: Apple solo manda el correo en el primer inicio de sesión y puede ser de retransmisión
    privada. Si falta configuración, **rechaza** en vez de dejar pasar.
  - `internal/adapter/storage/postgres/user_repository.go` — el proveedor se traduce a columna con un
    `switch`, no interpolando en el SQL: llega en el cuerpo de una petición pública.
  - `internal/config/config.go` y ambos `config*.yaml` — `apple.bundle_id`.
  - App: el botón de Apple **solo se muestra en iOS y macOS**; en Android requiere un Service ID que
    no está configurado y solo llevaba a un error.
- **Verificado:** **19 tests** nuevos —13 del validador, 6 del inicio de sesión—, entre ellos los
  nueve tokens que deben rechazarse: sin firma válida, de otra app, de otro emisor, caducado, sin
  caducidad, sin sujeto. Toda la suite en verde salvo los dos fallos conocidos y ajenos.
- **Desplegado y comprobado en producción el mismo día:** la migración primero, luego el binario. El
  token inventado que antes devolvía identidad ahora da **401 "Credenciales inválidas de red
  social"**. `/health` 200, eventos 200, el panel 200, el inicio de sesión con correo y contraseña
  sigue dando 200, y Google con un token falso, 401. Respaldos:
  `backup_20260812_preidentidadsocial.sql.gz`, `server_linux.bak.20260813_0413` y
  `config.docker.yaml.bak.20260813_0413`.
- **Criterios de QA** (hace falta un iPhone y un build nuevo):
  1. **Entrar con Apple** en iOS por primera vez: si no hay cuenta, lleva al registro con el correo
     que dé Apple —o vacío, si eligió ocultarlo—, no con `user_apple@example.com`.
  2. **Completar el registro** y volver a entrar con Apple: reconoce la cuenta **sin** pedir nada más.
  3. **Salir y volver a entrar** una tercera vez, que es cuando Apple ya no manda el correo: debe
     seguir reconociéndola por su identidad.
  4. **En Android**, el botón de Apple **no aparece**.
  5. **El inicio de sesión con correo y contraseña** y el de Google siguen funcionando.
  6. **Una cuenta que ya existía por correo** y entra por primera vez con Apple con ese mismo correo:
     debe enlazarse a la cuenta existente, no crear otra.

---

### [2026-08-12]: Los datos del participante y el método de pago dejan de tirarse

- **El problema:** la pantalla de pago pedía nombre, correo y teléfono del participante, los
  validaba desde el 2026-08-05… y los descartaba, porque **ninguna ruta los aceptaba**. Quien
  organiza un evento no tenía a quién llamar si alguien no aparecía. El selector de tarjeta o PSE
  tampoco viajaba: elegir uno u otro no cambiaba absolutamente nada.
- **Migración:** `scripts/20260812_datos_participante_y_metodo_pago.sql`, idempotente. Añade
  `participant_name`, `participant_email` y `participant_phone` a `events.registrations`, y
  `payment_method` a `core.transactions`.
- **Alcance:** `domain/event.go` y `domain/transaction.go`, `handler/http/event_handler.go`,
  `services/event_service.go` (cifrado), `services/payment_service.go`, y los repositorios de
  eventos y transacciones. En la app: `event_payment_screen.dart`, `event_service.dart`,
  `payment_service.dart` y `events_provider.dart`.
- **El contacto va cifrado**, como el resto de datos personales: lo cifra `EventService` con
  `CryptoService` al escribir. Una consulta directa a esas columnas devuelve texto cifrado, no el
  nombre. Vacío se queda vacío —significa "usa los del perfil"— en vez de guardar cadenas cifradas
  que no dicen nada.
- **No cambia de quién es la entrada.** El titular sigue siendo el del token: estos campos son el
  contacto para ese evento, que puede diferir del perfil. No reabren el agujero de inscribir a
  terceros que se cerró el 2026-08-05.
- **El método de pago es informativo** y se filtra en el servidor: solo se guardan `credit_card` y
  `pse`, y cualquier otro valor se descarta en silencio en vez de tumbar la compra. Quien decide los
  medios disponibles es la pasarela; esto sirve para soporte y para saber si PSE se usa lo bastante
  como para integrarlo de verdad.
- **Verificado:** `go build`, `go vet` y los tests del backend en verde; en la app, **117 tests**,
  con 5 nuevos que fijan que los campos viajan y que los vacíos no se envían.
- **Migración aplicada en producción el 2026-08-12.** Respaldo previo en el servidor:
  `backup_20260812_premigracion.sql.gz` (25 KB). Las cuatro columnas existen y son `NULL`ables; la
  **idempotencia se comprobó aplicándola dos veces**, la segunda sin errores. Los datos quedaron
  intactos —13 usuarios, 5 inscripciones, 11 transacciones— y la API siguió respondiendo durante
  todo el proceso: `/health` 200, `/api/events` 200, el panel 200 y el login con credenciales falsas
  401.
- **Binario desplegado el mismo día, después de la migración.** Respaldos previos en el servidor:
  `server_linux.bak.20260813_0313` y `config.docker.yaml.bak.20260813_0313`. Se subió **solo el
  binario**: ni `docker-compose.yml` ni `config.yaml`, que son los de desarrollo.
- **Verificado tras recrear el contenedor:** el binario en ejecución (`/app/server` dentro del
  contenedor) coincide **byte a byte** con el compilado, 58.645.567; el log de arranque muestra
  "Connected to Database" y el puerto 8080, **sin el aviso de pasarela simulada** —el modo está
  apagado en producción, como debe—; `/health` 200, `/api/events` 200, el panel 200, el login con
  credenciales falsas 401, la subida de imágenes sigue protegida con 401, y
  `/api/payments/simulado/...` responde **404**: esas rutas no se registran con el modo apagado.
- **Con esto entra también el cambio del payload de CredibanCo.** No arregla el bloqueo —la pasarela
  sigue devolviendo "acceso denegado"— pero tampoco lo empeora, y deja la llamada alineada con la
  implementación que sí funciona para cuando lleguen credenciales válidas.
- **Criterios de QA** (con la migración aplicada y la pasarela simulada activa):
  1. **Comprar un evento de pago** con los tres campos rellenos y aprobar: la inscripción se crea.
  2. **En la base**, `SELECT participant_name FROM events.registrations` devuelve **texto cifrado**,
     no el nombre. Si se lee en claro, el cifrado no se aplicó.
  3. **Elegir PSE** y comprobar que `core.transactions.payment_method` guarda `pse`.
  4. **Dejar los campos como vienen** (prellenados del perfil) y comprobar que se guardan igual.
  5. **Inscribirse a un evento gratuito**, que no pasa por esta pantalla: debe seguir funcionando
     sin datos de participante.
  6. **Repetir una inscripción ya existente**: no debe duplicarse ni perder los datos anteriores.

---

### [2026-08-12]: Pasarela de pago simulada para poder probar sin CredibanCo

- **Por qué:** la pasarela real lleva bloqueada desde el 2026-08-06 y la prueba de hoy confirmó que
  no es cosa del payload. Sin ella no había forma de probar **el resto del recorrido**, que es donde
  estaban ocho de los diez fallos de agosto: crear la intención, salir al navegador, volver por el
  deep link, recibir la notificación y confirmar la inscripción.
- **Alcance:**
  - `internal/infrastructure/credibanco/gateway_simulado.go` — nuevo. Implementa el mismo puerto
    `ports.PaymentGateway`, así que el servicio y los handlers no se enteran. Sirve una pantalla con
    tres botones —aprobar, rechazar, dejar pendiente— y redirige al `returnUrl` como haría el banco.
  - `internal/config/config.go` — `credibanco.simulado` y `credibanco.simulado_base_url`.
  - `cmd/server/main.go` — elige la implementación y registra las rutas **solo** si el modo está
    encendido.
- **Tres capas de protección**, porque un modo así en producción es regalar inscripciones:
  1. **El backend no arranca** si `simulado` está activo con una `base_url` que no sea de pruebas, y
     la comprobación va **antes de abrir la base de datos**: no hay camino por el que llegue a
     servir peticiones.
  2. La comprobación es **por lista blanca** (`ecouat`, `localhost`, `127.0.0.1`, `sandbox`,
     `test`), no por lista negra del dominio real: si mañana el banco cambia de dominio, una lista
     negra dejaría de proteger sin que nadie se entere.
  3. Las rutas `/api/payments/simulado/...` **no se registran** con el modo apagado, y el arranque
     deja un aviso en el log imposible de pasar por alto.
- **Verificado:** `go build ./...` y `go vet ./...` limpios; **14 tests** del paquete pasan, entre
  ellos el recorrido completo en sus tres desenlaces y los 7 casos de la salvaguarda.
- ⚠️ **Lo que NO se pudo probar aquí:** el arranque real con la configuración peligrosa. La política
  de control de aplicaciones de este equipo bloquea cualquier binario recién compilado, así que la
  llamada desde `main.go` está revisada a ojo pero no ejecutada. La decisión que toma —
  `SimuladoEsPeligroso`— sí está cubierta por tests.
- **Criterios de QA** (con `simulado: true` y el backend en local):
  1. **Arranque:** el log muestra el aviso de pasarela simulada.
  2. **Con `base_url` de producción y `simulado: true`**, el backend **debe negarse a arrancar**.
  3. **Con `simulado: false`**, `/api/payments/simulado/loquesea` debe dar 404.
  4. **Comprar un evento de pago** desde la app: se abre la pantalla de prueba con el importe
     correcto.
  5. **Aprobar**: la app vuelve por el deep link, "Mi credencial" muestra la inscripción y el evento
     pasa a `confirmed`.
  6. **Rechazar**: la inscripción sigue en `pending_payment` y el evento se puede reintentar.
  7. **Dejar pendiente**: la app informa de que el pago está en proceso, sin inscribir.
  8. **Desde el teléfono**, con `simulado_base_url` apuntando a la IP del equipo, la pantalla carga:
     quien la abre es el navegador del teléfono, no el servidor.

---

### [2026-08-12]: El registro de pagos se alinea con el plugin que sí funciona

- **Por qué:** los pagos están bloqueados desde el 2026-08-06 con `errorCode 5`, "acceso denegado",
  incluso llamando a la pasarela desde el servidor sin pasar por el backend. Al analizar el plugin
  `woocommerce-rbspayment-credibanco 1.1.3` —que funciona **contra la misma pasarela y el mismo
  comercio**— aparece la diferencia: nosotros enviábamos tres parámetros que el plugin no envía.
- **La hipótesis:** en la API de RBS, sobre la que corre CredibanCo, `merchant` lo mandan los
  agregadores que facturan por terceros. Un comercio normal que lo incluye pide una operación para
  la que su usuario no tiene permiso, y eso responde `errorCode 5`. Encaja con lo que no cuadraba:
  el error salía igual desde otra IP y con un usuario inventado, cosa que ni una lista blanca ni una
  contraseña mala explicarían. En el plugin, además, la línea de `currency` está **comentada a
  propósito**.
- **Alcance** — solo `internal/infrastructure/credibanco/client.go`:
  - **Se dejan de enviar `terminal`, `merchant` y `currency`** en `register.do`. Siguen en la
    configuración porque identifican el comercio ante el banco, pero no viajan en la llamada.
  - Se añaden `language=es` y `jsonParams`, que identifica a la app en los registros del banco y
    permite distinguirla del WooCommerce del mismo comercio al reclamar.
  - **El importe pasa a calcularse sobre enteros** (`math.Round(amount*100)`) en vez de formatear a
    texto y quitarle el punto. Por la vía anterior, 25,55 llegaba como **2554**: en coma flotante ese
    número es 25,549999…
  - **`orderStatus = 1` (preautorizado) sigue siendo pendiente, no aprobado**, y ahora deja aviso en
    el log. Aquí se difiere del plugin a propósito: preautorizado es dinero retenido, no cobrado, e
    inscribir a alguien de quien no se ha cobrado es peor que hacerle esperar. El plugin puede
    aceptarlo porque ofrece el modo de dos pasos; nosotros registramos con `register.do`, donde ese
    estado no debería aparecer.
  - **No se copia el sufijo `_{timestamp}` del `orderNumber`**: WooCommerce lo necesita porque
    reutiliza el mismo `order_id` al reintentar, mientras que aquí cada intento crea una transacción
    nueva con su UUID. Añadirlo rompería `transaccionDeLaNotificacion`, que resuelve la referencia
    parseando un UUID.
- 🔴 **Probado contra UAT el mismo día: el `errorCode 5` PERSISTE.** Una única llamada a
  `register.do` en `ecouat.credibanco.com` con el payload ya corregido devolvió otra vez "Acceso
  denegado". Mismo entorno y mismas credenciales que el intento del 2026-08-06, con la única variable
  cambiada siendo el payload. **La hipótesis queda descartada:** no eran `merchant`, `terminal` ni
  `currency`.
- **Lo que eso deja en pie:** el usuario de API no tiene permiso para registrar pagos, o no es el que
  corresponde a este comercio. La prueba conserva su valor porque cierra la vía técnica: ya no hay
  nada más que ajustar en el código, y la reclamación a CredibanCo puede ser concreta.
- **La comprobación pendiente, y no cuesta ninguna llamada:** el WooCommerce de Legacy funciona
  contra esta misma pasarela, así que **su login de API sí es válido**. Nuestro `username` empieza
  por `LEG` y termina en `-api`; el del plugin se ve en WordPress, en los ajustes de la pasarela
  CredibanCo. Si no coinciden, nos entregaron un usuario que no sirve.
- **Los cambios de código se conservan igualmente**: el fallo del importe (25,55 → 2554) era real e
  independiente, y el resto alinea la llamada con la implementación que sí funciona.
- **`uat_manual_test.go`** queda tras la etiqueta `uat` para repetir la prueba cuando haya
  credenciales nuevas, sin riesgo de ejecutarla por accidente:
  `go test -tags uat -count=1 -v ./internal/infrastructure/credibanco/ -run TestUATRegistroReal`
- **Verificado:** `go build ./...` y `go vet` limpios; **7 tests** del paquete pasan, incluidos los
  5 nuevos. La suite completa pasa salvo dos fallos conocidos y ajenos: `TestExhaustiveUserUpdate`
  (cadena de conexión con el usuario `postgres`) y `internal/handler/http`, que en este equipo no
  puede ejecutarse porque una política de control de aplicaciones de Windows bloquea ese binario de
  prueba.
- **Criterios de QA** (cuando se haga la prueba de la fase 2):
  1. **Una sola llamada** a `register.do` contra `ecouat.credibanco.com`, guardando petición y
     respuesta completas.
  2. Si **desaparece el `errorCode 5`**, la causa era ésta: continuar con el plan.
  3. Si **persiste**, queda descartada la vía técnica y el problema son las credenciales — con un
     argumento concreto que darle a CredibanCo.
  4. **No repetir** la llamada más de una vez por sesión de pruebas.
  5. Cuando haya pago real: **importe mínimo** y comprobación en el extracto antes de abrir el cobro.

---

### [2026-08-12]: Postgres deja de estar publicado a Internet

- **El problema:** el 5432 respondía desde cualquier parte (`0.0.0.0:5432` y también IPv6) y el
  servidor no tiene `ufw` activo. Estaba anotado como hallazgo abierto desde el 2026-08-11 y se
  volvió a comprobar hoy desde fuera antes de tocar nada.
- **Qué se hizo:** en el compose **del servidor** —no el del repositorio, que es el de
  desarrollo—, `ports: - "5432:5432"` pasa a `- "127.0.0.1:5432:5432"`, y se recreó solo el
  contenedor `legacy_db`. Respaldo previo en `docker-compose.yml.bak.20260812_2304`; el `diff`
  contra él muestra esa única línea.
- **Por qué atarlo a local en vez de retirar la publicación:** quitarla del todo habría cerrado
  también pgAdmin, DBeaver y `psql`, que la guía advertía de no romper sin decidirlo. Con
  `127.0.0.1` siguen sirviendo por túnel SSH.
- **`ufw` no habría servido:** Docker escribe sus reglas en la cadena `DOCKER` de iptables, fuera
  del gobierno de `ufw`; un puerto publicado sigue abierto con el firewall activo.
- **Verificado desde fuera, después del cambio:** el 5432 da *Connection refused*; `docker ps`
  muestra `127.0.0.1:5432->5432/tcp`; `/health` 200; el panel 200; y
  `POST /api/admin/login` con credenciales falsas devuelve **401 `invalid credentials`**, que es la
  prueba de que el backend sigue consultando la base y no un 500 de conexión.
- **Sin pérdida de datos:** el volumen `legacy_db_data` no se tocó; solo se recreó el contenedor.
- **Criterios de QA:**
  1. **Entrar al panel** con un usuario real y ver que carga el listado de miembros.
  2. **Iniciar sesión en la app** y abrir eventos: los datos vienen de la base.
  3. **Desde fuera**, `psql -h legacy.intelyclick.com -p 5432` debe fallar por conexión rechazada.
  4. **Con túnel SSH** (`ssh -L 5432:127.0.0.1:5432 …`), pgAdmin o DBeaver deben conectar a
     `localhost:5432` con normalidad.
  5. **El respaldo diario de las 03:30** debe seguir generándose mañana.

---

### [2026-08-11]: Despliegue de la subida de imágenes — y un tropiezo con el compose

- **Desplegado y verificado en producción.** `go test ./...` se ejecutó dentro de un contenedor
  `golang:1.25-alpine`, porque en el equipo de desarrollo una política de Windows impide ejecutar
  binarios de Go. Pasan todos salvo `TestExhaustiveUserUpdate`, el fallo conocido de
  `test_update_test.go` (cadena de conexión con el usuario `postgres` en vez de `dba`).
- **Verificado contra el dominio público:** `/health` 200; `GET /api/images/noexiste.jpg` responde
  `{"error":"Image not found"}` —el JSON del handler, no el `404 page not found` de chi que devuelve
  una ruta inventada, así que la ruta está registrada—; `POST /api/images/upload` sin token da 401.
  El volumen `legacy_legacy_uploads` está montado en `/data/uploads` dentro del contenedor.
- ⚠️ **Se subió por error el `docker-compose.yml` del repositorio**, que es el de desarrollo y lleva
  `POSTGRES_PASSWORD: "123"`. **La contraseña efectiva de la base no cambió**: Postgres ignora esa
  variable cuando el volumen de datos ya está inicializado, y el backend siguió conectando con la
  original. Se restauró el compose de producción desde el respaldo, conservando el volumen nuevo, y
  se comprobó por hash que la contraseña volvió a ser la de antes. **`DESPLIEGUE.md` ya advierte de
  no subir ese archivo**, y el compose del repo lleva ahora un aviso en la cabecera.
- **Respaldos en el servidor** antes de tocar nada: `server_linux.bak.20260811_1846`,
  `config.docker.yaml.bak.20260811_1846` y `docker-compose.yml.bak.20260811_1846`.
- 🔴 **Hallazgo abierto, anterior a este despliegue:** el 5432 de Postgres está **publicado a
  Internet** (`0.0.0.0:5432`) y el servidor **no tiene firewall** (`ufw` inactivo). Comprobado desde
  fuera. La contraseña es fuerte, pero la base no debería ser alcanzable. El backend no necesita esa
  publicación: se conecta por `proxy-net`.
- **Sin probar todavía:** una subida real con sesión. Requiere el token de un usuario; queda para la
  prueba desde la app.

---

### [2026-08-11]: La subida de imágenes de los foros nunca estuvo enrutada

- **El problema:** `ImageHandler` existía completo —subir, servir, redimensionar— con sus dos tests,
  pero **nunca se instanció en `main.go` ni se registró ninguna de sus rutas**. Adjuntar una imagen a
  un hilo respondía 404 y la app lo mostraba como un fallo al subir. Es el caso que el `CLAUDE.md`
  advierte: el paso que más se olvida de los seis del corte vertical es registrar la ruta.
- **Dónde se guardaban:** el handler tenía `UploadDir = "/tmp"` fijo. Esa ruta **no existe en
  Windows** y **dentro del contenedor la borra cada despliegue**, porque el contenedor se recrea. Aun
  arreglando el enrutado, las imágenes habrían desaparecido solas.
- **Rutas bajo `/api/` además de la forma antigua:** en producción HAProxy solo enruta `/api/...` al
  backend, así que `/images/...` desde la raíz habría seguido sin llegar, igual que pasó con
  `/social-login`. Se registran las dos formas y **la app pasa a pedir `/api/images/...`**; la forma
  antigua queda como alias para los builds ya instalados.
- **Ver es público, subir exige sesión:** la app pinta la imagen con `Image.network`, que no manda
  `Authorization`, así que `GET` no puede estar tras el middleware. El nombre lo genera el servidor
  con un UUID y `GetImage` lo recorta con `filepath.Base`, de modo que no se puede adivinar ni salir
  del directorio. `POST` sí va con `AuthMiddleware`: sin eso, cualquiera podría llenar el disco.
- **Alcance:**
  - `cmd/server/main.go` — instancia `NewImageHandler(cfg.Storage.UploadsDir)` y registra
    `POST /api/images/upload` y `/images/upload` (privadas) y `GET /api/images/{fileName}` y
    `/images/{fileName}` (públicas).
  - `internal/handler/http/image_handler.go` — el directorio pasa a ser configurable; se corrige que
    **un archivo sin extensión rompía el guardado** (`imaging.Save` elige el formato por la
    extensión y devolvía `unsupported image format` como 500); se elimina el nombre de archivo que
    se calculaba dos veces.
  - `internal/config/config.go`, `config.yaml`, `config.docker.yaml` — `storage.uploads_dir`.
  - `docker-compose.yml` — volumen `legacy_uploads` en `/data/uploads`.
- **Sin migración.** No toca la base: `forum_posts.image_url` ya guardaba el nombre del archivo.
- ⚠️ **No se pudieron ejecutar los tests en el equipo de desarrollo:** una política de control de
  aplicaciones de Windows bloquea cualquier binario recién compilado, así que `go test` y `go run`
  no arrancan. Sí pasan `go build ./...` y `go vet ./...`. **Ejecutar `go test
  ./internal/handler/http/ -run TestImageHandler -v` antes de dar esto por bueno.**
- **Criterios de QA:**
  1. **Adjuntar una imagen a un mensaje de foro** desde la app: se sube y se ve en el hilo.
  2. **La imagen sigue viéndose al reabrir la app** y desde otra cuenta distinta a la que la subió.
  3. **Subir sin sesión** (sin cabecera `Authorization`) responde 401, no 200.
  4. **Un archivo que no sea imagen** —un PDF renombrado a `.jpg`— se rechaza con 400.
  5. **Un archivo de más de 10 MB** se rechaza con 400.
  6. **Una imagen ancha se reduce a 600 px**: subir una de 2000 px y comprobar el tamaño servido.
  7. **`GET /api/images/../algo`** no devuelve ningún archivo de fuera del directorio de subidas.
  8. **Tras recrear el contenedor** (`docker compose up -d --build backend`), las imágenes subidas
     antes **siguen viéndose**. Es la comprobación que verifica el volumen.
  9. **En el servidor**, `docker volume ls` muestra `legacy_uploads` y el directorio
     `/data/uploads` existe dentro del contenedor.

---

### [2026-08-10]: Bloquear y reportar personas — API (directriz 1.2 de Apple)

- **Por qué:** Apple exige, para toda app con contenido generado por usuarios, poder **reportar
  contenido y bloquear a quien abusa desde la propia app**. Esta tiene chat 1:1, foros y
  publicaciones, y no había nada: solo se podían reportar publicaciones de foro. Mensajería directa
  entre desconocidos sin bloqueo es uno de los rechazos más frecuentes de la App Store.
- **Esto es solo la API.** Faltan la interfaz en la app Flutter y la bandeja en el panel.
- **Alcance:**
  - `scripts/20260810_bloqueo_y_reporte_usuarios.sql` — **migración**: `core.user_blocks` y
    `core.user_reports`. Aplicada y probada dos veces: es idempotente.
  - `domain/block.go`, `ports/block_ports.go`, `postgres/block_repository.go`,
    `services/block_service.go`, `handler/http/block_handler.go`.
  - **Rutas registradas en `main.go`** —comprobadas contra el servidor en marcha, las seis dan 401 y
    no 404—:
    `GET/POST/DELETE /api/blocks`, `POST /api/users/{userID}/report`, y en el bloque `AdminOnly`
    `GET /api/admin/user-reports` y `PATCH /api/admin/user-reports/{reportID}`.
  - `Tests`: `bloqueo_usuarios_test.go` (9 casos).
- **El bloqueo filtra, que es la mitad que importa.** Se aplica en cuatro sitios, no solo al
  guardarse:
  1. `ListMembers` — el directorio excluye a los bloqueados **en ambas direcciones**. Si quien
     bloquea siguiera viendo a quien bloqueó, podría volver a invitarle.
  2. `ListConnections` — la conversación desaparece de la lista.
  3. `GetChatHistory` — **y también al pedirla directamente**: esconderla de la lista no basta,
     porque el `connectionID` sigue siendo válido.
  4. `SendMessage` y `SendInvite` — quien tenga la pantalla abierta cuando le bloquean no puede
     seguir escribiendo.
- **El bloqueo es dirigido pero simétrico en sus efectos:** ninguno de los dos ve ni escribe al
  otro. Un bloqueo que dejara al bloqueado seguir enviando no protegería de nada.
- **El mensaje de error es el mismo en las dos direcciones** ("no es posible contactar con esta
  persona"): decir "te han bloqueado" revelaría una decisión de la otra persona y avisaría a quien
  acosa de que le conviene cambiar de cuenta.
- **Los mensajes no se borran al bloquear.** Si se desbloquea, la conversación vuelve intacta.
- **De paso:** `ListMembers` tampoco excluía las cuentas eliminadas, así que las cuentas
  anonimizadas aparecían en el directorio como "Usuario eliminado". Ahora filtra por `deleted_at`.
- **Criterios de QA:**
  1. **Bloquear desde el chat:** la conversación desaparece de la lista de quien bloquea.
  2. **La otra persona tampoco ve la conversación** ni puede escribir en ella.
  3. **El directorio de miembros** deja de mostrar a la persona bloqueada, **a los dos**.
  4. **No se puede invitar** a alguien con quien hay bloqueo, en ninguna de las dos direcciones.
  5. **Abrir el historial a la fuerza** (con el id de la conversación) tampoco funciona.
  6. **Desbloquear devuelve todo:** la conversación reaparece **con sus mensajes intactos**.
  7. **Bloquear dos veces** a la misma persona no da error ni duplica nada.
  8. **Nadie puede bloquear en nombre de otro:** quien bloquea sale del token, no del cuerpo.
  9. **Reportar exige motivo**; el reporte aparece en `GET /api/admin/user-reports?status=pending`.
  10. **Las cuentas eliminadas no aparecen** en el directorio de miembros.

---

### [2026-08-10]: El registro con correo y contraseña devolvía 500

- **El problema:** `email_verification_repository.go` consultaba, insertaba y borraba por
  `email_blind_index` en `core.email_verification_tokens`, y **esa columna no existe**: la tabla
  identifica por `user_id`, con clave foránea a `core.users`. Las tres consultas del archivo iban
  contra un esquema que no es el real.
- **Qué se veía:** al registrarse con correo, **la cuenta sí se creaba**, pero guardar el token de
  verificación fallaba y la API respondía `500 ERROR: column "email_blind_index" does not exist
  (SQLSTATE 42703)`. No se enviaba el correo, y el enlace tampoco habría servido porque
  `ValidateToken` consultaba la misma columna inexistente.
- **Por qué no se había visto:** el registro social no pasa por ahí. `auth_service.go` marca
  `EmailVerified = isSocial`, así que con Google o Apple todo el bloque de verificación se salta.
  Solo fallaba el registro con correo y contraseña.
- **Alcance:**
  - `ports/interfaces.go` — `EmailVerificationRepository` pasa a hablar de `userID`, y
    `MarkEmailAsVerified` también.
  - `postgres/email_verification_repository.go` — las tres consultas contra `user_id`.
  - `postgres/user_repository.go` — `MarkEmailAsVerified` marca por id y **excluye las cuentas
    eliminadas** (`deleted_at IS NULL`): un enlace pendiente no debe reactivar una cuenta que su
    dueño dio por eliminada.
  - `services/auth_service.go` — los tres puntos de llamada usan el id.
  - **Se descartó** añadir `email_blind_index` a la tabla: duplicaría la identidad de la persona en
    un sitio más y dejaría un rastro que el borrado de cuenta tendría que perseguir.
  - `Tests`: `verificacion_correo_test.go` (2 casos).
- **Verificado de extremo a extremo** contra la base local: registro → token guardado con `user_id`
  → `POST /verify-email` → la cuenta queda verificada y el token se consume.
- **Sin migración.** La tabla ya era correcta; lo que estaba mal era el código.
- **Criterios de QA:**
  1. **Registrarse con correo y contraseña** devuelve éxito, no 500.
  2. **Llega el correo de verificación** y su enlace marca la cuenta como verificada.
  3. **El token se consume:** el mismo enlace no sirve dos veces.
  4. **Reenviar la verificación** funciona y **invalida el enlace anterior**: solo debe haber un
     token vivo por persona.
  5. **El registro con Google y con Apple sigue funcionando** y no pide verificación.
  6. **Una cuenta eliminada no se puede verificar** con un enlace pendiente de antes.

---

### [2026-08-10]: El segundo registro de cada base fallaba por el alias

- **El problema:** `RegisterRequest` **no tenía campo `alias`**, así que todo registro insertaba
  `alias = ''`. Como `users_alias_key` es UNIQUE y en Postgres dos cadenas vacías **sí** colisionan
  —dos NULL no—, la segunda cuenta sin alias de cada base violaba la restricción.
- **Y el mensaje despistaba:** el repositorio traducía **cualquier** violación 23505 a
  `alias_in_use`, de modo que un correo repetido mandaba a cambiar un alias que nadie había escrito.
- **Alcance:**
  - `handler/http/user_handler.go` — `alias` en `RegisterRequest` y pasado al dominio.
  - `postgres/user_repository.go` — el INSERT guarda el alias con `NULLIF($28, '')`, para que "sin
    alias" sea NULL y no cadena vacía. El índice parcial `idx_users_alias` ya asumía NULL.
  - `postgres/user_repository.go` — los errores se distinguen por restricción: `users_alias_key` →
    `alias_in_use`, `users_email_key` → `user already exists`, y cualquier otro se propaga tal cual
    en vez de disfrazarse.
  - `scripts/20260810_alias_vacio_a_null.sql` — **migración**: pasa a NULL los alias vacíos que
    quedaron. Aplicada y probada dos veces: es idempotente. Sin ella, la primera cuenta nueva sin
    alias volvería a chocar con la fila que tenga `''`.
- **Verificado contra la base local:** dos registros seguidos sin alias conviven; un alias repetido
  da `alias_in_use`; un correo repetido da 409 "El usuario ya existe".
- **Criterios de QA:**
  1. **Dos cuentas nuevas sin alias** se registran sin error.
  2. **Alias repetido** → mensaje de alias en uso, y se puede corregir y reintentar.
  3. **Correo repetido** → mensaje de usuario ya existente, **no** de alias.
  4. **En producción, comprobar cuántas cuentas tienen el alias vacío** antes de desplegar: si hay
     una, la migración la deja en NULL; si hubiera más, algo más raro está pasando y conviene mirarlo.

---

### [2026-08-10]: Queda constancia de qué versión de los textos legales se aceptó

- **Por qué:** `core.users` guardaba el consentimiento en dos booleanos. Un booleano prueba que hubo
  aceptación, pero no **de qué texto** ni en qué fecha, y el Decreto 1377 de 2013 exige conservar
  prueba del modo, la fecha y el contenido de la autorización.
- **Por qué ahora y no después.** Los textos legales se están reescribiendo para poder publicar en
  las tiendas (`reports/20260810_documentacion_privacidad_contenido.md`). En cuanto se publique la
  redacción nueva, quien la acepte queda indistinguible de quien aceptó la anterior, y **ese corte no
  se puede reconstruir hacia atrás**. Por eso va antes del cambio de textos.
- **Alcance:**
  - `scripts/20260810_version_consentimiento.sql` — **migración**: cuatro columnas
    (`terms_version`, `terms_accepted_at`, `data_sharing_version`, `data_sharing_accepted_at`) más
    el backfill. **Aplicada y verificada en local contra Postgres real, y probada dos veces: es
    idempotente.** Pendiente de aplicar en producción con el próximo despliegue.
  - `domain/legal.go` (nuevo) — `TermsVersionVigente` y `PrivacyVersionVigente`, por fecha de entrada
    en vigor del documento. **Hay que subirlas cada vez que se publique una redacción nueva**; si el
    texto cambia y la constante no, todo el mundo queda registrado aceptando algo que no leyó.
  - `services/auth_service.go` — `sellarConsentimiento`. **La versión la fija el servidor, no el
    cliente**: si viniera en el cuerpo de la petición, una app antigua o manipulada podría declarar
    una versión que nunca mostró.
  - `postgres/user_repository.go` — las cuatro columnas en el INSERT.
  - `Tests`: `version_consentimiento_test.go` (3 casos). `go build`, `go vet` y los tests pasan.
- **Verificado de extremo a extremo** contra la base local: un registro nuevo aceptando ambas
  casillas quedó con `terms_version = 2026-04-01`, `data_sharing_version = 2026-06-02` y las dos
  fechas puestas; la cuenta anterior quedó con la fecha de su registro y la versión en NULL. Los
  datos de prueba se retiraron después.
- **El backfill deja la versión en NULL a propósito.** De las cuentas anteriores sabemos cuándo
  aceptaron —al registrarse— pero no qué texto vieron: la app muestra su propio aviso legal
  embebido, que no coincide con los T&C que la empresa considera vigentes. Escribir una versión
  supuesta convertiría una laguna conocida en un dato falso con apariencia de prueba.
- **Criterios de QA:**
  1. **Aplicar la migración** y comprobar que las cuatro columnas existen en `core.users`.
  2. **Idempotencia:** aplicarla dos veces seguidas no debe fallar ni duplicar nada.
  3. **Backfill:** las cuentas anteriores con `terms_accepted = true` deben quedar con
     `terms_accepted_at = created_at` y `terms_version` **en NULL**.
  4. **Registro nuevo aceptando ambas casillas:** `terms_version` = `2026-04-01`,
     `data_sharing_version` = `2026-06-02`, y las dos fechas puestas.
  5. **Registro aceptando solo los términos:** la política queda en NULL. Son dos casillas
     independientes; sellar la segunda por arrastre registraría una autorización comercial que nadie
     dio.
  6. **La app no cambia:** el registro sigue funcionando igual desde el móvil, sin enviar nada nuevo.
  7. **Eliminar la cuenta no borra el consentimiento:** al anonimizar, la versión y la fecha se
     conservan. Son prueba legal y no identifican a nadie.

---

### [2026-08-06]: Eliminar mi cuenta — `DELETE /api/me`

- **Por qué:** App Store lo exige desde junio de 2022 (directriz 5.1.1(v)) a toda app que permita
  registrarse, y Google Play también. **Sin esto la app no se puede publicar en ninguna tienda.**
  Hoy la app solo ofrecía cerrar sesión.
- **La cuenta se ANONIMIZA, no se borra.** El motivo está en el esquema: **catorce tablas**
  referencian `core.users` con `ON DELETE CASCADE`. Un borrado real destruiría los mensajes de chat
  —incluida la mitad de las conversaciones de **otras** personas—, las transacciones de eventos ya
  cobrados y las respuestas de encuestas. Y `events.registrations` ni siquiera tiene clave foránea:
  sus filas quedarían apuntando a un id inexistente. Anonimizar cumple igual con el RGPD y con las
  dos tiendas.
- **Alcance:**
  - `scripts/20260806_borrado_de_cuenta.sql` — **migración**: añade `core.users.deleted_at` y un
    índice parcial. **Aplicada y probada dos veces: es idempotente.**
  - `postgres/user_repository.go` — `AnonymizeUser`, **en transacción** con el borrado de los tokens
    FCM: si se anonimizara la cuenta pero fallara lo segundo, esa persona seguiría recibiendo push
    de una cuenta que cree eliminada.
  - El correo se libera con un valor derivado del id (`email_blind_index` es UNIQUE y NOT NULL), así
    que **esa persona puede volver a registrarse con el mismo correo**. `alias` se libera igual.
  - `first_name`/`last_name` quedan **en claro** con "Usuario"/"eliminado": el resto de la tabla va
    cifrada y los servicios descifran con el patrón "si falla, conserva el valor", así que salen
    legibles sin que el repositorio necesite la clave.
  - `services/auth_service.go` — `DeleteMyAccount`; `handler/http/user_handler.go` — `DeleteMe`, con
    **el usuario tomado del token, nunca del cuerpo**; ruta registrada en `main.go`.
  - `Tests`: `borrado_cuenta_test.go` (3 casos). Y **verificado contra Postgres real** con una
    cuenta sembrada, en transacción revertida.
- **Criterios de QA:**
  1. **Los datos personales desaparecen:** tras eliminar, la fila de `core.users` debe mostrar
     `Usuario eliminado`, sin teléfono, alias, bio ni correo, y con `deleted_at` puesto.
  2. **El historial se conserva:** sus inscripciones a eventos siguen en `events.registrations` y
     los mensajes de chat siguen visibles para la otra persona.
  3. **Deja de recibir push:** no quedan filas suyas en `core.user_fcm_tokens`.
  4. **No puede volver a entrar** con su correo y contraseña anteriores.
  5. **Puede volver a registrarse** con ese mismo correo, y sale una cuenta nueva y vacía.
  6. **Solo se borra a sí mismo:** con el token de A, la cuenta de B queda intacta.
  7. **Sin token** → 401. **Cuenta ya eliminada** → 404, no 204.

---

### [2026-08-06]: El botón "Eliminar foro" del panel devolvía 405

- **El problema:** `AdminDeleteForum` estaba escrito desde el módulo de foros y **nunca se registró
  en `main.go`**. No era un handler huérfano y ya está: **el panel ya llamaba a esa URL**
  (`forum-admin.service.ts:40`, `deleteForum`), así que el botón "Eliminar" con su diálogo de
  confirmación estaba roto desde el principio. El patrón `/api/admin/forums/{forumID}` existía para
  `PUT` pero no para `DELETE`, de ahí el **405** en vez de un 404.
  - Comprobado contra producción antes del arreglo: `DELETE` → **405**, `PUT` del mismo patrón →
    401, `DELETE /api/admin/forums/posts/{postID}` → 401. Los dos últimos existen; el primero no.
- **Alcance:**
  - `cmd/server/main.go` — `r.Delete("/api/admin/forums/{forumID}", forumHandler.AdminDeleteForum)`
    en el bloque `AdminOnly`, junto al resto de rutas de foros.
  - Se retiró una línea **duplicada**: `/api/admin/forums/flagged` estaba registrada dos veces con
    el mismo handler.
  - **Sin migración.** No se tocó el panel: ya llamaba a la ruta correcta.
- **Criterios de QA:**
  1. **El botón funciona:** en "Administrar Foros", eliminar un foro debe pedir confirmación y
     desaparecer de la lista. Antes no pasaba nada visible.
  2. **Sin token o sin rol admin:** `DELETE /api/admin/forums/{id}` → 401 / 403, no 405.
  3. **Ojo, el borrado arrastra los posts.** `forum_posts_forum_id_fkey` tiene **`ON DELETE
     CASCADE`** —verificado en el dump y en la base real—, así que eliminar un foro borra **todos
     sus mensajes**, sin aviso y sin vuelta atrás. El panel solo muestra un `confirm()` genérico.
     Probar con un foro que tenga posts y confirmar que desaparecen; y valorar si ese diálogo
     debería advertir de cuántos mensajes se van a perder.
  4. **El resto de foros no se toca:** los demás siguen en la lista y sus posts intactos.

---

### [2026-08-06]: CORS deja de aceptar cualquier origen

- **El problema:** `AllowedOrigins: {"*"}` junto con `AllowCredentials: true`. Cualquier página de
  cualquier dominio podía llamar a la API desde el navegador de quien la visitara. Además esa
  combinación es **inválida** según la especificación —los navegadores rechazan credenciales con
  comodín—, así que estaba abierta *y* mal.
- **Alcance:**
  - `cmd/server/main.go` — `AllowOriginFunc` en lugar de la lista con `*`. Acepta
    `https://legacy.intelyclick.com` y **cualquier `localhost`/`127.0.0.1` sin importar el puerto**:
    `ng serve` usa el 4200 y `flutter run -d chrome` levanta uno aleatorio en cada arranque, así que
    fijar puertos rompería el desarrollo.
  - **La app móvil no se ve afectada.** Un cliente nativo no envía cabecera `Origin` y CORS no
    interviene; esto solo gobierna navegadores, es decir el panel y la app compilada para web.
  - Si el panel o la app web se publican algún día en otro dominio, hay que añadirlo a
    `origenesDeConfianza` o dejarán de funcionar con un error de CORS.
  - `Tests`: `cmd/server/cors_test.go` (nuevo, 11 casos). Los negativos son los que importan:
    dominios que **imitan** el nuestro (`legacy.intelyclick.com.malicioso.com`,
    `malicioso-legacy.intelyclick.com`) y el mismo dominio por `http` en vez de `https`.
  - **Sin migración.**
- **Criterios de QA:**
  1. **El panel sigue funcionando** en `https://legacy.intelyclick.com`: iniciar sesión, listar
     usuarios y abrir eventos. Es la comprobación que dice si el cambio rompió algo.
  2. **Desarrollo local intacto:** `ng serve` en el 4200 contra la API de producción sigue
     funcionando.
  3. **La app móvil no cambia**, ni en Android ni en iOS.
  4. **Un origen ajeno queda bloqueado:** una petición con `Origin: https://malicioso.com` **no**
     debe recibir cabecera `Access-Control-Allow-Origin`.

---

### [2026-08-06]: Un evento sin categoría ya no desaparece del listado

- **El problema:** `GetEvents` y `GetEventByID` unían con `JOIN events.categories`, y
  `events.category_id` es **anulable**. Un evento sin categoría —o con una categoría borrada— se
  caía del listado **sin ningún error**: la respuesta era un 200 con un evento de menos, así que
  nadie se enteraba. En `GetEventByID` era peor: devolvía `ErrNotFound`, o sea un **404 sobre un
  evento que existe**, y quien tuviera el enlace veía "no encontrado".
- **Alcance:**
  - `postgres/event_repository.go` — `LEFT JOIN` en las dos consultas (líneas 49 y 80 anteriores).
  - `COALESCE` en las tres columnas que vienen de la categoría, que es la contrapartida obligatoria:
    `domain.Event` declara `CategoryID`, `Category` y `CategoryOrder` como no anulables, así que un
    nulo rompería el `Scan` y cambiaríamos un evento invisible por un error 500.
  - Los eventos sin categoría se ordenan con `COALESCE(c.order_index, 9999)`, es decir **al final**
    del listado, donde estorban menos.
  - **Verificado contra el Postgres real** con dos eventos sembrados en una transacción revertida:
    el `JOIN` antiguo devolvía 1 de 2; el `LEFT JOIN` devuelve los 2, y el detalle por id del
    evento huérfano pasa de 0 filas a 1.
  - Los tests con mocks **no cubren esto**: la lógica vive en el SQL y el doble del repositorio no
    ejecuta consultas. Por eso la comprobación se hizo contra la base.
  - **Sin migración.**
- **Criterios de QA:**
  1. **Evento sin categoría visible:** crear un evento dejando la categoría vacía y comprobar que
     aparece en el listado de la app y del panel. Antes desaparecía sin aviso.
  2. **Su detalle abre:** entrar a ese evento desde el listado → debe cargar, no dar 404.
  3. **Va al final:** en el listado aparece después de los que sí tienen categoría.
  4. **Nada cambia para los demás:** los eventos con categoría se siguen viendo en el mismo orden de
     siempre, agrupados por categoría.
  5. **Categoría borrada:** si se elimina una categoría que tenía eventos, esos eventos deben seguir
     apareciendo en lugar de esfumarse.

---

### [2026-08-06]: Un fallo pasajero de FCM ya no borra el dispositivo

- **El problema, visto en producción:** `SendNotification` borraba el token ante **cualquier** error
  de envío. Un corte momentáneo con Google se llevaba por delante dispositivos buenos, y esos
  usuarios dejaban de recibir notificaciones **para siempre**, sin rastro de por qué. Se detectó al
  probar un envío dirigido: desaparecieron los 7 tokens de un usuario de una sola llamada.
- **Alcance:**
  - `infrastructure/firebase/fcm.go` — `ErrTokenInvalido` (nuevo) y `esTokenInvalido`, que separa
    los errores **definitivos** (`IsRegistrationTokenNotRegistered`, `IsUnregistered`,
    `IsInvalidArgument`, `IsSenderIDMismatch`: app desinstalada, token rotado, mal formado o de otro
    proyecto) de los **transitorios** (servidor no disponible, error interno, cuota o frecuencia
    excedidas), que quedan fuera de la lista a propósito.
  - `SendToToken` envuelve el error con `%w` en vez de `%v`. Con `%v` se perdía la causa y era
    imposible clasificarla.
  - `services/notification_service.go` — el token solo se borra si `errors.Is(err,
    firebase.ErrTokenInvalido)`. Ante un fallo pasajero se conserva y el siguiente envío reintenta.
  - `ports.PushSender` (nuevo): el servicio dependía del tipo concreto `*firebase.FCMClient`, lo que
    hacía **imposible probar** justo la parte delicada. Ahora depende de la interfaz y `main.go` no
    cambia, porque `*FCMClient` la satisface.
  - `Tests`: `borrado_tokens_test.go` (nuevo, 4 casos).
  - **Sin migración.**
- **Criterios de QA:**
  1. **Un dispositivo con la app desinstalada** desaparece de `core.user_fcm_tokens` tras un envío
     dirigido. Ese borrado sí es el correcto.
  2. **Un dispositivo válido sobrevive** a un envío que falle por causas de red: el token debe
     seguir en la tabla.
  3. **Envío a un usuario con varios dispositivos:** si uno está muerto y otro vivo, la respuesta es
     200, llega al vivo y solo se borra el muerto.
  4. **Si fallan todos los envíos**, la respuesta sigue siendo 500 con el detalle en el cuerpo.

---

### [2026-08-06]: Suscripción al tópico "all" desde el servidor

- **El problema:** la app **nunca llamó a `subscribeToTopic`** — registra su token en
  `/api/me/fcm-token` (`auth_provider.dart:410`) y nada más. Como `SendToTopic(..., "all")` solo
  llega a dispositivos suscritos, **ningún envío a "todos" llegaba a nadie**, ni el manual del panel
  ni los avisos automáticos nuevos. Con FCM en modo mock no se notaba: no llegaba nada de todos
  modos. En producción había **11 tokens de 3 usuarios**, todos sin suscribir.
- **Alcance:**
  - `infrastructure/firebase/fcm.go` — `SubscribeToTopic`, en lotes de 1000 (límite del Admin SDK).
    Un token caducado hace fallar solo su parte del lote y queda anotado en el log.
  - `services/notification_service.go` — `RegisterToken` suscribe el dispositivo al tópico
    `TopicTodos` justo después de guardarlo. **Un fallo de suscripción no invalida el registro**: el
    token queda guardado, puede recibir envíos dirigidos y la suscripción se reintenta en bloque.
  - `SubscribeAllToTopic` (nuevo) y `POST /api/admin/notifications/subscribe-all`, bajo `AdminOnly`:
    suscribe los tokens ya registrados. Hace falta porque los dispositivos anteriores a este cambio
    no se suscribirían solos —eso solo ocurre al registrar el token, y esos usuarios ya lo hicieron.
  - `NotificationRepository.GetAllTokens` (nuevo).
  - **Se suscribe desde el servidor y no desde Flutter** a propósito: alcanza a los dispositivos ya
    registrados y no depende de publicar una versión nueva de la app, que hoy está parada.
  - `Tests`: `suscripcion_topico_test.go` (nuevo, 7 casos).
  - **Sin migración.**
- **Criterios de QA:**
  1. **Suscribir los existentes:** `POST /api/admin/notifications/subscribe-all` con token de
     administrador → 200 con `{"subscribed": N}`. Con los datos de hoy, N debería ser **11**.
  2. **Sin rol de administrador:** la misma ruta → 403.
  3. **Envío a todos:** desde el panel, mandar una notificación con destino "todos" → **debe llegar
     al teléfono**. Es la prueba que dice si el problema quedó resuelto; antes no llegaba.
  4. **Dispositivo nuevo:** iniciar sesión en la app con otro teléfono y comprobar que recibe el
     siguiente envío a "todos" sin necesidad de volver a llamar a `subscribe-all`.
  5. **Idempotencia:** ejecutar `subscribe-all` dos veces seguidas no debe dar error ni duplicar
     nada.
  6. **El registro de token sigue funcionando** aunque la suscripción falle: cerrar y abrir sesión
     en la app no debe dar error.

---

### [2026-08-06]: Avisos automáticos al publicar un evento o un contenido

- **Alcance:**
  - `handler/http/avisos.go` (nuevo) — `notificarNovedad`, el disparador común. Envía al topic
    `all`, el mismo del envío manual del panel.
  - `event_handler.go` — `CreateEvent` avisa tras crear el evento.
  - `content_handler.go` — `AdminCreateContent` avisa **solo si nace publicado**, y
    `AdminUpdateContent` **solo en la transición de borrador a publicado**. Sin esa comprobación,
    corregir una errata en un artículo ya publicado volvería a notificar a todos.
  - `cmd/server/main.go` — las notificaciones se cablean **antes** que eventos y contenido, porque
    ahora esos handlers las usan.
  - **El aviso nunca puede tumbar la operación.** `notificarNovedad` no devuelve error: si FCM está
    caído —o corre en modo mock por falta de credenciales, que es como está producción hoy— el
    evento se crea igual y el fallo queda en el log. Un aviso es un efecto secundario; convertirlo
    en un punto de fallo haría de una molestia una avería.
  - **El admin sale del token** (las rutas son `AdminOnly`), para que el historial de notificaciones
    diga quién publicó la novedad, igual que en un envío manual.
  - `Datos adjuntos`: `{"type": "event"|"content", "id": "..."}`, para que la app pueda abrir la
    pantalla correspondiente al tocar la notificación. **La app todavía no los usa**: hoy la push
    abre la aplicación sin navegar a ningún sitio.
  - `Cuerpo recortado` a 140 caracteres, cortando por el último espacio. Android e iOS truncan de
    todos modos.
  - **No se segmenta por grupos.** Los `core.custom_groups` existen para eso, pero decidir a quién
    interesa cada novedad es una decisión de producto, no una que convenga inventar en el código.
  - `Tests`: `avisos_test.go` y `avisos_contenido_test.go` (nuevos, 15 casos).
  - **Sin migración.**
- **Importante:** en producción **FCM está en modo mock** —falta `firebase-service-account.json` en
  el servidor—, así que los avisos se escribirán en el log del contenedor y **no llegará ninguna
  notificación a los teléfonos** hasta que se suba ese archivo. El código queda listo; el archivo es
  lo que falta.
- **Criterios de QA:**
  1. **Con FCM en mock (hoy):** crear un evento desde el panel y comprobar en
     `docker compose logs backend` que aparece el intento de envío. El evento debe crearse
     **siempre**, haya o no notificación.
  2. **Con `firebase-service-account.json` en el servidor:** crear un evento y comprobar que llega
     la push a un teléfono con la app instalada, con el título "Nuevo evento: ...".
  3. **Contenido publicado:** crear un contenido con "publicado" activo → llega aviso. Crearlo como
     borrador → **no** llega.
  4. **Editar un publicado no repite el aviso:** cambiar el título de un contenido ya publicado y
     guardar → **no** debe llegar ninguna notificación. Es lo que más fácil se rompe.
  5. **Publicar un borrador sí avisa:** editar un borrador marcándolo como publicado → llega una
     notificación, una sola.
  6. **Historial:** los avisos automáticos deben aparecer en el historial del panel con el
     administrador que los originó.
  7. **Cuerpo largo:** un evento con descripción larga debe mostrar el texto recortado, sin cortar
     una palabra por la mitad.

---

### [2026-08-06]: Webhook de CredibanCo

- **Alcance:**
  - `GET` y `POST` `/api/payments/credibanco/callback`, **en el bloque de rutas públicas**, sin
    `AuthMiddleware`: quien llama es la pasarela y no tiene un token nuestro.
  - **Por qué eso no es un agujero:** el contenido de la notificación **no decide nada**. Solo se usa
    para saber qué transacción mirar; el estado se pregunta a CredibanCo con nuestras credenciales,
    igual que hace `VerifyPayment`. Una notificación inventada no puede declarar un pago aprobado —
    lo cubre `TestWebhook_UnaNotificacionFalsaNoApruebaNada`. Este es el punto que hay que preservar
    si alguien toca este código: **en cuanto el estado se lea de la notificación, la ruta pasa a ser
    una vía para regalar inscripciones.**
  - **Para qué sirve:** hasta ahora la confirmación dependía de que el usuario volviera a la app.
    Quien pagaba y cerraba el navegador dejaba el cobro hecho y la inscripción sin confirmar **para
    siempre**.
  - `Identificador`: se acepta tanto el nuestro (`orderNumber`, que es el id de la transacción) como
    el de CredibanCo (`mdOrder`), más los alias `tx_id`, `orderId` y `order_id`. No está confirmado
    cuál enviarán, y probar varios sale más barato que un despliegue para añadir un nombre.
  - `Repositorio`: `GetTransactionByOrderID` (nuevo), para entrar por el id de la pasarela.
  - `Límite de abuso`: una transacción ya aprobada o rechazada **no se vuelve a consultar** a la
    pasarela. Sin ese corte, cualquiera con una referencia válida podría hacernos repetir llamadas
    salientes indefinidamente. Sí se reintenta la confirmación de la inscripción, que es local e
    idempotente, y repara el caso en que el pago se marcó aprobado pero la inscripción no llegó a
    confirmarse.
  - `Códigos`: **200** al procesar y también ante una referencia desconocida —un error haría que la
    pasarela reintentara en bucle algo que nunca va a existir—; **400** si no viene ningún
    identificador; **500** solo ante un fallo nuestro, donde sí interesa que reintenten.
  - `Log`: se registra **toda** notificación recibida con sus parámetros en crudo. Cuando CredibanCo
    active el aviso, ese log es lo único que dirá con qué nombres y formato lo manda de verdad.
  - `Tests`: `payment_webhook_test.go` en services (7 casos) y en handler/http (5 casos). La consulta
    nueva se ejercitó además contra el Postgres real, incluido el caso de dos filas con el mismo
    `credibanco_order_id`.
  - **Sin migración.**
- **Pendiente que no depende de nosotros:** hay que pedirle a CredibanCo que **configure la URL de
  notificación** apuntando a `https://legacy.intelyclick.com/api/payments/credibanco/callback`, y
  confirmar con qué nombre envían el identificador y si firman el aviso. Hasta entonces el endpoint
  existe pero nadie lo llama.
- **Criterios de QA:**
  1. **La ruta responde sin token:** `GET /api/payments/credibanco/callback?mdOrder=x` → **200**
     (no 401). Es lo primero que hay que ver desplegado.
  2. **Sin identificador:** la misma ruta sin parámetros → **400**.
  3. **Referencia desconocida:** con un `mdOrder` inventado → **200**, y en el log del contenedor la
     línea `[PAGO][webhook] referencia desconocida`.
  4. **Toda notificación queda registrada:** `docker compose logs backend | grep webhook` debe
     mostrar el método y los parámetros recibidos.
  5. **Con un pago real** (cuando CredibanCo desbloquee): pagar y **cerrar el navegador sin volver a
     la app**. La inscripción debe quedar `confirmed` igualmente, y el QR aparecer en "Mi
     credencial". Es la prueba que justifica todo el trabajo.
  6. **Idempotencia:** repetir la misma notificación tres veces no debe duplicar inscripciones ni
     cambiar el resultado.

---

### [2026-08-06]: El escáner de la puerta mostraba el nombre en texto cifrado

- **Alcance:**
  - `postgres/event_repository.go` — `GetRegistrationByQR` deja de hacer
    `u.first_name || ' ' || u.last_name`. Ambos campos están cifrados, así que esa concatenación
    unía **dos bloques AES independientes** en una cadena que ya no se puede descifrar ni por
    partes: `Decrypt` sobre ella falla siempre. El efecto era que al escanear un QR el panel
    mostraba el nombre del asistente en texto cifrado (`attendance-scanner.component.html:24`), y el
    correo igual. Ahora las tres columnas salen por separado.
  - `domain/event.go` — `CheckInResponse` gana `FirstName` y `LastName` con `json:"-"`, el mismo
    paso intermedio que `EventRegistrant`. **El contrato del panel no cambia:** sigue recibiendo
    `userName` y `userEmail`, solo que legibles.
  - `services/event_service.go` — `CheckIn` descifra y compone `UserName`. Como en el resto de
    servicios, un descifrado fallido conserva el valor original: las filas anteriores al cifrado
    están en claro y perderlas sería peor que mostrarlas.
  - `Tests`: 2 casos nuevos en `checkin_pago_test.go` (7 en total).
  - **La prueba unitaria no basta y por eso se hizo también contra Postgres:** con un doble de
    `CryptoService`, los tests pasarían igual aunque la consulta siguiera concatenando. Se verificó
    con una fila sembrada dentro de una transacción revertida que las tres columnas llegan separadas
    y con su valor intacto.
  - **Sin migración.**
- **Criterios de QA:**
  1. **El nombre se lee:** escanear el QR de un asistente en "Escáner de Asistencia" debe mostrar su
     nombre y su correo legibles. Es el punto entero de este cambio.
  2. **Nombre completo:** debe verse "Nombre Apellido", con un solo espacio y sin espacios sobrantes
     si alguno de los dos falta.
  3. **Usuarios antiguos:** un asistente cuyos datos estén sin cifrar debe seguir viéndose igual que
     antes, no en blanco.
  4. **Nada más cambia:** la respuesta del escáner sigue trayendo `registrationId`, `eventTitle`,
     `checkInTime` y los talleres.
  5. **Sigue rechazando lo que debe:** un QR pendiente de pago → 402; uno inventado → 404.

---

### [2026-08-06]: Lista de inscritos por evento

- **Alcance:**
  - `GET /api/events/{id}/registrations`, **bajo `AdminOnly`** — son nombres, correos y teléfonos
    de terceros, además de quién debe dinero. Corte vertical completo: `domain/event.go`
    (`EventRegistrant`), `ports/event_ports.go`, `postgres/event_repository.go`
    (`GetRegistrationsByEvent`), `services/event_service.go` (`GetEventRegistrants`),
    `handler/http/event_handler.go` y **la ruta registrada en `cmd/server/main.go`**.
  - **El nombre y el correo se descifran en el servicio.** Están cifrados con AES-256 en la base,
    así que una consulta que los devolviera directos entregaría texto cifrado. Por eso el
    repositorio los trae **por separado y sin concatenar**: un `first_name || ' ' || last_name` en
    SQL junta dos textos cifrados en uno que ya no se puede abrir por partes —que es exactamente lo
    que le pasa hoy a `GetRegistrationByQR`, ver "Pendiente" al final.
  - `NewEventService` ahora recibe el `CryptoService`. Único cambio de firma; `main.go` ya lo tenía
    instanciado.
  - **El orden alfabético se hace en Go, no en SQL:** un `ORDER BY` sobre nombres cifrados ordenaría
    por el texto cifrado. La consulta ordena por fecha y el servicio reordena ya en claro, sin
    distinguir mayúsculas y desempatando por fecha de inscripción.
  - **No devuelve `qr_data`**: quien organiza necesita saber quién viene y quién debe, no el código
    de entrada de cada asistente. Así una lista que se exporte o se reenvíe no reparte credenciales.
  - `Sin paginación`, como el resto de listados del repositorio. Un evento con miles de inscritos
    devuelve miles de filas de una vez; queda anotado en el código.
  - `Tests`: `inscritos_evento_test.go` (nuevo, 5 casos). La consulta se ejercitó además **contra el
    Postgres real** con tres filas sembradas dentro de una transacción revertida: teléfono nulo,
    correo nulo y `payment_status` nulo.
  - **Sin migración.**
- **Criterios de QA:**
  1. **Solo administradores:** `GET /api/events/{id}/registrations` con token de usuario normal →
     **403**; sin token → **401**; con token de administrador → **200**.
  2. **Los datos se leen:** el nombre y el correo salen legibles, no como texto cifrado. Es el punto
     que más fácil se rompe.
  3. **Estados visibles:** un evento con una inscripción pagada y otra pendiente debe mostrar
     `paid`/`confirmed` y `pending`/`pending_payment` respectivamente.
  4. **Orden alfabético** por nombre completo, sin que "ana" en minúscula caiga al final.
  5. **Evento sin inscritos** → `200` con `[]`, no `null` ni 404.
  6. **Ningún `qrData`** en la respuesta.

---

### [2026-08-06]: El QR de una reserva sin pagar ya no abre la puerta

- **Alcance:**
  - `services/event_service.go` — `CheckIn` rechaza una inscripción en `pending_payment` antes de
    registrar la asistencia. El QR se genera al **reservar el cupo**, o sea antes de pasar por la
    pasarela, así que existía desde el primer momento y `CheckIn` no miraba el estado: reservar sin
    pagar daba un código que abría la puerta de un evento de 250.000.
  - Se comprueba en la puerta y no solo al entregar el código: es el único punto por el que pasan
    todos los caminos, y no depende de que ningún cliente se acuerde de ocultar nada.
  - `handler/http/event_handler.go` — `CheckIn` distingue ahora **402** (pendiente de pago) de
    **404** (QR inexistente); antes ambos salían como 404 con el mismo texto. En la puerta son dos
    situaciones distintas: un código inventado, o un asistente real al que hay que cobrarle.
  - `handler/http/event_handler.go` — la respuesta **201 de `POST /api/events/{id}/register`** deja
    de incluir `qr_data` cuando la inscripción nace pendiente de pago. Era la vía por la que el
    código seguía saliendo del servidor: `GET /api/me/registrations` ya lo ocultaba desde el 05,
    pero la respuesta de la propia reserva no. La app lee el QR de `/api/me/registrations` (campo
    `qrData`), no de aquí (`qr_data`), así que no se queda sin nada que mostrar.
  - `Errores nuevos`: `ErrCheckInPendingPayment` y `ErrCheckInNotFound`, centinelas para que el
    handler los separe con `errors.Is`.
  - `Tests`: `checkin_pago_test.go` (nuevo, 4 casos) y `checkin_qr_test.go` (nuevo, 5 casos).
  - **Sin migración.**
- **Criterios de QA:**
  1. **Reserva sin pagar en la puerta:** reservar cupo en un evento de pago y escanear ese QR →
     **402** con "La inscripción está pendiente de pago". **La asistencia no debe quedar marcada**:
     comprobar que `attendance_confirmed` sigue en `false`.
  2. **Tras pagar sí entra:** con la inscripción en `confirmed`, el mismo QR → **200** y
     `attendance_confirmed = true`.
  3. **Evento gratuito:** el QR funciona igual que siempre; ese camino no pasa por la pasarela.
  4. **QR inventado** → **404**, no 402.
  5. **La reserva no entrega el código:** el 201 de reservar cupo en un evento de pago **no** debe
     traer `qr_data`, pero sí el estado y el importe. En un evento gratuito sí lo trae.
  6. **"Mi credencial" no cambia:** los eventos pagados siguen mostrando su QR.

---

### [2026-08-05]: El `tx_id` viaja en la URL de retorno del pago

- **Alcance:**
  - `internal/core/services/payment_service.go` — `InitiatePayment` añade `?tx_id={uuid}` a la URL
    de retorno antes de entregársela a CredibanCo. Al volver de la pasarela, la app necesita saber
    **qué** verificar, y `/api/payments/verify` espera **nuestro** id de transacción, no el de
    CredibanCo. Depender de que la pasarela añada un parámetro con un nombre concreto sería frágil;
    así está garantizado, se llame como se llame lo que ella agregue.
  - Se conservan los parámetros que la URL ya trajera, y si viene malformada el `tx_id` se
    concatena a mano en vez de perderse.
  - `Tests`: `payment_returnurl_test.go` (nuevo, 4 casos), incluido que el `tx_id` coincida con el
    `orderNumber` que recibe la pasarela: si difirieran, la verificación consultaría una transacción
    distinta de la que se cobró.
  - **Sin migración.**
- **Criterios de QA:**
  1. **La URL de retorno lleva el id:** iniciar un pago y comprobar en el formulario de CredibanCo
     que la URL de retorno contiene `tx_id=` con un uuid.
  2. **Coincide con la transacción:** ese `tx_id` debe ser el `id` de la fila recién creada en
     `core.transactions`.
  3. **Verificación con token:** `GET /api/payments/verify?tx_id=...` **con** cabecera
     `Authorization` → 200 con el estado. Sin cabecera → 401.
  4. **Pago aprobado inscribe:** con una transacción en `APPROVED`, verificar debe dejar la
     inscripción del evento en `payment_status = paid` y `registration_status = confirmed`.

---

### [2026-08-05]: El importe y el usuario de un pago los decide el servidor (fallos 4 y 3)

- **Alcance:**
  - **Fallo 4 — el importe lo dictaba el cliente.** `InitiatePayment` tomaba el `amount` del cuerpo
    de la petición y lo pasaba tal cual a CredibanCo, sin contrastarlo nunca con el precio real
    —que el backend tiene a mano, porque `reference_id` **es** el id del evento—. Un
    `{"amount": 1000}` en un evento de 250.000 **se cobraba por mil pesos**. Ahora se consulta el
    precio en la base y se responde **409** si no coincide. Se rechaza en vez de cobrar el precio
    correcto en silencio: si el cliente traía otro importe es que su información está obsoleta, y
    cobrar algo distinto de lo que el usuario vio en pantalla es peor que pedirle que lo revise.
  - **Fallo 3 — el usuario salía de una cabecera.** `CreatePaymentIntent` leía
    `r.Header.Get("X-User-ID")`, con un comentario que ya admitía que debía salir del JWT. La ruta
    está bajo `AuthMiddleware`, así que el `sub` estaba en el contexto y se ignoraba: **cualquiera
    con sesión podía iniciar una transacción a nombre de otro** cambiando esa cabecera, y quedaba
    registrada contra la víctima. Ahora se toma del token y la cabecera se ignora.
  - `Errores nuevos`: `ErrPaymentEventNotFound` (404), `ErrPaymentEventIsFree` (400) y
    `ErrPaymentAmountMismatch` (409). Un evento gratuito no debe pasar por la pasarela: se entra por
    `POST /api/events/{id}/register`.
  - `Comparación de importes` con tolerancia de medio centavo: viajan como `float` y se guardan como
    `numeric(10,2)`, así que comparar con `==` daría rechazos falsos.
  - `Tests`: `payment_importe_test.go` (nuevo, 7 casos).
  - **Sin migración.** Este despliegue no toca el esquema.
  - **Queda fuera:** `RefTypeCart` no se valida —no hay precio de referencia que consultar— y sigue
    aceptando el importe del cliente. Anotado en `reports/20260805_flujo_pago_eventos.md`.
- **Criterios de QA:**
  1. **Importe menor:** `POST /api/payments/intent` con `amount` 1000 sobre un evento de 250.000 →
     **409**, y **ninguna** transacción creada.
  2. **Importe mayor:** el mismo POST con 500.000 → **409**.
  3. **Importe correcto:** con 250.000 → llega a la pasarela y la transacción se crea con el precio
     del servidor.
  4. **Evento gratuito:** iniciar un pago sobre un evento `is_free` → **400**.
  5. **Evento inexistente:** → **404**, no 500.
  6. **Sin token:** → **401**.
  7. **Cabecera `X-User-ID` de otra persona** junto a un token válido → la transacción queda a
     nombre del **titular del token**, no del de la cabecera. Comprobar en `core.transactions`.

---

### [2026-08-05]: QR impredecible y endpoint de "Mi credencial"

- **Alcance:**
  - `Migración`: `scripts/20260805_qr_data_impredecible.sql` (nueva) — regenera los `qr_data`
    predecibles y añade el `UNIQUE` que faltaba.
  - `QR aleatorio`: `event_service.go` — el código se generaba como `REG-{user_id}-{event_id}`, la
    concatenación de dos uuid que el propio usuario conoce. **Cualquiera que supiera el id de otra
    persona y el del evento podía fabricar su código**, y `CheckIn` lo daba por bueno porque
    `GetRegistrationByQR` busca por `qr_data` sin mirar ni el pago ni el estado. Ahora es
    `REG-{uuid v4}`, generado con `crypto/rand`.
  - `UNIQUE en qr_data`: era la clave de búsqueda del control de acceso y **admitía duplicados**;
    dos filas con el mismo código dejaban el resultado del escaneo a merced del orden de Postgres.
  - `Endpoint nuevo`: `GET /api/me/registrations` — todas las inscripciones del usuario con los
    datos del evento incorporados. **Cuelga de `/api/me` y no de `/api/events`**: el patrón
    `/api/events/{id}` del grupo público captura cualquier segmento, y con
    `/api/events/my-registrations` la respuesta era
    `invalid input syntax for type uuid: "my-registrations"`.
  - `El QR de una inscripción pendiente no sale del servidor`: `GetMyRegistrations` lo vacía. Una
    credencial que no da derecho a entrar no debería viajar, así el cliente no tiene que acordarse
    de ocultarla.
  - `Tests`: `mis_inscripciones_test.go` (nuevo, 5 casos).
- **Criterios de QA:**
  1. **Aplicar la migración antes de subir el binario.**
  2. **Códigos regenerados:** tras la migración, ningún `qr_data` debe contener el `user_id` de su
     propia fila: `SELECT count(*) FROM events.registrations WHERE qr_data LIKE 'REG-' || user_id::text || '%'` → **0**.
  3. **Códigos únicos:** `UPDATE` que repita un `qr_data` existente debe fallar por la restricción.
  4. **Inscripción nueva:** el `qr_data` no contiene ni el id del usuario ni el del evento, y dos
     inscripciones distintas nunca comparten código.
  5. **El código no cambia al reinscribirse:** repetir `POST /register` conserva el `qr_data`, o el
     QR que el usuario tuviera abierto dejaría de servir.
  6. **`GET /api/me/registrations`** con sesión → 200 con la lista; sin token → 401.
  7. **Inscripción confirmada** → viaja con `qrData`; **pendiente de pago** → `qrData` vacío y
     `registrationStatus: pending_payment`, pero **sigue apareciendo** en la lista.
  8. **Sin inscripciones** → `[]`, no `null` ni error.

---

### [2026-08-05]: Estado de inscripción y cierre de dos fallos de suplantación

- **Alcance:**
  - `Migración`: `scripts/20260805_registration_status_pendiente_pago.sql` (nueva) — da uso real a
    `events.registrations.registration_status`, que **era una columna muerta**: existía con
    `DEFAULT 'confirmed'` y no aparecía en ningún archivo Go, así que toda inscripción valía
    `confirmed`. Ahora `NOT NULL` con `CHECK ('confirmed','pending_payment')` e índice
    `(event_id, registration_status)`.
  - `Estado de la inscripción`: un evento **gratuito** queda `confirmed` en el acto; uno **de pago**
    nace `pending_payment` y pasa a `confirmed` cuando la pasarela aprueba el cobro
    (`payment_service.go`). El estado se decide por `payment_status` y no por `event.IsFree`, para
    que una inscripción que un administrador crea ya pagada —por transferencia, por ejemplo— quede
    confirmada sin esperar a una pasarela que nunca la va a confirmar.
  - `Inscripción en el camino de pago`: la app ahora llama a `/register` **antes** de salir a la
    pasarela. Hasta hoy no lo hacía nunca, así que de un evento de pago no quedaba ni rastro de
    quién había intentado comprar.
  - **Fallos 9 y 10 (`event_handler.go`)**: `userID` y `paymentStatus` del cuerpo los honraba
    **cualquiera con sesión**, y esta ruta está bajo `AuthMiddleware`, no `AdminOnly`. Con
    `{"paymentStatus":"paid"}` se entraba gratis a un evento de pago, con QR válido y sin una sola
    transacción; con `{"userID":"<otro>"}` se le dejaba una deuda a un tercero. Ahora se responde
    **403** salvo que quien llame sea administrador. Se rechaza en vez de ignorarlos en silencio:
    un 201 mudo le daría la razón a quien cree que surtieron efecto.
  - `Middleware`: `UserRoleKey` e `IsAdmin(ctx)` — `AuthMiddleware` no ponía el rol en el contexto,
    así que un handler bajo esa ruta no tenía forma de saber quién le llamaba.
  - **Corrección no pedida, verificada**: `SocialLogin` firmaba el token con `user_id` mientras
    `AuthMiddleware` lee `sub`. Quien entraba con Google o Apple recibía un token válido con el que
    **ninguna ruta privada funcionaba** (401 `User ID not found in token`): ni inscribirse, ni la
    agenda, ni el chat, ni la encuesta. Se añade `sub` conservando `user_id`.
  - `Tests`: `registration_status_test.go` (4 casos) y `event_register_auth_test.go` (6 casos).
- **Criterios de QA:**
  1. **Aplicar la migración antes de subir el binario.**
  2. **Evento gratuito:** `POST /api/events/{gratis}/register` → `payment_status: free`,
     `registration_status: confirmed`.
  3. **Evento de pago:** el mismo POST → `payment_status: pending`,
     `registration_status: pending_payment`, y `total_paid` con el precio del evento.
  4. **Fallo 9 cerrado:** `POST .../register` con `{"paymentStatus":"paid"}` y sesión de usuario
     normal → **403**, y **ninguna** fila nueva en `events.registrations`.
  5. **Fallo 10 cerrado:** el mismo POST con `{"userID":"<otro>"}` → **403**.
  6. **El administrador sigue pudiendo:** con token de rol `admin`, ese mismo cuerpo → **201**, la
     inscripción queda a nombre del otro usuario y con `registration_status: confirmed`.
  7. **No estorba lo normal:** `POST .../register` sin cuerpo → **201**, a nombre del titular del
     token. Es lo que hace la app.
  8. **Los talleres siguen libres:** `{"workshops":["id1","id2"]}` sin ser admin → **201**.
  9. **Token sin claim `role`** → **403** en los casos 4 y 5: se deniega, no se concede por omisión.
  10. **Login social:** entrar con Google y luego inscribirse a un evento debe funcionar. Antes
      devolvía 401 en cualquier ruta privada.
  11. **El `CHECK` protege:** `UPDATE ... SET registration_status='pagado'` debe fallar.

---

### [2026-08-05]: Encuesta general del evento (eventos, fase 3)

- **Alcance:**
  - `Migración`: `scripts/20260805_add_event_surveys.sql` (nueva) — tabla `events.event_surveys`
    con `UNIQUE (event_id, user_id)` y `CHECK` de 1..5 en las cuatro calificaciones. **El `UNIQUE`
    es deliberado y se aparta del precedente:** `events.workshop_ratings` no lo tiene, así que allí
    un doble toque deja dos filas y sesga el promedio. Es idempotente: se puede aplicar dos veces.
  - `Dominio`: `internal/core/domain/event.go` — `EventSurvey`, `EventSurveyComment` y
    `EventSurveySummary`. Las calificaciones opcionales y los promedios del resumen son punteros a
    propósito: un `0` se leería como "pésimo" en vez de "sin datos".
  - `Errores`: `internal/core/domain/errors.go` (nuevo) — `ErrUniqueViolation` y `ErrNotFound`, para
    que el adaptador traduzca los códigos de Postgres y los servicios no tengan que importar pgx ni
    buscar subcadenas dentro del mensaje de error, como se hace hoy en `user_repository.go:73`.
  - `Puertos`, `repositorio`, `servicio`, `handler` y **rutas en `cmd/server/main.go`**: el corte
    vertical completo. `POST /api/events/{id}/survey` y `GET /api/events/{id}/survey/me` bajo
    `AuthMiddleware`; `GET /api/events/{id}/survey/summary` bajo `AdminOnly`.
  - `Regla de acceso`: responde solo quien esté registrado en el evento. **No se exige
    `attendance_confirmed`** a propósito: dejaría fuera a quien sí asistió pero el personal no
    alcanzó a escanear.
  - `Tests`: `internal/core/services/event_survey_test.go` (nuevo) — 18 casos.
  - **Dos correcciones de paso, ambas en `event_repository.go`:**
    1. `GetRegistrationByUserAndEvent` comparaba con `sql.ErrNoRows` mientras el driver es pgx v4,
       que devuelve `pgx.ErrNoRows`. La rama nunca casaba y "no hay registro" salía como error. El
       único llamador descarta el error (`event_service.go:101`), así que no rompía nada, pero
       dejaba muerta la comprobación.
    2. `GetEventByID` devolvía el error crudo de pgx; ahora traduce a `domain.ErrNotFound`, que es
       lo que permite responder 404 en vez de 500.
- **Criterios de QA:**
  1. **Aplicar la migración antes de subir el binario.** Si no, el backend nuevo consulta una tabla
     que no existe y las tres rutas devuelven 500:
     `docker exec -i legacy_db psql -U dba -d applegacy < scripts/20260805_add_event_surveys.sql`
  2. **Sin responder:** `GET /api/events/{id}/survey/me` con sesión → **204 sin cuerpo**. No es un
     error: la app lo usa para decidir si ofrece el formulario.
  3. **Calificación fuera de rango:** `POST .../survey` con `{"overallRating":9}` → **400**, no un
     500 con el mensaje de Postgres dentro.
  4. **Evento inexistente:** `POST` sobre un uuid que no existe → **404**.
  5. **Sin registro:** `POST` con la sesión de alguien que no se inscribió → **403**.
  6. **Sin token:** `POST` sin cabecera `Authorization` → **401**.
  7. **Envío válido:** `POST` con `overallRating` y opcionales → **201** y el cuerpo de la encuesta
     guardada. Un comentario con espacios sobrantes debe volver recortado; uno en blanco, como
     `null`.
  8. **Segundo envío:** repetir el `POST` → **409**, y sigue habiendo una sola fila en la tabla.
  9. **Resumen con token de usuario:** `GET .../survey/summary` → **403** (`Admin role required`).
  10. **Resumen con token de administrador:** → **200** con `responses`, los cuatro promedios,
      `recommendRate` y los comentarios sin el usuario que los escribió.
  11. **Resumen de un evento sin respuestas:** todos los promedios en `null` —no en `0`—,
      `responses: 0` y `comments: []`.
  12. **Pregunta que nadie respondió:** su promedio debe salir `null` aunque haya respuestas de
      otras preguntas.

---

### [2026-08-05]: `GET /api/users` devolvía 500 — desajuste de columnas en `FindAll`
- **Alcance:**
  - `Repositorio`: `internal/adapter/storage/postgres/user_repository.go` — el `SELECT` de `FindAll` pedía 27 columnas y el `rows.Scan` declaraba 26 destinos: faltaba el de `password_hash`. pgx aborta con `number of field descriptions must equal number of destinations, got 27 and 26`, que el handler traduce a 500 (`user_handler.go:178`). Se retira `password_hash` del `SELECT` en vez de añadir el destino: el listado del panel no usa el hash (`domain.User` lo marca `json:"-"`), así que no hay motivo para traer los hashes bcrypt de todos los usuarios a memoria. `FindByID` sí lo necesita y queda igual.
  - **El fallo solo se manifiesta con datos:** con la tabla vacía el `Scan` nunca se ejecuta y la ruta responde `200 []`. Por eso no salta en una base recién creada.
  - **Sin cambios en el panel ni en la app:** el contrato JSON de la respuesta es idéntico; el hash nunca se serializaba.
- **Criterios de QA:**
  1. **Listado de usuarios:** en el panel administrativo, abrir la sección de usuarios y comprobar que carga la lista. Antes respondía **500** con el cuerpo `number of field descriptions...`.
  2. **Contenido correcto:** los nombres y correos se ven en claro (no cifrados) y el listado viene ordenado del más reciente al más antiguo.
  3. **Sin regresión en el detalle:** editar y guardar un usuario desde el panel (`PUT /api/users/{id}`) y eliminar uno de prueba (`DELETE /api/users/{id}`) siguen funcionando.
  4. **Sin regresión en la app:** `GET /api/users/me` sigue devolviendo el perfil completo del usuario autenticado.
  5. **Login intacto:** iniciar sesión con correo y contraseña, y con Google, sigue funcionando (el hash se lee por otras rutas, no por `FindAll`).

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
