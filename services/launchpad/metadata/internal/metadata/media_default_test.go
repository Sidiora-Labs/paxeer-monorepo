package metadata

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/internaltest"
	iface "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestMediaReadDefaultCanonical(t *testing.T) {
	ctx := context.Background()
	pool, _ := internaltest.NewPostgresURL(t)
	for _, schema := range []string{"media_default_canonical", "media_default_document"} {
		if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
			t.Fatal(err)
		}
	}
	key := publicationRecoveryKey(t)
	creation := publicationRecoveryCreator(loadCreationFixture(t), key)
	const stream = "paxeer125-canonical"
	seedCanonical(t, pool, "media_default_canonical", stream, creation)
	store, err := NewProductionStore(pool, "media_default_document", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, store.SchemaDDL()+store.V2SchemaDDL()+store.ConnectedSchemaDDL()); err != nil {
		t.Fatal(err)
	}
	const audience = "https://metadata.sidiora.test"
	store.Verifier = V2SignedVerifier{Store: store, Audience: audience}
	connected := ConnectedMedia{
		Metadata:   V2ContractHandlers{Store: store, Canonical: PostgresCanonicalSource{Schema: "media_default_canonical", CanonicalStreamID: stream}},
		Moderation: DevelopmentVisibility{},
	}
	router := chi.NewRouter()
	connected.RegisterRoutes(router)
	path := "/v2/tokens/" + creation.Token
	read := func(route string) *httptest.ResponseRecorder {
		t.Helper()
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, route, nil))
		return response
	}
	assertDocument := func(t *testing.T, revision, description string) {
		t.Helper()
		metadata := read(path + "/metadata")
		media := read(path + "/media")
		if metadata.Code != http.StatusOK || media.Code != http.StatusOK {
			t.Fatalf("metadata = %d %s; media = %d %s", metadata.Code, metadata.Body.String(), media.Code, media.Body.String())
		}
		if err := iface.ValidateJSON("MetadataGetResponse", metadata.Body.Bytes()); err != nil {
			t.Fatal(err)
		}
		if err := iface.ValidateJSON("ConnectedTokenMediaResponse", media.Body.Bytes()); err != nil {
			t.Fatalf("media contract: %v, %s", err, media.Body.String())
		}
		var document iface.MetadataGetResponse
		var result iface.ConnectedTokenMediaResponse
		if err := json.Unmarshal(metadata.Body.Bytes(), &document); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(media.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		m := result.Data.Metadata
		if m.ChainId != 125 || m.PoolAddress != creation.Pool || m.TokenAddress != creation.Token || m.Revision != revision || string(document.Data.Revision) != revision || m.Display.Description != description || m.Display.Description != document.Data.Description || !reflect.DeepEqual(m.Display.Name, document.Data.DisplayName) || !reflect.DeepEqual(m.Display.Symbol, document.Data.DisplaySymbol) || m.Display.Tags == nil || !reflect.DeepEqual(*m.Display.Tags, document.Data.Tags) || m.Display.Socials == nil {
			t.Fatalf("media document = %+v, metadata = %+v", m, document.Data)
		}
		links, err := json.Marshal(*m.Display.Socials)
		if err != nil {
			t.Fatal(err)
		}
		expectedLinks, err := json.Marshal(document.Data.SocialLinks)
		if err != nil || string(links) != string(expectedLinks) {
			t.Fatalf("socials = %s, want %s (%v)", links, expectedLinks, err)
		}
		if result.Data.Image != nil || m.LogoAssetId != nil || m.Source != "creator" || m.ModerationStatus != "allowed" {
			t.Fatalf("media = %+v", result.Data)
		}
		if revision == "0" && (m.UpdatedAt != nil || m.Display.Name != nil || m.Display.Symbol != nil || len(*m.Display.Tags) != 0 || len(*m.Display.Socials) != 0) {
			t.Fatalf("default document = %+v", m)
		}
	}
	t.Run("canonical creation before a local pool", func(t *testing.T) {
		assertDocument(t, "0", "")
		var count int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+store.table("v2_documents")).Scan(&count); err != nil || count != 0 {
			t.Fatalf("stored documents = %d (%v)", count, err)
		}
	})
	if _, err := pool.Exec(ctx, "INSERT INTO "+store.table("pools")+"(pool,creator,token) VALUES($1,$2,$3)", creation.Pool, creation.Creator, creation.Token); err != nil {
		t.Fatal(err)
	}
	t.Run("local pool before a document", func(t *testing.T) { assertDocument(t, "0", "") })
	t.Run("moderation applies before a document", func(t *testing.T) {
		if err := store.Restrict(ctx, creation.Pool, "1", true); err != nil {
			t.Fatal(err)
		}
		for _, suffix := range []string{"/metadata", "/media"} {
			if response := read(path + suffix); response.Code != http.StatusForbidden {
				t.Fatalf("blocked %s = %d %s", suffix, response.Code, response.Body.String())
			}
		}
		if err := store.Restrict(ctx, creation.Pool, "2", false); err != nil {
			t.Fatal(err)
		}
		unavailable := connected
		unavailable.Moderation = nil
		r := chi.NewRouter()
		unavailable.RegisterRoutes(r)
		response := httptest.NewRecorder()
		r.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path+"/media", nil))
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("unavailable moderation = %d %s", response.Code, response.Body.String())
		}
	})
	t.Run("first document", func(t *testing.T) {
		raw := `{"expectedRevision":"0","displayName":"Falcon","displaySymbol":"FAL","description":"Creator document","tags":["art"],"socialLinks":[{"label":"Website","url":"https://falcon.example"}]}`
		hash := sha256.Sum256([]byte(raw))
		action := VerifiedAction{Wallet: creation.Creator, OperationID: uuid.NewString(), Method: http.MethodPatch, Path: path + "/metadata", BodyHash: hex.EncodeToString(hash[:]), Nonce: hex.EncodeToString(hash[:]), ExpiresAt: strconv.FormatInt(time.Now().Add(time.Minute).Unix(), 10), Audience: audience, ChainID: 125}
		signature, err := crypto.Sign(accounts.TextHash([]byte(V2SigningMessage(action))), key)
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodPatch, action.Path, strings.NewReader(raw))
		for name, value := range map[string]string{"Content-Type": "application/json", "X-Action-Wallet": action.Wallet, "Idempotency-Key": action.OperationID, "X-Action-Signature": "0x" + hex.EncodeToString(signature), "X-Action-Nonce": action.Nonce, "X-Action-Expires": action.ExpiresAt, "X-Action-Audience": audience} {
			request.Header.Set(name, value)
		}
		direct := chi.NewRouter()
		connected.Metadata.RegisterRoutes(direct)
		response := httptest.NewRecorder()
		direct.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("first document = %d %s", response.Code, response.Body.String())
		}
		assertDocument(t, "1", "Creator document")
	})
	t.Run("unknown canonical token", func(t *testing.T) {
		for _, suffix := range []string{"/metadata", "/media"} {
			response := read("/v2/tokens/0x" + strings.Repeat("9", 40) + suffix)
			if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), `"code":"not_found"`) {
				t.Fatalf("unknown token = %d %s", response.Code, response.Body.String())
			}
		}
	})
	t.Run("different stored pool", func(t *testing.T) {
		other := "0x" + strings.Repeat("8", 40)
		if _, err := pool.Exec(ctx, "UPDATE "+store.table("pools")+" SET token=$2 WHERE pool=$1", creation.Pool, "0x"+strings.Repeat("7", 40)); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, "INSERT INTO "+store.table("pools")+"(pool,creator,token) VALUES($1,$2,$3)", other, creation.Creator, creation.Token); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, "INSERT INTO "+store.table("v2_documents")+"(pool) VALUES($1)", other); err != nil {
			t.Fatal(err)
		}
		if response := read(path + "/media"); response.Code != http.StatusServiceUnavailable {
			t.Fatalf("different pool = %d %s", response.Code, response.Body.String())
		}
	})
}
