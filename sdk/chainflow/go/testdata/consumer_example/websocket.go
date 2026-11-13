package chainflowexample

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"time"

	sdk "chainflow-data-gateway"
	"chainflow-data-gateway/option"
	"github.com/gorilla/websocket"
)

func ConnectBTC(ctx context.Context, client *sdk.Client, mintKey, endpoint string, dialer *websocket.Dialer, output io.Writer) (*websocket.Conn, error) {
	credential, err := client.Gateway.NewWebSocketToken(
		ctx,
		sdk.GatewayNewWebSocketTokenParams{Symbol: sdk.F(sdk.WsScopeBtc)},
		option.WithWsMintKey(mintKey),
		option.WithQueryDel("token"),
	)
	if err != nil {
		return nil, errors.New("WebSocket token mint failed")
	}

	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "wss" || u.Host == "" || u.Path != "/ws/btc" {
		return nil, errors.New("expected a trusted wss endpoint for /ws/btc")
	}
	query := u.Query()
	query.Set("token", credential.Token)
	u.RawQuery = query.Encode()
	conn, response, err := dialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		if response != nil {
			response.Body.Close()
		}
		// Dial errors can contain the token-bearing URL; return only a safe message.
		return nil, errors.New("WebSocket handshake failed")
	}
	if _, err := fmt.Fprintf(output, "scope=%s expires_at=%s expires_in=%d\n", credential.Symbol, credential.ExpiresAt.UTC().Format(time.RFC3339), credential.ExpiresIn); err != nil {
		conn.Close()
		return nil, errors.New("WebSocket metadata output failed")
	}
	return conn, nil
}
