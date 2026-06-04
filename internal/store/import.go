package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tn00869679/QuizForge_Backend/internal/model"
)

// Import performs transactional batch import of a category and its questions.
type Import struct {
	pool *pgxpool.Pool
}

// NewImport constructs an Import store backed by the given pool.
func NewImport(pool *pgxpool.Pool) *Import {
	return &Import{pool: pool}
}

// Apply imports an entire payload in a single transaction: it get-or-creates the
// category (by code), and for every question get-or-creates its exam_session
// (by category+year+term) and subject (by category+name), then upserts the
// question on its (exam_session_id, subject_id, number) unique key. On any error
// the whole batch is rolled back. It returns per-entity counts.
//
// get-or-create caches resolved subject/session ids per call to avoid redundant
// round-trips and to count distinct entities touched.
func (s *Import) Apply(ctx context.Context, payload model.ImportPayload) (model.ImportResult, error) {
	var res model.ImportResult

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return res, fmt.Errorf("store.Import.Apply: begin: %w", err)
	}
	// Rollback is a no-op after a successful Commit.
	defer func() { _ = tx.Rollback(ctx) }()

	categoryID, err := upsertCategory(ctx, tx, payload)
	if err != nil {
		return res, err
	}
	res.Categories = 1

	// Cache resolved ids within this import so distinct entities are counted once
	// and we avoid repeat upserts for the same session/subject.
	sessionIDs := make(map[[2]int]int64) // {year, term} -> session id
	subjectIDs := make(map[string]int64) // subject name -> subject id

	for i := range payload.Questions {
		iq := payload.Questions[i]

		sessKey := [2]int{iq.Year, iq.Term}
		sessionID, ok := sessionIDs[sessKey]
		if !ok {
			sessionID, err = upsertSession(ctx, tx, categoryID, iq)
			if err != nil {
				return res, err
			}
			sessionIDs[sessKey] = sessionID
			res.Sessions++
		}

		subjectID, ok := subjectIDs[iq.Subject]
		if !ok {
			subjectID, err = upsertSubject(ctx, tx, categoryID, iq)
			if err != nil {
				return res, err
			}
			subjectIDs[iq.Subject] = subjectID
			res.Subjects++
		}

		inserted, err := upsertQuestion(ctx, tx, sessionID, subjectID, iq)
		if err != nil {
			return res, err
		}
		if inserted {
			res.QuestionsInserted++
		} else {
			res.QuestionsUpdated++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return res, fmt.Errorf("store.Import.Apply: commit: %w", err)
	}
	return res, nil
}

// upsertCategory get-or-creates a category by its unique code and returns its id.
// On conflict it refreshes name/description so re-imports can correct metadata.
// The no-op-on-conflict still uses DO UPDATE so RETURNING always yields the id.
func upsertCategory(ctx context.Context, tx pgx.Tx, p model.ImportPayload) (int64, error) {
	const q = `
		INSERT INTO categories (code, name, description)
		VALUES ($1, $2, $3)
		ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description
		RETURNING id`
	var id int64
	if err := tx.QueryRow(ctx, q, p.CategoryCode, p.CategoryName, p.CategoryDescription).Scan(&id); err != nil {
		return 0, fmt.Errorf("store.Import: upsert category %q: %w", p.CategoryCode, err)
	}
	return id, nil
}

// upsertSession get-or-creates an exam_session by (category_id, year, term).
func upsertSession(ctx context.Context, tx pgx.Tx, categoryID int64, iq model.ImportQuestion) (int64, error) {
	const q = `
		INSERT INTO exam_sessions (category_id, year, term, source_url, label)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (category_id, year, term)
		DO UPDATE SET source_url = EXCLUDED.source_url, label = EXCLUDED.label
		RETURNING id`
	var id int64
	if err := tx.QueryRow(ctx, q, categoryID, iq.Year, iq.Term, iq.SourceURL, iq.SessionLabel).Scan(&id); err != nil {
		return 0, fmt.Errorf("store.Import: upsert session %d/%d: %w", iq.Year, iq.Term, err)
	}
	return id, nil
}

// upsertSubject get-or-creates a subject by (category_id, name). subjects has no
// unique constraint on (category_id, name), so this resolves by SELECT first and
// INSERTs only when absent (all inside the import transaction).
func upsertSubject(ctx context.Context, tx pgx.Tx, categoryID int64, iq model.ImportQuestion) (int64, error) {
	const sel = `SELECT id FROM subjects WHERE category_id = $1 AND name = $2 LIMIT 1`
	var id int64
	err := tx.QueryRow(ctx, sel, categoryID, iq.Subject).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("store.Import: select subject %q: %w", iq.Subject, err)
	}
	const ins = `
		INSERT INTO subjects (category_id, name, order_index)
		VALUES ($1, $2, $3)
		RETURNING id`
	if err := tx.QueryRow(ctx, ins, categoryID, iq.Subject, iq.SubjectOrder).Scan(&id); err != nil {
		return 0, fmt.Errorf("store.Import: insert subject %q: %w", iq.Subject, err)
	}
	return id, nil
}

// upsertQuestion upserts a question on its (exam_session_id, subject_id, number)
// unique key. It returns inserted=true when a new row was created, false on
// update. The xmax = 0 check distinguishes the two: xmax is 0 for a freshly
// inserted tuple and non-zero for one updated by ON CONFLICT.
func upsertQuestion(ctx context.Context, tx pgx.Tx, sessionID, subjectID int64, iq model.ImportQuestion) (bool, error) {
	const q = `
		INSERT INTO questions
			(subject_id, exam_session_id, number, stem, options, answer, explanation, tags, difficulty, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
		ON CONFLICT (exam_session_id, subject_id, number) DO UPDATE SET
			stem        = EXCLUDED.stem,
			options     = EXCLUDED.options,
			answer      = EXCLUDED.answer,
			explanation = EXCLUDED.explanation,
			tags        = EXCLUDED.tags,
			difficulty  = EXCLUDED.difficulty,
			is_active   = true,
			updated_at  = now()
		RETURNING (xmax = 0) AS inserted`
	tags := iq.Tags
	if tags == nil {
		tags = []string{}
	}
	var inserted bool
	if err := tx.QueryRow(ctx, q,
		subjectID, sessionID, iq.Number, iq.Stem, iq.Options, iq.Answer, iq.Explanation, tags, iq.Difficulty,
	).Scan(&inserted); err != nil {
		return false, fmt.Errorf("store.Import: upsert question s%d/sub%d/n%d: %w", sessionID, subjectID, iq.Number, err)
	}
	return inserted, nil
}
