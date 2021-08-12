package transport

import (
	"bytes"
	"mime"

	"github.com/RobertoOrtis/fastgql/graphql"
	"github.com/valyala/fasthttp"
)

// POST implements the POST side of the default HTTP transport
// defined in https://github.com/APIs-guru/graphql-over-http#post
type POST struct{}

var _ graphql.Transport = POST{}

func (h POST) Supports(ctx *fasthttp.RequestCtx) bool {
	if string(ctx.Request.Header.Peek("Upgrade")) != "" {
		return false
	}

	mediaType, _, err := mime.ParseMediaType(string(ctx.Request.Header.ContentType()))
	if err != nil {
		return false
	}

	return string(ctx.Method()) == "POST" && mediaType == "application/json"
}

func (h POST) Do(ctx *fasthttp.RequestCtx, exec graphql.GraphExecutor) {
	ctx.Response.Header.SetContentType("application/json")

	var params *graphql.RawParams
	start := graphql.Now()
	if err := jsonDecode(bytes.NewReader(ctx.Request.Body()), &params); err != nil {
		ctx.Response.Header.SetStatusCode(fasthttp.StatusBadRequest)
		writeJsonErrorf(ctx, "json body could not be decoded: "+err.Error())
		return
	}
	params.ReadTime = graphql.TraceTiming{
		Start: start,
		End:   graphql.Now(),
	}

	rc, err := exec.CreateOperationContext(ctx, params)
	if err != nil {
		ctx.Response.Header.SetStatusCode(statusFor(err))
		resp := exec.DispatchError(graphql.WithOperationContext(ctx, rc), err)
		writeJson(ctx, resp)
		return
	}

	graphql.WithOperationContext(ctx, rc)
	responses, c := exec.DispatchOperation(ctx, rc)
	writeJson(ctx, responses(c))
}
