package main

import (
	"context"
	"errors"
	"io/ioutil"
	"log"

	"github.com/RobertoOrtis/fastgql/graphql/handler/extension"
	"github.com/RobertoOrtis/fastgql/graphql/handler/transport"
	"github.com/valyala/fasthttp"

	"github.com/RobertoOrtis/fastgql/graphql/playground"

	"github.com/RobertoOrtis/fastgql/example/fileupload"
	"github.com/RobertoOrtis/fastgql/example/fileupload/model"
	"github.com/RobertoOrtis/fastgql/graphql"
	"github.com/RobertoOrtis/fastgql/graphql/handler"
)

func main() {
	resolver := getResolver()
	var mb int64 = 1 << 20

	srv := handler.NewDefaultServer(fileupload.NewExecutableSchema(fileupload.Config{Resolvers: resolver}))
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.MultipartForm{
		MaxMemory:     32 * mb,
		MaxUploadSize: 50 * mb,
	})
	srv.Use(extension.Introspection{})

	playground := playground.Handler("File Upload Demo", "/query")
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

	log.Print("connect to http://localhost:8087/ for GraphQL playground")
	log.Fatal(fasthttp.ListenAndServe(":8087", h))
}

func getResolver() *fileupload.Stub {
	resolver := &fileupload.Stub{}

	resolver.MutationResolver.SingleUpload = func(ctx context.Context, file graphql.Upload) (*model.File, error) {
		content, err := ioutil.ReadAll(file.File)
		if err != nil {
			return nil, err
		}
		return &model.File{
			ID:      1,
			Name:    file.Filename,
			Content: string(content),
		}, nil
	}
	resolver.MutationResolver.SingleUploadWithPayload = func(ctx context.Context, req model.UploadFile) (*model.File, error) {
		content, err := ioutil.ReadAll(req.File.File)
		if err != nil {
			return nil, err
		}
		return &model.File{
			ID:      1,
			Name:    req.File.Filename,
			Content: string(content),
		}, nil
	}
	resolver.MutationResolver.MultipleUpload = func(ctx context.Context, files []*graphql.Upload) ([]*model.File, error) {
		if len(files) == 0 {
			return nil, errors.New("empty list")
		}
		var resp []*model.File
		for i := range files {
			content, err := ioutil.ReadAll(files[i].File)
			if err != nil {
				return []*model.File{}, err
			}
			resp = append(resp, &model.File{
				ID:      i + 1,
				Name:    files[i].Filename,
				Content: string(content),
			})
		}
		return resp, nil
	}
	resolver.MutationResolver.MultipleUploadWithPayload = func(ctx context.Context, req []*model.UploadFile) ([]*model.File, error) {
		if len(req) == 0 {
			return nil, errors.New("empty list")
		}
		var resp []*model.File
		for i := range req {
			content, err := ioutil.ReadAll(req[i].File.File)
			if err != nil {
				return []*model.File{}, err
			}
			resp = append(resp, &model.File{
				ID:      i + 1,
				Name:    req[i].File.Filename,
				Content: string(content),
			})
		}
		return resp, nil
	}
	return resolver
}
