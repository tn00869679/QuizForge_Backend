package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Error code constants used in API error responses.
const (
	CodeValidation  = "VALIDATION"
	CodeNotFound    = "NOT_FOUND"
	CodeUnauthorized = "UNAUTHORIZED"
	CodeForbidden   = "FORBIDDEN"
	CodeRateLimited = "RATE_LIMITED"
	CodeInternal    = "INTERNAL"
)

type envelope struct {
	Data  any          `json:"data"`
	Error *errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Ok responds with HTTP 200 and the given data in the envelope.
func Ok(c *gin.Context, data any) {
	OkStatus(c, http.StatusOK, data)
}

// OkStatus responds with the given HTTP status and data in the envelope.
func OkStatus(c *gin.Context, status int, data any) {
	c.JSON(status, envelope{Data: data, Error: nil})
}

// Fail responds with the given HTTP status and an error envelope.
func Fail(c *gin.Context, status int, code, msg string) {
	c.JSON(status, envelope{Data: nil, Error: &errorDetail{Code: code, Message: msg}})
}
