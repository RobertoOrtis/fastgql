package transport_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/RobertoOrtis/fastgql/client"
	"github.com/RobertoOrtis/fastgql/graphql"
	"github.com/RobertoOrtis/fastgql/graphql/handler"
	"github.com/RobertoOrtis/fastgql/graphql/handler/testserver"
	"github.com/RobertoOrtis/fastgql/graphql/handler/transport"
	"github.com/fasthttp/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

type ckey string

func TestWebsocket(t *testing.T) {
	handler := testserver.New()
	handler.AddTransport(transport.Websocket{})

	ln := startServerOnPort(t, 1234, handler.Handler())
	defer ln.Close()

	url := ln.Addr().String()

	t.Run("client must send valid json", func(t *testing.T) {
		c := wsConnect(url)
		defer c.Close()

		writeRaw(c, "hello")

		msg := readOp(c)
		assert.Equal(t, "connection_error", msg.Type)
		assert.Equal(t, `{"message":"invalid json"}`, string(msg.Payload))
	})

	t.Run("client can terminate before init", func(t *testing.T) {
		c := wsConnect(url)
		defer c.Close()

		require.NoError(t, c.WriteJSON(&operationMessage{Type: connectionTerminateMsg}))

		_, _, err := c.ReadMessage()
		assert.Equal(t, websocket.CloseNormalClosure, err.(*websocket.CloseError).Code)
	})

	t.Run("client must send init first", func(t *testing.T) {
		c := wsConnect(url)
		defer c.Close()

		require.NoError(t, c.WriteJSON(&operationMessage{Type: startMsg}))

		msg := readOp(c)
		assert.Equal(t, connectionErrorMsg, msg.Type)
		assert.Equal(t, `{"message":"unexpected message start"}`, string(msg.Payload))
	})

	t.Run("server acks init", func(t *testing.T) {
		c := wsConnect(url)
		defer c.Close()

		require.NoError(t, c.WriteJSON(&operationMessage{Type: connectionInitMsg}))

		assert.Equal(t, connectionAckMsg, readOp(c).Type)
		assert.Equal(t, connectionKeepAliveMsg, readOp(c).Type)
	})

	t.Run("client can terminate before run", func(t *testing.T) {
		c := wsConnect(url)
		defer c.Close()

		require.NoError(t, c.WriteJSON(&operationMessage{Type: connectionInitMsg}))
		assert.Equal(t, connectionAckMsg, readOp(c).Type)
		assert.Equal(t, connectionKeepAliveMsg, readOp(c).Type)

		require.NoError(t, c.WriteJSON(&operationMessage{Type: connectionTerminateMsg}))

		_, _, err := c.ReadMessage()
		assert.Equal(t, websocket.CloseNormalClosure, err.(*websocket.CloseError).Code)
	})

	t.Run("client gets parse errors", func(t *testing.T) {
		c := wsConnect(url)
		defer c.Close()

		require.NoError(t, c.WriteJSON(&operationMessage{Type: connectionInitMsg}))
		assert.Equal(t, connectionAckMsg, readOp(c).Type)
		assert.Equal(t, connectionKeepAliveMsg, readOp(c).Type)

		require.NoError(t, c.WriteJSON(&operationMessage{
			Type:    startMsg,
			ID:      "test_1",
			Payload: json.RawMessage(`{"query": "!"}`),
		}))

		msg := readOp(c)
		assert.Equal(t, errorMsg, msg.Type)
		assert.Equal(t, `[{"message":"Unexpected !","locations":[{"line":1,"column":1}],"extensions":{"code":"GRAPHQL_PARSE_FAILED"}}]`, string(msg.Payload))
	})

	t.Run("client can receive data", func(t *testing.T) {
		c := wsConnect(url)
		defer c.Close()

		require.NoError(t, c.WriteJSON(&operationMessage{Type: connectionInitMsg}))
		assert.Equal(t, connectionAckMsg, readOp(c).Type)
		assert.Equal(t, connectionKeepAliveMsg, readOp(c).Type)

		require.NoError(t, c.WriteJSON(&operationMessage{
			Type:    startMsg,
			ID:      "test_1",
			Payload: json.RawMessage(`{"query": "subscription { name }"}`),
		}))

		handler.SendNextSubscriptionMessage()
		msg := readOp(c)
		require.Equal(t, dataMsg, msg.Type, string(msg.Payload))
		require.Equal(t, "test_1", msg.ID, string(msg.Payload))
		require.Equal(t, `{"data":{"name":"test"}}`, string(msg.Payload))

		handler.SendNextSubscriptionMessage()
		msg = readOp(c)
		require.Equal(t, dataMsg, msg.Type, string(msg.Payload))
		require.Equal(t, "test_1", msg.ID, string(msg.Payload))
		require.Equal(t, `{"data":{"name":"test"}}`, string(msg.Payload))

		require.NoError(t, c.WriteJSON(&operationMessage{Type: stopMsg, ID: "test_1"}))

		msg = readOp(c)
		require.Equal(t, completeMsg, msg.Type)
		require.Equal(t, "test_1", msg.ID)
	})
}

func TestWebsocketWithKeepAlive(t *testing.T) {

	handler := testserver.New()
	handler.AddTransport(transport.Websocket{
		KeepAlivePingInterval: 100 * time.Millisecond,
	})

	ln := startServerOnPort(t, 1234, handler.Handler())
	defer ln.Close()

	url := ln.Addr().String()

	c := wsConnect(url)
	defer c.Close()

	require.NoError(t, c.WriteJSON(&operationMessage{Type: connectionInitMsg}))
	assert.Equal(t, connectionAckMsg, readOp(c).Type)
	assert.Equal(t, connectionKeepAliveMsg, readOp(c).Type)

	require.NoError(t, c.WriteJSON(&operationMessage{
		Type:    startMsg,
		ID:      "test_1",
		Payload: json.RawMessage(`{"query": "subscription { name }"}`),
	}))

	// keepalive
	msg := readOp(c)
	assert.Equal(t, connectionKeepAliveMsg, msg.Type)

	// server message
	handler.SendNextSubscriptionMessage()
	msg = readOp(c)
	assert.Equal(t, dataMsg, msg.Type)

	// keepalive
	msg = readOp(c)
	assert.Equal(t, connectionKeepAliveMsg, msg.Type)
}

func TestWebsocketInitFunc(t *testing.T) {
	t.Run("accept connection if WebsocketInitFunc is NOT provided", func(t *testing.T) {
		handler := testserver.New()
		handler.AddTransport(transport.Websocket{})

		ln := startServerOnPort(t, 1234, handler.Handler())
		defer ln.Close()

		url := ln.Addr().String()

		c := wsConnect(url)
		defer c.Close()

		require.NoError(t, c.WriteJSON(&operationMessage{Type: connectionInitMsg}))

		assert.Equal(t, connectionAckMsg, readOp(c).Type)
		assert.Equal(t, connectionKeepAliveMsg, readOp(c).Type)
	})

	t.Run("accept connection if WebsocketInitFunc is provided and is accepting connection", func(t *testing.T) {
		handler := testserver.New()
		handler.AddTransport(transport.Websocket{
			InitFunc: func(ctx *fasthttp.RequestCtx, initPayload transport.InitPayload) (*fasthttp.RequestCtx, error) {
				ctx.SetUserValue("newkey", "newvalue")
				return ctx, nil
			},
		})

		ln := startServerOnPort(t, 1234, handler.Handler())
		defer ln.Close()

		url := ln.Addr().String()

		c := wsConnect(url)
		defer c.Close()

		require.NoError(t, c.WriteJSON(&operationMessage{Type: connectionInitMsg}))

		assert.Equal(t, connectionAckMsg, readOp(c).Type)
		assert.Equal(t, connectionKeepAliveMsg, readOp(c).Type)
	})

	t.Run("reject connection if WebsocketInitFunc is provided and is accepting connection", func(t *testing.T) {
		handler := testserver.New()
		handler.AddTransport(transport.Websocket{
			InitFunc: func(ctx *fasthttp.RequestCtx, initPayload transport.InitPayload) (*fasthttp.RequestCtx, error) {
				return ctx, errors.New("invalid init payload")
			},
		})

		ln := startServerOnPort(t, 1234, handler.Handler())
		defer ln.Close()

		url := ln.Addr().String()

		c := wsConnect(url)
		defer c.Close()

		require.NoError(t, c.WriteJSON(&operationMessage{Type: connectionInitMsg}))

		msg := readOp(c)
		assert.Equal(t, connectionErrorMsg, msg.Type)
		assert.Equal(t, `{"message":"invalid init payload"}`, string(msg.Payload))
	})

	t.Run("can return context for request from WebsocketInitFunc", func(t *testing.T) {
		es := &graphql.ExecutableSchemaMock{
			ExecFunc: func(ctx context.Context) graphql.ResponseHandler {
				assert.Equal(t, "newvalue", ctx.Value(ckey("newkey")))
				return graphql.OneShot(&graphql.Response{Data: []byte(`{"empty":"ok"}`)})
			},
			SchemaFunc: func() *ast.Schema {
				return gqlparser.MustLoadSchema(&ast.Source{Input: `
				schema { query: Query }
				type Query {
					empty: String
				}
			`})
			},
		}
		h := handler.New(es)

		h.AddTransport(transport.Websocket{
			InitFunc: func(ctx *fasthttp.RequestCtx, initPayload transport.InitPayload) (*fasthttp.RequestCtx, error) {
				ctx.SetUserValue("newkey", "newvalue")
				return ctx, nil
			},
		})

		c := client.New(h.Handler())

		socket := c.Websocket("{ empty } ")
		defer socket.Close()
		var resp struct {
			Empty string
		}
		err := socket.Next(&resp)
		require.NoError(t, err)
		assert.Equal(t, "ok", resp.Empty)
	})
}

func startServerOnPort(t *testing.T, port int, h fasthttp.RequestHandler) net.Listener {
	ln, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		t.Fatalf("cannot start tcp server on port %d: %s", port, err)
	}
	go fasthttp.Serve(ln, h)
	return ln
}

func wsConnect(url string) *websocket.Conn {
	url = strings.Replace(url, "http://", "ws://", -1)
	if !strings.HasPrefix(url, "ws://") {
		url = "ws://" + url
	}

	c, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		panic(err)
	}
	_ = resp.Body.Close()

	return c
}

func writeRaw(conn *websocket.Conn, msg string) {
	if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
		panic(err)
	}
}

func readOp(conn *websocket.Conn) operationMessage {
	var msg operationMessage
	if err := conn.ReadJSON(&msg); err != nil {
		panic(err)
	}
	return msg
}

// copied out from weboscket.go to keep these private

const (
	connectionInitMsg      = "connection_init"      // Client -> Server
	connectionTerminateMsg = "connection_terminate" // Client -> Server
	startMsg               = "start"                // Client -> Server
	stopMsg                = "stop"                 // Client -> Server
	connectionAckMsg       = "connection_ack"       // Server -> Client
	connectionErrorMsg     = "connection_error"     // Server -> Client
	dataMsg                = "data"                 // Server -> Client
	errorMsg               = "error"                // Server -> Client
	completeMsg            = "complete"             // Server -> Client
	connectionKeepAliveMsg = "ka"                   // Server -> Client
)

type operationMessage struct {
	Payload json.RawMessage `json:"payload,omitempty"`
	ID      string          `json:"id,omitempty"`
	Type    string          `json:"type"`
}
