/**
 ##
 ## OverDrive 2026
 ## All Technical rights reserved
 ##
 ## database_scheme.sql - Database scheme file for PostgreSQL
 ##
*/

---------------------------
-- USERS
---------------------------
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    email VARCHAR(255) UNIQUE NOT NULL,
    display_name VARCHAR(50) NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);


---------------------------
-- AUTH SESSIONS
---------------------------
CREATE TABLE IF NOT EXISTS auth_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);


---------------------------
-- PROVIDERS
---------------------------
CREATE TYPE provider_status AS ENUM ('active', 'beta', 'disabled');

CREATE TABLE IF NOT EXISTS providers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    code VARCHAR(20) UNIQUE NOT NULL,
    name VARCHAR(50) NOT NULL,
    status provider_status DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT NOW()
);


---------------------------
-- PROVIDER CONNECTIONS
---------------------------
CREATE TABLE IF NOT EXISTS provider_connections (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider_id UUID NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    config_json JSONB NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, provider_id)
);


---------------------------
-- USER PRESETS
---------------------------
CREATE TABLE IF NOT EXISTS user_presets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(50) NOT NULL,
    is_default BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);


---------------------------
-- PRESET WIDGETS
---------------------------
CREATE TYPE widget_type AS ENUM ('video'); -- TODO: Add more widget types

CREATE TABLE IF NOT EXISTS preset_widgets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    preset_id UUID NOT NULL REFERENCES user_presets(id) ON DELETE CASCADE,
    type widget_type NOT NULL,
    settings_json JSONB NOT NULL,
    pos_x FLOAT NOT NULL,
    pos_y FLOAT NOT NULL,
    pos_z FLOAT NOT NULL,
    rot_x FLOAT NOT NULL,
    rot_y FLOAT NOT NULL,
    rot_z FLOAT NOT NULL,
    width FLOAT NOT NULL,
    height FLOAT NOT NULL,
    order_index INT DEFAULT 0
);


---------------------------
-- EVENTS
---------------------------
CREATE TYPE event_status AS ENUM ('scheduled', 'live', 'finished');

CREATE TABLE IF NOT EXISTS events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    provider_id UUID NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    season_year INT NOT NULL,
    name VARCHAR(50) NOT NULL,
    location VARCHAR(255) NOT NULL,
    start_time_utc TIMESTAMPTZ NOT NULL,
    end_time_utc TIMESTAMPTZ NOT NULL,
    status event_status NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);


---------------------------
-- SESSIONS
---------------------------
CREATE TYPE session_type AS ENUM ('race', 'quali', 'practice');
CREATE TYPE session_status AS ENUM ('live', 'finished');

CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    type session_type NOT NULL,
    status session_status NOT NULL,
    started_at_utc TIMESTAMPTZ NOT NULL,
    ended_at_utc TIMESTAMPTZ
);


---------------------------
-- RACE HIGHLIGHTS
---------------------------
CREATE TABLE IF NOT EXISTS race_highlights (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    timestamp_ms INT NOT NULL,
    type VARCHAR(50) NOT NULL,
    summary TEXT NOT NULL,
    metadata_json JSONB NOT NULL
);


---------------------------
-- TEAMS
---------------------------
CREATE TABLE IF NOT EXISTS teams (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    provider_id UUID NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    name VARCHAR(50) NOT NULL
);


---------------------------
-- DRIVERS
---------------------------
CREATE TABLE IF NOT EXISTS drivers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    provider_id UUID NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    display_name VARCHAR(50) NOT NULL,
    code VARCHAR(20) UNIQUE NOT NULL,
    number INT NOT NULL
);
