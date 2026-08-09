-- zakupki-parser search service: users, sessions, searchers, hits
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE users (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  login           TEXT NOT NULL UNIQUE,
  password_hash   TEXT NOT NULL,
  display_name    TEXT NOT NULL DEFAULT '',
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash   TEXT NOT NULL UNIQUE,
  expires_at   TIMESTAMPTZ NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX sessions_user_id_idx ON sessions(user_id);
CREATE INDEX sessions_expires_at_idx ON sessions(expires_at);

CREATE TABLE searchers (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name         TEXT NOT NULL,
  config       JSONB NOT NULL DEFAULT '{}'::jsonb,
  auto_ai      BOOLEAN NOT NULL DEFAULT false,
  last_run_at  TIMESTAMPTZ,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, name)
);

CREATE INDEX searchers_user_id_idx ON searchers(user_id);
CREATE INDEX searchers_config_gin ON searchers USING GIN (config);

CREATE TABLE searcher_tenders (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  searcher_id      UUID NOT NULL REFERENCES searchers(id) ON DELETE CASCADE,
  reg_number       TEXT NOT NULL,
  object_name      TEXT NOT NULL DEFAULT '',
  notice_url       TEXT NOT NULL DEFAULT '',
  nmck             DOUBLE PRECISION,
  application_end  TEXT NOT NULL DEFAULT '',
  law              TEXT NOT NULL DEFAULT '',
  found_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (searcher_id, reg_number)
);

CREATE INDEX searcher_tenders_searcher_id_idx ON searcher_tenders(searcher_id);
CREATE INDEX searcher_tenders_reg_number_idx ON searcher_tenders(reg_number);

-- Demo user: login=demo / password=demo
-- bcrypt hash for "demo" (cost 10)
INSERT INTO users (login, password_hash, display_name)
VALUES (
  'demo',
  '$2a$10$7DVFvBs46wuWHeLQG9WIie5l5bUGD/H2TiUrU5eoK2cMW2cqFzuqW',
  'Demo User'
);

INSERT INTO searchers (user_id, name, config, auto_ai)
SELECT
  u.id,
  'ПО и разработка',
  '{
    "search_string": "Разработка ПО",
    "morphology": true,
    "strict_equal": false,
    "fz44": true,
    "fz223": true,
    "pp_rf615": false,
    "stage_af": true,
    "stage_ca": true,
    "stage_pc": false,
    "stage_pa": false,
    "sort_by": "UPDATE_DATE",
    "sort_direction": false,
    "records_per_page": 50,
    "currency_code": "RUB",
    "okpd2_codes": "",
    "okpd2_with_nested": true,
    "okpd2_several": false
  }'::jsonb,
  false
FROM users u
WHERE u.login = 'demo';
