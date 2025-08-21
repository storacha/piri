package aws

import (
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/storacha/piri/pkg/server/handler"
)

type HandlerContext struct {
	response *echo.Response
	request  *http.Request
}

func (c *HandlerContext) Request() *http.Request {
	return c.request
}

func (c *HandlerContext) Response() *echo.Response {
	return c.response
}

func (c *HandlerContext) Stream(code int, contentType string, r io.Reader) error {
	header := c.Response().Header()
	if header.Get(echo.HeaderContentType) == "" {
		header.Set(echo.HeaderContentType, contentType)
	}
	c.response.WriteHeader(code)
	_, err := io.Copy(c.response, r)
	return err
}

var _ handler.Context = (*HandlerContext)(nil)

// NewHandlerContext creates a new context that satisfies [server.RequestContext]
// and allows echo style handlers to be used with AWS lambda.
func NewHandlerContext(w http.ResponseWriter, r *http.Request) *HandlerContext {
	return &HandlerContext{echo.NewResponse(w, nil), r}
}
