package store

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tn00869679/QuizForge_Backend/internal/model"
)

// Questions provides read access to the questions table.
type Questions struct {
	pool *pgxpool.Pool
}

// NewQuestions constructs a Questions store backed by the given pool.
func NewQuestions(pool *pgxpool.Pool) *Questions {
	return &Questions{pool: pool}
}

// questionColumns is the canonical column list / scan order for a questions row.
// Columns are qualified with the q alias so the list is safe to use in queries
// that join other tables (e.g. subjects, which also has an id column). Every
// query using this list must alias the questions table as q.
const questionColumns = `q.id, q.subject_id, q.exam_session_id, q.number, q.stem, q.options,
	q.answer, q.explanation, q.tags, q.difficulty, q.is_active, q.created_at, q.updated_at`

// scanQuestion scans a single questions row in questionColumns order.
func scanQuestion(row pgx.Row) (model.Question, error) {
	var q model.Question
	err := row.Scan(
		&q.ID, &q.SubjectID, &q.ExamSessionID, &q.Number, &q.Stem, &q.Options,
		&q.Answer, &q.Explanation, &q.Tags, &q.Difficulty, &q.IsActive, &q.CreatedAt, &q.UpdatedAt,
	)
	return q, err
}

// GetByID returns a single question by id. Returns ErrNotFound if absent.
// Inactive questions are still returned (a direct id lookup is explicit).
func (s *Questions) GetByID(ctx context.Context, id int64) (model.Question, error) {
	q := `SELECT ` + questionColumns + ` FROM questions q WHERE q.id = $1`
	out, err := scanQuestion(s.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Question{}, ErrNotFound
		}
		return model.Question{}, fmt.Errorf("store.Questions.GetByID: %w", err)
	}
	return out, nil
}

// QueryFilter holds all supported filters for Questions.Query.
type QueryFilter struct {
	CategoryID *int64
	SubjectIDs []int64
	SessionIDs []int64
	Keyword    string // matched as stem ILIKE '%keyword%' (uses pg_trgm GIN index)
	Status     string // "" | "unanswered" | "wrong" | "favorite"
	UserID     *int64 // required when Status is set

	// Limit, when > 0, is the "this batch" question count (5/10/20/50): exactly
	// that many rows are returned and Page/PageSize are ignored. When Limit == 0
	// (limit=all) the result is paginated via Page/PageSize.
	Limit  int
	Random bool // ORDER BY random() instead of the natural session/subject/number order

	Page     int // 1-based; used only when Limit == 0
	PageSize int // used only when Limit == 0
}

// Query returns questions matching the filter together with the total count of
// matches (ignoring limit/paging). It always restricts to is_active = true.
//
// status filtering joins the attempts table for filter.UserID:
//   - unanswered: questions with no attempt row for the user
//   - wrong:      the user's attempt has is_correct = false
//   - favorite:   the user's attempt has is_favorite = true
//
// A status filter without a UserID returns ErrStatusRequiresUser.
//
// Ordering: Random -> ORDER BY random(); otherwise ORDER BY exam_session_id,
// subject_id, number.
//
// Limit vs page (see QueryFilter.Limit): Limit > 0 takes exactly that many rows
// and ignores Page; Limit == 0 paginates with Page/PageSize.
func (s *Questions) Query(ctx context.Context, f QueryFilter) ([]model.Question, int, error) {
	if f.Status != "" && f.UserID == nil {
		return nil, 0, ErrStatusRequiresUser
	}

	// Build the shared FROM + WHERE once; reuse for both count and page queries.
	var (
		args  []any
		where []string
	)
	// next appends a value to args and returns its "$N" placeholder.
	next := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}

	from := "FROM questions q"
	where = append(where, "q.is_active = true")

	if f.CategoryID != nil {
		// category is reached via the question's subject.
		from += " JOIN subjects s ON s.id = q.subject_id"
		where = append(where, "s.category_id = "+next(*f.CategoryID))
	}
	if len(f.SubjectIDs) > 0 {
		where = append(where, "q.subject_id = ANY("+next(f.SubjectIDs)+")")
	}
	if len(f.SessionIDs) > 0 {
		where = append(where, "q.exam_session_id = ANY("+next(f.SessionIDs)+")")
	}
	if f.Keyword != "" {
		// ILIKE with both wildcards is accelerated by the pg_trgm GIN index on stem.
		where = append(where, "q.stem ILIKE "+next("%"+f.Keyword+"%"))
	}

	switch f.Status {
	case "unanswered":
		where = append(where, "NOT EXISTS (SELECT 1 FROM attempts a WHERE a.question_id = q.id AND a.user_id = "+next(*f.UserID)+")")
	case "wrong":
		where = append(where, "EXISTS (SELECT 1 FROM attempts a WHERE a.question_id = q.id AND a.user_id = "+next(*f.UserID)+" AND a.is_correct = false)")
	case "favorite":
		where = append(where, "EXISTS (SELECT 1 FROM attempts a WHERE a.question_id = q.id AND a.user_id = "+next(*f.UserID)+" AND a.is_favorite = true)")
	case "":
		// no status filter
	default:
		return nil, 0, fmt.Errorf("store.Questions.Query: unknown status %q", f.Status)
	}

	whereSQL := "WHERE " + strings.Join(where, " AND ")

	// Total count of matches, independent of limit/paging.
	var total int
	countSQL := "SELECT count(*) " + from + " " + whereSQL
	if err := s.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("store.Questions.Query: count: %w", err)
	}

	// ORDER BY.
	orderSQL := "ORDER BY q.exam_session_id, q.subject_id, q.number"
	if f.Random {
		orderSQL = "ORDER BY random()"
	}

	// LIMIT / OFFSET. Limit > 0 wins and ignores paging; otherwise page through.
	var limitSQL string
	switch {
	case f.Limit > 0:
		limitSQL = "LIMIT " + next(f.Limit)
	default:
		size := f.PageSize
		page := f.Page
		if page < 1 {
			page = 1
		}
		offset := (page - 1) * size
		limitSQL = "LIMIT " + next(size) + " OFFSET " + next(offset)
	}

	selectSQL := "SELECT " + questionColumns + " " + from + " " + whereSQL + " " + orderSQL + " " + limitSQL
	rows, err := s.pool.Query(ctx, selectSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("store.Questions.Query: select: %w", err)
	}
	defer rows.Close()

	out := make([]model.Question, 0)
	for rows.Next() {
		q, err := scanQuestion(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("store.Questions.Query: scan: %w", err)
		}
		out = append(out, q)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("store.Questions.Query: rows: %w", err)
	}
	return out, total, nil
}
