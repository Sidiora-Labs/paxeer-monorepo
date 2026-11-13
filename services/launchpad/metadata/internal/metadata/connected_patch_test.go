package metadata

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hash/crc32"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	raster "github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/image"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/internaltest"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
)

func TestConnectedCanonicalPatchAndSearch(t *testing.T) {
	ctx := context.Background()
	pool, dsn := internaltest.NewPostgresURL(t)
	for _, schema := range []string{"metadata_patch_canonical", "metadata_patch_document"} {
		if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
			t.Fatal(err)
		}
	}
	key := repair1007Key(t)
	creation := repair1007Creator(loadCreationFixture(t), key)
	const stream = "paxeer125-canonical"
	seedCanonical(t, pool, "metadata_patch_canonical", stream, creation)
	store, err := NewProductionStore(pool, "metadata_patch_document", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, store.SchemaDDL()+store.V2SchemaDDL()+store.ConnectedSchemaDDL()); err != nil {
		t.Fatal(err)
	}
	const audience = "https://metadata.sidiora.test"
	store.Verifier = V2SignedVerifier{Store: store, Audience: audience}
	objects := internaltest.NewMinIO(t, "metadata-patch")
	approvedClient, approved := edgeObjects(t, "metadata-patch-approved")
	quarantine := internaltest.NewMinIO(t, "metadata-patch-quarantine")
	scannerHost, scannerPort := internaltest.NewClamd(t)
	redisURL := internaltest.NewRedisURL(t)
	gateway := internaltest.NewGateway(t, dsn, redisURL, objects, "https://app.sidiora.test")
	wallet, bearer := gateway.SignInWithKey(t, key)
	if wallet != creation.Creator {
		t.Fatalf("creation owner = %q, signed in = %q", creation.Creator, wallet)
	}
	connected := ConnectedMedia{
		Metadata:   V2ContractHandlers{Store: store, Canonical: PostgresCanonicalSource{Schema: "metadata_patch_canonical", CanonicalStreamID: stream}},
		Identity:   sharedauth.HTTPIdentityAuthority{Endpoint: gateway.URL, ServiceToken: gateway.ServiceToken, AllowPlainHTTP: true},
		Objects:    S3AssetObjects{Client: approvedClient, Endpoint: approved.Endpoint, QuarantineBucket: quarantine.Bucket, PublicBucket: approved.Bucket, PublicOrigin: "https://metadata.sidiora.test"},
		Scanner:    sharedhttp.ScanOptions{Host: scannerHost, Port: scannerPort},
		Worker:     raster.InProcessWorker{},
		Moderation: DevelopmentVisibility{},
	}
	router := chi.NewRouter()
	connected.RegisterRoutes(router)
	direct := chi.NewRouter()
	connected.Metadata.RegisterRoutes(direct)
	path := "/v2/tokens/" + creation.Token + "/metadata"
	directPatch := func(token, actor string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(http.MethodPatch, "/v2/tokens/"+token+"/metadata", strings.NewReader(`{"expectedRevision":"0"}`))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Action-Wallet", actor)
		r.Header.Set("Idempotency-Key", uuid.NewString())
		response := httptest.NewRecorder()
		direct.ServeHTTP(response, r)
		return response
	}
	if response := directPatch(creation.Token, "invalid"); response.Code != http.StatusForbidden {
		t.Fatalf("invalid direct action wallet = %d %s", response.Code, response.Body.String())
	}
	if response := directPatch("0x"+strings.Repeat("f", 40), creation.Creator); response.Code != http.StatusNotFound {
		t.Fatalf("unknown direct action token = %d %s", response.Code, response.Body.String())
	}
	if response := directPatch(creation.Token, "0x"+strings.Repeat("b", 40)); response.Code != http.StatusForbidden {
		t.Fatalf("foreign direct action wallet = %d %s", response.Code, response.Body.String())
	}
	unpublished := httptest.NewRecorder()
	router.ServeHTTP(unpublished, httptest.NewRequest(http.MethodGet, path, nil))
	if unpublished.Code != http.StatusOK || !strings.Contains(unpublished.Body.String(), `"revision":"0"`) {
		t.Fatalf("unpublished canonical metadata = %d %s", unpublished.Code, unpublished.Body.String())
	}
	if _, err := pool.Exec(ctx, "INSERT INTO "+store.table("pools")+"(pool,creator,token) VALUES($1,$2,$3)", creation.Pool, creation.Creator, creation.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE "+store.table("documents")+" RENAME TO documents_hidden"); err != nil {
		t.Fatal(err)
	}
	missingModerationTable := httptest.NewRecorder()
	direct.ServeHTTP(missingModerationTable, httptest.NewRequest(http.MethodGet, path, nil))
	if missingModerationTable.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable moderation table = %d %s", missingModerationTable.Code, missingModerationTable.Body.String())
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE "+store.table("documents_hidden")+" RENAME TO documents"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO "+store.table("documents")+"(pool,blocked) VALUES($1,true)", creation.Pool); err != nil {
		t.Fatal(err)
	}
	blockedUnpublished := httptest.NewRecorder()
	router.ServeHTTP(blockedUnpublished, httptest.NewRequest(http.MethodGet, path, nil))
	if blockedUnpublished.Code != http.StatusForbidden {
		t.Fatalf("blocked unpublished metadata = %d %s", blockedUnpublished.Code, blockedUnpublished.Body.String())
	}
	if _, err := pool.Exec(ctx, "DELETE FROM "+store.table("documents")+" WHERE pool=$1", creation.Pool); err != nil {
		t.Fatal(err)
	}
	patch := func(raw []byte, operation string) *httptest.ResponseRecorder {
		t.Helper()
		hash := sha256.Sum256(raw)
		action := VerifiedAction{Wallet: wallet, OperationID: operation, Method: http.MethodPatch, Path: path, BodyHash: hex.EncodeToString(hash[:]), Nonce: hex.EncodeToString(hash[:]), ExpiresAt: strconv.FormatInt(time.Now().Add(time.Minute).Unix(), 10), Audience: audience, ChainID: 125}
		signature, err := crypto.Sign(accounts.TextHash([]byte(V2SigningMessage(action))), key)
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodPatch, path, bytes.NewReader(raw))
		request.Header.Set("Origin", "https://app.sidiora.test")
		request.Header.Set("Authorization", "Bearer "+bearer)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", operation)
		request.Header.Set("X-Action-Wallet", wallet)
		request.Header.Set("X-Action-Signature", "0x"+hex.EncodeToString(signature))
		request.Header.Set("X-Action-Nonce", action.Nonce)
		request.Header.Set("X-Action-Expires", action.ExpiresAt)
		request.Header.Set("X-Action-Audience", audience)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}
	raw := []byte(`{"expectedRevision":"0","displayName":"Éclair","description":"Creator text","tags":["art"],"socialLinks":[]}`)
	operation := uuid.NewString()
	response := patch(raw, operation)
	if response.Code != http.StatusOK {
		t.Fatalf("canonical patch: %d %s", response.Code, response.Body.String())
	}
	var result struct {
		Data struct {
			Revision string `json:"revision"`
		}
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result.Data.Revision != "1" {
		t.Fatalf("patch revision = %+v, %v", result, err)
	}
	for _, tc := range []struct {
		name, contentType, wallet string
		body                      []byte
		want                      int
	}{
		{"unsupported patch type", "text/plain", wallet, []byte(`{"expectedRevision":"1"}`), http.StatusUnsupportedMediaType},
		{"oversized patch body", "application/json", wallet, bytes.Repeat([]byte("x"), V2BodyLimit+1), http.StatusUnprocessableEntity},
		{"foreign action wallet", "application/json", "0x" + strings.Repeat("b", 40), []byte(`{"expectedRevision":"1"}`), http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPatch, path, bytes.NewReader(tc.body))
			request.Header.Set("Authorization", "Bearer "+bearer)
			request.Header.Set("Content-Type", tc.contentType)
			request.Header.Set("X-Action-Wallet", tc.wallet)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tc.want {
				t.Fatalf("patch admission = %d %s, want %d", response.Code, response.Body.String(), tc.want)
			}
		})
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("pools")+" SET creator=$2 WHERE token=$1", creation.Token, "0x"+strings.Repeat("b", 40)); err != nil {
		t.Fatal(err)
	}
	if response := directPatch(creation.Token, creation.Creator); response.Code != http.StatusForbidden {
		t.Fatalf("changed direct action owner = %d %s", response.Code, response.Body.String())
	}
	if changedOwner := patch([]byte(`{"expectedRevision":"1","description":"Owner check"}`), uuid.NewString()); changedOwner.Code != http.StatusForbidden {
		t.Fatalf("changed pool creator = %d %s", changedOwner.Code, changedOwner.Body.String())
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("pools")+" SET creator=$2 WHERE token=$1", creation.Token, wallet); err != nil {
		t.Fatal(err)
	}
	var search []byte
	if err := pool.QueryRow(ctx, "SELECT search_document FROM "+store.table("v2_documents")+" WHERE pool=$1", creation.Pool).Scan(&search); err != nil || !bytes.Contains(search, []byte("éclair")) {
		t.Fatalf("search document = %s, %v", search, err)
	}
	read := httptest.NewRecorder()
	router.ServeHTTP(read, httptest.NewRequest(http.MethodGet, path, nil))
	if read.Code != http.StatusOK || !strings.Contains(read.Body.String(), "Creator text") {
		t.Fatalf("canonical read: %d %s", read.Code, read.Body.String())
	}
	var directDocument []byte
	if err := pool.QueryRow(ctx, "SELECT document FROM "+store.table("v2_documents")+" WHERE pool=$1", creation.Pool).Scan(&directDocument); err != nil {
		t.Fatal(err)
	}
	for _, document := range []string{`"invalid"`, `{"description":"` + strings.Repeat("x", 5000) + `","tags":[],"socials":[]}`} {
		if _, err := pool.Exec(ctx, "UPDATE "+store.table("v2_documents")+" SET document=$2::jsonb WHERE pool=$1", creation.Pool, document); err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		direct.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("unrepresentable direct document = %d %s", response.Code, response.Body.String())
		}
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("v2_documents")+" SET document=$2::jsonb WHERE pool=$1", creation.Pool, string(directDocument)); err != nil {
		t.Fatal(err)
	}
	cachedMetadataRequest := httptest.NewRequest(http.MethodGet, path, nil)
	cachedMetadataRequest.Header.Set("If-None-Match", read.Header().Get("ETag"))
	cachedMetadata := httptest.NewRecorder()
	router.ServeHTTP(cachedMetadata, cachedMetadataRequest)
	if cachedMetadata.Code != http.StatusOK || cachedMetadata.Header().Get("Cache-Control") != "no-store" || !strings.Contains(cachedMetadata.Body.String(), "Creator text") {
		t.Fatalf("metadata read: %d %s", cachedMetadata.Code, cachedMetadata.Body.String())
	}
	if response := patch([]byte(`{"expectedRevision":"0","displayName":"Stale"}`), uuid.NewString()); response.Code != http.StatusConflict {
		t.Fatalf("stale metadata revision: %d %s", response.Code, response.Body.String())
	}
	if response := patch([]byte(`{"expectedRevision":"1","displayName":null}`), uuid.NewString()); response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid metadata patch: %d %s", response.Code, response.Body.String())
	}
	for invalidToken, want := range map[string]int{"0x" + strings.Repeat("9", 40): http.StatusNotFound, "invalid-token": http.StatusServiceUnavailable} {
		unknown := httptest.NewRecorder()
		router.ServeHTTP(unknown, httptest.NewRequest(http.MethodGet, "/v2/tokens/"+invalidToken+"/metadata", nil))
		if unknown.Code != want {
			t.Fatalf("unknown canonical token %q: %d %s", invalidToken, unknown.Code, unknown.Body.String())
		}
	}
	assetID := uuid.NewString()
	assetPath := "/v2/tokens/" + creation.Token + "/assets/" + assetID
	image := pngBytes(t, 48, 32)
	foreignWallet, foreignBearer := gateway.SignIn(t)
	if foreignWallet == wallet {
		t.Fatal("foreign session has the creator wallet")
	}
	upload := func(body []byte, mime, key string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodPut, assetPath, bytes.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+bearer)
		request.Header.Set("Content-Type", mime)
		request.Header.Set("Idempotency-Key", key)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}
	foreignRequest := httptest.NewRequest(http.MethodPut, assetPath, bytes.NewReader(image))
	foreignRequest.Header.Set("Authorization", "Bearer "+foreignBearer)
	foreignRequest.Header.Set("Content-Type", "image/png")
	foreignRequest.Header.Set("Idempotency-Key", assetID)
	foreignUpload := httptest.NewRecorder()
	router.ServeHTTP(foreignUpload, foreignRequest)
	if foreignUpload.Code != http.StatusForbidden {
		t.Fatalf("foreign creator upload = %d %s", foreignUpload.Code, foreignUpload.Body.String())
	}
	noIdentity := connected
	noIdentity.Identity = nil
	noIdentityRouter := chi.NewRouter()
	noIdentity.RegisterRoutes(noIdentityRouter)
	identityUnavailable := httptest.NewRecorder()
	noIdentityRouter.ServeHTTP(identityUnavailable, httptest.NewRequest(http.MethodPut, assetPath, bytes.NewReader(image)))
	if identityUnavailable.Code != http.StatusServiceUnavailable {
		t.Fatalf("unwired creator identity = %d %s", identityUnavailable.Code, identityUnavailable.Body.String())
	}
	if response := upload(image, "image/png", uuid.NewString()); response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("upload key: %d %s", response.Code, response.Body.String())
	}
	if response := upload(bytes.Repeat([]byte("x"), raster.MaxBytes+1), "image/png", assetID); response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized image: %d %s", response.Code, response.Body.String())
	}
	if response := upload(image, "image/jpeg", assetID); response.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("upload content type: %d %s", response.Code, response.Body.String())
	}
	if response := upload([]byte("not a raster"), "text/plain", assetID); response.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("unsupported raster: %d %s", response.Code, response.Body.String())
	}
	if response := upload(image[:24], "image/png", assetID); response.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("truncated raster: %d %s", response.Code, response.Body.String())
	}
	tooWide := append([]byte(nil), image...)
	binary.BigEndian.PutUint32(tooWide[16:20], 8193)
	binary.BigEndian.PutUint32(tooWide[29:33], crc32.ChecksumIEEE(tooWide[12:29]))
	if response := upload(tooWide, "image/png", assetID); response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("raster dimension limit: %d %s", response.Code, response.Body.String())
	}
	for _, tc := range []struct {
		name   string
		change func(*ConnectedMedia)
	}{
		{"worker unavailable", func(c *ConnectedMedia) { c.Worker = raster.ProcessWorker{} }},
		{"scanner unavailable", func(c *ConnectedMedia) { c.Scanner.Host = "" }},
		{"objects unavailable", func(c *ConnectedMedia) { c.Objects.PublicOrigin = "http://metadata.sidiora.test" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			variant := connected
			tc.change(&variant)
			isolated := chi.NewRouter()
			variant.RegisterRoutes(isolated)
			request := httptest.NewRequest(http.MethodPut, assetPath, bytes.NewReader(image))
			request.Header.Set("Authorization", "Bearer "+bearer)
			request.Header.Set("Content-Type", "image/png")
			request.Header.Set("Idempotency-Key", assetID)
			response := httptest.NewRecorder()
			isolated.ServeHTTP(response, request)
			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("unavailable upload = %d %s", response.Code, response.Body.String())
			}
		})
	}
	if response := upload(image, "image/png", assetID); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), assetID) {
		t.Fatalf("approved token image: %d %s", response.Code, response.Body.String())
	}
	if response := upload(image, "image/png", assetID); response.Code != http.StatusOK {
		t.Fatalf("idempotent token image: %d %s", response.Code, response.Body.String())
	}
	if response := upload(pngBytes(t, 49, 32), "image/png", assetID); response.Code != http.StatusConflict {
		t.Fatalf("changed token image: %d %s", response.Code, response.Body.String())
	}
	decodedImage, err := connected.Worker.Decode(ctx, image)
	if err != nil {
		t.Fatal(err)
	}
	cleanScan := sharedhttp.ScanVerifiedBuffer(ctx, image, connected.Scanner)
	if !cleanScan.Clean || cleanScan.Verdict != sharedhttp.ScanClean {
		t.Fatalf("clean image scan = %+v", cleanScan)
	}
	promotable := *store
	promotable.RasterWorker = connected.Worker
	invalidVariants := map[int][]byte{256: bytes.Clone(decodedImage.Variants[256]), 512: bytes.Clone(decodedImage.Variants[512]), 1024: bytes.Clone(decodedImage.Variants[1024])}
	invalidVariants[512][0] ^= 1
	wrongHash := cleanScan
	wrongHash.ContentHash = strings.Repeat("0", 64)
	for _, tc := range []struct {
		name     string
		store    *ProductionStore
		id       string
		body     []byte
		variants map[int][]byte
		scan     sharedhttp.ScanResult
		want     error
	}{
		{"invalid id", &promotable, "invalid", image, decodedImage.Variants, cleanScan, ErrValidation},
		{"missing variant", &promotable, assetID, image, map[int][]byte{256: decodedImage.Variants[256], 512: decodedImage.Variants[512]}, cleanScan, ErrValidation},
		{"empty variant", &promotable, assetID, image, map[int][]byte{256: decodedImage.Variants[256], 512: nil, 1024: decodedImage.Variants[1024]}, cleanScan, ErrValidation},
		{"unverified original", &promotable, assetID, image, decodedImage.Variants, wrongHash, ErrForbidden},
		{"unavailable scan", &promotable, assetID, image, decodedImage.Variants, sharedhttp.ScanResult{Verdict: sharedhttp.ScanUnavailable}, ErrForbidden},
		{"changed variant", &promotable, assetID, image, invalidVariants, cleanScan, ErrValidation},
		{"missing worker", store, assetID, image, decodedImage.Variants, cleanScan, raster.ErrUnavailable},
	} {
		t.Run(tc.name+" promotion", func(t *testing.T) {
			if err := tc.store.Promote(ctx, tc.id, tc.body, tc.variants, tc.scan, connected.Objects); err != tc.want {
				t.Fatalf("promotion = %v, want %v", err, tc.want)
			}
		})
	}
	if err := promotable.Promote(ctx, uuid.NewString(), image, decodedImage.Variants, cleanScan, connected.Objects); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("unknown image promotion = %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("documents")+" SET blocked=true WHERE pool=$1", creation.Pool); err != nil {
		t.Fatal(err)
	}
	if err := promotable.Promote(ctx, assetID, image, decodedImage.Variants, cleanScan, connected.Objects); err != ErrForbidden {
		t.Fatalf("blocked image promotion = %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("documents")+" SET blocked=false WHERE pool=$1", creation.Pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("assets")+" SET hash=$2 WHERE id=$1", assetID, strings.Repeat("0", 64)); err != nil {
		t.Fatal(err)
	}
	if err := promotable.Promote(ctx, assetID, image, decodedImage.Variants, cleanScan, connected.Objects); err != ErrConflict {
		t.Fatalf("changed image digest = %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("assets")+" SET hash=$2 WHERE id=$1", assetID, cleanScan.ContentHash); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("assets")+" SET state='REJECTED' WHERE id=$1", assetID); err != nil {
		t.Fatal(err)
	}
	if err := promotable.Promote(ctx, assetID, image, decodedImage.Variants, cleanScan, connected.Objects); err != ErrConflict {
		t.Fatalf("rejected image promotion = %v", err)
	}
	if response := upload(image, "image/png", assetID); response.Code != http.StatusForbidden {
		t.Fatalf("rejected token image upload = %d %s", response.Code, response.Body.String())
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("assets")+" SET state='APPROVED' WHERE id=$1", assetID); err != nil {
		t.Fatal(err)
	}
	changedID := uuid.NewString()
	changedBody := pngBytes(t, 61, 32)
	changedHash := sha256.Sum256(changedBody)
	changedRecord := AssetRecord{ID: changedID, Pool: creation.Pool, Hash: hex.EncodeToString(changedHash[:]), Size: int64(len(changedBody)), ObjectKey: "quarantine/" + changedID}
	if err := store.CreateConnectedAsset(ctx, changedRecord, "image/png", 61, 32); err != nil {
		t.Fatal(err)
	}
	for _, transition := range [][2]string{{"CREATED", "UPLOADING"}, {"UPLOADING", "QUARANTINED"}} {
		if err := store.TransitionAsset(ctx, changedID, transition[0], transition[1]); err != nil {
			t.Fatal(err)
		}
	}
	if err := connected.Objects.put(ctx, quarantine.Bucket, changedRecord.ObjectKey, pngBytes(t, 62, 32), "image/png"); err != nil {
		t.Fatal(err)
	}
	changedRequest := httptest.NewRequest(http.MethodPut, "/v2/tokens/"+creation.Token+"/assets/"+changedID, bytes.NewReader(changedBody))
	changedRequest.Header.Set("Authorization", "Bearer "+bearer)
	changedRequest.Header.Set("Content-Type", "image/png")
	changedRequest.Header.Set("Idempotency-Key", changedID)
	changedResponse := httptest.NewRecorder()
	router.ServeHTTP(changedResponse, changedRequest)
	if changedResponse.Code != http.StatusConflict {
		t.Fatalf("changed quarantine object = %d %s", changedResponse.Code, changedResponse.Body.String())
	}
	annotatedID := uuid.NewString()
	cleanPNG := pngBytes(t, 24, 24)
	textChunk := append([]byte("tEXtcomment\x00"), []byte("creator annotation")...)
	chunk := make([]byte, 4, len(textChunk)+8)
	binary.BigEndian.PutUint32(chunk, uint32(len(textChunk)-4))
	chunk = append(chunk, textChunk...)
	checksum := make([]byte, 4)
	binary.BigEndian.PutUint32(checksum, crc32.ChecksumIEEE(textChunk))
	chunk = append(chunk, checksum...)
	annotated := append(append([]byte{}, cleanPNG[:len(cleanPNG)-12]...), chunk...)
	annotated = append(annotated, cleanPNG[len(cleanPNG)-12:]...)
	annotatedRequest := httptest.NewRequest(http.MethodPut, "/v2/tokens/"+creation.Token+"/assets/"+annotatedID, bytes.NewReader(annotated))
	annotatedRequest.Header.Set("Authorization", "Bearer "+bearer)
	annotatedRequest.Header.Set("Content-Type", "image/png")
	annotatedRequest.Header.Set("Idempotency-Key", annotatedID)
	annotatedResponse := httptest.NewRecorder()
	router.ServeHTTP(annotatedResponse, annotatedRequest)
	if annotatedResponse.Code != http.StatusOK || !strings.Contains(annotatedResponse.Body.String(), "approved/") {
		t.Fatalf("annotated token image: %d %s", annotatedResponse.Code, annotatedResponse.Body.String())
	}
	var encodedVariants []byte
	if err := pool.QueryRow(ctx, "SELECT variants FROM "+store.table("asset_details_v2")+" WHERE asset_id=$1", annotatedID).Scan(&encodedVariants); err != nil {
		t.Fatal(err)
	}
	var annotatedVariants map[string]string
	if err := json.Unmarshal(encodedVariants, &annotatedVariants); err != nil {
		t.Fatal(err)
	}
	variant, err := connected.Objects.read(ctx, approved.Bucket, annotatedVariants["512"])
	if err != nil || bytes.Contains(variant, []byte("creator annotation")) {
		t.Fatalf("reencoded variant retains source annotation: %v", err)
	}
	assetRead := httptest.NewRecorder()
	router.ServeHTTP(assetRead, httptest.NewRequest(http.MethodGet, assetPath, nil))
	if assetRead.Code != http.StatusOK || !strings.Contains(assetRead.Body.String(), assetID) {
		t.Fatalf("token image read: %d %s", assetRead.Code, assetRead.Body.String())
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("assets")+" SET state='CREATED' WHERE id=$1", assetID); err != nil {
		t.Fatal(err)
	}
	createdAsset := httptest.NewRecorder()
	router.ServeHTTP(createdAsset, httptest.NewRequest(http.MethodGet, assetPath, nil))
	if createdAsset.Code != http.StatusServiceUnavailable {
		t.Fatalf("unrepresentable asset state = %d %s", createdAsset.Code, createdAsset.Body.String())
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("assets")+" SET state='APPROVED' WHERE id=$1", assetID); err != nil {
		t.Fatal(err)
	}
	var originalVariants []byte
	if err := pool.QueryRow(ctx, "SELECT variants FROM "+store.table("asset_details_v2")+" WHERE asset_id=$1", assetID).Scan(&originalVariants); err != nil {
		t.Fatal(err)
	}
	for name, variants := range map[string]string{
		"missing variants": `{}`,
		"unscoped variant": `{"256":"approved/x/256/x.png","512":"unscoped/image.png","1024":"approved/x/1024/x.png"}`,
	} {
		if _, err := pool.Exec(ctx, "UPDATE "+store.table("asset_details_v2")+" SET variants=$2::jsonb WHERE asset_id=$1", assetID, variants); err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, assetPath, nil))
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s: %d %s", name, response.Code, response.Body.String())
		}
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("asset_details_v2")+" SET variants=$2::jsonb WHERE asset_id=$1", assetID, string(originalVariants)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO "+store.table("documents")+"(pool,blocked) VALUES($1,true) ON CONFLICT(pool) DO UPDATE SET blocked=true", creation.Pool); err != nil {
		t.Fatal(err)
	}
	blockedAsset := httptest.NewRecorder()
	router.ServeHTTP(blockedAsset, httptest.NewRequest(http.MethodGet, assetPath, nil))
	if blockedAsset.Code != http.StatusNotFound {
		t.Fatalf("restricted token image: %d %s", blockedAsset.Code, blockedAsset.Body.String())
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("documents")+" SET blocked=false WHERE pool=$1", creation.Pool); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]int{
		"/v2/tokens/" + creation.Token + "/assets/invalid":               http.StatusUnprocessableEntity,
		"/v2/tokens/" + creation.Token + "/assets/" + uuid.NewString():   http.StatusNotFound,
		"/v2/tokens/0x" + strings.Repeat("f", 40) + "/assets/" + assetID: http.StatusNotFound,
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != want {
			t.Fatalf("asset path %s: %d %s", path, response.Code, response.Body.String())
		}
	}
	withLogo, err := json.Marshal(map[string]any{"expectedRevision": "1", "logoAssetId": assetID})
	if err != nil {
		t.Fatal(err)
	}
	if response := patch(withLogo, uuid.NewString()); response.Code != http.StatusOK {
		t.Fatalf("image publication: %d %s", response.Code, response.Body.String())
	}
	mediaPath := "/v2/tokens/" + creation.Token + "/media"
	mediaRead := httptest.NewRecorder()
	router.ServeHTTP(mediaRead, httptest.NewRequest(http.MethodGet, mediaPath, nil))
	if mediaRead.Code != http.StatusOK || !strings.Contains(mediaRead.Body.String(), assetID) || !strings.Contains(mediaRead.Body.String(), "approved/") {
		t.Fatalf("connected media: %d %s", mediaRead.Code, mediaRead.Body.String())
	}
	cachedRequest := httptest.NewRequest(http.MethodGet, mediaPath, nil)
	cachedRequest.Header.Set("If-None-Match", mediaRead.Header().Get("ETag"))
	cached := httptest.NewRecorder()
	router.ServeHTTP(cached, cachedRequest)
	if cached.Code != http.StatusNotModified {
		t.Fatalf("conditional media read: %d %s", cached.Code, cached.Body.String())
	}
	missing := httptest.NewRecorder()
	router.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/v2/tokens/0x"+strings.Repeat("9", 40)+"/media", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("unknown token media: %d %s", missing.Code, missing.Body.String())
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("asset_details_v2")+" SET variants='{}'::jsonb WHERE asset_id=$1", assetID); err != nil {
		t.Fatal(err)
	}
	badVariants := httptest.NewRecorder()
	router.ServeHTTP(badVariants, httptest.NewRequest(http.MethodGet, mediaPath, nil))
	if badVariants.Code != http.StatusServiceUnavailable {
		t.Fatalf("unrepresentable variant set: %d %s", badVariants.Code, badVariants.Body.String())
	}
	for _, tc := range []struct {
		name, variants string
	}{
		{"unscoped media variant", `{"256":"approved/other/256/image.png","512":"approved/other/512/image.png","1024":"approved/other/1024/image.png"}`},
		{"missing media variant", `{"256":"approved/other/256/image.png","512":"","1024":"approved/other/1024/image.png"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := pool.Exec(ctx, "UPDATE "+store.table("asset_details_v2")+" SET variants=$2::jsonb WHERE asset_id=$1", assetID, tc.variants); err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, mediaPath, nil))
			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("unsafe media variant = %d %s", response.Code, response.Body.String())
			}
		})
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("asset_details_v2")+" SET variants=$2::jsonb WHERE asset_id=$1", assetID, string(originalVariants)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("assets")+" SET state='REJECTED' WHERE id=$1", assetID); err != nil {
		t.Fatal(err)
	}
	noApprovedImage := httptest.NewRecorder()
	router.ServeHTTP(noApprovedImage, httptest.NewRequest(http.MethodGet, mediaPath, nil))
	if noApprovedImage.Code != http.StatusServiceUnavailable || !strings.Contains(noApprovedImage.Body.String(), `"code":"unavailable"`) {
		t.Fatalf("unapproved media image = %d %s", noApprovedImage.Code, noApprovedImage.Body.String())
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("assets")+" SET state='APPROVED' WHERE id=$1", assetID); err != nil {
		t.Fatal(err)
	}
	unwired := chi.NewRouter()
	(ConnectedMedia{}).RegisterRoutes(unwired)
	unavailable := httptest.NewRecorder()
	unwired.ServeHTTP(unavailable, httptest.NewRequest(http.MethodGet, mediaPath, nil))
	if unavailable.Code != http.StatusServiceUnavailable {
		t.Fatalf("unwired media = %d %s", unavailable.Code, unavailable.Body.String())
	}
	rejectedID := uuid.NewString()
	rejectedImage := pngBytes(t, 24, 24)
	rejectedHash := sha256.Sum256(rejectedImage)
	rejected := AssetRecord{ID: rejectedID, Pool: creation.Pool, Hash: hex.EncodeToString(rejectedHash[:]), Size: int64(len(rejectedImage)), ObjectKey: "quarantine/" + rejectedID}
	if err := store.CreateConnectedAsset(ctx, rejected, "image/png", 24, 24); err != nil {
		t.Fatal(err)
	}
	if err := connected.Objects.put(ctx, quarantine.Bucket, rejected.ObjectKey, rejectedImage, "image/png"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("assets")+" SET state='REJECTED' WHERE id=$1", rejectedID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("asset_details_v2")+" SET rejected_until=clock_timestamp()-interval '1 hour' WHERE asset_id=$1", rejectedID); err != nil {
		t.Fatal(err)
	}
	if err := connected.ExpireRejected(ctx); err != nil {
		t.Fatalf("rejected image retention: %v", err)
	}
	var rejectedState string
	if err := pool.QueryRow(ctx, "SELECT state FROM "+store.table("assets")+" WHERE id=$1", rejectedID).Scan(&rejectedState); err != nil || rejectedState != "DELETED" {
		t.Fatalf("rejected image state %q: %v", rejectedState, err)
	}
	missingModeration := connected
	missingModeration.Moderation = PostgresVisibilityAuthority{Pool: pool, Schema: "missing_visibility"}
	moderationRouter := chi.NewRouter()
	missingModeration.RegisterRoutes(moderationRouter)
	for _, route := range []string{path, assetPath, mediaPath} {
		response := httptest.NewRecorder()
		moderationRouter.ServeHTTP(response, httptest.NewRequest(http.MethodGet, route, nil))
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("unavailable moderation on %s = %d %s", route, response.Code, response.Body.String())
		}
	}
	moderationID := uuid.NewString()
	moderationRequest := httptest.NewRequest(http.MethodPut, "/v2/tokens/"+creation.Token+"/assets/"+moderationID, bytes.NewReader(pngBytes(t, 68, 32)))
	moderationRequest.Header.Set("Authorization", "Bearer "+bearer)
	moderationRequest.Header.Set("Content-Type", "image/png")
	moderationRequest.Header.Set("Idempotency-Key", moderationID)
	moderationResponse := httptest.NewRecorder()
	moderationRouter.ServeHTTP(moderationResponse, moderationRequest)
	if moderationResponse.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable moderation during upload = %d %s", moderationResponse.Code, moderationResponse.Body.String())
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE "+store.table("assets")+" ADD CONSTRAINT checked_approval CHECK(state <> 'APPROVED') NOT VALID"); err != nil {
		t.Fatal(err)
	}
	resumableID := uuid.NewString()
	resumablePath := "/v2/tokens/" + creation.Token + "/assets/" + resumableID
	resumableBody := pngBytes(t, 69, 32)
	resumableUpload := func() *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPut, resumablePath, bytes.NewReader(resumableBody))
		request.Header.Set("Authorization", "Bearer "+bearer)
		request.Header.Set("Content-Type", "image/png")
		request.Header.Set("Idempotency-Key", resumableID)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}
	if response := resumableUpload(); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable approval commit = %d %s", response.Code, response.Body.String())
	}
	var resumableState string
	if err := pool.QueryRow(ctx, "SELECT state FROM "+store.table("assets")+" WHERE id=$1", resumableID).Scan(&resumableState); err != nil || resumableState != "VALIDATING" {
		t.Fatalf("private image after approval refusal = %q, %v", resumableState, err)
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE "+store.table("assets")+" DROP CONSTRAINT checked_approval"); err != nil {
		t.Fatal(err)
	}
	if response := resumableUpload(); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), resumableID) {
		t.Fatalf("resumed image approval = %d %s", response.Code, response.Body.String())
	}
	uploadThrough := func(media ConnectedMedia, id string, body []byte) *httptest.ResponseRecorder {
		t.Helper()
		isolated := chi.NewRouter()
		media.RegisterRoutes(isolated)
		request := httptest.NewRequest(http.MethodPut, "/v2/tokens/"+creation.Token+"/assets/"+id, bytes.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+bearer)
		request.Header.Set("Content-Type", "image/png")
		request.Header.Set("Idempotency-Key", id)
		response := httptest.NewRecorder()
		isolated.ServeHTTP(response, request)
		return response
	}
	unavailableBucket := connected
	unavailableBucket.Objects.QuarantineBucket = "missing-quarantine-bucket"
	if response := uploadThrough(unavailableBucket, uuid.NewString(), pngBytes(t, 70, 32)); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable quarantine bucket = %d %s", response.Code, response.Body.String())
	}
	missingObjectID := uuid.NewString()
	missingObjectBody := pngBytes(t, 71, 32)
	missingObjectHash := sha256.Sum256(missingObjectBody)
	missingObject := AssetRecord{ID: missingObjectID, Pool: creation.Pool, Hash: hex.EncodeToString(missingObjectHash[:]), Size: int64(len(missingObjectBody)), ObjectKey: "quarantine/" + missingObjectID}
	if err := store.CreateConnectedAsset(ctx, missingObject, "image/png", 71, 32); err != nil {
		t.Fatal(err)
	}
	for _, transition := range [][2]string{{"CREATED", "UPLOADING"}, {"UPLOADING", "QUARANTINED"}} {
		if err := store.TransitionAsset(ctx, missingObjectID, transition[0], transition[1]); err != nil {
			t.Fatal(err)
		}
	}
	if response := uploadThrough(connected, missingObjectID, missingObjectBody); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing quarantine object = %d %s", response.Code, response.Body.String())
	}
	for _, tc := range []struct {
		state, retained string
	}{
		{"UPLOADING", "CREATED"},
		{"QUARANTINED", "UPLOADING"},
		{"VALIDATING", "QUARANTINED"},
	} {
		t.Run(tc.state+" transition unavailable", func(t *testing.T) {
			constraint := "reject_" + strings.ToLower(tc.state)
			if _, err := pool.Exec(ctx, "ALTER TABLE "+store.table("assets")+" ADD CONSTRAINT "+constraint+" CHECK(state <> '"+tc.state+"') NOT VALID"); err != nil {
				t.Fatal(err)
			}
			id := uuid.NewString()
			if response := uploadThrough(connected, id, pngBytes(t, 73+len(tc.state), 32)); response.Code != http.StatusServiceUnavailable {
				t.Fatalf("restricted state transition = %d %s", response.Code, response.Body.String())
			}
			var state string
			if err := pool.QueryRow(ctx, "SELECT state FROM "+store.table("assets")+" WHERE id=$1", id).Scan(&state); err != nil || state != tc.retained {
				t.Fatalf("private retained state = %q, %v", state, err)
			}
			if _, err := pool.Exec(ctx, "ALTER TABLE "+store.table("assets")+" DROP CONSTRAINT "+constraint); err != nil {
				t.Fatal(err)
			}
		})
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE "+store.table("assets")+" RENAME TO assets_hidden"); err != nil {
		t.Fatal(err)
	}
	if response := uploadThrough(connected, uuid.NewString(), pngBytes(t, 74, 32)); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable asset inventory = %d %s", response.Code, response.Body.String())
	}
	unavailableAsset := httptest.NewRecorder()
	connected.respondAsset(unavailableAsset, httptest.NewRequest(http.MethodGet, "/v2/tokens/"+creation.Token+"/assets/"+assetID, nil), assetID, http.StatusOK)
	if unavailableAsset.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable asset read = %d %s", unavailableAsset.Code, unavailableAsset.Body.String())
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE "+store.table("assets_hidden")+" RENAME TO assets"); err != nil {
		t.Fatal(err)
	}
	canonical := PostgresCanonicalSource{Schema: "metadata_patch_canonical", CanonicalStreamID: stream}
	for _, tc := range []struct {
		name, statement string
		want            error
	}{
		{"missing creation", "DELETE FROM metadata_patch_canonical.indexer_canonical_creations_v1 WHERE token=$1", ErrMissing},
		{"unrepresentable creation", "UPDATE metadata_patch_canonical.indexer_canonical_creations_v1 SET payload='{}'::jsonb WHERE token=$1", ErrUnrepresentable},
		{"unrepresentable provenance", "UPDATE metadata_patch_canonical.indexer_canonical_creations_v1 SET provenance='{}'::jsonb WHERE token=$1", ErrUnrepresentable},
		{"incomplete block", "UPDATE metadata_patch_canonical.indexer_block_completeness_v1 SET fetch_complete=false", ErrUnrepresentable},
		{"missing log topics", "UPDATE metadata_patch_canonical.indexer_raw_logs_v1 SET topics='[]'::jsonb", ErrUnrepresentable},
		{"missing log data", "UPDATE metadata_patch_canonical.indexer_raw_logs_v1 SET data=decode('','hex')", ErrUnrepresentable},
		{"different log emitter", "UPDATE metadata_patch_canonical.indexer_raw_logs_v1 SET emitter='0x9999999999999999999999999999999999999999'", ErrUnrepresentable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			args := []any{}
			if strings.Contains(tc.statement, "$1") {
				args = append(args, creation.Token)
			}
			if _, err := tx.Exec(ctx, tc.statement, args...); err != nil {
				t.Fatal(err)
			}
			_, err = canonical.ResolveTokenInTx(ctx, tx, creation.Token)
			if !errors.Is(err, tc.want) {
				t.Fatalf("canonical creation = %v, want %v", err, tc.want)
			}
		})
	}
	if _, err := canonical.ResolveTokenInTx(ctx, nil, creation.Token); !errors.Is(err, ErrUnrepresentable) {
		t.Fatalf("missing canonical transaction = %v", err)
	}
	if _, err := pool.Exec(ctx, "DROP SCHEMA metadata_patch_document CASCADE"); err != nil {
		t.Fatal(err)
	}
	storeRead := httptest.NewRecorder()
	router.ServeHTTP(storeRead, httptest.NewRequest(http.MethodGet, path, nil))
	if storeRead.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable metadata store: %d %s", storeRead.Code, storeRead.Body.String())
	}
	for _, requestPath := range []string{mediaPath, assetPath} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, requestPath, nil))
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("unavailable media store %s: %d %s", requestPath, response.Code, response.Body.String())
		}
	}
	if err := connected.ExpireRejected(ctx); err == nil {
		t.Fatal("unavailable rejected inventory accepted")
	}
}
