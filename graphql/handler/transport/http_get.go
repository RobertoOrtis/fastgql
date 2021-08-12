package transport

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/RobertoOrtis/fastgql/graphql"
	"github.com/RobertoOrtis/fastgql/graphql/errcode"
	"github.com/valyala/fasthttp"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// GET implements the GET side of the default HTTP transport
// defined in https://github.com/APIs-guru/graphql-over-http#get
type GET struct{}

var _ graphql.Transport = GET{}

func (h GET) Supports(ctx *fasthttp.RequestCtx) bool {
	if string(ctx.Request.Header.Peek("Upgrade")) != "" {
		return false
	}

	return string(ctx.Method()) == fasthttp.MethodGet
}

func (h GET) Do(ctx *fasthttp.RequestCtx, exec graphql.GraphExecutor) {
	ctx.Response.Header.SetContentType("application/json")

	raw := &graphql.RawParams{
		Query:         string(ctx.QueryArgs().Peek("query")),
		OperationName: string(ctx.QueryArgs().Peek("operationName")),
	}
	raw.ReadTime.Start = graphql.Now()

	if variables := string(ctx.QueryArgs().Peek("variables")); variables != "" {
		if err := jsonDecode(strings.NewReader(variables), &raw.Variables); err != nil {
			ctx.Response.Header.SetStatusCode(fasthttp.StatusBadRequest)
			writeJsonError(ctx, "variables could not be decoded")
			return
		}
	}

	if extensions := string(ctx.QueryArgs().Peek("extensions")); extensions != "" {
		if err := jsonDecode(strings.NewReader(extensions), &raw.Extensions); err != nil {
			ctx.Response.Header.SetStatusCode(fasthttp.StatusBadRequest)
			writeJsonError(ctx, "extensions could not be decoded")
			return
		}
	}

	raw.ReadTime.End = graphql.Now()

	rc, err := exec.CreateOperationContext(ctx, raw)
	if err != nil {
		ctx.Response.Header.SetStatusCode(statusFor(err))
		resp := exec.DispatchError(graphql.WithOperationContext(ctx, rc), err)
		writeJson(ctx, resp)
		return
	}
	op := rc.Doc.Operations.ForName(rc.OperationName)
	if op.Operation != ast.Query {
		ctx.Response.Header.SetStatusCode(fasthttp.StatusNotAcceptable)
		writeJsonError(ctx, "GET requests only allow query operations")
		return
	}

	responses, c := exec.DispatchOperation(ctx, rc)
	writeJson(ctx, responses(c))
}

func jsonDecode(r io.Reader, val interface{}) error {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	return dec.Decode(val)
}

func statusFor(errs gqlerror.List) int {
	switch errcode.GetErrorKind(errs) {
	case errcode.KindProtocol:
		return fasthttp.StatusUnprocessableEntity
	default:
		return fasthttp.StatusOK
	}
}
