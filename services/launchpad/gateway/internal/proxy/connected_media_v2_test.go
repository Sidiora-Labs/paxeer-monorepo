package proxy

import (
	"net/http"
	"testing"
)

func TestSocialConnectedRoutesForwardFrozenVoiceTemplates(t *testing.T) {
	want := []ConnectedRoute{
		{Method: http.MethodPost, Template: "/v3/voice-rooms", Authenticate: true},
		{Method: http.MethodPost, Template: "/v3/voice-rooms/{roomId}/moderation-actions", Authenticate: true},
		{Method: http.MethodPost, Template: "/v3/voice-rooms/{roomId}/tickets", Authenticate: true},
		{Method: http.MethodGet, Template: "/v2/voice-rooms/{roomId}", Authenticate: true},
		{Method: http.MethodPost, Template: "/v2/voice-rooms/{roomId}:end", Authenticate: true},
		{Method: http.MethodPost, Template: "/v2/voice-rooms/{roomId}:leave", Authenticate: true},
	}
	routes := SocialConnectedRoutes()
	for _, expected := range want {
		t.Run(expected.Method+" "+expected.Template, func(t *testing.T) {
			found := 0
			for _, route := range routes {
				if route.Method != expected.Method || route.Template != expected.Template {
					continue
				}
				found++
				if route != expected {
					t.Errorf("voice route = %#v, want %#v", route, expected)
				}
				if !allowedConnectedRoute("social", route) {
					t.Errorf("social upstream rejected voice route %#v", route)
				}
			}
			if found != 1 {
				t.Errorf("found %d voice routes, want exactly one", found)
			}
		})
	}
}

func TestLivestreamConnectedRoutesForwardFrozenStreamTemplates(t *testing.T) {
	want := []ConnectedRoute{
		{Method: http.MethodGet, Template: "/v2/streams"},
		{Method: http.MethodPost, Template: "/v2/streams/{streamId}:start", Authenticate: true},
		{Method: http.MethodPost, Template: "/v2/streams/{streamId}:end", Authenticate: true},
		{Method: http.MethodPost, Template: "/v2/streams/{streamId}/viewers:heartbeat"},
		{Method: http.MethodPost, Template: "/v2/provider-webhooks/livepeer"},
	}
	routes := LivestreamConnectedRoutes()
	for _, expected := range want {
		t.Run(expected.Method+" "+expected.Template, func(t *testing.T) {
			found := 0
			for _, route := range routes {
				if route.Method != expected.Method || route.Template != expected.Template {
					continue
				}
				found++
				if route != expected {
					t.Errorf("stream route = %#v, want %#v", route, expected)
				}
				if !allowedConnectedRoute("livestream", route) {
					t.Errorf("livestream upstream rejected stream route %#v", route)
				}
			}
			if found != 1 {
				t.Errorf("found %d stream routes, want exactly one", found)
			}
		})
	}
}
