package main

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
)

func TestOperatorCommandArguments(t *testing.T) {
	cases := [][]string{
		nil, {"unknown"}, {"show", "extra"}, {"enrol"}, {"revoke"}, {"revoke", "invalid"},
		{"reduce"}, {"raise"},
		changeArgs("raise", requesterActor, approverActor, "0", "1"),
		changeArgs("raise", requesterActor, approverActor, "-1", "1"),
		changeArgs("raise", requesterActor, approverActor, "1", "0"),
		changeArgs("raise", requesterActor, approverActor, "1", "25"),
		changeArgs("raise", requesterActor, approverActor, "1", "hours"),
		changeArgs("raise", requesterActor, "invalid", "1", "1"),
		changeArgs("reduce", requesterActor, "", "18446744073709551616", "1"),
		{"reduce", requesterActor, "unknown", "0", "1", "reduce capability"},
		{"reduce", requesterActor, "pool_creation", "0", "1", ""},
		{"reduce", requesterActor, "pool_creation", "0", "1", " padded"},
		{"reduce", requesterActor, "pool_creation", "0", "1", "line\nbreak"},
		{"reduce", requesterActor, "pool_creation", "0", "1", string([]byte{255})},
		{"reduce", requesterActor, "pool_creation", "0", "1", strings.Repeat("a", 1001)},
	}
	for _, args := range cases {
		var output bytes.Buffer
		if Run(args, nil, strings.NewReader(""), &output) == 0 || output.String() != "invalid command arguments\n" {
			t.Fatal("invalid arguments admitted or echoed")
		}
	}
	for _, args := range [][]string{
		{"show"}, {"revoke", requesterActor}, {"enrol", requesterActor, "operator-wallet", "infrastructure", "platform,pool:125"},
		changeArgs("raise", requesterActor, approverActor, "1", "1"), changeArgs("reduce", requesterActor, "", "0", "24"),
	} {
		if _, err := parse(args); err != nil {
			t.Fatal("valid arguments refused")
		}
	}
}

func TestOperatorCommandSettings(t *testing.T) {
	cases := []struct {
		args []string
		env  []string
		want string
	}{
		{[]string{"show"}, nil, "invalid OPERATOR_GATEWAY_URL"},
		{[]string{"enrol", requesterActor, "operator-wallet", "infrastructure", "platform"}, nil, "missing OPERATOR_REGISTRAR_DATABASE_URL"},
		{[]string{"revoke", requesterActor}, nil, "missing OPERATOR_REGISTRAR_DATABASE_URL"},
		{[]string{"revoke", requesterActor}, []string{registrarSetting + "=://invalid"}, "invalid OPERATOR_REGISTRAR_DATABASE_URL"},
		{[]string{"enrol", requesterActor, "operator-wallet", "infrastructure", "platform"}, []string{registrarSetting + "=postgres://"}, "invalid OPERATOR_FACTOR_KEY"},
		{changeArgs("reduce", requesterActor, "", "0", "1"), []string{gatewaySetting + "=https://localhost"}, "missing OPERATOR_DATABASE_URL"},
		{changeArgs("raise", requesterActor, approverActor, "1", "1"), []string{gatewaySetting + "=https://localhost", runtimeSetting + "=://invalid"}, "invalid OPERATOR_DATABASE_URL"},
	}
	for _, tc := range cases {
		var output bytes.Buffer
		if Run(tc.args, tc.env, strings.NewReader(""), &output) == 0 || strings.TrimSpace(output.String()) != tc.want {
			t.Fatal("setting validation differs:", output.String())
		}
	}
	for _, key := range []string{"", strings.Repeat("a", 63), strings.Repeat("a", 65), strings.Repeat("g", 64), strings.Repeat("é", 32)} {
		var output bytes.Buffer
		env := []string{gatewaySetting + "=https://localhost", runtimeSetting + "=postgres://", factorSetting + "=" + key}
		if Run(changeArgs("reduce", requesterActor, "", "0", "1"), env, strings.NewReader(""), &output) == 0 || output.String() != "invalid OPERATOR_FACTOR_KEY\n" {
			t.Fatal("invalid factor key admitted or echoed")
		}
	}
	values := settings([]string{"unassigned", "NAME=first", "NAME=second=value"})
	if values["NAME"] != "second=value" || len(values) != 1 {
		t.Fatal("environment parsing differs")
	}
}

func TestOperatorCommandGatewayAddress(t *testing.T) {
	for _, address := range []string{
		"https://localhost", "https://example.invalid:443", "http://localhost", "http://127.0.0.1", "http://127.25.1.3:8080/", "http://[::1]:8080",
	} {
		g, err := gateway(address)
		if err != nil {
			t.Fatal("secure or loopback gateway refused")
		}
		defer g.client.CloseIdleConnections()
		if g.client.CheckRedirect(nil, nil) != http.ErrUseLastResponse {
			t.Fatal("gateway redirects must be refused")
		}
	}
	for _, address := range []string{
		"", "/relative", "ftp://localhost", "http://example.invalid", "http://0.0.0.0", "http://[::]", "http://127.0.0.1.example.invalid", "http://localhost.example.invalid",
		"https://user:password@localhost", "https://localhost?token=private", "https://localhost?", "https://localhost/#private", "https://localhost/path", "https://localhost:invalid", "https://localhost:0", "https://localhost:65536", "https://localhost/%2f", "https://%zz",
	} {
		if _, err := gateway(address); err != errGateway {
			t.Fatal("invalid gateway admitted or address disclosed")
		}
	}
}

func TestOperatorCommandMissingCodes(t *testing.T) {
	f := operatorCommandDatabase(t)
	f.enrol(t, requesterActor, "infrastructure")
	for _, input := range []string{"", "12345\n", "12345x\n", strings.Repeat("1", 256) + "\n"} {
		status, output := f.run(t, changeArgs("reduce", requesterActor, "", "0", "1"), input)
		if status == 0 || (!strings.Contains(output, "one-time code required") && !strings.Contains(output, "invalid one-time code")) {
			t.Fatal("missing or malformed code admitted")
		}
	}
	if len(f.observed()) != 0 {
		t.Fatal("missing code must not reach gateway")
	}
}

func TestOperatorCommandUnusedSettings(t *testing.T) {
	f := operatorCommandDatabase(t)
	values := settings(f.environment)
	f.environment = []string{gatewaySetting + "=" + values[gatewaySetting]}
	status, output := f.run(t, []string{"show"}, "")
	if status != 0 || strings.Count(output, "\n") != 4 {
		t.Fatal("show required unused database or factor settings")
	}
}
