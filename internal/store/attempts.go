package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tn00869679/QuizForge_Backend/internal/model"
)

// Attempts provides access to the per-(user, question) attempts state table.
type Attempts struct {
	pool *pgxpool.Pool
}

// NewAttempts constructs an Attempts store backed by the given pool.
func NewAttempts(pool *pgxpool.Pool) *Attempts {
	return &Attempts{pool: pool}
}

// attemptColumns is the canonical column list / scan order for an attempts row.
const attemptColumns = `id, user_id, question_id, selected, is_correct,
	is_marked_uncertain, is_favorite, mode, created_at, updated_at`

// scanAttempt scans a single attempts row in attemptColumns order.
func scanAttempt(row pgx.Row) (model.Attempt, error) {
	var a model.Attempt
	err := row.Scan(
		&a.ID, &a.UserID, &a.QuestionID, &a.Selected, &a.IsCorrect,
		&a.IsMarkedUncertain, &a.IsFavorite, &a.Mode, &a.CreatedAt, &a.UpdatedAt,
	)
	return a, err
}

// EnsureUser makes sure a users row with the given id exists, creating a
// placeholder if absent. The auth layer is a stub (X-User-Id header) with no
// real signup, but attempts.user_id is an FK to users(id); without this an
// otherwise valid attempt write would fail with a foreign-key violation.
//
// OVERRIDING SYSTEM VALUE is required because users.id is GENERATED ALWAYS AS
// IDENTITY. This is acceptable here precisely because auth is a stub; real
// signup would let the identity assign ids.
func (s *Attempts) EnsureUser(ctx context.Context, userID int64) error {
	const q = `
		INSERT INTO users (id, display_name)
		OVERRIDING SYSTEM VALUE
		VALUES ($1, '')
		ON CONFLICT (id) DO NOTHING`
	if _, err := s.pool.Exec(ctx, q, userID); err != nil {
		return fmt.Errorf("store.Attempts.EnsureUser %d: %w", userID, err)
	}
	return nil
}

// UpsertAnswer records a user's answer for a question, computing is_correct from
// the supplied isCorrect. It inserts a new row or, on conflict for the
// (user_id, question_id) pair, updates only selected/is_correct/mode/updated_at.
//
// The favorite and marked-uncertain flags are intentionally left out of the
// conflict update so answering (or re-answering) a question never clears a flag
// the user set via PATCH. Returns the resulting row.
func (s *Attempts) UpsertAnswer(ctx context.Context, userID, questionID int64, selected string, isCorrect bool, mode string) (model.Attempt, error) {
	const q = `
		INSERT INTO attempts (user_id, question_id, selected, is_correct, mode, updated_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (user_id, question_id) DO UPDATE SET
			selected   = EXCLUDED.selected,
			is_correct = EXCLUDED.is_correct,
			mode       = EXCLUDED.mode,
			updated_at = now()
		RETURNING ` + attemptColumns
	out, err := scanAttempt(s.pool.QueryRow(ctx, q, userID, questionID, selected, isCorrect, mode))
	if err != nil {
		return model.Attempt{}, fmt.Errorf("store.Attempts.UpsertAnswer u%d/q%d: %w", userID, questionID, err)
	}
	return out, nil
}

// PatchFlags partially updates the favorite / marked-uncertain flags for a
// (user, question). Nil pointers leave the corresponding flag unchanged. When no
// row exists yet one is created (selected NULL, is_correct false) so a user can
// favorite a question before ever answering it. Returns the resulting row.
//
// COALESCE($n, column) applies the new value only when provided; passing the
// flag as a *bool lets pgx send SQL NULL for "unchanged".
func (s *Attempts) PatchFlags(ctx context.Context, userID, questionID int64, isFavorite, isMarkedUncertain *bool) (model.Attempt, error) {
	const q = `
		INSERT INTO attempts (user_id, question_id, is_favorite, is_marked_uncertain, updated_at)
		VALUES ($1, $2, COALESCE($3, false), COALESCE($4, false), now())
		ON CONFLICT (user_id, question_id) DO UPDATE SET
			is_favorite         = COALESCE($3, attempts.is_favorite),
			is_marked_uncertain = COALESCE($4, attempts.is_marked_uncertain),
			updated_at          = now()
		RETURNING ` + attemptColumns
	out, err := scanAttempt(s.pool.QueryRow(ctx, q, userID, questionID, isFavorite, isMarkedUncertain))
	if err != nil {
		return model.Attempt{}, fmt.Errorf("store.Attempts.PatchFlags u%d/q%d: %w", userID, questionID, err)
	}
	return out, nil
}

// Stats returns aggregate counts for a user in a single pass over their rows.
// answered counts rows with a non-NULL selected (a flag-only row is not an
// answer); wrong is answered AND is_correct = false.
func (s *Attempts) Stats(ctx context.Context, userID int64) (model.StatsResponse, error) {
	const q = `
		SELECT
			count(*) FILTER (WHERE selected IS NOT NULL)                          AS answered,
			count(*) FILTER (WHERE selected IS NOT NULL AND is_correct)           AS correct,
			count(*) FILTER (WHERE selected IS NOT NULL AND NOT is_correct)       AS wrong,
			count(*) FILTER (WHERE is_favorite)                                   AS favorites,
			count(*) FILTER (WHERE is_marked_uncertain)                           AS uncertain
		FROM attempts
		WHERE user_id = $1`
	var out model.StatsResponse
	if err := s.pool.QueryRow(ctx, q, userID).Scan(
		&out.Answered, &out.Correct, &out.Wrong, &out.Favorites, &out.Uncertain,
	); err != nil {
		return model.StatsResponse{}, fmt.Errorf("store.Attempts.Stats u%d: %w", userID, err)
	}
	if out.Answered > 0 {
		out.Accuracy = float64(out.Correct) / float64(out.Answered)
	}
	return out, nil
}

// statusReviewWhere maps a review status to its attempts predicate. wrong means
// answered-and-incorrect; favorite / uncertain key off the respective flag.
var statusReviewWhere = map[string]string{
	"wrong":     "a.selected IS NOT NULL AND a.is_correct = false",
	"favorite":  "a.is_favorite = true",
	"uncertain": "a.is_marked_uncertain = true",
}

// ListByStatus returns the user's questions matching a review status, joined with
// the attempt state, paginated and ordered by most-recently-updated. It returns
// the page of items plus the total match count. status must be one of
// wrong|favorite|uncertain (validated upstream); an unknown status is an error.
func (s *Attempts) ListByStatus(ctx context.Context, userID int64, status string, page, pageSize int) ([]model.ReviewItem, int, error) {
	pred, ok := statusReviewWhere[status]
	if !ok {
		return nil, 0, fmt.Errorf("store.Attempts.ListByStatus: unknown status %q", status)
	}
	where := "a.user_id = $1 AND " + pred

	var total int
	countSQL := "SELECT count(*) FROM attempts a WHERE " + where
	if err := s.pool.QueryRow(ctx, countSQL, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("store.Attempts.ListByStatus: count: %w", err)
	}

	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	selectSQL := `
		SELECT ` + questionColumns + `,
			a.selected, a.is_correct, a.is_marked_uncertain, a.is_favorite, a.mode
		FROM attempts a
		JOIN questions q ON q.id = a.question_id
		WHERE ` + where + `
		ORDER BY a.updated_at DESC, q.id
		LIMIT $2 OFFSET $3`
	rows, err := s.pool.Query(ctx, selectSQL, userID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("store.Attempts.ListByStatus: select: %w", err)
	}
	defer rows.Close()

	out := make([]model.ReviewItem, 0)
	for rows.Next() {
		var q model.Question
		var at model.AttemptStatus
		if err := rows.Scan(
			&q.ID, &q.SubjectID, &q.ExamSessionID, &q.Number, &q.Stem, &q.Options,
			&q.Answer, &q.Explanation, &q.Tags, &q.Difficulty, &q.IsActive, &q.CreatedAt, &q.UpdatedAt,
			&at.Selected, &at.IsCorrect, &at.IsMarkedUncertain, &at.IsFavorite, &at.Mode,
		); err != nil {
			return nil, 0, fmt.Errorf("store.Attempts.ListByStatus: scan: %w", err)
		}
		out = append(out, model.ReviewItem{
			Question: q.ToPublic(true), // review mode reveals answer/explanation
			Attempt:  at,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("store.Attempts.ListByStatus: rows: %w", err)
	}
	return out, total, nil
}
