package app

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	raster "github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/image"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/metadata"
)

// Development publication transport. The environment is what the browser
// signs into the operation envelope (it reads it from
// GET /v3/metadata-publication-authority); the headers match the user
// service's operation transport.
const (
	developmentPublicationEnvironment = "development"
	developmentEnvelopeHeader         = "X-Kindle-Operation"
	developmentSignatureHeader        = "X-Kindle-Operation-Signature"
	developmentIdentityAudience       = "metadata-development"
)

// developmentMedia is the production creator-media surface (ConnectedMedia
// token assets + PublicationService) wired with development dependencies.
type developmentMedia struct {
	connected   *metadata.ConnectedMedia
	publication *metadata.PublicationService
	pool        *pgxpool.Pool
}

// newDevelopmentMedia builds the development creator-media surface. It
// returns (nil, nil), with a warning, when a required input is missing:
// IDENTITY_SERVICE_URL + IDENTITY_SERVICE_TOKEN (session identity), RPC_URL
// (creation receipts and wallet-signature verification) and an https
// PUBLIC_URL origin (approved variant URLs). The routes then answer 503.
func newDevelopmentMedia(ctx context.Context, cfg config.Config, logger *slog.Logger, reads *metadata.DevelopmentV2Reads, chain *gethrpc.Client) (*developmentMedia, error) {
	missing := []string{}
	if strings.TrimSpace(cfg.IdentityServiceURL) == "" || len(cfg.IdentityServiceToken) < 32 {
		missing = append(missing, "IDENTITY_SERVICE_URL/IDENTITY_SERVICE_TOKEN")
	}
	if chain == nil {
		missing = append(missing, "RPC_URL")
	}
	publicOrigin := strings.TrimRight(strings.TrimSpace(cfg.PublicURL), "/")
	if u, e := url.Parse(publicOrigin); e != nil || u.Scheme != "https" || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.User != nil {
		missing = append(missing, "PUBLIC_URL (https origin)")
	}
	if len(missing) > 0 {
		if logger != nil {
			logger.Warn("development token image upload and metadata publication disabled", slog.String("missing", strings.Join(missing, ", ")))
		}
		return nil, nil
	}
	objects, e := newDevelopmentObjects(ctx, cfg)
	if e != nil {
		return nil, e
	}
	schema := metadata.DevelopmentConnectedSchema
	poolConfig, e := pgxpool.ParseConfig(cfg.DatabaseURL)
	if e != nil {
		return nil, e
	}
	// Same pinning as the production runtime role: unqualified ledger tables
	// resolve to this schema only.
	poolConfig.ConnConfig.RuntimeParams["search_path"] = schema + ",pg_catalog"
	if poolConfig.MaxConns < 4 {
		poolConfig.MaxConns = 4
	}
	pool, e := pgxpool.NewWithConfig(ctx, poolConfig)
	if e != nil {
		return nil, e
	}
	built := false
	defer func() {
		if !built {
			pool.Close()
		}
	}()
	if e = metadata.EnsureDevelopmentConnectedSchema(ctx, pool, schema); e != nil {
		return nil, e
	}
	backend := sharedauth.NewReportedHashBackend(chain)
	audience := cfg.IdentityAudience
	if audience == "" {
		audience = developmentIdentityAudience
	}
	v2, e := metadata.NewV2ProductionHandlers(pool, schema, audience, backend, metadata.DevelopmentCanonicalSource{Reads: reads}, time.Now)
	if e != nil {
		return nil, e
	}
	identity := sharedauth.HTTPIdentityAuthority{Endpoint: strings.TrimRight(strings.TrimSpace(cfg.IdentityServiceURL), "/"), ServiceToken: cfg.IdentityServiceToken, AllowPlainHTTP: true}
	instance := metadata.ConnectedMedia{
		Metadata:   v2,
		Identity:   identity,
		Objects:    objects,
		Scanner:    sharedhttp.ScanOptions{Host: cfg.ClamAVHost, Port: cfg.ClamAVPort, Timeout: 5 * time.Second},
		Worker:     raster.InProcessWorker{},
		Moderation: metadata.DevelopmentVisibility{},
	}
	operations := sharedauth.OperationAuthorizer{
		Ledger:    sharedauth.Ledger{Pool: pool, Environment: developmentPublicationEnvironment, AuditKey: developmentAuditKey(cfg)},
		Verifier:  sharedauth.WalletOperationVerifier{Backend: backend},
		Transport: sharedauth.OperationTransport{EnvelopeHeader: developmentEnvelopeHeader, SignatureHeader: developmentSignatureHeader},
	}
	publication, e := metadata.NewDevelopmentPublicationService(instance, operations, metadata.SessionCheckIdentities{Authority: sharedauth.HTTPStreamAuthority(identity)})
	if e != nil {
		return nil, e
	}
	built = true
	return &developmentMedia{connected: &instance, publication: &publication, pool: pool}, nil
}

// developmentAuditKey is METADATA_PUBLICATION_AUDIT_KEY (base64, 32 bytes)
// when set, else a key derived from IDENTITY_SERVICE_TOKEN. It only
// pseudonymises actors in security_audit_outbox.
func developmentAuditKey(cfg config.Config) []byte {
	if key, e := base64.StdEncoding.Strict().DecodeString(cfg.PublicationAuditKey); e == nil && len(key) == 32 {
		return key
	}
	mac := hmac.New(sha256.New, []byte(cfg.IdentityServiceToken))
	mac.Write([]byte("metadata-publication-audit-v1"))
	return mac.Sum(nil)
}

func firstSet(values ...string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

// registerDevelopmentMedia mounts the creator token-image routes and the /v3
// metadata-publication routes. Without the development media surface they
// answer the production unavailable shape (503) instead of 404.
func registerDevelopmentMedia(r chi.Router, media *developmentMedia) {
	if media == nil {
		unavailable := func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintf(w, `{"requestId":%q,"error":{"code":"unavailable"}}`+"\n", uuid.NewString())
		}
		r.Put("/v2/tokens/{tokenAddress}/assets/{assetId}", unavailable)
		r.Get("/v2/tokens/{tokenAddress}/assets/{assetId}", unavailable)
		r.Get("/v3/metadata-publication-authority", unavailable)
		r.Post(metadata.PublicationRoute, unavailable)
		r.Get(metadata.PublicationRoute+"/{operationId}", unavailable)
		return
	}
	r.Group(func(cr chi.Router) {
		cr.Use(metadata.DevelopmentSessionChannel(media.connected.Identity))
		media.connected.RegisterTokenAssetRoutes(cr)
		media.publication.RegisterRoutes(cr)
	})
}

// requireMetadataChain admits the configured RPC before any development
// receipt can be labelled as chain 125.
func requireMetadataChain(ctx context.Context, client *ethclient.Client) error {
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	chain, err := client.ChainID(bounded)
	if err != nil || chain == nil || chain.Cmp(big.NewInt(125)) != 0 {
		return fmt.Errorf("metadata_chain_authority_unavailable")
	}
	return nil
}

type developmentChainReceipts struct{ client *ethclient.Client }

func (d developmentChainReceipts) TransactionReceipt(ctx context.Context, hash common.Hash) (*types.Receipt, error) {
	if d.client == nil || requireMetadataChain(ctx, d.client) != nil {
		return nil, metadata.ErrChainUnavailable
	}
	return d.client.TransactionReceipt(ctx, hash)
}

func newDevelopmentObjects(ctx context.Context, cfg config.Config) (metadata.S3AssetObjects, error) {
	objects := metadata.S3AssetObjects{
		QuarantineBucket: strings.TrimSpace(cfg.QuarantineBucket),
		PublicBucket:     firstSet(cfg.ApprovedBucket, cfg.S3Bucket),
		PublicOrigin:     strings.TrimRight(strings.TrimSpace(cfg.PublicURL), "/"),
		Endpoint:         cfg.S3Endpoint,
	}
	if objects.QuarantineBucket == "" || objects.QuarantineBucket == objects.PublicBucket {
		return objects, fmt.Errorf("metadata_private_quarantine_required")
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.S3Region), awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.S3AccessKeyID, cfg.S3SecretAccessKey, "")))
	if err != nil {
		return objects, err
	}
	objects.Client = s3.NewFromConfig(awsCfg, func(o *s3.Options) { o.BaseEndpoint = aws.String(cfg.S3Endpoint); o.UsePathStyle = true })
	probe := objects
	objects.Readiness = func(c context.Context) error { return developmentQuarantineReady(c, probe) }
	if err = objects.Ready(ctx); err != nil {
		return objects, fmt.Errorf("metadata_private_quarantine_unavailable")
	}
	return objects, nil
}

func developmentQuarantineReady(ctx context.Context, objects metadata.S3AssetObjects) (err error) {
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// Use the production ACL/policy and unsigned-list checks as well as
	// reads of existing keys in both original namespaces. Random missing
	// keys alone cannot prove that an object-read policy denies access.
	objects.Readiness = nil
	if err = objects.Ready(bounded); err != nil {
		return err
	}
	base, err := url.Parse(strings.TrimRight(objects.Endpoint, "/"))
	if err != nil || base.Host == "" {
		return raster.ErrUnavailable
	}
	client := http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	var keys []string
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		for _, key := range keys {
			_, deleteErr := objects.Client.DeleteObject(cleanup, &s3.DeleteObjectInput{Bucket: aws.String(objects.QuarantineBucket), Key: aws.String(key)})
			err = errors.Join(err, deleteErr)
		}
	}()
	for _, prefix := range []string{"quarantine/", "quarantine/account/"} {
		key := prefix + uuid.NewString()
		// Track even a failed write: the server may have committed it before
		// the response was lost.
		keys = append(keys, key)
		body := []byte("metadata quarantine privacy probe")
		if _, err = objects.Client.PutObject(bounded, &s3.PutObjectInput{Bucket: aws.String(objects.QuarantineBucket), Key: aws.String(key), Body: bytes.NewReader(body), ContentLength: aws.Int64(int64(len(body))), ContentType: aws.String("application/octet-stream"), IfNoneMatch: aws.String("*")}); err != nil {
			return err
		}
		request, e := http.NewRequestWithContext(bounded, http.MethodGet, base.String()+"/"+url.PathEscape(objects.QuarantineBucket)+"/"+key, nil)
		if e != nil {
			return raster.ErrUnavailable
		}
		response, e := client.Do(request)
		if e != nil {
			return raster.ErrUnavailable
		}
		_, readErr := io.Copy(io.Discard, io.LimitReader(response.Body, 65536))
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil || response.StatusCode != http.StatusForbidden {
			return raster.ErrUnavailable
		}
	}
	return nil
}
