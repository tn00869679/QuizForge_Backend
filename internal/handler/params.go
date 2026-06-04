package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// requireUserID returns the authenticated user id, writing a 401 and returning
// ok=false when none is present. RequireUser middleware already guards the
// status routes, so a missing id here is defense in depth rather than an
// expected path.
func requireUserID(c *gin.Context) (int64, bool) {
	if id := userIDPtr(c); id != nil {
		return *id, true
	}
	Fail(c, http.StatusUnauthorized, CodeUnauthorized, "authentication required")
	return 0, false
}

const (
	defaultPageSize = 20
	maxPageSize     = 100

	// ctxKeyUserID / ctxKeyRequestID mirror the gin-context keys set by
	// internal/middleware. They are referenced by string (not by importing
	// middleware) because middleware already imports this package for the
	// response helpers, and importing it back would create a cycle.
	ctxKeyUserID    = "user_id"
	ctxKeyRequestID = "request_id"
)

// allowedLimits is the whitelist of "this batch" question counts. 0 (mapped from
// limit=all or absent) means paginate via page/page_size.
var allowedLimits = map[int]struct{}{5: {}, 10: {}, 20: {}, 50: {}}

// validAnswers is the set of accepted single-choice answer keys, used to
// validate submitted selections.
var validAnswers = map[string]struct{}{"A": {}, "B": {}, "C": {}, "D": {}}

// failInternal logs the underlying error with the request id and returns a
// generic 500 without leaking internals to the client.
func failInternal(c *gin.Context, err error) {
	slog.Error("request failed",
		"request_id", c.GetString(ctxKeyRequestID),
		"method", c.Request.Method,
		"path", c.FullPath(),
		"error", err,
	)
	Fail(c, http.StatusInternalServerError, CodeInternal, "internal error")
}

// parseInt64CSV parses a multi-value int64 query param. It accepts BOTH repeated
// keys (k=1&k=2) and comma-separated values (k=1,2), merging the two forms.
// Empty / blank entries are skipped. A non-integer entry yields ok=false.
func parseInt64CSV(c *gin.Context, key string) (out []int64, ok bool) {
	for _, raw := range c.QueryArray(key) {
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			v, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				return nil, false
			}
			out = append(out, v)
		}
	}
	return out, true
}

// parsePaging reads page (default 1, min 1) and page_size (default 20, max 100).
// Returns ok=false on malformed or out-of-range values.
func parsePaging(c *gin.Context) (page, pageSize int, ok bool) {
	page = 1
	pageSize = defaultPageSize

	if raw := c.Query("page"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 {
			return 0, 0, false
		}
		page = v
	}
	if raw := c.Query("page_size"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 || v > maxPageSize {
			return 0, 0, false
		}
		pageSize = v
	}
	return page, pageSize, true
}

// parseLimit reads the limit param. Absent or "all" -> 0 (paginate). Otherwise it
// must be one of the whitelisted batch sizes. Returns ok=false otherwise.
func parseLimit(c *gin.Context) (limit int, ok bool) {
	raw := c.Query("limit")
	if raw == "" || raw == "all" {
		return 0, true
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	if _, allowed := allowedLimits[v]; !allowed {
		return 0, false
	}
	return v, true
}

// parseBool reads an optional boolean param, returning def when absent and
// ok=false when present but unparseable.
func parseBool(c *gin.Context, key string, def bool) (val, ok bool) {
	raw := c.Query(key)
	if raw == "" {
		return def, true
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return def, false
	}
	return v, true
}

// userIDPtr returns the optional authenticated user id from context as a pointer
// (nil when no user is set), suitable for filter structs. It reads the same
// context key that middleware.UserStub writes (see ctxKeyUserID).
func userIDPtr(c *gin.Context) *int64 {
	v, ok := c.Get(ctxKeyUserID)
	if !ok {
		return nil
	}
	id, ok := v.(int64)
	if !ok {
		return nil
	}
	return &id
}
