package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/tn00869679/QuizForge_Backend/internal/model"
	"github.com/tn00869679/QuizForge_Backend/internal/service"
)

// StartExam handles POST /exam/start: it samples a category into a timed exam,
// returning the answer-free question set and a signed exam token. Public route.
func (h *Handlers) StartExam(c *gin.Context) {
	var req model.ExamStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeValidation, "invalid JSON body: "+err.Error())
		return
	}
	if req.CategoryID <= 0 {
		Fail(c, http.StatusBadRequest, CodeValidation, "category_id is required and must be a positive integer")
		return
	}

	resp, err := h.exam.Start(c.Request.Context(), req.CategoryID, req.PerSubjectN, req.DurationSec)
	if err != nil {
		if errors.Is(err, service.ErrUnknownCategory) {
			Fail(c, http.StatusNotFound, CodeNotFound, "no questions available for this category")
			return
		}
		failInternal(c, err)
		return
	}
	Ok(c, resp)
}

// GradeExam handles POST /exam/grade: it verifies the exam token, scores the
// submitted answers, and returns per-question and per-subject results. A
// malformed token is 400; a signature mismatch is 401. An authentic but expired
// token is still graded (expired=true). Public route.
func (h *Handlers) GradeExam(c *gin.Context) {
	var req model.ExamGradeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeValidation, "invalid JSON body: "+err.Error())
		return
	}
	if strings.TrimSpace(req.ExamToken) == "" {
		Fail(c, http.StatusBadRequest, CodeValidation, "exam_token is required")
		return
	}

	// Normalise + validate each submitted choice. An empty selected (skipped
	// question) is allowed; a non-empty one must be A-D.
	for i := range req.Answers {
		sel := strings.ToUpper(strings.TrimSpace(req.Answers[i].Selected))
		if sel != "" {
			if _, ok := validAnswers[sel]; !ok {
				Fail(c, http.StatusBadRequest, CodeValidation, "answers.selected must be one of A, B, C, D")
				return
			}
		}
		req.Answers[i].Selected = sel
	}

	resp, err := h.exam.Grade(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidToken):
			Fail(c, http.StatusBadRequest, CodeValidation, "malformed exam_token")
			return
		case errors.Is(err, service.ErrBadSignature):
			Fail(c, http.StatusUnauthorized, CodeUnauthorized, "invalid exam_token signature")
			return
		}
		failInternal(c, err)
		return
	}
	Ok(c, resp)
}
