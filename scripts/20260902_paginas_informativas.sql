-- 20260902_paginas_informativas.sql
--
-- Crea core.paginas_informativas y siembra la página "legacy-board".
--
-- Contexto: Legacy Board vivía entero en el código de la app —los dos nombres
-- en App-Movil/assets/data/board_contacts.json y los textos dentro de
-- comunidad_screen.dart—, así que cambiar una palabra exigía publicar una
-- versión nueva en las dos tiendas. Esta tabla la alimenta el panel
-- (Sitio-Administrativo, módulo "Páginas") y la app la pide por su slug en
-- GET /api/paginas/{slug}.
--
-- Archivos que dependen de ella:
--   internal/adapter/storage/postgres/pagina_repository.go
--   internal/core/services/pagina_service.go
--   internal/handler/http/pagina_handler.go
--   cmd/server/main.go (rutas /api/paginas/{slug} y /api/admin/paginas)
--
-- La clave es el slug y no un uuid a propósito: la app pide "legacy-board" por
-- su nombre, escrito en su propio código, y ese nombre no puede depender de un
-- identificador que cambia entre la base local y la de producción.
--
-- Es idempotente: se puede aplicar sobre una base que ya la tenga.

CREATE TABLE IF NOT EXISTS core.paginas_informativas (
    slug           text PRIMARY KEY,
    titulo         text NOT NULL,
    subtitulo      text NOT NULL DEFAULT '',
    imagen_url     text NOT NULL DEFAULT '',
    cuerpo         text NOT NULL DEFAULT '',
    publicada      boolean NOT NULL DEFAULT true,
    creada_en      timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    actualizada_en timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- El texto sembrado es un punto de partida editable, no contenido definitivo:
-- sale de la definición que ya publican los T&C de la app (literal (i)) y del
-- texto que hoy muestra el diálogo de Comunidad. Lo primero que hará el cliente
-- es reescribirlo desde el panel.
--
-- ON CONFLICT DO NOTHING: si la fila ya existe, la migración no pisa lo que
-- alguien haya editado.
INSERT INTO core.paginas_informativas (slug, titulo, subtitulo, cuerpo, publicada)
VALUES (
    'legacy-board',
    'Legacy Board',
    'Gobierno corporativo para familias empresarias',
    'Legacy Board es el espacio destinado a la interacción entre miembros interesados en la conformación, vinculación o participación en órganos de gobierno corporativo.

Aquí encontrará qué es el board, para qué sirve y cómo se participa en él.

Para ser parte del board o recibir más información, escríbanos desde Comunidad → Legacy Board.',
    true
)
ON CONFLICT (slug) DO NOTHING;
