package middleware

import (
	"net/http"
	"sync"

	"github.com/labstack/echo/v4"
)

// UploadLimiter creates a middleware that limits concurrent uploads
type UploadLimiter struct {
	semaphore chan struct{}
	mu        sync.Mutex
	active    int
	maxActive int
}

// NewUploadLimiter creates a new upload limiter with the specified max concurrent uploads
func NewUploadLimiter(maxConcurrent int) *UploadLimiter {
	return &UploadLimiter{
		semaphore: make(chan struct{}, maxConcurrent),
		maxActive: maxConcurrent,
	}
}

// Middleware returns the Echo middleware function
func (ul *UploadLimiter) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Try to acquire a slot
			select {
			case ul.semaphore <- struct{}{}:
				// Got a slot, increment active count
				ul.mu.Lock()
				ul.active++
				ul.mu.Unlock()

				// Process request
				defer func() {
					// Release the slot
					<-ul.semaphore
					ul.mu.Lock()
					ul.active--
					ul.mu.Unlock()
				}()

				return next(c)
			default:
				// No slots available, return 503 Service Unavailable
				return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
					"error":       "Server is currently processing maximum number of uploads",
					"retry_after": 30, // seconds
				})
			}
		}
	}
}
