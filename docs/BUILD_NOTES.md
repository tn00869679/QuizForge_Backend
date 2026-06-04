# QuizForge Backend — Build Notes（技術決策，by Tech Lead）

> 權威來源 = 本檔 + `docs/SPEC.md`。**所有 sub-agent 動手前必讀本檔**。本檔為設計決策，SPEC.md 為需求。衝突時以本檔為準並回報。

## Tech stack（已定，勿換）
- Go 1.22 · Gin · `github.com/jackc/pgx/v5`（pgxpool）· `golang-migrate/migrate/v4`（用 lib + `embed.FS`，server 啟動時自動套用）· `log/slog` · `golang.org/x/time/rate`（限流）
- PostgreSQL 16（docker-compose 提供）
- standard library 優先；非必要不引第三方。
- **DB 存取一律用 pgx 直接寫 SQL（不用 ORM、不用 sqlc）**，集中於 `internal/store`。動態 `WHERE` 手組 SQL + 參數陣列（`$1,$2...`），**絕不把值字串拼接進 SQL**。單一 SQL 風格（Rule 7）。

## 專案 layout
```
cmd/server/main.go        # API server 進入點
cmd/importer/main.go      # PDF 匯入 CLI skeleton
internal/config/          # env 設定載入
internal/db/              # pgxpool 建立 + migrations 套用（embed.FS）
internal/store/           # repository 層（DB 存取）
internal/service/         # 業務邏輯（exam grade、question query 組裝）
internal/handler/         # gin handlers + response envelope helper + router
internal/middleware/      # cors / recovery / requestid / ratelimit / auth-stub / admin
internal/model/           # domain types + request/response DTO
internal/importer/        # pdftotext 抽取 → 切題 → JSON（skeleton）
db/migrations/            # NNNN_name.up.sql / .down.sql
seed/                     # 種子題目 JSON（手造）
docs/
```

## 通用慣例
- 回應 envelope：成功 `{"data":<x>,"error":null}`，失敗 `{"data":null,"error":{"code","message"}}`。helper 放 `internal/handler/response.go`：`Ok(c, data)` / `Fail(c, status, code, msg)`。
- error code 列舉：`VALIDATION` `NOT_FOUND` `UNAUTHORIZED` `FORBIDDEN` `RATE_LIMITED` `INTERNAL`。
- 分頁 `page`(預設1) / `page_size`(預設20、上限100)；回應帶 `{items, page, page_size, total}` 包在 data 內。
- `context.Context` 傳到所有 DB/IO 邊界。error 用 `errors.Is/As` + `fmt.Errorf("...: %w", err)`，**絕不吞 error**。
- slog structured log，含 request_id。

## DDL — 權威 schema（migrations 0001 照抄，含索引）
```sql
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
    year        INT  NOT NULL,                 -- 民國年
    term        INT  NOT NULL,                 -- 梯次
    source_url  TEXT NOT NULL DEFAULT '',
    label       TEXT NOT NULL,                 -- 例 "114年第3次"
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
    options         JSONB NOT NULL,            -- [{"key":"A","text":"..."}]
    answer          TEXT NOT NULL,             -- 'A'|'B'|'C'|'D'
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
    mode                TEXT NOT NULL DEFAULT 'practice',  -- practice|exam
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_attempts_user_question ON attempts(user_id, question_id);
CREATE INDEX idx_attempts_user ON attempts(user_id);
CREATE INDEX idx_attempts_question ON attempts(question_id);
```

## 關鍵設計決策（必讀，影響多個檔案）
1. **中文關鍵字搜尋**：`to_tsvector` 對中文無內建好的 parser，故 keyword 用 `stem ILIKE '%kw%'`，配 `pg_trgm` GIN index 加速。（SPEC 寫 GIN full-text，這裡用 trgm GIN 達成同目的。）
2. **attempts = per-(user_id, question_id) upsert 狀態模型**（非 append log）。寫入用 `INSERT ... ON CONFLICT (user_id,question_id) DO UPDATE`。PATCH by question_id 即更新該狀態列。`GET /stats` 與 `?status=` 都基於此。
3. **exam_token = stateless HMAC**，不另開 table：
   `token = base64url(payloadJSON) + "." + hex(HMAC_SHA256(EXAM_HMAC_SECRET, payloadJSON))`
   payload = `{"question_ids":[...], "issued_at":<unix>, "duration_sec":<int>}`。
   `exam/grade` 收 `{exam_token, answers:[{question_id,selected}]}`，驗簽 + 檢查未過期 → 從 DB 撈這些題的 answer/explanation/subject → 算逐題對錯、總分、各科分數。
4. **模擬考不洩漏答案**：`questions/:id?include_answer=false` 與 `exam/start` 回傳的題目都**移除 answer/explanation 欄位**。
5. **auth stub**：middleware 讀 `X-User-Id`（int，optional）放進 context。`5.3` 端點要求必須有 user（無 → 401 UNAUTHORIZED）。admin 端點要求 header `X-Admin-Token == env ADMIN_TOKEN`（不符 → 403）。
6. **rate limit**：per-IP token bucket（`x/time/rate`），一般 10 rps / burst 20；`/admin/*` 另用較嚴格 bucket。記憶體 map + mutex 即可（單機）。
7. **random**：`ORDER BY random()`（資料量小可接受）；limit=`all` 表不限。

## Config（env，`internal/config`）
`DATABASE_URL`（預設見 docker-compose）· `PORT`(8080) · `ADMIN_TOKEN`(dev: "dev-admin-token") · `EXAM_HMAC_SECRET`(dev: "dev-secret") · `CORS_ORIGINS`("http://localhost:5173")

## 明確 stub / 不做（Rule 12 誠實揭露）
- **PDF 真實解析不做**：`cmd/importer` 只是 skeleton — 用 `pdftotext`（外部指令）抽文字 → 正則切題號/選項 → 輸出待校對 JSON。附 `seed/` 手造題目讓全站可跑。
- **OAuth 不做**：users table 存在，但只走 auth stub。
- admin import 走真實 DB 寫入（upsert by unique index），這是真的、不是 stub。
