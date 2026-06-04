CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE categories (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code        TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE subjects (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    category_id BIGINT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    order_index INT  NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_subjects_category ON subjects(category_id);

CREATE TABLE exam_sessions (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    category_id BIGINT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    year        INT  NOT NULL,
    term        INT  NOT NULL,
    source_url  TEXT NOT NULL DEFAULT '',
    label       TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(category_id, year, term)
);
CREATE INDEX idx_sessions_category ON exam_sessions(category_id);

CREATE TABLE questions (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    subject_id      BIGINT NOT NULL REFERENCES subjects(id),
    exam_session_id BIGINT NOT NULL REFERENCES exam_sessions(id),
    number          INT  NOT NULL,
    stem            TEXT NOT NULL,
    options         JSONB NOT NULL,
    answer          TEXT NOT NULL,
    explanation     TEXT NOT NULL DEFAULT '',
    tags            TEXT[] NOT NULL DEFAULT '{}',
    difficulty      INT  NOT NULL DEFAULT 0,
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(exam_session_id, subject_id, number)
);
CREATE INDEX idx_questions_subject ON questions(subject_id);
CREATE INDEX idx_questions_session ON questions(exam_session_id);
CREATE INDEX idx_questions_active  ON questions(is_active);
CREATE INDEX idx_questions_stem_trgm ON questions USING GIN (stem gin_trgm_ops);

CREATE TABLE users (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email        TEXT UNIQUE,
    display_name TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE attempts (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id             BIGINT REFERENCES users(id),
    question_id         BIGINT NOT NULL REFERENCES questions(id),
    selected            TEXT,
    is_correct          BOOLEAN NOT NULL DEFAULT false,
    is_marked_uncertain BOOLEAN NOT NULL DEFAULT false,
    is_favorite         BOOLEAN NOT NULL DEFAULT false,
    mode                TEXT NOT NULL DEFAULT 'practice',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_attempts_user_question ON attempts(user_id, question_id);
CREATE INDEX idx_attempts_user ON attempts(user_id);
CREATE INDEX idx_attempts_question ON attempts(question_id);
