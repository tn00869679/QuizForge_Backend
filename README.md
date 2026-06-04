# QuizForge Backend

Go 微服務後端，提供證照題庫查詢、模擬考核心 API，以及管理員題目匯入功能。

## 技術棧

| 元件 | 版本 / 說明 |
|------|-------------|
| Go | 1.22 |
| Gin | HTTP 框架 |
| PostgreSQL | 16，透過 docker-compose 提供 |
| pgx/v5 | pgxpool 直接寫 SQL（無 ORM） |
| golang-migrate | 嵌入式 migration，server 啟動時自動套用 |
| log/slog | Structured logging |
| golang.org/x/time/rate | per-IP rate limiting |

## 目錄結構

```
cmd/
  server/main.go         # API server 進入點
  importer/main.go       # seed loader + PDF 匯入 CLI skeleton
internal/
  config/                # env 設定載入
  db/                    # pgxpool 建立、migration 套用
  store/                 # repository 層（DB 存取）
  service/               # 業務邏輯
  handler/               # Gin handlers、response envelope、router
  middleware/            # CORS / recovery / requestid / ratelimit / auth-stub / admin
  model/                 # domain types + DTO
  importer/              # pdftotext 抽取 → 切題 → JSON（skeleton）
db/migrations/           # NNNN_name.up.sql / .down.sql
seed/
  seed.json              # 種子題庫（IMPORT_FORMAT，手造原創題）
docs/
  IMPORT_FORMAT.md       # 匯入 JSON 格式規格（權威）
  BUILD_NOTES.md         # 技術決策記錄
.env.example             # 環境變數範本
docker-compose.yml       # PostgreSQL 16，host port 5433
```

## 如何跑

### 1. 啟動資料庫

```bash
docker compose up -d
```

PostgreSQL 監聽 `localhost:5433`（container 內 5432）。

### 2. 設定環境變數

```bash
cp .env.example .env
# 依需求修改 .env，dev 預設值已可直接使用
export $(grep -v '^#' .env | xargs)
```

### 3. 啟動 API server

server 啟動時自動執行 migrations。

```bash
go run ./cmd/server
```

預設監聽 `localhost:8080`。

### 4. 匯入種子題庫

```bash
go run ./cmd/importer seed seed/seed.json
```

預期輸出：

```
categories:1  subjects:3  sessions:2  questions_inserted:24  questions_updated:0
```

重複執行為 idempotent（upsert），會印出 `questions_updated:24`。

### 5. PDF 匯入（skeleton）

```bash
go run ./cmd/importer parse \
  --questions q.pdf \
  --answers a.pdf \
  --out out.json \
  --subject "證券交易相關法規與實務" \
  --year 114 --term 3 \
  --session-label "114年第3次"
```

需要系統已安裝 `pdftotext`（poppler-utils）：

```bash
brew install poppler        # macOS
apt-get install poppler-utils  # Debian/Ubuntu
```

輸出 JSON 每題標記 `"needs_review": true`，**必須人工校對後**再以 `importer seed` 正式匯入。詳見 [docs/IMPORT_FORMAT.md](docs/IMPORT_FORMAT.md)。

## API 端點摘要

所有端點回應格式：`{"data": <payload>, "error": null}` / `{"data": null, "error": {"code": "...", "message": "..."}}`。

分頁參數：`page`（預設 1）、`page_size`（預設 20，上限 100）。

### 已實作（M1）

| 方法 | 路徑 | 說明 |
|------|------|------|
| `GET` | `/health` | Health check |
| `GET` | `/api/v1/categories` | 列出所有科目大類 |
| `GET` | `/api/v1/categories/:id/subjects` | 列出指定大類下的科目 |
| `GET` | `/api/v1/exam-sessions?category_id=` | 列出指定大類的考次 |
| `GET` | `/api/v1/questions` | 查詢題目（支援 keyword / category_id / subject_id / session_ids / status / random / limit） |
| `GET` | `/api/v1/questions/:id` | 取單題（`?include_answer=false` 可隱藏答案） |
| `POST` | `/api/v1/admin/import` | 批次匯入題目（需 `X-Admin-Token`） |

查詢參數說明（`GET /api/v1/questions`）：

| 參數 | 說明 |
|------|------|
| `category_id` | 篩選大類 |
| `subject_id` | 篩選科目（可重複或逗號分隔） |
| `session_ids` | 篩選考次（可重複或逗號分隔） |
| `keyword` | 關鍵字搜尋（題幹 ILIKE，配 pg_trgm GIN index） |
| `status` | `unanswered` / `wrong` / `favorite`（需 `X-User-Id` header） |
| `random` | `true` 隨機排序 |
| `limit` | `5` / `10` / `20` / `50` / `all`；`all` 時走分頁 |

### 後續計畫（尚未實作）

| 方法 | 路徑 | 說明 |
|------|------|------|
| `POST` | `/api/v1/exam/start` | 開始模擬考（回傳 exam_token，題目不含答案） |
| `POST` | `/api/v1/exam/grade` | 交卷評分（驗 HMAC token） |
| `PATCH` | `/api/v1/attempts/:question_id` | 更新作答狀態（favorite / marked_uncertain） |
| `GET` | `/api/v1/stats` | 使用者作答統計 |

## 環境變數

| 變數 | 預設（dev） | 說明 |
|------|-------------|------|
| `DATABASE_URL` | `postgres://quizforge:quizforge@localhost:5433/quizforge?sslmode=disable` | pgx DSN |
| `PORT` | `8080` | HTTP 監聽 port |
| `ADMIN_TOKEN` | `dev-admin-token` | `X-Admin-Token` header 驗證值 |
| `EXAM_HMAC_SECRET` | `dev-secret` | 模擬考 token HMAC 金鑰 |
| `CORS_ORIGINS` | `http://localhost:5173` | 允許的 CORS origin（逗號分隔） |

## PDF 匯入流程

```
q.pdf + a.pdf
     │
     ▼ pdftotext (external)
  raw text
     │
     ▼ splitQuestions + detectOptions + parseAnswers
  []ParsedQuestion  (needs_review: true)
     │
     ▼ out.json  ← 人工校對：補 explanation、確認 answer、調整 tags/difficulty
     │
     ▼ importer seed out.json
  DB (questions upserted)
```

詳細 JSON 欄位規格見 [docs/IMPORT_FORMAT.md](docs/IMPORT_FORMAT.md)。

## 明確 Stub（未實作）

- **PDF 真實解析**：`internal/importer` 為骨架，正則切題需依實際 PDF 排版逐案調整，輸出需人工校對。
- **OAuth / 使用者認證**：`users` table 已建，但 auth 只走 `X-User-Id` header stub，未實作 OAuth 流程。
