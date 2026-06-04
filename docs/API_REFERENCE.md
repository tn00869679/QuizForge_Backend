# API Reference — QuizForge Backend

## Base path

All `/api/v1/…` routes are mounted under `/api/v1`. The health endpoint lives at the root.

```
Base URL: http://localhost:8080
API prefix: /api/v1
```

---

## Response envelope

Every response — success or error — is wrapped in the same JSON envelope:

**Success**

```json
{
  "data": <payload>,
  "error": null
}
```

**Error**

```json
{
  "data": null,
  "error": {
    "code": "VALIDATION",
    "message": "human-readable description"
  }
}
```

---

## Error codes

| Code | HTTP status | When |
|------|-------------|------|
| `VALIDATION` | 400 | Malformed input, missing required field, out-of-range value |
| `NOT_FOUND` | 404 | Resource does not exist |
| `UNAUTHORIZED` | 401 | Missing or invalid auth credential |
| `FORBIDDEN` | 403 | Credential present but not permitted (e.g. wrong admin token) |
| `RATE_LIMITED` | 429 | Rate limit exceeded |
| `INTERNAL` | 500 | Unexpected server/DB error |

---

## Pagination

Endpoints that return lists use a shared page object inside `data`:

```json
{
  "items": [...],
  "page": 1,
  "page_size": 20,
  "total": 153
}
```

| Parameter | Type | Default | Max | Notes |
|-----------|------|---------|-----|-------|
| `page` | int | `1` | — | Must be >= 1 |
| `page_size` | int | `20` | `100` | Must be between 1 and 100 |

---

## Authentication

QuizForge uses two separate header-based credentials:

### X-User-Id (user stub)

```
X-User-Id: 42
```

- A numeric user identifier. Accepted by all routes as an **optional** header.
- The server does not validate the value against a users table — it is a development stub for a future OAuth flow.
- Routes marked **RequireUser** below return `401 UNAUTHORIZED` when this header is absent.
- Routes marked **Public** accept it optionally (e.g. to enable `status` filtering on questions).

### X-Admin-Token

```
X-Admin-Token: dev-admin-token
```

- A static secret compared to the `ADMIN_TOKEN` env var (dev default: `dev-admin-token`).
- Required by all `/admin/…` routes. Wrong or missing value → `403 FORBIDDEN`.
- Admin routes also have a stricter rate limit (1 req/s, burst 5).

---

## 1. System

### `GET /health`

Health check. No auth required. Not under `/api/v1`.

**Response 200**

```json
{
  "data": { "status": "ok" },
  "error": null
}
```

---

## 2. Catalogue

### `GET /api/v1/categories`

List all exam categories. Public.

**Response 200**

```json
{
  "data": [
    {
      "id": 1,
      "code": "02",
      "name": "證券商業務員",
      "description": "",
      "created_at": "2024-01-01T00:00:00Z"
    }
  ],
  "error": null
}
```

**Errors**

| Code | When |
|------|------|
| `INTERNAL` | DB error |

---

### `GET /api/v1/categories/:id/subjects`

List subjects belonging to a category. Public.

**Path parameters**

| Name | Type | Notes |
|------|------|-------|
| `id` | int64 | Positive integer; the category id |

**Response 200**

```json
{
  "data": [
    {
      "id": 3,
      "category_id": 1,
      "name": "證券交易相關法規與實務",
      "order_index": 1,
      "created_at": "2024-01-01T00:00:00Z"
    }
  ],
  "error": null
}
```

**Errors**

| Code | When |
|------|------|
| `VALIDATION` | `:id` is not a positive integer |
| `INTERNAL` | DB error |

---

### `GET /api/v1/exam-sessions`

List exam sessions for a category. Public.

**Query parameters**

| Name | Type | Required | Notes |
|------|------|----------|-------|
| `category_id` | int64 | yes | Positive integer |

**Response 200**

```json
{
  "data": [
    {
      "id": 7,
      "category_id": 1,
      "year": 114,
      "term": 3,
      "source_url": "",
      "label": "114年第3次",
      "created_at": "2024-01-01T00:00:00Z"
    }
  ],
  "error": null
}
```

**Errors**

| Code | When |
|------|------|
| `VALIDATION` | `category_id` absent or not a positive integer |
| `INTERNAL` | DB error |

---

### `GET /api/v1/questions`

Search / list questions. Public (status filter requires `X-User-Id`).

**Query parameters**

| Name | Type | Default | Notes |
|------|------|---------|-------|
| `category_id` | int64 | — | Filter by category |
| `subject_id` | int64 (multi) | — | Filter by subject. Accepts repeated keys (`subject_id=1&subject_id=2`) **or** comma-separated (`subject_id=1,2`) |
| `session_ids` | int64 (multi) | — | Filter by exam session. Same multi-value syntax as `subject_id` |
| `keyword` | string | — | Case-insensitive substring match on question stem (ILIKE with pg_trgm index) |
| `status` | string | — | One of `unanswered`, `wrong`, `favorite`. **Requires `X-User-Id` header** |
| `random` | bool | `false` | `true` randomises result order |
| `limit` | string | — | Whitelist: `5`, `10`, `20`, `50`, `all`. When absent or `all`, falls back to `page`/`page_size` pagination |
| `page` | int | `1` | Pagination page (used when `limit` is absent or `all`) |
| `page_size` | int | `20` | Pagination page size, max 100 |
| `include_answer` | — | — | Not applicable to this endpoint (always returns answers) |

**`include_answer` note:** `GET /questions` always includes `answer` and `explanation`. Use `GET /questions/:id?include_answer=false` for exam-mode single-question fetch.

**Response 200** — paginated (`Page[QuestionPublic]`)

```json
{
  "data": {
    "items": [
      {
        "id": 101,
        "subject_id": 3,
        "exam_session_id": 7,
        "number": 1,
        "stem": "下列何者…？",
        "options": [
          { "key": "A", "text": "選項甲" },
          { "key": "B", "text": "選項乙" },
          { "key": "C", "text": "選項丙" },
          { "key": "D", "text": "選項丁" }
        ],
        "answer": "A",
        "explanation": "因為…",
        "tags": ["法規"],
        "difficulty": 1
      }
    ],
    "page": 1,
    "page_size": 20,
    "total": 153
  },
  "error": null
}
```

**Errors**

| Code | When |
|------|------|
| `VALIDATION` | `status` value not in whitelist |
| `VALIDATION` | `status` present but `X-User-Id` absent |
| `VALIDATION` | `limit` not in whitelist |
| `VALIDATION` | `category_id` / `subject_id` / `session_ids` not parseable as integers |
| `VALIDATION` | `page` / `page_size` out of range |
| `INTERNAL` | DB error |

---

### `GET /api/v1/questions/:id`

Fetch a single question by id. Public.

**Path parameters**

| Name | Type | Notes |
|------|------|-------|
| `id` | int64 | Positive integer |

**Query parameters**

| Name | Type | Default | Notes |
|------|------|---------|-------|
| `include_answer` | bool | `true` | When `false`, `answer` and `explanation` are **omitted** from the response (exam mode). Both fields use `omitempty` so they disappear entirely from the JSON when suppressed. |

**Response 200** — `include_answer=true` (default)

```json
{
  "data": {
    "id": 101,
    "subject_id": 3,
    "exam_session_id": 7,
    "number": 1,
    "stem": "下列何者…？",
    "options": [
      { "key": "A", "text": "選項甲" },
      { "key": "B", "text": "選項乙" },
      { "key": "C", "text": "選項丙" },
      { "key": "D", "text": "選項丁" }
    ],
    "answer": "A",
    "explanation": "因為…",
    "tags": ["法規"],
    "difficulty": 1
  },
  "error": null
}
```

**Response 200** — `include_answer=false`

Same shape but `answer` and `explanation` keys are absent from the JSON object entirely.

**Errors**

| Code | When |
|------|------|
| `VALIDATION` | `:id` not a positive integer |
| `VALIDATION` | `include_answer` not parseable as boolean |
| `NOT_FOUND` | No question with that id |
| `INTERNAL` | DB error |

---

## 3. Practice / Exam

### `POST /api/v1/practice/generate`

Generate a practice set: run the same filter logic as `GET /questions` and return the ordered list of matching question IDs. The client is expected to fetch/step through each question individually. Public (status filter requires `X-User-Id`).

**Request body** (all fields optional)

```json
{
  "category_id": 1,
  "subject_id": [3, 4],
  "session_ids": [7],
  "keyword": "",
  "status": "unanswered",
  "limit": 20,
  "random": true,
  "page": 1,
  "page_size": 20
}
```

| Field | Type | Notes |
|-------|------|-------|
| `category_id` | int64 \| null | Positive integer |
| `subject_id` | int64[] | Array of subject ids |
| `session_ids` | int64[] | Array of exam session ids |
| `keyword` | string | Keyword filter |
| `status` | string | One of `unanswered`, `wrong`, `favorite`. Requires `X-User-Id` |
| `limit` | int \| null | One of `5`, `10`, `20`, `50`. When absent, falls back to `page`/`page_size` |
| `random` | bool | Randomise order |
| `page` | int \| null | Page number (default 1) |
| `page_size` | int \| null | Page size (default 20, max 100) |

Note: `limit` in the body only accepts the numeric values `5`, `10`, `20`, `50`. The string `"all"` is not valid in the body (omit `limit` entirely to get paginated results instead).

**Response 200**

```json
{
  "data": {
    "question_ids": [101, 45, 78, 33]
  },
  "error": null
}
```

**Errors**

| Code | When |
|------|------|
| `VALIDATION` | Invalid JSON body |
| `VALIDATION` | `status` value not in whitelist |
| `VALIDATION` | `status` present but `X-User-Id` absent |
| `VALIDATION` | `limit` not in `{5, 10, 20, 50}` |
| `VALIDATION` | `category_id` <= 0 |
| `VALIDATION` | `page` < 1 or `page_size` out of range |
| `INTERNAL` | DB error |

---

### `POST /api/v1/exam/start`

Start a timed mock exam for a category. The server samples questions per-subject, signs them into a stateless HMAC token, and returns the question set **without answers or explanations**. Public.

**exam_token semantics:** The token encodes the exam metadata (question ids, correct answers, issue time, duration) as a signed payload. It is entirely stateless — no server-side session is stored. The client must return it unchanged to `POST /exam/grade`.

**Request body**

```json
{
  "category_id": 1,
  "per_subject_n": 10,
  "duration_sec": 3600
}
```

| Field | Type | Required | Default | Notes |
|-------|------|----------|---------|-------|
| `category_id` | int64 | yes | — | Positive integer |
| `per_subject_n` | int | no | `50` | Questions sampled per subject |
| `duration_sec` | int | no | `7200` | Exam duration in seconds |

**Response 200**

```json
{
  "data": {
    "exam_token": "eyJ...",
    "duration_sec": 3600,
    "total": 30,
    "per_subject": [
      { "subject_id": 3, "subject": "證券交易相關法規與實務", "count": 10 },
      { "subject_id": 4, "subject": "證券商業務員業務相關法規", "count": 10 },
      { "subject_id": 5, "subject": "其他財經相關法規", "count": 10 }
    ],
    "questions": [
      {
        "id": 101,
        "subject_id": 3,
        "exam_session_id": 7,
        "number": 1,
        "stem": "下列何者…？",
        "options": [
          { "key": "A", "text": "選項甲" },
          { "key": "B", "text": "選項乙" },
          { "key": "C", "text": "選項丙" },
          { "key": "D", "text": "選項丁" }
        ],
        "tags": ["法規"],
        "difficulty": 1
      }
    ]
  },
  "error": null
}
```

Note: `questions[].answer` and `questions[].explanation` are **absent** from the JSON (omitempty, exam mode).

**Errors**

| Code | When |
|------|------|
| `VALIDATION` | Invalid JSON body |
| `VALIDATION` | `category_id` <= 0 |
| `NOT_FOUND` | No questions available for the category |
| `INTERNAL` | DB error |

---

### `POST /api/v1/exam/grade`

Submit answers and receive graded results. The exam token is verified (HMAC signature check). An expired but authentic token is **still graded** — `expired: true` is reported but no error is returned. Public.

**Request body**

```json
{
  "exam_token": "eyJ...",
  "answers": [
    { "question_id": 101, "selected": "A" },
    { "question_id": 102, "selected": "C" },
    { "question_id": 103, "selected": "" }
  ]
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `exam_token` | string | yes | The token returned by `exam/start`; must not be empty |
| `answers` | array | yes | One entry per question. `selected` is one of `A`, `B`, `C`, `D`, or `""` (skipped) |

**Response 200**

```json
{
  "data": {
    "score": 80,
    "correct": 24,
    "total": 30,
    "expired": false,
    "per_subject": [
      {
        "subject_id": 3,
        "subject": "證券交易相關法規與實務",
        "correct": 8,
        "total": 10,
        "score": 80
      }
    ],
    "items": [
      {
        "question_id": 101,
        "number": 1,
        "selected": "A",
        "answer": "A",
        "is_correct": true,
        "explanation": "因為…",
        "subject": "證券交易相關法規與實務"
      }
    ]
  },
  "error": null
}
```

**Errors**

| Code | When |
|------|------|
| `VALIDATION` | Invalid JSON body |
| `VALIDATION` | `exam_token` is empty |
| `VALIDATION` | Any `answers[].selected` not in `{A, B, C, D, ""}` |
| `VALIDATION` | Malformed (unparseable) `exam_token` |
| `UNAUTHORIZED` | Valid-format token with bad HMAC signature |
| `INTERNAL` | DB error |

---

## 4. Attempt records (RequireUser)

All endpoints in this group require `X-User-Id`. Absent header → `401 UNAUTHORIZED`.

---

### `POST /api/v1/attempts`

Record the user's answer for a question. Grades the answer server-side and upserts the attempt row (does not overwrite `is_favorite` or `is_marked_uncertain`). **RequireUser.**

**Request body**

```json
{
  "question_id": 101,
  "selected": "A",
  "mode": "practice"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `question_id` | int64 | yes | Positive integer |
| `selected` | string | yes | One of `A`, `B`, `C`, `D` |
| `mode` | string | no | One of `practice`, `exam`. Defaults to `practice` when absent |

**Response 200**

```json
{
  "data": {
    "id": 500,
    "user_id": 42,
    "question_id": 101,
    "selected": "A",
    "is_correct": true,
    "is_marked_uncertain": false,
    "is_favorite": false,
    "mode": "practice",
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  },
  "error": null
}
```

**Errors**

| Code | When |
|------|------|
| `UNAUTHORIZED` | `X-User-Id` absent |
| `VALIDATION` | Invalid JSON body |
| `VALIDATION` | `question_id` <= 0 |
| `VALIDATION` | `selected` not in `{A, B, C, D}` |
| `VALIDATION` | `mode` not in `{practice, exam}` |
| `NOT_FOUND` | No question with that id |
| `INTERNAL` | DB error |

---

### `PATCH /api/v1/attempts/:question_id`

Partial update of the `is_favorite` and/or `is_marked_uncertain` flags. Creates the attempt row if none exists yet (no answer required). At least one flag must be provided. **RequireUser.**

**Path parameters**

| Name | Type | Notes |
|------|------|-------|
| `question_id` | int64 | Positive integer |

**Request body**

```json
{
  "is_favorite": true,
  "is_marked_uncertain": false
}
```

Both fields are nullable/omittable. Any field absent from the body is left unchanged.

| Field | Type | Notes |
|-------|------|-------|
| `is_favorite` | bool \| null | Set or unset favourite flag |
| `is_marked_uncertain` | bool \| null | Set or unset uncertain flag |

At least one of `is_favorite` or `is_marked_uncertain` must be present.

**Response 200** — same `Attempt` shape as `POST /attempts`

```json
{
  "data": {
    "id": 500,
    "user_id": 42,
    "question_id": 101,
    "selected": null,
    "is_correct": false,
    "is_marked_uncertain": false,
    "is_favorite": true,
    "mode": "",
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:32:00Z"
  },
  "error": null
}
```

**Errors**

| Code | When |
|------|------|
| `UNAUTHORIZED` | `X-User-Id` absent |
| `VALIDATION` | `:question_id` not a positive integer |
| `VALIDATION` | Invalid JSON body |
| `VALIDATION` | Both `is_favorite` and `is_marked_uncertain` absent |
| `INTERNAL` | DB error |

---

### `GET /api/v1/attempts`

Paginated review list of questions the user has marked as wrong, favourite, or uncertain. Returns the full question (with answer and explanation — review mode) paired with the user's attempt state. **RequireUser.**

**Query parameters**

| Name | Type | Required | Notes |
|------|------|----------|-------|
| `status` | string | yes | One of `wrong`, `favorite`, `uncertain` |
| `page` | int | no | Default 1 |
| `page_size` | int | no | Default 20, max 100 |

**Response 200** — `Page[ReviewItem]`

```json
{
  "data": {
    "items": [
      {
        "question": {
          "id": 101,
          "subject_id": 3,
          "exam_session_id": 7,
          "number": 1,
          "stem": "下列何者…？",
          "options": [
            { "key": "A", "text": "選項甲" },
            { "key": "B", "text": "選項乙" },
            { "key": "C", "text": "選項丙" },
            { "key": "D", "text": "選項丁" }
          ],
          "answer": "A",
          "explanation": "因為…",
          "tags": ["法規"],
          "difficulty": 1
        },
        "attempt": {
          "selected": "B",
          "is_correct": false,
          "is_marked_uncertain": false,
          "is_favorite": false,
          "mode": "practice"
        }
      }
    ],
    "page": 1,
    "page_size": 20,
    "total": 7
  },
  "error": null
}
```

**Errors**

| Code | When |
|------|------|
| `UNAUTHORIZED` | `X-User-Id` absent |
| `VALIDATION` | `status` absent or not in `{wrong, favorite, uncertain}` |
| `VALIDATION` | `page` / `page_size` out of range |
| `INTERNAL` | DB error |

---

### `GET /api/v1/stats`

Aggregate answer statistics for the authenticated user. **RequireUser.**

**Response 200**

```json
{
  "data": {
    "answered": 120,
    "correct": 95,
    "accuracy": 0.7917,
    "wrong": 25,
    "favorites": 12,
    "uncertain": 8
  },
  "error": null
}
```

`accuracy` is `correct / answered`; returns `0` when `answered` is `0`.

**Errors**

| Code | When |
|------|------|
| `UNAUTHORIZED` | `X-User-Id` absent |
| `INTERNAL` | DB error |

---

## 5. Admin (RequireAdmin)

All endpoints require `X-Admin-Token`. Wrong or missing token → `403 FORBIDDEN`.  
Admin routes have a stricter rate limit: 1 req/s, burst 5 → `429 RATE_LIMITED`.

---

### `POST /api/v1/admin/import`

Batch-import questions for one category. Idempotent (upsert semantics). Entire batch runs in a single transaction — any error rolls everything back.

For the full body schema, field rules, and upsert semantics see [docs/IMPORT_FORMAT.md](IMPORT_FORMAT.md).

**Headers**

```
Content-Type: application/json
X-Admin-Token: dev-admin-token
```

**Request body** (abbreviated — see IMPORT_FORMAT.md for complete spec)

```json
{
  "category_code": "02",
  "category_name": "證券商業務員",
  "category_description": "",
  "questions": [
    {
      "year": 114, "term": 3,
      "session_label": "114年第3次", "source_url": "",
      "subject": "證券交易相關法規與實務", "subject_order": 1,
      "number": 1,
      "stem": "下列何者…？",
      "options": [
        { "key": "A", "text": "…" },
        { "key": "B", "text": "…" },
        { "key": "C", "text": "…" },
        { "key": "D", "text": "…" }
      ],
      "answer": "A",
      "explanation": "…",
      "tags": ["法規"],
      "difficulty": 1
    }
  ]
}
```

**Response 200**

```json
{
  "data": {
    "categories": 1,
    "subjects": 2,
    "sessions": 1,
    "questions_inserted": 3,
    "questions_updated": 0
  },
  "error": null
}
```

**Errors**

| Code | When |
|------|------|
| `VALIDATION` | Malformed JSON or field validation failure (see IMPORT_FORMAT.md) |
| `FORBIDDEN` | Missing or wrong `X-Admin-Token` |
| `RATE_LIMITED` | Admin rate limit exceeded |
| `INTERNAL` | Unexpected DB error (batch rolled back) |
