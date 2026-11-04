// Package user implements the media/user HTTP handlers, porting the TS index.ts
// monolith into discrete routes: the public profile read (Redis-cached), the
// EIP-191-gated profile upsert, EIP-191-gated avatar/banner uploads (virus scan
// -> R2 bucket), public byte-serving of avatars/banners straight from R2, and
// the per-wallet watchlist (list / add / remove, all signature-gated).
//
// Storage is BUCKET-PRIMARY (R2): uploads stream to the bucket and reads are
// served back from it with mandatory cache revalidation. media/user owns its own
// bucket, separate from media/metadata.
package user

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	goredis "github.com/redis/go-redis/v9"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	sharedredis "github.com/Sidiora-Technologies/KindleLaunch/shared/redis"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/storage"

	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/db/sqlcdb"
	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/image"
	"github.com/Sidiora-Technologies/KindleLaunch/media/user/pkg/types"
)

const (
	// jsonCacheTTL is the Redis TTL for cached public profile responses.
	jsonCacheTTL = 60 * time.Second

	cacheKeyPrefix = "user:json:v2:"
)

// addrRe matches a normalized (lowercased) EVM address.
var addrRe = regexp.MustCompile(`^0x[a-f0-9]{40}$`)

// ObjectStore is the subset of shared/storage.Client the handlers use. The
// concrete client (real R2 / MinIO in tests) satisfies it — no fakes.
type ObjectStore interface {
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	Get(ctx context.Context, key string) (*storage.Object, error)
}

// Scanner scans an uploaded buffer for malware.
type Scanner func(ctx context.Context, data []byte) sharedhttp.ScanResult

// Deps are the handler dependencies.
type Deps struct {
	Queries   sqlcdb.Querier
	Redis     *goredis.Client
	Store     ObjectStore
	PublicURL string
	MaxAvatar int64
	MaxBanner int64
	Logger    *slog.Logger

	// Scan defaults to shared/http.ScanBuffer (clamd via $CLAMAV_HOST).
	Scan Scanner
	// Clock defaults to time.Now.
	Clock func() time.Time
}

// Handlers serves the user routes.
type Handlers struct {
	q         sqlcdb.Querier
	redis     *goredis.Client
	store     ObjectStore
	publicURL string
	maxAvatar int64
	maxBanner int64
	logger    *slog.Logger
	scan      Scanner
	clock     func() time.Time
}

// New constructs Handlers, applying defaults for optional dependencies.
func New(d Deps) *Handlers {
	if d.PublicURL != "" {
		u, err := url.Parse(d.PublicURL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
			panic("user: PublicURL must be a canonical HTTP(S) URL")
		}
	}
	scan := d.Scan
	if scan == nil {
		scan = func(ctx context.Context, data []byte) sharedhttp.ScanResult {
			return sharedhttp.ScanBuffer(ctx, data, sharedhttp.ScanOptions{})
		}
	}
	clock := d.Clock
	if clock == nil {
		clock = time.Now
	}
	maxAvatar := d.MaxAvatar
	if maxAvatar <= 0 {
		maxAvatar = 2 << 20
	}
	maxBanner := d.MaxBanner
	if maxBanner <= 0 {
		maxBanner = 5 << 20
	}
	return &Handlers{
		q:         d.Queries,
		redis:     d.Redis,
		store:     d.Store,
		publicURL: strings.TrimRight(d.PublicURL, "/"),
		maxAvatar: maxAvatar,
		maxBanner: maxBanner,
		logger:    d.Logger,
		scan:      scan,
		clock:     clock,
	}
}

// RegisterRoutes mounts the user endpoints onto r.
func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/users/{walletAddress}", h.getProfile)
	r.Post("/users/{walletAddress}", h.updateProfile)
	r.Post("/users/{walletAddress}/avatar", h.uploadImage(image.TypeAvatar))
	r.Post("/users/{walletAddress}/banner", h.uploadImage(image.TypeBanner))
	r.Get("/users/{walletAddress}/avatar", h.serveImage(image.TypeAvatar))
	r.Get("/users/{walletAddress}/banner", h.serveImage(image.TypeBanner))
	r.Get("/users/{walletAddress}/watchlist", h.getWatchlist)
	r.Put("/users/{walletAddress}/watchlist/{poolAddress}", h.addWatchlist)
	r.Delete("/users/{walletAddress}/watchlist/{poolAddress}", h.removeWatchlist)
}

// ── GET /users/{walletAddress}  (also serves the ".json" alias) ──────────────

func (h *Handlers) getProfile(w http.ResponseWriter, r *http.Request) {
	addr := normalizeAddr(strings.TrimSuffix(chi.URLParam(r, "walletAddress"), ".json"))
	if !addrRe.MatchString(addr) {
		sharedhttp.WriteError(w, http.StatusBadRequest, "Bad Request", "invalid wallet address")
		return
	}
	ctx := r.Context()

	cacheKey := h.profileCacheKey(addr)
	if cached, found, err := sharedredis.CacheGet[types.PublicProfile](ctx, h.redis, cacheKey); err == nil && found {
		w.Header().Set("Cache-Control", "public, max-age=60")
		sharedhttp.WriteJSON(w, http.StatusOK, cached)
		return
	}

	out, err := h.buildProfile(ctx, addr, baseURL(h.publicURL, r))
	if err != nil {
		h.fail(w, "build profile", err)
		return
	}
	if err := sharedredis.CacheSet(ctx, h.redis, cacheKey, out, jsonCacheTTL); err != nil {
		h.logErr("cache set", err)
	}
	w.Header().Set("Cache-Control", "public, max-age=60")
	sharedhttp.WriteJSON(w, http.StatusOK, out)
}

// buildProfile assembles the public profile from profile + images + created
// pools. An absent profile row yields a wallet-only response (parity).
func (h *Handlers) buildProfile(ctx context.Context, addr, base string) (types.PublicProfile, error) {
	profile, err := h.q.GetUserProfile(ctx, addr)
	hasProfile := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return types.PublicProfile{}, err
	}

	images, err := h.q.GetUserImages(ctx, addr)
	if err != nil {
		return types.PublicProfile{}, err
	}

	pools, err := h.q.ListCreatedPools(ctx, addr)
	if err != nil {
		return types.PublicProfile{}, err
	}

	out := types.PublicProfile{
		WalletAddress: addr,
		Socials:       types.Socials{},
		Images:        buildImages(base, addr, images),
		CreatedPools:  buildCreatedPools(pools),
	}
	if hasProfile {
		out.DisplayName = textPtr(profile.DisplayName)
		out.Bio = textPtr(profile.Bio)
		out.Socials = types.Socials{
			Website:  textPtr(profile.Website),
			Twitter:  textPtr(profile.Twitter),
			Telegram: textPtr(profile.Telegram),
			Discord:  textPtr(profile.Discord),
		}
		out.CreatedAt = int64Ptr(profile.CreatedAt)
		out.UpdatedAt = int64Ptr(profile.UpdatedAt)
	}
	return out, nil
}

// ── POST /users/{walletAddress}  (EIP-191-gated profile upsert) ──────────────

func (h *Handlers) updateProfile(w http.ResponseWriter, r *http.Request) {
	addr := normalizeAddr(chi.URLParam(r, "walletAddress"))
	if !addrRe.MatchString(addr) {
		sharedhttp.WriteError(w, http.StatusBadRequest, "Bad Request", "invalid wallet address")
		return
	}
	ctx := r.Context()

	body, ok := readBoundedBody(w, r, maxJSONBody)
	if !ok {
		return
	}
	var req types.UpdateProfileRequest
	if _, err := sharedauth.BodyDigest(body); err != nil || json.Unmarshal(body, &req) != nil {
		sharedhttp.WriteRequestBodyError(w, http.StatusBadRequest, sharedhttp.RuleJSON, "invalid JSON body")
		return
	}
	e, ok := h.operationHeaders(w, r, addr, "profile.update")
	if !ok || !h.consumeOperation(w, r, e, body) {
		return
	}

	now := h.clock().Unix()
	if err := h.q.UpsertUserProfile(ctx, sqlcdb.UpsertUserProfileParams{
		WalletAddress: addr,
		DisplayName:   strToText(req.Data.DisplayName),
		Bio:           strToText(req.Data.Bio),
		Twitter:       strToText(req.Data.Twitter),
		Telegram:      strToText(req.Data.Telegram),
		Discord:       strToText(req.Data.Discord),
		Website:       strToText(req.Data.Website),
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		h.fail(w, "upsert profile", err)
		return
	}

	if err := sharedredis.CacheInvalidate(ctx, h.redis, h.profileCacheKey(addr)); err != nil {
		h.logErr("cache invalidate", err)
	}
	sharedhttp.WriteJSON(w, http.StatusOK, types.SuccessResponse{Success: true})
}

// ── POST /users/{walletAddress}/{avatar|banner}  (multipart image upload) ────

func (h *Handlers) uploadImage(imageType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addr := normalizeAddr(chi.URLParam(r, "walletAddress"))
		if !addrRe.MatchString(addr) {
			sharedhttp.WriteError(w, http.StatusBadRequest, "Bad Request", "invalid wallet address")
			return
		}
		ctx := r.Context()

		maxSize := h.maxAvatar
		if imageType == image.TypeBanner {
			maxSize = h.maxBanner
		}
		// Bound the whole request, including unused parts and multipart headers.
		r.Body = http.MaxBytesReader(w, r.Body, maxSize+multipartOverhead)
		if r.ContentLength > maxSize+multipartOverhead {
			sharedhttp.WriteRequestBodyError(w, http.StatusRequestEntityTooLarge, sharedhttp.RuleMaxBytes, "multipart body too large")
			return
		}
		e, ok := h.operationHeaders(w, r, addr, "image."+imageType+".update")
		if !ok {
			return
		}
		raw, ok := readBoundedBody(w, r, maxSize+multipartOverhead)
		if !ok {
			return
		}
		if !h.consumeOperation(w, r, e, uploadIntent(r.Header.Get("Content-Type"), raw)) {
			return
		}
		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "multipart/form-data" || params["boundary"] == "" {
			sharedhttp.WriteRequestBodyError(w, http.StatusBadRequest, sharedhttp.RuleMultipart, "invalid multipart form")
			return
		}
		reader := multipart.NewReader(bytes.NewReader(raw), params["boundary"])
		part, err := reader.NextRawPart()
		if err != nil {
			sharedhttp.WriteError(w, http.StatusBadRequest, "Bad Request", "No file uploaded")
			return
		}
		if (part.FormName() != imageType && part.FormName() != "file") || part.FileName() == "" || len(part.Header) > 2 || part.Header.Get("Content-Transfer-Encoding") != "" {
			sharedhttp.WriteRequestBodyError(w, http.StatusBadRequest, sharedhttp.RuleMultipart, "expected one image file part")
			return
		}
		buf, err := io.ReadAll(io.LimitReader(part, maxSize+1))
		if err != nil {
			sharedhttp.WriteRequestBodyError(w, http.StatusBadRequest, sharedhttp.RuleMultipart, "image could not be read")
			return
		}
		if int64(len(buf)) > maxSize {
			sharedhttp.WriteRequestBodyError(w, http.StatusRequestEntityTooLarge, sharedhttp.RuleMaxBytes, imageType+" too large")
			return
		}
		if _, err := reader.NextRawPart(); err != io.EOF {
			sharedhttp.WriteRequestBodyError(w, http.StatusBadRequest, sharedhttp.RuleMultipart, "expected one image file part")
			return
		}
		mime := part.Header.Get("Content-Type")
		if !image.AllowedMime(mime) {
			sharedhttp.WriteRequestBodyError(w, http.StatusUnsupportedMediaType, sharedhttp.RuleMediaType,
				"Unsupported format. Use webp, png, svg, or jpeg.")
			return
		}
		if image.IsSVG(mime) && !image.IsSVGSafe(buf) {
			sharedhttp.WriteError(w, http.StatusBadRequest, "Bad Request",
				"SVG rejected: contains script, event handlers, or unsafe elements")
			return
		}
		// Only an explicit INFECTED verdict is the uploader's fault (422). An
		// absent, unreachable or malformed scanner fails closed as a dependency
		// outage (503) so clients retry instead of treating a clean image as bad.
		if res := h.scan(ctx, buf); !res.Clean {
			if res.Verdict == sharedhttp.ScanInfected {
				sharedhttp.WriteError(w, http.StatusUnprocessableEntity, "Unprocessable Entity",
					imageType+" rejected by virus scanner")
				return
			}
			h.logErr("scan "+imageType, errors.New(res.Reason))
			sharedhttp.WriteError(w, http.StatusServiceUnavailable, "Service Unavailable",
				"virus scanner unavailable: "+res.Reason)
			return
		}

		ext := image.ExtForMime(mime)
		storageKey := imageType + "s/" + imageType + "-" + addr + "." + ext
		if err := h.store.Put(ctx, storageKey, strings.NewReader(string(buf)), int64(len(buf)), mime); err != nil {
			h.fail(w, "store put", err)
			return
		}

		now := h.clock().Unix()
		if err := h.q.UpsertUserImage(ctx, sqlcdb.UpsertUserImageParams{
			ID:            addr + "-" + imageType,
			WalletAddress: addr,
			ImageType:     imageType,
			StorageKey:    storageKey,
			MimeType:      mime,
			SizeBytes:     int32(len(buf)),
			UploadedAt:    now,
		}); err != nil {
			h.fail(w, "upsert image", err)
			return
		}

		if err := sharedredis.CacheInvalidate(ctx, h.redis, h.profileCacheKey(addr)); err != nil {
			h.logErr("cache invalidate", err)
		}
		url := baseURL(h.publicURL, r) + "/users/" + addr + "/" + imageType
		sharedhttp.WriteJSON(w, http.StatusOK, types.UploadResponse{Success: true, URL: url})
	}
}

// ── GET /users/{walletAddress}/{avatar|banner}  (serve bytes from R2) ────────

func (h *Handlers) serveImage(imageType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addr := normalizeAddr(chi.URLParam(r, "walletAddress"))
		if !addrRe.MatchString(addr) {
			sharedhttp.WriteError(w, http.StatusBadRequest, "Bad Request", "invalid wallet address")
			return
		}
		ctx := r.Context()

		img, err := h.q.GetImageByType(ctx, sqlcdb.GetImageByTypeParams{WalletAddress: addr, ImageType: imageType})
		if errors.Is(err, pgx.ErrNoRows) {
			sharedhttp.WriteError(w, http.StatusNotFound, "Not Found", "No "+imageType+" found")
			return
		}
		if err != nil {
			h.fail(w, "get image", err)
			return
		}

		obj, err := h.store.Get(ctx, img.StorageKey)
		if errors.Is(err, storage.ErrNotFound) {
			sharedhttp.WriteError(w, http.StatusNotFound, "Not Found", "Image not found in storage")
			return
		}
		if err != nil {
			h.fail(w, "store get", err)
			return
		}
		defer obj.Body.Close()

		w.Header().Set("Content-Type", img.MimeType)
		w.Header().Set("Cache-Control", "public, no-cache, max-age=0, must-revalidate")
		w.Header().Set("CDN-Cache-Control", "public, no-cache, max-age=0, must-revalidate")
		w.Header().Set("Vary", "Accept-Encoding")
		if img.SizeBytes > 0 {
			w.Header().Set("Content-Length", itoa64(int64(img.SizeBytes)))
		}
		w.WriteHeader(http.StatusOK)
		if _, err := io.Copy(w, obj.Body); err != nil {
			h.logErr("stream image", err)
		}
	}
}

// ── GET /users/{walletAddress}/watchlist ─────────────────────────────────────

func (h *Handlers) getWatchlist(w http.ResponseWriter, r *http.Request) {
	addr := normalizeAddr(chi.URLParam(r, "walletAddress"))
	if !addrRe.MatchString(addr) {
		sharedhttp.WriteError(w, http.StatusBadRequest, "Bad Request", "invalid wallet address")
		return
	}
	rows, err := h.q.GetWatchlist(r.Context(), addr)
	if err != nil {
		h.fail(w, "get watchlist", err)
		return
	}
	pools := make([]types.WatchlistEntry, 0, len(rows))
	for _, row := range rows {
		pools = append(pools, types.WatchlistEntry{PoolAddress: row.PoolAddress, AddedAt: row.AddedAt})
	}
	sharedhttp.WriteJSON(w, http.StatusOK, types.WatchlistResponse{WalletAddress: addr, Pools: pools})
}

// ── PUT /users/{walletAddress}/watchlist/{poolAddress} ───────────────────────

func (h *Handlers) addWatchlist(w http.ResponseWriter, r *http.Request) {
	addr, pool, ok := h.watchlistAuth(w, r)
	if !ok {
		return
	}
	if err := h.q.AddWatchlist(r.Context(), sqlcdb.AddWatchlistParams{
		WalletAddress: addr,
		PoolAddress:   pool,
		AddedAt:       h.clock().Unix(),
	}); err != nil {
		h.fail(w, "add watchlist", err)
		return
	}
	sharedhttp.WriteJSON(w, http.StatusOK, types.SuccessResponse{Success: true})
}

// ── DELETE /users/{walletAddress}/watchlist/{poolAddress} ────────────────────

func (h *Handlers) removeWatchlist(w http.ResponseWriter, r *http.Request) {
	addr, pool, ok := h.watchlistAuth(w, r)
	if !ok {
		return
	}
	if err := h.q.RemoveWatchlist(r.Context(), sqlcdb.RemoveWatchlistParams{
		WalletAddress: addr,
		PoolAddress:   pool,
	}); err != nil {
		h.fail(w, "remove watchlist", err)
		return
	}
	sharedhttp.WriteJSON(w, http.StatusOK, types.SuccessResponse{Success: true})
}

// watchlistAuth validates the wallet+pool addresses and consumes the signed
// operation from the bounded header transport. It writes errors itself and
// returns ok=false on any failure.
func (h *Handlers) watchlistAuth(w http.ResponseWriter, r *http.Request) (addr, pool string, ok bool) {
	addr = normalizeAddr(chi.URLParam(r, "walletAddress"))
	pool = normalizeAddr(chi.URLParam(r, "poolAddress"))
	if !addrRe.MatchString(addr) {
		sharedhttp.WriteError(w, http.StatusBadRequest, "Bad Request", "invalid wallet address")
		return "", "", false
	}
	if !addrRe.MatchString(pool) {
		sharedhttp.WriteError(w, http.StatusBadRequest, "Bad Request", "invalid pool address")
		return "", "", false
	}
	body, valid := readBoundedBody(w, r, maxJSONBody)
	if !valid {
		return "", "", false
	}
	if _, err := sharedauth.BodyDigest(body); err != nil {
		sharedhttp.WriteRequestBodyError(w, http.StatusBadRequest, sharedhttp.RuleJSON, "invalid JSON body")
		return "", "", false
	}
	action := "watchlist.update"
	if r.Method == http.MethodDelete {
		action = "watchlist.remove"
	}
	e, valid := h.operationHeaders(w, r, addr, action)
	if !valid || !h.consumeOperation(w, r, e, body) {
		return "", "", false
	}
	return addr, pool, true
}

// ── shared helpers ───────────────────────────────────────────────────────────

// firstFile returns the first uploaded file found under any of the given form
// field names, or nil when none is present.
func firstFile(r *http.Request, fields ...string) *multipart.FileHeader {
	if r.MultipartForm == nil {
		return nil
	}
	for _, field := range fields {
		if fhs := r.MultipartForm.File[field]; len(fhs) > 0 {
			return fhs[0]
		}
	}
	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// ── error helpers ────────────────────────────────────────────────────────────

func (h *Handlers) fail(w http.ResponseWriter, op string, err error) {
	h.logErr(op, err)
	sharedhttp.WriteError(w, http.StatusInternalServerError, "Internal Server Error", "")
}

func (h *Handlers) logErr(op string, err error) {
	if h.logger != nil {
		h.logger.Error("user handler error", slog.String("op", op), slog.String("err", err.Error()))
	}
}

func (h *Handlers) profileCacheKey(addr string) string {
	return cacheKeyPrefix + h.publicURL + ":" + addr
}
