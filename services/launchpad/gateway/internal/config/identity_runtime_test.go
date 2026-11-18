package config

import (
	"net/url"
	"testing"
)

func TestIdentityRuntimeURLSelectsRegisteredSchema(t *testing.T) {
	const raw = "postgresql://identity_runtime:password%2Fvalue@localhost:5432/launchpad?sslmode=verify-full&application_name=gateway&search_path=public"
	bound, err := identityRuntimeURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	endpoint, err := url.Parse(bound)
	if err != nil {
		t.Fatal(err)
	}
	password, present := endpoint.User.Password()
	if endpoint.User.Username() != "identity_runtime" || !present || password != "password/value" || endpoint.Host != "localhost:5432" || endpoint.Path != "/launchpad" {
		t.Fatal("identity connection authority changed")
	}
	if endpoint.Query().Get("search_path") != "users,pg_catalog" || endpoint.Query().Get("sslmode") != "verify-full" || endpoint.Query().Get("application_name") != "gateway" {
		t.Fatal("identity connection must retain transport options and select users")
	}
	for _, invalid := range []string{"", "postgres://", "https://localhost/launchpad", "postgres://%zz"} {
		if _, err := identityRuntimeURL(invalid); err == nil {
			t.Fatal("invalid identity connection admitted")
		}
	}
}
