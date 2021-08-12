package transport_test

import (
	"testing"

	"github.com/RobertoOrtis/fastgql/graphql/handler/testserver"
	"github.com/RobertoOrtis/fastgql/graphql/handler/transport"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
)

func TestGET(t *testing.T) {
	h := testserver.New()
	h.AddTransport(transport.GET{})

	t.Run("success", func(t *testing.T) {
		resp := doRequest(h.Handler(), "GET", "/graphql?query={name}", ``)
		assert.Equal(t, fasthttp.StatusOK, resp.StatusCode(), string(resp.Body()))
		assert.Equal(t, `{"data":{"name":"test"}}`, string(resp.Body()))
	})

	t.Run("has json content-type header", func(t *testing.T) {
		resp := doRequest(h.Handler(), "GET", "/graphql?query={name}", ``)
		assert.Equal(t, "application/json", string(resp.Header.ContentType()))
	})

	t.Run("decode failure", func(t *testing.T) {
		resp := doRequest(h.Handler(), "GET", "/graphql?query={name}&variables=notjson", "")
		assert.Equal(t, fasthttp.StatusBadRequest, resp.StatusCode(), string(resp.Body()))
		assert.Equal(t, `{"errors":[{"message":"variables could not be decoded"}],"data":null}`, string(resp.Body()))
	})

	t.Run("invalid variable", func(t *testing.T) {
		resp := doRequest(h.Handler(), "GET", `/graphql?query=query($id:Int!){find(id:$id)}&variables={"id":false}`, "")
		assert.Equal(t, fasthttp.StatusUnprocessableEntity, resp.StatusCode(), string(resp.Body()))
		assert.Equal(t, `{"errors":[{"message":"cannot use bool as Int","path":["variable","id"],"extensions":{"code":"GRAPHQL_VALIDATION_FAILED"}}],"data":null}`, string(resp.Body()))
	})

	t.Run("parse failure", func(t *testing.T) {
		resp := doRequest(h.Handler(), "GET", "/graphql?query=!", "")
		assert.Equal(t, fasthttp.StatusUnprocessableEntity, resp.StatusCode(), string(resp.Body()))
		assert.Equal(t, `{"errors":[{"message":"Unexpected !","locations":[{"line":1,"column":1}],"extensions":{"code":"GRAPHQL_PARSE_FAILED"}}],"data":null}`, string(resp.Body()))
	})

	t.Run("no mutations", func(t *testing.T) {
		resp := doRequest(h.Handler(), "GET", "/graphql?query=mutation{name}", "")
		assert.Equal(t, fasthttp.StatusNotAcceptable, resp.StatusCode(), string(resp.Body()))
		assert.Equal(t, `{"errors":[{"message":"GET requests only allow query operations"}],"data":null}`, string(resp.Body()))
	})

}
