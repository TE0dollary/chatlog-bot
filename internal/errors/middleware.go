package errors

import (
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// ErrorHandlerMiddleware is a Gin middleware that attaches a unique request ID
// to every request and writes a structured error response when errors occur.
func ErrorHandlerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := uuid.New().String()
		c.Set("RequestID", requestID)
		c.Header("X-Request-ID", requestID)

		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors[0].Err
			Err(c, err)
			c.Abort()
		}
	}
}

// RecoveryMiddleware is a Gin middleware that recovers from panics and returns 500.
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				var err *Error
				switch v := r.(type) {
				case error:
					err = New(v, http.StatusInternalServerError, "panic recovered")
				default:
					err = Newf(nil, http.StatusInternalServerError, "panic recovered: %v", r)
				}

				log.Err(err).Msgf("PANIC RECOVERED\n%s", string(debug.Stack()))

				c.JSON(http.StatusInternalServerError, err)
				c.Abort()
			}
		}()

		c.Next()
	}
}
