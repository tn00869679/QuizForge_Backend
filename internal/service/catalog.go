// Package service holds business logic: it parses/validates request intent,
// orchestrates store calls, and assembles response DTOs.
package service

import (
	"context"
	"fmt"

	"github.com/tn00869679/QuizForge_Backend/internal/model"
	"github.com/tn00869679/QuizForge_Backend/internal/store"
)

// Catalog serves category / subject / exam-session reads.
type Catalog struct {
	categories   *store.Categories
	subjects     *store.Subjects
	examSessions *store.ExamSessions
}

// NewCatalog constructs a Catalog service from its stores.
func NewCatalog(categories *store.Categories, subjects *store.Subjects, examSessions *store.ExamSessions) *Catalog {
	return &Catalog{categories: categories, subjects: subjects, examSessions: examSessions}
}

// Categories returns all categories.
func (s *Catalog) Categories(ctx context.Context) ([]model.Category, error) {
	cats, err := s.categories.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("service.Catalog.Categories: %w", err)
	}
	return cats, nil
}

// Subjects returns the subjects of a category.
func (s *Catalog) Subjects(ctx context.Context, categoryID int64) ([]model.Subject, error) {
	subs, err := s.subjects.ListByCategory(ctx, categoryID)
	if err != nil {
		return nil, fmt.Errorf("service.Catalog.Subjects: %w", err)
	}
	return subs, nil
}

// ExamSessions returns the exam sessions of a category.
func (s *Catalog) ExamSessions(ctx context.Context, categoryID int64) ([]model.ExamSession, error) {
	sessions, err := s.examSessions.ListByCategory(ctx, categoryID)
	if err != nil {
		return nil, fmt.Errorf("service.Catalog.ExamSessions: %w", err)
	}
	return sessions, nil
}
