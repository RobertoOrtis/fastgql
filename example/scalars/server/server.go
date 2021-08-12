package main

import (
	"log"

	"github.com/RobertoOrtis/fastgql/example/scalars"
	"github.com/RobertoOrtis/fastgql/graphql/handler"
	"github.com/RobertoOrtis/fastgql/graphql/playground"
	"github.com/valyala/fasthttp"
)

func main() {
	playground := playground.Handler("Starwars", "/query")
	gqlHandler := handler.NewDefaultServer(scalars.NewExecutableSchema(scalars.Config{Resolvers: &scalars.Resolver{}})).Handler()

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

	log.Fatal(fasthttp.ListenAndServe(":8084", h))
}
