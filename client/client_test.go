package client_test

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/RobertoOrtis/fastgql/client"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestClient(t *testing.T) {
	h := func(ctx *fasthttp.RequestCtx) {
		b := ctx.Request.Body()
		require.Equal(t, `{"query":"user(id:$id){name}","variables":{"id":1}}`, string(b))
		if err := json.NewEncoder(ctx).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"name": "bob",
			},
		}); err != nil {
			panic(err)
		}
	}

	c := client.New(h)

	var resp struct {
		Name string
	}

	c.MustPost("user(id:$id){name}", &resp, client.Var("id", 1))

	require.Equal(t, "bob", resp.Name)
}

func TestAddHeader(t *testing.T) {
	h := func(ctx *fasthttp.RequestCtx) {
		require.Equal(t, "ASDF", string(ctx.Request.Header.Peek("Test-Key")))
		ctx.WriteString(`{}`)
	}

	c := client.New(h)

	var resp struct{}
	c.MustPost("{ id }", &resp,
		client.AddHeader("Test-Key", "ASDF"),
	)
}

func TestAddClientHeader(t *testing.T) {
	h := func(ctx *fasthttp.RequestCtx) {
		require.Equal(t, "ASDF", string(ctx.Request.Header.Peek("Test-Key")))
		ctx.WriteString(`{}`)
	}

	c := client.New(h, client.AddHeader("Test-Key", "ASDF"))

	var resp struct{}
	c.MustPost("{ id }", &resp)
}

func TestBasicAuth(t *testing.T) {
	h := func(ctx *fasthttp.RequestCtx) {
		user, pass, ok := BasicAuth(ctx)
		require.True(t, ok)
		require.Equal(t, "user", user)
		require.Equal(t, "pass", pass)

		ctx.WriteString("{}")
	}

	c := client.New(h)

	var resp struct{}
	c.MustPost("{ id }", &resp,
		client.BasicAuth("user", "pass"),
	)
}

func TestAddCookie(t *testing.T) {
	h := func(ctx *fasthttp.RequestCtx) {
		c := ctx.Request.Header.Cookie("foo")
		require.NotNil(t, c)
		require.Equal(t, "value", string(c))

		ctx.WriteString("{}")
	}

	c := client.New(h)

	var resp struct{}
	c.MustPost("{ id }", &resp,
		client.AddCookie("foo", "value"),
	)
}

func BasicAuth(ctx *fasthttp.RequestCtx) (username, password string, ok bool) {
	auth := string(ctx.Request.Header.Peek("Authorization"))
	if auth == "" {
		return
	}
	return parseBasicAuth(auth)
}

func parseBasicAuth(auth string) (username, password string, ok bool) {
	const prefix = "Basic "
	// Case insensitive prefix match. See Issue 22736.
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return
	}
	c, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return
	}
	cs := string(c)
	s := strings.IndexByte(cs, ':')
	if s < 0 {
		return
	}
	return cs[:s], cs[s+1:], true
}
