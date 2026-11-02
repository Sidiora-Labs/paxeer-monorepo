package proxy

import (
	"fmt"
	"net/http"
	"reflect"
	"testing"
)

func TestAllowedConnectedRouteRefusesRetiredTemplates(t *testing.T) {
	want := []ConnectedRoute{
		{Method: http.MethodPost, Template: "/v2/assets/uploads"},
		{Method: http.MethodGet, Template: "/v2/assets/{assetId}"},
		{Method: http.MethodPost, Template: "/v2/assets/{assetId}:complete"},
		{Method: http.MethodGet, Template: "/v2/rooms/{roomId}/messages"},
		{Method: http.MethodPost, Template: "/v2/rooms/{roomId}/messages"},
	}
	routes := RetiredConnectedRoutes()
	if !reflect.DeepEqual(routes, want) {
		t.Fatalf("RetiredConnectedRoutes() = %#v, want %#v", routes, want)
	}
	for _, upstream := range []string{"metadata", "social"} {
		for _, route := range routes {
			for _, authenticate := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s %s/authenticate=%t", upstream, route.Method, route.Template, authenticate), func(t *testing.T) {
					candidate := route
					candidate.Authenticate = authenticate
					if allowedConnectedRoute(upstream, candidate) {
						t.Errorf("allowedConnectedRoute(%q, %#v) admitted a retired route", upstream, candidate)
					}
				})
			}
		}
	}
}

func TestDevelopmentSocialRoutesOmitRoomMessages(t *testing.T) {
	for _, route := range DevelopmentSocialRoutes() {
		if route.Template == "/v2/rooms/{roomId}/messages" {
			t.Errorf("DevelopmentSocialRoutes() includes retired route %#v", route)
		}
	}
}
