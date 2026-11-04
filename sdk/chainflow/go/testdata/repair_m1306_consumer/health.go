package chainflowexample

import (
	"context"
	"net/http"

	sdk "chainflow-data-gateway"
	"chainflow-data-gateway/option"
)

func CheckHealth(ctx context.Context, transport *http.Client, baseURL, restKey string) (string, error) {
	client := sdk.NewClient(
		option.WithBaseURL(baseURL),
		option.WithHTTPClient(transport),
		option.WithMaxRetries(0),
		option.WithRestAPIKey(restKey),
		option.WithQueryDel("token"),
	)
	health, err := client.Gateway.ListHealth(ctx)
	if err != nil {
		return "", err
	}
	return *health, nil
}
