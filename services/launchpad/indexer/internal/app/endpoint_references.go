package app

import "log/slog"

func (d lifecycleRuntimeDocument) logEndpointReferences(logger *slog.Logger) {
	for endpoint, reference := range d.EndpointSecretReferences {
		logger.Info("committed endpoint reference", "endpoint_id", endpoint, "secret_reference", reference)
	}
}
