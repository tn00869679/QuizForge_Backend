package service

import (
	"context"
	"fmt"

	"github.com/tn00869679/QuizForge_Backend/internal/model"
	"github.com/tn00869679/QuizForge_Backend/internal/store"
)

// ValidationError is returned by Import.Apply when the payload is malformed. The
// handler maps it to a 400 VALIDATION response.
type ValidationError struct {
	Msg string
}

func (e *ValidationError) Error() string { return e.Msg }

// validAnswers is the set of accepted answer keys.
var validAnswers = map[string]struct{}{"A": {}, "B": {}, "C": {}, "D": {}}

// Import validates and applies an import payload.
type Import struct {
	store *store.Import
}

// NewImport constructs an Import service from its store.
func NewImport(s *store.Import) *Import {
	return &Import{store: s}
}

// Apply validates the payload and applies it in one transaction, returning
// per-entity counts. Validation failures are returned as *ValidationError.
func (s *Import) Apply(ctx context.Context, payload model.ImportPayload) (model.ImportResult, error) {
	if err := validatePayload(payload); err != nil {
		return model.ImportResult{}, err
	}
	res, err := s.store.Apply(ctx, payload)
	if err != nil {
		return model.ImportResult{}, fmt.Errorf("service.Import.Apply: %w", err)
	}
	return res, nil
}

// validatePayload enforces the import contract before any DB write.
func validatePayload(p model.ImportPayload) error {
	if p.CategoryCode == "" {
		return &ValidationError{Msg: "category_code is required"}
	}
	if p.CategoryName == "" {
		return &ValidationError{Msg: "category_name is required"}
	}
	if len(p.Questions) == 0 {
		return &ValidationError{Msg: "questions must not be empty"}
	}
	for i, q := range p.Questions {
		where := fmt.Sprintf("questions[%d]", i)
		if q.Year <= 0 {
			return &ValidationError{Msg: where + ": year must be positive"}
		}
		if q.Term <= 0 {
			return &ValidationError{Msg: where + ": term must be positive"}
		}
		if q.Subject == "" {
			return &ValidationError{Msg: where + ": subject is required"}
		}
		if q.Number <= 0 {
			return &ValidationError{Msg: where + ": number must be positive"}
		}
		if q.Stem == "" {
			return &ValidationError{Msg: where + ": stem is required"}
		}
		if len(q.Options) < 2 {
			return &ValidationError{Msg: where + ": at least 2 options are required"}
		}
		for j, opt := range q.Options {
			if opt.Key == "" || opt.Text == "" {
				return &ValidationError{Msg: fmt.Sprintf("%s.options[%d]: key and text are required", where, j)}
			}
		}
		if _, ok := validAnswers[q.Answer]; !ok {
			return &ValidationError{Msg: where + ": answer must be one of A, B, C, D"}
		}
	}
	return nil
}
