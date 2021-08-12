package transport_test

import (
	"testing"

	"github.com/RobertoOrtis/fastgql/graphql/handler/testserver"
	"github.com/RobertoOrtis/fastgql/graphql/handler/transport"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
)

func TestOptions(t *testing.T) {
	h := testserver.New()
	h.AddTransport(transport.Options{})

	t.Run("responds to options requests", func(t *testing.T) {
		resp := doRequest(h.Handler(), "OPTIONS", "/graphql?query={me{name}}", ``)
		assert.Equal(t, fasthttp.StatusOK, resp.StatusCode())
		assert.Equal(t, "OPTIONS, GET, POST", string(resp.Header.Peek("Allow")))
	})

	t.Run("responds to head requests", func(t *testing.T) {
		resp := doRequest(h.Handler(), "HEAD", "/graphql?query={me{name}}", ``)
		assert.Equal(t, fasthttp.StatusMethodNotAllowed, resp.StatusCode())
	})
}
