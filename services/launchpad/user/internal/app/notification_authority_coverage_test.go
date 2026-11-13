package app

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/account"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/google/uuid"
)

func TestNotificationAuthorityTargetLifecycle(t *testing.T) {
	f := repairM1062New(t)
	pki := testpki.New(t)
	serverLeaf := pki.Leaf("user", "user.test")
	clientLeaf := pki.Leaf("notifications", "notifications.test")
	runtime := &account.Runtime{EndpointKey: bytes.Repeat([]byte{62}, 32)}
	server := httptest.NewUnstartedServer(accountNotificationAuthority(f.store, runtime, repairM1062Token, clientLeaf.SHA256))
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverLeaf.Pair}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.Roots()}
	server.StartTLS()
	t.Cleanup(server.Close)
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pki.Roots(), Certificates: []tls.Certificate{clientLeaf.Pair}}}}
	t.Cleanup(client.CloseIdleConnections)
	accountPath := "/internal/v2/accounts/" + f.principal.AccountID.String()
	targetPath := accountPath + "/targets/" + f.device.String()
	request := func(method, path string, body any, authorized bool) (int, []byte) {
		t.Helper()
		var input io.Reader
		if body != nil {
			encoded, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			input = bytes.NewReader(encoded)
		}
		req, err := http.NewRequestWithContext(context.Background(), method, server.URL+path, input)
		if err != nil {
			t.Fatal(err)
		}
		if authorized {
			req.Header.Set("Authorization", "Bearer "+repairM1062Token)
		}
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, raw
	}

	if status, _ := request(http.MethodGet, targetPath, nil, false); status != http.StatusForbidden {
		t.Fatalf("unauthorized target read = %d", status)
	}
	if status, _ := request(http.MethodGet, "/internal/v2/accounts/invalid/targets", nil, true); status != http.StatusForbidden {
		t.Fatalf("invalid account target list = %d", status)
	}
	if status, _ := request(http.MethodGet, accountPath+"/targets/invalid", nil, true); status != http.StatusForbidden {
		t.Fatalf("invalid device target = %d", status)
	}
	status, raw := request(http.MethodGet, accountPath+"/targets", nil, true)
	var targets []accountPushTarget
	if status != http.StatusOK || json.Unmarshal(raw, &targets) != nil || len(targets) != 1 || targets[0].Status != "active" {
		t.Fatalf("target list = %d %s", status, raw)
	}
	target := targets[0]
	if target.AccountID != f.principal.AccountID.String() || target.DeviceInstallationID != f.device.String() || target.Provider != "fcm" || target.EncryptedEndpointRef == "" {
		t.Fatalf("target identity changed: %+v", target)
	}
	status, raw = request(http.MethodPost, targetPath+":resolve", target, true)
	var resolved struct{ Token string }
	if status != http.StatusOK || json.Unmarshal(raw, &resolved) != nil || resolved.Token != "repair-m1062-device-token" {
		t.Fatalf("target resolution = %d %s", status, raw)
	}
	status, raw = request(http.MethodGet, accountPath+"/verified-account", nil, true)
	var linked struct{ Verified bool }
	if status != http.StatusOK || json.Unmarshal(raw, &linked) != nil || !linked.Verified {
		t.Fatalf("verified account = %d %s", status, raw)
	}
	if status, raw := request(http.MethodGet, accountPath+"/delivery-preference", nil, true); status != http.StatusConflict {
		t.Fatalf("unpublished delivery preference = %d %s", status, raw)
	}
	if err := f.store.PatchPreferences(context.Background(), f.principal, f.principal.AccountID, account.PreferencesPatch{ExpectedRevision: "0"}); err != nil {
		t.Fatal(err)
	}
	status, raw = request(http.MethodGet, accountPath+"/delivery-preference", nil, true)
	var preference accountDeliveryPreference
	if status != http.StatusOK || json.Unmarshal(raw, &preference) != nil || preference.AccountID != f.principal.AccountID.String() || preference.Revision != 1 || !preference.Linked || preference.QuietHours == nil {
		t.Fatalf("delivery preference = %d %s", status, raw)
	}
	if _, err := f.store.DB.Exec(context.Background(), `INSERT INTO account_consent(account_id,category,channel,enabled,source,policy_version,revision,revoke_revision,changed_at) VALUES($1,'trade','push',true,'settings','1',1,0,clock_timestamp())`, f.principal.AccountID); err != nil {
		t.Fatal(err)
	}
	status, raw = request(http.MethodGet, accountPath+"/delivery-preference", nil, true)
	if status != http.StatusOK || json.Unmarshal(raw, &preference) != nil || preference.CategoryChannels["trade"]["push"] != preference.Channels["push"] || preference.Categories["trade"] != preference.Channels["push"] {
		t.Fatalf("effective trade consent = %d %s", status, raw)
	}
	if _, err := f.store.DB.Exec(context.Background(), `INSERT INTO account_consent(account_id,category,channel,enabled,source,policy_version,revision,revoke_revision,changed_at) VALUES($1,'unknown','push',true,'settings','1',1,0,clock_timestamp())`, f.principal.AccountID); err != nil {
		t.Fatal(err)
	}
	if status, raw := request(http.MethodGet, accountPath+"/delivery-preference", nil, true); status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown consent category = %d %s", status, raw)
	}
	if _, err := f.store.DB.Exec(context.Background(), `DELETE FROM account_consent WHERE account_id=$1 AND category='unknown'`, f.principal.AccountID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.DB.Exec(context.Background(), `UPDATE account_state SET preferences=jsonb_set(preferences,'{quietHours,start}','"invalid"'::jsonb) WHERE account_id=$1`, f.principal.AccountID); err != nil {
		t.Fatal(err)
	}
	if status, raw := request(http.MethodGet, accountPath+"/delivery-preference", nil, true); status != http.StatusUnprocessableEntity {
		t.Fatalf("invalid quiet hours = %d %s", status, raw)
	}
	if _, err := f.store.DB.Exec(context.Background(), `UPDATE account_state SET preferences=jsonb_set(preferences,'{quietHours,start}','"22:00"'::jsonb) WHERE account_id=$1`, f.principal.AccountID); err != nil {
		t.Fatal(err)
	}
	status, raw = request(http.MethodGet, accountPath+"/watchlist", nil, true)
	var watch struct {
		AccountID string
		Revision  uint64
		Entries   []struct{ PoolAddress string }
	}
	if status != http.StatusOK || json.Unmarshal(raw, &watch) != nil || watch.AccountID != f.principal.AccountID.String() || watch.Revision != 1 || len(watch.Entries) != 0 {
		t.Fatalf("empty watchlist = %d %s", status, raw)
	}
	poolAddress := "0x3333333333333333333333333333333333333333"
	if _, err := f.store.DB.Exec(context.Background(), `INSERT INTO account_watch(account_id,chain_id,pool,added_at,revision,alert_enabled) VALUES($1,125,$2,clock_timestamp(),1,true)`, f.principal.AccountID, poolAddress); err != nil {
		t.Fatal(err)
	}
	status, raw = request(http.MethodGet, accountPath+"/watchlist", nil, true)
	if status != http.StatusOK || json.Unmarshal(raw, &watch) != nil || len(watch.Entries) != 1 || watch.Entries[0].PoolAddress != poolAddress {
		t.Fatalf("watchlist target = %d %s", status, raw)
	}
	wallet := "0x1062106210621062106210621062106210621062"
	status, raw = request(http.MethodGet, "/internal/v2/wallets/"+wallet+"/attribution", nil, true)
	var attribution struct {
		AccountID, ProfileID string
		Verified, OptedIn    bool
	}
	if status != http.StatusOK || json.Unmarshal(raw, &attribution) != nil || attribution.AccountID != f.principal.AccountID.String() || !attribution.Verified || attribution.OptedIn || attribution.ProfileID != "" {
		t.Fatalf("private attribution = %d %s", status, raw)
	}
	if _, err := f.store.DB.Exec(context.Background(), `UPDATE identity_wallet_links SET visibility='public_optin' WHERE account_id=$1`, f.principal.AccountID); err != nil {
		t.Fatal(err)
	}
	status, raw = request(http.MethodGet, accountPath+"/attribution?wallet="+url.QueryEscape(wallet), nil, true)
	if status != http.StatusOK || json.Unmarshal(raw, &attribution) != nil || !attribution.OptedIn || attribution.ProfileID != wallet {
		t.Fatalf("public attribution = %d %s", status, raw)
	}
	status, raw = request(http.MethodGet, targetPath, nil, true)
	if status != http.StatusOK || json.Unmarshal(raw, &target) != nil || target.Revision != 1 || target.Status != "active" {
		t.Fatalf("updated target = %d %s", status, raw)
	}
	stale := target
	stale.Revision++
	if status, _ := request(http.MethodPost, targetPath+":resolve", stale, true); status != http.StatusForbidden {
		t.Fatalf("stale target resolution = %d", status)
	}
	if status, _ := request(http.MethodPost, targetPath+":disable", map[string]any{"target": target}, true); status != http.StatusUnprocessableEntity {
		t.Fatalf("target disable without reason = %d", status)
	}
	if status, raw = request(http.MethodPost, targetPath+":disable", map[string]any{"target": target, "reason": "owner revoked installation"}, true); status != http.StatusNoContent {
		currentStatus, current := request(http.MethodGet, targetPath, nil, true)
		t.Fatalf("target disable = %d %s; current target = %d %s; submitted target = %+v", status, raw, currentStatus, current, target)
	}
	status, raw = request(http.MethodGet, targetPath, nil, true)
	var revoked accountPushTarget
	if status != http.StatusOK || json.Unmarshal(raw, &revoked) != nil || revoked.Status != "revoked" {
		t.Fatalf("revoked target = %d %s", status, raw)
	}
	if status, _ = request(http.MethodPost, targetPath+":resolve", target, true); status != http.StatusForbidden {
		t.Fatalf("revoked target resolved: %d", status)
	}
	if status, raw = request(http.MethodDelete, accountPath+"/targets", nil, true); status != http.StatusNoContent {
		t.Fatalf("target removal = %d %s", status, raw)
	}
	for _, tc := range []struct {
		name, method, path string
		body               any
		want               int
	}{
		{"missing target", http.MethodGet, accountPath + "/targets/" + uuid.NewString(), nil, http.StatusServiceUnavailable},
		{"invalid preference account", http.MethodGet, "/internal/v2/accounts/invalid/delivery-preference", nil, http.StatusForbidden},
		{"missing preference account", http.MethodGet, "/internal/v2/accounts/" + uuid.NewString() + "/delivery-preference", nil, http.StatusNotFound},
		{"invalid account watchlist", http.MethodGet, "/internal/v2/accounts/invalid/watchlist", nil, http.StatusForbidden},
		{"unknown account watchlist", http.MethodGet, "/internal/v2/accounts/" + uuid.NewString() + "/watchlist", nil, http.StatusConflict},
		{"invalid verified account", http.MethodGet, "/internal/v2/accounts/invalid/verified-account", nil, http.StatusForbidden},
		{"invalid account attribution", http.MethodGet, "/internal/v2/accounts/invalid/attribution?wallet=" + wallet, nil, http.StatusForbidden},
		{"invalid target removal", http.MethodDelete, "/internal/v2/accounts/invalid/targets", nil, http.StatusForbidden},
		{"unknown target operation", http.MethodPost, targetPath + ":other", target, http.StatusForbidden},
		{"invalid target account", http.MethodPost, "/internal/v2/accounts/invalid/targets/" + f.device.String() + ":resolve", target, http.StatusForbidden},
		{"invalid target device", http.MethodPost, accountPath + "/targets/invalid:resolve", target, http.StatusForbidden},
		{"invalid target document", http.MethodPost, targetPath + ":resolve", map[string]any{"unknown": true}, http.StatusUnprocessableEntity},
		{"invalid disable reason", http.MethodPost, targetPath + ":disable", map[string]any{"target": target, "reason": strings.Repeat("x", 161)}, http.StatusUnprocessableEntity},
		{"invalid wallet attribution", http.MethodGet, "/internal/v2/wallets/invalid/attribution", nil, http.StatusUnprocessableEntity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, raw := request(tc.method, tc.path, tc.body, true)
			if status != tc.want {
				t.Fatalf("notification route %s = %d %s, want %d", tc.path, status, raw, tc.want)
			}
		})
	}
	missingWallet := "0x4444444444444444444444444444444444444444"
	status, raw = request(http.MethodGet, "/internal/v2/wallets/"+missingWallet+"/attribution", nil, true)
	if status != http.StatusOK || json.Unmarshal(raw, &attribution) != nil || attribution.Verified || attribution.AccountID != "" {
		t.Fatalf("missing wallet attribution = %d %s", status, raw)
	}
	status, raw = request(http.MethodGet, "/internal/v2/accounts/"+uuid.NewString()+"/attribution?wallet="+url.QueryEscape(wallet), nil, true)
	if status != http.StatusForbidden {
		t.Fatalf("foreign account attribution = %d %s", status, raw)
	}
	sealedWebpush, err := account.Seal(runtime.EndpointKey, []byte("invalid-webpush-document"), []byte(f.principal.AccountID.String()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.DB.Exec(context.Background(), `UPDATE account_endpoint SET revoked=false,provider='webpush',ciphertext=$2 WHERE account_id=$1 AND device_id=$3`, f.principal.AccountID, sealedWebpush, f.device); err != nil {
		t.Fatal(err)
	}
	status, raw = request(http.MethodGet, targetPath, nil, true)
	if status != http.StatusOK || json.Unmarshal(raw, &target) != nil || target.Status != "active" || target.Provider != "webpush" {
		t.Fatalf("webpush target = %d %s", status, raw)
	}
	if status, raw := request(http.MethodPost, targetPath+":resolve", target, true); status != http.StatusUnprocessableEntity {
		t.Fatalf("invalid webpush payload = %d %s", status, raw)
	}
	webpush, err := account.Seal(runtime.EndpointKey, []byte(`{"url":"https://example.org/push","publicKey":"public-key","auth":"auth-key","permission":"granted"}`), []byte(f.principal.AccountID.String()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.DB.Exec(context.Background(), `UPDATE account_endpoint SET ciphertext=$2 WHERE account_id=$1 AND device_id=$3`, f.principal.AccountID, webpush, f.device); err != nil {
		t.Fatal(err)
	}
	status, raw = request(http.MethodGet, targetPath, nil, true)
	if status != http.StatusOK || json.Unmarshal(raw, &target) != nil {
		t.Fatalf("approved webpush target = %d %s", status, raw)
	}
	status, raw = request(http.MethodPost, targetPath+":resolve", target, true)
	var resolvedWebpush struct{ URL, PublicKey, Auth, Permission string }
	if status != http.StatusOK || json.Unmarshal(raw, &resolvedWebpush) != nil || resolvedWebpush.URL != "https://example.org/push" || resolvedWebpush.PublicKey != "public-key" || resolvedWebpush.Auth != "auth-key" || resolvedWebpush.Permission != "granted" {
		t.Fatalf("approved webpush resolution = %d %s", status, raw)
	}
	if _, err := f.store.DB.Exec(context.Background(), `UPDATE account_state SET revision=0 WHERE account_id=$1`, f.principal.AccountID); err != nil {
		t.Fatal(err)
	}
	if status, raw := request(http.MethodGet, accountPath+"/watchlist", nil, true); status != http.StatusConflict {
		t.Fatalf("unpublished watchlist = %d %s", status, raw)
	}
	if _, err := f.store.DB.Exec(context.Background(), `UPDATE account_state SET revision=1 WHERE account_id=$1`, f.principal.AccountID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.DB.Exec(context.Background(), `INSERT INTO account_endpoint(fingerprint,account_id,ciphertext,platform,consent_revision,validated_at,revoked,device_id,provider)
		SELECT decode(repeat(lpad(to_hex(i),2,'0'),32),'hex'),$1,'sealed','android',1,clock_timestamp(),false,md5(i::text)::uuid,'fcm' FROM generate_series(1,33) i`, f.principal.AccountID); err != nil {
		t.Fatal(err)
	}
	if status, raw := request(http.MethodGet, accountPath+"/targets", nil, true); status != http.StatusConflict {
		t.Fatalf("oversized target list = %d %s", status, raw)
	}
	f.store.DB.Close()
	for _, tc := range []struct {
		path string
		want int
	}{
		{accountPath + "/delivery-preference", http.StatusServiceUnavailable},
		{accountPath + "/targets", http.StatusServiceUnavailable},
		{targetPath, http.StatusServiceUnavailable},
		{accountPath + "/verified-account", http.StatusServiceUnavailable},
		{accountPath + "/watchlist", http.StatusConflict},
		{"/internal/v2/wallets/" + wallet + "/attribution", http.StatusServiceUnavailable},
	} {
		status, raw := request(http.MethodGet, tc.path, nil, true)
		if status != tc.want {
			t.Fatalf("unavailable account data for %s = %d %s, want %d", tc.path, status, raw, tc.want)
		}
	}
}
