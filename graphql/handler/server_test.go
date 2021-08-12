package handler_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/RobertoOrtis/fastgql/graphql"
	"github.com/RobertoOrtis/fastgql/graphql/handler/testserver"
	"github.com/RobertoOrtis/fastgql/graphql/handler/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vektah/gqlparser/v2/parser"
)

func TestServer(t *testing.T) {
	srv := testserver.New()
	srv.AddTransport(&transport.GET{})

	h := srv.Handler()

	t.Run("returns an error if no transport matches", func(t *testing.T) {
		resp := post(h, "/foo", "application/json")
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode())
		assert.Equal(t, `{"errors":[{"message":"transport not supported"}],"data":null}`, string(resp.Body()))
	})

	t.Run("calls query on executable schema", func(t *testing.T) {
		resp := get(h, "/foo?query={name}")
		assert.Equal(t, http.StatusOK, resp.StatusCode())
		assert.Equal(t, `{"data":{"name":"test"}}`, string(resp.Body()))
	})

	t.Run("mutations are forbidden", func(t *testing.T) {
		resp := get(h, "/foo?query=mutation{name}")
		assert.Equal(t, http.StatusNotAcceptable, resp.StatusCode())
		assert.Equal(t, `{"errors":[{"message":"GET requests only allow query operations"}],"data":null}`, string(resp.Body()))
	})

	t.Run("subscriptions are forbidden", func(t *testing.T) {
		resp := get(h, "/foo?query=subscription{name}")
		assert.Equal(t, http.StatusNotAcceptable, resp.StatusCode())
		assert.Equal(t, `{"errors":[{"message":"GET requests only allow query operations"}],"data":null}`, string(resp.Body()))
	})

	t.Run("invokes operation middleware in order", func(t *testing.T) {
		var calls []string
		srv.AroundOperations(func(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
			calls = append(calls, "first")
			return next(ctx)
		})
		srv.AroundOperations(func(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
			calls = append(calls, "second")
			return next(ctx)
		})

		resp := get(h, "/foo?query={name}")
		assert.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body()))
		assert.Equal(t, []string{"first", "second"}, calls)
	})

	t.Run("invokes response middleware in order", func(t *testing.T) {
		var calls []string
		srv.AroundResponses(func(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
			calls = append(calls, "first")
			return next(ctx)
		})
		srv.AroundResponses(func(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
			calls = append(calls, "second")
			return next(ctx)
		})

		resp := get(h, "/foo?query={name}")
		assert.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body()))
		assert.Equal(t, []string{"first", "second"}, calls)
	})

	t.Run("invokes field middleware in order", func(t *testing.T) {
		var calls []string
		srv.AroundFields(func(ctx context.Context, next graphql.Resolver) (res interface{}, err error) {
			calls = append(calls, "first")
			return next(ctx)
		})
		srv.AroundFields(func(ctx context.Context, next graphql.Resolver) (res interface{}, err error) {
			calls = append(calls, "second")
			return next(ctx)
		})

		resp := get(h, "/foo?query={name}")
		assert.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body()))
		assert.Equal(t, []string{"first", "second"}, calls)
	})

	t.Run("get query parse error in AroundResponses", func(t *testing.T) {
		var errors1 gqlerror.List
		var errors2 gqlerror.List
		srv.AroundResponses(func(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
			resp := next(ctx)
			errors1 = graphql.GetErrors(ctx)
			errors2 = resp.Errors
			return resp
		})

		resp := get(h, "/foo?query=invalid")
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode(), string(resp.Body()))
		assert.Equal(t, 1, len(errors1))
		assert.Equal(t, 1, len(errors2))
	})

	t.Run("query caching", func(t *testing.T) {
		ctx := context.Background()
		cache := &graphql.MapCache{}
		srv.SetQueryCache(cache)
		qry := `query Foo {name}`

		t.Run("cache miss populates cache", func(t *testing.T) {
			resp := get(h, "/foo?query="+url.QueryEscape(qry))
			assert.Equal(t, http.StatusOK, resp.StatusCode())
			assert.Equal(t, `{"data":{"name":"test"}}`, string(resp.Body()))

			cacheDoc, ok := cache.Get(ctx, qry)
			require.True(t, ok)
			require.Equal(t, "Foo", cacheDoc.(*ast.QueryDocument).Operations[0].Name)
		})

		t.Run("cache hits use document from cache", func(t *testing.T) {
			doc, err := parser.ParseQuery(&ast.Source{Input: `query Bar {name}`})
			require.Nil(t, err)
			cache.Add(ctx, qry, doc)

			resp := get(h, "/foo?query="+url.QueryEscape(qry))
			assert.Equal(t, http.StatusOK, resp.StatusCode())
			assert.Equal(t, `{"data":{"name":"test"}}`, string(resp.Body()))

			cacheDoc, ok := cache.Get(ctx, qry)
			require.True(t, ok)
			require.Equal(t, "Bar", cacheDoc.(*ast.QueryDocument).Operations[0].Name)
		})
	})
}

func TestErrorServer(t *testing.T) {
	srv := testserver.NewError()
	srv.AddTransport(&transport.GET{})

	h := srv.Handler()

	t.Run("get resolver error in AroundResponses", func(t *testing.T) {
		var errors1 gqlerror.List
		var errors2 gqlerror.List
		srv.AroundResponses(func(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
			resp := next(ctx)
			errors1 = graphql.GetErrors(ctx)
			errors2 = resp.Errors
			return resp
		})

		resp := get(h, "/foo?query={name}")
		assert.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body()))
		assert.Equal(t, 1, len(errors1))
		assert.Equal(t, 1, len(errors2))
	})
}

func get(handler fasthttp.RequestHandler, target string) *fasthttp.Response {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	req.SetRequestURI(target)
	req.Header.SetMethod("GET")

	var fctx fasthttp.RequestCtx
	fctx.Init(req, nil, nil)

	handler(&fctx)

	return &fctx.Response
}

func post(handler fasthttp.RequestHandler, target, contentType string) *fasthttp.Response {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	req.SetRequestURI(target)
	req.Header.SetMethod("POST")
	req.Header.SetContentType(contentType)

	var fctx fasthttp.RequestCtx
	fctx.Init(req, nil, nil)

	handler(&fctx)

	return &fctx.Response
}
