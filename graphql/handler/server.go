package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/RobertoOrtis/fastgql/graphql"
	"github.com/RobertoOrtis/fastgql/graphql/executor"
	"github.com/RobertoOrtis/fastgql/graphql/handler/extension"
	"github.com/RobertoOrtis/fastgql/graphql/handler/lru"
	"github.com/RobertoOrtis/fastgql/graphql/handler/transport"
	"github.com/valyala/fasthttp"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

type (
	Server struct {
		transports []graphql.Transport
		exec       *executor.Executor
	}
)

func New(es graphql.ExecutableSchema) *Server {
	return &Server{
		exec: executor.New(es),
	}
}

func NewDefaultServer(es graphql.ExecutableSchema) *Server {
	srv := New(es)

	srv.AddTransport(transport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
	})
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.MultipartForm{})

	srv.SetQueryCache(lru.New(1000))

	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New(100),
	})

	return srv
}

func (s *Server) AddTransport(transport graphql.Transport) {
	s.transports = append(s.transports, transport)
}

func (s *Server) SetErrorPresenter(f graphql.ErrorPresenterFunc) {
	s.exec.SetErrorPresenter(f)
}

func (s *Server) SetRecoverFunc(f graphql.RecoverFunc) {
	s.exec.SetRecoverFunc(f)
}

func (s *Server) SetQueryCache(cache graphql.Cache) {
	s.exec.SetQueryCache(cache)
}

func (s *Server) Use(extension graphql.HandlerExtension) {
	s.exec.Use(extension)
}

// AroundFields is a convenience method for creating an extension that only implements field middleware
func (s *Server) AroundFields(f graphql.FieldMiddleware) {
	s.exec.AroundFields(f)
}

// AroundOperations is a convenience method for creating an extension that only implements operation middleware
func (s *Server) AroundOperations(f graphql.OperationMiddleware) {
	s.exec.AroundOperations(f)
}

// AroundResponses is a convenience method for creating an extension that only implements response middleware
func (s *Server) AroundResponses(f graphql.ResponseMiddleware) {
	s.exec.AroundResponses(f)
}

func (s *Server) getTransport(ctx *fasthttp.RequestCtx) graphql.Transport {
	for _, t := range s.transports {
		if t.Supports(ctx) {
			return t
		}
	}
	return nil
}

func (s *Server) Handler() fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		defer func() {
			if err := recover(); err != nil {
				err := s.exec.PresentRecoveredError(ctx, err)
				resp := &graphql.Response{Errors: []*gqlerror.Error{err}}
				b, _ := json.Marshal(resp)
				ctx.Response.Header.SetStatusCode(fasthttp.StatusUnprocessableEntity)
				ctx.Write(b)
			}
		}()

		graphql.StartOperationTrace(ctx)

		transport := s.getTransport(ctx)
		if transport == nil {
			sendErrorf(ctx, http.StatusBadRequest, "transport not supported")
			return
		}

		transport.Do(ctx, s.exec)
	}
}

func sendError(ctx *fasthttp.RequestCtx, code int, errors ...*gqlerror.Error) {
	ctx.Response.SetStatusCode(code)
	b, err := json.Marshal(&graphql.Response{Errors: errors})
	if err != nil {
		panic(err)
	}
	ctx.Write(b)
}

func sendErrorf(ctx *fasthttp.RequestCtx, code int, format string, args ...interface{}) {
	sendError(ctx, code, &gqlerror.Error{Message: fmt.Sprintf(format, args...)})
}

type OperationFunc func(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler

func (r OperationFunc) ExtensionName() string {
	return "InlineOperationFunc"
}

func (r OperationFunc) Validate(schema graphql.ExecutableSchema) error {
	if r == nil {
		return fmt.Errorf("OperationFunc can not be nil")
	}
	return nil
}

func (r OperationFunc) InterceptOperation(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
	return r(ctx, next)
}

type ResponseFunc func(ctx context.Context, next graphql.ResponseHandler) *graphql.Response

func (r ResponseFunc) ExtensionName() string {
	return "InlineResponseFunc"
}

func (r ResponseFunc) Validate(schema graphql.ExecutableSchema) error {
	if r == nil {
		return fmt.Errorf("ResponseFunc can not be nil")
	}
	return nil
}

func (r ResponseFunc) InterceptResponse(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
	return r(ctx, next)
}

type FieldFunc func(ctx context.Context, next graphql.Resolver) (res interface{}, err error)

func (f FieldFunc) ExtensionName() string {
	return "InlineFieldFunc"
}

func (f FieldFunc) Validate(schema graphql.ExecutableSchema) error {
	if f == nil {
		return fmt.Errorf("FieldFunc can not be nil")
	}
	return nil
}

func (f FieldFunc) InterceptField(ctx context.Context, next graphql.Resolver) (res interface{}, err error) {
	return f(ctx, next)
}
