package testserver

import (
	"context"
	"testing"

	"github.com/RobertoOrtis/fastgql/client"
	"github.com/RobertoOrtis/fastgql/graphql/handler"
	"github.com/stretchr/testify/require"
)

func TestTypeFallback(t *testing.T) {
	resolvers := &Stub{}

	c := client.New(handler.NewDefaultServer(NewExecutableSchema(Config{Resolvers: resolvers})).Handler())

	resolvers.QueryResolver.Fallback = func(ctx context.Context, arg FallbackToStringEncoding) (FallbackToStringEncoding, error) {
		return arg, nil
	}

	t.Run("fallback to string passthrough", func(t *testing.T) {
		var resp struct {
			Fallback string
		}
		c.MustPost(`query { fallback(arg: A) }`, &resp)
		require.Equal(t, "A", resp.Fallback)
	})
}
