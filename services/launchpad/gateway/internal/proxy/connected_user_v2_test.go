package proxy

import (
	"net/http"
	"testing"
)

func TestUserConnectedRoutesForwardFrozenV2Owner(t *testing.T) {
	want := []ConnectedRoute{
		{Method: http.MethodGet, Template: "/v2/me/watchlist", Authenticate: true},
		{Method: http.MethodPut, Template: "/v2/me/watchlist/{poolAddress}", Authenticate: true},
		{Method: http.MethodDelete, Template: "/v2/me/watchlist/{poolAddress}", Authenticate: true},
		{Method: http.MethodPatch, Template: "/v2/profiles/{wallet}", Authenticate: true},
		{Method: http.MethodPut, Template: "/v2/me/devices/{deviceId}", Authenticate: true},
		{Method: http.MethodDelete, Template: "/v2/me/devices/{deviceId}", Authenticate: true},
		{Method: http.MethodPost, Template: "/v2/me/exports", Authenticate: true},
		{Method: http.MethodGet, Template: "/v2/me/exports/{jobId}", Authenticate: true},
		{Method: http.MethodPost, Template: "/v2/me/deletion-requests", Authenticate: true},
		{Method: http.MethodGet, Template: "/v2/me/deletion-requests/{jobId}", Authenticate: true},
	}
	routes := UserConnectedRoutes()
	for _, expected := range want {
		t.Run(expected.Method+" "+expected.Template, func(t *testing.T) {
			found := 0
			for _, route := range routes {
				if route.Method != expected.Method || route.Template != expected.Template {
					continue
				}
				found++
				if route != expected {
					t.Errorf("owner route = %#v, want %#v", route, expected)
				}
				if !allowedConnectedRoute("user", route) {
					t.Errorf("user upstream rejected owner route %#v", route)
				}
			}
			if found != 1 {
				t.Errorf("found %d owner routes, want exactly one", found)
			}
		})
	}
}

func TestUserConnectedRoutesKeepV2ProfileReadPublic(t *testing.T) {
	routes := UserConnectedRoutes()
	for _, expected := range []ConnectedRoute{
		{Method: http.MethodGet, Template: "/v2/profiles/{wallet}"},
		{Method: http.MethodPatch, Template: "/v2/profiles/{wallet}", Authenticate: true},
	} {
		t.Run(expected.Method, func(t *testing.T) {
			found := 0
			for _, route := range routes {
				if route.Method != expected.Method || route.Template != expected.Template {
					continue
				}
				found++
				if route != expected {
					t.Errorf("profile route = %#v, want %#v", route, expected)
				}
			}
			if found != 1 {
				t.Errorf("found %d profile routes, want exactly one", found)
			}
		})
	}
}
