package middleware

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tn00869679/QuizForge_Backend/internal/handler"
)

// Recovery returns a middleware that recovers from panics, logs the error, and returns 500.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic recovered",
					"request_id", RequestIDFrom(c),
					"error", r,
				)
				handler.Fail(c, http.StatusInternalServerError, handler.CodeInternal, "internal server error")
				c.Abort()
			}
		}()
		c.Next()
	}
}
