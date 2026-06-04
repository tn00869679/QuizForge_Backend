package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tn00869679/QuizForge_Backend/internal/handler"
)

const userIDKey = "user_id"

// UserStub reads X-User-Id header. If present and parseable as int64,
// stores it in the gin context. Non-parseable values are silently ignored.
func UserStub() gin.HandlerFunc {
	return func(c *gin.Context) {
		if raw := c.GetHeader("X-User-Id"); raw != "" {
			if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
				c.Set(userIDKey, id)
			}
		}
		c.Next()
	}
}

// UserIDFrom retrieves the authenticated user ID from the gin context.
// Returns (0, false) if no user is set.
func UserIDFrom(c *gin.Context) (int64, bool) {
	v, ok := c.Get(userIDKey)
	if !ok {
		return 0, false
	}
	id, ok := v.(int64)
	return id, ok
}

// RequireUser aborts with 401 UNAUTHORIZED if no valid user ID is in context.
func RequireUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := UserIDFrom(c); !ok {
			handler.Fail(c, http.StatusUnauthorized, handler.CodeUnauthorized, "authentication required")
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireAdmin aborts with 403 FORBIDDEN if the X-Admin-Token header does not match token.
func RequireAdmin(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("X-Admin-Token") != token {
			handler.Fail(c, http.StatusForbidden, handler.CodeForbidden, "forbidden")
			c.Abort()
			return
		}
		c.Next()
	}
}
