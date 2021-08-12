//go:generate go run ../../../testdata/gqlgen.go
package main

import (
	"log"
	"os"

	"github.com/RobertoOrtis/fastgql/example/federation/reviews/graph"
	"github.com/RobertoOrtis/fastgql/example/federation/reviews/graph/generated"
	"github.com/RobertoOrtis/fastgql/graphql/handler"
	"github.com/RobertoOrtis/fastgql/graphql/handler/debug"
	"github.com/RobertoOrtis/fastgql/graphql/playground"
	"github.com/valyala/fasthttp"
)

const defaultPort = "4003"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: &graph.Resolver{}}))
	srv.Use(&debug.Tracer{})

	playground := playground.Handler("GraphQL playground", "/query")
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

	log.Printf("connect to http://localhost:%s/ for GraphQL playground", port)
	log.Fatal(fasthttp.ListenAndServe(":"+port, h))
}
