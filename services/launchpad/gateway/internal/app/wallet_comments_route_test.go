package app_test

import (
	"net/http"
	"os"
	"regexp"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/proxy"
)

func TestWalletCommentsReadReachesSocial(t *testing.T) {
	base, social, _, _ := boot(t)
	token, wallet := login(t, base)
	path := "/v2/social/users/" + tokenAddr + "/comments"

	for _, tc := range []struct {
		name, token, suppliedActor, wantActor string
	}{
		{name: "anonymous"},
		{name: "anonymous caller actor", suppliedActor: tokenAddr},
		{name: "session", token: token, wantActor: wallet},
		{name: "session caller actor", token: token, suppliedActor: tokenAddr, wantActor: wallet},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, base+path, nil)
			if err != nil {
				t.Fatal(err)
			}
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			if tc.suppliedActor != "" {
				req.Header.Set("X-Actor-Wallet", tc.suppliedActor)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("read wallet comments: %v", err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("wallet comments status = %d, want %d", resp.StatusCode, http.StatusOK)
			}
			if social.restPath != path {
				t.Errorf("social path = %q, want %q", social.restPath, path)
			}
			if social.restActor != tc.wantActor {
				t.Errorf("social actor = %q, want %q", social.restActor, tc.wantActor)
			}
		})
	}

	resp, err := http.Get(base + "/v2/social/users/" + tokenAddr + "/unlisted")
	if err != nil {
		t.Fatalf("read unlisted wallet path: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unlisted wallet path status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	if social.restPath != path {
		t.Errorf("unlisted wallet path reached social: %q", social.restPath)
	}
}

func TestSocialWalletReadsAreInTheGatewayTable(t *testing.T) {
	source, err := os.ReadFile("../../../social/internal/interactions/http.go")
	if err != nil {
		t.Fatalf("read social wallet route registrations: %v", err)
	}
	registrations := regexp.MustCompile(`\b\w+\.Get\s*\(\s*"(/users/\{wallet\}(?:/[^"]*)?)"`).FindAllSubmatch(source, -1)
	if len(registrations) == 0 {
		t.Fatal("social service registers no wallet GET routes")
	}
	routes := make(map[string]bool)
	for _, route := range proxy.SocialConnectedRoutes() {
		routes[route.Method+" "+route.Template] = true
	}
	for _, registration := range registrations {
		path := "/v2/social" + string(registration[1])
		if !routes[http.MethodGet+" "+path] {
			t.Errorf("gateway table is missing GET %s", path)
		}
	}
}
