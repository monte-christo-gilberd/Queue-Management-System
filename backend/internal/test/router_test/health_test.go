package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"qms/internal/router"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type stubSSE struct{}

func (stubSSE) ServeSSE(c *gin.Context) {}

func TestSpec_Health_ReturnsOKWithoutAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app := router.NewAppRouter(struct{ router.TicketHandler }{}, struct{ router.UserHandler }{}, struct{ router.QueueHandler }{},
		func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) },
		stubSSE{}, nil)
	engine := router.SetupRouter(app)

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"status":"ok"}`, w.Body.String())
}
