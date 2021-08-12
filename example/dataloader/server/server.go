package main

import (
	"log"

	"github.com/RobertoOrtis/fastgql/example/dataloader"
	"github.com/RobertoOrtis/fastgql/graphql/handler"
	"github.com/RobertoOrtis/fastgql/graphql/playground"
	"github.com/valyala/fasthttp"
)

func main() {

	playground := playground.Handler("Dataloader", "/query")
	gqlHandler := handler.NewDefaultServer(dataloader.NewExecutableSchema(dataloader.Config{Resolvers: &dataloader.Resolver{}})).Handler()
	gqlHandler = dataloader.LoaderMiddleware(gqlHandler)

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

	log.Println("connect to http://localhost:8082/ for graphql playground")
	log.Fatal(fasthttp.ListenAndServe(":8082", h))
}
