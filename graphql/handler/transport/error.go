package transport

import (
	"encoding/json"
	"fmt"

	"github.com/RobertoOrtis/fastgql/graphql"
	"github.com/valyala/fasthttp"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// SendError sends a best effort error to a raw response writer. It assumes the client can understand the standard
// json error response
func SendError(ctx *fasthttp.RequestCtx, code int, errors ...*gqlerror.Error) {
	ctx.Response.SetStatusCode(code)
	b, err := json.Marshal(&graphql.Response{Errors: errors})
	if err != nil {
		panic(err)
	}
	ctx.Write(b)
}

// SendErrorf wraps SendError to add formatted messages
func SendErrorf(ctx *fasthttp.RequestCtx, code int, format string, args ...interface{}) {
	SendError(ctx, code, &gqlerror.Error{Message: fmt.Sprintf(format, args...)})
}
