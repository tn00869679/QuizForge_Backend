package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/tn00869679/QuizForge_Backend/internal/model"
	"github.com/tn00869679/QuizForge_Backend/internal/store"
)

// validModes is the set of accepted attempt modes.
var validModes = map[string]struct{}{"practice": {}, "exam": {}}

// validReviewStatuses is the set of accepted statuses for GET /attempts.
var validReviewStatuses = map[string]struct{}{"wrong": {}, "favorite": {}, "uncertain": {}}

// AnswerAttempt handles POST /attempts: it records the user's answer for a
// question (grading it server-side) and upserts the attempt without disturbing
// the favorite/uncertain flags. Requires login (RequireUser upstream).
func (h *Handlers) AnswerAttempt(c *gin.Context) {
	uid, ok := requireUserID(c)
	if !ok {
		return
	}

	var req model.AttemptAnswerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeValidation, "invalid JSON body: "+err.Error())
		return
	}
	if req.QuestionID <= 0 {
		Fail(c, http.StatusBadRequest, CodeValidation, "question_id is required and must be a positive integer")
		return
	}

	req.Selected = strings.ToUpper(strings.TrimSpace(req.Selected))
	if _, valid := validAnswers[req.Selected]; !valid {
		Fail(c, http.StatusBadRequest, CodeValidation, "selected must be one of A, B, C, D")
		return
	}
	if req.Mode == "" {
		req.Mode = "practice"
	}
	if _, valid := validModes[req.Mode]; !valid {
		Fail(c, http.StatusBadRequest, CodeValidation, "mode must be one of practice, exam")
		return
	}

	out, err := h.attempts.Answer(c.Request.Context(), uid, req)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			Fail(c, http.StatusNotFound, CodeNotFound, "question not found")
			return
		}
		failInternal(c, err)
		return
	}
	Ok(c, out)
}

// PatchAttempt handles PATCH /attempts/:question_id: partial update of the
// favorite / uncertain flags, creating the row if absent. Requires login.
func (h *Handlers) PatchAttempt(c *gin.Context) {
	uid, ok := requireUserID(c)
	if !ok {
		return
	}

	questionID, err := strconv.ParseInt(c.Param("question_id"), 10, 64)
	if err != nil || questionID <= 0 {
		Fail(c, http.StatusBadRequest, CodeValidation, "invalid question id")
		return
	}

	var req model.AttemptPatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeValidation, "invalid JSON body: "+err.Error())
		return
	}
	if req.IsFavorite == nil && req.IsMarkedUncertain == nil {
		Fail(c, http.StatusBadRequest, CodeValidation, "at least one of is_favorite, is_marked_uncertain is required")
		return
	}

	out, err := h.attempts.Patch(c.Request.Context(), uid, questionID, req)
	if err != nil {
		failInternal(c, err)
		return
	}
	Ok(c, out)
}

// GetStats handles GET /stats: aggregate counts for the authenticated user.
func (h *Handlers) GetStats(c *gin.Context) {
	uid, ok := requireUserID(c)
	if !ok {
		return
	}
	out, err := h.attempts.Stats(c.Request.Context(), uid)
	if err != nil {
		failInternal(c, err)
		return
	}
	Ok(c, out)
}

// ListAttempts handles GET /attempts?status=wrong|favorite|uncertain: paginated
// review list of the user's questions with their attempt state. Requires login.
func (h *Handlers) ListAttempts(c *gin.Context) {
	uid, ok := requireUserID(c)
	if !ok {
		return
	}

	status := c.Query("status")
	if _, valid := validReviewStatuses[status]; !valid {
		Fail(c, http.StatusBadRequest, CodeValidation, "status must be one of wrong, favorite, uncertain")
		return
	}

	page, pageSize, ok := parsePaging(c)
	if !ok {
		Fail(c, http.StatusBadRequest, CodeValidation, "invalid page or page_size (page_size max 100)")
		return
	}

	out, err := h.attempts.Review(c.Request.Context(), uid, status, page, pageSize)
	if err != nil {
		failInternal(c, err)
		return
	}
	Ok(c, out)
}
