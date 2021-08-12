package extension_test

import (
	"context"
	"testing"

	"github.com/RobertoOrtis/fastgql/graphql"
	"github.com/RobertoOrtis/fastgql/graphql/handler/extension"
	"github.com/RobertoOrtis/fastgql/graphql/handler/testserver"
	"github.com/RobertoOrtis/fastgql/graphql/handler/transport"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestHandlerComplexity(t *testing.T) {
	h := testserver.New()
	h.Use(&extension.ComplexityLimit{
		Func: func(ctx context.Context, rc *graphql.OperationContext) int {
			if rc.RawQuery == "{ ok: name }" {
				return 4
			}
			return 2
		},
	})
	h.AddTransport(&transport.POST{})
	var stats *extension.ComplexityStats
	h.AroundResponses(func(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
		stats = extension.GetComplexityStats(ctx)
		return next(ctx)
	})

	t.Run("below complexity limit", func(t *testing.T) {
		stats = nil
		h.SetCalculatedComplexity(2)
		resp := doRequest(h.Handler(), "POST", "/graphql", `{"query":"{ name }"}`)
		require.Equal(t, fasthttp.StatusOK, resp.StatusCode(), string(resp.Body()))
		require.Equal(t, `{"data":{"name":"test"}}`, string(resp.Body()))

		require.Equal(t, 2, stats.ComplexityLimit)
		require.Equal(t, 2, stats.Complexity)
	})

	t.Run("above complexity limit", func(t *testing.T) {
		stats = nil
		h.SetCalculatedComplexity(4)
		resp := doRequest(h.Handler(), "POST", "/graphql", `{"query":"{ name }"}`)
		require.Equal(t, fasthttp.StatusOK, resp.StatusCode(), string(resp.Body()))
		require.Equal(t, `{"errors":[{"message":"operation has complexity 4, which exceeds the limit of 2","extensions":{"code":"COMPLEXITY_LIMIT_EXCEEDED"}}],"data":null}`, string(resp.Body()))

		require.Equal(t, 2, stats.ComplexityLimit)
		require.Equal(t, 4, stats.Complexity)
	})

	t.Run("within dynamic complexity limit", func(t *testing.T) {
		stats = nil
		h.SetCalculatedComplexity(4)
		resp := doRequest(h.Handler(), "POST", "/graphql", `{"query":"{ ok: name }"}`)
		require.Equal(t, fasthttp.StatusOK, resp.StatusCode(), string(resp.Body()))
		require.Equal(t, `{"data":{"name":"test"}}`, string(resp.Body()))

		require.Equal(t, 4, stats.ComplexityLimit)
		require.Equal(t, 4, stats.Complexity)
	})
}

func TestFixedComplexity(t *testing.T) {
	h := testserver.New()
	h.Use(extension.FixedComplexityLimit(2))
	h.AddTransport(&transport.POST{})

	var stats *extension.ComplexityStats
	h.AroundResponses(func(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
		stats = extension.GetComplexityStats(ctx)
		return next(ctx)
	})

	t.Run("below complexity limit", func(t *testing.T) {
		h.SetCalculatedComplexity(2)
		resp := doRequest(h.Handler(), "POST", "/graphql", `{"query":"{ name }"}`)
		require.Equal(t, fasthttp.StatusOK, resp.StatusCode(), string(resp.Body()))
		require.Equal(t, `{"data":{"name":"test"}}`, string(resp.Body()))

		require.Equal(t, 2, stats.ComplexityLimit)
		require.Equal(t, 2, stats.Complexity)
	})

	t.Run("above complexity limit", func(t *testing.T) {
		h.SetCalculatedComplexity(4)
		resp := doRequest(h.Handler(), "POST", "/graphql", `{"query":"{ name }"}`)
		require.Equal(t, fasthttp.StatusOK, resp.StatusCode(), string(resp.Body()))
		require.Equal(t, `{"errors":[{"message":"operation has complexity 4, which exceeds the limit of 2","extensions":{"code":"COMPLEXITY_LIMIT_EXCEEDED"}}],"data":null}`, string(resp.Body()))

		require.Equal(t, 2, stats.ComplexityLimit)
		require.Equal(t, 4, stats.Complexity)
	})
}

func doRequest(handler fasthttp.RequestHandler, method string, target string, body string) *fasthttp.Response {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	req.SetRequestURI(target)
	req.Header.SetMethod(method)
	req.Header.SetContentType("application/json")
	req.SetBody([]byte(body))

	var fctx fasthttp.RequestCtx
	fctx.Init(req, nil, nil)

	handler(&fctx)

	return &fctx.Response
}
