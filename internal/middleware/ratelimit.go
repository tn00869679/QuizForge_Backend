package middleware

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/tn00869679/QuizForge_Backend/internal/handler"
	"golang.org/x/time/rate"
)

type ipLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	r        rate.Limit
	b        int
}

func newIPLimiter(r rate.Limit, b int) *ipLimiter {
	return &ipLimiter{
		limiters: make(map[string]*rate.Limiter),
		r:        r,
		b:        b,
	}
}

func (l *ipLimiter) get(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	if lim, ok := l.limiters[ip]; ok {
		return lim
	}
	lim := rate.NewLimiter(l.r, l.b)
	l.limiters[ip] = lim
	return lim
}

// RateLimit returns a per-IP token bucket rate limiting middleware.
// r is the rate (requests/second), b is the burst size.
func RateLimit(r rate.Limit, b int) gin.HandlerFunc {
	limiter := newIPLimiter(r, b)
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !limiter.get(ip).Allow() {
			handler.Fail(c, http.StatusTooManyRequests, handler.CodeRateLimited, "rate limit exceeded")
			c.Abort()
			return
		}
		c.Next()
	}
}
