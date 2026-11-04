package bringup

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func readTransaction(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("transaction file unreadable")
	}
	if strings.HasPrefix(strings.TrimSpace(string(raw)), "0x") {
		raw, err = hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(string(raw)), "0x"))
		if err != nil {
			return nil, errors.New("transaction hex malformed")
		}
	}
	if len(raw) == 0 || len(raw) > 1<<20 {
		return nil, errors.New("transaction size refused")
	}
	return raw, nil
}

func liveHTTPClient(ca string) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if ca != "" && ca != "-" {
		raw, err := os.ReadFile(ca)
		if err != nil {
			return nil, errors.New("configured CA unreadable")
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(raw) {
			return nil, errors.New("configured CA malformed")
		}
		transport.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	}
	return &http.Client{Transport: transport, Timeout: 30 * time.Second}, nil
}

func runLiveInspector(ctx context.Context, args []string, input io.Reader, output io.Writer) (bool, error) {
	recognized := map[string]bool{"regional-instances": true, "regional-readiness": true, "human-child": true, "indexer-state": true, "fresh-delivery": true, "kernel-node": true, "archive-delivery": true, "interop-payment": true, "search-payment": true, "developers-delivery": true}
	if !recognized[args[0]] {
		return false, nil
	}
	invalid := errors.New("invalid live evidence arguments")
	switch args[0] {
	case "regional-instances":
		if len(args) != 1 {
			return true, invalid
		}
		d, err := ReadRegionalDeployment(input)
		if err != nil {
			return true, err
		}
		for _, i := range d.Instances {
			if _, err := fmt.Fprintln(output, d.ID, i.ID); err != nil {
				return true, err
			}
		}
		return true, nil
	case "regional-readiness":
		if len(args) != 2 {
			return true, invalid
		}
		file, err := os.Open(args[1])
		if err != nil {
			return true, errors.New("deployment record unreadable")
		}
		defer file.Close()
		d, err := ReadRegionalDeployment(file)
		if err != nil {
			return true, err
		}
		return true, ValidateRegionalResponses(d, input, time.Now(), output)
	case "human-child":
		if len(args) != 2 {
			return true, invalid
		}
		status, err := strconv.Atoi(args[1])
		if err != nil {
			return true, invalid
		}
		return true, ValidateHumanChild(input, status, output)
	case "indexer-state":
		if len(args) != 1 {
			return true, invalid
		}
		return true, ValidateIndexerState(input, output)
	case "fresh-delivery":
		if len(args) != 6 {
			return true, invalid
		}
		events, err := os.Open(args[1])
		if err != nil {
			return true, errors.New("events unreadable")
		}
		defer events.Close()
		deliveries, err := os.Open(args[2])
		if err != nil {
			return true, errors.New("deliveries unreadable")
		}
		defer deliveries.Close()
		started, err := strconv.ParseUint(args[5], 10, 64)
		if err != nil {
			return true, invalid
		}
		return true, ValidateFreshDelivery(events, deliveries, args[3], args[4], started, uint64(time.Now().Unix()), output)
	case "kernel-node":
		if len(args) != 3 {
			return true, invalid
		}
		var process KernelProcess
		if json.NewDecoder(input).Decode(&process) != nil {
			return true, errors.New("process evidence unreadable")
		}
		client, err := liveHTTPClient("")
		if err != nil {
			return true, err
		}
		defer client.CloseIdleConnections()
		return true, CheckKernelNode(ctx, client, args[1], args[2], process, output)
	case "archive-delivery":
		if len(args) != 4 {
			return true, invalid
		}
		raw, err := readTransaction(args[2])
		if err != nil {
			return true, err
		}
		client, err := liveHTTPClient(args[3])
		if err != nil {
			return true, err
		}
		defer client.CloseIdleConnections()
		return true, CheckArchiveDelivery(ctx, client, args[1], raw, output)
	case "interop-payment":
		if len(args) != 6 {
			return true, invalid
		}
		raw, err := readTransaction(args[4])
		if err != nil {
			return true, err
		}
		token, err := os.ReadFile(args[5])
		if err != nil {
			return true, errors.New("interop key unreadable")
		}
		client, err := liveHTTPClient("")
		if err != nil {
			return true, err
		}
		defer client.CloseIdleConnections()
		return true, CheckInteropPayment(ctx, client, args[1], args[2], args[3], strings.TrimSpace(string(token)), raw, time.Now(), output)
	case "search-payment":
		if len(args) != 5 {
			return true, invalid
		}
		client, err := liveHTTPClient("")
		if err != nil {
			return true, err
		}
		defer client.CloseIdleConnections()
		return true, CheckSearchPayment(ctx, client, args[1], args[2], args[3], args[4], os.Getenv("CHECK_LIVE_GATEWAY_TOKEN"), output)
	case "developers-delivery":
		if len(args) != 8 {
			return true, invalid
		}
		body, err := os.ReadFile(args[4])
		if err != nil {
			return true, errors.New("source request unreadable")
		}
		token, err := os.ReadFile(args[5])
		if err != nil {
			return true, errors.New("webhook token unreadable")
		}
		sourceToken, err := os.ReadFile(args[6])
		if err != nil {
			return true, errors.New("source token unreadable")
		}
		attempts, err := strconv.Atoi(args[7])
		if err != nil {
			return true, invalid
		}
		client, err := liveHTTPClient("")
		if err != nil {
			return true, err
		}
		defer client.CloseIdleConnections()
		return true, CheckDevelopersDelivery(ctx, client, args[1], args[2], args[3], strings.TrimSpace(string(token)), strings.TrimSpace(string(sourceToken)), body, attempts, time.Now, output)
	}
	return true, invalid
}
