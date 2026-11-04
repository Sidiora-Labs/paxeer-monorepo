package metadata

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/db/sqlcdb"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/image"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/pkg/types"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// LegacyPublicationDDL belongs to the serial migration owner, never the runtime role.
const LegacyPublicationDDL = `CREATE TABLE metadata.legacy_publications(
 operation uuid PRIMARY KEY, token text NOT NULL, state text NOT NULL CHECK(state IN('staging','published')),
 staged jsonb NOT NULL, retired jsonb NOT NULL DEFAULT '[]', created_at timestamptz NOT NULL DEFAULT clock_timestamp());`

func (h *Handlers) publishLegacy(ctx context.Context, addr, pool, wallet string, meta *types.UploadMetadata, uploads []imageUpload, base string) (types.UploadResponse, error) {
	var response types.UploadResponse
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	conn, err := h.db.Acquire(ctx)
	if err != nil {
		return response, err
	}
	defer conn.Release()
	// A session lock spans the durable intent, object writes and pointer commit.
	// Recovery takes the same lock before deleting any staged object.
	if _, err = conn.Exec(ctx, "SELECT pg_advisory_lock(hashtextextended($1,1007))", addr); err != nil {
		return response, err
	}
	defer func() {
		bounded, stop := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer stop()
		if _, e := conn.Exec(bounded, "SELECT pg_advisory_unlock(hashtextextended($1,1007))", addr); e != nil {
			_ = conn.Conn().Close(bounded)
		}
	}()
	operation := uuid.NewString()
	keys := make([]string, len(uploads))
	for i, up := range uploads {
		keys[i] = fmt.Sprintf("legacy/%s/%s/%x.%s", operation, up.imageType, sha256.Sum256(up.data), image.ExtForMime(up.mime))
	}
	staged, err := json.Marshal(keys)
	if err != nil {
		return response, err
	}
	if _, err = conn.Exec(ctx, "INSERT INTO metadata.legacy_publications(operation,token,state,staged) VALUES($1,$2,'staging',$3::jsonb)", operation, addr, string(staged)); err != nil {
		return response, err
	}
	for i, up := range uploads {
		if err = h.store.Put(ctx, keys[i], bytes.NewReader(up.data), int64(len(up.data)), up.mime); err != nil {
			return response, err
		}
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return response, err
	}
	defer rollbackV2(ctx, tx)
	writer := *h
	writer.q = sqlcdb.New(tx)
	now := h.clock().UnixMilli()
	retired := []string{}
	for i, up := range uploads {
		old, e := writer.q.GetImageByType(ctx, sqlcdb.GetImageByTypeParams{TokenAddress: addr, ImageType: up.imageType})
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return response, e
		}
		if old.StorageKey != "" {
			retired = append(retired, old.StorageKey)
		}
		if err = writer.q.UpsertTokenImage(ctx, sqlcdb.UpsertTokenImageParams{ID: addr + "-" + up.imageType, TokenAddress: addr, ImageType: up.imageType, StorageKey: keys[i], MimeType: up.mime, SizeBytes: int32(len(up.data)), UploadedAt: now}); err != nil {
			return response, err
		}
		url := base + "/" + up.imageType + "/" + addr + "." + image.ExtForMime(up.mime)
		if up.imageType == image.TypeLogo {
			response.LogoURL = &url
		} else {
			response.BannerURL = &url
		}
	}
	if err = writer.recordMetadata(ctx, addr, pool, wallet, meta, now); err != nil {
		return response, err
	}
	raw, err := json.Marshal(retired)
	if err != nil {
		return response, err
	}
	tag, err := tx.Exec(ctx, "UPDATE metadata.legacy_publications SET state='published',retired=$2::jsonb WHERE operation=$1 AND state='staging'", operation, string(raw))
	if err != nil {
		return response, err
	}
	if tag.RowsAffected() != 1 {
		return response, ErrConflict
	}
	if err = tx.Commit(ctx); err != nil {
		return response, err
	}
	response.Success, response.MetadataUpdated = true, meta != nil
	return response, nil
}

// RecoverLegacyPublications is run on startup and periodically by the legacy
// runtime owner. Failed operations remove only their private staged objects;
// published operations retire replaced objects after readers have drained.
func (h *Handlers) RecoverLegacyPublications(ctx context.Context) error {
	if h.db == nil || h.store == nil {
		return ErrMissing
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	rows, err := h.db.Query(ctx, "SELECT operation::text,token FROM metadata.legacy_publications WHERE state='staging' OR created_at<clock_timestamp()-interval '1 hour' ORDER BY created_at LIMIT 100")
	if err != nil {
		return err
	}
	type item struct{ operation, token string }
	var items []item
	for rows.Next() {
		var v item
		if err = rows.Scan(&v.operation, &v.token); err != nil {
			rows.Close()
			return err
		}
		items = append(items, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, v := range items {
		if err = h.recoverLegacyPublication(ctx, v.operation, v.token); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handlers) recoverLegacyPublication(ctx context.Context, operation, token string) error {
	tx, err := h.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollbackV2(ctx, tx)
	var locked bool
	if err = tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock(hashtextextended($1,1007))", token).Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return nil
	}
	var raw []byte
	err = tx.QueryRow(ctx, "SELECT CASE WHEN state='staging' THEN staged ELSE retired END FROM metadata.legacy_publications WHERE operation=$1 AND (state='staging' OR created_at<clock_timestamp()-interval '1 hour') FOR UPDATE", operation).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var keys []string
	if err = json.Unmarshal(raw, &keys); err != nil {
		return err
	}
	for _, key := range keys {
		var referenced bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM metadata.token_images WHERE storage_key=$1)", key).Scan(&referenced); err != nil {
			return err
		}
		if referenced {
			return ErrConflict
		}
		if err = h.store.Delete(ctx, key); err != nil && !errors.Is(err, storage.ErrNotFound) {
			return err
		}
	}
	if _, err = tx.Exec(ctx, "DELETE FROM metadata.legacy_publications WHERE operation=$1", operation); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
