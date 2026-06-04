package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ListCategories handles GET /categories.
func (h *Handlers) ListCategories(c *gin.Context) {
	cats, err := h.catalog.Categories(c.Request.Context())
	if err != nil {
		failInternal(c, err)
		return
	}
	Ok(c, cats)
}

// ListSubjects handles GET /categories/:id/subjects.
func (h *Handlers) ListSubjects(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		Fail(c, http.StatusBadRequest, CodeValidation, "invalid category id")
		return
	}
	subs, err := h.catalog.Subjects(c.Request.Context(), id)
	if err != nil {
		failInternal(c, err)
		return
	}
	Ok(c, subs)
}

// ListExamSessions handles GET /exam-sessions?category_id=.
func (h *Handlers) ListExamSessions(c *gin.Context) {
	raw := c.Query("category_id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		Fail(c, http.StatusBadRequest, CodeValidation, "category_id is required and must be a positive integer")
		return
	}
	sessions, err := h.catalog.ExamSessions(c.Request.Context(), id)
	if err != nil {
		failInternal(c, err)
		return
	}
	Ok(c, sessions)
}
