package account

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type BackfillRange struct {
	ID    string
	After string
	Limit int
}
type BackfillCounts struct {
	Scanned        int    `json:"scanned"`
	Linked         int    `json:"linked"`
	Quarantined    int    `json:"quarantined"`
	Verified       int    `json:"verified"`
	WatchlistCount int    `json:"watchlistCount"`
	LastWallet     string `json:"lastWallet"`
	Checksum       string `json:"checksum"`
}

// Keep complete legacy rows in the source digest, including image storage
// metadata and timestamps; a field cannot disappear from migration accounting.
func legacyProfileInventory(ctx context.Context, tx pgx.Tx, wallet string) (profiles, images []json.RawMessage, err error) {
	profiles = []json.RawMessage{}
	images = []json.RawMessage{}
	for i, query := range []string{
		`SELECT to_jsonb(p) FROM users.user_profiles p WHERE lower(wallet_address)=$1 ORDER BY wallet_address FOR SHARE`,
		`SELECT to_jsonb(i) FROM users.user_images i WHERE lower(wallet_address)=$1 ORDER BY image_type,id FOR SHARE`,
	} {
		rows, e := tx.Query(ctx, query, wallet)
		if e != nil {
			return nil, nil, e
		}
		for rows.Next() {
			var raw []byte
			if e = rows.Scan(&raw); e != nil {
				rows.Close()
				return nil, nil, e
			}
			if i == 0 {
				profiles = append(profiles, json.RawMessage(raw))
			} else {
				images = append(images, json.RawMessage(raw))
			}
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, nil, e
		}
	}
	return profiles, images, nil
}

type legacyWatch struct {
	Pool    string `json:"pool"`
	AddedAt int64  `json:"addedAt"`
}

func legacyWatchInventory(ctx context.Context, tx pgx.Tx, wallet string) ([]legacyWatch, error) {
	rows, e := tx.Query(ctx, `SELECT lower(pool_address),min(added_at) FROM users.watchlists WHERE lower(wallet_address)=$1 GROUP BY lower(pool_address) ORDER BY lower(pool_address)`, wallet)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	watchlist := []legacyWatch{}
	for rows.Next() {
		var watch legacyWatch
		if e = rows.Scan(&watch.Pool, &watch.AddedAt); e != nil {
			return nil, e
		}
		watchlist = append(watchlist, watch)
	}
	return watchlist, rows.Err()
}

func legacySourceBytes(wallet string, profiles, images []json.RawMessage, watches []legacyWatch) ([]byte, error) {
	return json.Marshal(map[string]any{"wallet": wallet, "profiles": profiles, "images": images, "watchlist": watches})
}

func legacyProfileProjection(ctx context.Context, wallet string, raw json.RawMessage, images []json.RawMessage) (Profile, error) {
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(raw, &fields); e != nil {
		return Profile{}, e
	}
	profile := Profile{PublicProfile: PublicProfile{Wallet: wallet}, Socials: map[string]string{}}
	for key, destination := range map[string]*string{"display_name": &profile.DisplayName, "bio": &profile.Bio} {
		if e := json.Unmarshal(fields[key], destination); e != nil {
			return profile, e
		}
	}
	patch := ProfilePatch{ExpectedRevision: "0", DisplayName: &profile.DisplayName, Bio: &profile.Bio}
	if e := patch.Validate(); e != nil {
		return profile, e
	}
	if patch.DisplayName != nil {
		profile.DisplayName = *patch.DisplayName
	}
	for _, key := range []string{"website", "twitter", "telegram", "discord"} {
		var value *string
		if e := json.Unmarshal(fields[key], &value); e != nil {
			return profile, e
		}
		if value == nil || *value == "" {
			continue
		}
		if e := SafeLink(*value); e != nil {
			return profile, e
		}
		if key == "website" {
			profile.Website = value
		} else {
			profile.Socials[key] = *value
		}
	}
	for _, raw := range images {
		var image struct {
			ID   string `json:"id"`
			Type string `json:"image_type"`
		}
		if e := json.Unmarshal(raw, &image); e != nil {
			return profile, e
		}
		asset, e := uuid.Parse(image.ID)
		if e != nil || asset == uuid.Nil {
			return profile, ErrInvalid
		}
		runtime, _ := ctx.Value(runtimeKey{}).(*Runtime)
		if runtime == nil || runtime.Assets == nil {
			return profile, ErrState
		}
		if e = runtime.Assets.ApprovedAccountAsset(ctx, wallet, image.ID); e != nil {
			return profile, e
		}
		switch image.Type {
		case "avatar":
			if profile.AvatarAssetID != nil {
				return profile, ErrConflict
			}
			profile.AvatarAssetID = &image.ID
		case "banner":
			if profile.BannerAssetID != nil {
				return profile, ErrConflict
			}
			profile.BannerAssetID = &image.ID
		default:
			return profile, ErrInvalid
		}
	}
	return profile, nil
}

func sameLegacyProfile(a, b Profile) bool {
	// Normalize only representation: revision and timestamps are not public content.
	a.Revision, b.Revision = "", ""
	if !reflect.DeepEqual(a.PublicProfile, b.PublicProfile) {
		return false
	}
	if len(a.Socials) != len(b.Socials) {
		return false
	}
	for key, value := range a.Socials {
		if b.Socials[key] != value {
			return false
		}
	}
	return true
}

func reconcileLegacyProfile(current, source Profile, revision int64, edited bool) (Profile, error) {
	if current.Wallet != source.Wallet {
		return current, ErrConflict
	}
	if sameLegacyProfile(current, source) {
		return current, nil
	}
	blank := current.DisplayName == "" && current.Bio == "" && current.Website == nil && current.AvatarAssetID == nil && current.BannerAssetID == nil && len(current.Socials) == 0
	if edited || !blank {
		return current, ErrConflict
	}
	source.Revision = strconv.FormatInt(revision, 10)
	return source, nil
}

func quarantineLegacyProfile(ctx context.Context, tx pgx.Tx, wallet string, digest []byte, account uuid.UUID, reason string) error {
	_, e := tx.Exec(ctx, `INSERT INTO account_backfill VALUES($1,$2,$3,'quarantined',$4) ON CONFLICT(source_key) DO UPDATE SET state='quarantined',reason=EXCLUDED.reason`, "legacy:"+wallet, digest, account, reason)
	return e
}

func (s Store) BackfillLegacyRange(ctx context.Context, r BackfillRange) (BackfillCounts, error) {
	out := BackfillCounts{}
	if r.ID == "" || len(r.ID) > 128 || r.Limit < 1 || r.Limit > 500 {
		return out, ErrInvalid
	}
	if r.After != "" {
		normalized, e := Wallet(r.After)
		if e != nil {
			return out, e
		}
		r.After = normalized
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return out, e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "account-stable-read-cutover"); e != nil {
		return out, e
	}
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "account-range:"+r.ID); e != nil {
		return out, e
	}
	var previous []byte
	e = tx.QueryRow(ctx, `SELECT result FROM account_backfill_range WHERE range_id=$1 AND after_wallet=$2 AND batch_limit=$3`, r.ID, r.After, r.Limit).Scan(&previous)
	if e == nil {
		e = json.Unmarshal(previous, &out)
		return out, e
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return out, e
	}
	rows, e := tx.Query(ctx, `SELECT lower(wallet_address) FROM users.user_profiles WHERE lower(wallet_address)>$1 GROUP BY lower(wallet_address) ORDER BY lower(wallet_address) LIMIT $2`, strings.ToLower(r.After), r.Limit)
	if e != nil {
		return out, e
	}
	type source struct {
		wallet string
	}
	sources := []source{}
	for rows.Next() {
		var v source
		if e = rows.Scan(&v.wallet); e != nil {
			rows.Close()
			return out, e
		}
		sources = append(sources, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	hash := sha256.New()
	for _, source := range sources {
		if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "l35-source:legacy:"+source.wallet); e != nil {
			return out, e
		}
		if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "l35-wallet:"+source.wallet); e != nil {
			return out, e
		}
		profiles, images, e := legacyProfileInventory(ctx, tx, source.wallet)
		if e != nil {
			return out, e
		}
		out.Scanned++
		out.LastWallet = source.wallet
		watchlist, e := legacyWatchInventory(ctx, tx, source.wallet)
		if e != nil {
			return out, e
		}
		raw, e := legacySourceBytes(source.wallet, profiles, images, watchlist)
		if e != nil {
			return out, e
		}
		hash.Write(raw)
		digest := sha256.Sum256(raw)
		var account uuid.UUID
		e = tx.QueryRow(ctx, `SELECT account_id FROM identity_wallet_links WHERE chain_id=125 AND wallet=$1 AND state='verified'`, source.wallet).Scan(&account)
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return out, e
		}
		if len(profiles) != 1 || errors.Is(e, pgx.ErrNoRows) {
			out.Quarantined++
			if _, e = tx.Exec(ctx, `INSERT INTO account_backfill VALUES($1,$2,NULL,'quarantined','ambiguous legacy source or no verified identity') ON CONFLICT(source_key) DO NOTHING`, "legacy:"+source.wallet, digest[:]); e != nil {
				return out, e
			}
			continue
		}
		if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "l35-account-provision:"+account.String()); e != nil {
			return out, e
		}
		var deleted bool
		var oldWallet string
		e = tx.QueryRow(ctx, `SELECT wallet,deleted FROM account_state WHERE account_id=$1 FOR UPDATE`, account).Scan(&oldWallet, &deleted)
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return out, e
		}
		if e == nil && (deleted || oldWallet != source.wallet) {
			if e = quarantineLegacyProfile(ctx, tx, source.wallet, digest[:], account, "stable account ownership or deletion conflicts with legacy source"); e != nil {
				return out, e
			}
			out.Quarantined++
			continue
		}
		var priorHash []byte
		e = tx.QueryRow(ctx, `SELECT source_hash FROM account_backfill WHERE source_key=$1`, "legacy:"+source.wallet).Scan(&priorHash)
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return out, e
		}
		if e == nil && !bytes.Equal(priorHash, digest[:]) {
			if _, e = tx.Exec(ctx, `UPDATE account_backfill SET state='quarantined',reason='source bytes changed; reviewed resolution required' WHERE source_key=$1`, "legacy:"+source.wallet); e != nil {
				return out, e
			}
			out.Quarantined++
			continue
		}
		var quarantine bool
		if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM account_backfill WHERE source_key=$1 AND state='quarantined')`, "legacy:"+source.wallet).Scan(&quarantine); e != nil {
			return out, e
		}
		if quarantine {
			out.Quarantined++
			continue
		}
		profile, profileErr := legacyProfileProjection(ctx, source.wallet, profiles[0], images)
		if profileErr != nil {
			out.Quarantined++
			if _, e = tx.Exec(ctx, `INSERT INTO account_backfill VALUES($1,$2,$3,'quarantined','legacy profile field or asset requires reviewed migration') ON CONFLICT(source_key) DO UPDATE SET state='quarantined',reason=EXCLUDED.reason`, "legacy:"+source.wallet, digest[:], account); e != nil {
				return out, e
			}
			continue
		}
		count := len(watchlist)
		invalidPool := false
		for _, watch := range watchlist {
			if _, e := Wallet(watch.Pool); e != nil {
				invalidPool = true
			}
		}
		var combinedCount int
		if e = tx.QueryRow(ctx, `SELECT count(*) FROM (SELECT pool FROM account_watch WHERE account_id=$1 UNION SELECT lower(pool_address) FROM users.watchlists WHERE lower(wallet_address)=$2) pools`, account, source.wallet).Scan(&combinedCount); e != nil {
			return out, e
		}
		if count > 500 || combinedCount > 500 || invalidPool {
			out.Quarantined++
			if _, e = tx.Exec(ctx, `INSERT INTO account_backfill VALUES($1,$2,$3,'quarantined','legacy watchlist violates frozen capacity or pool validation') ON CONFLICT(source_key) DO UPDATE SET state='quarantined',reason=EXCLUDED.reason`, "legacy:"+source.wallet, digest[:], account); e != nil {
				return out, e
			}
			continue
		}
		profile.Revision = "0"
		profile.SchemaVersion = 1
		profile.UpdatedAt = s.now()
		pr, e := json.Marshal(profile)
		if e != nil {
			return out, e
		}
		pf, e := json.Marshal(initialPreferences(s.now()))
		if e != nil {
			return out, e
		}
		if _, e = tx.Exec(ctx, `INSERT INTO account_state(account_id,wallet,profile,preferences) VALUES($1,$2,$3,$4) ON CONFLICT(account_id) DO NOTHING`, account, source.wallet, pr, pf); e != nil {
			return out, e
		}
		a, e := load(ctx, tx, account)
		if e != nil {
			return out, e
		}
		if !sameLegacyProfile(a.Profile, profile) {
			var edited bool
			if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM account_outbox WHERE account_id=$1 AND kind='profile.updated')`, account).Scan(&edited); e != nil {
				return out, e
			}
			profile, e = reconcileLegacyProfile(a.Profile, profile, a.Revision, edited)
			if e != nil {
				if e = quarantineLegacyProfile(ctx, tx, source.wallet, digest[:], account, "stable profile conflicts with legacy content; preserve user edits"); e != nil {
					return out, e
				}
				out.Quarantined++
				continue
			}
			pr, e = json.Marshal(profile)
			if e != nil {
				return out, e
			}
			if _, e = tx.Exec(ctx, `UPDATE account_state SET profile=$2,updated_at=$3 WHERE account_id=$1`, account, pr, s.now()); e != nil {
				return out, e
			}
		}
		// Compare the stored content, including socials and asset references,
		// before publishing a verified result or completion event.
		a, e = load(ctx, tx, account)
		if e != nil {
			return out, e
		}
		if !sameLegacyProfile(a.Profile, profile) {
			return out, ErrConflict
		}

		_, e = tx.Exec(ctx, `INSERT INTO account_watch(account_id,chain_id,pool,added_at,revision,alert_enabled) SELECT $1,125,lower(pool_address),to_timestamp(min(added_at)),0,false FROM users.watchlists WHERE lower(wallet_address)=$2 GROUP BY lower(pool_address) ON CONFLICT DO NOTHING`, account, source.wallet)
		if e != nil {
			return out, e
		}
		linkedTag, e := tx.Exec(ctx, `INSERT INTO account_backfill VALUES($1,$2,$3,'linked','') ON CONFLICT(source_key) DO NOTHING`, "legacy:"+source.wallet, digest[:], account)
		if e != nil {
			return out, e
		}
		out.Linked++
		out.Verified++
		out.WatchlistCount += count
		eventID := uuid.NewSHA1(uuid.NameSpaceOID, append([]byte("kindle-account-backfill:"+source.wallet+":"), digest[:]...))
		if linkedTag.RowsAffected() == 1 {
			if e = emitIdentity(ctx, tx, a, eventID, "account.backfill.completed", map[string]any{"accountId": account, "sourceDigest": hex.EncodeToString(digest[:]), "watchlistCount": count}); e != nil {
				return out, e
			}
		}
	}
	out.Checksum = hex.EncodeToString(hash.Sum(nil))
	result, e := json.Marshal(out)
	if e != nil {
		return out, e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO account_backfill_range(range_id,after_wallet,batch_limit,result,completed_at) VALUES($1,$2,$3,$4,$5)`, r.ID, r.After, r.Limit, result, s.now()); e != nil {
		return out, e
	}
	if e = tx.Commit(ctx); e != nil {
		return out, e
	}
	return out, nil
}
func (s Store) SetStableAccountReads(ctx context.Context, enabled bool) error {
	if !enabled {
		_, e := s.DB.Exec(ctx, `INSERT INTO account_read_switch(singleton,stable_reads,changed_at,generation) VALUES(true,$1,$2,1) ON CONFLICT(singleton) DO UPDATE SET stable_reads=EXCLUDED.stable_reads,changed_at=EXCLUDED.changed_at,generation=account_read_switch.generation+CASE WHEN account_read_switch.stable_reads<>EXCLUDED.stable_reads THEN 1 ELSE 0 END`, enabled, s.now())
		return e
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "account-stable-read-cutover"); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `LOCK TABLE users.user_profiles,users.user_images,users.watchlists IN SHARE MODE`); e != nil {
		return e
	}
	var unresolved bool
	if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM account_backfill WHERE state='quarantined') OR EXISTS(SELECT 1 FROM users.user_profiles p WHERE NOT EXISTS(SELECT 1 FROM account_backfill b WHERE b.source_key='legacy:'||lower(p.wallet_address) AND b.state='linked')) OR EXISTS(SELECT 1 FROM users.user_images i WHERE NOT EXISTS(SELECT 1 FROM account_backfill b WHERE b.source_key='legacy:'||lower(i.wallet_address) AND b.state='linked'))`).Scan(&unresolved); e != nil {
		return e
	}
	if unresolved {
		return ErrState
	}
	rows, e := tx.Query(ctx, `SELECT DISTINCT lower(wallet_address) FROM users.user_profiles ORDER BY 1`)
	if e != nil {
		return e
	}
	wallets := []string{}
	for rows.Next() {
		var wallet string
		if e = rows.Scan(&wallet); e != nil {
			rows.Close()
			return e
		}
		wallets = append(wallets, wallet)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, wallet := range wallets {
		profiles, images, e := legacyProfileInventory(ctx, tx, wallet)
		if e != nil {
			return e
		}
		watches, e := legacyWatchInventory(ctx, tx, wallet)
		if e != nil {
			return e
		}
		raw, e := legacySourceBytes(wallet, profiles, images, watches)
		if e != nil {
			return e
		}
		digest := sha256.Sum256(raw)
		var verified bool
		if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM account_backfill WHERE source_key=$1 AND state='linked' AND source_hash=$2)`, "legacy:"+wallet, digest[:]).Scan(&verified); e != nil {
			return e
		}
		if !verified {
			return ErrState
		}
	}
	if _, e = tx.Exec(ctx, `INSERT INTO account_read_switch(singleton,stable_reads,changed_at,generation) VALUES(true,true,$1,1) ON CONFLICT(singleton) DO UPDATE SET stable_reads=true,changed_at=EXCLUDED.changed_at,generation=account_read_switch.generation+CASE WHEN NOT account_read_switch.stable_reads THEN 1 ELSE 0 END`, s.now()); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

type ReadProjection struct {
	Profile    PublicProfile
	Source     string
	Generation int64
}

func (s Store) PublicFromReadSwitch(ctx context.Context, wallet string) (ReadProjection, error) {
	current, e := s.Public(ctx, wallet)
	if e != nil {
		return ReadProjection{}, e
	}
	var stable, deleted bool
	var generation int64
	if e = s.reader(ctx).QueryRow(ctx, `SELECT coalesce((SELECT stable_reads FROM account_read_switch WHERE singleton),true),deleted,coalesce((SELECT generation FROM account_read_switch WHERE singleton),1) FROM account_state WHERE wallet=$1`, current.Wallet).Scan(&stable, &deleted, &generation); e != nil {
		return ReadProjection{}, e
	}
	if stable || deleted {
		return ReadProjection{Profile: current, Source: "stable", Generation: generation}, nil
	}
	var name, bio string
	var website *string
	e = s.reader(ctx).QueryRow(ctx, `SELECT coalesce(display_name,''),coalesce(bio,''),nullif(website,'') FROM users.user_profiles WHERE lower(wallet_address)=$1 ORDER BY wallet_address LIMIT 1`, current.Wallet).Scan(&name, &bio, &website)
	if errors.Is(e, pgx.ErrNoRows) {
		return ReadProjection{Profile: current, Source: "stable-only-record", Generation: generation}, nil
	}
	if e != nil {
		return ReadProjection{}, e
	}
	patch := ProfilePatch{ExpectedRevision: current.Revision, DisplayName: &name, Bio: &bio}
	if e = patch.Validate(); e != nil {
		return ReadProjection{}, e
	}
	if website != nil {
		if e = SafeLink(*website); e != nil {
			return ReadProjection{}, e
		}
	}
	return ReadProjection{Profile: PublicProfile{Wallet: current.Wallet, DisplayName: *patch.DisplayName, Bio: bio, Website: website, Revision: current.Revision}, Source: "legacy", Generation: generation}, nil
}

func (h HTTP) BackfillLegacyRange(ctx context.Context, r BackfillRange) (BackfillCounts, error) {
	if h.Runtime == nil {
		return BackfillCounts{}, ErrInvalid
	}
	ctx = context.WithValue(ctx, runtimeKey{}, h.Runtime)
	started := time.Now()
	counts, e := h.Store.BackfillLegacyRange(ctx, r)
	outcome := "success"
	if e != nil {
		outcome = "failed"
	}
	h.Runtime.Metrics.Observe("backfill", outcome, time.Since(started))
	return counts, e
}
