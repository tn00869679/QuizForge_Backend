package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tn00869679/QuizForge_Backend/internal/model"
)

// Categories provides read access to the categories table.
type Categories struct {
	pool *pgxpool.Pool
}

// NewCategories constructs a Categories store backed by the given pool.
func NewCategories(pool *pgxpool.Pool) *Categories {
	return &Categories{pool: pool}
}

// List returns all categories ordered by code.
func (s *Categories) List(ctx context.Context) ([]model.Category, error) {
	const q = `
		SELECT id, code, name, description, created_at
		FROM categories
		ORDER BY code`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("store.Categories.List: query: %w", err)
	}
	defer rows.Close()

	out := make([]model.Category, 0)
	for rows.Next() {
		var c model.Category
		if err := rows.Scan(&c.ID, &c.Code, &c.Name, &c.Description, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("store.Categories.List: scan: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store.Categories.List: rows: %w", err)
	}
	return out, nil
}

// Subjects provides read access to the subjects table.
type Subjects struct {
	pool *pgxpool.Pool
}

// NewSubjects constructs a Subjects store backed by the given pool.
func NewSubjects(pool *pgxpool.Pool) *Subjects {
	return &Subjects{pool: pool}
}

// ListByCategory returns subjects for a category ordered by order_index then name.
func (s *Subjects) ListByCategory(ctx context.Context, categoryID int64) ([]model.Subject, error) {
	const q = `
		SELECT id, category_id, name, order_index, created_at
		FROM subjects
		WHERE category_id = $1
		ORDER BY order_index, name`
	rows, err := s.pool.Query(ctx, q, categoryID)
	if err != nil {
		return nil, fmt.Errorf("store.Subjects.ListByCategory: query: %w", err)
	}
	defer rows.Close()

	out := make([]model.Subject, 0)
	for rows.Next() {
		var sub model.Subject
		if err := rows.Scan(&sub.ID, &sub.CategoryID, &sub.Name, &sub.OrderIndex, &sub.CreatedAt); err != nil {
			return nil, fmt.Errorf("store.Subjects.ListByCategory: scan: %w", err)
		}
		out = append(out, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store.Subjects.ListByCategory: rows: %w", err)
	}
	return out, nil
}

// ExamSessions provides read access to the exam_sessions table.
type ExamSessions struct {
	pool *pgxpool.Pool
}

// NewExamSessions constructs an ExamSessions store backed by the given pool.
func NewExamSessions(pool *pgxpool.Pool) *ExamSessions {
	return &ExamSessions{pool: pool}
}

// ListByCategory returns exam sessions for a category, newest first.
func (s *ExamSessions) ListByCategory(ctx context.Context, categoryID int64) ([]model.ExamSession, error) {
	const q = `
		SELECT id, category_id, year, term, source_url, label, created_at
		FROM exam_sessions
		WHERE category_id = $1
		ORDER BY year DESC, term DESC`
	rows, err := s.pool.Query(ctx, q, categoryID)
	if err != nil {
		return nil, fmt.Errorf("store.ExamSessions.ListByCategory: query: %w", err)
	}
	defer rows.Close()

	out := make([]model.ExamSession, 0)
	for rows.Next() {
		var e model.ExamSession
		if err := rows.Scan(&e.ID, &e.CategoryID, &e.Year, &e.Term, &e.SourceURL, &e.Label, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("store.ExamSessions.ListByCategory: scan: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store.ExamSessions.ListByCategory: rows: %w", err)
	}
	return out, nil
}
