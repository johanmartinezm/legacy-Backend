-- 20261125_legacy_board_formularios.sql
--
-- Crea el esquema `board`: los catálogos, los formularios A y B de Legacy Board
-- y los destinatarios del aviso de cada envío nuevo.
--
-- Contexto: fase 1 del plan reports/20261002_plan_legacy_board_formularios.md
-- (repositorio raíz, no versionado). El documento del cliente es
-- «Legacy_App Fase 1 - Legacy board formularios.docx», §7.1 (Formulario A,
-- postulación del consejero), §7.2 (Formulario B, solicitud del empresario o la
-- familia) y §8 (el matching entre los dos). Hasta hoy Legacy Board era solo una
-- página informativa (core.paginas_informativas) y el diálogo de dos contactos
-- de Comunidad (/api/board/contact): no había nada que guardar.
--
-- Tablas:
--
--   board.opciones   UNA sola tabla para todas las listas de los dos
--                    formularios. Si A y B tuvieran listas propias, el matching
--                    compararía opciones que se llaman igual pero no son la
--                    misma y fallaría sin avisar (§8 del documento). Las
--                    respuestas guardan el id, nunca el texto: renombrar una
--                    opción desde el panel no rompe lo ya enviado. Las opciones
--                    no se borran, se desactivan (activo = false): dejan de
--                    salir en la app y siguen leyéndose en lo enviado.
--                    `codigo` es la clave estable con la que el código se
--                    refiere a una opción («otro», que nunca puntúa; «remoto»;
--                    las que un formulario oculta). Las que cree el panel
--                    también llevan uno.
--
--   board.envios     Un registro por formulario enviado, A o B (`formulario`).
--                    Una sola tabla porque el panel tiene una sola bandeja para
--                    los dos y el estado funciona igual en ambos.
--                    - `respuestas` (jsonb) guarda lo que va en claro, por clave
--                      de campo: ids de opción, números, sí/no y textos no
--                      confidenciales. Es lo que lee el matching y lo que el
--                      panel filtra (sector, expertise).
--                    - `datos_cifrados` guarda, cifrado con
--                      security.CryptoService, un JSON con los datos
--                      personales y los textos confidenciales: nombre, tipo y
--                      número de documento, fecha de nacimiento, correo,
--                      teléfono, «Empresas / contextos» (A) y «Descripción del
--                      contexto» (B). Lo cifrado no se filtra ni se ordena en
--                      SQL: por eso nada de esto vive en `respuestas`.
--                    - Estado: pendiente → aprobada | rechazada. Se guarda solo
--                      el último cambio (quién, cuándo y el motivo, obligatorio
--                      al rechazar), no la secuencia completa: decidido el
--                      2026-10-02 para bajar horas.
--                    - El Formulario A es uno por persona: en la app no se edita
--                      ni se reenvía (decisión 5 del plan). El B admite varias
--                      solicitudes en el tiempo, pero solo una pendiente a la vez.
--
--   board.destinatarios_aviso
--                    Los correos que reciben el aviso de cada envío nuevo,
--                    editables desde el panel (decidido el 2026-11-13). Una sola
--                    lista para A y B. Vacía, el aviso va al buzón de contacto
--                    del config.yaml para que ninguno se pierda.
--
-- Siembra: los 18 catálogos con las opciones del documento y las erratas
-- resueltas el 2026-10-02 (§3.1 del plan):
--   A  cliente_alumni: 5 unidades más «No soy cliente», de selección simple.
--   B  tipo_documento: un solo catálogo de 6, con el Permiso de protección temporal.
--   C  rol: se añade «Otro».
--   D  pais: un solo catálogo para los dos formularios; la ciudad va en texto
--      libre, y «Remoto» es una opción más de este catálogo que solo ofrece el
--      campo de regiones del Formulario A (el resto de campos de país la oculta).
--   H  reto: catálogo propio, fuera del matching.
-- Los países son los mismos 28 del selector de indicativo de la app
-- (App-Movil/lib/domain/utils/indicativos.dart) más «Otro».
-- Los números de juntas y de comités (0 a 9) no son catálogo: son un rango fijo
-- y el formulario los valida como número.
--
-- Archivos que dependerán de ella (fases 2 a 6 del plan):
--   internal/core/domain, ports, adapter/storage/postgres, services y
--   handler/http del módulo board; cmd/server/main.go (rutas).
--
-- Aditiva e idempotente: CREATE ... IF NOT EXISTS y la siembra con
-- ON CONFLICT DO NOTHING, que no pisa lo que el panel haya renombrado,
-- reordenado o desactivado. Se puede correr dos veces sin efecto.

BEGIN;

CREATE SCHEMA IF NOT EXISTS board;

CREATE TABLE IF NOT EXISTS board.opciones (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    catalogo       text    NOT NULL,
    codigo         text    NOT NULL,
    etiqueta       text    NOT NULL,
    orden          integer NOT NULL DEFAULT 0,
    activo         boolean NOT NULL DEFAULT true,
    creado_en      timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    actualizado_en timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT opciones_codigo_formato CHECK (codigo ~ '^[a-z0-9_]+$'),
    CONSTRAINT opciones_etiqueta_no_vacia CHECK (btrim(etiqueta) <> '')
);

-- La clave estable de la siembra y lo que impide dos «otro» en un catálogo.
CREATE UNIQUE INDEX IF NOT EXISTS uq_opciones_catalogo_codigo
    ON board.opciones (catalogo, codigo);

-- La app pide un catálogo entero, ya ordenado.
CREATE INDEX IF NOT EXISTS idx_opciones_catalogo_orden
    ON board.opciones (catalogo, orden, id);

CREATE TABLE IF NOT EXISTS board.envios (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    formulario          char(1) NOT NULL,
    user_id             uuid    NOT NULL REFERENCES core.users(id) ON DELETE CASCADE,
    respuestas          jsonb   NOT NULL DEFAULT '{}'::jsonb,
    datos_cifrados      text    NOT NULL,
    foto_url            text    NOT NULL DEFAULT '',
    acepta_terminos     boolean NOT NULL,
    -- Sin ENUM a propósito, como core.contact_messages: añadir un estado a un
    -- tipo enumerado obliga a un ALTER TYPE que no es idempotente.
    estado              varchar(20) NOT NULL DEFAULT 'pendiente',
    estado_motivo       text    NOT NULL DEFAULT '',
    estado_cambiado_por uuid REFERENCES core.admin_users(id) ON DELETE SET NULL,
    estado_cambiado_en  timestamptz,
    -- Si el correo al comité no salió, el envío sigue aquí y el panel lo
    -- destaca, igual que en Contáctenos.
    aviso_enviado       boolean NOT NULL DEFAULT false,
    creado_en           timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    actualizado_en      timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT envios_formulario_valido CHECK (formulario IN ('A', 'B')),
    CONSTRAINT envios_estado_valido CHECK (estado IN ('pendiente', 'aprobada', 'rechazada')),
    CONSTRAINT envios_respuestas_objeto CHECK (jsonb_typeof(respuestas) = 'object'),
    CONSTRAINT envios_acepta_terminos CHECK (acepta_terminos),
    CONSTRAINT envios_rechazo_con_motivo CHECK (estado <> 'rechazada' OR btrim(estado_motivo) <> '')
);

-- Un solo Formulario A por persona: en la app no se reenvía.
CREATE UNIQUE INDEX IF NOT EXISTS uq_envios_un_formulario_a
    ON board.envios (user_id) WHERE formulario = 'A';

-- Del B, una sola solicitud pendiente a la vez.
CREATE UNIQUE INDEX IF NOT EXISTS uq_envios_un_b_pendiente
    ON board.envios (user_id) WHERE formulario = 'B' AND estado = 'pendiente';

-- La bandeja: un formulario, filtrado por estado, del más reciente al más
-- antiguo, con el id de desempate para que la paginación sea estable.
CREATE INDEX IF NOT EXISTS idx_envios_bandeja
    ON board.envios (formulario, estado, creado_en DESC, id DESC);

-- «Mis envíos» en la app.
CREATE INDEX IF NOT EXISTS idx_envios_usuario
    ON board.envios (user_id, creado_en DESC);

-- Los filtros por sector y expertise del panel. jsonb_path_ops solo sirve a
-- @>: filtrar con respuestas @> '{"sectores": ["<id>"]}', no con ?, que no
-- usa este índice.
CREATE INDEX IF NOT EXISTS idx_envios_respuestas
    ON board.envios USING gin (respuestas jsonb_path_ops);

COMMENT ON TABLE board.envios IS
    'Formularios A (consejero) y B (empresario) de Legacy Board. datos_cifrados va cifrado (AES-256): leerlo por fuera del servicio devuelve base64.';

CREATE TABLE IF NOT EXISTS board.destinatarios_aviso (
    correo    text PRIMARY KEY,
    creado_en timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT destinatarios_correo_minusculas CHECK (correo = lower(btrim(correo))),
    CONSTRAINT destinatarios_correo_formato CHECK (correo ~ '^[^@\s]+@[^@\s]+\.[^@\s]+$')
);

-- Siembra de los catálogos. El orden va de 10 en 10 para que el panel pueda
-- meter una opción entre dos sin renumerar.
INSERT INTO board.opciones (catalogo, codigo, etiqueta, orden)
SELECT v.catalogo, v.codigo, v.etiqueta, v.orden
  FROM (VALUES
    -- Tipo de documento (errata B)
    ('tipo_documento', 'cc',             'Cédula',                              10),
    ('tipo_documento', 'ce',             'Cédula de extranjería',               20),
    ('tipo_documento', 'ti',             'Tarjeta de identidad',                30),
    ('tipo_documento', 'pasaporte',      'Pasaporte',                           40),
    ('tipo_documento', 'doc_extranjero', 'Documento de identidad extranjero',   50),
    ('tipo_documento', 'ppt',            'Permiso de protección temporal',      60),

    -- País (errata D). «remoto» solo lo ofrece el campo de regiones del A.
    ('pais', 'colombia',             'Colombia',              10),
    ('pais', 'alemania',             'Alemania',              20),
    ('pais', 'argentina',            'Argentina',             30),
    ('pais', 'bolivia',              'Bolivia',               40),
    ('pais', 'brasil',               'Brasil',                50),
    ('pais', 'canada',               'Canadá',                60),
    ('pais', 'chile',                'Chile',                 70),
    ('pais', 'costa_rica',           'Costa Rica',            80),
    ('pais', 'ecuador',              'Ecuador',               90),
    ('pais', 'el_salvador',          'El Salvador',          100),
    ('pais', 'espana',               'España',               110),
    ('pais', 'estados_unidos',       'Estados Unidos',       120),
    ('pais', 'francia',              'Francia',              130),
    ('pais', 'guatemala',            'Guatemala',            140),
    ('pais', 'honduras',             'Honduras',             150),
    ('pais', 'italia',               'Italia',               160),
    ('pais', 'mexico',               'México',               170),
    ('pais', 'nicaragua',            'Nicaragua',            180),
    ('pais', 'panama',               'Panamá',               190),
    ('pais', 'paraguay',             'Paraguay',             200),
    ('pais', 'peru',                 'Perú',                 210),
    ('pais', 'portugal',             'Portugal',             220),
    ('pais', 'puerto_rico',          'Puerto Rico',          230),
    ('pais', 'reino_unido',          'Reino Unido',          240),
    ('pais', 'republica_dominicana', 'República Dominicana', 250),
    ('pais', 'suiza',                'Suiza',                260),
    ('pais', 'uruguay',              'Uruguay',              270),
    ('pais', 'venezuela',            'Venezuela',            280),
    ('pais', 'otro',                 'Otro',                 990),
    ('pais', 'remoto',               'Remoto',               999),

    -- ¿Es cliente o alumni de Legacy Network? (errata A, selección simple)
    ('cliente_alumni', 'lso',               'Legacy School of Ownership', 10),
    ('cliente_alumni', 'legacy_management', 'Legacy & Management',        20),
    ('cliente_alumni', 'legacy_legal',      'Legacy Legal',               30),
    ('cliente_alumni', 'aurum',             'Aurum',                      40),
    ('cliente_alumni', 'legacy_summit',     'Legacy Summit',              50),
    ('cliente_alumni', 'no_cliente',        'No soy cliente',             60),

    -- Nivel de cargo / rol (errata C: se añade «Otro»)
    ('rol', 'fundador',            'Fundador',                       10),
    ('rol', 'propietario',         'Propietario',                    20),
    ('rol', 'miembro_junta',       'Miembro de junta directiva',     30),
    ('rol', 'presidente',          'Presidente',                     40),
    ('rol', 'vicepresidente',      'Vicepresidente',                 50),
    ('rol', 'ceo',                 'CEO/Gerente general',            60),
    ('rol', 'socio',               'Socio',                          70),
    ('rol', 'director_ejecutivo',  'Director ejecutivo',             80),
    ('rol', 'gerente',             'Gerente',                        90),
    ('rol', 'director',            'Director',                      100),
    ('rol', 'jefe_area',           'Jefe/líder de área',            110),
    ('rol', 'consultor',           'Consultor',                     120),
    ('rol', 'asesor',              'Asesor',                        130),
    ('rol', 'consejero',           'Consejero o miembro consultivo', 140),
    ('rol', 'otro',                'Otro',                          990),

    -- Sector (matching 20 %)
    ('sector', 'manufactura',   'Manufactura / Industria',      10),
    ('sector', 'comercio',      'Comercio / Distribución',      20),
    ('sector', 'servicios_prof','Servicios profesionales',      30),
    ('sector', 'agro',          'Agroindustria / Agropecuario', 40),
    ('sector', 'inmobiliario',  'Inmobiliario / Construcción',  50),
    ('sector', 'financiero',    'Servicios financieros',        60),
    ('sector', 'salud',         'Salud',                        70),
    ('sector', 'educacion',     'Educación',                    80),
    ('sector', 'otro',          'Otro',                        990),

    -- Idioma (matching 10 %). El B no ofrece «Otro».
    ('idioma', 'es',   'Español',   10),
    ('idioma', 'en',   'Inglés',    20),
    ('idioma', 'pt',   'Portugués', 30),
    ('idioma', 'otro', 'Otro',     990),

    -- Tipo de órgano de gobierno (matching 20 %). El B no ofrece «No aplica».
    ('tipo_organo', 'junta_directiva',    'Junta directiva',                    10),
    ('tipo_organo', 'consejo_familia',    'Consejo de familia',                 20),
    ('tipo_organo', 'directorio',         'Directorio',                         30),
    ('tipo_organo', 'advisory_board',     'Advisory board',                     40),
    ('tipo_organo', 'comite_auditoria',   'Comité de auditoría',                50),
    ('tipo_organo', 'comite_riesgos',     'Comité de riesgos',                  60),
    ('tipo_organo', 'comite_remuneracion','Comité de evaluación y remuneración', 70),
    ('tipo_organo', 'comite_inversion',   'Comité de inversión/financiero',     80),
    ('tipo_organo', 'comites_apoyo',      'Comités de apoyo',                   90),
    ('tipo_organo', 'asamblea',           'Asamblea',                          100),
    ('tipo_organo', 'no_aplica',          'No aplica',                         990),

    -- Áreas de expertise (matching 20 %)
    ('expertise', 'gobierno_corporativo', 'Gobierno corporativo', 10),
    ('expertise', 'empresa_familiar',     'Empresa familiar',     20),
    ('expertise', 'sucesion',             'Sucesión',             30),
    ('expertise', 'mya',                  'M&A',                  40),
    ('expertise', 'finanzas',             'Finanzas/patrimonio',  50),
    ('expertise', 'legal',                'Legal/tributario',     60),
    ('expertise', 'estrategia',           'Estrategia',           70),
    ('expertise', 'esg',                  'ESG',                  80),
    ('expertise', 'internacionalizacion', 'Internacionalización', 90),
    ('expertise', 'tecnologia',           'Tecnología/IA',       100),

    -- Dimensiones DINASTÍA (matching 10 %)
    ('dinastia', 'cultura',    'Cultura',    10),
    ('dinastia', 'gobierno',   'Gobierno',   20),
    ('dinastia', 'estrategia', 'Estrategia', 30),
    ('dinastia', 'estructura', 'Estructura', 40),
    ('dinastia', 'sistemas',   'Sistemas',   50),
    ('dinastia', 'sucesion',   'Sucesión',   60),
    ('dinastia', 'individuo',  'Individuo',  70),

    -- Disponibilidad (juntas al mes)
    ('disponibilidad', '1_2',     '1–2', 10),
    ('disponibilidad', '3_4',     '3–4', 20),
    ('disponibilidad', '5_o_mas', '5+',  30),

    -- Modalidad (matching 5 %)
    ('modalidad', 'presencial', 'Presencial', 10),
    ('modalidad', 'virtual',    'Virtual',    20),
    ('modalidad', 'mixta',      'Mixta',      30),

    -- Honorarios, en dólares como en el documento (informativo, errata I)
    ('honorarios', 'r1', '0 - 1.353 USD',      10),
    ('honorarios', 'r2', '1.354 - 2.705 USD',  20),
    ('honorarios', 'r3', '2.706 - 5.410 USD',  30),
    ('honorarios', 'r4', '5.411 - 8.115 USD',  40),
    ('honorarios', 'r5', '8.116 - 10.820 USD', 50),

    -- Solo del Formulario B
    ('tipo_empresa', 'familiar',    'Empresa familiar',    10),
    ('tipo_empresa', 'no_familiar', 'Empresa no familiar', 20),
    ('tipo_empresa', 'estatal',     'Estatal',             30),
    ('tipo_empresa', 'ong',         'ONG',                 40),

    ('tamano_empresa', '0_10',     '0 - 10',      10),
    ('tamano_empresa', '11_50',    '11 - 50',     20),
    ('tamano_empresa', '51_200',   '51 - 200',    30),
    ('tamano_empresa', 'mas_200',  'Más de 200',  40),

    ('generacion', 'primera',  'Primera generación (fundador activo)', 10),
    ('generacion', 'segunda',  'Segunda generación',                   20),
    ('generacion', 'tercera',  'Tercera generación',                   30),
    ('generacion', 'cuarta',   'Cuarta generación o más',              40),

    ('estado_junta', 'no_tiene',   'No tiene',   10),
    ('estado_junta', 'ceremonial', 'Ceremonial', 20),
    ('estado_junta', 'activa',     'Activa',     30),

    -- Reto principal (errata H: fuera del matching)
    ('reto', 'gobierno_corporativo', 'Gobierno corporativo',  10),
    ('reto', 'profesionalizacion',   'Profesionalización',    20),
    ('reto', 'conflicto_familiar',   'Conflicto familiar',    30),
    ('reto', 'sucesion',             'Sucesión',              40),
    ('reto', 'mya',                  'M&A',                   50),
    ('reto', 'finanzas',             'Finanzas/patrimonio',   60),
    ('reto', 'legal',                'Legal/tributario',      70),
    ('reto', 'estrategia',           'Estrategia',            80),
    ('reto', 'esg',                  'ESG',                   90),
    ('reto', 'internacionalizacion', 'Internacionalización', 100),
    ('reto', 'tecnologia',           'Tecnología/IA',        110),
    ('reto', 'otro',                 'Otro',                 990),

    ('urgencia', 'inmediata',  'Inmediata',      10),
    ('urgencia', 'trimestre',  'Este trimestre', 20),
    ('urgencia', 'anio',       'Este año',       30),
    ('urgencia', 'explorando', 'Explorando',     40)
  ) AS v(catalogo, codigo, etiqueta, orden)
ON CONFLICT (catalogo, codigo) DO NOTHING;

COMMIT;
