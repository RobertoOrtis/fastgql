package transport_test

import (
	"fmt"
	"testing"

	"github.com/RobertoOrtis/fastgql/graphql/handler/testserver"
	"github.com/RobertoOrtis/fastgql/graphql/handler/transport"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
)

func TestPOST(t *testing.T) {
	h := testserver.New()
	h.AddTransport(transport.POST{})

	t.Run("success", func(t *testing.T) {
		resp := doRequest(h.Handler(), "POST", "/graphql", `{"query":"{ name }"}`)
		assert.Equal(t, fasthttp.StatusOK, resp.StatusCode())
		assert.Equal(t, `{"data":{"name":"test"}}`, string(resp.Body()))
	})

	t.Run("decode failure", func(t *testing.T) {
		resp := doRequest(h.Handler(), "POST", "/graphql", "notjson")
		assert.Equal(t, fasthttp.StatusBadRequest, resp.StatusCode(), string(resp.Body()))
		assert.Equal(t, string(resp.Header.ContentType()), "application/json")
		assert.Equal(t, `{"errors":[{"message":"json body could not be decoded: invalid character 'o' in literal null (expecting 'u')"}],"data":null}`, string(resp.Body()))
	})

	t.Run("parse failure", func(t *testing.T) {
		resp := doRequest(h.Handler(), "POST", "/graphql", `{"query": "!"}`)
		assert.Equal(t, fasthttp.StatusUnprocessableEntity, resp.StatusCode(), string(resp.Body()))
		assert.Equal(t, string(resp.Header.ContentType()), "application/json")
		assert.Equal(t, `{"errors":[{"message":"Unexpected !","locations":[{"line":1,"column":1}],"extensions":{"code":"GRAPHQL_PARSE_FAILED"}}],"data":null}`, string(resp.Body()))
	})

	t.Run("validation failure", func(t *testing.T) {
		resp := doRequest(h.Handler(), "POST", "/graphql", `{"query": "{ title }"}`)
		assert.Equal(t, fasthttp.StatusUnprocessableEntity, resp.StatusCode(), string(resp.Body()))
		assert.Equal(t, string(resp.Header.ContentType()), "application/json")
		assert.Equal(t, `{"errors":[{"message":"Cannot query field \"title\" on type \"Query\".","locations":[{"line":1,"column":3}],"extensions":{"code":"GRAPHQL_VALIDATION_FAILED"}}],"data":null}`, string(resp.Body()))
	})

	t.Run("invalid variable", func(t *testing.T) {
		resp := doRequest(h.Handler(), "POST", "/graphql", `{"query": "query($id:Int!){find(id:$id)}","variables":{"id":false}}`)
		assert.Equal(t, fasthttp.StatusUnprocessableEntity, resp.StatusCode(), string(resp.Body()))
		assert.Equal(t, string(resp.Header.ContentType()), "application/json")
		assert.Equal(t, `{"errors":[{"message":"cannot use bool as Int","path":["variable","id"],"extensions":{"code":"GRAPHQL_VALIDATION_FAILED"}}],"data":null}`, string(resp.Body()))
	})

	t.Run("execution failure", func(t *testing.T) {
		resp := doRequest(h.Handler(), "POST", "/graphql", `{"query": "mutation { name }"}`)
		assert.Equal(t, fasthttp.StatusOK, resp.StatusCode(), string(resp.Body()))
		assert.Equal(t, string(resp.Header.ContentType()), "application/json")
		assert.Equal(t, `{"errors":[{"message":"mutations are not supported"}],"data":null}`, string(resp.Body()))
	})

	t.Run("validate content type", func(t *testing.T) {
		doReq := func(handler fasthttp.RequestHandler, method string, target string, body string, contentType string) *fasthttp.Response {
			req := fasthttp.AcquireRequest()
			defer fasthttp.ReleaseRequest(req)

			req.SetRequestURI(target)
			req.Header.SetMethod(method)
			req.SetBody([]byte(body))
			if contentType != "" {
				req.Header.SetContentType(contentType)
			}

			var fctx fasthttp.RequestCtx
			fctx.Init(req, nil, nil)

			handler(&fctx)

			return &fctx.Response
		}

		validContentTypes := []string{
			"application/json",
			"application/json; charset=utf-8",
		}

		for _, contentType := range validContentTypes {
			t.Run(fmt.Sprintf("allow for content type %s", contentType), func(t *testing.T) {
				resp := doReq(h.Handler(), "POST", "/graphql", `{"query":"{ name }"}`, contentType)
				assert.Equal(t, fasthttp.StatusOK, resp.StatusCode(), string(resp.Body()))
				assert.Equal(t, `{"data":{"name":"test"}}`, string(resp.Body()))
			})
		}

		invalidContentTypes := []string{
			"",
			"text/plain",

			// These content types are currently not supported, but they are supported by other GraphQL servers, like express-graphql.
			"application/x-www-form-urlencoded",
			"application/graphql",
		}

		for _, tc := range invalidContentTypes {
			t.Run(fmt.Sprintf("reject for content type %s", tc), func(t *testing.T) {
				resp := doReq(h.Handler(), "POST", "/graphql", `{"query":"{ name }"}`, tc)
				assert.Equal(t, fasthttp.StatusBadRequest, resp.StatusCode(), string(resp.Body()))
				assert.Equal(t, fmt.Sprintf(`{"errors":[{"message":"%s"}],"data":null}`, "transport not supported"), string(resp.Body()))
			})
		}
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
