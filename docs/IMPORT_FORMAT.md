# Import Format — `POST /api/v1/admin/import`

Authoritative contract for the admin import payload. The seed files and the
future PDF importer both produce this shape. Server-side type:
`internal/model.ImportPayload` (see `internal/model/import.go`).

## Request

- Method: `POST /api/v1/admin/import`
- Headers:
  - `Content-Type: application/json`
  - `X-Admin-Token: <ADMIN_TOKEN>` (dev default `dev-admin-token`)
- Body: a single JSON object (below). One payload carries exactly **one
  category** and any number of its questions, spanning multiple sessions /
  subjects.

```json
{
  "category_code": "02",
  "category_name": "證券商業務員",
  "category_description": "",
  "questions": [
    {
      "year": 114, "term": 3, "session_label": "114年第3次", "source_url": "",
      "subject": "證券交易相關法規與實務", "subject_order": 1,
      "number": 1,
      "stem": "下列何者…？",
      "options": [
        {"key": "A", "text": "…"},
        {"key": "B", "text": "…"},
        {"key": "C", "text": "…"},
        {"key": "D", "text": "…"}
      ],
      "answer": "A",
      "explanation": "…",
      "tags": ["法規"],
      "difficulty": 1
    }
  ]
}
```

## Fields

### Top level

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `category_code` | string | yes | Official category code, e.g. `"02"`. Unique key for the category (get-or-create). |
| `category_name` | string | yes | Display name. Refreshed on re-import. |
| `category_description` | string | no | Defaults to `""`. Refreshed on re-import. |
| `questions` | array | yes, non-empty | Question rows (below). |

### `questions[]`

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `year` | int | yes (> 0) | Republic-of-China year (民國年). |
| `term` | int | yes (> 0) | Sitting number within the year. |
| `session_label` | string | no | Human label, e.g. `"114年第3次"`. Refreshed on re-import of the session. |
| `source_url` | string | no | Provenance URL. Refreshed on re-import of the session. |
| `subject` | string | yes | Subject name. Get-or-create by `(category, name)`. |
| `subject_order` | int | no | `order_index` used only when the subject is first created. |
| `number` | int | yes (> 0) | Question number within the session+subject. |
| `stem` | string | yes | Question text. |
| `options` | array | yes (>= 2) | Each `{ "key": string, "text": string }`; both non-empty. |
| `answer` | string | yes | One of `A` / `B` / `C` / `D`. |
| `explanation` | string | no | Defaults to `""`. |
| `tags` | string[] | no | Defaults to `[]`. |
| `difficulty` | int | no | Defaults to `0`. |

## Upsert semantics

Everything runs in **one transaction**; any error rolls the whole batch back.

- **Category** — get-or-create by `code`. On conflict, `name` and `description`
  are updated.
- **Exam session** — get-or-create by `(category_id, year, term)`. On conflict,
  `source_url` and `label` are updated.
- **Subject** — get-or-create by `(category_id, name)`. Created once with
  `subject_order`; not mutated afterwards.
- **Question** — upsert by the unique key `(exam_session_id, subject_id,
  number)`. On conflict, `stem`, `options`, `answer`, `explanation`, `tags`,
  `difficulty` are overwritten, `is_active` is reset to `true`, and `updated_at`
  is bumped. Re-importing the same payload therefore **updates in place** rather
  than inserting duplicates.

## Response

`200 OK` with per-entity counts inside the standard envelope:

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

- `categories` is always `1`.
- `subjects` / `sessions` count the **distinct** entities referenced by the
  payload (deduplicated within the batch).
- `questions_inserted` vs `questions_updated` distinguishes new rows from
  in-place updates.

## Errors

| Status | `error.code` | When |
|--------|--------------|------|
| 400 | `VALIDATION` | Malformed JSON, or a field fails the rules above (missing required field, `answer` not A–D, fewer than 2 options, empty option key/text, etc.). |
| 403 | `FORBIDDEN` | Missing or wrong `X-Admin-Token`. |
| 429 | `RATE_LIMITED` | Admin rate limit exceeded. |
| 500 | `INTERNAL` | Unexpected DB error (batch rolled back). |
