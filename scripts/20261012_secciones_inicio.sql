-- 20261012_secciones_inicio.sql
--
-- Crea core.secciones_inicio y siembra una fila por tarjeta del Inicio de la app.
--
-- Contexto: el Inicio de la app (App-Movil/lib/presentation/screens/home/
-- home_content_screen.dart) estaba entero escrito en su código y no tenía ni una
-- imagen, solo iconos. El cliente pidió imágenes en todas las secciones y poder
-- cambiarlas desde el panel. Esta tabla guarda solo la imagen de cada tarjeta:
-- el título, el texto y a dónde lleva siguen en la app.
--
-- Archivos que dependen de ella:
--   internal/core/domain/seccion_inicio.go
--   internal/adapter/storage/postgres/seccion_inicio_repository.go
--   internal/core/services/seccion_inicio_service.go
--   internal/handler/http/seccion_inicio_handler.go
--   cmd/server/main.go (GET /api/inicio/secciones, GET y PUT /api/admin/inicio/secciones)
--
-- La clave es el slug, como en core.paginas_informativas: la app busca cada
-- tarjeta por su nombre, escrito en su propio código. «Beneficios» aparece dos
-- veces en el Inicio y comparte una sola fila, por decisión del cliente.
--
-- Idempotente: CREATE IF NOT EXISTS y ON CONFLICT DO NOTHING, para no pisar las
-- imágenes que ya se hayan cargado si se vuelve a aplicar.

CREATE TABLE IF NOT EXISTS core.secciones_inicio (
    slug           text PRIMARY KEY,
    nombre         text NOT NULL,
    orden          integer NOT NULL DEFAULT 0,
    imagen_url     text NOT NULL DEFAULT '',
    actualizada_en timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO core.secciones_inicio (slug, nombre, orden) VALUES
    ('nuevo-cada-semana', 'Nuevo cada semana',  10),
    ('legacy-board',      'Legacy Board',       20),
    ('legacy-knowledge',  'Legacy Knowledge',   30),
    ('eventos',           'Eventos',            40),
    ('lso-escuela',       'LSO · Escuela',      50),
    ('asesorias',         'Asesorías',          60),
    ('beneficios',        'Beneficios',         70),
    ('legacy-plus',       'Legacy+',            80),
    ('contactenos',       'Contáctenos',        90),
    ('miembros',          'Miembros',          100),
    ('legacy-test',       'Legacy Test',       110)
ON CONFLICT (slug) DO NOTHING;
