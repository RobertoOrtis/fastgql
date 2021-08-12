package main

import (
	"context"
	"errors"
	"log"
	"os"

	"github.com/valyala/fasthttp"

	"github.com/RobertoOrtis/fastgql/graphql"
	"github.com/RobertoOrtis/fastgql/graphql/handler"
	"github.com/RobertoOrtis/fastgql/graphql/handler/extension"
	"github.com/RobertoOrtis/fastgql/graphql/playground"
	"github.com/RobertoOrtis/fastgql/integration"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

const defaultPort = "8080"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	cfg := integration.Config{Resolvers: &integration.Resolver{}}
	cfg.Complexity.Query.Complexity = func(childComplexity, value int) int {
		// Allow the integration client to dictate the complexity, to verify this
		// function is executed.
		return value
	}

	srv := handler.NewDefaultServer(integration.NewExecutableSchema(cfg))
	srv.SetErrorPresenter(func(ctx context.Context, e error) *gqlerror.Error {
		var ie *integration.CustomError
		if errors.As(e, &ie) {
			return &gqlerror.Error{
				Message: ie.UserMessage,
				Path:    graphql.GetPath(ctx),
			}
		}
		return graphql.DefaultErrorPresenter(ctx, e)
	})
	srv.Use(extension.FixedComplexityLimit(1000))

	serverHandler := srv.Handler()
	playgroundHandler := playground.Handler("GraphQL playground", "/query")

	requestHandler := func(ctx *fasthttp.RequestCtx) {
		switch string(ctx.Path()) {
		case "/":
			playgroundHandler(ctx)
		case "/query":
			serverHandler(ctx)
		default:
			ctx.Error("Unsupported path", fasthttp.StatusNotFound)
		}
	}

	log.Printf("connect to http://localhost:%s/ for GraphQL playground", port)
	log.Fatal(fasthttp.ListenAndServe(":"+port, requestHandler))
}
