package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tn00869679/QuizForge_Backend/internal/model"
	"github.com/tn00869679/QuizForge_Backend/internal/service"
)

// practiceGenerateRequest is the JSON body of POST /practice/generate. It mirrors
// the GET /questions filter params, but as a body for convenience.
type practiceGenerateRequest struct {
	CategoryID *int64  `json:"category_id"`
	SubjectIDs []int64 `json:"subject_id"`
	SessionIDs []int64 `json:"session_ids"`
	Keyword    string  `json:"keyword"`
	Status     string  `json:"status"`
	Limit      *int    `json:"limit"`
	Random     bool    `json:"random"`
	Page       *int    `json:"page"`
	PageSize   *int    `json:"page_size"`
}

// GeneratePractice handles POST /practice/generate: it runs the same query as the
// question search and returns just the matching ids. A status filter requires an
// X-User-Id (else 400). Public route (no login required for unfiltered use).
func (h *Handlers) GeneratePractice(c *gin.Context) {
	var req practiceGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeValidation, "invalid JSON body: "+err.Error())
		return
	}

	if req.CategoryID != nil && *req.CategoryID <= 0 {
		Fail(c, http.StatusBadRequest, CodeValidation, "category_id must be a positive integer")
		return
	}

	q := service.QuestionQuery{
		CategoryID: req.CategoryID,
		SubjectIDs: req.SubjectIDs,
		SessionIDs: req.SessionIDs,
		Keyword:    req.Keyword,
		Status:     req.Status,
		UserID:     userIDPtr(c),
		Random:     req.Random,
	}

	if req.Status != "" {
		if _, valid := validStatuses[req.Status]; !valid {
			Fail(c, http.StatusBadRequest, CodeValidation, "status must be one of unanswered, wrong, favorite")
			return
		}
		if q.UserID == nil {
			Fail(c, http.StatusBadRequest, CodeValidation, "status filter requires login (X-User-Id)")
			return
		}
	}

	if req.Limit != nil {
		if _, allowed := allowedLimits[*req.Limit]; !allowed {
			Fail(c, http.StatusBadRequest, CodeValidation, "limit must be one of 5, 10, 20, 50")
			return
		}
		q.Limit = *req.Limit
	}

	q.Page = 1
	q.PageSize = defaultPageSize
	if req.Page != nil {
		if *req.Page < 1 {
			Fail(c, http.StatusBadRequest, CodeValidation, "page must be >= 1")
			return
		}
		q.Page = *req.Page
	}
	if req.PageSize != nil {
		if *req.PageSize < 1 || *req.PageSize > maxPageSize {
			Fail(c, http.StatusBadRequest, CodeValidation, "page_size must be between 1 and 100")
			return
		}
		q.PageSize = *req.PageSize
	}

	ids, err := h.practice.Generate(c.Request.Context(), q)
	if err != nil {
		if errors.Is(err, service.ErrStatusRequiresUser) {
			Fail(c, http.StatusBadRequest, CodeValidation, "status filter requires login (X-User-Id)")
			return
		}
		failInternal(c, err)
		return
	}
	Ok(c, model.PracticeGenerateResponse{QuestionIDs: ids})
}
