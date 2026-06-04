package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tn00869679/QuizForge_Backend/internal/model"
	"github.com/tn00869679/QuizForge_Backend/internal/service"
)

// Import handles POST /admin/import: it decodes the import payload, applies it
// transactionally, and returns per-entity counts. The route is guarded by
// RequireAdmin upstream.
func (h *Handlers) Import(c *gin.Context) {
	var payload model.ImportPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		Fail(c, http.StatusBadRequest, CodeValidation, "invalid JSON body: "+err.Error())
		return
	}

	res, err := h.imports.Apply(c.Request.Context(), payload)
	if err != nil {
		var ve *service.ValidationError
		if errors.As(err, &ve) {
			Fail(c, http.StatusBadRequest, CodeValidation, ve.Msg)
			return
		}
		failInternal(c, err)
		return
	}
	Ok(c, res)
}
