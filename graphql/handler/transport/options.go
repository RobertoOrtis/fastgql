package transport

import (
	"net/http"

	"github.com/RobertoOrtis/fastgql/graphql"
	"github.com/valyala/fasthttp"
)

// Options responds to http OPTIONS and HEAD requests
type Options struct{}

var _ graphql.Transport = Options{}

func (o Options) Supports(ctx *fasthttp.RequestCtx) bool {
	method := string(ctx.Method())
	return method == "HEAD" || method == "OPTIONS"
}

func (o Options) Do(ctx *fasthttp.RequestCtx, exec graphql.GraphExecutor) {
	switch string(ctx.Method()) {
	case fasthttp.MethodOptions:
		ctx.Response.Header.SetStatusCode(fasthttp.StatusOK)
		ctx.Response.Header.Set("Allow", "OPTIONS, GET, POST")
	case http.MethodHead:
		ctx.Response.Header.SetStatusCode(fasthttp.StatusMethodNotAllowed)
	}
}
