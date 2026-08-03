--
-- PostgreSQL database dump
--

\restrict v7Y9YFExDK6YGfaGwp7ZywKy130Kk3ZKQKTGBHnJhpUZ5y1PjdYvb4IYtvTg9LH

-- Dumped from database version 17.10
-- Dumped by pg_dump version 17.10

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: chat; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA chat;


--
-- Name: commerce; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA commerce;


--
-- Name: community; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA community;


--
-- Name: consultancy; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA consultancy;


--
-- Name: content; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA content;


--
-- Name: core; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA core;


--
-- Name: events; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA events;


--
-- Name: networking; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA networking;


--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: EXTENSION "uuid-ossp"; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION "uuid-ossp" IS 'generate universally unique identifiers (UUIDs)';


--
-- Name: user_role; Type: TYPE; Schema: core; Owner: -
--

CREATE TYPE core.user_role AS ENUM (
    'familia',
    'empresa',
    'profesional'
);


--
-- Name: content_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.content_type AS ENUM (
    'text',
    'video'
);


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: connections; Type: TABLE; Schema: chat; Owner: -
--

CREATE TABLE chat.connections (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    requester_id uuid NOT NULL,
    receiver_id uuid NOT NULL,
    status character varying(20) DEFAULT 'PENDING'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: messages; Type: TABLE; Schema: chat; Owner: -
--

CREATE TABLE chat.messages (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    connection_id uuid NOT NULL,
    sender_id uuid NOT NULL,
    content_encrypted text NOT NULL,
    is_read boolean DEFAULT false,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: synergies; Type: TABLE; Schema: community; Owner: -
--

CREATE TABLE community.synergies (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    author_id uuid NOT NULL,
    title character varying(255) NOT NULL,
    description text NOT NULL,
    category character varying(50) NOT NULL,
    image_url text,
    status character varying(20) DEFAULT 'active'::character varying,
    views_count integer DEFAULT 0,
    likes_count integer DEFAULT 0,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: TABLE synergies; Type: COMMENT; Schema: community; Owner: -
--

COMMENT ON TABLE community.synergies IS 'Almacena las propuestas e ideas de colaboración de los miembros.';


--
-- Name: synergy_comments; Type: TABLE; Schema: community; Owner: -
--

CREATE TABLE community.synergy_comments (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    synergy_id uuid NOT NULL,
    user_id uuid NOT NULL,
    content text NOT NULL,
    parent_comment_id uuid,
    is_expert_feedback boolean DEFAULT false,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: TABLE synergy_comments; Type: COMMENT; Schema: community; Owner: -
--

COMMENT ON TABLE community.synergy_comments IS 'Almacena las opiniones y debates generados en torno a una sinergia.';


--
-- Name: synergy_likes; Type: TABLE; Schema: community; Owner: -
--

CREATE TABLE community.synergy_likes (
    synergy_id uuid NOT NULL,
    user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: admin_users; Type: TABLE; Schema: core; Owner: -
--

CREATE TABLE core.admin_users (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    email text NOT NULL,
    password_hash text NOT NULL,
    first_name text,
    last_name text,
    role text DEFAULT 'admin'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);


--
-- Name: administrators; Type: TABLE; Schema: core; Owner: -
--

CREATE TABLE core.administrators (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    email character varying(255) NOT NULL,
    password_hash character varying(255) NOT NULL,
    full_name character varying(100) NOT NULL,
    is_active boolean DEFAULT true,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    last_login_at timestamp with time zone
);


--
-- Name: banners; Type: TABLE; Schema: core; Owner: -
--

CREATE TABLE core.banners (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    title text NOT NULL,
    subtitle text,
    image_url text NOT NULL,
    action_type text DEFAULT 'none'::text,
    action_target text,
    is_active boolean DEFAULT true,
    sort_order integer DEFAULT 0,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    category text DEFAULT 'home'::text
);


--
-- Name: content_categories; Type: TABLE; Schema: core; Owner: -
--

CREATE TABLE core.content_categories (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name character varying(100) NOT NULL,
    slug character varying(100) NOT NULL,
    description text,
    icon_url text,
    is_active boolean DEFAULT true,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: custom_contents; Type: TABLE; Schema: core; Owner: -
--

CREATE TABLE core.custom_contents (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    category_id uuid,
    type public.content_type NOT NULL,
    title character varying(255) NOT NULL,
    excerpt text,
    body_text text,
    video_url text,
    thumbnail_url text,
    is_published boolean DEFAULT false,
    published_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: custom_group_members; Type: TABLE; Schema: core; Owner: -
--

CREATE TABLE core.custom_group_members (
    group_id uuid NOT NULL,
    user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: custom_groups; Type: TABLE; Schema: core; Owner: -
--

CREATE TABLE core.custom_groups (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name character varying(255) NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: email_verification_tokens; Type: TABLE; Schema: core; Owner: -
--

CREATE TABLE core.email_verification_tokens (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    token character varying(255) NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: forum_post_reports; Type: TABLE; Schema: core; Owner: -
--

CREATE TABLE core.forum_post_reports (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    post_id uuid NOT NULL,
    reporter_id uuid NOT NULL,
    reason text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: forum_posts; Type: TABLE; Schema: core; Owner: -
--

CREATE TABLE core.forum_posts (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    forum_id uuid NOT NULL,
    user_id uuid NOT NULL,
    parent_id uuid,
    content text NOT NULL,
    image_url text,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    report_count integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT forum_posts_content_check CHECK ((char_length(content) >= 1)),
    CONSTRAINT forum_posts_status_check CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'deleted'::character varying, 'flagged'::character varying])::text[])))
);


--
-- Name: forums; Type: TABLE; Schema: core; Owner: -
--

CREATE TABLE core.forums (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    title character varying(255) NOT NULL,
    description text,
    cover_url text,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_by_user_id uuid,
    created_by_admin boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT forums_status_check CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'locked'::character varying, 'hidden'::character varying, 'deleted'::character varying])::text[])))
);


--
-- Name: modules; Type: TABLE; Schema: core; Owner: -
--

CREATE TABLE core.modules (
    id integer NOT NULL,
    key character varying(50) NOT NULL,
    name character varying(100) NOT NULL,
    description text,
    icon_url character varying(255),
    is_active boolean DEFAULT true,
    display_order integer
);


--
-- Name: modules_id_seq; Type: SEQUENCE; Schema: core; Owner: -
--

CREATE SEQUENCE core.modules_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: modules_id_seq; Type: SEQUENCE OWNED BY; Schema: core; Owner: -
--

ALTER SEQUENCE core.modules_id_seq OWNED BY core.modules.id;


--
-- Name: notifications_history; Type: TABLE; Schema: core; Owner: -
--

CREATE TABLE core.notifications_history (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    admin_id uuid,
    title character varying(255) NOT NULL,
    body text NOT NULL,
    target_type character varying(20) NOT NULL,
    target_value character varying(255),
    sent_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    status character varying(20) DEFAULT 'sent'::character varying
);


--
-- Name: TABLE notifications_history; Type: COMMENT; Schema: core; Owner: -
--

COMMENT ON TABLE core.notifications_history IS 'Historial de notificaciones push despachadas por la administración.';


--
-- Name: password_reset_tokens; Type: TABLE; Schema: core; Owner: -
--

CREATE TABLE core.password_reset_tokens (
    email character varying(255) NOT NULL,
    token character varying(255) NOT NULL,
    expires_at timestamp without time zone NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: post_likes; Type: TABLE; Schema: core; Owner: -
--

CREATE TABLE core.post_likes (
    id integer NOT NULL,
    user_id uuid NOT NULL,
    post_id character varying(255) NOT NULL,
    created_at timestamp without time zone DEFAULT now()
);


--
-- Name: post_likes_id_seq; Type: SEQUENCE; Schema: core; Owner: -
--

CREATE SEQUENCE core.post_likes_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: post_likes_id_seq; Type: SEQUENCE OWNED BY; Schema: core; Owner: -
--

ALTER SEQUENCE core.post_likes_id_seq OWNED BY core.post_likes.id;


--
-- Name: post_views; Type: TABLE; Schema: core; Owner: -
--

CREATE TABLE core.post_views (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    post_id text NOT NULL,
    user_id uuid,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    title text
);


--
-- Name: transactions; Type: TABLE; Schema: core; Owner: -
--

CREATE TABLE core.transactions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    reference_type character varying(20) NOT NULL,
    reference_id uuid NOT NULL,
    amount numeric(10,2) NOT NULL,
    credibanco_order_id character varying(255),
    status character varying(20) DEFAULT 'PENDING'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: user_fcm_tokens; Type: TABLE; Schema: core; Owner: -
--

CREATE TABLE core.user_fcm_tokens (
    user_id uuid NOT NULL,
    fcm_token text NOT NULL,
    device_type character varying(20) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: TABLE user_fcm_tokens; Type: COMMENT; Schema: core; Owner: -
--

COMMENT ON TABLE core.user_fcm_tokens IS 'Almacena los tokens FCM asociados a cada dispositivo de un usuario.';


--
-- Name: user_interests; Type: TABLE; Schema: core; Owner: -
--

CREATE TABLE core.user_interests (
    user_id uuid NOT NULL,
    interest character varying(255) NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: users; Type: TABLE; Schema: core; Owner: -
--

CREATE TABLE core.users (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    email_blind_index character varying(255) NOT NULL,
    password_hash character varying(255) NOT NULL,
    first_name text NOT NULL,
    last_name text NOT NULL,
    phone text,
    location text,
    role core.user_role DEFAULT 'familia'::core.user_role NOT NULL,
    bio text,
    industry character varying(100),
    profile_image_url character varying(255),
    generation character varying(50),
    company_name text,
    job_title text,
    is_public_profile boolean DEFAULT true,
    allow_messages_from_strangers boolean DEFAULT true,
    show_activity boolean DEFAULT true,
    refresh_token text,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    email_encrypted text,
    country character varying(100) DEFAULT 'Colombia'::character varying,
    identification_type character varying(50),
    identification_number text,
    customer_status character varying(50),
    birth_date date,
    terms_accepted boolean DEFAULT false,
    data_sharing_accepted boolean DEFAULT false,
    email_verified boolean DEFAULT false,
    alias character varying(60)
);


--
-- Name: attendance_logs; Type: TABLE; Schema: events; Owner: -
--

CREATE TABLE events.attendance_logs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    registration_id uuid NOT NULL,
    staff_user_id uuid NOT NULL,
    check_in_time timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    notes text
);


--
-- Name: categories; Type: TABLE; Schema: events; Owner: -
--

CREATE TABLE events.categories (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name character varying(50) NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    order_index integer DEFAULT 0
);


--
-- Name: events; Type: TABLE; Schema: events; Owner: -
--

CREATE TABLE events.events (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    category_id uuid,
    title character varying(255) NOT NULL,
    description text,
    image_url text,
    location character varying(255),
    speaker_main character varying(255),
    start_date date NOT NULL,
    end_date date,
    price numeric(10,2) DEFAULT 0.00,
    is_free boolean DEFAULT false,
    action_status character varying(50),
    button_text character varying(50),
    attendees_limit integer,
    status character varying(20) DEFAULT 'active'::character varying,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    includes text
);


--
-- Name: COLUMN events.includes; Type: COMMENT; Schema: events; Owner: -
--

COMMENT ON COLUMN events.events.includes IS 'Lista de elementos incluidos en el evento, separados por saltos de línea o formato JSON';


--
-- Name: registration_workshops; Type: TABLE; Schema: events; Owner: -
--

CREATE TABLE events.registration_workshops (
    registration_id uuid NOT NULL,
    workshop_id uuid NOT NULL
);


--
-- Name: registrations; Type: TABLE; Schema: events; Owner: -
--

CREATE TABLE events.registrations (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    event_id uuid,
    payment_status character varying(20) DEFAULT 'pending'::character varying,
    registration_status character varying(20) DEFAULT 'confirmed'::character varying,
    registration_date timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    qr_data text,
    total_paid numeric(10,2) DEFAULT 0.00,
    attendance_confirmed boolean DEFAULT false,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: user_agenda; Type: TABLE; Schema: events; Owner: -
--

CREATE TABLE events.user_agenda (
    user_id uuid NOT NULL,
    workshop_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: workshop_ratings; Type: TABLE; Schema: events; Owner: -
--

CREATE TABLE events.workshop_ratings (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    workshop_id uuid NOT NULL,
    user_id uuid NOT NULL,
    rating integer NOT NULL,
    comment text,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT workshop_ratings_rating_check CHECK (((rating >= 1) AND (rating <= 5)))
);


--
-- Name: workshops; Type: TABLE; Schema: events; Owner: -
--

CREATE TABLE events.workshops (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    event_id uuid,
    name character varying(255) NOT NULL,
    description text,
    image_url text,
    room character varying(100),
    speaker character varying(255),
    start_date_time timestamp with time zone NOT NULL,
    end_date_time timestamp with time zone NOT NULL,
    attendees_limit integer,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: modules id; Type: DEFAULT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.modules ALTER COLUMN id SET DEFAULT nextval('core.modules_id_seq'::regclass);


--
-- Name: post_likes id; Type: DEFAULT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.post_likes ALTER COLUMN id SET DEFAULT nextval('core.post_likes_id_seq'::regclass);


--
-- Name: connections connections_pkey; Type: CONSTRAINT; Schema: chat; Owner: -
--

ALTER TABLE ONLY chat.connections
    ADD CONSTRAINT connections_pkey PRIMARY KEY (id);


--
-- Name: connections connections_requester_id_receiver_id_key; Type: CONSTRAINT; Schema: chat; Owner: -
--

ALTER TABLE ONLY chat.connections
    ADD CONSTRAINT connections_requester_id_receiver_id_key UNIQUE (requester_id, receiver_id);


--
-- Name: messages messages_pkey; Type: CONSTRAINT; Schema: chat; Owner: -
--

ALTER TABLE ONLY chat.messages
    ADD CONSTRAINT messages_pkey PRIMARY KEY (id);


--
-- Name: synergies synergies_pkey; Type: CONSTRAINT; Schema: community; Owner: -
--

ALTER TABLE ONLY community.synergies
    ADD CONSTRAINT synergies_pkey PRIMARY KEY (id);


--
-- Name: synergy_comments synergy_comments_pkey; Type: CONSTRAINT; Schema: community; Owner: -
--

ALTER TABLE ONLY community.synergy_comments
    ADD CONSTRAINT synergy_comments_pkey PRIMARY KEY (id);


--
-- Name: synergy_likes synergy_likes_pkey; Type: CONSTRAINT; Schema: community; Owner: -
--

ALTER TABLE ONLY community.synergy_likes
    ADD CONSTRAINT synergy_likes_pkey PRIMARY KEY (synergy_id, user_id);


--
-- Name: admin_users admin_users_email_key; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.admin_users
    ADD CONSTRAINT admin_users_email_key UNIQUE (email);


--
-- Name: admin_users admin_users_pkey; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.admin_users
    ADD CONSTRAINT admin_users_pkey PRIMARY KEY (id);


--
-- Name: administrators administrators_email_key; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.administrators
    ADD CONSTRAINT administrators_email_key UNIQUE (email);


--
-- Name: administrators administrators_pkey; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.administrators
    ADD CONSTRAINT administrators_pkey PRIMARY KEY (id);


--
-- Name: banners banners_pkey; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.banners
    ADD CONSTRAINT banners_pkey PRIMARY KEY (id);


--
-- Name: content_categories content_categories_pkey; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.content_categories
    ADD CONSTRAINT content_categories_pkey PRIMARY KEY (id);


--
-- Name: content_categories content_categories_slug_key; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.content_categories
    ADD CONSTRAINT content_categories_slug_key UNIQUE (slug);


--
-- Name: custom_contents custom_contents_pkey; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.custom_contents
    ADD CONSTRAINT custom_contents_pkey PRIMARY KEY (id);


--
-- Name: custom_group_members custom_group_members_pkey; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.custom_group_members
    ADD CONSTRAINT custom_group_members_pkey PRIMARY KEY (group_id, user_id);


--
-- Name: custom_groups custom_groups_name_key; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.custom_groups
    ADD CONSTRAINT custom_groups_name_key UNIQUE (name);


--
-- Name: custom_groups custom_groups_pkey; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.custom_groups
    ADD CONSTRAINT custom_groups_pkey PRIMARY KEY (id);


--
-- Name: email_verification_tokens email_verification_tokens_pkey; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.email_verification_tokens
    ADD CONSTRAINT email_verification_tokens_pkey PRIMARY KEY (id);


--
-- Name: forum_post_reports forum_post_reports_pkey; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.forum_post_reports
    ADD CONSTRAINT forum_post_reports_pkey PRIMARY KEY (id);


--
-- Name: forum_post_reports forum_post_reports_post_id_reporter_id_key; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.forum_post_reports
    ADD CONSTRAINT forum_post_reports_post_id_reporter_id_key UNIQUE (post_id, reporter_id);


--
-- Name: forum_posts forum_posts_pkey; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.forum_posts
    ADD CONSTRAINT forum_posts_pkey PRIMARY KEY (id);


--
-- Name: forums forums_pkey; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.forums
    ADD CONSTRAINT forums_pkey PRIMARY KEY (id);


--
-- Name: modules modules_key_key; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.modules
    ADD CONSTRAINT modules_key_key UNIQUE (key);


--
-- Name: modules modules_pkey; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.modules
    ADD CONSTRAINT modules_pkey PRIMARY KEY (id);


--
-- Name: notifications_history notifications_history_pkey; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.notifications_history
    ADD CONSTRAINT notifications_history_pkey PRIMARY KEY (id);


--
-- Name: password_reset_tokens password_reset_tokens_pkey; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_pkey PRIMARY KEY (email);


--
-- Name: post_likes post_likes_pkey; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.post_likes
    ADD CONSTRAINT post_likes_pkey PRIMARY KEY (id);


--
-- Name: post_likes post_likes_user_id_post_id_key; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.post_likes
    ADD CONSTRAINT post_likes_user_id_post_id_key UNIQUE (user_id, post_id);


--
-- Name: post_views post_views_pkey; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.post_views
    ADD CONSTRAINT post_views_pkey PRIMARY KEY (id);


--
-- Name: transactions transactions_pkey; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.transactions
    ADD CONSTRAINT transactions_pkey PRIMARY KEY (id);


--
-- Name: user_fcm_tokens user_fcm_tokens_pkey; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.user_fcm_tokens
    ADD CONSTRAINT user_fcm_tokens_pkey PRIMARY KEY (user_id, fcm_token);


--
-- Name: user_interests user_interests_pkey; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.user_interests
    ADD CONSTRAINT user_interests_pkey PRIMARY KEY (user_id, interest);


--
-- Name: users users_alias_key; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.users
    ADD CONSTRAINT users_alias_key UNIQUE (alias);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.users
    ADD CONSTRAINT users_email_key UNIQUE (email_blind_index);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: attendance_logs attendance_logs_pkey; Type: CONSTRAINT; Schema: events; Owner: -
--

ALTER TABLE ONLY events.attendance_logs
    ADD CONSTRAINT attendance_logs_pkey PRIMARY KEY (id);


--
-- Name: categories categories_name_key; Type: CONSTRAINT; Schema: events; Owner: -
--

ALTER TABLE ONLY events.categories
    ADD CONSTRAINT categories_name_key UNIQUE (name);


--
-- Name: categories categories_pkey; Type: CONSTRAINT; Schema: events; Owner: -
--

ALTER TABLE ONLY events.categories
    ADD CONSTRAINT categories_pkey PRIMARY KEY (id);


--
-- Name: events events_pkey; Type: CONSTRAINT; Schema: events; Owner: -
--

ALTER TABLE ONLY events.events
    ADD CONSTRAINT events_pkey PRIMARY KEY (id);


--
-- Name: registration_workshops registration_workshops_pkey; Type: CONSTRAINT; Schema: events; Owner: -
--

ALTER TABLE ONLY events.registration_workshops
    ADD CONSTRAINT registration_workshops_pkey PRIMARY KEY (registration_id, workshop_id);


--
-- Name: registrations registrations_pkey; Type: CONSTRAINT; Schema: events; Owner: -
--

ALTER TABLE ONLY events.registrations
    ADD CONSTRAINT registrations_pkey PRIMARY KEY (id);


--
-- Name: user_agenda user_agenda_pkey; Type: CONSTRAINT; Schema: events; Owner: -
--

ALTER TABLE ONLY events.user_agenda
    ADD CONSTRAINT user_agenda_pkey PRIMARY KEY (user_id, workshop_id);


--
-- Name: workshop_ratings workshop_ratings_pkey; Type: CONSTRAINT; Schema: events; Owner: -
--

ALTER TABLE ONLY events.workshop_ratings
    ADD CONSTRAINT workshop_ratings_pkey PRIMARY KEY (id);


--
-- Name: workshop_ratings workshop_ratings_workshop_id_user_id_key; Type: CONSTRAINT; Schema: events; Owner: -
--

ALTER TABLE ONLY events.workshop_ratings
    ADD CONSTRAINT workshop_ratings_workshop_id_user_id_key UNIQUE (workshop_id, user_id);


--
-- Name: workshops workshops_pkey; Type: CONSTRAINT; Schema: events; Owner: -
--

ALTER TABLE ONLY events.workshops
    ADD CONSTRAINT workshops_pkey PRIMARY KEY (id);


--
-- Name: idx_chat_connections_users; Type: INDEX; Schema: chat; Owner: -
--

CREATE INDEX idx_chat_connections_users ON chat.connections USING btree (requester_id, receiver_id);


--
-- Name: idx_chat_messages_connection; Type: INDEX; Schema: chat; Owner: -
--

CREATE INDEX idx_chat_messages_connection ON chat.messages USING btree (connection_id);


--
-- Name: idx_comments_synergy; Type: INDEX; Schema: community; Owner: -
--

CREATE INDEX idx_comments_synergy ON community.synergy_comments USING btree (synergy_id);


--
-- Name: idx_synergies_author; Type: INDEX; Schema: community; Owner: -
--

CREATE INDEX idx_synergies_author ON community.synergies USING btree (author_id);


--
-- Name: idx_synergies_status; Type: INDEX; Schema: community; Owner: -
--

CREATE INDEX idx_synergies_status ON community.synergies USING btree (status);


--
-- Name: idx_custom_contents_category_id; Type: INDEX; Schema: core; Owner: -
--

CREATE INDEX idx_custom_contents_category_id ON core.custom_contents USING btree (category_id);


--
-- Name: idx_custom_contents_is_published; Type: INDEX; Schema: core; Owner: -
--

CREATE INDEX idx_custom_contents_is_published ON core.custom_contents USING btree (is_published);


--
-- Name: idx_custom_contents_type; Type: INDEX; Schema: core; Owner: -
--

CREATE INDEX idx_custom_contents_type ON core.custom_contents USING btree (type);


--
-- Name: idx_forum_posts_forum_id; Type: INDEX; Schema: core; Owner: -
--

CREATE INDEX idx_forum_posts_forum_id ON core.forum_posts USING btree (forum_id);


--
-- Name: idx_forum_posts_status; Type: INDEX; Schema: core; Owner: -
--

CREATE INDEX idx_forum_posts_status ON core.forum_posts USING btree (status);


--
-- Name: idx_forums_created_by; Type: INDEX; Schema: core; Owner: -
--

CREATE INDEX idx_forums_created_by ON core.forums USING btree (created_by_user_id);


--
-- Name: idx_forums_status; Type: INDEX; Schema: core; Owner: -
--

CREATE INDEX idx_forums_status ON core.forums USING btree (status);


--
-- Name: idx_post_likes_post_id; Type: INDEX; Schema: core; Owner: -
--

CREATE INDEX idx_post_likes_post_id ON core.post_likes USING btree (post_id);


--
-- Name: idx_post_views_post_id; Type: INDEX; Schema: core; Owner: -
--

CREATE INDEX idx_post_views_post_id ON core.post_views USING btree (post_id);


--
-- Name: idx_transactions_credibanco_order_id; Type: INDEX; Schema: core; Owner: -
--

CREATE INDEX idx_transactions_credibanco_order_id ON core.transactions USING btree (credibanco_order_id);


--
-- Name: idx_transactions_user_id; Type: INDEX; Schema: core; Owner: -
--

CREATE INDEX idx_transactions_user_id ON core.transactions USING btree (user_id);


--
-- Name: idx_user_fcm_tokens_user; Type: INDEX; Schema: core; Owner: -
--

CREATE INDEX idx_user_fcm_tokens_user ON core.user_fcm_tokens USING btree (user_id);


--
-- Name: idx_users_alias; Type: INDEX; Schema: core; Owner: -
--

CREATE INDEX idx_users_alias ON core.users USING btree (alias) WHERE (alias IS NOT NULL);


--
-- Name: idx_attendance_registration; Type: INDEX; Schema: events; Owner: -
--

CREATE INDEX idx_attendance_registration ON events.attendance_logs USING btree (registration_id);


--
-- Name: idx_workshop_ratings_user_id; Type: INDEX; Schema: events; Owner: -
--

CREATE INDEX idx_workshop_ratings_user_id ON events.workshop_ratings USING btree (user_id);


--
-- Name: idx_workshop_ratings_workshop_id; Type: INDEX; Schema: events; Owner: -
--

CREATE INDEX idx_workshop_ratings_workshop_id ON events.workshop_ratings USING btree (workshop_id);


--
-- Name: connections connections_receiver_id_fkey; Type: FK CONSTRAINT; Schema: chat; Owner: -
--

ALTER TABLE ONLY chat.connections
    ADD CONSTRAINT connections_receiver_id_fkey FOREIGN KEY (receiver_id) REFERENCES core.users(id) ON DELETE CASCADE;


--
-- Name: connections connections_requester_id_fkey; Type: FK CONSTRAINT; Schema: chat; Owner: -
--

ALTER TABLE ONLY chat.connections
    ADD CONSTRAINT connections_requester_id_fkey FOREIGN KEY (requester_id) REFERENCES core.users(id) ON DELETE CASCADE;


--
-- Name: messages messages_connection_id_fkey; Type: FK CONSTRAINT; Schema: chat; Owner: -
--

ALTER TABLE ONLY chat.messages
    ADD CONSTRAINT messages_connection_id_fkey FOREIGN KEY (connection_id) REFERENCES chat.connections(id) ON DELETE CASCADE;


--
-- Name: messages messages_sender_id_fkey; Type: FK CONSTRAINT; Schema: chat; Owner: -
--

ALTER TABLE ONLY chat.messages
    ADD CONSTRAINT messages_sender_id_fkey FOREIGN KEY (sender_id) REFERENCES core.users(id);


--
-- Name: synergy_comments synergy_comments_parent_comment_id_fkey; Type: FK CONSTRAINT; Schema: community; Owner: -
--

ALTER TABLE ONLY community.synergy_comments
    ADD CONSTRAINT synergy_comments_parent_comment_id_fkey FOREIGN KEY (parent_comment_id) REFERENCES community.synergy_comments(id);


--
-- Name: synergy_comments synergy_comments_synergy_id_fkey; Type: FK CONSTRAINT; Schema: community; Owner: -
--

ALTER TABLE ONLY community.synergy_comments
    ADD CONSTRAINT synergy_comments_synergy_id_fkey FOREIGN KEY (synergy_id) REFERENCES community.synergies(id) ON DELETE CASCADE;


--
-- Name: synergy_likes synergy_likes_synergy_id_fkey; Type: FK CONSTRAINT; Schema: community; Owner: -
--

ALTER TABLE ONLY community.synergy_likes
    ADD CONSTRAINT synergy_likes_synergy_id_fkey FOREIGN KEY (synergy_id) REFERENCES community.synergies(id) ON DELETE CASCADE;


--
-- Name: custom_contents custom_contents_category_id_fkey; Type: FK CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.custom_contents
    ADD CONSTRAINT custom_contents_category_id_fkey FOREIGN KEY (category_id) REFERENCES core.content_categories(id) ON DELETE SET NULL;


--
-- Name: custom_group_members custom_group_members_group_id_fkey; Type: FK CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.custom_group_members
    ADD CONSTRAINT custom_group_members_group_id_fkey FOREIGN KEY (group_id) REFERENCES core.custom_groups(id) ON DELETE CASCADE;


--
-- Name: custom_group_members custom_group_members_user_id_fkey; Type: FK CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.custom_group_members
    ADD CONSTRAINT custom_group_members_user_id_fkey FOREIGN KEY (user_id) REFERENCES core.users(id) ON DELETE CASCADE;


--
-- Name: email_verification_tokens email_verification_tokens_user_id_fkey; Type: FK CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.email_verification_tokens
    ADD CONSTRAINT email_verification_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES core.users(id) ON DELETE CASCADE;


--
-- Name: forum_post_reports forum_post_reports_post_id_fkey; Type: FK CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.forum_post_reports
    ADD CONSTRAINT forum_post_reports_post_id_fkey FOREIGN KEY (post_id) REFERENCES core.forum_posts(id) ON DELETE CASCADE;


--
-- Name: forum_post_reports forum_post_reports_reporter_id_fkey; Type: FK CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.forum_post_reports
    ADD CONSTRAINT forum_post_reports_reporter_id_fkey FOREIGN KEY (reporter_id) REFERENCES core.users(id) ON DELETE CASCADE;


--
-- Name: forum_posts forum_posts_forum_id_fkey; Type: FK CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.forum_posts
    ADD CONSTRAINT forum_posts_forum_id_fkey FOREIGN KEY (forum_id) REFERENCES core.forums(id) ON DELETE CASCADE;


--
-- Name: forum_posts forum_posts_parent_id_fkey; Type: FK CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.forum_posts
    ADD CONSTRAINT forum_posts_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES core.forum_posts(id) ON DELETE CASCADE;


--
-- Name: forum_posts forum_posts_user_id_fkey; Type: FK CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.forum_posts
    ADD CONSTRAINT forum_posts_user_id_fkey FOREIGN KEY (user_id) REFERENCES core.users(id) ON DELETE CASCADE;


--
-- Name: forums forums_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.forums
    ADD CONSTRAINT forums_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES core.users(id) ON DELETE SET NULL;


--
-- Name: notifications_history notifications_history_admin_id_fkey; Type: FK CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.notifications_history
    ADD CONSTRAINT notifications_history_admin_id_fkey FOREIGN KEY (admin_id) REFERENCES core.admin_users(id) ON DELETE SET NULL;


--
-- Name: post_likes post_likes_user_id_fkey; Type: FK CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.post_likes
    ADD CONSTRAINT post_likes_user_id_fkey FOREIGN KEY (user_id) REFERENCES core.users(id) ON DELETE CASCADE;


--
-- Name: post_views post_views_user_id_fkey; Type: FK CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.post_views
    ADD CONSTRAINT post_views_user_id_fkey FOREIGN KEY (user_id) REFERENCES core.users(id);


--
-- Name: transactions transactions_user_id_fkey; Type: FK CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.transactions
    ADD CONSTRAINT transactions_user_id_fkey FOREIGN KEY (user_id) REFERENCES core.users(id) ON DELETE CASCADE;


--
-- Name: user_fcm_tokens user_fcm_tokens_user_id_fkey; Type: FK CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.user_fcm_tokens
    ADD CONSTRAINT user_fcm_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES core.users(id) ON DELETE CASCADE;


--
-- Name: user_interests user_interests_user_id_fkey; Type: FK CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.user_interests
    ADD CONSTRAINT user_interests_user_id_fkey FOREIGN KEY (user_id) REFERENCES core.users(id) ON DELETE CASCADE;


--
-- Name: attendance_logs attendance_logs_registration_id_fkey; Type: FK CONSTRAINT; Schema: events; Owner: -
--

ALTER TABLE ONLY events.attendance_logs
    ADD CONSTRAINT attendance_logs_registration_id_fkey FOREIGN KEY (registration_id) REFERENCES events.registrations(id) ON DELETE CASCADE;


--
-- Name: events events_category_id_fkey; Type: FK CONSTRAINT; Schema: events; Owner: -
--

ALTER TABLE ONLY events.events
    ADD CONSTRAINT events_category_id_fkey FOREIGN KEY (category_id) REFERENCES events.categories(id);


--
-- Name: registration_workshops registration_workshops_registration_id_fkey; Type: FK CONSTRAINT; Schema: events; Owner: -
--

ALTER TABLE ONLY events.registration_workshops
    ADD CONSTRAINT registration_workshops_registration_id_fkey FOREIGN KEY (registration_id) REFERENCES events.registrations(id) ON DELETE CASCADE;


--
-- Name: registration_workshops registration_workshops_workshop_id_fkey; Type: FK CONSTRAINT; Schema: events; Owner: -
--

ALTER TABLE ONLY events.registration_workshops
    ADD CONSTRAINT registration_workshops_workshop_id_fkey FOREIGN KEY (workshop_id) REFERENCES events.workshops(id) ON DELETE CASCADE;


--
-- Name: registrations registrations_event_id_fkey; Type: FK CONSTRAINT; Schema: events; Owner: -
--

ALTER TABLE ONLY events.registrations
    ADD CONSTRAINT registrations_event_id_fkey FOREIGN KEY (event_id) REFERENCES events.events(id);


--
-- Name: user_agenda user_agenda_workshop_id_fkey; Type: FK CONSTRAINT; Schema: events; Owner: -
--

ALTER TABLE ONLY events.user_agenda
    ADD CONSTRAINT user_agenda_workshop_id_fkey FOREIGN KEY (workshop_id) REFERENCES events.workshops(id) ON DELETE CASCADE;


--
-- Name: workshop_ratings workshop_ratings_workshop_id_fkey; Type: FK CONSTRAINT; Schema: events; Owner: -
--

ALTER TABLE ONLY events.workshop_ratings
    ADD CONSTRAINT workshop_ratings_workshop_id_fkey FOREIGN KEY (workshop_id) REFERENCES events.workshops(id) ON DELETE CASCADE;


--
-- Name: workshops workshops_event_id_fkey; Type: FK CONSTRAINT; Schema: events; Owner: -
--

ALTER TABLE ONLY events.workshops
    ADD CONSTRAINT workshops_event_id_fkey FOREIGN KEY (event_id) REFERENCES events.events(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

\unrestrict v7Y9YFExDK6YGfaGwp7ZywKy130Kk3ZKQKTGBHnJhpUZ5y1PjdYvb4IYtvTg9LH

