package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/tn00869679/QuizForge_Backend/internal/model"
	"github.com/tn00869679/QuizForge_Backend/internal/store"
)

// Attempts serves answer recording, flag patches, stats, and review listing for
// the authenticated user. is_correct is computed here by comparing the submitted
// choice against the question's stored answer (never trusting the client).
type Attempts struct {
	attempts  *store.Attempts
	questions *store.Questions
}

// NewAttempts constructs an Attempts service from its stores.
func NewAttempts(attempts *store.Attempts, questions *store.Questions) *Attempts {
	return &Attempts{attempts: attempts, questions: questions}
}

// Answer records the user's answer for a question. It loads the question to grade
// the choice server-side, ensures the (stub) user exists, then upserts the
// attempt — preserving any existing favorite / uncertain flags. Returns
// store.ErrNotFound if the question does not exist.
func (s *Attempts) Answer(ctx context.Context, userID int64, req model.AttemptAnswerRequest) (model.Attempt, error) {
	q, err := s.questions.GetByID(ctx, req.QuestionID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return model.Attempt{}, store.ErrNotFound
		}
		return model.Attempt{}, fmt.Errorf("service.Attempts.Answer: load question: %w", err)
	}

	isCorrect := req.Selected == q.Answer

	if err := s.attempts.EnsureUser(ctx, userID); err != nil {
		return model.Attempt{}, fmt.Errorf("service.Attempts.Answer: %w", err)
	}
	out, err := s.attempts.UpsertAnswer(ctx, userID, req.QuestionID, req.Selected, isCorrect, req.Mode)
	if err != nil {
		return model.Attempt{}, fmt.Errorf("service.Attempts.Answer: %w", err)
	}
	return out, nil
}

// Patch partially updates the favorite / uncertain flags for a question,
// creating the attempt row if it does not exist yet. nil fields are unchanged.
func (s *Attempts) Patch(ctx context.Context, userID, questionID int64, req model.AttemptPatchRequest) (model.Attempt, error) {
	if err := s.attempts.EnsureUser(ctx, userID); err != nil {
		return model.Attempt{}, fmt.Errorf("service.Attempts.Patch: %w", err)
	}
	out, err := s.attempts.PatchFlags(ctx, userID, questionID, req.IsFavorite, req.IsMarkedUncertain)
	if err != nil {
		return model.Attempt{}, fmt.Errorf("service.Attempts.Patch: %w", err)
	}
	return out, nil
}

// Stats returns the user's aggregate answer/flag counts.
func (s *Attempts) Stats(ctx context.Context, userID int64) (model.StatsResponse, error) {
	out, err := s.attempts.Stats(ctx, userID)
	if err != nil {
		return model.StatsResponse{}, fmt.Errorf("service.Attempts.Stats: %w", err)
	}
	return out, nil
}

// Review returns a paginated list of the user's questions for a review status
// (wrong|favorite|uncertain), each with its attempt state.
func (s *Attempts) Review(ctx context.Context, userID int64, status string, page, pageSize int) (model.Page[model.ReviewItem], error) {
	items, total, err := s.attempts.ListByStatus(ctx, userID, status, page, pageSize)
	if err != nil {
		return model.Page[model.ReviewItem]{}, fmt.Errorf("service.Attempts.Review: %w", err)
	}
	return model.Page[model.ReviewItem]{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}
