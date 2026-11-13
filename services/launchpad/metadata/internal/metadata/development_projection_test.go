package metadata

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/db/sqlcdb"
	raster "github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/image"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestDevelopmentLegacyProjectionTracksPublishedRevisions(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.NewPostgres(t)
	if err := EnsureDevelopmentConnectedSchema(ctx, pool, DevelopmentConnectedSchema); err != nil {
		t.Fatal(err)
	}
	store, err := NewProductionStore(pool, DevelopmentConnectedSchema, nil)
	if err != nil {
		t.Fatal(err)
	}
	client, objects := edgeObjects(t, "metadata-development-projection")
	media := ConnectedMedia{Metadata: V2ContractHandlers{Store: store}, Objects: S3AssetObjects{Client: client, PublicBucket: objects.Bucket}}
	legacy := storage.NewFromS3(client, objects.Bucket)
	queries := sqlcdb.New(pool)
	address := "0x" + strings.Repeat("a", 40)
	poolAddress := "0x" + strings.Repeat("b", 40)
	creator := "0x" + strings.Repeat("c", 40)
	if _, err := pool.Exec(ctx, "INSERT INTO "+store.table("pools")+"(pool,creator,token) VALUES($1,$2,$3)", poolAddress, creator, address); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO "+store.table("v2_documents")+"(pool,revision,document) VALUES($1,1,$2::jsonb)", poolAddress, `{"name":"First name","symbol":"ONE","description":"First description","tags":["art"],"socials":[]}`); err != nil {
		t.Fatal(err)
	}
	clock := func() time.Time { return time.UnixMilli(1760000000000) }
	if err := media.ProjectDevelopmentLegacy(ctx, queries, legacy, clock); err != nil {
		t.Fatal(err)
	}
	first, err := queries.GetTokenMetadata(ctx, address)
	if err != nil || first.Name.String != "First name" || first.Symbol.String != "ONE" || first.Description.String != "First description" || first.CreatedBy != creator || first.CustomTags.String != `["art"]` {
		t.Fatalf("first projection = %+v, %v", first, err)
	}
	if err := media.ProjectDevelopmentLegacy(ctx, queries, legacy, clock); err != nil {
		t.Fatal(err)
	}
	unchanged, err := queries.GetTokenMetadata(ctx, address)
	if err != nil || unchanged.UpdatedAt != first.UpdatedAt {
		t.Fatalf("unchanged projection = %+v, %v", unchanged, err)
	}
	image := pngBytes(t, 16, 16)
	hash := sha256.Sum256(image)
	id := uuid.NewString()
	record := AssetRecord{ID: id, Pool: poolAddress, Hash: hex.EncodeToString(hash[:]), Size: int64(len(image)), ObjectKey: "quarantine/" + id}
	if err := store.CreateConnectedAsset(ctx, record, "image/png", 16, 16); err != nil {
		t.Fatal(err)
	}
	key := "approved/" + record.Hash + "/512/" + record.Hash + ".png"
	if err := media.Objects.put(ctx, objects.Bucket, key, image, "image/png"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("assets")+" SET state='APPROVED' WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("asset_details_v2")+" SET variants=$2::jsonb WHERE asset_id=$1", id, `{"512":"`+key+`"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("v2_documents")+" SET revision=2,document=$2::jsonb,asset_id=$3 WHERE pool=$1", poolAddress, `{"name":"Second name","symbol":"TWO","description":"Second description","tags":[],"socials":[{"label":"site","url":"https://example.com"}]}`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("v2_documents")+" SET asset_id=$2 WHERE pool=$1", poolAddress, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if err := media.ProjectDevelopmentLegacy(ctx, queries, legacy, clock); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing approved image = %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("v2_documents")+" SET asset_id=$2 WHERE pool=$1", poolAddress, id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("asset_details_v2")+" SET variants='{}'::jsonb WHERE asset_id=$1", id); err != nil {
		t.Fatal(err)
	}
	if err := media.ProjectDevelopmentLegacy(ctx, queries, legacy, clock); !errors.Is(err, ErrUnrepresentable) {
		t.Fatalf("unrepresentable approved image = %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("asset_details_v2")+" SET variants=$2::jsonb WHERE asset_id=$1", id, `{"512":"`+key+`"}`); err != nil {
		t.Fatal(err)
	}
	missingObjects := media
	missingObjects.Objects.PublicBucket = "missing-projection-bucket"
	if err := missingObjects.ProjectDevelopmentLegacy(ctx, queries, legacy, clock); !errors.Is(err, raster.ErrUnavailable) {
		t.Fatalf("unavailable approved image = %v", err)
	}
	if err := media.ProjectDevelopmentLegacy(ctx, queries, storage.NewFromS3(client, "missing-projection-bucket"), clock); err == nil {
		t.Fatal("unavailable legacy bucket accepted the image")
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE metadata.token_metadata RENAME TO token_metadata_unavailable"); err != nil {
		t.Fatal(err)
	}
	if err := media.ProjectDevelopmentLegacy(ctx, queries, legacy, clock); err == nil {
		t.Fatal("unavailable legacy metadata table accepted the projection")
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE metadata.token_metadata_unavailable RENAME TO token_metadata"); err != nil {
		t.Fatal(err)
	}
	if err := media.ProjectDevelopmentLegacy(ctx, queries, legacy, clock); err != nil {
		t.Fatal(err)
	}
	second, err := queries.GetTokenMetadata(ctx, address)
	if err != nil || second.Name.String != "Second name" || second.CreatedAt != first.CreatedAt || second.UpdatedAt <= first.UpdatedAt || second.Website.String != "https://example.com" {
		t.Fatalf("updated projection = %+v, %v", second, err)
	}
	logos, err := queries.GetTokenImages(ctx, address)
	if err != nil || len(logos) != 1 || logos[0].StorageKey != "logos/logo-"+address+".png" || logos[0].SizeBytes != int32(len(image)) {
		t.Fatalf("projected logo = %+v, %v", logos, err)
	}
	object, err := legacy.Get(ctx, logos[0].StorageKey)
	if err != nil {
		t.Fatal(err)
	}
	defer object.Body.Close()
	if object.Size != int64(len(image)) {
		t.Fatalf("projected image size = %d", object.Size)
	}
}
