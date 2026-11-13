package app

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"fmt"
	"log/slog"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/account"
	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/config"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	goredis "github.com/redis/go-redis/v9"
)

type accountWiring struct {
	tlsConfig  *tls.Config
	stop       context.CancelFunc
	done       chan struct{}
	natsConn   *nats.Conn
	rpcBackend *sharedauth.ReportedHashBackend
}

func connectAccountRuntime(ctx context.Context, cfg config.Config, logger *slog.Logger, pool *pgxpool.Pool, rdb *goredis.Client, store *storage.Client, mux chi.Router, accountHealth *accountRuntimeHealth) (accountWiring, error) {
	var err error
	var accountsTLS *tls.Config
	var accountStop context.CancelFunc
	var accountDone chan struct{}
	var accountNATS *nats.Conn
	var accountRPC *sharedauth.ReportedHashBackend
	var committedBlocks sharedauth.CommittedBlockProvider
	if err = cfg.ValidateAccountRuntime(); err != nil {
		return accountWiring{}, err
	}
	accountsTLS, err = accountTLS(cfg)
	if err != nil {
		return accountWiring{}, err
	}
	committedBlocks = sharedauth.PostgresCommittedBlockProvider{Pool: pool}
	accountRPC, err = dialAccountChain(ctx, cfg.AccountRPCURL)
	if err != nil {
		return accountWiring{}, err
	}
	natsAuth, e := accountNATSOptions(cfg)
	if e != nil {
		accountRPC.Close()
		return accountWiring{}, e
	}
	accountNATS, err = nats.Connect(cfg.AccountNATSURL, append(natsAuth, nats.Name("userd-"+cfg.RuntimeInstanceID), nats.Timeout(2*time.Second), nats.MaxReconnects(-1), nats.ReconnectWait(time.Second))...)
	if err != nil {
		accountRPC.Close()
		return accountWiring{}, err
	}
	setupFailure := func(e error) (accountWiring, error) {
		accountNATS.Close()
		accountRPC.Close()
		return accountWiring{}, e
	}
	publisher, e := natsdriver.NewPublisher(accountNATS, cfg.AccountNATSStream)
	if e != nil {
		return setupFailure(e)
	}
	var subjects map[string]string
	if e = config.ReadBoundDocument(cfg.AccountSubjectsFile, cfg.AccountSubjectsSHA256, &subjects); e != nil {
		return setupFailure(e)
	}
	policy := delivery.Policy{Subjects: map[string]bool{}, EventTypes: map[string]bool{}}
	for kind, subject := range subjects {
		if subject == "" || policy.Subjects[subject] {
			return setupFailure(fmt.Errorf("account_subject_configuration_invalid"))
		}
		policy.Subjects[subject] = true
		policy.EventTypes[kind] = true
	}
	durable, e := delivery.NewStore(pool, cfg.AccountDeliverySchema, policy)
	if e != nil {
		return setupFailure(e)
	}
	cursor, e := config.DecodeAccountKey(cfg.AccountCursorKey)
	if e != nil {
		return setupFailure(e)
	}
	exportKey, e := config.DecodeAccountKey(cfg.AccountExportKey)
	if e != nil {
		return setupFailure(e)
	}
	endpointKey, e := config.DecodeAccountKey(cfg.AccountEndpointKey)
	if e != nil {
		return setupFailure(e)
	}
	fingerprintKey, e := config.DecodeAccountKey(cfg.AccountEndpointFingerprintKey)
	if e != nil {
		return setupFailure(e)
	}
	auditKey, e := config.DecodeAccountKey(cfg.AccountAuditKey)
	if e != nil {
		return setupFailure(e)
	}
	signingSeed, e := config.DecodeAccountKey(cfg.AccountServiceSigningSeed)
	if e != nil {
		return setupFailure(e)
	}
	proofKey, e := config.DecodeAccountKey(cfg.AccountEndpointProofPublicKey)
	if e != nil {
		return setupFailure(e)
	}
	gatewayKey, e := config.DecodeAccountKey(cfg.AccountGatewayPublicKey)
	if e != nil {
		return setupFailure(e)
	}
	privateObjects, e := storage.New(ctx, storage.Config{Endpoint: cfg.S3Endpoint, Region: cfg.S3Region, AccessKeyID: cfg.S3AccessKeyID, SecretAccessKey: cfg.S3SecretAccessKey, Bucket: cfg.AccountExportBucket})
	if e != nil {
		return setupFailure(e)
	}
	policies, routes := sharedauth.AccountOwnerPoliciesV2()
	operations := sharedauth.OperationAuthorizer{Ledger: sharedauth.Ledger{Pool: pool, Environment: cfg.AccountEnvironment, AuditKey: auditKey}, Verifier: sharedauth.CommittedProofVerifier{Backend: accountRPC, Blocks: committedBlocks}, Policies: policies, Routes: routes, Transport: sharedauth.OperationTransport{EnvelopeHeader: "X-Kindle-Operation", SignatureHeader: "X-Kindle-Operation-Signature"}}
	userStore := account.Store{DB: pool, Clock: time.Now}
	notificationClient, e := pinnedAccountClient(accountsTLS, cfg.NotificationCertSHA256)
	if e != nil {
		return setupFailure(e)
	}
	signer := sharedauth.ServiceRequestIssuer{Issuer: "user", Audience: "identity", Key: ed25519.NewKeyFromSeed(signingSeed), Transport: sharedauth.AssertionTransport{AssertionHeader: sharedauth.IdentityAssertionHeader, SignatureHeader: sharedauth.IdentitySignatureHeader}}
	identityClient, e := pinnedAccountClient(accountsTLS, cfg.IdentityCertSHA256)
	if e != nil {
		return setupFailure(e)
	}
	identityAuthority, e := sharedauth.NewMTLSIdentityAuthority(cfg.IdentityURL, cfg.IdentityCertSHA256, identityClient, signer)
	if e != nil {
		return setupFailure(e)
	}
	metadataClient, e := pinnedAccountClient(accountsTLS, cfg.MetadataCertSHA256)
	if e != nil {
		return setupFailure(e)
	}
	metadataSigner := signer
	metadataSigner.Audience = "metadata"
	assets := accountAssetsAuthority{Store: userStore, Client: metadataClient, Endpoint: cfg.MetadataServiceURL, Signer: metadataSigner}

	recoveryVerifier := sharedauth.ServiceRequestVerifier{Issuer: "gateway", Audience: cfg.AccountAssertionAudience, PeerCertSHA256: cfg.AccountGatewayCertSHA256, Key: ed25519.PublicKey(gatewayKey), Transport: sharedauth.AssertionTransport{AssertionHeader: sharedauth.IdentityAssertionHeader, SignatureHeader: sharedauth.IdentitySignatureHeader}}
	runtime := account.Runtime{RecoveryGatewayVerifier: &recoveryVerifier, IdentityAuthority: identityAuthority, Assets: assets, ExtendedV3: cfg.AccountEnableExtendedV3, UseReadSwitch: cfg.AccountUseReadSwitch, AllowMetadataAvatars: cfg.AccountAllowMetadataAvatars, Authorizer: &operations, Objects: privateObjects, ExportKey: exportKey, EndpointKey: endpointKey, EndpointFingerprintKey: fingerprintKey, EndpointVerifier: account.CompactEndpointProofVerifier(ed25519.PublicKey(proofKey)), DeviceVerifier: account.CompactDeviceProofVerifier(ed25519.PublicKey(proofKey)), DownloadBaseURL: cfg.AccountDownloadBaseURL, Delivery: durable, Subjects: subjects, Metrics: &account.Metrics{}, DeletionOwners: map[string]account.DeletionOwner{"images": accountImageDeletionOwner{Redis: rdb, Legacy: account.ImagePurger{Store: userStore, Objects: store, Exports: privateObjects}, Assets: assets}, "notifications": notificationDeletionOwner{Client: notificationClient, Endpoint: cfg.NotificationServiceURL, Token: cfg.NotificationServiceToken, PeerCertSHA256: cfg.NotificationCertSHA256}}}
	accounts, e := account.NewConnectedHTTP(userStore, runtime, cursor)
	if e != nil {
		return setupFailure(e)
	}
	assertions := sharedauth.ServiceAssertionVerifier{Transport: sharedauth.AssertionTransport{AssertionHeader: cfg.AccountAssertionHeader, SignatureHeader: cfg.AccountAssertionSignatureHeader}, Issuer: cfg.AccountAssertionIssuer, Audience: cfg.AccountAssertionAudience, PeerCertSHA256: cfg.AccountGatewayCertSHA256, Key: ed25519.PublicKey(gatewayKey)}
	accountRoutes := accounts.GatewayHandler(assertions, sharedauth.PostgresIdentityResolver{Pool: pool}, operations)
	mux.Handle("/v1/*", accountRoutes)
	mux.Handle("/v2/*", accountRoutes)
	mux.Handle("/v3/*", accountRoutes)
	internalAccounts := accountNotificationAuthority(userStore, accounts.Runtime, cfg.NotificationInboundToken, cfg.NotificationCertSHA256)
	mux.Handle("/internal/v2/accounts/*", internalAccounts)
	mux.Handle("/internal/v2/wallets/*", internalAccounts)
	workerContext, stop := context.WithCancel(ctx)
	accountStop = stop
	accountDone = make(chan struct{})
	worker := account.Worker{Store: userStore, Runtime: accounts.Runtime, Owner: cfg.RuntimeInstanceID, LeaseDuration: time.Minute}
	relay := delivery.Relay{Store: durable, Publisher: publisher, Owner: cfg.RuntimeInstanceID, Batch: 16, LeaseDuration: 30 * time.Second, PublishTimeout: 5 * time.Second, RetryDelay: time.Second, MaxAttempts: 20}
	go runAccountWorkers(workerContext, worker, relay, accountNATS, accountHealth, accountDone, logger)
	return accountWiring{tlsConfig: accountsTLS, stop: accountStop, done: accountDone, natsConn: accountNATS, rpcBackend: accountRPC}, nil
}
