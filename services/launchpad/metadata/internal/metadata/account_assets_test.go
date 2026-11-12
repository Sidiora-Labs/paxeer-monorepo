package metadata

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	raster "github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/image"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/internaltest"
)

func TestAccountAssetUploadReadAndPurge(t *testing.T) {
	ctx := context.Background()
	pool, dsn := internaltest.NewPostgresURL(t)
	const schema = "metadata_account_assets"
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	store, err := NewProductionStore(pool, schema, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, store.SchemaDDL()+store.V2SchemaDDL()+store.ConnectedSchemaDDL()+store.AccountAssetSchemaDDL()); err != nil {
		t.Fatal(err)
	}
	client, approved := edgeObjects(t, "metadata-account-approved")
	quarantine := internaltest.NewMinIO(t, "metadata-account-quarantine")
	redisURL := internaltest.NewRedisURL(t)
	gateway := internaltest.NewGateway(t, dsn, redisURL, approved, "https://app.sidiora.test")
	wallet, token := gateway.SignIn(t)
	scannerHost, scannerPort := internaltest.NewClamd(t)
	if scannerHost == "" || scannerPort == 0 {
		t.Fatal("scanner required for account asset flow")
	}
	media := ConnectedMedia{
		Metadata:   V2ContractHandlers{Store: store},
		Identity:   sharedauth.HTTPIdentityAuthority{Endpoint: gateway.URL, ServiceToken: gateway.ServiceToken, AllowPlainHTTP: true},
		Objects:    S3AssetObjects{Client: client, Endpoint: approved.Endpoint, QuarantineBucket: quarantine.Bucket, PublicBucket: approved.Bucket, PublicOrigin: "https://metadata.sidiora.test"},
		Scanner:    sharedhttp.ScanOptions{Host: scannerHost, Port: scannerPort},
		Worker:     raster.InProcessWorker{},
		Moderation: DevelopmentVisibility{},
	}
	developmentObjects := media.Objects
	developmentObjects.Readiness = DevelopmentObjectsReadiness(developmentObjects)
	if err := developmentObjects.Ready(ctx); err != nil {
		t.Fatalf("development buckets: %v", err)
	}
	badOrigin := developmentObjects
	badOrigin.PublicOrigin = "http://metadata.sidiora.test"
	if err := DevelopmentObjectsReadiness(badOrigin)(ctx); err == nil {
		t.Fatal("insecure public origin accepted")
	}
	missingBucket := developmentObjects
	missingBucket.PublicBucket = "missing-public-bucket"
	if err := DevelopmentObjectsReadiness(missingBucket)(ctx); err == nil {
		t.Fatal("missing public bucket accepted")
	}
	router := chi.NewRouter()
	media.RegisterRoutes(router)
	media.RegisterAccountAuthority(router)
	media.RegisterApprovedObjects(router)
	request := func(method, path, bearer string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Origin", "https://app.sidiora.test")
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	id := uuid.NewString()
	path := "/v2/accounts/" + wallet + "/assets/" + id
	body := pngBytes(t, 16, 12)
	otherWallet := "0x" + strings.Repeat("b", 40)
	if response := request(http.MethodPut, "/v2/accounts/"+otherWallet+"/assets/"+id, token, body, map[string]string{"Content-Type": "image/png", "Idempotency-Key": id}); response.Code != http.StatusForbidden {
		t.Fatalf("foreign account upload: %d %s", response.Code, response.Body.String())
	}
	oversizedID := uuid.NewString()
	if response := request(http.MethodPut, "/v2/accounts/"+wallet+"/assets/"+oversizedID, token, bytes.Repeat([]byte("x"), raster.MaxBytes+1), map[string]string{"Content-Type": "image/png", "Idempotency-Key": oversizedID}); response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized account upload: %d %s", response.Code, response.Body.String())
	}
	invalidID := uuid.NewString()
	if response := request(http.MethodPut, "/v2/accounts/"+wallet+"/assets/"+invalidID, token, []byte("not an image"), map[string]string{"Content-Type": "text/plain", "Idempotency-Key": invalidID}); response.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("unsupported account image: %d %s", response.Code, response.Body.String())
	}
	if response := request(http.MethodPut, path, token, body, map[string]string{"Content-Type": "image/png", "Idempotency-Key": uuid.NewString()}); response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("mismatched upload key: %d %s", response.Code, response.Body.String())
	}
	if response := request(http.MethodPut, path, token, body, map[string]string{"Content-Type": "image/jpeg", "Idempotency-Key": id}); response.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("mismatched media type: %d %s", response.Code, response.Body.String())
	}
	if response := request(http.MethodPut, path, token, body[:24], map[string]string{"Content-Type": "image/png", "Idempotency-Key": id}); response.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("truncated account image: %d %s", response.Code, response.Body.String())
	}
	tooWide := append([]byte(nil), body...)
	binary.BigEndian.PutUint32(tooWide[16:20], 8193)
	binary.BigEndian.PutUint32(tooWide[29:33], crc32.ChecksumIEEE(tooWide[12:29]))
	if response := request(http.MethodPut, path, token, tooWide, map[string]string{"Content-Type": "image/png", "Idempotency-Key": id}); response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("account raster dimension limit: %d %s", response.Code, response.Body.String())
	}
	for _, tc := range []struct {
		name   string
		change func(*ConnectedMedia)
	}{
		{"worker", func(c *ConnectedMedia) { c.Worker = raster.ProcessWorker{} }},
		{"objects", func(c *ConnectedMedia) { c.Objects.PublicOrigin = "http://metadata.sidiora.test" }},
		{"identity", func(c *ConnectedMedia) { c.Identity = nil }},
	} {
		t.Run(tc.name+" unavailable", func(t *testing.T) {
			variant := media
			tc.change(&variant)
			isolated := chi.NewRouter()
			variant.RegisterRoutes(isolated)
			otherID := uuid.NewString()
			otherPath := "/v2/accounts/" + wallet + "/assets/" + otherID
			r := httptest.NewRequest(http.MethodPut, otherPath, bytes.NewReader(body))
			r.Header.Set("Authorization", "Bearer "+token)
			r.Header.Set("Origin", "https://app.sidiora.test")
			r.Header.Set("Content-Type", "image/png")
			r.Header.Set("Idempotency-Key", otherID)
			response := httptest.NewRecorder()
			isolated.ServeHTTP(response, r)
			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("unavailable account upload = %d %s", response.Code, response.Body.String())
			}
		})
	}
	deferredScanner := media
	deferredScanner.Scanner.Port = 1
	deferredRouter := chi.NewRouter()
	deferredScanner.RegisterRoutes(deferredRouter)
	deferredID := uuid.NewString()
	deferredPath := "/v2/accounts/" + wallet + "/assets/" + deferredID
	deferredRequest := httptest.NewRequest(http.MethodPut, deferredPath, bytes.NewReader(body))
	deferredRequest.Header.Set("Authorization", "Bearer "+token)
	deferredRequest.Header.Set("Origin", "https://app.sidiora.test")
	deferredRequest.Header.Set("Content-Type", "image/png")
	deferredRequest.Header.Set("Idempotency-Key", deferredID)
	deferredResponse := httptest.NewRecorder()
	deferredRouter.ServeHTTP(deferredResponse, deferredRequest)
	if deferredResponse.Code != http.StatusAccepted {
		t.Fatalf("deferred account scan = %d %s", deferredResponse.Code, deferredResponse.Body.String())
	}
	var deferredState string
	if err := pool.QueryRow(ctx, "SELECT state FROM "+store.table("account_assets_v2")+" WHERE id=$1", deferredID).Scan(&deferredState); err != nil || deferredState != metadataStateValidating {
		t.Fatalf("deferred account state = %q, %v", deferredState, err)
	}
	response := request(http.MethodPut, path, token, body, map[string]string{"Content-Type": "image/png", "Idempotency-Key": id})
	var asset WireAsset
	if err := json.Unmarshal(response.Body.Bytes(), &asset); err != nil || response.Code != http.StatusOK || asset.AssetID != id || asset.OwnerWallet != wallet || asset.State != metadataStateApprovedWire || asset.PublicURL == nil {
		t.Fatalf("approved account asset: %d %+v, %v", response.Code, asset, err)
	}
	if !strings.HasPrefix(*asset.PublicURL, media.Objects.PublicOrigin+"/approved/account/"+wallet+"/"+id+"/") {
		t.Fatalf("approved URL = %q", *asset.PublicURL)
	}
	if response := request(http.MethodGet, path, token, nil, nil); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), id) {
		t.Fatalf("owned asset read: %d %s", response.Code, response.Body.String())
	}
	if response := request(http.MethodGet, "/internal/v2/accounts/"+strings.Repeat("b", 40)+"/assets/"+id, "", nil, nil); response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid account address: %d %s", response.Code, response.Body.String())
	}
	if response := request(http.MethodGet, "/v2/accounts/"+otherWallet+"/assets/"+id, token, nil, nil); response.Code != http.StatusForbidden {
		t.Fatalf("foreign account read: %d %s", response.Code, response.Body.String())
	}
	if response := request(http.MethodGet, "/internal/v2/accounts/"+wallet+"/assets/invalid", "", nil, nil); response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid asset id: %d %s", response.Code, response.Body.String())
	}
	if response := request(http.MethodPut, path, token, body, map[string]string{"Content-Type": "image/png", "Idempotency-Key": id}); response.Code != http.StatusOK {
		t.Fatalf("idempotent upload: %d %s", response.Code, response.Body.String())
	}
	if response := request(http.MethodPut, path, token, pngBytes(t, 17, 12), map[string]string{"Content-Type": "image/png", "Idempotency-Key": id}); response.Code != http.StatusConflict {
		t.Fatalf("changed upload body: %d %s", response.Code, response.Body.String())
	}
	if err := media.Objects.put(ctx, quarantine.Bucket, "quarantine/account/"+id, pngBytes(t, 17, 12), "image/png"); err != ErrConflict {
		t.Fatalf("changed immutable object: %v", err)
	}
	for _, key := range []string{"public/unscoped.png", "approved/../escape.png"} {
		if err := media.Objects.PutImmutable(ctx, key, body); err != ErrValidation {
			t.Fatalf("unsafe approved key %q: %v", key, err)
		}
	}
	if response := request(http.MethodGet, "/internal"+path, "", nil, nil); response.Code != http.StatusOK {
		t.Fatalf("internal asset read: %d %s", response.Code, response.Body.String())
	}
	if response := request(http.MethodGet, strings.TrimPrefix(*asset.PublicURL, media.Objects.PublicOrigin), "", nil, nil); response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("approved object read: %d %s", response.Code, response.Body.String())
	}
	var variants []byte
	if err := pool.QueryRow(ctx, "SELECT variants FROM "+store.table("account_assets_v2")+" WHERE id=$1", id).Scan(&variants); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("account_assets_v2")+" SET variants='{}'::jsonb WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	if response := request(http.MethodGet, path, token, nil, nil); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("invalid approved variants: %d %s", response.Code, response.Body.String())
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("account_assets_v2")+" SET variants=$2::jsonb,blocked=true,moderation_revision=1 WHERE id=$1", id, string(variants)); err != nil {
		t.Fatal(err)
	}
	if response := request(http.MethodGet, path, token, nil, nil); response.Code != http.StatusNotFound {
		t.Fatalf("restricted account asset: %d %s", response.Code, response.Body.String())
	}
	if err := media.ProcessAccountAsset(ctx, id); err != ErrForbidden {
		t.Fatalf("restricted processing: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("account_assets_v2")+" SET blocked=false,moderation_revision=0 WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	deletion := uuid.NewString()
	if response := request(http.MethodDelete, "/internal/v2/accounts/"+wallet+"/assets?deletionId=invalid", "", nil, nil); response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid deletion id: %d %s", response.Code, response.Body.String())
	}
	purgePath := "/internal/v2/accounts/" + wallet + "/assets?deletionId=" + deletion
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("account_assets_v2")+" SET variants='\"invalid\"'::jsonb WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	if response := request(http.MethodDelete, purgePath, "", nil, nil); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("invalid variant inventory during purge = %d %s", response.Code, response.Body.String())
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("account_assets_v2")+" SET variants=$2::jsonb WHERE id=$1", id, string(variants)); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*ConnectedMedia)
	}{
		{"quarantine bucket unavailable", func(c *ConnectedMedia) { c.Objects.QuarantineBucket = "missing-quarantine-bucket" }},
		{"public bucket unavailable", func(c *ConnectedMedia) { c.Objects.PublicBucket = "missing-public-bucket" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			variant := media
			tc.change(&variant)
			isolated := chi.NewRouter()
			variant.RegisterAccountAuthority(isolated)
			response := httptest.NewRecorder()
			isolated.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, purgePath, nil))
			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("object purge dependency = %d %s", response.Code, response.Body.String())
			}
		})
	}
	if response := request(http.MethodDelete, purgePath, "", nil, nil); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"deleted":true`) {
		t.Fatalf("account asset purge: %d %s", response.Code, response.Body.String())
	}
	if response := request(http.MethodGet, path, token, nil, nil); response.Code != http.StatusNotFound {
		t.Fatalf("deleted account asset read: %d %s", response.Code, response.Body.String())
	}
	if response := request(http.MethodPut, path, token, body, map[string]string{"Content-Type": "image/png", "Idempotency-Key": id}); response.Code != http.StatusForbidden {
		t.Fatalf("deleted account asset upload: %d %s", response.Code, response.Body.String())
	}
	pendingID := uuid.NewString()
	pendingPath := "/v2/accounts/" + wallet + "/assets/" + pendingID
	if _, err := pool.Exec(ctx, "INSERT INTO "+store.table("account_assets_v2")+"(id,owner_wallet,hash,size,mime,width,height,state) VALUES($1,$2,$3,$4,'image/png',16,12,'UPLOADING')", pendingID, wallet, strings.TrimPrefix(asset.SHA256, "0x"), len(body)); err != nil {
		t.Fatal(err)
	}
	if err := media.ProcessAccountAsset(ctx, pendingID); err != nil {
		t.Fatalf("pending account object: %v", err)
	}
	if err := media.MaintainAccountAssets(ctx, false); err != nil {
		t.Fatalf("pending account maintenance: %v", err)
	}
	if response := request(http.MethodGet, pendingPath, token, nil, nil); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), metadataStateUploadingWire) {
		t.Fatalf("pending account asset: %d %s", response.Code, response.Body.String())
	}
	if err := media.Objects.put(ctx, quarantine.Bucket, "quarantine/account/"+pendingID, body, "image/png"); err != nil {
		t.Fatal(err)
	}
	if err := media.MaintainAccountAssets(ctx, false); err != nil {
		t.Fatalf("account maintenance: %v", err)
	}
	if response := request(http.MethodGet, pendingPath, token, nil, nil); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), metadataStateApprovedWire) {
		t.Fatalf("approved maintained asset: %d %s", response.Code, response.Body.String())
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("account_assets_v2")+" SET investigation_hold=true WHERE id=$1", pendingID); err != nil {
		t.Fatal(err)
	}
	deletion = uuid.NewString()
	if response := request(http.MethodDelete, "/internal/v2/accounts/"+wallet+"/assets?deletionId="+deletion, "", nil, nil); response.Code != http.StatusForbidden {
		t.Fatalf("held account asset purge: %d %s", response.Code, response.Body.String())
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("account_assets_v2")+" SET investigation_hold=false WHERE id=$1", pendingID); err != nil {
		t.Fatal(err)
	}
	if response := request(http.MethodDelete, "/internal/v2/accounts/"+wallet+"/assets?deletionId="+deletion, "", nil, nil); response.Code != http.StatusOK {
		t.Fatalf("released account asset purge: %d %s", response.Code, response.Body.String())
	}
	rejectedID := uuid.NewString()
	if _, err := pool.Exec(ctx, "INSERT INTO "+store.table("account_assets_v2")+"(id,owner_wallet,hash,size,mime,width,height,state,rejected_until) VALUES($1,$2,$3,$4,'image/png',16,12,'REJECTED',clock_timestamp()-interval '1 hour')", rejectedID, wallet, strings.TrimPrefix(asset.SHA256, "0x"), len(body)); err != nil {
		t.Fatal(err)
	}
	if err := media.Objects.put(ctx, quarantine.Bucket, "quarantine/account/"+rejectedID, body, "image/png"); err != nil {
		t.Fatal(err)
	}
	if err := media.MaintainAccountAssets(ctx, true); err != nil {
		t.Fatalf("rejected account retention: %v", err)
	}
	var state string
	if err := pool.QueryRow(ctx, "SELECT state FROM "+store.table("account_assets_v2")+" WHERE id=$1", rejectedID).Scan(&state); err != nil || state != metadataStateDeleted {
		t.Fatalf("retained rejected asset = %q, %v", state, err)
	}
	corruptID := uuid.NewString()
	if _, err := pool.Exec(ctx, "INSERT INTO "+store.table("account_assets_v2")+"(id,owner_wallet,hash,size,mime,width,height,state) VALUES($1,$2,$3,$4,'image/png',16,12,'UPLOADING')", corruptID, wallet, strings.TrimPrefix(asset.SHA256, "0x"), len(body)); err != nil {
		t.Fatal(err)
	}
	if err := media.Objects.put(ctx, quarantine.Bucket, "quarantine/account/"+corruptID, pngBytes(t, 17, 12), "image/png"); err != nil {
		t.Fatal(err)
	}
	if err := media.ProcessAccountAsset(ctx, corruptID); err != ErrConflict {
		t.Fatalf("changed quarantine bytes: %v", err)
	}
	missingID := uuid.NewString()
	if _, err := pool.Exec(ctx, "INSERT INTO "+store.table("account_assets_v2")+"(id,owner_wallet,hash,size,mime,width,height,state) VALUES($1,$2,$3,$4,'image/png',16,12,'UPLOADING')", missingID, wallet, strings.TrimPrefix(asset.SHA256, "0x"), len(body)); err != nil {
		t.Fatal(err)
	}
	if err := media.ProcessAccountAsset(ctx, missingID); err != nil {
		t.Fatalf("upload awaiting object: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("account_assets_v2")+" SET state='QUARANTINED' WHERE id=$1", missingID); err != nil {
		t.Fatal(err)
	}
	if err := media.ProcessAccountAsset(ctx, missingID); err != raster.ErrUnavailable {
		t.Fatalf("quarantined asset without object: %v", err)
	}
	if err := media.Objects.put(ctx, quarantine.Bucket, "quarantine/account/"+missingID, body, "image/png"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("account_assets_v2")+" SET width=17 WHERE id=$1", missingID); err != nil {
		t.Fatal(err)
	}
	if err := media.ProcessAccountAsset(ctx, missingID); err != ErrConflict {
		t.Fatalf("asset dimensions differ from stored image: %v", err)
	}
	if err := media.ProcessAccountAsset(ctx, uuid.NewString()); err == nil {
		t.Fatal("unknown account asset accepted")
	}
	unsafeID := uuid.NewString()
	if _, err := pool.Exec(ctx, "INSERT INTO "+store.table("account_assets_v2")+"(id,owner_wallet,hash,size,mime,width,height,state,variants) VALUES($1,$2,$3,$4,'image/png',16,12,'APPROVED',$5::jsonb)", unsafeID, wallet, strings.TrimPrefix(asset.SHA256, "0x"), len(body), `{"256":"approved/other/asset.png"}`); err != nil {
		t.Fatal(err)
	}
	deletion = uuid.NewString()
	if response := request(http.MethodDelete, "/internal/v2/accounts/"+wallet+"/assets?deletionId="+deletion, "", nil, nil); response.Code != http.StatusForbidden {
		t.Fatalf("unscoped variant purge: %d %s", response.Code, response.Body.String())
	}
	if _, err := pool.Exec(ctx, "DROP TABLE "+store.table("account_assets_v2")); err != nil {
		t.Fatal(err)
	}
	if err := media.MaintainAccountAssets(ctx, false); err == nil {
		t.Fatal("missing account inventory accepted")
	}
	if err := media.ProcessAccountAsset(ctx, uuid.NewString()); err == nil {
		t.Fatal("missing account asset table accepted")
	}
	if response := request(http.MethodGet, path, token, nil, nil); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("account read without inventory: %d %s", response.Code, response.Body.String())
	}
	unavailableID := uuid.NewString()
	if response := request(http.MethodPut, "/v2/accounts/"+wallet+"/assets/"+unavailableID, token, body, map[string]string{"Content-Type": "image/png", "Idempotency-Key": unavailableID}); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("account upload without inventory: %d %s", response.Code, response.Body.String())
	}
}
