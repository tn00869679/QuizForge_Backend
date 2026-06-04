package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/tn00869679/QuizForge_Backend/internal/service"
	"github.com/tn00869679/QuizForge_Backend/internal/store"
)

// validStatuses is the set of accepted status filter values.
var validStatuses = map[string]struct{}{"unanswered": {}, "wrong": {}, "favorite": {}}

// ListQuestions handles GET /questions, the core question search.
//
// Multi-value params (subject_id, session_ids) accept both repeated keys and
// comma-separated values. limit is one of 5/10/20/50 (exact batch) or all/absent
// (paginated via page/page_size).
func (h *Handlers) ListQuestions(c *gin.Context) {
	q := service.QuestionQuery{
		Keyword:       c.Query("keyword"),
		Status:        c.Query("status"),
		UserID:        userIDPtr(c),
		IncludeAnswer: true,
	}

	if raw := c.Query("category_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			Fail(c, http.StatusBadRequest, CodeValidation, "category_id must be a positive integer")
			return
		}
		q.CategoryID = &id
	}

	subjectIDs, ok := parseInt64CSV(c, "subject_id")
	if !ok {
		Fail(c, http.StatusBadRequest, CodeValidation, "subject_id must be integers")
		return
	}
	q.SubjectIDs = subjectIDs

	sessionIDs, ok := parseInt64CSV(c, "session_ids")
	if !ok {
		Fail(c, http.StatusBadRequest, CodeValidation, "session_ids must be integers")
		return
	}
	q.SessionIDs = sessionIDs

	if q.Status != "" {
		if _, valid := validStatuses[q.Status]; !valid {
			Fail(c, http.StatusBadRequest, CodeValidation, "status must be one of unanswered, wrong, favorite")
			return
		}
		if q.UserID == nil {
			Fail(c, http.StatusBadRequest, CodeValidation, "status filter requires login (X-User-Id)")
			return
		}
	}

	limit, ok := parseLimit(c)
	if !ok {
		Fail(c, http.StatusBadRequest, CodeValidation, "limit must be one of 5, 10, 20, 50, all")
		return
	}
	q.Limit = limit

	random, ok := parseBool(c, "random", false)
	if !ok {
		Fail(c, http.StatusBadRequest, CodeValidation, "random must be a boolean")
		return
	}
	q.Random = random

	page, pageSize, ok := parsePaging(c)
	if !ok {
		Fail(c, http.StatusBadRequest, CodeValidation, "invalid page or page_size (page_size max 100)")
		return
	}
	q.Page = page
	q.PageSize = pageSize

	result, err := h.question.Search(c.Request.Context(), q)
	if err != nil {
		// Defense in depth: the store also guards this, but the handler already
		// validated it above, so reaching here means the same condition.
		if errors.Is(err, service.ErrStatusRequiresUser) {
			Fail(c, http.StatusBadRequest, CodeValidation, "status filter requires login (X-User-Id)")
			return
		}
		failInternal(c, err)
		return
	}
	Ok(c, result)
}

// GetQuestion handles GET /questions/:id. include_answer defaults to true; when
// false the answer and explanation are omitted (exam mode).
func (h *Handlers) GetQuestion(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		Fail(c, http.StatusBadRequest, CodeValidation, "invalid question id")
		return
	}

	includeAnswer, ok := parseBool(c, "include_answer", true)
	if !ok {
		Fail(c, http.StatusBadRequest, CodeValidation, "include_answer must be a boolean")
		return
	}

	q, err := h.question.GetByID(c.Request.Context(), id, includeAnswer)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			Fail(c, http.StatusNotFound, CodeNotFound, "question not found")
			return
		}
		failInternal(c, err)
		return
	}
	Ok(c, q)
}
