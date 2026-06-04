package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/tn00869679/QuizForge_Backend/internal/model"
	"github.com/tn00869679/QuizForge_Backend/internal/store"
)

// ErrStatusRequiresUser is surfaced to handlers (mapped to 400 VALIDATION) when
// a status filter is requested without an authenticated user. It mirrors
// store.ErrStatusRequiresUser so callers can match on either via errors.Is.
var ErrStatusRequiresUser = store.ErrStatusRequiresUser

// QuestionQuery is the validated input for a question search. Handlers are
// responsible for parsing/validating raw query params into this struct.
type QuestionQuery struct {
	CategoryID    *int64
	SubjectIDs    []int64
	SessionIDs    []int64
	Keyword       string
	Status        string // "" | unanswered | wrong | favorite
	UserID        *int64
	Limit         int // 0 = all (paginated); 5/10/20/50 = exact batch size
	Random        bool
	Page          int
	PageSize      int
	IncludeAnswer bool // false -> strip answer/explanation from each item
}

// Questions serves the core question search and single-question read.
type Questions struct {
	questions *store.Questions
}

// NewQuestions constructs a Questions service from its store.
func NewQuestions(questions *store.Questions) *Questions {
	return &Questions{questions: questions}
}

// Search runs a question query and assembles the paginated public response.
//
// Page semantics mirror the store: when Limit > 0 the result is a single
// "batch" of exactly Limit rows, so the response reports page=1 and
// page_size=len(items); when Limit == 0 the response echoes the request's
// page/page_size. total is always the unfiltered-by-paging match count.
func (s *Questions) Search(ctx context.Context, q QuestionQuery) (model.Page[model.QuestionPublic], error) {
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

	items, total, err := s.questions.Query(ctx, filter)
	if err != nil {
		if errors.Is(err, store.ErrStatusRequiresUser) {
			return model.Page[model.QuestionPublic]{}, ErrStatusRequiresUser
		}
		return model.Page[model.QuestionPublic]{}, fmt.Errorf("service.Questions.Search: %w", err)
	}

	public := make([]model.QuestionPublic, 0, len(items))
	for _, it := range items {
		public = append(public, it.ToPublic(q.IncludeAnswer))
	}

	page, pageSize := q.Page, q.PageSize
	if q.Limit > 0 {
		page, pageSize = 1, len(public)
	}

	return model.Page[model.QuestionPublic]{
		Items:    public,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

// GetByID returns a single question as its public DTO. When includeAnswer is
// false the answer and explanation are stripped (exam mode). Returns
// store.ErrNotFound if the question does not exist.
func (s *Questions) GetByID(ctx context.Context, id int64, includeAnswer bool) (model.QuestionPublic, error) {
	q, err := s.questions.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return model.QuestionPublic{}, store.ErrNotFound
		}
		return model.QuestionPublic{}, fmt.Errorf("service.Questions.GetByID: %w", err)
	}
	return q.ToPublic(includeAnswer), nil
}
