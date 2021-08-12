package main

import (
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/RobertoOrtis/fastgql/graphql/handler/extension"
	"github.com/RobertoOrtis/fastgql/graphql/handler/transport"
	"github.com/valyala/fasthttp"

	"github.com/RobertoOrtis/fastgql/graphql/playground"

	"github.com/RobertoOrtis/fastgql/example/chat"
	"github.com/RobertoOrtis/fastgql/graphql/handler"
	"github.com/fasthttp/websocket"
	"github.com/opentracing/opentracing-go"
	"sourcegraph.com/sourcegraph/appdash"
	appdashtracer "sourcegraph.com/sourcegraph/appdash/opentracing"
	"sourcegraph.com/sourcegraph/appdash/traceapp"
)

func main() {
	startAppdashServer()

	// c := cors.New(cors.Options{
	// 	AllowedOrigins:   []string{"http://localhost:3000"},
	// 	AllowCredentials: true,
	// })

	srv := handler.New(chat.NewExecutableSchema(chat.New()))
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.Websocket{
		KeepAlivePingInterval: 2 * time.Second,
		Upgrader: websocket.FastHTTPUpgrader{
			CheckOrigin: func(ctx *fasthttp.RequestCtx) bool {
				return true
			},
		},
	})
	srv.Use(extension.Introspection{})

	playground := playground.Handler("Todo", "/query")
	gqlHandler := srv.Handler()

	h := func(ctx *fasthttp.RequestCtx) {
		switch string(ctx.Path()) {
		case "/query":
			gqlHandler(ctx)
		case "/":
			playground(ctx)
		default:
			ctx.Error("not found", fasthttp.StatusNotFound)
		}
	}

	log.Fatal(fasthttp.ListenAndServe(":8081", h))
}

func startAppdashServer() opentracing.Tracer {
	memStore := appdash.NewMemoryStore()
	store := &appdash.RecentStore{
		MinEvictAge: 5 * time.Minute,
		DeleteStore: memStore,
	}

	url, err := url.Parse("http://localhost:8700")
	if err != nil {
		log.Fatal(err)
	}
	tapp, err := traceapp.New(nil, url)
	if err != nil {
		log.Fatal(err)
	}
	tapp.Store = store
	tapp.Queryer = memStore

	go func() {
		log.Fatal(http.ListenAndServe(":8700", tapp))
	}()
	tapp.Store = store
	tapp.Queryer = memStore

	collector := appdash.NewLocalCollector(store)
	tracer := appdashtracer.NewTracer(collector)
	opentracing.InitGlobalTracer(tracer)

	log.Println("Appdash web UI running on HTTP :8700")
	return tracer
}
