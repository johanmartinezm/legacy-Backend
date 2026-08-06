# Bitácora de QA - Proyecto Go [BACKEND]

Entrada de trabajo para validación de API.

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
