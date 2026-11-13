package proxy

import (
	"crypto/tls"
	"net/http"
	"strings"
	"testing"
)

func TestConnectedConstructorsRequireBoundedServiceRoutes(t *testing.T) {
	rest, socket, _ := connectedValidationHandlers(t)
	base := ConnectedDeps{Name: rest.audience, URL: rest.target.String(), Routes: rest.routes, Identity: rest.identity, SigningKey: rest.key, TLS: rest.client.Transport.(*http.Transport).TLSClientConfig}
	for _, tc := range []struct {
		name string
		edit func(*ConnectedDeps)
		want string
	}{
		{"unknown service", func(d *ConnectedDeps) { d.Name = "unknown" }, "unknown media upstream"},
		{"missing identity", func(d *ConnectedDeps) { d.Identity = nil }, "identity authority unavailable"},
		{"duplicate route", func(d *ConnectedDeps) { d.Routes = append(append([]ConnectedRoute(nil), d.Routes...), d.Routes[0]) }, "unknown media route"},
		{"empty catalog", func(d *ConnectedDeps) { d.Routes = nil }, "private media channel unavailable"},
		{"untrusted TLS", func(d *ConnectedDeps) { d.TLS = &tls.Config{MinVersion: tls.VersionTLS13} }, "private media channel unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := base
			tc.edit(&candidate)
			connected, err := NewConnected(candidate)
			if connected != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("connected service binding: connected=%v err=%v", connected, err)
			}
		})
	}
	routes := DevelopmentUserRoutes()
	if len(routes) != 1 || routes[0].Method != http.MethodPatch || routes[0].Template != v3ProfileRoute || !routes[0].Authenticate {
		t.Fatalf("development profile write inventory: %+v", routes)
	}
	if socket == nil {
		t.Fatal("bound private socket was unavailable")
	}
}
