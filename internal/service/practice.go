package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/tn00869679/QuizForge_Backend/internal/store"
)

// Practice serves practice-set generation. It reuses the question query engine
// and projects the result down to just the matching ids, which the client then
// walks through.
type Practice struct {
	questions *store.Questions
}

// NewPractice constructs a Practice service from the questions store.
func NewPractice(questions *store.Questions) *Practice {
	return &Practice{questions: questions}
}

// Generate runs a question query (same filter semantics as the question search)
// and returns only the ordered ids. A status filter without a user surfaces
// ErrStatusRequiresUser for the handler to map to 400.
func (s *Practice) Generate(ctx context.Context, q QuestionQuery) ([]int64, error) {
	filter := store.QueryFilter{
		CategoryID: q.CategoryID,
		SubjectIDs: q.SubjectIDs,
		SessionIDs: q.SessionIDs,
		Keyword:    q.Keyword,
		Status:     q.Status,
		UserID:     q.UserID,
		Limit:      q.Limit,
		Random:     q.Random,
		Page:       q.Page,
		PageSize:   q.PageSize,
	}

	items, _, err := s.questions.Query(ctx, filter)
	if err != nil {
		if errors.Is(err, store.ErrStatusRequiresUser) {
			return nil, ErrStatusRequiresUser
		}
		return nil, fmt.Errorf("service.Practice.Generate: %w", err)
	}

	ids := make([]int64, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	return ids, nil
}
