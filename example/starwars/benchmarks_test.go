package starwars

import (
	"strings"
	"testing"

	"github.com/RobertoOrtis/fastgql/example/starwars/generated"
	"github.com/RobertoOrtis/fastgql/graphql/handler"
	"github.com/valyala/fasthttp"
)

func BenchmarkSimpleQueryNoArgs(b *testing.B) {
	handler := handler.NewDefaultServer(generated.NewExecutableSchema(NewResolver())).Handler()
	q := `{"query":"{ search(text:\"Luke\") { ... on Human { starships { name } } } }"}`

	var body strings.Reader

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	req.Header.SetMethod("POST")
	req.SetRequestURI("/graphql")
	req.SetBodyStream(&body, body.Len())
	req.Header.SetContentType("application/json")

	b.ReportAllocs()
	b.ResetTimer()

	var fctx fasthttp.RequestCtx
	fctx.Init(req, nil, nil)

	for i := 0; i < b.N; i++ {
		body.Reset(q)
		fctx.Response.Reset()
		handler(&fctx)
		if string(fctx.Response.Body()) != `{"data":{"search":[{"starships":[{"name":"X-Wing"},{"name":"Imperial shuttle"}]}]}}` {
			b.Fatalf("Unexpected response: %s", string(fctx.Response.Body()))
		}
	}
}
