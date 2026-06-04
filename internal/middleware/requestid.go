package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// RequestID generates or propagates an X-Request-Id header, storing it in the gin context.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-Id")
		if id == "" {
			id = newRequestID()
		}
		c.Set(string(requestIDKey), id)
		c.Header("X-Request-Id", id)
		c.Next()
	}
}

// RequestIDFrom retrieves the request ID from the gin context.
func RequestIDFrom(c *gin.Context) string {
	v, _ := c.Get(string(requestIDKey))
	s, _ := v.(string)
	return s
}

func newRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
