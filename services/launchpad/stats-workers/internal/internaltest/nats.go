package internaltest

import (
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

// ConnectNATS trusts an explicitly supplied fixture CA without changing the
// production broker client or disabling certificate and hostname verification.
func ConnectNATS(t *testing.T, brokerURL string) *nats.Conn {
	t.Helper()
	options := []nats.Option{nats.Timeout(5 * time.Second)}
	if ca := os.Getenv("STATS_TEST_NATS_CA_FILE"); ca != "" {
		options = append(options, nats.RootCAs(ca))
	}
	connection, err := nats.Connect(brokerURL, options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(connection.Close)
	return connection
}
