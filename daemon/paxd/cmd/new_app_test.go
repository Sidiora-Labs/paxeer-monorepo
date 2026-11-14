package cmd

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmcfg "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server"
	servertypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/types"
	"github.com/holiman/uint256"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func ReadConfig(t *testing.T, opts servertypes.AppOptions) sequencer.Config {
	t.Helper()
	cfg, err := sequencer.ReadConfig(opts)
	require.NoError(t, err)
	return cfg
}

func newAppReplicationTLS(t *testing.T) (sequencer.ReplicationTLSConfig, sequencer.ReplicationTLSConfig) {
	t.Helper()
	dir := t.TempDir()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "replication-test-root"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, public, private)
	require.NoError(t, err)
	roots := filepath.Join(dir, "roots.pem")
	require.NoError(t, os.WriteFile(roots, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	issue := func(name string, serial int64) (sequencer.ReplicationTLSConfig, string) {
		public, key, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)
		cert := &x509.Certificate{
			SerialNumber: big.NewInt(serial), DNSNames: []string{name},
			NotBefore: ca.NotBefore, NotAfter: ca.NotAfter,
			KeyUsage:    x509.KeyUsageDigitalSignature,
			ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		}
		der, err := x509.CreateCertificate(rand.Reader, cert, ca, public, private)
		require.NoError(t, err)
		parsed, err := x509.ParseCertificate(der)
		require.NoError(t, err)
		digest := sha256.Sum256(parsed.RawSubjectPublicKeyInfo)
		auth := sequencer.ReplicationTLSConfig{
			CAFile: roots, CertFile: filepath.Join(dir, name+".pem"), KeyFile: filepath.Join(dir, name+".key"),
		}
		require.NoError(t, os.WriteFile(auth.CertFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
		encodedKey, err := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(auth.KeyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encodedKey}), 0o600))
		return auth, hex.EncodeToString(digest[:])
	}
	primary, primaryPin := issue("primary.test", 2)
	standby, standbyPin := issue("standby.test", 3)
	primary.PeerName, primary.PeerSPKI = "standby.test", standbyPin
	standby.PeerName, standby.PeerSPKI = "primary.test", primaryPin
	return primary, standby
}

func newAppReplicationStandby(t *testing.T, opts *viper.Viper, state []byte, start time.Time, primary *privval.FilePV, keyPath string, auth sequencer.ReplicationTLSConfig) string {
	t.Helper()
	home := t.TempDir()
	standbyOpts := viper.New()
	require.NoError(t, standbyOpts.MergeConfigMap(opts.AllSettings()))
	standbyOpts.Set(flags.FlagHome, home)
	cfg := tmcfg.DefaultConfig().SetRoot(home)
	standbyApp := newApp(dbm.NewMemDB(), nil, cfg, standbyOpts).(*sequencedApp)
	t.Cleanup(func() { require.NoError(t, standbyApp.Close()) })
	_, err := standbyApp.InitChain(t.Context(), &abci.RequestInitChain{
		Time: start, ChainId: "pax-test", ConsensusParams: app.DefaultConsensusParams, AppStateBytes: state,
	})
	require.NoError(t, err)
	_, err = standbyApp.FinalizeBlock(t.Context(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{ChainID: "pax-test", Height: 1, Time: start},
	})
	require.NoError(t, err)
	standbyApp.SetDeliverStateToCommit()
	_, err = standbyApp.Commit(t.Context())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(cfg.PrivValidator.KeyFile()), 0o700))
	standbyPV, err := privval.GenFilePV(cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile(), "")
	require.NoError(t, err)
	_, standbyReceipt, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	pins, err := standbyApp.SequencerKeeper.ReceiptKeyPins(standbyApp.GetCheckCtx())
	require.NoError(t, err)
	primaryConsensus := tmcfg.DefaultConfig().SetRoot(opts.GetString(flags.FlagHome)).PrivValidator
	key, err := sequencer.LoadReceiptKey(keyPath, primaryConsensus.KeyFile(), pins, 2)
	require.NoError(t, err)
	exec, err := sequencer.NewExecutor(standbyApp.App, sequencer.Options{
		ChainID: "pax-test", ProposerAddress: primary.Key.PubKey.Address(),
	})
	require.NoError(t, err)
	receipter, err := sequencer.NewReceipter(exec, standbyApp.App, &standbyApp.EvmKeeper, key, app.EVMReceiptModuleID)
	require.NoError(t, err)
	journal, err := wal.OpenWAL(filepath.Join(home, "sequencer-wal"), wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(journal.Close)
	public := key.PublicKey()
	standby, err := sequencer.NewStandby(t.Context(), sequencer.StandbyConfig{
		WAL: journal, Executor: sequencer.NewAppReplayer(exec, receipter),
		ConsensusKey: standbyPV, ReceiptKey: standbyReceipt,
		PrimaryConsensusKey: primary.Key.PubKey, PrimaryReceiptKey: public[:], CommittedHeight: 1,
	})
	require.NoError(t, err)
	tlsConfig, err := auth.TLSConfig(true)
	require.NoError(t, err)
	listener, err := tls.Listen("tcp", "127.0.0.1:0", tlsConfig)
	require.NoError(t, err)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = standby.Serve(listener)
	}()
	t.Cleanup(func() {
		require.NoError(t, listener.Close())
		<-done
	})
	return listener.Addr().String()
}

func TestNewAppPassesSequencerConfigToNode(t *testing.T) {
	for _, mode := range []string{"sequential", "microbatch"} {
		t.Run(mode, func(t *testing.T) {
			home := t.TempDir()
			cfg := tmcfg.DefaultConfig().SetRoot(home)
			require.NoError(t, os.MkdirAll(filepath.Dir(cfg.PrivValidator.KeyFile()), 0o700))
			require.NoError(t, os.MkdirAll(filepath.Dir(cfg.PrivValidator.StateFile()), 0o700))
			pv, err := privval.GenFilePV(cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile(), "")
			require.NoError(t, err)
			require.NoError(t, pv.Save())
			public, private, err := ed25519.GenerateKey(nil)
			require.NoError(t, err)
			keyPath := filepath.Join(home, "receipt-key")
			require.NoError(t, os.WriteFile(keyPath, []byte(hex.EncodeToString(private.Seed())+"\n"), 0o600))

			opts := viper.New()
			opts.Set(flags.FlagHome, home)
			opts.Set(flags.FlagChainID, "pax-test")
			opts.Set(server.FlagPruning, "nothing")
			opts.Set(server.FlagMinGasPrices, "0uhpx")
			opts.Set(app.FlagSCEnable, true)
			opts.Set(app.FlagSSEnable, true)
			opts.Set(app.FlagSSBackend, "pebbledb")
			opts.Set("evm.http_enabled", false)
			opts.Set("evm.ws_enabled", false)
			dbDir := t.TempDir()
			database, err := dbm.NewGoLevelDB("application", dbDir)
			require.NoError(t, err)
			initial := newApp(database, nil, cfg, opts).(*sequencedApp)
			require.Nil(t, initial.SequencerNode())
			genesis := app.NewDefaultGenesisState(app.MakeEncodingConfig().Marshaler)
			keyID := [32]byte{1}
			genesis[seqtypes.ModuleName], err = json.Marshal(struct {
				ActivationHeight uint64                `json:"activation_height"`
				Leaders          [][]byte              `json:"leaders"`
				ReceiptKeys      seqkeeper.ReceiptKeys `json:"receipt_keys"`
			}{
				ActivationHeight: 2,
				Leaders:          [][]byte{pv.Key.PubKey.Bytes()},
				ReceiptKeys: seqkeeper.ReceiptKeys{{
					KeyId: keyID[:], PublicKey: public,
					FirstHeight: 1, LastHeight: math.MaxUint64,
				}},
			})
			require.NoError(t, err)
			state, err := json.Marshal(genesis)
			require.NoError(t, err)
			start := time.Now().UTC()
			_, err = initial.InitChain(t.Context(), &abci.RequestInitChain{
				Time: start, ChainId: "pax-test", ConsensusParams: app.DefaultConsensusParams, AppStateBytes: state,
			})
			require.NoError(t, err)
			_, err = initial.FinalizeBlock(t.Context(), &abci.RequestFinalizeBlock{
				Header: &tmproto.Header{ChainID: "pax-test", Height: 1, Time: start},
			})
			require.NoError(t, err)
			initial.SetDeliverStateToCommit()
			_, err = initial.Commit(t.Context())
			require.NoError(t, err)
			require.NoError(t, initial.Close())

			auth, standbyAuth := newAppReplicationTLS(t)
			standbyAddress := newAppReplicationStandby(t, opts, state, start, pv, keyPath, standbyAuth)
			opts.Set(sequencer.FlagEnable, true)
			opts.Set(sequencer.FlagMode, mode)
			opts.Set(sequencer.FlagWALDir, filepath.Join(home, "sequencer-wal"))
			opts.Set(sequencer.FlagReceiptKeyFile, keyPath)
			opts.Set(sequencer.FlagStandbyAddress, standbyAddress)
			opts.Set(sequencer.FlagReplicationTLSCert, auth.CertFile)
			opts.Set(sequencer.FlagReplicationTLSKey, auth.KeyFile)
			opts.Set(sequencer.FlagReplicationTLSCA, auth.CAFile)
			opts.Set(sequencer.FlagReplicationPeerName, auth.PeerName)
			opts.Set(sequencer.FlagReplicationPeerSPKI, auth.PeerSPKI)
			opts.Set(sequencer.FlagReplicationTimeout, 2*time.Second)
			opts.Set(sequencer.FlagWALRetainBlocks, uint64(7))
			opts.Set(sequencer.FlagNonceQueueMaxPerSender, 3)
			opts.Set(sequencer.FlagNonceQueueMaxTotal, 11)
			opts.Set(sequencer.FlagMicroBatchWindow, 5*time.Millisecond)
			opts.Set(sequencer.FlagMicroBatchMaxCount, 4)
			expected := ReadConfig(t, opts)
			journal, err := wal.OpenWAL(expected.WALDir, wal.DefaultConfig())
			require.NoError(t, err)
			openTime := start.Add(time.Second).UnixMilli()
			_, err = journal.Append(&wal.Open{Height: 2, OpenTimeMs: uint64(openTime), BaseFee: uint256.NewInt(0)})
			require.NoError(t, err)
			_, err = journal.GroupSync()
			require.NoError(t, err)
			journal.Close()

			database, err = dbm.NewGoLevelDB("application", dbDir)
			require.NoError(t, err)
			application := newApp(database, nil, cfg, opts)
			closed := false
			t.Cleanup(func() {
				if !closed {
					require.NoError(t, application.Close())
				}
			})
			sequenced, ok := application.(interface{ SequencerNode() *sequencer.Node })
			require.True(t, ok)
			node := sequenced.SequencerNode()
			require.NotNil(t, node)
			require.Equal(t, expected, node.Config)
			require.Same(t, application.(*sequencedApp).App, node.App)
			require.Equal(t, expected.Mode, node.Executor.Mode())
			require.Equal(t, [32]byte(public), node.ReceiptKey.PublicKey())
			require.Same(t, node.ConsensusSource, node.App.SequencerSource())
			pending, err := node.App.PendingSnapshot()
			require.NoError(t, err)
			require.Equal(t, int64(2), pending.BlockHeight())
			require.Equal(t, openTime, pending.BlockTime().UnixMilli())
			require.NoError(t, application.Close())
			closed = true
			_, err = node.Accepter.Accept([]byte{1})
			require.ErrorIs(t, err, sequencer.ErrReplicationHalted)
			reopened, err := wal.OpenWAL(expected.WALDir, wal.DefaultConfig())
			require.NoError(t, err)
			reopened.Close()
		})
	}
}

func TestNewAppRefusesSequencerWithoutWALDir(t *testing.T) {
	opts := viper.New()
	opts.Set(flags.FlagHome, t.TempDir())
	opts.Set(server.FlagPruning, "nothing")
	opts.Set(sequencer.FlagEnable, true)
	database := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	require.PanicsWithValue(t, sequencer.ErrConfigWALDir, func() {
		newApp(database, nil, tmcfg.DefaultConfig(), opts)
	})
}

func TestNewAppRefusesReceiptForwardURLWhenSequencingEnabled(t *testing.T) {
	opts := viper.New()
	opts.Set(flags.FlagHome, t.TempDir())
	opts.Set(server.FlagPruning, "nothing")
	opts.Set(sequencer.FlagEnable, true)
	opts.Set("sequencer.receipt-forward-url", "https://sequencer.example/receipts")
	database := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	require.PanicsWithError(t,
		"sequencer config: invalid value: sequencer.receipt-forward-url cannot be set when sequencer.enable is true",
		func() { newApp(database, nil, tmcfg.DefaultConfig(), opts) })
}
