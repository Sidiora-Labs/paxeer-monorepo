# Sequencer contract

The sequencer runtime is integrated into `paxd`. [Node startup](../../../daemon/paxd/cmd/root.go) creates it with `sequencer.NewNode` and starts it with the application; application shutdown closes it. The runtime executes and signs receipts before block commitment, while consensus commits the resulting blocks. Paxeer mainnet-beta uses EVM chain ID `125` (`0x7d`), and finality follows block commitment.

The tables identify code declarations and configuration names. Activation, authentication and transport registration are defined by the linked implementations.

## Signed receipt v2

[Receipter.Accept](../../../node/sequencer/receipt.go) executes a transaction on open block state and signs a protocol-v2 receipt. The envelope uses structure tag `0x5201` and layout version `1`. Array fields carry a big-endian `u32` length prefix; addresses occupy zero-left-padded 32-byte words. Integers are big-endian.

The signed envelope binds the transaction identifier, global sequence, previous and resulting sequence hashes, return-data hash, result status, effects, charged fee, batch height, module, operation, payment asset, amount, sender, recipient, balances, nonce, authorization hash, signing-key identifier and timestamp. The payment asset is `sha256(paymentDenom)`; the summary separately carries the fee denomination. Payment and fee denominations need not be equal.

The signing preimage is `sha256("LXP/v1/receipt\0" || unsignedReceipt)`. An unsigned receipt ends in flag `0`; a signed receipt ends in flag `1`, a big-endian length `64` and the Ed25519 signature. [Receipt keys](../../../node/sequencer/receiptkey.go) and [the verifier](../../../modules/sequencer/keeper/receipt_keys.go) define signing and verification.

## Receipt key pin and rotation

Governed keys bind a nonzero key ID and canonical Ed25519 public key to an inclusive `[FirstHeight, LastHeight]` range. `FirstHeight` is at least one. Verification selects the key using the receipt's context hash and batch height. A signing key returned beside a receipt does not replace independently trusted pins. The node's `px_getReceiptKeys` implementation publishes the governed list; the hosted gateway relays that list under its declared method contract.

## Rungs: submitted, receipted, committed

| Rung | Evidence |
|---|---|
| `submitted` | Transaction hash and nonce; execution receipt absent. |
| `receipted` | Signed execution receipt on open block state. |
| `committed` | Receipt covered by a committed block and its sequence batch. |

[SequencerReceiptResult.Verify](../../../rpc/sequencer_receipt.go) checks fields, hashes, logs, signature and rung consistency. Caller-supplied pins authenticate signing authority. `eth_getTransactionReceipt` serves committed EVM receipts. A node without a local receipt source or configured forwarder refuses the sequencer receipt request.

<a id="sections"></a>

<a id="envelope-fields-in-canonical-order"></a>

## Effects

The summary effect encodes gas used, cumulative gas used, effective gas price, transaction index, contract address, sponsor, VM-error hash, return-data length, log count and fee denomination. Each log effect binds emitter, topic count, topics and `sha256(data)`. [SummaryEffect and LogEffect](../../../node/sequencer/receipt.go) enforce the field widths and bounds.

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `var ErrReplicationHalted = errors.New("sequencer: replication halted")` | [node/sequencer/accepter.go](../../../node/sequencer/accepter.go) |
| `go_exported` | `func NewAccepter(r *Receipter, rep *Replicator, src *ReceiptSource, m *Metrics) *Accepter` | [node/sequencer/accepter.go](../../../node/sequencer/accepter.go) |
| `go_exported` | `func NewFollowerReceiptOption(cfg Config) (evmrpc.SequencerReceiptOption, bool)` | [node/sequencer/follower.go](../../../node/sequencer/follower.go) |
| `go_exported` | `func NewLogFeedPublisher(feed *evmrpc.ReceiptLogFeed) *LogFeedPublisher` | [node/sequencer/log_feed.go](../../../node/sequencer/log_feed.go) |
| `go_exported` | `type ReceiptSource struct` | [node/sequencer/receipt_source.go](../../../node/sequencer/receipt_source.go) |
| `file_format` | `func BuildReceipt(in *ReceiptInput, contextHash [32]byte) ([]byte, error)` | [node/sequencer/receipt.go](../../../node/sequencer/receipt.go) |
| `file_format` | `func encodeReceipt(in *ReceiptInput, contextHash [32]byte) ([]byte, error)` | [node/sequencer/receipt.go](../../../node/sequencer/receipt.go) |
| `file_format` | `receiptProtocolVersion uint16 = 2` | [node/sequencer/receipt.go](../../../node/sequencer/receipt.go) |

## MsgSequenceBatch (tx 0 of a sequenced block)

[EncodeSequenceBatchTx](../../../modules/sequencer/types/batch_tx.go) encodes the domain prefix, height, sequence range, previous sequence hash, last sequence hash, transaction root, key ID and signature. [VerifyBatchTxs](../../../modules/sequencer/types/batch_verify.go) binds it to the ordered activity identifiers. Batch messages are block-only and are refused through ordinary transaction admission.

| Kind | Declaration | Source |
|---|---|---|
| `file_format` | `const sequenceBatchDomain = seqtypes.SequenceBatchDomain` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func DecodeSequenceBatchTx(tx []byte) (MsgSequenceBatch, error)` | [modules/sequencer/types/batch_tx.go](../../../modules/sequencer/types/batch_tx.go) |
| `go_exported` | `func EncodeSequenceBatchTx(m MsgSequenceBatch) ([]byte, error)` | [modules/sequencer/types/batch_tx.go](../../../modules/sequencer/types/batch_tx.go) |
| `go_exported` | `func VerifyBatchTxs(m MsgSequenceBatch, activityIDs [][32]byte) error` | [modules/sequencer/types/batch_verify.go](../../../modules/sequencer/types/batch_verify.go) |
| `go_exported` | `func SeqHash(prev [32]byte, seq uint64, activityID [32]byte) [32]byte` | [modules/sequencer/types/seqhash.go](../../../modules/sequencer/types/seqhash.go) |
| `go_exported` | `func TxRoot(activityIDs [][32]byte) [32]byte` | [modules/sequencer/types/txroot.go](../../../modules/sequencer/types/txroot.go) |
| `file_format` | `func (m MsgSequenceBatch) SignBytes(chainID string) ([HashLength]byte, error)` | [modules/sequencer/types/msgs.go](../../../modules/sequencer/types/msgs.go) |
| `go_exported` | `var EmptyTxRoot = sha256.Sum256(nil)` | [modules/sequencer/types/msgs.go](../../../modules/sequencer/types/msgs.go) |
| `go_exported` | `ErrBadSignature = sdkerrors.Register(ModuleName, 3, "bad sequence batch signature")` | [modules/sequencer/types/msgs.go](../../../modules/sequencer/types/msgs.go) |
| `go_exported` | `ErrInvalidSequenceBatch = sdkerrors.Register(ModuleName, 2, "invalid sequence batch")` | [modules/sequencer/types/msgs.go](../../../modules/sequencer/types/msgs.go) |
| `go_exported` | `HashLength = 32` | [modules/sequencer/types/msgs.go](../../../modules/sequencer/types/msgs.go) |
| `go_exported` | `KeyIDLength = 32` | [modules/sequencer/types/msgs.go](../../../modules/sequencer/types/msgs.go) |
| `go_exported` | `type MsgSequenceBatch struct` | [modules/sequencer/types/msgs.go](../../../modules/sequencer/types/msgs.go) |
| `go_exported` | `func (m MsgSequenceBatch) IsEmpty() bool { return m.ToSeq == m.FromSeq-1 }` | [modules/sequencer/types/msgs.go](../../../modules/sequencer/types/msgs.go) |
| `go_exported` | `func (m MsgSequenceBatch) SignBytes(chainID string) ([HashLength]byte, error)` | [modules/sequencer/types/msgs.go](../../../modules/sequencer/types/msgs.go) |
| `go_exported` | `func (m MsgSequenceBatch) ValidateBasic() error` | [modules/sequencer/types/msgs.go](../../../modules/sequencer/types/msgs.go) |
| `go_exported` | `func (m MsgSequenceBatch) VerifySignature(chainID string, publicKey [32]byte) error` | [modules/sequencer/types/msgs.go](../../../modules/sequencer/types/msgs.go) |
| `go_exported` | `SignatureLength = 64` | [modules/sequencer/types/msgs.go](../../../modules/sequencer/types/msgs.go) |
| `cosmos_msg` | `type MsgSequenceBatch struct` | [modules/sequencer/types/msgs.go](../../../modules/sequencer/types/msgs.go) |
| `file_format` | `func EncodeSequenceBatch(m seqtypes.MsgSequenceBatch) ([]byte, error)` | [node/app.go](../../../node/app.go) |

## Sequencer WAL and standby replication

WAL frames are `u32be crc32c(entry) || u32be len(entry) || entry`; entry version is `1`. Entries above `8 MiB` are refused. Entry kinds include OPEN, ACCEPT, SEAL, COMMITTED and LEASE. Segments default to `64 MiB`. [The WAL](../../../node/sequencer/wal/wal.go) validates framing and replays persisted state.

[The accepter](../../../node/sequencer/accepter.go) releases a signed receipt after durable replication acknowledgement. Replication failure fences further acceptance. The driver commits, prunes retained WAL state and opens the next block; startup recovery replays persisted entries.

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `type ReplayResult struct` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `func (w *WAL) Replay(exec Executor) (ReplayResult, error)` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `func DialStandby(address string, timeout time.Duration, auth ...ReplicationTLSConfig) func(ctx context.Context) (net.Conn, error) {` | [node/sequencer/dial.go](../../../node/sequencer/dial.go) |
| `go_exported` | `func NewDriver(cfg DriverConfig) (*Driver, error) {` | [node/sequencer/driver.go](../../../node/sequencer/driver.go) |
| `go_exported` | `ErrDivergentLogs = errors.New("sequencer: two logs hold different receipts for one seq")` | [node/sequencer/failover.go](../../../node/sequencer/failover.go) |
| `go_exported` | `func Failover(s *Standby, key *ReceiptKey, openHeight int64, expiryMs uint64) (FailoverResult, error) {` | [node/sequencer/failover.go](../../../node/sequencer/failover.go) |
| `go_exported` | `type FailoverResult struct {` | [node/sequencer/failover.go](../../../node/sequencer/failover.go) |
| `go_exported` | `func RecoveryOrder(a, b wal.ReplayResult) (int, error)` | [node/sequencer/failover.go](../../../node/sequencer/failover.go) |
| `go_exported` | `type Fencer interface { Fence() error }` | [node/sequencer/handover.go](../../../node/sequencer/handover.go) |
| `go_exported` | `type HandoverConfig struct {` | [node/sequencer/handover.go](../../../node/sequencer/handover.go) |
| `go_exported` | `func Handover(ctx context.Context, cfg HandoverConfig) (HandoverResult, error)` | [node/sequencer/handover.go](../../../node/sequencer/handover.go) |
| `go_exported` | `func (r *Replicator) Drain(ctx context.Context) (Ack, error)` | [node/sequencer/handover.go](../../../node/sequencer/handover.go) |
| `go_exported` | `type LeaseHolder struct` | [node/sequencer/lease.go](../../../node/sequencer/lease.go) |
| `go_exported` | `func NewAppReplayer(exec Executor, r *Receipter) *AppReplayer` | [node/sequencer/replay.go](../../../node/sequencer/replay.go) |
| `go_exported` | `ErrFenced = errors.New("sequencer: a later lease fences this primary")` | [node/sequencer/standby.go](../../../node/sequencer/standby.go) |
| `go_exported` | `func (r *Replicator) Epoch() uint64` | [node/sequencer/standby.go](../../../node/sequencer/standby.go) |
| `go_exported` | `func (s *Standby) TakeLease(l *wal.Lease) error` | [node/sequencer/standby.go](../../../node/sequencer/standby.go) |
| `file_format` | `func EncodeEntry(e Entry) ([]byte, error)` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `file_format` | `func NewFrame(e Entry) (Frame, error)` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `file_format` | `type Accept struct` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `file_format` | `type Committed struct` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `file_format` | `type Lease struct` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `file_format` | `type Open struct` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `file_format` | `type Seal struct` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `file_format` | `const Version byte = 1` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `type Accept struct` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `func (*Accept) Kind() EntryKind { return KindAccept }` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `type Committed struct` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `func (*Committed) Kind() EntryKind { return KindCommitted }` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `type Config struct` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `type CorruptTail struct` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `func DecodeEntry(b []byte) (Entry, error)` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `func DefaultConfig() Config { return Config{SegmentSizeLimit: 64 << 20} }` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `func EncodeEntry(e Entry) ([]byte, error)` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `type Entry interface` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `type EntryKind uint8` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `func (k EntryKind) String() string` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `ErrChecksum = errors.New("wal frame checksum mismatch")` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `ErrClosed = errors.New("sequencer wal closed")` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `ErrCorruptSegment = errors.New("sealed wal segment corrupt")` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `ErrEntryTooLarge = errors.New("wal entry too large")` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `ErrMalformed = errors.New("malformed wal entry")` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `ErrReceiptMismatch = errors.New("replayed receipt differs from stored receipt")` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `ErrTorn = errors.New("wal frame torn")` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `ErrUnknownKind = errors.New("unknown wal entry kind")` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `ErrUnknownVersion = errors.New("unknown wal entry version")` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `type Executor interface` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `type Frame []byte` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `KindAccept EntryKind = 2` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `KindCommitted EntryKind = 4` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `KindLease EntryKind = 5` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `KindOpen EntryKind = 1` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `KindSeal EntryKind = 3` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `type Lease struct` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `func (*Lease) Kind() EntryKind { return KindLease }` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `const MaxEntrySize = 8 << 20` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `func NewFrame(e Entry) (Frame, error)` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `type Open struct` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `func (*Open) Kind() EntryKind { return KindOpen }` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `func OpenWAL(dir string, cfg Config) (w *WAL, err error)` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `type Pending struct` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `func (p *Pending) Wait(ctx context.Context) error` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `type Seal struct` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `func (*Seal) Kind() EntryKind { return KindSeal }` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `const Version byte = 1` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `type WAL struct` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `func (w *WAL) Append(e Entry) (*Pending, error)` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `func (w *WAL) Close()` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `func (w *WAL) GroupSync() (int, error)` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `func (w *WAL) LastCommitted() *Committed` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `go_exported` | `func (w *WAL) Prune(retainHeight uint64) (int, error)` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `storage_key` | `headName = "head.seg"` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `storage_key` | `lockName = "wal.lock"` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `storage_key` | `sealedSuffix = ".seg"` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |
| `storage_key` | `func (s segment) name() string` | [node/sequencer/wal/wal.go](../../../node/sequencer/wal/wal.go) |

<a id="executor-nonce-rules-and-fee-rules-at-receipt"></a>

<a id="nonce-rules"></a>

## Fee rules

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `type Config struct` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `go_exported` | ``const ConfigTemplate = ` `` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `go_exported` | `func (c Config) Validate() error` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `go_exported` | `func ReadConfig(opts servertypes.AppOptions) (Config, error)` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `go_exported` | `func (s *ConsensusSource) Seal(ctx context.Context, height int64) (tmtypes.SealedBlock, error) {` | [node/sequencer/consensus_source.go](../../../node/sequencer/consensus_source.go) |
| `go_exported` | `func NewConsensusSource(a *app.App, exec Executor, key *ReceiptKey, opts ...ConsensusSourceOption) (*ConsensusSource, error) {` | [node/sequencer/consensus_source.go](../../../node/sequencer/consensus_source.go) |
| `file_format` | `var seqHashDomain = []byte(seqtypes.SeqHashDomain)` | [node/sequencer/executor.go](../../../node/sequencer/executor.go) |
| `go_exported` | `type Host interface` | [node/sequencer/executor.go](../../../node/sequencer/executor.go) |
| `go_exported` | `func NewExecutor(host Host, opts Options) (Executor, error)` | [node/sequencer/executor.go](../../../node/sequencer/executor.go) |
| `go_exported` | `func NewFeeFloor(exec Executor, host Host, minGasPrices string, params paramskeeper.Keeper, oracle oraclekeeper.Keeper, pricefeed pricefeedkeeper.Keeper, evm *evmkeeper.Keeper) (*FeeFloor, error) {` | [node/sequencer/fee.go](../../../node/sequencer/fee.go) |
| `go_exported` | `func NewMicroBatch(host BatchHost, opts Options, mb MicroBatchOptions) (Executor, error)` | [node/sequencer/microbatch.go](../../../node/sequencer/microbatch.go) |
| `go_exported` | `func RefreshPrecompiles(host PrecompileHost, exec Executor) error` | [node/sequencer/precompiles.go](../../../node/sequencer/precompiles.go) |
| `go_exported` | `func SetTxPosition(ctx sdk.Context, txIndex int) (sdk.Context, error)` | [node/sequencer/precompiles.go](../../../node/sequencer/precompiles.go) |
| `go_exported` | `func NewReceipter(exec Executor, host Host, evm *evmkeeper.Keeper, key *ReceiptKey, moduleID uint16) (*Receipter, error) {` | [node/sequencer/receipt.go](../../../node/sequencer/receipt.go) |
| `go_exported` | `func (r *Receipter) Accept(txBytes []byte) (*Accepted, []byte, error)` | [node/sequencer/receipt.go](../../../node/sequencer/receipt.go) |
| `go_exported` | `func LoadReceiptKey(keyFile, consensusKeyFile string, pins []verify.SequencerAuthorization, openHeight int64) (*ReceiptKey, error) {` | [node/sequencer/receiptkey.go](../../../node/sequencer/receiptkey.go) |
| `go_exported` | `type Ack struct` | [node/sequencer/standby.go](../../../node/sequencer/standby.go) |
| `go_exported` | `func NewReplicator(cfg ReplicatorConfig) (*Replicator, error)` | [node/sequencer/standby.go](../../../node/sequencer/standby.go) |
| `go_exported` | `func NewStandby(ctx context.Context, cfg StandbyConfig) (*Standby, error)` | [node/sequencer/standby.go](../../../node/sequencer/standby.go) |
| `go_exported` | `func (r *Replicator) Release(ctx context.Context, entries ...wal.Entry) (Ack, error)` | [node/sequencer/standby.go](../../../node/sequencer/standby.go) |
| `go_exported` | `func NewPromotedConsensusSource(a *app.App, host Host, opts Options, lastSeq uint64, key *ReceiptKey) (*ConsensusSource, error) {` | [node/sequencer/consensus_source.go](../../../node/sequencer/consensus_source.go) |
| `go_exported` | `Fence() error` | [node/sequencer/executor.go](../../../node/sequencer/executor.go) |
| `go_exported` | `func NewNonceAdmission(cfg Config, nonces NextNonceReader, accept AcceptFunc, lifetime time.Duration) (*NonceAdmission, error) {` | [node/sequencer/nonce_admission.go](../../../node/sequencer/nonce_admission.go) |
| `go_exported` | `func (c Config) MicroBatchOptions() MicroBatchOptions` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `go_exported` | `func DefaultConfig() Config` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `go_exported` | `ErrConfigMode = errors.New("sequencer config: unknown mode")` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `go_exported` | `ErrConfigReceiptKeyFile = errors.New("sequencer config: receipt-key-file is required")` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `go_exported` | `ErrConfigReplicationTimeout = errors.New("sequencer config: invalid replication-timeout")` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `go_exported` | `ErrConfigStandbyAddress = errors.New("sequencer config: invalid standby-address")` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `go_exported` | `ErrConfigValue = errors.New("sequencer config: invalid value")` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `go_exported` | `ErrConfigWALDir = errors.New("sequencer config: wal-dir is required")` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `go_exported` | `FlagEnable = "sequencer.enable"` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `go_exported` | `FlagMicroBatchMaxCount = "sequencer.microbatch-max-count"` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `go_exported` | `FlagMicroBatchWindow = "sequencer.microbatch-window"` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `go_exported` | `FlagMode = "sequencer.mode"` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `go_exported` | `FlagReceiptKeyFile = "sequencer.receipt-key-file"` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `go_exported` | `FlagReplicationTimeout = "sequencer.replication-timeout"` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `go_exported` | `FlagStandbyAddress = "sequencer.standby-address"` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `go_exported` | `FlagWALDir = "sequencer.wal-dir"` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `go_exported` | `func (m Mode) String() string` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `go_exported` | `func ParseMode(name string) (Mode, error)` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `go_exported` | `type ConsensusSource struct` | [node/sequencer/consensus_source.go](../../../node/sequencer/consensus_source.go) |
| `go_exported` | `func (s *ConsensusSource) Sealed() *SealedLayer` | [node/sequencer/consensus_source.go](../../../node/sequencer/consensus_source.go) |
| `go_exported` | `func (s *ConsensusSource) Sequenced(height int64) bool` | [node/sequencer/consensus_source.go](../../../node/sequencer/consensus_source.go) |
| `go_exported` | `var ErrNotSequenced = errors.New("sequencer: height is not sequenced")` | [node/sequencer/consensus_source.go](../../../node/sequencer/consensus_source.go) |
| `go_exported` | `type Accepted struct` | [node/sequencer/executor.go](../../../node/sequencer/executor.go) |
| `go_exported` | `type Batch struct` | [node/sequencer/executor.go](../../../node/sequencer/executor.go) |
| `go_exported` | `ErrBeginBlock = errors.New("sequencer: begin block failed")` | [node/sequencer/executor.go](../../../node/sequencer/executor.go) |
| `go_exported` | `ErrLayerOpen = errors.New("sequencer: a block is already open")` | [node/sequencer/executor.go](../../../node/sequencer/executor.go) |
| `go_exported` | `ErrOpenHeight = errors.New("sequencer: open height is not the next height")` | [node/sequencer/executor.go](../../../node/sequencer/executor.go) |
| `go_exported` | `ErrOpenTimestamp = errors.New("sequencer: open timestamp must be positive")` | [node/sequencer/executor.go](../../../node/sequencer/executor.go) |
| `go_exported` | `ErrSealHeight = errors.New("sequencer: seal height is not the open height")` | [node/sequencer/executor.go](../../../node/sequencer/executor.go) |
| `go_exported` | `ErrSeqExhausted = errors.New("sequencer: sequence number space exhausted")` | [node/sequencer/executor.go](../../../node/sequencer/executor.go) |
| `go_exported` | `type Executor interface` | [node/sequencer/executor.go](../../../node/sequencer/executor.go) |
| `go_exported` | `type Mode uint8` | [node/sequencer/executor.go](../../../node/sequencer/executor.go) |
| `go_exported` | `const ModeSequential Mode = 0` | [node/sequencer/executor.go](../../../node/sequencer/executor.go) |
| `go_exported` | `type Options struct` | [node/sequencer/executor.go](../../../node/sequencer/executor.go) |
| `go_exported` | `type PendingTx = evmrpc.PendingTx` | [node/sequencer/executor.go](../../../node/sequencer/executor.go) |
| `go_exported` | `type FeeFloor struct` | [node/sequencer/fee.go](../../../node/sequencer/fee.go) |
| `go_exported` | `func (f *FeeFloor) Accept(txBytes []byte) (*Accepted, error)` | [node/sequencer/fee.go](../../../node/sequencer/fee.go) |
| `go_exported` | `func (f *FeeFloor) BaseFeePerGas() (sdk.Dec, error)` | [node/sequencer/fee.go](../../../node/sequencer/fee.go) |
| `go_exported` | `func (f *FeeFloor) CheckNodeFloor(ctx sdk.Context, tx sdk.Tx) error` | [node/sequencer/fee.go](../../../node/sequencer/fee.go) |
| `go_exported` | `func (f *FeeFloor) Open(height int64, timestampMs int64) error` | [node/sequencer/fee.go](../../../node/sequencer/fee.go) |
| `go_exported` | `func (f *FeeFloor) Seal(height int64) (*Batch, error)` | [node/sequencer/fee.go](../../../node/sequencer/fee.go) |
| `go_exported` | `ErrBeginBlockPanic = errors.New("sequencer: begin block panicked")` | [node/sequencer/layers.go](../../../node/sequencer/layers.go) |
| `go_exported` | `ErrBlockFull = errors.New("sequencer: transaction does not fit the block limits")` | [node/sequencer/layers.go](../../../node/sequencer/layers.go) |
| `go_exported` | `ErrDeliverTxPanicked = errors.New("sequencer: deliver tx panicked")` | [node/sequencer/layers.go](../../../node/sequencer/layers.go) |
| `go_exported` | `ErrHeightMismatch = errors.New("sequencer: height does not follow the layer below")` | [node/sequencer/layers.go](../../../node/sequencer/layers.go) |
| `go_exported` | `ErrMaliciousGasless = errors.New("sequencer: gasless check panicked")` | [node/sequencer/layers.go](../../../node/sequencer/layers.go) |
| `go_exported` | `ErrNoBlockParams = errors.New("sequencer: consensus block params are not set")` | [node/sequencer/layers.go](../../../node/sequencer/layers.go) |
| `go_exported` | `ErrNoBlockSpace = errors.New("sequencer: block max bytes leave no room for transactions")` | [node/sequencer/layers.go](../../../node/sequencer/layers.go) |
| `go_exported` | `ErrNoOpenLayer = errors.New("sequencer: no open layer")` | [node/sequencer/layers.go](../../../node/sequencer/layers.go) |
| `go_exported` | `ErrNoSealedLayer = errors.New("sequencer: no sealed layer")` | [node/sequencer/layers.go](../../../node/sequencer/layers.go) |
| `go_exported` | `ErrSealedPending = errors.New("sequencer: the sealed layer is not committed yet")` | [node/sequencer/layers.go](../../../node/sequencer/layers.go) |
| `go_exported` | `ErrUndecodableTx = errors.New("sequencer: transaction does not decode")` | [node/sequencer/layers.go](../../../node/sequencer/layers.go) |
| `go_exported` | `ErrUpgradeHeight = errors.New("sequencer: an upgrade plan executes at this height")` | [node/sequencer/layers.go](../../../node/sequencer/layers.go) |
| `go_exported` | `type SealedLayer struct` | [node/sequencer/layers.go](../../../node/sequencer/layers.go) |
| `go_exported` | `func (s *SealedLayer) Context() sdk.Context` | [node/sequencer/layers.go](../../../node/sequencer/layers.go) |
| `go_exported` | `type SequenceBatchFields struct` | [node/sequencer/layers.go](../../../node/sequencer/layers.go) |
| `go_exported` | `type BatchHost interface` | [node/sequencer/microbatch.go](../../../node/sequencer/microbatch.go) |
| `go_exported` | `ErrMicroBatchMode = errors.New("sequencer: micro-batch executor needs ModeMicroBatch")` | [node/sequencer/microbatch.go](../../../node/sequencer/microbatch.go) |
| `go_exported` | `ErrMicroBatchWindow = errors.New("sequencer: micro-batch window must be positive")` | [node/sequencer/microbatch.go](../../../node/sequencer/microbatch.go) |
| `go_exported` | `type MicroBatchOptions struct` | [node/sequencer/microbatch.go](../../../node/sequencer/microbatch.go) |
| `go_exported` | `const ModeMicroBatch Mode = 1` | [node/sequencer/microbatch.go](../../../node/sequencer/microbatch.go) |
| `go_exported` | `type AcceptFunc func(sender common.Address, nonce uint64, tx []byte) error` | [node/sequencer/nonce_queue.go](../../../node/sequencer/nonce_queue.go) |
| `go_exported` | `ErrNonceTooLow = core.ErrNonceTooLow` | [node/sequencer/nonce_queue.go](../../../node/sequencer/nonce_queue.go) |
| `go_exported` | `ErrQueueFull = errors.New("future-nonce queue full")` | [node/sequencer/nonce_queue.go](../../../node/sequencer/nonce_queue.go) |
| `go_exported` | `ErrSameNonce = errors.New("same nonce already receipted or held")` | [node/sequencer/nonce_queue.go](../../../node/sequencer/nonce_queue.go) |
| `go_exported` | `ErrSenderQueueFull = errors.New("sender future-nonce queue full")` | [node/sequencer/nonce_queue.go](../../../node/sequencer/nonce_queue.go) |
| `go_exported` | `type HeldTx struct` | [node/sequencer/nonce_queue.go](../../../node/sequencer/nonce_queue.go) |
| `go_exported` | `func NewNonceQueue(cfg NonceQueueConfig, nonces NextNonceReader, accept AcceptFunc) (*NonceQueue, error) {` | [node/sequencer/nonce_queue.go](../../../node/sequencer/nonce_queue.go) |
| `go_exported` | `type NextNonceReader interface` | [node/sequencer/nonce_queue.go](../../../node/sequencer/nonce_queue.go) |
| `go_exported` | `type NonceQueue struct` | [node/sequencer/nonce_queue.go](../../../node/sequencer/nonce_queue.go) |
| `go_exported` | `type NonceQueueConfig struct` | [node/sequencer/nonce_queue.go](../../../node/sequencer/nonce_queue.go) |
| `go_exported` | `func (q *NonceQueue) Drain(sender common.Address)` | [node/sequencer/nonce_queue.go](../../../node/sequencer/nonce_queue.go) |
| `go_exported` | `func (q *NonceQueue) Expire(now time.Time) []HeldTx` | [node/sequencer/nonce_queue.go](../../../node/sequencer/nonce_queue.go) |
| `go_exported` | `func (q *NonceQueue) Held(sender common.Address, nonce uint64) bool` | [node/sequencer/nonce_queue.go](../../../node/sequencer/nonce_queue.go) |
| `go_exported` | `func (q *NonceQueue) Hold(sender common.Address, nonce uint64, tx []byte) (bool, error)` | [node/sequencer/nonce_queue.go](../../../node/sequencer/nonce_queue.go) |
| `go_exported` | `func (q *NonceQueue) Len() int` | [node/sequencer/nonce_queue.go](../../../node/sequencer/nonce_queue.go) |
| `go_exported` | `func (q *NonceQueue) Sealed()` | [node/sequencer/nonce_queue.go](../../../node/sequencer/nonce_queue.go) |
| `go_exported` | `var ErrTxPosition = errors.New("sequencer: transaction position outside the sealed block")` | [node/sequencer/precompiles.go](../../../node/sequencer/precompiles.go) |
| `go_exported` | `type PrecompileHost interface` | [node/sequencer/precompiles.go](../../../node/sequencer/precompiles.go) |
| `go_exported` | `func BuildReceipt(in *ReceiptInput, contextHash [32]byte) ([]byte, error)` | [node/sequencer/receipt.go](../../../node/sequencer/receipt.go) |
| `go_exported` | `CosmosOperation uint8 = 0xFF` | [node/sequencer/receipt.go](../../../node/sequencer/receipt.go) |
| `go_exported` | `ErrReceiptInput = errors.New("sequencer: invalid receipt input")` | [node/sequencer/receipt.go](../../../node/sequencer/receipt.go) |
| `go_exported` | `ErrReceiptInvariant = errors.New("sequencer: accepted transaction has no receipt")` | [node/sequencer/receipt.go](../../../node/sequencer/receipt.go) |
| `go_exported` | `KernelReservedModules uint16 = 11` | [node/sequencer/receipt.go](../../../node/sequencer/receipt.go) |
| `go_exported` | `func LogEffect(log *evmtypes.Log) ([]byte, error)` | [node/sequencer/receipt.go](../../../node/sequencer/receipt.go) |
| `go_exported` | `type ReceiptInput struct` | [node/sequencer/receipt.go](../../../node/sequencer/receipt.go) |
| `go_exported` | `ReceiptLayoutVersion uint32 = 1` | [node/sequencer/receipt.go](../../../node/sequencer/receipt.go) |
| `go_exported` | `type Receipter struct` | [node/sequencer/receipt.go](../../../node/sequencer/receipt.go) |
| `go_exported` | `func SummaryEffect(in *ReceiptInput) ([]byte, error)` | [node/sequencer/receipt.go](../../../node/sequencer/receipt.go) |
| `go_exported` | `ErrReceiptKeyFile = errors.New("sequencer: receipt key file")` | [node/sequencer/receiptkey.go](../../../node/sequencer/receiptkey.go) |
| `go_exported` | `ErrReceiptKeyIsConsensusKey = errors.New("sequencer: receipt key is the consensus key")` | [node/sequencer/receiptkey.go](../../../node/sequencer/receiptkey.go) |
| `go_exported` | `ErrReceiptKeyNotPinned = errors.New("sequencer: receipt key is not pinned for the height")` | [node/sequencer/receiptkey.go](../../../node/sequencer/receiptkey.go) |
| `go_exported` | `ErrReceiptKeyPin = errors.New("sequencer: invalid receipt key pin")` | [node/sequencer/receiptkey.go](../../../node/sequencer/receiptkey.go) |
| `go_exported` | `type ReceiptKey struct` | [node/sequencer/receiptkey.go](../../../node/sequencer/receiptkey.go) |
| `go_exported` | `func (k *ReceiptKey) KeyID(height int64) ([32]byte, error)` | [node/sequencer/receiptkey.go](../../../node/sequencer/receiptkey.go) |
| `go_exported` | `func (k *ReceiptKey) PublicKey() [32]byte { return k.public }` | [node/sequencer/receiptkey.go](../../../node/sequencer/receiptkey.go) |
| `go_exported` | `func (k *ReceiptKey) SignReceipt(in *ReceiptInput) ([]byte, error)` | [node/sequencer/receiptkey.go](../../../node/sequencer/receiptkey.go) |
| `go_exported` | `func BatchFields(b *Batch) SequenceBatchFields` | [node/sequencer/seal.go](../../../node/sequencer/seal.go) |
| `go_exported` | `ErrMalformedBatch = errors.New("sequencer: sealed batch is malformed")` | [node/sequencer/seal.go](../../../node/sequencer/seal.go) |
| `go_exported` | `ErrSealPanicked = errors.New("sequencer: seal panicked")` | [node/sequencer/seal.go](../../../node/sequencer/seal.go) |
| `go_exported` | `func SealBatch(a *app.App, b *Batch, msg seqtypes.MsgSequenceBatch) (*SealedLayer, []byte, error) {` | [node/sequencer/seal.go](../../../node/sequencer/seal.go) |
| `go_exported` | `AckGap AckStatus = 2` | [node/sequencer/standby.go](../../../node/sequencer/standby.go) |
| `go_exported` | `AckOK AckStatus = 1` | [node/sequencer/standby.go](../../../node/sequencer/standby.go) |
| `go_exported` | `AckRefused AckStatus = 3` | [node/sequencer/standby.go](../../../node/sequencer/standby.go) |
| `go_exported` | `type AckStatus uint8` | [node/sequencer/standby.go](../../../node/sequencer/standby.go) |
| `go_exported` | `ErrGroupTooLarge = errors.New("sequencer: replication group too large")` | [node/sequencer/standby.go](../../../node/sequencer/standby.go) |
| `go_exported` | `ErrNoAck = errors.New("sequencer: standby did not acknowledge")` | [node/sequencer/standby.go](../../../node/sequencer/standby.go) |
| `go_exported` | `ErrSealContinuity = errors.New("sequencer: seal does not cover the replicated seqs")` | [node/sequencer/standby.go](../../../node/sequencer/standby.go) |
| `go_exported` | `ErrSharedKey = errors.New("sequencer: standby shares a key with the primary")` | [node/sequencer/standby.go](../../../node/sequencer/standby.go) |
| `go_exported` | `ErrStaleLease = errors.New("sequencer: lease epoch below the held epoch")` | [node/sequencer/standby.go](../../../node/sequencer/standby.go) |
| `go_exported` | `ErrStandbyRefused = errors.New("sequencer: standby refused the group")` | [node/sequencer/standby.go](../../../node/sequencer/standby.go) |
| `go_exported` | `type Replicator struct` | [node/sequencer/standby.go](../../../node/sequencer/standby.go) |
| `go_exported` | `func (r *Replicator) Close()` | [node/sequencer/standby.go](../../../node/sequencer/standby.go) |
| `go_exported` | `type ReplicatorConfig struct` | [node/sequencer/standby.go](../../../node/sequencer/standby.go) |
| `go_exported` | `type Standby struct` | [node/sequencer/standby.go](../../../node/sequencer/standby.go) |
| `go_exported` | `type StandbyConfig struct` | [node/sequencer/standby.go](../../../node/sequencer/standby.go) |
| `go_exported` | `func (s *Standby) ConsensusKey() tmtypes.PrivValidator { return s.consensusKey }` | [node/sequencer/standby.go](../../../node/sequencer/standby.go) |
| `go_exported` | `func (s *Standby) Err() error` | [node/sequencer/standby.go](../../../node/sequencer/standby.go) |
| `go_exported` | `func (s *Standby) LastSeq() uint64` | [node/sequencer/standby.go](../../../node/sequencer/standby.go) |
| `go_exported` | `func (s *Standby) ReceiptKey() ed25519.PrivateKey { return s.receiptKey }` | [node/sequencer/standby.go](../../../node/sequencer/standby.go) |
| `go_exported` | `func (s *Standby) Serve(l net.Listener) error` | [node/sequencer/standby.go](../../../node/sequencer/standby.go) |

## Sequencer config section

The `[sequencer]` configuration is parsed and validated at startup. `enable` selects the runtime; `role` is `primary` or `standby`; `mode` is `sequential` or `microbatch`. Configuration includes WAL directory and retention, receipt-key file and forwarding URL, standby address, replication timeout and TLS settings, microbatch window/count, nonce-queue bounds and lease duration. [config.go](../../../node/sequencer/config.go) declares the precise names and combinations.

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `FlagEnable = "sequencer.enable"` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `config_key` | `FlagMicroBatchMaxCount = "sequencer.microbatch-max-count"` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `config_key` | `FlagMicroBatchWindow = "sequencer.microbatch-window"` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `config_key` | `FlagMode = "sequencer.mode"` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `config_key` | `FlagReceiptKeyFile = "sequencer.receipt-key-file"` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `config_key` | `FlagReplicationTimeout = "sequencer.replication-timeout"` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `config_key` | `FlagStandbyAddress = "sequencer.standby-address"` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `config_key` | `FlagWALDir = "sequencer.wal-dir"` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `file_format` | `func LoadReceiptKey(keyFile, consensusKeyFile string, pins []verify.SequencerAuthorization, openHeight int64) (*ReceiptKey, error) {` | [node/sequencer/receiptkey.go](../../../node/sequencer/receiptkey.go) |
| `config_key` | `sequencer.lease-duration` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `config_key` | `sequencer.nonce-queue-max-per-sender` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `config_key` | `sequencer.nonce-queue-max-total` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `config_key` | `sequencer.receipt-forward-url` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `config_key` | `sequencer.role` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `config_key` | `sequencer.wal-retain-blocks` | [node/sequencer/config.go](../../../node/sequencer/config.go) |
| `go_exported` | `func AddStartFlags(cmd *cobra.Command)` | [node/sequencer/config.go](../../../node/sequencer/config.go) |

## modules/sequencer store, genesis and params

[GenesisState](../../../modules/sequencer/types/genesis.go) contains `Leaders`, `ReceiptKeys`, `ActivationHeight`, `LastSeq` and `LastSeqHash`. Validation refuses empty, zero or duplicate leader keys, invalid receipt keys/ranges, and inconsistent last-sequence/hash pairs. A nonzero activation height requires a covering receipt key; zero lets initialization select the next block.

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func (k Keeper) DeliverSequenceBatch(ctx sdk.Context, msg types.MsgSequenceBatch) error` | [modules/sequencer/keeper/keeper.go](../../../modules/sequencer/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) ExportGenesis(ctx sdk.Context) (types.GenesisState, error)` | [modules/sequencer/keeper/keeper.go](../../../modules/sequencer/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) InitGenesis(ctx sdk.Context, gs types.GenesisState) error` | [modules/sequencer/keeper/keeper.go](../../../modules/sequencer/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) Leaders(ctx sdk.Context) ([][32]byte, error)` | [modules/sequencer/keeper/keeper.go](../../../modules/sequencer/keeper/keeper.go) |
| `go_exported` | `func ParamKeyTable() paramtypes.KeyTable` | [modules/sequencer/keeper/receipt_keys.go](../../../modules/sequencer/keeper/receipt_keys.go) |
| `go_exported` | `func (e ReceiptKeyEntry) Validate() error` | [modules/sequencer/keeper/receipt_keys.go](../../../modules/sequencer/keeper/receipt_keys.go) |
| `go_exported` | `func (am AppModule) ExportGenesis(ctx sdk.Context, _ codec.JSONCodec) json.RawMessage` | [modules/sequencer/module.go](../../../modules/sequencer/module.go) |
| `go_exported` | `func (am AppModule) InitGenesis(ctx sdk.Context, _ codec.JSONCodec, gs json.RawMessage) []abci.ValidatorUpdate {` | [modules/sequencer/module.go](../../../modules/sequencer/module.go) |
| `go_exported` | `func NewParamChangeProposalHandler(k keeper.Keeper, paramSpace paramtypes.Subspace, next govtypes.Handler) govtypes.Handler {` | [modules/sequencer/module.go](../../../modules/sequencer/module.go) |
| `go_exported` | `type GenesisState struct` | [modules/sequencer/types/genesis.go](../../../modules/sequencer/types/genesis.go) |
| `go_exported` | `func (gs GenesisState) Validate() error` | [modules/sequencer/types/genesis.go](../../../modules/sequencer/types/genesis.go) |
| `go_exported` | `StoredVotesKey = []byte{0x05}` | [modules/sequencer/types/keys.go](../../../modules/sequencer/types/keys.go) |
| `storage_key` | `StoredVotesKey = []byte{0x05}` | [modules/sequencer/types/keys.go](../../../modules/sequencer/types/keys.go) |
| `go_exported` | `func (k Keeper) ActivationHeight(ctx sdk.Context) (int64, error)` | [modules/sequencer/keeper/activation.go](../../../modules/sequencer/keeper/activation.go) |
| `go_exported` | `func (k Keeper) Sequenced(ctx sdk.Context, height int64) (bool, error)` | [modules/sequencer/keeper/activation.go](../../../modules/sequencer/keeper/activation.go) |
| `go_exported` | `func (k Keeper) SetActivationHeight(ctx sdk.Context, height int64) error` | [modules/sequencer/keeper/activation.go](../../../modules/sequencer/keeper/activation.go) |
| `go_exported` | `func (k Keeper) SetLeaders(ctx sdk.Context, leaders [][32]byte) error` | [modules/sequencer/keeper/leaders.go](../../../modules/sequencer/keeper/leaders.go) |
| `go_exported` | `func GetLeadersParam(ctx sdk.Context, paramSpace paramtypes.Subspace) [][32]byte` | [modules/sequencer/keeper/leaders_param.go](../../../modules/sequencer/keeper/leaders_param.go) |
| `go_exported` | `var KeyLeaders = []byte("Leaders")` | [modules/sequencer/keeper/leaders_param.go](../../../modules/sequencer/keeper/leaders_param.go) |
| `go_exported` | `func SetLeadersParam(ctx sdk.Context, paramSpace paramtypes.Subspace, leaders [][32]byte) error {` | [modules/sequencer/keeper/leaders_param.go](../../../modules/sequencer/keeper/leaders_param.go) |
| `go_exported` | `func (k Keeper) ReceiptKeyPins(ctx sdk.Context) ([]verify.SequencerAuthorization, error)` | [modules/sequencer/keeper/pins.go](../../../modules/sequencer/keeper/pins.go) |
| `go_exported` | `func (k Keeper) StoreVotes(ctx sdk.Context, height int64, votes []abci.VoteInfo) error` | [modules/sequencer/keeper/votes.go](../../../modules/sequencer/keeper/votes.go) |
| `go_exported` | `func (k Keeper) StoredVotes(ctx sdk.Context, height int64) ([]abci.VoteInfo, error)` | [modules/sequencer/keeper/votes.go](../../../modules/sequencer/keeper/votes.go) |
| `go_exported` | `ActivationHeightKey = []byte{0x07}` | [modules/sequencer/types/keys.go](../../../modules/sequencer/types/keys.go) |
| `go_exported` | `func StoredVotesKeyAt(height int64) []byte` | [modules/sequencer/types/keys.go](../../../modules/sequencer/types/keys.go) |
| `storage_key` | `ActivationHeightKey` | [modules/sequencer/types/keys.go](../../../modules/sequencer/types/keys.go) |
| `go_exported` | `func ValidateLeaders(leaders [][32]byte) error` | [modules/sequencer/types/leaders.go](../../../modules/sequencer/types/leaders.go) |
| `go_exported` | `func DecodeStoredVotes(bz []byte) ([]abci.VoteInfo, error)` | [modules/sequencer/types/votes.go](../../../modules/sequencer/types/votes.go) |
| `go_exported` | `func EncodeStoredVotes(votes []abci.VoteInfo) ([]byte, error)` | [modules/sequencer/types/votes.go](../../../modules/sequencer/types/votes.go) |
| `go_exported` | `func NewNode(cfg Config, a *app.App) (_ *Node, err error) {` | [node/sequencer/node.go](../../../node/sequencer/node.go) |
| `go_exported` | `func (n *Node) Start(ctx context.Context) (err error) {` | [node/sequencer/node_start.go](../../../node/sequencer/node_start.go) |
| `go_exported` | `func (n *Node) Stop() error` | [node/sequencer/node_stop.go](../../../node/sequencer/node_stop.go) |
| `go_exported` | `func StartStandby(ctx context.Context, cfg Config, a *app.App) (*Standby, error)` | [node/sequencer/standby_role.go](../../../node/sequencer/standby_role.go) |
| `go_exported` | `ErrBatchPosition = sdkerrors.Register(types.ModuleName, 5, "sequence batch is not tx 0")` | [modules/sequencer/keeper/keeper.go](../../../modules/sequencer/keeper/keeper.go) |
| `go_exported` | `type Keeper struct` | [modules/sequencer/keeper/keeper.go](../../../modules/sequencer/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) LastSeq(ctx sdk.Context) (uint64, error)` | [modules/sequencer/keeper/keeper.go](../../../modules/sequencer/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) LastSeqHash(ctx sdk.Context) ([types.HashLength]byte, error)` | [modules/sequencer/keeper/keeper.go](../../../modules/sequencer/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) ReceiptKeys(ctx sdk.Context) ([]types.ReceiptKey, error)` | [modules/sequencer/keeper/keeper.go](../../../modules/sequencer/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) SetReceiptKeys(ctx sdk.Context, keys ReceiptKeys) error` | [modules/sequencer/keeper/keeper.go](../../../modules/sequencer/keeper/keeper.go) |
| `go_exported` | `func NewKeeper(storeKey sdk.StoreKey) Keeper { return Keeper{storeKey: storeKey} }` | [modules/sequencer/keeper/keeper.go](../../../modules/sequencer/keeper/keeper.go) |
| `go_exported` | `func RefuseCheckTx(ctx sdk.Context) error` | [modules/sequencer/keeper/keeper.go](../../../modules/sequencer/keeper/keeper.go) |
| `storage_key` | `var lastBatchHeightKey = []byte{0x06}` | [modules/sequencer/keeper/keeper.go](../../../modules/sequencer/keeper/keeper.go) |
| `go_exported` | `ErrInvalidReceiptKey = errors.New("sequencer: invalid receipt key")` | [modules/sequencer/keeper/receipt_keys.go](../../../modules/sequencer/keeper/receipt_keys.go) |
| `go_exported` | `ErrReceiptBatchID = errors.New("sequencer: receipt batch_id is not a block height")` | [modules/sequencer/keeper/receipt_keys.go](../../../modules/sequencer/keeper/receipt_keys.go) |
| `go_exported` | `ErrReceiptKeyNotPinned = errors.New("sequencer: receipt key not pinned at height")` | [modules/sequencer/keeper/receipt_keys.go](../../../modules/sequencer/keeper/receipt_keys.go) |
| `go_exported` | `ErrReceiptKeyRotation = errors.New("sequencer: receipt key change rewrites a past height")` | [modules/sequencer/keeper/receipt_keys.go](../../../modules/sequencer/keeper/receipt_keys.go) |
| `go_exported` | `func GetReceiptKeys(ctx sdk.Context, paramSpace paramtypes.Subspace) ReceiptKeys` | [modules/sequencer/keeper/receipt_keys.go](../../../modules/sequencer/keeper/receipt_keys.go) |
| `go_exported` | `func InitReceiptKeys(ctx sdk.Context, paramSpace paramtypes.Subspace, keys ReceiptKeys) error {` | [modules/sequencer/keeper/receipt_keys.go](../../../modules/sequencer/keeper/receipt_keys.go) |
| `go_exported` | `var KeyReceiptKeys = []byte("ReceiptKeys")` | [modules/sequencer/keeper/receipt_keys.go](../../../modules/sequencer/keeper/receipt_keys.go) |
| `go_exported` | `func ReceiptHeight(receipt *codec.Receipt) (uint64, error)` | [modules/sequencer/keeper/receipt_keys.go](../../../modules/sequencer/keeper/receipt_keys.go) |
| `go_exported` | `type ReceiptKeyEntry struct` | [modules/sequencer/keeper/receipt_keys.go](../../../modules/sequencer/keeper/receipt_keys.go) |
| `go_exported` | `type ReceiptKeys []ReceiptKeyEntry` | [modules/sequencer/keeper/receipt_keys.go](../../../modules/sequencer/keeper/receipt_keys.go) |
| `go_exported` | `func (keys ReceiptKeys) KeyForHeight(keyID [32]byte, height uint64) (ReceiptKeyEntry, error) {` | [modules/sequencer/keeper/receipt_keys.go](../../../modules/sequencer/keeper/receipt_keys.go) |
| `go_exported` | `func (keys ReceiptKeys) Validate() error` | [modules/sequencer/keeper/receipt_keys.go](../../../modules/sequencer/keeper/receipt_keys.go) |
| `go_exported` | `func (keys ReceiptKeys) VerifyReceipt(receiptBytes []byte) (*verify.VerifiedReceipt, error) {` | [modules/sequencer/keeper/receipt_keys.go](../../../modules/sequencer/keeper/receipt_keys.go) |
| `go_exported` | `func ValidateRotation(current, next ReceiptKeys, height int64) error` | [modules/sequencer/keeper/receipt_keys.go](../../../modules/sequencer/keeper/receipt_keys.go) |
| `storage_key` | `var KeyReceiptKeys = []byte("ReceiptKeys")` | [modules/sequencer/keeper/receipt_keys.go](../../../modules/sequencer/keeper/receipt_keys.go) |
| `go_exported` | `type AppModule struct` | [modules/sequencer/module.go](../../../modules/sequencer/module.go) |
| `go_exported` | `type AppModuleBasic struct{}` | [modules/sequencer/module.go](../../../modules/sequencer/module.go) |
| `go_exported` | `func (am AppModuleBasic) ValidateGenesisStream(cdc codec.JSONCodec, config client.TxEncodingConfig, genesisCh <-chan json.RawMessage) error {` | [modules/sequencer/module.go](../../../modules/sequencer/module.go) |
| `go_exported` | `func (am AppModule) ExportGenesisStream(ctx sdk.Context, cdc codec.JSONCodec) <-chan json.RawMessage {` | [modules/sequencer/module.go](../../../modules/sequencer/module.go) |
| `go_exported` | `func NewAppModule(k keeper.Keeper, paramSpace paramtypes.Subspace) AppModule` | [modules/sequencer/module.go](../../../modules/sequencer/module.go) |
| `go_exported` | `var ErrInvalidGenesis = sdkerrors.Register(ModuleName, 4, "invalid sequencer genesis")` | [modules/sequencer/types/genesis.go](../../../modules/sequencer/types/genesis.go) |
| `go_exported` | `type ReceiptKey struct` | [modules/sequencer/types/genesis.go](../../../modules/sequencer/types/genesis.go) |
| `go_exported` | `func (k ReceiptKey) Covers(height uint64) bool` | [modules/sequencer/types/genesis.go](../../../modules/sequencer/types/genesis.go) |
| `go_exported` | `LastSeqHashKey = []byte{0x02}` | [modules/sequencer/types/keys.go](../../../modules/sequencer/types/keys.go) |
| `go_exported` | `LastSeqKey = []byte{0x01}` | [modules/sequencer/types/keys.go](../../../modules/sequencer/types/keys.go) |
| `go_exported` | `LeaderListKey = []byte{0x04}` | [modules/sequencer/types/keys.go](../../../modules/sequencer/types/keys.go) |
| `go_exported` | `ModuleName = "sequencer"` | [modules/sequencer/types/keys.go](../../../modules/sequencer/types/keys.go) |
| `go_exported` | `ReceiptKeysKey = []byte{0x03}` | [modules/sequencer/types/keys.go](../../../modules/sequencer/types/keys.go) |
| `go_exported` | `StoreKey = ModuleName` | [modules/sequencer/types/keys.go](../../../modules/sequencer/types/keys.go) |
| `storage_key` | `LastSeqHashKey = []byte{0x02}` | [modules/sequencer/types/keys.go](../../../modules/sequencer/types/keys.go) |
| `storage_key` | `LastSeqKey = []byte{0x01}` | [modules/sequencer/types/keys.go](../../../modules/sequencer/types/keys.go) |
| `storage_key` | `LeaderListKey = []byte{0x04}` | [modules/sequencer/types/keys.go](../../../modules/sequencer/types/keys.go) |
| `storage_key` | `ModuleName = "sequencer"` | [modules/sequencer/types/keys.go](../../../modules/sequencer/types/keys.go) |
| `storage_key` | `ReceiptKeysKey = []byte{0x03}` | [modules/sequencer/types/keys.go](../../../modules/sequencer/types/keys.go) |

## Consensus leader pin

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func (blockExec *BlockExecutor) CreateProposalBlock(` | [consensus/internal/state/execution.go](../../../consensus/internal/state/execution.go) |
| `go_exported` | `func New(` | [consensus/node/public.go](../../../consensus/node/public.go) |
| `go_exported` | `func (t Testnet) Validate() error` | [consensus/test/e2e/pkg/testnet.go](../../../consensus/test/e2e/pkg/testnet.go) |
| `go_exported` | `func (cs *State) SetLeaderSource(src types.LeaderSource)` | [consensus/internal/consensus/state.go](../../../consensus/internal/consensus/state.go) |
| `go_exported` | `func (p LeaderPin) Applies(height int64) bool` | [consensus/internal/consensus/types/leader_schedule.go](../../../consensus/internal/consensus/types/leader_schedule.go) |
| `go_exported` | `func LeaderPinFromSchedule(activationHeight int64, leaders [][32]byte) (LeaderPin, error)` | [consensus/internal/consensus/types/leader_schedule.go](../../../consensus/internal/consensus/types/leader_schedule.go) |
| `go_exported` | `func (app *Application) LeaderSchedule(_ int64) (int64, [][32]byte, error) {` | [consensus/test/e2e/app/leaders.go](../../../consensus/test/e2e/app/leaders.go) |
| `go_exported` | `type LeaderSource interface {` | [consensus/types/sequencer.go](../../../consensus/types/sequencer.go) |
| `go_exported` | `func (k Keeper) LeaderSchedule(ctx sdk.Context) (int64, [][32]byte, error)` | [modules/sequencer/keeper/leaders.go](../../../modules/sequencer/keeper/leaders.go) |
| source | — | [consensus/test/e2e/app/app.go](../../../consensus/test/e2e/app/app.go) |
| source | — | [consensus/test/e2e/pkg/manifest.go](../../../consensus/test/e2e/pkg/manifest.go) |

## Error codes

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/sequencer/keeper/keeper.go](../../../modules/sequencer/keeper/keeper.go) |
| source | — | [modules/sequencer/types/genesis.go](../../../modules/sequencer/types/genesis.go) |
| source | — | [modules/sequencer/types/msgs.go](../../../modules/sequencer/types/msgs.go) |

<a id="integration-into-the-node"></a>

## EVM receipt store

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func BackendTypeName(store ReceiptStore) string` | [storage/ledger_db/receipt/receipt_store.go](../../../storage/ledger_db/receipt/receipt_store.go) |
| `go_exported` | `ErrNotConfigured = errors.New("receipt store not configured")` | [storage/ledger_db/receipt/receipt_store.go](../../../storage/ledger_db/receipt/receipt_store.go) |
| `go_exported` | `ErrNotFound = errors.New("receipt not found")` | [storage/ledger_db/receipt/receipt_store.go](../../../storage/ledger_db/receipt/receipt_store.go) |
| `go_exported` | `ErrRangeQueryNotSupported = errors.New("range query not supported by this backend")` | [storage/ledger_db/receipt/receipt_store.go](../../../storage/ledger_db/receipt/receipt_store.go) |
| `go_exported` | `func NewReceiptStore(config dbconfig.ReceiptStoreConfig, storeKey sdk.StoreKey) (ReceiptStore, error) {` | [storage/ledger_db/receipt/receipt_store.go](../../../storage/ledger_db/receipt/receipt_store.go) |
| `go_exported` | `func NewReceiptStoreWithReadMetrics(` | [storage/ledger_db/receipt/receipt_store.go](../../../storage/ledger_db/receipt/receipt_store.go) |
| `go_exported` | `type ReceiptReadMetrics interface` | [storage/ledger_db/receipt/receipt_store.go](../../../storage/ledger_db/receipt/receipt_store.go) |
| `go_exported` | `type ReceiptRecord struct` | [storage/ledger_db/receipt/receipt_store.go](../../../storage/ledger_db/receipt/receipt_store.go) |
| `go_exported` | `type ReceiptStore interface` | [storage/ledger_db/receipt/receipt_store.go](../../../storage/ledger_db/receipt/receipt_store.go) |
| `go_exported` | `func NewPebbleTxHashIndex(dir string) (*PebbleTxHashIndex, error)` | [storage/ledger_db/receipt/tx_hash_index.go](../../../storage/ledger_db/receipt/tx_hash_index.go) |
| `go_exported` | `type PebbleTxHashIndex struct` | [storage/ledger_db/receipt/tx_hash_index.go](../../../storage/ledger_db/receipt/tx_hash_index.go) |
| `go_exported` | `func (idx *PebbleTxHashIndex) Close() error` | [storage/ledger_db/receipt/tx_hash_index.go](../../../storage/ledger_db/receipt/tx_hash_index.go) |
| `go_exported` | `func (idx *PebbleTxHashIndex) GetBlockNumber(_ context.Context, txHash common.Hash) (uint64, bool, error) {` | [storage/ledger_db/receipt/tx_hash_index.go](../../../storage/ledger_db/receipt/tx_hash_index.go) |
| `go_exported` | `func (idx *PebbleTxHashIndex) IndexBlock(_ context.Context, blockNumber uint64, txHashes []common.Hash) (err error) {` | [storage/ledger_db/receipt/tx_hash_index.go](../../../storage/ledger_db/receipt/tx_hash_index.go) |
| `go_exported` | `func (idx *PebbleTxHashIndex) PruneBefore(_ context.Context, blockNumber uint64) (err error) {` | [storage/ledger_db/receipt/tx_hash_index.go](../../../storage/ledger_db/receipt/tx_hash_index.go) |
| `go_exported` | `type TxHashIndex interface` | [storage/ledger_db/receipt/tx_hash_index.go](../../../storage/ledger_db/receipt/tx_hash_index.go) |
| `go_exported` | `func TxHashIndexDir(receiptDBDir string) string` | [storage/ledger_db/receipt/tx_hash_index.go](../../../storage/ledger_db/receipt/tx_hash_index.go) |
| `storage_key` | `blockPrefix = 'b'` | [storage/ledger_db/receipt/tx_hash_index.go](../../../storage/ledger_db/receipt/tx_hash_index.go) |
| `storage_key` | `txHashPrefix = 'h'` | [storage/ledger_db/receipt/tx_hash_index.go](../../../storage/ledger_db/receipt/tx_hash_index.go) |

## Node ABCI surface

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func (app *App) BeginBlock(` | [node/abci.go](../../../node/abci.go) |
| `go_exported` | `func (app *App) CheckTx(ctx context.Context, req *abci.RequestCheckTxV2) *abci.ResponseCheckTxV2 {` | [node/abci.go](../../../node/abci.go) |
| `go_exported` | `func (app *App) Commit(ctx context.Context) (res *abci.ResponseCommit, err error)` | [node/abci.go](../../../node/abci.go) |
| `go_exported` | `func (app *App) FinalizeBlocker(ctx sdk.Context, req *abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) ProcessBlock(ctx sdk.Context, txs [][]byte, req *BlockProcessRequest, lastCommit abci.CommitInfo, simulate bool, preDecoded []sdk.Tx) (events []abci.Event, txResults []*abci.ExecTxResult, endBlockResp abci.ResponseEndBlock, err error) {` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) ProcessProposalHandler(ctx sdk.Context, req *abci.RequestProcessProposal) (resp *abci.ResponseProcessProposal, err error) {` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func BeginBlock(` | [node/legacyabci/begin_block.go](../../../node/legacyabci/begin_block.go) |
| `go_exported` | `func EndBlock(ctx sdk.Context, height int64, blockGasUsed int64, keepers EndBlockKeepers) []abci.ValidatorUpdate {` | [node/legacyabci/end_block.go](../../../node/legacyabci/end_block.go) |
| `go_exported` | `type EndBlockKeepers struct` | [node/legacyabci/end_block.go](../../../node/legacyabci/end_block.go) |
| `go_exported` | `func (app *App) LeaderSchedule(_ int64) (int64, [][32]byte, error) {` | [node/leader_source.go](../../../node/leader_source.go) |
| `go_exported` | `var ErrSequencedBlockHalt = errors.New("sequenced block halt")` | [node/sequenced_halt.go](../../../node/sequenced_halt.go) |
| `go_exported` | `CodeTypeBadNonce uint32 = 2` | [consensus/abci/example/code/code.go](../../../consensus/abci/example/code/code.go) |
| `go_exported` | `CodeTypeEncodingError uint32 = 1` | [consensus/abci/example/code/code.go](../../../consensus/abci/example/code/code.go) |
| `go_exported` | `CodeTypeOK uint32 = 0` | [consensus/abci/example/code/code.go](../../../consensus/abci/example/code/code.go) |
| `go_exported` | `CodeTypeUnauthorized uint32 = 3` | [consensus/abci/example/code/code.go](../../../consensus/abci/example/code/code.go) |
| `go_exported` | `type Application struct` | [consensus/abci/example/kvstore/kvstore.go](../../../consensus/abci/example/kvstore/kvstore.go) |
| `go_exported` | `func MakeValSetChangeTx(pubkey cryptoproto.PublicKey, power int64) []byte` | [consensus/abci/example/kvstore/kvstore.go](../../../consensus/abci/example/kvstore/kvstore.go) |
| `go_exported` | `func NewApplication() *Application` | [consensus/abci/example/kvstore/kvstore.go](../../../consensus/abci/example/kvstore/kvstore.go) |
| `go_exported` | `func NewProxy() *proxy.Proxy` | [consensus/abci/example/kvstore/kvstore.go](../../../consensus/abci/example/kvstore/kvstore.go) |
| `go_exported` | `ProtocolVersion uint64 = 0x1` | [consensus/abci/example/kvstore/kvstore.go](../../../consensus/abci/example/kvstore/kvstore.go) |
| `go_exported` | `type Application interface` | [consensus/abci/types/application.go](../../../consensus/abci/types/application.go) |
| `go_exported` | `type BaseApplication struct{}` | [consensus/abci/types/application.go](../../../consensus/abci/types/application.go) |
| `go_exported` | `func NewApplication(t interface` | [consensus/abci/types/mocks/application.go](../../../consensus/abci/types/mocks/application.go) |
| `go_exported` | `type CommitInfo struct` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `type Evidence struct` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `type LastCommitInfo struct` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `type Misbehavior struct` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `type RequestApplySnapshotChunk struct` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `type RequestBeginBlock struct` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `type RequestEndBlock struct` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `type RequestFinalizeBlock struct` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `type RequestInfo struct` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `type RequestInitChain struct` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `type RequestListSnapshots struct{}` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `type RequestLoadSnapshotChunk struct` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `type RequestOfferSnapshot struct` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `type RequestProcessProposal struct` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `type RequestQuery struct` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `type ResponseApplySnapshotChunk struct` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `ResponseApplySnapshotChunk_ABORT` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `ResponseApplySnapshotChunk_ACCEPT` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `ResponseApplySnapshotChunk_REJECT_SNAPSHOT` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `type ResponseApplySnapshotChunk_Result int32` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `ResponseApplySnapshotChunk_RETRY` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `ResponseApplySnapshotChunk_RETRY_SNAPSHOT` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `ResponseApplySnapshotChunk_UNKNOWN ResponseApplySnapshotChunk_Result = iota` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `type ResponseGetTxPriorityHint struct` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `type ResponseInitChain struct` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `type ResponseListSnapshots struct` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `type ResponseLoadSnapshotChunk struct` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `type ResponseOfferSnapshot struct` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `ResponseOfferSnapshot_ABORT` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `ResponseOfferSnapshot_ACCEPT` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `ResponseOfferSnapshot_REJECT` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `ResponseOfferSnapshot_REJECT_FORMAT` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `ResponseOfferSnapshot_REJECT_SENDER` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `type ResponseOfferSnapshot_Result int32` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `ResponseOfferSnapshot_UNKNOWN ResponseOfferSnapshot_Result = iota` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `type ResponseProcessProposal struct` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `ResponseProcessProposal_ACCEPT` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `type ResponseProcessProposal_ProposalStatus int32` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `ResponseProcessProposal_REJECT` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `type Snapshot struct` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `type Validator struct` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `type VoteInfo struct` | [consensus/abci/types/plain_types.go](../../../consensus/abci/types/plain_types.go) |
| `go_exported` | `CheckTxTypeV2New CheckTxTypeV2 = iota` | [consensus/abci/types/types.go](../../../consensus/abci/types/types.go) |
| `go_exported` | `CheckTxTypeV2Recheck` | [consensus/abci/types/types.go](../../../consensus/abci/types/types.go) |
| `go_exported` | `CodeTypeOK uint32 = 0` | [consensus/abci/types/types.go](../../../consensus/abci/types/types.go) |
| `go_exported` | `func MarshalTxResults(r []*ExecTxResult) ([][]byte, error)` | [consensus/abci/types/types.go](../../../consensus/abci/types/types.go) |
| `go_exported` | `type RequestCheckTxV2 struct` | [consensus/abci/types/types.go](../../../consensus/abci/types/types.go) |
| `go_exported` | `type RequestDeliverTxV2 struct` | [consensus/abci/types/types.go](../../../consensus/abci/types/types.go) |
| `go_exported` | `type RequestGetTxPriorityHintV2 struct` | [consensus/abci/types/types.go](../../../consensus/abci/types/types.go) |
| `go_exported` | `type ResponseCheckTxV2 struct` | [consensus/abci/types/types.go](../../../consensus/abci/types/types.go) |
| `go_exported` | `type TxResultV2 struct` | [consensus/abci/types/types.go](../../../consensus/abci/types/types.go) |
| `go_exported` | `type BlockParams struct` | [consensus/abci/types/types.pb.go](../../../consensus/abci/types/types.pb.go) |
| `go_exported` | `type ConsensusParams struct` | [consensus/abci/types/types.pb.go](../../../consensus/abci/types/types.pb.go) |
| `go_exported` | `type Event struct` | [consensus/abci/types/types.pb.go](../../../consensus/abci/types/types.pb.go) |
| `go_exported` | `type EventAttribute struct` | [consensus/abci/types/types.pb.go](../../../consensus/abci/types/types.pb.go) |
| `go_exported` | `type EvmTxInfo struct` | [consensus/abci/types/types.pb.go](../../../consensus/abci/types/types.pb.go) |
| `go_exported` | `type ExecTxResult struct` | [consensus/abci/types/types.pb.go](../../../consensus/abci/types/types.pb.go) |
| `go_exported` | `MisbehaviorType_DUPLICATE_VOTE MisbehaviorType = 1` | [consensus/abci/types/types.pb.go](../../../consensus/abci/types/types.pb.go) |
| `go_exported` | `MisbehaviorType_LIGHT_CLIENT_ATTACK MisbehaviorType = 2` | [consensus/abci/types/types.pb.go](../../../consensus/abci/types/types.pb.go) |
| `go_exported` | `type ResponseBeginBlock struct` | [consensus/abci/types/types.pb.go](../../../consensus/abci/types/types.pb.go) |
| `go_exported` | `type ResponseCheckTx struct` | [consensus/abci/types/types.pb.go](../../../consensus/abci/types/types.pb.go) |
| `go_exported` | `type ResponseCommit struct` | [consensus/abci/types/types.pb.go](../../../consensus/abci/types/types.pb.go) |
| `go_exported` | `type ResponseDeliverTx struct` | [consensus/abci/types/types.pb.go](../../../consensus/abci/types/types.pb.go) |
| `go_exported` | `type ResponseEndBlock struct` | [consensus/abci/types/types.pb.go](../../../consensus/abci/types/types.pb.go) |
| `go_exported` | `type ResponseFinalizeBlock struct` | [consensus/abci/types/types.pb.go](../../../consensus/abci/types/types.pb.go) |
| `go_exported` | `type ResponseInfo struct` | [consensus/abci/types/types.pb.go](../../../consensus/abci/types/types.pb.go) |
| `go_exported` | `type ResponseQuery struct` | [consensus/abci/types/types.pb.go](../../../consensus/abci/types/types.pb.go) |
| `go_exported` | `type TxResult struct` | [consensus/abci/types/types.pb.go](../../../consensus/abci/types/types.pb.go) |
| `go_exported` | `type ValidatorUpdate struct` | [consensus/abci/types/types.pb.go](../../../consensus/abci/types/types.pb.go) |
| `go_exported` | `type ValidatorUpdates []ValidatorUpdate` | [consensus/abci/types/util.go](../../../consensus/abci/types/util.go) |
| `go_exported` | `func (app *App) DeliverTx(ctx sdk.Context, req abci.RequestDeliverTxV2, tx sdk.Tx, checksum [32]byte) abci.ResponseDeliverTx {` | [node/abci.go](../../../node/abci.go) |
| `go_exported` | `func (app *App) DeliverTxBatch(ctx sdk.Context, req sdk.DeliverTxBatchRequest) (res sdk.DeliverTxBatchResponse) {` | [node/abci.go](../../../node/abci.go) |
| `go_exported` | `func (app *App) EndBlock(ctx sdk.Context, height int64, blockGasUsed int64) (res abci.ResponseEndBlock) {` | [node/abci.go](../../../node/abci.go) |
| `go_exported` | `func (app *App) EvmBalance(evmAddr common.Address, paxAddrBz []byte) uint256.Int` | [node/abci.go](../../../node/abci.go) |
| `go_exported` | `func (app *App) EvmNonce(evmAddr common.Address) uint64` | [node/abci.go](../../../node/abci.go) |
| `go_exported` | `func (app *App) MidBlock(ctx sdk.Context, height int64) []abci.Event` | [node/abci.go](../../../node/abci.go) |
| `go_exported` | `func (app *App) InitChainer(ctx sdk.Context, req abci.RequestInitChain) abci.ResponseInitChain {` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) MidBlocker(ctx sdk.Context, height int64) []abci.Event` | [node/app.go](../../../node/app.go) |
| `go_exported` | `type BeginBlockKeepers struct` | [node/legacyabci/begin_block.go](../../../node/legacyabci/begin_block.go) |
| `go_exported` | `func CheckTx(` | [node/legacyabci/check_tx.go](../../../node/legacyabci/check_tx.go) |
| `go_exported` | `type CheckTxKeepers struct` | [node/legacyabci/check_tx.go](../../../node/legacyabci/check_tx.go) |
| `go_exported` | `func DeliverTx(` | [node/legacyabci/deliver_tx.go](../../../node/legacyabci/deliver_tx.go) |
| `go_exported` | `type DeliverTxKeepers struct` | [node/legacyabci/deliver_tx.go](../../../node/legacyabci/deliver_tx.go) |
| `go_exported` | `type RecoveryHandler func(recoveryObj interface{}) error` | [node/legacyabci/recovery.go](../../../node/legacyabci/recovery.go) |

## Ante handlers

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `type HandlerOptions struct` | [node/ante.go](../../../node/ante.go) |
| `go_exported` | `func NewAnteHandler(options HandlerOptions) (sdk.AnteHandler, sdk.AnteHandler, error)` | [node/ante.go](../../../node/ante.go) |
| `go_exported` | `func CheckAndChargeFees(ctx sdk.Context, tx sdk.Tx, accountKeeper authkeeper.AccountKeeper, bankKeeper bankkeeper.Keeper, feegrantKeeper *feegrantkeeper.Keeper, paramsKeeper paramskeeper.Keeper, isGasless bool) (priority int64, err error) {` | [node/ante/cosmos_checktx.go](../../../node/ante/cosmos_checktx.go) |
| `go_exported` | `func CheckAuthzContainsEvm(authzMsg *authz.MsgExec, nestedLvl int) (bool, error)` | [node/ante/cosmos_checktx.go](../../../node/ante/cosmos_checktx.go) |
| `go_exported` | `func CheckMemoLength(tx sdk.Tx, authParams authtypes.Params) error` | [node/ante/cosmos_checktx.go](../../../node/ante/cosmos_checktx.go) |
| `go_exported` | `func CheckMessage(ctx sdk.Context, tx sdk.Tx, ibcKeeper *ibckeeper.Keeper, oracleKeeper oraclekeeper.Keeper, pricefeedKeeper pricefeedkeeper.Keeper) error {` | [node/ante/cosmos_checktx.go](../../../node/ante/cosmos_checktx.go) |
| `go_exported` | `func CheckPubKeys(ctx sdk.Context, tx sdk.Tx, accountKeeper authkeeper.AccountKeeper, authParams authtypes.Params) ([]authtypes.AccountI, error) {` | [node/ante/cosmos_checktx.go](../../../node/ante/cosmos_checktx.go) |
| `go_exported` | `func CheckSignatures(ctx sdk.Context, txConfig client.TxConfig, tx sdk.Tx, signerAccounts []authtypes.AccountI, authParams authtypes.Params) (sdk.Events, error) {` | [node/ante/cosmos_checktx.go](../../../node/ante/cosmos_checktx.go) |
| `go_exported` | `func CosmosCheckTxAnte(` | [node/ante/cosmos_checktx.go](../../../node/ante/cosmos_checktx.go) |
| `go_exported` | `func CosmosStatelessChecks(tx sdk.Tx, height int64, consensusParams *tmproto.ConsensusParams) (` | [node/ante/cosmos_checktx.go](../../../node/ante/cosmos_checktx.go) |
| `go_exported` | `func DecoratePriority(ctx sdk.Context, priority int64, oracleVote bool) sdk.Context` | [node/ante/cosmos_checktx.go](../../../node/ante/cosmos_checktx.go) |
| `go_exported` | `type GasTx interface` | [node/ante/cosmos_checktx.go](../../../node/ante/cosmos_checktx.go) |
| `go_exported` | `func HandleOutofGas(recoveredErr any, gasLimit uint64, gasConsumed uint64) error` | [node/ante/cosmos_checktx.go](../../../node/ante/cosmos_checktx.go) |
| `go_exported` | `type HasExtensionOptionsTx interface` | [node/ante/cosmos_checktx.go](../../../node/ante/cosmos_checktx.go) |
| `go_exported` | `func SetGasMeter(ctx sdk.Context, gasLimit uint64, paramsKeeper paramskeeper.Keeper) sdk.Context {` | [node/ante/cosmos_checktx.go](../../../node/ante/cosmos_checktx.go) |
| `go_exported` | `type TxWithTimeoutHeight interface` | [node/ante/cosmos_checktx.go](../../../node/ante/cosmos_checktx.go) |
| `go_exported` | `func UpdateSigners(ctx sdk.Context, tx sdk.Tx, accountKeeper authkeeper.AccountKeeper, evmKeeper *evmkeeper.Keeper) (sdk.Events, error) {` | [node/ante/cosmos_checktx.go](../../../node/ante/cosmos_checktx.go) |
| `go_exported` | `func ChargeFees(ctx sdk.Context, tx sdk.Tx, accountKeeper authkeeper.AccountKeeper, bankKeeper bankkeeper.Keeper, feegrantKeeper *feegrantkeeper.Keeper, paramsKeeper paramskeeper.Keeper) error {` | [node/ante/cosmos_delivertx.go](../../../node/ante/cosmos_delivertx.go) |
| `go_exported` | `func CosmosDeliverTxAnte(` | [node/ante/cosmos_delivertx.go](../../../node/ante/cosmos_delivertx.go) |
| `go_exported` | `func AssociateAddress(ctx sdk.Context, ek *evmkeeper.Keeper, evmAddr common.Address, paxAddr sdk.AccAddress, paxPubkey cryptotypes.PubKey) error {` | [node/ante/evm_checktx.go](../../../node/ante/evm_checktx.go) |
| `go_exported` | `func CalculatePriority(ctx sdk.Context, txData ethtx.EthereumTxData, evmKeeper *evmkeeper.Keeper) *big.Int {` | [node/ante/evm_checktx.go](../../../node/ante/evm_checktx.go) |
| `go_exported` | `func CheckAndDecodeSignature(ctx sdk.Context, txData ethtx.EthereumTxData, chainID *big.Int, isBlockTest bool) (common.Address, sdk.AccAddress, cryptotypes.PubKey, derived.SignerVersion, error) {` | [node/ante/evm_checktx.go](../../../node/ante/evm_checktx.go) |
| `go_exported` | `func CheckNonce(ctx sdk.Context, ek *evmkeeper.Keeper, etx *ethtypes.Transaction, evmAddr common.Address) (sdk.Context, error) {` | [node/ante/evm_checktx.go](../../../node/ante/evm_checktx.go) |
| `go_exported` | `func DecorateContext(ctx sdk.Context, ek *evmkeeper.Keeper, tx sdk.Tx, txData ethtx.EthereumTxData, etx *ethtypes.Transaction, sender common.Address, paxSender sdk.AccAddress) sdk.Context {` | [node/ante/evm_checktx.go](../../../node/ante/evm_checktx.go) |
| `go_exported` | `func EvmCheckAndChargeFees(ctx sdk.Context, sender common.Address, ek *evmkeeper.Keeper, upgradeKeeper *upgradekeeper.Keeper, txData ethtx.EthereumTxData, etx *ethtypes.Transaction, msg *evmtypes.MsgEVMTransaction, version derived.SignerVersion, statelessChecks bool) (*state.DBImpl, error) {` | [node/ante/evm_checktx.go](../../../node/ante/evm_checktx.go) |
| `go_exported` | `func EvmCheckTxAnte(` | [node/ante/evm_checktx.go](../../../node/ante/evm_checktx.go) |
| `go_exported` | `func EvmStatelessChecks(ctx sdk.Context, tx sdk.Tx, chainID *big.Int, chains ...*chainskeeper.Keeper) error {` | [node/ante/evm_checktx.go](../../../node/ante/evm_checktx.go) |
| `go_exported` | `func GetBaseFee(ctx sdk.Context, evmKeeper *evmkeeper.Keeper, upgradeKeeper *upgradekeeper.Keeper) *big.Int {` | [node/ante/evm_checktx.go](../../../node/ante/evm_checktx.go) |
| `go_exported` | `func GetMinimumFee(ctx sdk.Context, evmKeeper *evmkeeper.Keeper) *big.Int` | [node/ante/evm_checktx.go](../../../node/ante/evm_checktx.go) |
| `go_exported` | `func HandleAssociateTx(ctx sdk.Context, ek *evmkeeper.Keeper, atx *ethtx.AssociateTx, readOnly bool) (sdk.Context, error) {` | [node/ante/evm_checktx.go](../../../node/ante/evm_checktx.go) |
| `go_exported` | `func IsAccountBalancePositive(ctx sdk.Context, evmKeeper *evmkeeper.Keeper, paxAddr sdk.AccAddress, evmAddr common.Address) bool {` | [node/ante/evm_checktx.go](../../../node/ante/evm_checktx.go) |
| `go_exported` | `func DecorateNonceCallback(ctx sdk.Context, ek *evmkeeper.Keeper, evmAddr common.Address, txNonce uint64) sdk.Context {` | [node/ante/evm_delivertx.go](../../../node/ante/evm_delivertx.go) |
| `go_exported` | `func EvmDeliverChargeFees(ctx sdk.Context, ek *evmkeeper.Keeper, upgradeKeeper *upgradekeeper.Keeper, txData ethtx.EthereumTxData, etx *ethtypes.Transaction, msg *evmtypes.MsgEVMTransaction, version derived.SignerVersion, evmAddr common.Address) error {` | [node/ante/evm_delivertx.go](../../../node/ante/evm_delivertx.go) |
| `go_exported` | `func EvmDeliverHandleSignatures(ctx sdk.Context, ek *evmkeeper.Keeper, txData ethtx.EthereumTxData, chainID *big.Int, msg *evmtypes.MsgEVMTransaction) (common.Address, sdk.AccAddress, derived.SignerVersion, error) {` | [node/ante/evm_delivertx.go](../../../node/ante/evm_delivertx.go) |
| `go_exported` | `func EvmDeliverTxAnte(` | [node/ante/evm_delivertx.go](../../../node/ante/evm_delivertx.go) |
| `go_exported` | `type TxAuthInfo interface` | [node/ante/types.go](../../../node/ante/types.go) |
| `go_exported` | `type TxBody interface` | [node/ante/types.go](../../../node/ante/types.go) |
| `go_exported` | `type TxSignaturesV2 interface` | [node/ante/types.go](../../../node/ante/types.go) |
| `go_exported` | `type AuthzNestedMessageDecorator struct{}` | [node/antedecorators/authz_nested_message.go](../../../node/antedecorators/authz_nested_message.go) |
| `go_exported` | `func (ad AuthzNestedMessageDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {` | [node/antedecorators/authz_nested_message.go](../../../node/antedecorators/authz_nested_message.go) |
| `go_exported` | `func (ad AuthzNestedMessageDecorator) CheckAuthzContainsEvm(ctx sdk.Context, authzMsg *authz.MsgExec, nestedLvl int) (bool, error) {` | [node/antedecorators/authz_nested_message.go](../../../node/antedecorators/authz_nested_message.go) |
| `go_exported` | `func NewAuthzNestedMessageDecorator() AuthzNestedMessageDecorator` | [node/antedecorators/authz_nested_message.go](../../../node/antedecorators/authz_nested_message.go) |
| `go_exported` | `func NewFeeDenomTxFeeChecker(k *evmkeeper.Keeper, inner ante.TxFeeChecker) ante.TxFeeChecker {` | [node/antedecorators/feedenom.go](../../../node/antedecorators/feedenom.go) |
| `go_exported` | `func GetGasMeterSetter(pk paramskeeper.Keeper) func(bool, sdk.Context, uint64, sdk.Tx) sdk.Context {` | [node/antedecorators/gas.go](../../../node/antedecorators/gas.go) |
| `go_exported` | `type GaslessDecorator struct` | [node/antedecorators/gasless.go](../../../node/antedecorators/gasless.go) |
| `go_exported` | `func (gd GaslessDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {` | [node/antedecorators/gasless.go](../../../node/antedecorators/gasless.go) |
| `go_exported` | `func IsTxGasless(tx sdk.Tx, ctx sdk.Context, oracleKeeper oraclekeeper.Keeper, evmKeeper *evmkeeper.Keeper, pricefeedKeeper pricefeedkeeper.Keeper) (isGasless bool, err error) {` | [node/antedecorators/gasless.go](../../../node/antedecorators/gasless.go) |
| `go_exported` | `func NewGaslessDecorator(wrapped []sdk.AnteDecorator, oracleKeeper oraclekeeper.Keeper, evmKeeper *evmkeeper.Keeper, pricefeedKeeper pricefeedkeeper.Keeper) GaslessDecorator {` | [node/antedecorators/gasless.go](../../../node/antedecorators/gasless.go) |
| `go_exported` | `EVMAssociatePriority = math.MaxInt64 - 101` | [node/antedecorators/priority.go](../../../node/antedecorators/priority.go) |
| `go_exported` | `MaxPriority = math.MaxInt64 - 1000` | [node/antedecorators/priority.go](../../../node/antedecorators/priority.go) |
| `go_exported` | `func NewPriorityDecorator() PriorityDecorator` | [node/antedecorators/priority.go](../../../node/antedecorators/priority.go) |
| `go_exported` | `OraclePriority = math.MaxInt64 - 100` | [node/antedecorators/priority.go](../../../node/antedecorators/priority.go) |
| `go_exported` | `type PriorityDecorator struct{}` | [node/antedecorators/priority.go](../../../node/antedecorators/priority.go) |
| `go_exported` | `func (pd PriorityDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {` | [node/antedecorators/priority.go](../../../node/antedecorators/priority.go) |
| `go_exported` | `func NewTracedAnteDecorator(wrapped sdk.AnteDecorator, tracingInfo *tracing.Info) TracedAnteDecorator {` | [node/antedecorators/traced.go](../../../node/antedecorators/traced.go) |
| `go_exported` | `type TracedAnteDecorator struct` | [node/antedecorators/traced.go](../../../node/antedecorators/traced.go) |
| `go_exported` | `func (d TracedAnteDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {` | [node/antedecorators/traced.go](../../../node/antedecorators/traced.go) |

## Execution engine

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `RegisterSendEnabledChecker(SendEnabledChecker)` | [engine/deps/xbank/keeper/send.go](../../../engine/deps/xbank/keeper/send.go) |
| `go_exported` | `var AllCustomPrecompilesFailFastMerge = map[common.Address]vm.PrecompiledContract{}` | [engine/executor/precompiles/failfast.go](../../../engine/executor/precompiles/failfast.go) |
| `go_exported` | `var MergeFailFastPrecompileAddresses = []common.Address{` | [engine/executor/precompiles/failfast.go](../../../engine/executor/precompiles/failfast.go) |
| `go_exported` | `var ErrIteratorUnsupported = errors.New("unexpected iterator call on cachekv store")` | [engine/deps/store/cachekv.go](../../../engine/deps/store/cachekv.go) |
| `go_exported` | `func NewStore(parent types.KVStore, storeKey types.StoreKey, cacheSize int) *Store` | [engine/deps/store/cachekv.go](../../../engine/deps/store/cachekv.go) |
| `go_exported` | `type Store struct` | [engine/deps/store/cachekv.go](../../../engine/deps/store/cachekv.go) |
| `go_exported` | `func NewScheduler(workers int, tracingInfo *tracing.Info, deliverTxFunc func(ctx sdk.Context, req types.RequestDeliverTxV2, tx sdk.Tx, checksum [32]byte) (res types.ResponseDeliverTx)) Scheduler {` | [engine/deps/tasks/scheduler.go](../../../engine/deps/tasks/scheduler.go) |
| `go_exported` | `func MockAddressPair() (sdk.AccAddress, common.Address)` | [engine/deps/testutil/keeper/evm.go](../../../engine/deps/testutil/keeper/evm.go) |
| `go_exported` | `func MockApp(t *testing.T) (*app.App, sdk.Context)` | [engine/deps/testutil/keeper/evm.go](../../../engine/deps/testutil/keeper/evm.go) |
| `go_exported` | `func MockEVMKeeper(t *testing.T) (*evmkeeper.Keeper, sdk.Context)` | [engine/deps/testutil/keeper/evm.go](../../../engine/deps/testutil/keeper/evm.go) |
| `go_exported` | `func MockPrivateKey() cryptotypes.PrivKey` | [engine/deps/testutil/keeper/evm.go](../../../engine/deps/testutil/keeper/evm.go) |
| `go_exported` | `type GenesisBalance interface` | [engine/deps/xbank/exported/exported.go](../../../engine/deps/xbank/exported/exported.go) |
| `go_exported` | `type BaseKeeper struct` | [engine/deps/xbank/keeper/keeper.go](../../../engine/deps/xbank/keeper/keeper.go) |
| `go_exported` | `type Keeper interface` | [engine/deps/xbank/keeper/keeper.go](../../../engine/deps/xbank/keeper/keeper.go) |
| `go_exported` | `func NewBaseKeeperWithDeferredCache(` | [engine/deps/xbank/keeper/keeper.go](../../../engine/deps/xbank/keeper/keeper.go) |
| `go_exported` | `func SanitizeGenesisBalances(balances []Balance) []Balance` | [engine/deps/xbank/types/balance.go](../../../engine/deps/xbank/types/balance.go) |
| `go_exported` | `type AllowList struct` | [engine/deps/xbank/types/bank.pb.go](../../../engine/deps/xbank/types/bank.pb.go) |
| `go_exported` | `type DenomUnit struct` | [engine/deps/xbank/types/bank.pb.go](../../../engine/deps/xbank/types/bank.pb.go) |
| `go_exported` | `type Input struct` | [engine/deps/xbank/types/bank.pb.go](../../../engine/deps/xbank/types/bank.pb.go) |
| `go_exported` | `type Metadata struct` | [engine/deps/xbank/types/bank.pb.go](../../../engine/deps/xbank/types/bank.pb.go) |
| `go_exported` | `type Output struct` | [engine/deps/xbank/types/bank.pb.go](../../../engine/deps/xbank/types/bank.pb.go) |
| `go_exported` | `type Params struct` | [engine/deps/xbank/types/bank.pb.go](../../../engine/deps/xbank/types/bank.pb.go) |
| `go_exported` | `type SendEnabled struct` | [engine/deps/xbank/types/bank.pb.go](../../../engine/deps/xbank/types/bank.pb.go) |
| `go_exported` | `ErrSendDisabled = bankerrors.ErrSendDisabled` | [engine/deps/xbank/types/errors.go](../../../engine/deps/xbank/types/errors.go) |
| `go_exported` | `AttributeKeyRecipient = "recipient"` | [engine/deps/xbank/types/events.go](../../../engine/deps/xbank/types/events.go) |
| `go_exported` | `AttributeKeySender = "sender"` | [engine/deps/xbank/types/events.go](../../../engine/deps/xbank/types/events.go) |
| `go_exported` | `EventTypeTransfer = "transfer"` | [engine/deps/xbank/types/events.go](../../../engine/deps/xbank/types/events.go) |
| `go_exported` | `EventTypeWeiTransfer = "wei_transfer"` | [engine/deps/xbank/types/events.go](../../../engine/deps/xbank/types/events.go) |
| `go_exported` | `func NewCoinBurnEvent(burner sdk.AccAddress, amount sdk.Coins) sdk.Event` | [engine/deps/xbank/types/events.go](../../../engine/deps/xbank/types/events.go) |
| `go_exported` | `func NewCoinMintEvent(minter sdk.AccAddress, amount sdk.Coins) sdk.Event` | [engine/deps/xbank/types/events.go](../../../engine/deps/xbank/types/events.go) |
| `go_exported` | `func NewCoinReceivedEvent(receiver sdk.AccAddress, amount sdk.Coins) sdk.Event` | [engine/deps/xbank/types/events.go](../../../engine/deps/xbank/types/events.go) |
| `go_exported` | `func NewCoinSpentEvent(spender sdk.AccAddress, amount sdk.Coins) sdk.Event` | [engine/deps/xbank/types/events.go](../../../engine/deps/xbank/types/events.go) |
| `go_exported` | `func NewWeiReceivedEvent(receiver sdk.AccAddress, amount sdk.Int) sdk.Event` | [engine/deps/xbank/types/events.go](../../../engine/deps/xbank/types/events.go) |
| `go_exported` | `func NewWeiSpentEvent(spender sdk.AccAddress, amount sdk.Int) sdk.Event` | [engine/deps/xbank/types/events.go](../../../engine/deps/xbank/types/events.go) |
| `go_exported` | `type AccountKeeper interface` | [engine/deps/xbank/types/expected_keepers.go](../../../engine/deps/xbank/types/expected_keepers.go) |
| `go_exported` | `func DefaultGenesisState() *GenesisState` | [engine/deps/xbank/types/genesis.go](../../../engine/deps/xbank/types/genesis.go) |
| `go_exported` | `func NewGenesisState(params Params, balances []Balance, supply sdk.Coins, denomMetaData []Metadata, weiBalances []WeiBalance) *GenesisState {` | [engine/deps/xbank/types/genesis.go](../../../engine/deps/xbank/types/genesis.go) |
| `go_exported` | `type Balance struct` | [engine/deps/xbank/types/genesis.pb.go](../../../engine/deps/xbank/types/genesis.pb.go) |
| `go_exported` | `type GenesisState struct` | [engine/deps/xbank/types/genesis.pb.go](../../../engine/deps/xbank/types/genesis.pb.go) |
| `go_exported` | `type WeiBalance struct` | [engine/deps/xbank/types/genesis.pb.go](../../../engine/deps/xbank/types/genesis.pb.go) |
| `go_exported` | `func AddressFromDeferredCacheStore(key []byte) (sdk.AccAddress, error)` | [engine/deps/xbank/types/key.go](../../../engine/deps/xbank/types/key.go) |
| `go_exported` | `func CreateAccountBalancesPrefix(addr []byte) []byte` | [engine/deps/xbank/types/key.go](../../../engine/deps/xbank/types/key.go) |
| `go_exported` | `func CreateDeferredCacheModuleTxIndexedPrefix(moduleAddr []byte, index uint64) []byte` | [engine/deps/xbank/types/key.go](../../../engine/deps/xbank/types/key.go) |
| `go_exported` | `DeferredCachePrefix = []byte{0x03}` | [engine/deps/xbank/types/key.go](../../../engine/deps/xbank/types/key.go) |
| `go_exported` | `DeferredCacheStoreKey = "deferredcache"` | [engine/deps/xbank/types/key.go](../../../engine/deps/xbank/types/key.go) |
| `go_exported` | `func DenomAllowListKey(denom string) []byte` | [engine/deps/xbank/types/key.go](../../../engine/deps/xbank/types/key.go) |
| `go_exported` | `func DenomMetadataKey(denom string) []byte` | [engine/deps/xbank/types/key.go](../../../engine/deps/xbank/types/key.go) |
| `go_exported` | `ModuleName = "bank"` | [engine/deps/xbank/types/key.go](../../../engine/deps/xbank/types/key.go) |
| `go_exported` | `StoreKey = ModuleName` | [engine/deps/xbank/types/key.go](../../../engine/deps/xbank/types/key.go) |
| `go_exported` | `SupplyKey = []byte{0x00}` | [engine/deps/xbank/types/key.go](../../../engine/deps/xbank/types/key.go) |
| `go_exported` | `WeiBalancesPrefix = []byte{0x04}` | [engine/deps/xbank/types/key.go](../../../engine/deps/xbank/types/key.go) |
| `go_exported` | `func ValidateInputsOutputs(inputs []Input, outputs []Output) error` | [engine/deps/xbank/types/msgs.go](../../../engine/deps/xbank/types/msgs.go) |
| `go_exported` | `func ParamKeyTable() paramtypes.KeyTable` | [engine/deps/xbank/types/params.go](../../../engine/deps/xbank/types/params.go) |
| `go_exported` | `const DefaultChainID = canonical.DefaultChainID` | [engine/deps/xevm/config/config.go](../../../engine/deps/xevm/config/config.go) |
| `go_exported` | `func GetEVMChainID(cosmosChainID string) *big.Int` | [engine/deps/xevm/config/config.go](../../../engine/deps/xevm/config/config.go) |
| `go_exported` | `type Derived struct` | [engine/deps/xevm/derived/derived.go](../../../engine/deps/xevm/derived/derived.go) |
| `go_exported` | `type Keeper struct` | [engine/deps/xevm/keeper/keeper.go](../../../engine/deps/xevm/keeper/keeper.go) |
| `go_exported` | `func NewKeeper(` | [engine/deps/xevm/keeper/keeper.go](../../../engine/deps/xevm/keeper/keeper.go) |
| `go_exported` | `var ZeroInt = uint256.NewInt(0)` | [engine/deps/xevm/state/balance.go](../../../engine/deps/xevm/state/balance.go) |
| `go_exported` | `type DBImpl struct` | [engine/deps/xevm/state/statedb.go](../../../engine/deps/xevm/state/statedb.go) |
| `go_exported` | `func NewDBImpl(ctx sdk.Context, k EVMKeeper, simulation bool) *DBImpl` | [engine/deps/xevm/state/statedb.go](../../../engine/deps/xevm/state/statedb.go) |
| `go_exported` | `func TransferWithoutEvents(db vm.StateDB, sender, recipient common.Address, amount *uint256.Int) {` | [engine/deps/xevm/state/transfer.go](../../../engine/deps/xevm/state/transfer.go) |
| `go_exported` | `func GetCoinbaseAddress(txIdx int) sdk.AccAddress` | [engine/deps/xevm/state/utils.go](../../../engine/deps/xevm/state/utils.go) |
| `go_exported` | `var SdkUhpxToSweiMultiplier = sdk.NewIntFromBigInt(UhpxToSweiMultiplier)` | [engine/deps/xevm/state/utils.go](../../../engine/deps/xevm/state/utils.go) |
| `go_exported` | `func DefaultChainConfig() ChainConfig` | [engine/deps/xevm/types/config.go](../../../engine/deps/xevm/types/config.go) |
| `go_exported` | `func NewDynamicFeeTx(tx *ethtypes.Transaction) (*DynamicFeeTx, error)` | [engine/deps/xevm/types/ethtx/dynamic_fee_tx.go](../../../engine/deps/xevm/types/ethtx/dynamic_fee_tx.go) |
| `go_exported` | `type AccessListTx struct` | [engine/deps/xevm/types/ethtx/tx.pb.go](../../../engine/deps/xevm/types/ethtx/tx.pb.go) |
| `go_exported` | `type AccessTuple struct` | [engine/deps/xevm/types/ethtx/tx.pb.go](../../../engine/deps/xevm/types/ethtx/tx.pb.go) |
| `go_exported` | `type AssociateTx struct` | [engine/deps/xevm/types/ethtx/tx.pb.go](../../../engine/deps/xevm/types/ethtx/tx.pb.go) |
| `go_exported` | `type BlobTx struct` | [engine/deps/xevm/types/ethtx/tx.pb.go](../../../engine/deps/xevm/types/ethtx/tx.pb.go) |
| `go_exported` | `type DynamicFeeTx struct` | [engine/deps/xevm/types/ethtx/tx.pb.go](../../../engine/deps/xevm/types/ethtx/tx.pb.go) |
| `go_exported` | `type LegacyTx struct` | [engine/deps/xevm/types/ethtx/tx.pb.go](../../../engine/deps/xevm/types/ethtx/tx.pb.go) |
| `go_exported` | `type SetCodeTx struct` | [engine/deps/xevm/types/ethtx/tx.pb.go](../../../engine/deps/xevm/types/ethtx/tx.pb.go) |
| `go_exported` | `type EthereumTxData interface` | [engine/deps/xevm/types/ethtx/txdata.go](../../../engine/deps/xevm/types/ethtx/txdata.go) |
| `go_exported` | `type TxData interface` | [engine/deps/xevm/types/ethtx/txdata.go](../../../engine/deps/xevm/types/ethtx/txdata.go) |
| `go_exported` | `AttributeKeyEvmAddress = "evm_addr"` | [engine/deps/xevm/types/events.go](../../../engine/deps/xevm/types/events.go) |
| `go_exported` | `AttributeKeyPaxAddress = "pax_addr"` | [engine/deps/xevm/types/events.go](../../../engine/deps/xevm/types/events.go) |
| `go_exported` | `EventTypeAddressAssociated = "address_associated"` | [engine/deps/xevm/types/events.go](../../../engine/deps/xevm/types/events.go) |
| `go_exported` | `func DefaultGenesis() *GenesisState` | [engine/deps/xevm/types/genesis.go](../../../engine/deps/xevm/types/genesis.go) |
| `go_exported` | `type GenesisState struct` | [engine/deps/xevm/types/genesis.pb.go](../../../engine/deps/xevm/types/genesis.pb.go) |
| `go_exported` | `AnteSurplusPrefix = []byte{0x18} // transient` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `go_exported` | `BaseFeePerGasPrefix = []byte{0x1b}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `go_exported` | `func BlockBloomKey(height int64) []byte` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `go_exported` | `BlockBloomPrefix = []byte{0x0d}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `go_exported` | `CodeHashKeyPrefix = []byte{0x08}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `go_exported` | `CodeKeyPrefix = []byte{0x07}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `go_exported` | `CodeSizeKeyPrefix = []byte{0x09}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `go_exported` | `DeferredInfoPrefix = []byte{0x19} // transient` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `go_exported` | `func EVMAddressToPaxAddressKey(evmAddress common.Address) []byte` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `go_exported` | `EVMAddressToPaxAddressKeyPrefix = []byte{0x01}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `go_exported` | `EvmOnlyBlockBloomPrefix = []byte{0x1d}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `go_exported` | `LegacyBlockBloomCutoffHeightKey = []byte{0x1a}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `go_exported` | `ModuleName = "evm"` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `go_exported` | `func NewTransientReceiptKey(txIndex uint64, txHash common.Hash) TransientReceiptKey` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `go_exported` | `NextBaseFeePerGasPrefix = []byte{0x1c}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `go_exported` | `NonceKeyPrefix = []byte{0x0a}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `go_exported` | `func PaxAddressToEVMAddressKey(paxAddress sdk.AccAddress) []byte` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `go_exported` | `func ReceiptKey(txHash common.Hash) []byte` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `go_exported` | `func StateKey(evmAddress common.Address) []byte` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `go_exported` | `StateKeyPrefix = []byte{0x03}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `go_exported` | `func NewMsgEVMTransaction(txData proto.Message) (*MsgEVMTransaction, error)` | [engine/deps/xevm/types/message_evm_transaction.go](../../../engine/deps/xevm/types/message_evm_transaction.go) |
| `go_exported` | `var DefaultBaseFeePerGas = sdk.NewDec(0)         // used for static base fee, deprecated in favor of dynamic base fee` | [engine/deps/xevm/types/params.go](../../../engine/deps/xevm/types/params.go) |
| `go_exported` | `var DefaultDeliverTxHookWasmGasLimit = uint64(300000)` | [engine/deps/xevm/types/params.go](../../../engine/deps/xevm/types/params.go) |
| `go_exported` | `var DefaultMaxDynamicBaseFeeDownwardAdjustment = sdk.NewDecWithPrec(39, 4) // .39%` | [engine/deps/xevm/types/params.go](../../../engine/deps/xevm/types/params.go) |
| `go_exported` | `var DefaultMaxDynamicBaseFeeUpwardAdjustment = sdk.NewDecWithPrec(189, 4) // 1.89%` | [engine/deps/xevm/types/params.go](../../../engine/deps/xevm/types/params.go) |
| `go_exported` | `var DefaultMaxFeePerGas = sdk.NewDec(1000000000000) // 1,000gwei` | [engine/deps/xevm/types/params.go](../../../engine/deps/xevm/types/params.go) |
| `go_exported` | `var DefaultMinFeePerGas = sdk.NewDec(1000000000) // 1gwei` | [engine/deps/xevm/types/params.go](../../../engine/deps/xevm/types/params.go) |
| `go_exported` | `func DefaultParams() Params` | [engine/deps/xevm/types/params.go](../../../engine/deps/xevm/types/params.go) |
| `go_exported` | `var DefaultPriorityNormalizer = sdk.NewDec(1)` | [engine/deps/xevm/types/params.go](../../../engine/deps/xevm/types/params.go) |
| `go_exported` | `var DefaultRegisterPointerDisabled = false` | [engine/deps/xevm/types/params.go](../../../engine/deps/xevm/types/params.go) |
| `go_exported` | `var DefaultTargetGasUsedPerBlock = uint64(250000) // 250k` | [engine/deps/xevm/types/params.go](../../../engine/deps/xevm/types/params.go) |
| `go_exported` | `KeyBaseFeePerGas = []byte("KeyBaseFeePerGas")` | [engine/deps/xevm/types/params.go](../../../engine/deps/xevm/types/params.go) |
| `go_exported` | `KeyDeliverTxHookWasmGasLimit = []byte("KeyDeliverTxHookWasmGasLimit")` | [engine/deps/xevm/types/params.go](../../../engine/deps/xevm/types/params.go) |
| `go_exported` | `KeyMaxDynamicBaseFeeDownwardAdjustment = []byte("KeyMaxDynamicBaseFeeDownwardAdjustment")` | [engine/deps/xevm/types/params.go](../../../engine/deps/xevm/types/params.go) |
| `go_exported` | `KeyMaxDynamicBaseFeeUpwardAdjustment = []byte("KeyMaxDynamicBaseFeeUpwardAdjustment")` | [engine/deps/xevm/types/params.go](../../../engine/deps/xevm/types/params.go) |
| `go_exported` | `KeyMaxFeePerGas = []byte("KeyMaximumFeePerGas")` | [engine/deps/xevm/types/params.go](../../../engine/deps/xevm/types/params.go) |
| `go_exported` | `KeyMinFeePerGas = []byte("KeyMinFeePerGas")` | [engine/deps/xevm/types/params.go](../../../engine/deps/xevm/types/params.go) |
| `go_exported` | `KeyPriorityNormalizer = []byte("KeyPriorityNormalizer")` | [engine/deps/xevm/types/params.go](../../../engine/deps/xevm/types/params.go) |
| `go_exported` | `KeyRegisterPointerDisabled = []byte("KeyRegisterPointerDisabled")` | [engine/deps/xevm/types/params.go](../../../engine/deps/xevm/types/params.go) |
| `go_exported` | `KeyTargetGasUsedPerBlock = []byte("KeyTargetGasUsedPerBlock")` | [engine/deps/xevm/types/params.go](../../../engine/deps/xevm/types/params.go) |
| `go_exported` | `func ParamKeyTable() paramtypes.KeyTable` | [engine/deps/xevm/types/params.go](../../../engine/deps/xevm/types/params.go) |
| `go_exported` | `type Params struct` | [engine/deps/xevm/types/params.pb.go](../../../engine/deps/xevm/types/params.pb.go) |
| `go_exported` | `type ParamsPreV580 struct` | [engine/deps/xevm/types/params.pb.go](../../../engine/deps/xevm/types/params.pb.go) |
| `go_exported` | `type ParamsPreV600 struct` | [engine/deps/xevm/types/params.pb.go](../../../engine/deps/xevm/types/params.pb.go) |
| `go_exported` | `type ParamsPreV601 struct` | [engine/deps/xevm/types/params.pb.go](../../../engine/deps/xevm/types/params.pb.go) |
| `go_exported` | `type ParamsPreV606 struct` | [engine/deps/xevm/types/params.pb.go](../../../engine/deps/xevm/types/params.pb.go) |
| `go_exported` | `type Log struct` | [engine/deps/xevm/types/receipt.pb.go](../../../engine/deps/xevm/types/receipt.pb.go) |
| `go_exported` | `type Receipt struct` | [engine/deps/xevm/types/receipt.pb.go](../../../engine/deps/xevm/types/receipt.pb.go) |
| `go_exported` | `type MsgEVMTransaction struct` | [engine/deps/xevm/types/tx.pb.go](../../../engine/deps/xevm/types/tx.pb.go) |
| `go_exported` | `type DeferredInfo struct` | [engine/deps/xevm/types/types.pb.go](../../../engine/deps/xevm/types/types.pb.go) |
| `config_key` | `FlagEnabled = "giga_executor.enabled"` | [engine/executor/config/config.go](../../../engine/executor/config/config.go) |
| `config_key` | `FlagOCCEnabled = "giga_executor.occ_enabled"` | [engine/executor/config/config.go](../../../engine/executor/config/config.go) |
| `go_exported` | `type Config struct` | [engine/executor/config/config.go](../../../engine/executor/config/config.go) |
| `go_exported` | ``const ConfigTemplate = ` `` | [engine/executor/config/config.go](../../../engine/executor/config/config.go) |
| `go_exported` | `var DefaultConfig = Config` | [engine/executor/config/config.go](../../../engine/executor/config/config.go) |
| `go_exported` | `FlagEnabled = "giga_executor.enabled"` | [engine/executor/config/config.go](../../../engine/executor/config/config.go) |
| `go_exported` | `FlagOCCEnabled = "giga_executor.occ_enabled"` | [engine/executor/config/config.go](../../../engine/executor/config/config.go) |
| `go_exported` | `func ReadConfig(opts servertypes.AppOptions) (Config, error)` | [engine/executor/config/config.go](../../../engine/executor/config/config.go) |
| `go_exported` | `func NewGethExecutor(blockCtx vm.BlockContext, stateDB vm.StateDB, chainConfig *params.ChainConfig, config vm.Config, customPrecompiles map[common.Address]vm.PrecompiledContract) *Executor {` | [engine/executor/executor.go](../../../engine/executor/executor.go) |
| `go_exported` | `func NewHostContext(vm *evmc.VM, evm *vm.EVM, config HostContextConfig) *HostContext` | [engine/executor/internal/host_context.go](../../../engine/executor/internal/host_context.go) |
| `go_exported` | `func NewHostContextConfig(chainConfig *params.ChainConfig) HostContextConfig` | [engine/executor/internal/host_context.go](../../../engine/executor/internal/host_context.go) |
| `go_exported` | `func NewEVMInterpreter(hostContext *HostContext, evm *vm.EVM) *EVMInterpreter` | [engine/executor/internal/interpreter.go](../../../engine/executor/internal/interpreter.go) |
| `go_exported` | `func TransactionToMessage(tx *types.Transaction, sender common.Address, baseFee *big.Int) (*core.Message, error) {` | [engine/executor/internal/message.go](../../../engine/executor/internal/message.go) |
| `go_exported` | `func InitEvmoneVM() (*evmc.VM, error)` | [engine/executor/lib/evmlib.go](../../../engine/executor/lib/evmlib.go) |
| `go_exported` | `var AllCustomPrecompilesFailFast = map[common.Address]vm.PrecompiledContract{}` | [engine/executor/precompiles/failfast.go](../../../engine/executor/precompiles/failfast.go) |
| `go_exported` | `var AllCustomPrecompilesFailFastLate = map[common.Address]vm.PrecompiledContract{}` | [engine/executor/precompiles/failfast.go](../../../engine/executor/precompiles/failfast.go) |
| `go_exported` | `var ErrBalanceMigrationRequired error = &BalanceMigrationAbortError{}` | [engine/executor/precompiles/failfast.go](../../../engine/executor/precompiles/failfast.go) |
| `go_exported` | `var ErrSelfDestructUnsupported error = &SelfDestructAbortError{}` | [engine/executor/precompiles/failfast.go](../../../engine/executor/precompiles/failfast.go) |
| `go_exported` | `var FailFastPrecompileAddresses = []common.Address` | [engine/executor/precompiles/failfast.go](../../../engine/executor/precompiles/failfast.go) |
| `go_exported` | `var FailFastSingleton vm.PrecompiledContract = &FailFastPrecompile{}` | [engine/executor/precompiles/failfast.go](../../../engine/executor/precompiles/failfast.go) |
| `go_exported` | `var LateFailFastPrecompileAddresses = []common.Address` | [engine/executor/precompiles/failfast.go](../../../engine/executor/precompiles/failfast.go) |
| `go_exported` | `GigaAbortCode = uint32(1)` | [engine/executor/utils/errors.go](../../../engine/executor/utils/errors.go) |
| `go_exported` | `GigaAbortCodespace = "giga"` | [engine/executor/utils/errors.go](../../../engine/executor/utils/errors.go) |
| `go_exported` | `GigaAbortInfo = "giga_fallback_to_v2"` | [engine/executor/utils/errors.go](../../../engine/executor/utils/errors.go) |
| `go_exported` | `func ShouldExecutionAbort(err error) bool` | [engine/executor/utils/errors.go](../../../engine/executor/utils/errors.go) |
| `go_exported` | `func BuildTransaction(st *StateTestJSON, subtest StateTestPost) (*ethtypes.Transaction, common.Address, error) {` | [engine/tests/harness/builder.go](../../../engine/tests/harness/builder.go) |
| `go_exported` | `func EncodeTxForApp(signedTx *ethtypes.Transaction) ([]byte, error)` | [engine/tests/harness/builder.go](../../../engine/tests/harness/builder.go) |
| `go_exported` | `func GetStateTestsPath() (string, error)` | [engine/tests/harness/loader.go](../../../engine/tests/harness/loader.go) |
| `go_exported` | `func LoadStateTestsFromDir(dirPath string) (map[string]*StateTestJSON, error)` | [engine/tests/harness/loader.go](../../../engine/tests/harness/loader.go) |
| `go_exported` | `func LoadSkipList() (*SkipList, error)` | [engine/tests/harness/skip.go](../../../engine/tests/harness/skip.go) |
| `go_exported` | `type StateTestJSON struct` | [engine/tests/harness/types.go](../../../engine/tests/harness/types.go) |
| `go_exported` | `type StateTestPost struct` | [engine/tests/harness/types.go](../../../engine/tests/harness/types.go) |

## Storage

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `LayerXAuthStoreKey = "layerxauth" // modules/layerxauth/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `go_exported` | `LayerXLedgerStoreKey = "layerxledger" // modules/layerxledger/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `go_exported` | `LayerXPayStoreKey = "layerxpay" // modules/layerxpay/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `go_exported` | `var MemIAVLStoreKeys = []string` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `LayerXAuthStoreKey = "layerxauth" // modules/layerxauth/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `LayerXLedgerStoreKey = "layerxledger" // modules/layerxledger/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `LayerXPayStoreKey = "layerxpay" // modules/layerxpay/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `go_exported` | ``const StateCommitConfigTemplate = ` `` | [storage/config/toml.go](../../../storage/config/toml.go) |
| `go_exported` | `LayerXAgentStoreKey = "layerxagent"` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `go_exported` | `LayerXAnchorStoreKey = "layerxanchor" // modules/layerxanchor/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `LayerXAnchorStoreKey = "layerxanchor" // modules/layerxanchor/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `BalancesPrefix = []byte{0x02}` | [engine/deps/xbank/types/key.go](../../../engine/deps/xbank/types/key.go) |
| `storage_key` | `DeferredCachePrefix = []byte{0x03}` | [engine/deps/xbank/types/key.go](../../../engine/deps/xbank/types/key.go) |
| `storage_key` | `DeferredCacheStoreKey = "deferredcache"` | [engine/deps/xbank/types/key.go](../../../engine/deps/xbank/types/key.go) |
| `storage_key` | `DenomAllowListPrefix = []byte{0x11}` | [engine/deps/xbank/types/key.go](../../../engine/deps/xbank/types/key.go) |
| `storage_key` | `DenomMetadataPrefix = []byte{0x1}` | [engine/deps/xbank/types/key.go](../../../engine/deps/xbank/types/key.go) |
| `storage_key` | `ModuleName = "bank"` | [engine/deps/xbank/types/key.go](../../../engine/deps/xbank/types/key.go) |
| `storage_key` | `SupplyKey = []byte{0x00}` | [engine/deps/xbank/types/key.go](../../../engine/deps/xbank/types/key.go) |
| `storage_key` | `WeiBalancesPrefix = []byte{0x04}` | [engine/deps/xbank/types/key.go](../../../engine/deps/xbank/types/key.go) |
| `storage_key` | `var CoinbaseAddressPrefix = []byte("evm_coinbase")` | [engine/deps/xevm/state/utils.go](../../../engine/deps/xevm/state/utils.go) |
| `storage_key` | `AccountTransientStateKeyPrefix = []byte{0x05} // deprecated` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `AnteSurplusPrefix = []byte{0x18} // transient` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `BaseFeePerGasPrefix = []byte{0x1b}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `BlockBloomPrefix = []byte{0x0d}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `CodeHashKeyPrefix = []byte{0x08}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `CodeKeyPrefix = []byte{0x07}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `CodeSizeKeyPrefix = []byte{0x09}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `DeferredInfoPrefix = []byte{0x19} // transient` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `EVMAddressToPaxAddressKeyPrefix = []byte{0x01}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `EvmOnlyBlockBloomPrefix = []byte{0x1d}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `LegacyBlockBloomCutoffHeightKey = []byte{0x1a}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `ModuleName = "evm"` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `NextBaseFeePerGasPrefix = []byte{0x1c}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `NonceKeyPrefix = []byte{0x0a}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `PaxAddressToEVMAddressKeyPrefix = []byte{0x02}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `PointerCWCodePrefix = []byte{0x16}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `PointerCW1155ERC1155Prefix = []byte{0x6}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `PointerCW20ERC20Prefix = []byte{0x3}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `PointerCW721ERC721Prefix = []byte{0x4}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `PointerERC1155CW1155Prefix = []byte{0x5}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `PointerERC20CW20Prefix = []byte{0x1}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `PointerERC20NativePrefix = []byte{0x0}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `PointerERC721CW721Prefix = []byte{0x2}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `PointerRegistryPrefix = []byte{0x15}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `PointerReverseRegistryPrefix = []byte{0x17}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `ReceiptKeyPrefix = []byte{0x0b}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `ReceiptStoreKey = "receipt"` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `ReplayInitialHeight = []byte{0x14}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `ReplaySeenAddrPrefix = []byte{0x12}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `ReplayedHeight = []byte{0x13}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `StateKeyPrefix = []byte{0x03}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `StoreKey = "evm"` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `TransientModuleStateKeyPrefix = []byte{0x06} // deprecated` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `TransientStateKeyPrefix = []byte{0x04} // deprecated` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `TransientStoreKey = "evm_transient"` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `TxHashesPrefix = []byte{0x0e} // deprecated` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `WhitelistedCodeHashesForBankSendPrefix = []byte{0x0c}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `WhitelistedCodeHashesForDelegateCallPrefix = []byte{0x0f}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `ZeroStorageCleanupCheckpointKey = []byte{0x1e}` | [engine/deps/xevm/types/keys.go](../../../engine/deps/xevm/types/keys.go) |
| `storage_key` | `tkeys := sdk.NewTransientStoreKeys(paramstypes.TStoreKey, evmtypes.TransientStoreKey)` | [node/app.go](../../../node/app.go) |
| `go_exported` | `ErrFileLockUnavailable = errors.New("file lock unavailable")` | [storage/common/errors/errors.go](../../../storage/common/errors/errors.go) |
| `go_exported` | `ErrKeyEmpty = errors.New("key empty")` | [storage/common/errors/errors.go](../../../storage/common/errors/errors.go) |
| `go_exported` | `ErrNotFound = errors.New("not found")` | [storage/common/errors/errors.go](../../../storage/common/errors/errors.go) |
| `go_exported` | `ErrRecordNotFound = errors.New("record not found")` | [storage/common/errors/errors.go](../../../storage/common/errors/errors.go) |
| `go_exported` | `ErrStartAfterEnd = errors.New("start key after end key")` | [storage/common/errors/errors.go](../../../storage/common/errors/errors.go) |
| `go_exported` | `ErrorExportDone = errors.New("export is complete")` | [storage/common/errors/errors.go](../../../storage/common/errors/errors.go) |
| `go_exported` | `func IsFileLockError(err error) bool` | [storage/common/errors/errors.go](../../../storage/common/errors/errors.go) |
| `go_exported` | `func IsNotFound(err error) bool` | [storage/common/errors/errors.go](../../../storage/common/errors/errors.go) |
| `go_exported` | `func Join(errs ...error) error` | [storage/common/errors/errors.go](../../../storage/common/errors/errors.go) |
| `go_exported` | `func NewDomainIterator(parent dbm.Iterator, start, end []byte) (dbm.Iterator, error)` | [storage/common/iterators/domain_iterator.go](../../../storage/common/iterators/domain_iterator.go) |
| `go_exported` | `func NewMapIterator[T any](` | [storage/common/iterators/map_iterator.go](../../../storage/common/iterators/map_iterator.go) |
| `go_exported` | `func NewMergingIterator(ascending bool, iterators ...dbm.Iterator) (dbm.Iterator, error)` | [storage/common/iterators/merging_iterator.go](../../../storage/common/iterators/merging_iterator.go) |
| `go_exported` | `type IteratorTransform func(` | [storage/common/iterators/transforming_iterator.go](../../../storage/common/iterators/transforming_iterator.go) |
| `go_exported` | `func NewTransformingIterator(parent dbm.Iterator, transform IteratorTransform) (dbm.Iterator, error) {` | [storage/common/iterators/transforming_iterator.go](../../../storage/common/iterators/transforming_iterator.go) |
| `go_exported` | `const AddressLen = 20` | [storage/common/keys/evm.go](../../../storage/common/keys/evm.go) |
| `go_exported` | `func BuildEVMKey(kind EVMKeyKind, keyBytes []byte) []byte` | [storage/common/keys/evm.go](../../../storage/common/keys/evm.go) |
| `go_exported` | `EVMKeyCode // Stripped key: 20-byte address` | [storage/common/keys/evm.go](../../../storage/common/keys/evm.go) |
| `go_exported` | `EVMKeyCodeHash // Stripped key: 20-byte address` | [storage/common/keys/evm.go](../../../storage/common/keys/evm.go) |
| `go_exported` | `EVMKeyEmpty EVMKeyKind = iota // Returned only for zero-length keys` | [storage/common/keys/evm.go](../../../storage/common/keys/evm.go) |
| `go_exported` | `type EVMKeyKind uint8` | [storage/common/keys/evm.go](../../../storage/common/keys/evm.go) |
| `go_exported` | `EVMKeyLegacy // Full original key preserved (address mappings, codesize, etc.)` | [storage/common/keys/evm.go](../../../storage/common/keys/evm.go) |
| `go_exported` | `EVMKeyNonce // Stripped key: 20-byte address` | [storage/common/keys/evm.go](../../../storage/common/keys/evm.go) |
| `go_exported` | `func EVMKeyPrefixByte(kind EVMKeyKind) (byte, bool)` | [storage/common/keys/evm.go](../../../storage/common/keys/evm.go) |
| `go_exported` | `EVMKeyStorage // Stripped key: addr\|\|slot (20+32 bytes)` | [storage/common/keys/evm.go](../../../storage/common/keys/evm.go) |
| `go_exported` | `const FlatKVStoreKey = "flatkv"` | [storage/common/keys/evm.go](../../../storage/common/keys/evm.go) |
| `go_exported` | `func ParseEVMKey(key []byte) (kind EVMKeyKind, keyBytes []byte)` | [storage/common/keys/evm.go](../../../storage/common/keys/evm.go) |
| `go_exported` | `func StateKeyPrefix() []byte { return stateKeyPrefix }` | [storage/common/keys/evm.go](../../../storage/common/keys/evm.go) |
| `storage_key` | `codeHashKeyPrefix = []byte{0x08}` | [storage/common/keys/evm.go](../../../storage/common/keys/evm.go) |
| `storage_key` | `codeKeyPrefix = []byte{0x07}` | [storage/common/keys/evm.go](../../../storage/common/keys/evm.go) |
| `storage_key` | `const FlatKVStoreKey = "flatkv"` | [storage/common/keys/evm.go](../../../storage/common/keys/evm.go) |
| `storage_key` | `nonceKeyPrefix = []byte{0x0a}` | [storage/common/keys/evm.go](../../../storage/common/keys/evm.go) |
| `storage_key` | `stateKeyPrefix = []byte{0x03}` | [storage/common/keys/evm.go](../../../storage/common/keys/evm.go) |
| `go_exported` | `func AllModulesExcept(modulesNotToInclude ...string) ([]string, error)` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `go_exported` | `BankStoreKey = "bank" // cosmos/x/bank/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `go_exported` | `EVMStoreKey = "evm" // modules/evm/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `go_exported` | `func IsMemIAVLStoreKey(name string) bool` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `go_exported` | `LaunchpadStoreKey = "launchpad" // modules/launchpad/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `go_exported` | `LayerXBridgeStoreKey = "layerxbridge" // modules/layerxbridge/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `go_exported` | `LayerXCustodyStoreKey = "layerxcustody" // modules/layerxcustody/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `go_exported` | `LayerXExchangeStoreKey = "layerxexchange" // modules/layerxexchange/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `go_exported` | `PerpsStoreKey = "perps" // modules/perps/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `go_exported` | `PriceFeedStoreKey = "pricefeed" // modules/pricefeed/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `go_exported` | `SequencerStoreKey = "sequencer" // modules/sequencer/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `go_exported` | `SpotStoreKey = "spot" // modules/spot/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `go_exported` | `StakingStoreKey = "staking" // cosmos/x/staking/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `go_exported` | `XWebStoreKey = "xweb" // modules/xweb/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `AuthStoreKey = "acc" // cosmos/x/auth/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `AuthzStoreKey = "authz" // cosmos/x/authz/keeper.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `BankStoreKey = "bank" // cosmos/x/bank/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `CapabilityStoreKey = "capability" // cosmos/x/capability/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `DistributionStoreKey = "distribution" // cosmos/x/distribution/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `EpochStoreKey = "epoch" // modules/epoch/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `EvidenceStoreKey = "evidence" // cosmos/x/evidence/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `EVMStoreKey = "evm" // modules/evm/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `FeegrantStoreKey = "feegrant" // cosmos/x/feegrant.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `GovStoreKey = "gov" // cosmos/x/gov/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `IBCStoreKey = "ibc" // ibc-go/modules/core/24-host.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `IBCTransferStoreKey = "transfer" // ibc-go/modules/apps/transfer/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `LaunchpadStoreKey = "launchpad" // modules/launchpad/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `LayerXBridgeStoreKey = "layerxbridge" // modules/layerxbridge/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `LayerXCustodyStoreKey = "layerxcustody" // modules/layerxcustody/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `LayerXExchangeStoreKey = "layerxexchange" // modules/layerxexchange/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `MintStoreKey = "mint" // modules/mint/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `OracleStoreKey = "oracle" // modules/oracle/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `ParamsStoreKey = "params" // cosmos/x/params/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `PerpsStoreKey = "perps" // modules/perps/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `PriceFeedStoreKey = "pricefeed" // modules/pricefeed/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `SequencerStoreKey = "sequencer" // modules/sequencer/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `SlashingStoreKey = "slashing" // cosmos/x/slashing/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `SpotStoreKey = "spot" // modules/spot/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `StakingStoreKey = "staking" // cosmos/x/staking/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `TokenfactoryStoreKey = "tokenfactory" // modules/tokenfactory/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `UpgradeStoreKey = "upgrade" // cosmos/x/upgrade/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `WasmStoreKey = "wasm" // wasm/x/wasm/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `storage_key` | `XWebStoreKey = "xweb" // modules/xweb/types.StoreKey` | [storage/common/keys/store_keys.go](../../../storage/common/keys/store_keys.go) |
| `go_exported` | `var ByteSizeBuckets = []float64` | [storage/common/metrics/buckets.go](../../../storage/common/metrics/buckets.go) |
| `go_exported` | `var CountBuckets = []float64` | [storage/common/metrics/buckets.go](../../../storage/common/metrics/buckets.go) |
| `go_exported` | `var LatencyBuckets = []float64` | [storage/common/metrics/buckets.go](../../../storage/common/metrics/buckets.go) |
| `go_exported` | `func NewPhaseTimer(meter metric.Meter, timerName string) *PhaseTimer` | [storage/common/metrics/phase_timer.go](../../../storage/common/metrics/phase_timer.go) |
| `go_exported` | `func NewPhaseTimerFactory(meter metric.Meter, timerName string) *PhaseTimerFactory` | [storage/common/metrics/phase_timer.go](../../../storage/common/metrics/phase_timer.go) |
| `go_exported` | `type PhaseTimer struct` | [storage/common/metrics/phase_timer.go](../../../storage/common/metrics/phase_timer.go) |
| `go_exported` | `type PhaseTimerFactory struct` | [storage/common/metrics/phase_timer.go](../../../storage/common/metrics/phase_timer.go) |
| `go_exported` | `func SetupOtelPrometheus() (*prometheus.Registry, func(context.Context) error, error)` | [storage/common/metrics/setup.go](../../../storage/common/metrics/setup.go) |
| `go_exported` | `func StartMetricsServer(ctx context.Context, gatherer prometheus.Gatherer, addr string)` | [storage/common/metrics/setup.go](../../../storage/common/metrics/setup.go) |
| `go_exported` | `type MonitoredDir struct` | [storage/common/metrics/system_metrics.go](../../../storage/common/metrics/system_metrics.go) |
| `go_exported` | `func StartSystemMetrics(ctx context.Context, prefix string, intervalSeconds int, dirs []MonitoredDir) {` | [storage/common/metrics/system_metrics.go](../../../storage/common/metrics/system_metrics.go) |
| `go_exported` | `type CannedRandom struct` | [storage/common/rand/canned_random.go](../../../storage/common/rand/canned_random.go) |
| `go_exported` | `func NewCannedRandom(` | [storage/common/rand/canned_random.go](../../../storage/common/rand/canned_random.go) |
| `go_exported` | `func NewTestRandom(fixedSeed ...int64) *TestRandom` | [storage/common/testutil/test_random.go](../../../storage/common/testutil/test_random.go) |
| `go_exported` | `func NewTestRandomNoPrint(fixedSeed ...int64) *TestRandom` | [storage/common/testutil/test_random.go](../../../storage/common/testutil/test_random.go) |
| `go_exported` | `type TestRandom struct` | [storage/common/testutil/test_random.go](../../../storage/common/testutil/test_random.go) |
| `go_exported` | `func NewAdHocPool() Pool` | [storage/common/threading/adhoc_pool.go](../../../storage/common/threading/adhoc_pool.go) |
| `go_exported` | `func InterruptiblePull[T any](ctx context.Context, ch <-chan T) (T, error)` | [storage/common/threading/chan_utils.go](../../../storage/common/threading/chan_utils.go) |
| `go_exported` | `func InterruptiblePush[T any](ctx context.Context, ch chan T, value T) error` | [storage/common/threading/chan_utils.go](../../../storage/common/threading/chan_utils.go) |
| `go_exported` | `func NewElasticPool(` | [storage/common/threading/elastic_pool.go](../../../storage/common/threading/elastic_pool.go) |
| `go_exported` | `func NewFixedPool(` | [storage/common/threading/fixed_pool.go](../../../storage/common/threading/fixed_pool.go) |
| `go_exported` | `type Pool interface` | [storage/common/threading/pool.go](../../../storage/common/threading/pool.go) |
| `go_exported` | `GB = MB * 1024` | [storage/common/unit/data_units.go](../../../storage/common/unit/data_units.go) |
| `go_exported` | `KB = 1024` | [storage/common/unit/data_units.go](../../../storage/common/unit/data_units.go) |
| `go_exported` | `MB = KB * 1024` | [storage/common/unit/data_units.go](../../../storage/common/unit/data_units.go) |
| `go_exported` | `func Clone(b []byte) []byte` | [storage/common/utils/bytes.go](../../../storage/common/utils/bytes.go) |
| `go_exported` | `type Config interface` | [storage/common/utils/config.go](../../../storage/common/utils/config.go) |
| `go_exported` | `func LoadConfigFromFile(path string, defaults Config) error` | [storage/common/utils/config.go](../../../storage/common/utils/config.go) |
| `go_exported` | `func StringifyConfig(cfg Config) (string, error)` | [storage/common/utils/config.go](../../../storage/common/utils/config.go) |
| `go_exported` | `func FormatBytes(b int64) string` | [storage/common/utils/format.go](../../../storage/common/utils/format.go) |
| `go_exported` | `func FormatDuration(d time.Duration, decimals int) string` | [storage/common/utils/format.go](../../../storage/common/utils/format.go) |
| `go_exported` | `func Int64Commas(n int64) string` | [storage/common/utils/format.go](../../../storage/common/utils/format.go) |
| `go_exported` | `func Hash64(x int64) int64` | [storage/common/utils/hash64.go](../../../storage/common/utils/hash64.go) |
| `go_exported` | `func PositiveHash64(x int64) int64` | [storage/common/utils/hash64.go](../../../storage/common/utils/hash64.go) |
| `go_exported` | `func GetChangelogPath(dbPath string) string` | [storage/common/utils/path.go](../../../storage/common/utils/path.go) |
| `go_exported` | `func GetCosmosSCStorePath(homePath string) string` | [storage/common/utils/path.go](../../../storage/common/utils/path.go) |
| `go_exported` | `func GetEVMStateStorePath(homePath string, backend string) string` | [storage/common/utils/path.go](../../../storage/common/utils/path.go) |
| `go_exported` | `func GetFlatKVPath(homePath string) string` | [storage/common/utils/path.go](../../../storage/common/utils/path.go) |
| `go_exported` | `func GetReceiptStorePath(homePath string, backend string) string` | [storage/common/utils/path.go](../../../storage/common/utils/path.go) |
| `go_exported` | `func GetStateStorePath(homePath string, backend string) string` | [storage/common/utils/path.go](../../../storage/common/utils/path.go) |
| `go_exported` | `func ResolveAndCreateDir(dir string) (string, error)` | [storage/common/utils/path.go](../../../storage/common/utils/path.go) |
| `go_exported` | `func IndexToVersion(index uint64, initialVersion uint32) int64` | [storage/common/utils/versions.go](../../../storage/common/utils/versions.go) |
| `go_exported` | `func NextVersion(v int64, initialVersion uint32) int64` | [storage/common/utils/versions.go](../../../storage/common/utils/versions.go) |
| `go_exported` | `func VersionToIndex(version int64, initialVersion uint32) uint64` | [storage/common/utils/versions.go](../../../storage/common/utils/versions.go) |
| `go_exported` | `type AppOptions interface` | [storage/config/receipt_config.go](../../../storage/config/receipt_config.go) |
| `go_exported` | `func DefaultReceiptStoreConfig() ReceiptStoreConfig` | [storage/config/receipt_config.go](../../../storage/config/receipt_config.go) |
| `go_exported` | `func ReadReceiptConfig(opts AppOptions) (ReceiptStoreConfig, error)` | [storage/config/receipt_config.go](../../../storage/config/receipt_config.go) |
| `go_exported` | `type ReceiptStoreConfig struct` | [storage/config/receipt_config.go](../../../storage/config/receipt_config.go) |
| `go_exported` | `func DefaultStateCommitConfig() StateCommitConfig` | [storage/config/sc_config.go](../../../storage/config/sc_config.go) |
| `go_exported` | `type StateCommitConfig struct` | [storage/config/sc_config.go](../../../storage/config/sc_config.go) |
| `go_exported` | `DefaultSSAsyncBuffer = 100` | [storage/config/ss_config.go](../../../storage/config/ss_config.go) |
| `go_exported` | `DefaultSSBackend = PebbleDBBackend` | [storage/config/ss_config.go](../../../storage/config/ss_config.go) |
| `go_exported` | `DefaultSSImportWorkers = 1` | [storage/config/ss_config.go](../../../storage/config/ss_config.go) |
| `go_exported` | `DefaultSSKeepRecent = 100000` | [storage/config/ss_config.go](../../../storage/config/ss_config.go) |
| `go_exported` | `DefaultSSPruneInterval = 600` | [storage/config/ss_config.go](../../../storage/config/ss_config.go) |
| `go_exported` | `func DefaultStateStoreConfig() StateStoreConfig` | [storage/config/ss_config.go](../../../storage/config/ss_config.go) |
| `go_exported` | `PebbleDBBackend = "pebbledb"` | [storage/config/ss_config.go](../../../storage/config/ss_config.go) |
| `go_exported` | `RocksDBBackend = "rocksdb"` | [storage/config/ss_config.go](../../../storage/config/ss_config.go) |
| `go_exported` | `type StateStoreConfig struct` | [storage/config/ss_config.go](../../../storage/config/ss_config.go) |
| `go_exported` | ``const ReceiptStoreConfigTemplate = ` `` | [storage/config/toml.go](../../../storage/config/toml.go) |
| `go_exported` | ``const StateStoreConfigTemplate = ` `` | [storage/config/toml.go](../../../storage/config/toml.go) |
| `go_exported` | `AllMigratedButBank WriteMode = "all_migrated_but_bank"` | [storage/config/write_mode.go](../../../storage/config/write_mode.go) |
| `go_exported` | `EVMMigrated WriteMode = "evm_migrated"` | [storage/config/write_mode.go](../../../storage/config/write_mode.go) |
| `go_exported` | `FlatKVOnly WriteMode = "flatkv_only"` | [storage/config/write_mode.go](../../../storage/config/write_mode.go) |
| `go_exported` | `MemiavlOnly WriteMode = "memiavl_only"` | [storage/config/write_mode.go](../../../storage/config/write_mode.go) |
| `go_exported` | `MigrateAllButBank WriteMode = "migrate_all_but_bank"` | [storage/config/write_mode.go](../../../storage/config/write_mode.go) |
| `go_exported` | `MigrateBank WriteMode = "migrate_bank"` | [storage/config/write_mode.go](../../../storage/config/write_mode.go) |
| `go_exported` | `MigrateEVM WriteMode = "migrate_evm"` | [storage/config/write_mode.go](../../../storage/config/write_mode.go) |
| `go_exported` | `func ParseWriteMode(s string) (WriteMode, error)` | [storage/config/write_mode.go](../../../storage/config/write_mode.go) |
| `go_exported` | `TestOnlyDualWrite WriteMode = "test_only_dual_write"` | [storage/config/write_mode.go](../../../storage/config/write_mode.go) |
| `go_exported` | `type WriteMode string` | [storage/config/write_mode.go](../../../storage/config/write_mode.go) |
| `go_exported` | `func BuildCache(` | [storage/db_engine/dbcache/cache.go](../../../storage/db_engine/dbcache/cache.go) |
| `go_exported` | `type CacheConfig struct` | [storage/db_engine/dbcache/cache_config.go](../../../storage/db_engine/dbcache/cache_config.go) |
| `go_exported` | `func DefaultCacheConfig() CacheConfig` | [storage/db_engine/dbcache/cache_config.go](../../../storage/db_engine/dbcache/cache_config.go) |
| `go_exported` | `func NewCachedKeyValueDB(db types.KeyValueDB, cache Cache) types.KeyValueDB` | [storage/db_engine/dbcache/cached_key_value_db.go](../../../storage/db_engine/dbcache/cached_key_value_db.go) |
| `go_exported` | `func Unwrap(db types.KeyValueDB) types.KeyValueDB` | [storage/db_engine/dbcache/unwrap.go](../../../storage/db_engine/dbcache/unwrap.go) |
| `go_exported` | `func NewBenchmarkEngine(configPath string) (*BenchmarkEngine, error)` | [storage/db_engine/litt/benchmark/benchmark_engine.go](../../../storage/db_engine/litt/benchmark/benchmark_engine.go) |
| `file_format` | `const CohortFileExtension = ".cohort"` | [storage/db_engine/litt/benchmark/cohort.go](../../../storage/db_engine/litt/benchmark/cohort.go) |
| `go_exported` | `type BenchmarkConfig struct` | [storage/db_engine/litt/benchmark/config/benchmark_config.go](../../../storage/db_engine/litt/benchmark/config/benchmark_config.go) |
| `go_exported` | `func DefaultBenchmarkConfig() *BenchmarkConfig` | [storage/db_engine/litt/benchmark/config/benchmark_config.go](../../../storage/db_engine/litt/benchmark/config/benchmark_config.go) |
| `go_exported` | `func LoadConfig(path string) (*BenchmarkConfig, error)` | [storage/db_engine/litt/benchmark/config/benchmark_config.go](../../../storage/db_engine/litt/benchmark/config/benchmark_config.go) |
| `go_exported` | `type DB interface` | [storage/db_engine/litt/db.go](../../../storage/db_engine/litt/db.go) |
| `go_exported` | `func NewCachedTable(` | [storage/db_engine/litt/dbcache/cached_table.go](../../../storage/db_engine/litt/dbcache/cached_table.go) |
| `file_format` | `const LowerBoundFileName = "lower-bound.txt"` | [storage/db_engine/litt/disktable/boundary_file.go](../../../storage/db_engine/litt/disktable/boundary_file.go) |
| `file_format` | `const UpperBoundFileName = "upper-bound.txt"` | [storage/db_engine/litt/disktable/boundary_file.go](../../../storage/db_engine/litt/disktable/boundary_file.go) |
| `go_exported` | `func LoadBoundaryFile(boundaryType BoundaryType, parentDirectory string) (*BoundaryFile, error) {` | [storage/db_engine/litt/disktable/boundary_file.go](../../../storage/db_engine/litt/disktable/boundary_file.go) |
| `go_exported` | `LowerBound BoundaryType = true` | [storage/db_engine/litt/disktable/boundary_file.go](../../../storage/db_engine/litt/disktable/boundary_file.go) |
| `go_exported` | `const LowerBoundFileName = "lower-bound.txt"` | [storage/db_engine/litt/disktable/boundary_file.go](../../../storage/db_engine/litt/disktable/boundary_file.go) |
| `go_exported` | `UpperBound BoundaryType = false` | [storage/db_engine/litt/disktable/boundary_file.go](../../../storage/db_engine/litt/disktable/boundary_file.go) |
| `go_exported` | `const UpperBoundFileName = "upper-bound.txt"` | [storage/db_engine/litt/disktable/boundary_file.go](../../../storage/db_engine/litt/disktable/boundary_file.go) |
| `go_exported` | `func NewDiskTable(` | [storage/db_engine/litt/disktable/disk_table.go](../../../storage/db_engine/litt/disktable/disk_table.go) |
| `file_format` | `const KeymapInitializedFileName = "initialized"` | [storage/db_engine/litt/disktable/keymap/keymap.go](../../../storage/db_engine/litt/disktable/keymap/keymap.go) |
| `go_exported` | `type BuildKeymap func(logger *slog.Logger, keymapPath string, doubleWriteProtection bool) (Keymap, bool, error)` | [storage/db_engine/litt/disktable/keymap/keymap.go](../../../storage/db_engine/litt/disktable/keymap/keymap.go) |
| `go_exported` | `type Keymap interface` | [storage/db_engine/litt/disktable/keymap/keymap.go](../../../storage/db_engine/litt/disktable/keymap/keymap.go) |
| `go_exported` | `const KeymapDataDirectoryName = "data"` | [storage/db_engine/litt/disktable/keymap/keymap.go](../../../storage/db_engine/litt/disktable/keymap/keymap.go) |
| `go_exported` | `const KeymapDirectoryName = "keymap"` | [storage/db_engine/litt/disktable/keymap/keymap.go](../../../storage/db_engine/litt/disktable/keymap/keymap.go) |
| `go_exported` | `const KeymapInitializedFileName = "initialized"` | [storage/db_engine/litt/disktable/keymap/keymap.go](../../../storage/db_engine/litt/disktable/keymap/keymap.go) |
| `go_exported` | `type KeymapType string` | [storage/db_engine/litt/disktable/keymap/keymap_type.go](../../../storage/db_engine/litt/disktable/keymap/keymap_type.go) |
| `go_exported` | `const MemKeymapType = "MemKeymap"` | [storage/db_engine/litt/disktable/keymap/keymap_type.go](../../../storage/db_engine/litt/disktable/keymap/keymap_type.go) |
| `go_exported` | `const PebbleDBKeymapType = "PebbleDBKeymap"` | [storage/db_engine/litt/disktable/keymap/keymap_type.go](../../../storage/db_engine/litt/disktable/keymap/keymap_type.go) |
| `go_exported` | `const UnsafePebbleDBKeymapType = "UnsafePebbleDBKeymap"` | [storage/db_engine/litt/disktable/keymap/keymap_type.go](../../../storage/db_engine/litt/disktable/keymap/keymap_type.go) |
| `file_format` | `const KeymapTypeFileName = "keymap-type.txt"` | [storage/db_engine/litt/disktable/keymap/keymap_type_file.go](../../../storage/db_engine/litt/disktable/keymap/keymap_type_file.go) |
| `go_exported` | `func KeymapFileExists(keymapPath string) (bool, error)` | [storage/db_engine/litt/disktable/keymap/keymap_type_file.go](../../../storage/db_engine/litt/disktable/keymap/keymap_type_file.go) |
| `go_exported` | `type KeymapTypeFile struct` | [storage/db_engine/litt/disktable/keymap/keymap_type_file.go](../../../storage/db_engine/litt/disktable/keymap/keymap_type_file.go) |
| `go_exported` | `func LoadKeymapTypeFile(keymapPath string) (*KeymapTypeFile, error)` | [storage/db_engine/litt/disktable/keymap/keymap_type_file.go](../../../storage/db_engine/litt/disktable/keymap/keymap_type_file.go) |
| `go_exported` | `func NewKeymapTypeFile(keymapPath string, keymapType KeymapType) *KeymapTypeFile` | [storage/db_engine/litt/disktable/keymap/keymap_type_file.go](../../../storage/db_engine/litt/disktable/keymap/keymap_type_file.go) |
| `go_exported` | `func NewMemKeymap(logger *slog.Logger,` | [storage/db_engine/litt/disktable/keymap/mem_keymap.go](../../../storage/db_engine/litt/disktable/keymap/mem_keymap.go) |
| `go_exported` | `func NewPebbleDBKeymap(` | [storage/db_engine/litt/disktable/keymap/pebble_db_keymap.go](../../../storage/db_engine/litt/disktable/keymap/pebble_db_keymap.go) |
| `go_exported` | `func NewUnsafePebbleDBKeymap(` | [storage/db_engine/litt/disktable/keymap/pebble_db_keymap.go](../../../storage/db_engine/litt/disktable/keymap/pebble_db_keymap.go) |
| `file_format` | `const KeyFileExtension = ".keys"` | [storage/db_engine/litt/disktable/segment/key_file.go](../../../storage/db_engine/litt/disktable/segment/key_file.go) |
| `go_exported` | `const KeyFileExtension = ".keys"` | [storage/db_engine/litt/disktable/segment/key_file.go](../../../storage/db_engine/litt/disktable/segment/key_file.go) |
| `file_format` | `MetadataFileExtension = ".metadata"` | [storage/db_engine/litt/disktable/segment/metadata_file.go](../../../storage/db_engine/litt/disktable/segment/metadata_file.go) |
| `go_exported` | `MetadataFileExtension = ".metadata"` | [storage/db_engine/litt/disktable/segment/metadata_file.go](../../../storage/db_engine/litt/disktable/segment/metadata_file.go) |
| `go_exported` | `V3MetadataSize = 18` | [storage/db_engine/litt/disktable/segment/metadata_file.go](../../../storage/db_engine/litt/disktable/segment/metadata_file.go) |
| `go_exported` | `func CreateSegment(` | [storage/db_engine/litt/disktable/segment/segment.go](../../../storage/db_engine/litt/disktable/segment/segment.go) |
| `go_exported` | `type FlushWaitFunction func() ([]*types.ScopedKey, error)` | [storage/db_engine/litt/disktable/segment/segment.go](../../../storage/db_engine/litt/disktable/segment/segment.go) |
| `go_exported` | `type Segment struct` | [storage/db_engine/litt/disktable/segment/segment.go](../../../storage/db_engine/litt/disktable/segment/segment.go) |
| `go_exported` | `func BuildSegmentPaths(` | [storage/db_engine/litt/disktable/segment/segment_path.go](../../../storage/db_engine/litt/disktable/segment/segment_path.go) |
| `go_exported` | `const HardLinkDirectory = "snapshot"` | [storage/db_engine/litt/disktable/segment/segment_path.go](../../../storage/db_engine/litt/disktable/segment/segment_path.go) |
| `go_exported` | `func NewSegmentPath(` | [storage/db_engine/litt/disktable/segment/segment_path.go](../../../storage/db_engine/litt/disktable/segment/segment_path.go) |
| `go_exported` | `const SegmentDirectory = "segments"` | [storage/db_engine/litt/disktable/segment/segment_path.go](../../../storage/db_engine/litt/disktable/segment/segment_path.go) |
| `go_exported` | `type SegmentPath struct` | [storage/db_engine/litt/disktable/segment/segment_path.go](../../../storage/db_engine/litt/disktable/segment/segment_path.go) |
| `go_exported` | `type SegmentReader struct` | [storage/db_engine/litt/disktable/segment/segment_reader.go](../../../storage/db_engine/litt/disktable/segment/segment_reader.go) |
| `go_exported` | `func GatherSegmentFiles(` | [storage/db_engine/litt/disktable/segment/segment_scanner.go](../../../storage/db_engine/litt/disktable/segment/segment_scanner.go) |
| `file_format` | `ShardedAddressSegmentVersion SegmentVersion = 3` | [storage/db_engine/litt/disktable/segment/segment_version.go](../../../storage/db_engine/litt/disktable/segment/segment_version.go) |
| `go_exported` | `const LatestSegmentVersion = ShardedAddressSegmentVersion` | [storage/db_engine/litt/disktable/segment/segment_version.go](../../../storage/db_engine/litt/disktable/segment/segment_version.go) |
| `file_format` | `const ValuesFileExtension = ".values"` | [storage/db_engine/litt/disktable/segment/value_file.go](../../../storage/db_engine/litt/disktable/segment/value_file.go) |
| `go_exported` | `const ValuesFileExtension = ".values"` | [storage/db_engine/litt/disktable/segment/value_file.go](../../../storage/db_engine/litt/disktable/segment/value_file.go) |
| `go_exported` | `func Unlock(logger *slog.Logger, sourcePaths []string) error` | [storage/db_engine/litt/disktable/unlock.go](../../../storage/db_engine/litt/disktable/unlock.go) |
| `go_exported` | `func FindKeymapLocation(` | [storage/db_engine/litt/littbuilder/build_utils.go](../../../storage/db_engine/litt/littbuilder/build_utils.go) |
| `go_exported` | `func NewDB(config *litt.Config, runtimeConfig ...*litt.RuntimeConfig) (litt.DB, error)` | [storage/db_engine/litt/littbuilder/db_impl.go](../../../storage/db_engine/litt/littbuilder/db_impl.go) |
| `go_exported` | `type Config struct` | [storage/db_engine/litt/littdb_config.go](../../../storage/db_engine/litt/littdb_config.go) |
| `go_exported` | `func DefaultConfig(paths ...string) (*Config, error)` | [storage/db_engine/litt/littdb_config.go](../../../storage/db_engine/litt/littdb_config.go) |
| `go_exported` | `func DefaultConfigNoPaths() *Config` | [storage/db_engine/litt/littdb_config.go](../../../storage/db_engine/litt/littdb_config.go) |
| `go_exported` | `const MaxShardingFactor = 255` | [storage/db_engine/litt/littdb_config.go](../../../storage/db_engine/litt/littdb_config.go) |
| `go_exported` | `type LittDBMetrics struct` | [storage/db_engine/litt/metrics/littdb_metrics.go](../../../storage/db_engine/litt/metrics/littdb_metrics.go) |
| `go_exported` | `func NewLittDBMetrics() *LittDBMetrics` | [storage/db_engine/litt/metrics/littdb_metrics.go](../../../storage/db_engine/litt/metrics/littdb_metrics.go) |
| `go_exported` | `func DefaultRuntimeConfig() *RuntimeConfig` | [storage/db_engine/litt/runtime_config.go](../../../storage/db_engine/litt/runtime_config.go) |
| `go_exported` | `type RuntimeConfig struct` | [storage/db_engine/litt/runtime_config.go](../../../storage/db_engine/litt/runtime_config.go) |
| `go_exported` | `func IsTableNameValid(name string) bool` | [storage/db_engine/litt/table.go](../../../storage/db_engine/litt/table.go) |
| `go_exported` | `type Iterator interface` | [storage/db_engine/litt/table.go](../../../storage/db_engine/litt/table.go) |
| `go_exported` | `type ManagedTable interface` | [storage/db_engine/litt/table.go](../../../storage/db_engine/litt/table.go) |
| `go_exported` | `type Table interface` | [storage/db_engine/litt/table.go](../../../storage/db_engine/litt/table.go) |
| `go_exported` | `func DefaultTableConfig(name string) TableConfig` | [storage/db_engine/litt/table_config.go](../../../storage/db_engine/litt/table_config.go) |
| `go_exported` | `type GCFilter func(key []byte, isPrimaryKey bool) (bool, error)` | [storage/db_engine/litt/table_config.go](../../../storage/db_engine/litt/table_config.go) |
| `go_exported` | `type TableConfig struct` | [storage/db_engine/litt/table_config.go](../../../storage/db_engine/litt/table_config.go) |
| `go_exported` | `type Address struct` | [storage/db_engine/litt/types/address.go](../../../storage/db_engine/litt/types/address.go) |
| `go_exported` | `const AddressSerializedSize = 13` | [storage/db_engine/litt/types/address.go](../../../storage/db_engine/litt/types/address.go) |
| `go_exported` | `func DeserializeAddress(bytes []byte) (Address, error)` | [storage/db_engine/litt/types/address.go](../../../storage/db_engine/litt/types/address.go) |
| `go_exported` | `func NewAddress(index uint32, offset uint32, shardID uint8, valueSize uint32) Address` | [storage/db_engine/litt/types/address.go](../../../storage/db_engine/litt/types/address.go) |
| `go_exported` | `type KeyKind uint8` | [storage/db_engine/litt/types/key_kind.go](../../../storage/db_engine/litt/types/key_kind.go) |
| `go_exported` | `KeyKindFinalSecondary KeyKind = 3` | [storage/db_engine/litt/types/key_kind.go](../../../storage/db_engine/litt/types/key_kind.go) |
| `go_exported` | `KeyKindPrimary KeyKind = 1` | [storage/db_engine/litt/types/key_kind.go](../../../storage/db_engine/litt/types/key_kind.go) |
| `go_exported` | `KeyKindSecondary KeyKind = 2` | [storage/db_engine/litt/types/key_kind.go](../../../storage/db_engine/litt/types/key_kind.go) |
| `go_exported` | `KeyKindStandalone KeyKind = 0` | [storage/db_engine/litt/types/key_kind.go](../../../storage/db_engine/litt/types/key_kind.go) |
| `go_exported` | `type PutRequest struct` | [storage/db_engine/litt/types/put_request.go](../../../storage/db_engine/litt/types/put_request.go) |
| `go_exported` | `type ScopedKey struct` | [storage/db_engine/litt/types/scoped_key.go](../../../storage/db_engine/litt/types/scoped_key.go) |
| `go_exported` | `type SecondaryKey struct` | [storage/db_engine/litt/types/secondary_key.go](../../../storage/db_engine/litt/types/secondary_key.go) |
| `go_exported` | `type Cache[K comparable, V any] interface` | [storage/db_engine/litt/util/cache.go](../../../storage/db_engine/litt/util/cache.go) |
| `go_exported` | `type CacheMetrics struct` | [storage/db_engine/litt/util/cache_metrics.go](../../../storage/db_engine/litt/util/cache_metrics.go) |
| `go_exported` | `func NewCacheMetrics(cacheName string) *CacheMetrics` | [storage/db_engine/litt/util/cache_metrics.go](../../../storage/db_engine/litt/util/cache_metrics.go) |
| `go_exported` | `const LockfileName = "litt.lock"` | [storage/db_engine/litt/util/constants.go](../../../storage/db_engine/litt/util/constants.go) |
| `go_exported` | `func CloseLogOnError(c io.Closer, name string, log *slog.Logger)` | [storage/db_engine/litt/util/core_utils.go](../../../storage/db_engine/litt/util/core_utils.go) |
| `go_exported` | `func MapDoesNotContainKey[K comparable, V any](m map[K]V, key K, message string, args ...any) {` | [storage/db_engine/litt/util/enforce.go](../../../storage/db_engine/litt/util/enforce.go) |
| `go_exported` | `func True(condition bool, message string, args ...any)` | [storage/db_engine/litt/util/enforce.go](../../../storage/db_engine/litt/util/enforce.go) |
| `go_exported` | `func Await[T any](handler *ErrorMonitor, channel <-chan T) (T, error)` | [storage/db_engine/litt/util/error_monitor.go](../../../storage/db_engine/litt/util/error_monitor.go) |
| `go_exported` | `type ErrorMonitor struct` | [storage/db_engine/litt/util/error_monitor.go](../../../storage/db_engine/litt/util/error_monitor.go) |
| `go_exported` | `func NewErrorMonitor(` | [storage/db_engine/litt/util/error_monitor.go](../../../storage/db_engine/litt/util/error_monitor.go) |
| `go_exported` | `func Send[T any](handler *ErrorMonitor, channel chan<- any, value T) error` | [storage/db_engine/litt/util/error_monitor.go](../../../storage/db_engine/litt/util/error_monitor.go) |
| `go_exported` | `func NewFIFOCache[K comparable, V any](` | [storage/db_engine/litt/util/fifo_cache.go](../../../storage/db_engine/litt/util/fifo_cache.go) |
| `go_exported` | `func IsProcessAlive(pid int) bool` | [storage/db_engine/litt/util/file_lock.go](../../../storage/db_engine/litt/util/file_lock.go) |
| `go_exported` | `func LockDirectories(` | [storage/db_engine/litt/util/file_lock.go](../../../storage/db_engine/litt/util/file_lock.go) |
| `go_exported` | `func NewFileLock(logger *slog.Logger, path string, fsync bool) (*FileLock, error)` | [storage/db_engine/litt/util/file_lock.go](../../../storage/db_engine/litt/util/file_lock.go) |
| `file_format` | `const SwapFileExtension = ".swap"` | [storage/db_engine/litt/util/file_utils.go](../../../storage/db_engine/litt/util/file_utils.go) |
| `go_exported` | `func AtomicRename(oldPath string, newPath string, fsync bool) error` | [storage/db_engine/litt/util/file_utils.go](../../../storage/db_engine/litt/util/file_utils.go) |
| `go_exported` | `func AtomicWrite(destination string, data []byte, fsync bool) error` | [storage/db_engine/litt/util/file_utils.go](../../../storage/db_engine/litt/util/file_utils.go) |
| `go_exported` | `func DeepDelete(path string, backingRoots ...string) error {` | [storage/db_engine/litt/util/file_utils.go](../../../storage/db_engine/litt/util/file_utils.go) |
| `go_exported` | `func DeleteOrphanedSwapFiles(directory string) error` | [storage/db_engine/litt/util/file_utils.go](../../../storage/db_engine/litt/util/file_utils.go) |
| `go_exported` | `func EnsureDirectoryExists(dirPath string, fsync bool) error` | [storage/db_engine/litt/util/file_utils.go](../../../storage/db_engine/litt/util/file_utils.go) |
| `go_exported` | `func ErrIfNotExists(path string) error` | [storage/db_engine/litt/util/file_utils.go](../../../storage/db_engine/litt/util/file_utils.go) |
| `go_exported` | `func ErrIfNotWritableFile(path string) (exists bool, size int64, err error)` | [storage/db_engine/litt/util/file_utils.go](../../../storage/db_engine/litt/util/file_utils.go) |
| `go_exported` | `func Exists(path string) (bool, error)` | [storage/db_engine/litt/util/file_utils.go](../../../storage/db_engine/litt/util/file_utils.go) |
| `go_exported` | `func IsDirectory(path string) (bool, error)` | [storage/db_engine/litt/util/file_utils.go](../../../storage/db_engine/litt/util/file_utils.go) |
| `go_exported` | `func SanitizePath(path string) (string, error)` | [storage/db_engine/litt/util/file_utils.go](../../../storage/db_engine/litt/util/file_utils.go) |
| `go_exported` | `const SwapFileExtension = ".swap"` | [storage/db_engine/litt/util/file_utils.go](../../../storage/db_engine/litt/util/file_utils.go) |
| `go_exported` | `func NewPprofProfiler(httpPort string, logger *slog.Logger) *PprofProfiler` | [storage/db_engine/litt/util/pprof.go](../../../storage/db_engine/litt/util/pprof.go) |
| `go_exported` | `func NewQueue[T any](initialCapacity uint64) *Queue[T]` | [storage/db_engine/litt/util/queue.go](../../../storage/db_engine/litt/util/queue.go) |
| `go_exported` | `func RecursiveMove(` | [storage/db_engine/litt/util/recursive_move.go](../../../storage/db_engine/litt/util/recursive_move.go) |
| `go_exported` | `func NewSSHSession(` | [storage/db_engine/litt/util/ssh.go](../../../storage/db_engine/litt/util/ssh.go) |
| `go_exported` | `type SSHSession struct` | [storage/db_engine/litt/util/ssh.go](../../../storage/db_engine/litt/util/ssh.go) |
| `go_exported` | `func AssertEventuallyTrue(t *testing.T, condition func() bool, duration time.Duration, debugInfo ...any) {` | [storage/db_engine/litt/util/test_assertions.go](../../../storage/db_engine/litt/util/test_assertions.go) |
| `go_exported` | `func NewTestRandom(fixedSeed ...int64) *TestRandom` | [storage/db_engine/litt/util/test_random.go](../../../storage/db_engine/litt/util/test_random.go) |
| `go_exported` | `type TestRandom struct` | [storage/db_engine/litt/util/test_random.go](../../../storage/db_engine/litt/util/test_random.go) |
| `go_exported` | `func NewThreadSafeCache[K comparable, V any](cache Cache[K, V]) Cache[K, V]` | [storage/db_engine/litt/util/thread_safe_cache.go](../../../storage/db_engine/litt/util/thread_safe_cache.go) |
| `go_exported` | `func CommaOMatic(value uint64) string` | [storage/db_engine/litt/util/units.go](../../../storage/db_engine/litt/util/units.go) |
| `go_exported` | `func PrettyPrintBytes(bytes uint64) string` | [storage/db_engine/litt/util/units.go](../../../storage/db_engine/litt/util/units.go) |
| `go_exported` | `func PrettyPrintTime(nanoseconds uint64) string` | [storage/db_engine/litt/util/units.go](../../../storage/db_engine/litt/util/units.go) |
| `go_exported` | `func UnsafeBytesToString(b []byte) string` | [storage/db_engine/litt/util/unsafe_string.go](../../../storage/db_engine/litt/util/unsafe_string.go) |
| `go_exported` | `func Open(` | [storage/db_engine/pebbledb/db.go](../../../storage/db_engine/pebbledb/db.go) |
| `go_exported` | `func OpenWithCache(` | [storage/db_engine/pebbledb/db.go](../../../storage/db_engine/pebbledb/db.go) |
| `go_exported` | `type Database struct` | [storage/db_engine/pebbledb/mvcc/db.go](../../../storage/db_engine/pebbledb/mvcc/db.go) |
| `go_exported` | `func OpenDB(dataDir string, config config.StateStoreConfig) (types.StateStore, error)` | [storage/db_engine/pebbledb/mvcc/db.go](../../../storage/db_engine/pebbledb/mvcc/db.go) |
| `storage_key` | `descendingMVCCMarkerKey = "s/_mvcc_descending"` | [storage/db_engine/pebbledb/mvcc/db.go](../../../storage/db_engine/pebbledb/mvcc/db.go) |
| `storage_key` | `earliestVersionKey = "s/_earliest"` | [storage/db_engine/pebbledb/mvcc/db.go](../../../storage/db_engine/pebbledb/mvcc/db.go) |
| `storage_key` | `latestVersionKey = "s/_latest"` | [storage/db_engine/pebbledb/mvcc/db.go](../../../storage/db_engine/pebbledb/mvcc/db.go) |
| `storage_key` | `StorePrefixTpl = "s/k:%s/" // s/k:<storeKey>` | [storage/db_engine/pebbledb/mvcc/db.go](../../../storage/db_engine/pebbledb/mvcc/db.go) |
| `storage_key` | `tombstoneVal = "TOMBSTONE"` | [storage/db_engine/pebbledb/mvcc/db.go](../../../storage/db_engine/pebbledb/mvcc/db.go) |
| `go_exported` | `func DefaultConfig() PebbleDBConfig` | [storage/db_engine/pebbledb/pebbledb_config.go](../../../storage/db_engine/pebbledb/pebbledb_config.go) |
| `go_exported` | `type PebbleDBConfig struct` | [storage/db_engine/pebbledb/pebbledb_config.go](../../../storage/db_engine/pebbledb/pebbledb_config.go) |
| `go_exported` | `func DefaultTestCacheConfig() dbcache.CacheConfig` | [storage/db_engine/pebbledb/pebbledb_test_config.go](../../../storage/db_engine/pebbledb/pebbledb_test_config.go) |
| `go_exported` | `func DefaultTestConfig(t *testing.T) PebbleDBConfig` | [storage/db_engine/pebbledb/pebbledb_test_config.go](../../../storage/db_engine/pebbledb/pebbledb_test_config.go) |
| `go_exported` | `func TableIters(db types.KeyValueDB) (int64, error)` | [storage/db_engine/pebbledb/table_iters.go](../../../storage/db_engine/pebbledb/table_iters.go) |
| `go_exported` | `func OpenDB(dataDir string, config config.StateStoreConfig) (*Database, error)` | [storage/db_engine/rocksdb/mvcc/db.go](../../../storage/db_engine/rocksdb/mvcc/db.go) |
| `storage_key` | `earliestVersionKey = "s/earliest"` | [storage/db_engine/rocksdb/mvcc/db.go](../../../storage/db_engine/rocksdb/mvcc/db.go) |
| `storage_key` | `latestVersionKey = "s/latest"` | [storage/db_engine/rocksdb/mvcc/db.go](../../../storage/db_engine/rocksdb/mvcc/db.go) |
| `storage_key` | `StorePrefixTpl = "s/k:%s/"` | [storage/db_engine/rocksdb/mvcc/db.go](../../../storage/db_engine/rocksdb/mvcc/db.go) |
| `go_exported` | `type StorageBenchSuite struct` | [storage/db_engine/test/storage_bench_suite.go](../../../storage/db_engine/test/storage_bench_suite.go) |
| `go_exported` | `type BaseStorageTestSuite struct` | [storage/db_engine/test/storage_test_suite.go](../../../storage/db_engine/test/storage_test_suite.go) |
| `go_exported` | `type StorageTestSuite struct` | [storage/db_engine/test/storage_test_suite.go](../../../storage/db_engine/test/storage_test_suite.go) |
| `go_exported` | `type Batch interface` | [storage/db_engine/types/types.go](../../../storage/db_engine/types/types.go) |
| `go_exported` | `type BatchGetResult struct` | [storage/db_engine/types/types.go](../../../storage/db_engine/types/types.go) |
| `go_exported` | `type Checkpointable interface` | [storage/db_engine/types/types.go](../../../storage/db_engine/types/types.go) |
| `go_exported` | `type IterOptions struct` | [storage/db_engine/types/types.go](../../../storage/db_engine/types/types.go) |
| `go_exported` | `type KeyValueDB interface` | [storage/db_engine/types/types.go](../../../storage/db_engine/types/types.go) |
| `go_exported` | `type SnapshotNode struct` | [storage/db_engine/types/types.go](../../../storage/db_engine/types/types.go) |
| `go_exported` | `type StateStore interface` | [storage/db_engine/types/types.go](../../../storage/db_engine/types/types.go) |
| `go_exported` | `type WriteOptions struct` | [storage/db_engine/types/types.go](../../../storage/db_engine/types/types.go) |
| `go_exported` | `func IterateWithPrefix(prefix, begin, end []byte) ([]byte, []byte)` | [storage/db_engine/util/iterator.go](../../../storage/db_engine/util/iterator.go) |
| `go_exported` | `type BinaryBlock struct` | [storage/ledger_db/block/block_db.go](../../../storage/ledger_db/block/block_db.go) |
| `go_exported` | `type BinaryTransaction struct` | [storage/ledger_db/block/block_db.go](../../../storage/ledger_db/block/block_db.go) |
| `go_exported` | `type BlockDB interface` | [storage/ledger_db/block/block_db.go](../../../storage/ledger_db/block/block_db.go) |
| `go_exported` | `var ErrNoBlocks = errors.New("block db: no blocks")` | [storage/ledger_db/block/block_db.go](../../../storage/ledger_db/block/block_db.go) |
| `go_exported` | `func NewBlockSim(` | [storage/ledger_db/block/blocksim/blocksim.go](../../../storage/ledger_db/block/blocksim/blocksim.go) |
| `go_exported` | `func DefaultBlocksimConfig() *BlocksimConfig` | [storage/ledger_db/block/blocksim/blocksim_config.go](../../../storage/ledger_db/block/blocksim/blocksim_config.go) |
| `go_exported` | `func NewBlocksimMetrics(ctx context.Context, config *BlocksimConfig) *BlocksimMetrics` | [storage/ledger_db/block/blocksim/blocksim_metrics.go](../../../storage/ledger_db/block/blocksim/blocksim_metrics.go) |
| `go_exported` | `func NewMemBlockDB() block.BlockDB` | [storage/ledger_db/block/mem_block_db/mem_block_db.go](../../../storage/ledger_db/block/mem_block_db/mem_block_db.go) |
| `go_exported` | `type ChangelogEntry struct` | [storage/proto/changelog.pb.go](../../../storage/proto/changelog.pb.go) |
| `go_exported` | `type MultiTreeMetadata struct` | [storage/proto/changelog.pb.go](../../../storage/proto/changelog.pb.go) |
| `go_exported` | `type NamedChangeSet struct` | [storage/proto/changelog.pb.go](../../../storage/proto/changelog.pb.go) |
| `go_exported` | `type TreeNameUpgrade struct` | [storage/proto/changelog.pb.go](../../../storage/proto/changelog.pb.go) |
| `go_exported` | `type ChangeSet struct` | [storage/proto/changeset.pb.go](../../../storage/proto/changeset.pb.go) |
| `go_exported` | `type KVPair struct` | [storage/proto/changeset.pb.go](../../../storage/proto/changeset.pb.go) |
| `go_exported` | `type CommitID struct` | [storage/proto/commit_info.pb.go](../../../storage/proto/commit_info.pb.go) |
| `go_exported` | `type CommitInfo struct` | [storage/proto/commit_info.pb.go](../../../storage/proto/commit_info.pb.go) |
| `go_exported` | `type StoreInfo struct` | [storage/proto/commit_info.pb.go](../../../storage/proto/commit_info.pb.go) |
| `go_exported` | `func NewCryptoSim(` | [storage/state_db/bench/cryptosim/cryptosim.go](../../../storage/state_db/bench/cryptosim/cryptosim.go) |
| `go_exported` | `func LoadConfigFromFile(path string) (*CryptoSimConfig, error)` | [storage/state_db/bench/cryptosim/cryptosim_config.go](../../../storage/state_db/bench/cryptosim/cryptosim_config.go) |
| `go_exported` | `CompositeCosmos DBType = "CompositeCosmos"` | [storage/state_db/bench/wrappers/db_implementations.go](../../../storage/state_db/bench/wrappers/db_implementations.go) |
| `go_exported` | `CompositeDual DBType = "CompositeDual"` | [storage/state_db/bench/wrappers/db_implementations.go](../../../storage/state_db/bench/wrappers/db_implementations.go) |
| `go_exported` | `CompositeDual_SSComposite DBType = "CompositeDual+SSComposite"` | [storage/state_db/bench/wrappers/db_implementations.go](../../../storage/state_db/bench/wrappers/db_implementations.go) |
| `go_exported` | `CompositeSplit DBType = "CompositeSplit"` | [storage/state_db/bench/wrappers/db_implementations.go](../../../storage/state_db/bench/wrappers/db_implementations.go) |
| `go_exported` | `type DBType string` | [storage/state_db/bench/wrappers/db_implementations.go](../../../storage/state_db/bench/wrappers/db_implementations.go) |
| `go_exported` | `func DefaultBenchStateStoreConfig() *config.StateStoreConfig` | [storage/state_db/bench/wrappers/db_implementations.go](../../../storage/state_db/bench/wrappers/db_implementations.go) |
| `go_exported` | `const EVMStoreName = commonevm.EVMStoreKey` | [storage/state_db/bench/wrappers/db_implementations.go](../../../storage/state_db/bench/wrappers/db_implementations.go) |
| `go_exported` | `FlatKV DBType = "FlatKV"` | [storage/state_db/bench/wrappers/db_implementations.go](../../../storage/state_db/bench/wrappers/db_implementations.go) |
| `go_exported` | `MemIAVL DBType = "MemIAVL"` | [storage/state_db/bench/wrappers/db_implementations.go](../../../storage/state_db/bench/wrappers/db_implementations.go) |
| `go_exported` | `func NewDBImpl(ctx context.Context, dbType DBType, dataDir string, dbConfig any) (DBWrapper, error) {` | [storage/state_db/bench/wrappers/db_implementations.go](../../../storage/state_db/bench/wrappers/db_implementations.go) |
| `go_exported` | `NoOp DBType = "NoOp"` | [storage/state_db/bench/wrappers/db_implementations.go](../../../storage/state_db/bench/wrappers/db_implementations.go) |
| `go_exported` | `SSComposite DBType = "SSComposite"` | [storage/state_db/bench/wrappers/db_implementations.go](../../../storage/state_db/bench/wrappers/db_implementations.go) |
| `go_exported` | `SSHistoricalOffload DBType = "SSHistoricalOffload"` | [storage/state_db/bench/wrappers/db_implementations.go](../../../storage/state_db/bench/wrappers/db_implementations.go) |
| `go_exported` | `type DBWrapper interface` | [storage/state_db/bench/wrappers/db_wrapper.go](../../../storage/state_db/bench/wrappers/db_wrapper.go) |
| `go_exported` | `type HistoricalOffloadConfig struct` | [storage/state_db/bench/wrappers/historical_offload_wrapper.go](../../../storage/state_db/bench/wrappers/historical_offload_wrapper.go) |
| `go_exported` | `type KafkaHistoricalOffloadConfig struct` | [storage/state_db/bench/wrappers/historical_offload_wrapper.go](../../../storage/state_db/bench/wrappers/historical_offload_wrapper.go) |
| `go_exported` | `type CompositeCommitStore struct` | [storage/state_db/sc/composite/store.go](../../../storage/state_db/sc/composite/store.go) |
| `go_exported` | `func NewCompositeCommitStore(` | [storage/state_db/sc/composite/store.go](../../../storage/state_db/sc/composite/store.go) |
| `go_exported` | `type Store interface` | [storage/state_db/sc/flatkv/api.go](../../../storage/state_db/sc/flatkv/api.go) |
| `go_exported` | `type Config struct` | [storage/state_db/sc/flatkv/config/config.go](../../../storage/state_db/sc/flatkv/config/config.go) |
| `go_exported` | `func DefaultConfig() *Config` | [storage/state_db/sc/flatkv/config/config.go](../../../storage/state_db/sc/flatkv/config/config.go) |
| `go_exported` | `func DefaultTestConfig(t *testing.T) *Config` | [storage/state_db/sc/flatkv/config/flatkv_test_config.go](../../../storage/state_db/sc/flatkv/config/flatkv_test_config.go) |
| `go_exported` | `func NewImportTranslator(blockHeight int64) *ImportTranslator` | [storage/state_db/sc/flatkv/import_translator.go](../../../storage/state_db/sc/flatkv/import_translator.go) |
| `go_exported` | `type PhysicalKVPair struct` | [storage/state_db/sc/flatkv/import_translator.go](../../../storage/state_db/sc/flatkv/import_translator.go) |
| `go_exported` | `type KVImporter struct` | [storage/state_db/sc/flatkv/importer.go](../../../storage/state_db/sc/flatkv/importer.go) |
| `go_exported` | `type Address [AddressLen]byte` | [storage/state_db/sc/flatkv/ktype/ktype.go](../../../storage/state_db/sc/flatkv/ktype/ktype.go) |
| `go_exported` | `AddressLen = 20` | [storage/state_db/sc/flatkv/ktype/ktype.go](../../../storage/state_db/sc/flatkv/ktype/ktype.go) |
| `go_exported` | `const EVMKeyAccount = keys.EVMKeyNonce` | [storage/state_db/sc/flatkv/ktype/ktype.go](../../../storage/state_db/sc/flatkv/ktype/ktype.go) |
| `go_exported` | `func EVMPhysicalKey(kind keys.EVMKeyKind, strippedKey []byte) []byte` | [storage/state_db/sc/flatkv/ktype/ktype.go](../../../storage/state_db/sc/flatkv/ktype/ktype.go) |
| `go_exported` | `func ModulePhysicalKey(moduleName string, key []byte) []byte` | [storage/state_db/sc/flatkv/ktype/ktype.go](../../../storage/state_db/sc/flatkv/ktype/ktype.go) |
| `go_exported` | `func PrefixEnd(prefix []byte) []byte` | [storage/state_db/sc/flatkv/ktype/ktype.go](../../../storage/state_db/sc/flatkv/ktype/ktype.go) |
| `go_exported` | `type Slot [SlotLen]byte` | [storage/state_db/sc/flatkv/ktype/ktype.go](../../../storage/state_db/sc/flatkv/ktype/ktype.go) |
| `go_exported` | `SlotLen = 32` | [storage/state_db/sc/flatkv/ktype/ktype.go](../../../storage/state_db/sc/flatkv/ktype/ktype.go) |
| `go_exported` | `func StorageKey(addr Address, slot Slot) []byte` | [storage/state_db/sc/flatkv/ktype/ktype.go](../../../storage/state_db/sc/flatkv/ktype/ktype.go) |
| `go_exported` | `func StripEVMPhysicalKey(physicalKey []byte) (kind keys.EVMKeyKind, strippedKey []byte, err error) {` | [storage/state_db/sc/flatkv/ktype/ktype.go](../../../storage/state_db/sc/flatkv/ktype/ktype.go) |
| `go_exported` | `func StripModulePrefix(physicalKey []byte) (moduleName string, originalKey []byte, err error) {` | [storage/state_db/sc/flatkv/ktype/ktype.go](../../../storage/state_db/sc/flatkv/ktype/ktype.go) |
| `go_exported` | `func IsMetaKey(key []byte) bool` | [storage/state_db/sc/flatkv/ktype/meta.go](../../../storage/state_db/sc/flatkv/ktype/meta.go) |
| `go_exported` | `type LocalMeta struct` | [storage/state_db/sc/flatkv/ktype/meta.go](../../../storage/state_db/sc/flatkv/ktype/meta.go) |
| `go_exported` | `MetaLtHashKey = []byte(metaLtHash)` | [storage/state_db/sc/flatkv/ktype/meta.go](../../../storage/state_db/sc/flatkv/ktype/meta.go) |
| `go_exported` | `MetaVersionKey = []byte(metaVersion)` | [storage/state_db/sc/flatkv/ktype/meta.go](../../../storage/state_db/sc/flatkv/ktype/meta.go) |
| `storage_key` | `const metaKeyPrefix = "_meta/"` | [storage/state_db/sc/flatkv/ktype/meta.go](../../../storage/state_db/sc/flatkv/ktype/meta.go) |
| `go_exported` | `func ComputeLtHash(prev *LtHash, kvPairs []KVPairWithLastValue) (*LtHash, *LtHashTimings)` | [storage/state_db/sc/flatkv/lthash/api.go](../../../storage/state_db/sc/flatkv/lthash/api.go) |
| `go_exported` | `type KVPairWithLastValue struct` | [storage/state_db/sc/flatkv/lthash/api.go](../../../storage/state_db/sc/flatkv/lthash/api.go) |
| `go_exported` | `type LtHash struct` | [storage/state_db/sc/flatkv/lthash/lthash.go](../../../storage/state_db/sc/flatkv/lthash/lthash.go) |
| `go_exported` | `func New() *LtHash` | [storage/state_db/sc/flatkv/lthash/lthash.go](../../../storage/state_db/sc/flatkv/lthash/lthash.go) |
| `go_exported` | `func Unmarshal(data []byte) (*LtHash, error)` | [storage/state_db/sc/flatkv/lthash/lthash.go](../../../storage/state_db/sc/flatkv/lthash/lthash.go) |
| `storage_key` | `snapshotPrefix = "snapshot-"` | [storage/state_db/sc/flatkv/snapshot.go](../../../storage/state_db/sc/flatkv/snapshot.go) |
| `go_exported` | `type CommitStore struct` | [storage/state_db/sc/flatkv/store.go](../../../storage/state_db/sc/flatkv/store.go) |
| `go_exported` | `func NewCommitStore(` | [storage/state_db/sc/flatkv/store.go](../../../storage/state_db/sc/flatkv/store.go) |
| `storage_key` | `accountDBDir = "account"` | [storage/state_db/sc/flatkv/store.go](../../../storage/state_db/sc/flatkv/store.go) |
| `storage_key` | `changelogDir = "changelog"` | [storage/state_db/sc/flatkv/store.go](../../../storage/state_db/sc/flatkv/store.go) |
| `storage_key` | `codeDBDir = "code"` | [storage/state_db/sc/flatkv/store.go](../../../storage/state_db/sc/flatkv/store.go) |
| `storage_key` | `flatkvRootDir = "flatkv"` | [storage/state_db/sc/flatkv/store.go](../../../storage/state_db/sc/flatkv/store.go) |
| `storage_key` | `legacyDBDir = "legacy"` | [storage/state_db/sc/flatkv/store.go](../../../storage/state_db/sc/flatkv/store.go) |
| `storage_key` | `lockFileName = "LOCK"` | [storage/state_db/sc/flatkv/store.go](../../../storage/state_db/sc/flatkv/store.go) |
| `storage_key` | `metadataDir = "metadata"` | [storage/state_db/sc/flatkv/store.go](../../../storage/state_db/sc/flatkv/store.go) |
| `storage_key` | `storageDBDir = "storage"` | [storage/state_db/sc/flatkv/store.go](../../../storage/state_db/sc/flatkv/store.go) |
| `go_exported` | `func VerifyLtHash(s Store) error` | [storage/state_db/sc/flatkv/verify.go](../../../storage/state_db/sc/flatkv/verify.go) |
| `go_exported` | `type AccountData struct` | [storage/state_db/sc/flatkv/vtype/account_data.go](../../../storage/state_db/sc/flatkv/vtype/account_data.go) |
| `go_exported` | `func DeserializeAccountData(data []byte) (*AccountData, error)` | [storage/state_db/sc/flatkv/vtype/account_data.go](../../../storage/state_db/sc/flatkv/vtype/account_data.go) |
| `go_exported` | `func NewAccountData() *AccountData` | [storage/state_db/sc/flatkv/vtype/account_data.go](../../../storage/state_db/sc/flatkv/vtype/account_data.go) |
| `go_exported` | `type Balance [BalanceLen]byte` | [storage/state_db/sc/flatkv/vtype/base_types.go](../../../storage/state_db/sc/flatkv/vtype/base_types.go) |
| `go_exported` | `type CodeHash [CodeHashLen]byte` | [storage/state_db/sc/flatkv/vtype/base_types.go](../../../storage/state_db/sc/flatkv/vtype/base_types.go) |
| `go_exported` | `CodeHashLen = 32` | [storage/state_db/sc/flatkv/vtype/base_types.go](../../../storage/state_db/sc/flatkv/vtype/base_types.go) |
| `go_exported` | `NonceLen = 8` | [storage/state_db/sc/flatkv/vtype/base_types.go](../../../storage/state_db/sc/flatkv/vtype/base_types.go) |
| `go_exported` | `func ParseBalance(b []byte) (*Balance, error)` | [storage/state_db/sc/flatkv/vtype/base_types.go](../../../storage/state_db/sc/flatkv/vtype/base_types.go) |
| `go_exported` | `func ParseCodeHash(b []byte) (*CodeHash, error)` | [storage/state_db/sc/flatkv/vtype/base_types.go](../../../storage/state_db/sc/flatkv/vtype/base_types.go) |
| `go_exported` | `func ParseNonce(b []byte) (uint64, error)` | [storage/state_db/sc/flatkv/vtype/base_types.go](../../../storage/state_db/sc/flatkv/vtype/base_types.go) |
| `go_exported` | `func ParseStorageValue(b []byte) (*[32]byte, error)` | [storage/state_db/sc/flatkv/vtype/base_types.go](../../../storage/state_db/sc/flatkv/vtype/base_types.go) |
| `go_exported` | `SlotLen = 32` | [storage/state_db/sc/flatkv/vtype/base_types.go](../../../storage/state_db/sc/flatkv/vtype/base_types.go) |
| `go_exported` | `type CodeData struct` | [storage/state_db/sc/flatkv/vtype/code_data.go](../../../storage/state_db/sc/flatkv/vtype/code_data.go) |
| `go_exported` | `func DeserializeCodeData(data []byte) (*CodeData, error)` | [storage/state_db/sc/flatkv/vtype/code_data.go](../../../storage/state_db/sc/flatkv/vtype/code_data.go) |
| `go_exported` | `func NewCodeData() *CodeData` | [storage/state_db/sc/flatkv/vtype/code_data.go](../../../storage/state_db/sc/flatkv/vtype/code_data.go) |
| `go_exported` | `BalanceLength = 32` | [storage/state_db/sc/flatkv/vtype/constants.go](../../../storage/state_db/sc/flatkv/vtype/constants.go) |
| `go_exported` | `BlockHeightLength = 8` | [storage/state_db/sc/flatkv/vtype/constants.go](../../../storage/state_db/sc/flatkv/vtype/constants.go) |
| `go_exported` | `CodeHashLength = 32` | [storage/state_db/sc/flatkv/vtype/constants.go](../../../storage/state_db/sc/flatkv/vtype/constants.go) |
| `go_exported` | `NonceLength = 8` | [storage/state_db/sc/flatkv/vtype/constants.go](../../../storage/state_db/sc/flatkv/vtype/constants.go) |
| `go_exported` | `VersionLength = 1` | [storage/state_db/sc/flatkv/vtype/constants.go](../../../storage/state_db/sc/flatkv/vtype/constants.go) |
| `go_exported` | `func DeserializeLegacyData(data []byte) (*LegacyData, error)` | [storage/state_db/sc/flatkv/vtype/legacy_data.go](../../../storage/state_db/sc/flatkv/vtype/legacy_data.go) |
| `go_exported` | `type LegacyData struct` | [storage/state_db/sc/flatkv/vtype/legacy_data.go](../../../storage/state_db/sc/flatkv/vtype/legacy_data.go) |
| `go_exported` | `func NewLegacyData() *LegacyData` | [storage/state_db/sc/flatkv/vtype/legacy_data.go](../../../storage/state_db/sc/flatkv/vtype/legacy_data.go) |
| `go_exported` | `type PendingAccountWrite struct` | [storage/state_db/sc/flatkv/vtype/pending_account_write.go](../../../storage/state_db/sc/flatkv/vtype/pending_account_write.go) |
| `go_exported` | `func DeserializeStorageData(data []byte) (*StorageData, error)` | [storage/state_db/sc/flatkv/vtype/storage_data.go](../../../storage/state_db/sc/flatkv/vtype/storage_data.go) |
| `go_exported` | `func NewStorageData() *StorageData` | [storage/state_db/sc/flatkv/vtype/storage_data.go](../../../storage/state_db/sc/flatkv/vtype/storage_data.go) |
| `go_exported` | `type StorageData struct` | [storage/state_db/sc/flatkv/vtype/storage_data.go](../../../storage/state_db/sc/flatkv/vtype/storage_data.go) |
| `go_exported` | `type VType interface` | [storage/state_db/sc/flatkv/vtype/vtype.go](../../../storage/state_db/sc/flatkv/vtype/vtype.go) |
| `storage_key` | `hashKeyPrefix = []byte("h")` | [storage/state_db/sc/hashvault/pebble_hashvault_codec.go](../../../storage/state_db/sc/hashvault/pebble_hashvault_codec.go) |
| `storage_key` | `pruneBoundaryKey = []byte("prune_boundary")` | [storage/state_db/sc/hashvault/pebble_hashvault_codec.go](../../../storage/state_db/sc/hashvault/pebble_hashvault_codec.go) |
| `go_exported` | `type Config struct` | [storage/state_db/sc/memiavl/config.go](../../../storage/state_db/sc/memiavl/config.go) |
| `go_exported` | `func DefaultConfig() Config` | [storage/state_db/sc/memiavl/config.go](../../../storage/state_db/sc/memiavl/config.go) |
| `go_exported` | `type DB struct` | [storage/state_db/sc/memiavl/db.go](../../../storage/state_db/sc/memiavl/db.go) |
| `go_exported` | `func GetLatestVersion(dir string) (int64, error)` | [storage/state_db/sc/memiavl/db.go](../../../storage/state_db/sc/memiavl/db.go) |
| `go_exported` | `func OpenDB(targetVersion int64, opts Options) (database *DB, _err error)` | [storage/state_db/sc/memiavl/db.go](../../../storage/state_db/sc/memiavl/db.go) |
| `storage_key` | `SnapshotPrefix = "snapshot-"` | [storage/state_db/sc/memiavl/db.go](../../../storage/state_db/sc/memiavl/db.go) |
| `go_exported` | `func NewMultiTreeExporter(dir string, version uint32, onlyAllowExportOnSnapshotVersion bool) (exporter *MultiTreeExporter, err error) {` | [storage/state_db/sc/memiavl/export.go](../../../storage/state_db/sc/memiavl/export.go) |
| `storage_key` | `MetadataFileName = "__metadata"` | [storage/state_db/sc/memiavl/multitree.go](../../../storage/state_db/sc/memiavl/multitree.go) |
| `go_exported` | `type Node interface` | [storage/state_db/sc/memiavl/node.go](../../../storage/state_db/sc/memiavl/node.go) |
| `go_exported` | `type Options struct` | [storage/state_db/sc/memiavl/opts.go](../../../storage/state_db/sc/memiavl/opts.go) |
| `file_format` | `SnapshotFileMagic = 1280721225` | [storage/state_db/sc/memiavl/snapshot.go](../../../storage/state_db/sc/memiavl/snapshot.go) |
| `go_exported` | `type CommitStore struct` | [storage/state_db/sc/memiavl/store.go](../../../storage/state_db/sc/memiavl/store.go) |
| `go_exported` | `func NewCommitStore(homeDir string, config Config) *CommitStore` | [storage/state_db/sc/memiavl/store.go](../../../storage/state_db/sc/memiavl/store.go) |
| `go_exported` | `func New(_ int) *Tree` | [storage/state_db/sc/memiavl/tree.go](../../../storage/state_db/sc/memiavl/tree.go) |
| `go_exported` | `type Tree struct` | [storage/state_db/sc/memiavl/tree.go](../../../storage/state_db/sc/memiavl/tree.go) |
| `go_exported` | `func DeserializeMigrationBoundary(data []byte) (MigrationBoundary, error)` | [storage/state_db/sc/migration/migration_boundary.go](../../../storage/state_db/sc/migration/migration_boundary.go) |
| `go_exported` | `func NewMigrationBoundary(` | [storage/state_db/sc/migration/migration_boundary.go](../../../storage/state_db/sc/migration/migration_boundary.go) |
| `go_exported` | `MigrationBoundaryKey = "migration-boundary"` | [storage/state_db/sc/migration/migration_keys.go](../../../storage/state_db/sc/migration/migration_keys.go) |
| `go_exported` | `const MigrationStore = "migration"` | [storage/state_db/sc/migration/migration_keys.go](../../../storage/state_db/sc/migration/migration_keys.go) |
| `go_exported` | `MigrationVersionKey = "migration-version"` | [storage/state_db/sc/migration/migration_keys.go](../../../storage/state_db/sc/migration/migration_keys.go) |
| `storage_key` | `MigrationBoundaryKey = "migration-boundary"` | [storage/state_db/sc/migration/migration_keys.go](../../../storage/state_db/sc/migration/migration_keys.go) |
| `storage_key` | `MigrationVersionKey = "migration-version"` | [storage/state_db/sc/migration/migration_keys.go](../../../storage/state_db/sc/migration/migration_keys.go) |
| `go_exported` | `func IsAtVersion(reader DBReader, version uint64) (bool, error)` | [storage/state_db/sc/migration/migration_manager.go](../../../storage/state_db/sc/migration/migration_manager.go) |
| `go_exported` | `type DBReader func(store string, key []byte) ([]byte, bool, error)` | [storage/state_db/sc/migration/migration_types.go](../../../storage/state_db/sc/migration/migration_types.go) |
| `go_exported` | `MigrationNotStarted MigrationStatus = 0` | [storage/state_db/sc/migration/migration_types.go](../../../storage/state_db/sc/migration/migration_types.go) |
| `go_exported` | `type Router interface` | [storage/state_db/sc/migration/migration_types.go](../../../storage/state_db/sc/migration/migration_types.go) |
| `go_exported` | `Version0_MemiavlOnly = 0` | [storage/state_db/sc/migration/migration_versions.go](../../../storage/state_db/sc/migration/migration_versions.go) |
| `go_exported` | `Version1_MigrateEVM = 1` | [storage/state_db/sc/migration/migration_versions.go](../../../storage/state_db/sc/migration/migration_versions.go) |
| `go_exported` | `Version2_MigrateAllButBank = 2` | [storage/state_db/sc/migration/migration_versions.go](../../../storage/state_db/sc/migration/migration_versions.go) |
| `go_exported` | `Version3_FlatKVOnly = 3` | [storage/state_db/sc/migration/migration_versions.go](../../../storage/state_db/sc/migration/migration_versions.go) |
| `go_exported` | `func BuildRouter(` | [storage/state_db/sc/migration/router_builder.go](../../../storage/state_db/sc/migration/router_builder.go) |
| `go_exported` | `func NewRouterCommitKVStore(` | [storage/state_db/sc/migration/router_kvstore.go](../../../storage/state_db/sc/migration/router_kvstore.go) |
| `go_exported` | `type CommitKVStore interface` | [storage/state_db/sc/types/types.go](../../../storage/state_db/sc/types/types.go) |
| `go_exported` | `type Committer interface` | [storage/state_db/sc/types/types.go](../../../storage/state_db/sc/types/types.go) |
| `go_exported` | `type Exporter interface` | [storage/state_db/sc/types/types.go](../../../storage/state_db/sc/types/types.go) |
| `go_exported` | `type Importer interface` | [storage/state_db/sc/types/types.go](../../../storage/state_db/sc/types/types.go) |
| `go_exported` | `type SnapshotNode struct` | [storage/state_db/sc/types/types.go](../../../storage/state_db/sc/types/types.go) |
| `go_exported` | `func ResolveBackend(backendName string) OpenFunc` | [storage/state_db/ss/backend/resolve.go](../../../storage/state_db/ss/backend/resolve.go) |
| `go_exported` | `type CompositeStateStore struct` | [storage/state_db/ss/composite/store.go](../../../storage/state_db/ss/composite/store.go) |
| `go_exported` | `func NewCompositeStateStore(` | [storage/state_db/ss/composite/store.go](../../../storage/state_db/ss/composite/store.go) |
| `go_exported` | `func NewCosmosStateStore(db types.StateStore) types.StateStore` | [storage/state_db/ss/cosmos/store.go](../../../storage/state_db/ss/cosmos/store.go) |
| `go_exported` | `func NewEVMStateStore(dir string, ssConfig config.StateStoreConfig) (*EVMStateStore, error) {` | [storage/state_db/ss/evm/store.go](../../../storage/state_db/ss/evm/store.go) |
| `go_exported` | `const EVMStoreKey = commonevm.EVMStoreKey` | [storage/state_db/ss/evm/types.go](../../../storage/state_db/ss/evm/types.go) |
| `go_exported` | `type KafkaConfig struct` | [storage/state_db/ss/offload/kafka.go](../../../storage/state_db/ss/offload/kafka.go) |
| `go_exported` | `func NewKafkaStream(cfg KafkaConfig) (Stream, error)` | [storage/state_db/ss/offload/kafka.go](../../../storage/state_db/ss/offload/kafka.go) |
| `go_exported` | `type Ack struct` | [storage/state_db/ss/offload/types.go](../../../storage/state_db/ss/offload/types.go) |
| `go_exported` | `type Stream interface` | [storage/state_db/ss/offload/types.go](../../../storage/state_db/ss/offload/types.go) |
| `go_exported` | `type Manager struct` | [storage/state_db/ss/pruning/manager.go](../../../storage/state_db/ss/pruning/manager.go) |
| `go_exported` | `func NewPruningManager(` | [storage/state_db/ss/pruning/manager.go](../../../storage/state_db/ss/pruning/manager.go) |
| `go_exported` | `func NewStateStore(homeDir string, ssConfig config.StateStoreConfig) (types.StateStore, error) {` | [storage/state_db/ss/store.go](../../../storage/state_db/ss/store.go) |
| `go_exported` | `func BenchmarkDBForwardIteration(db types.StateStore, inputKVDir string, numVersions int, concurrency int, maxOps int64, iterationSteps int) {` | [storage/tools/bench/benchmark.go](../../../storage/tools/bench/benchmark.go) |
| `go_exported` | `func BenchmarkDBRead(db types.StateStore, inputKVDir string, numVersions int, concurrency int, maxOps int64) {` | [storage/tools/bench/benchmark.go](../../../storage/tools/bench/benchmark.go) |
| `go_exported` | `func BenchmarkDBReverseIteration(db types.StateStore, inputKVDir string, numVersions int, concurrency int, maxOps int64, iterationSteps int) {` | [storage/tools/bench/benchmark.go](../../../storage/tools/bench/benchmark.go) |
| `go_exported` | `func BenchmarkDBWrite(db types.StateStore, inputKVDir string, numVersions int, concurrency int, batchSize int) {` | [storage/tools/bench/benchmark.go](../../../storage/tools/bench/benchmark.go) |
| `go_exported` | `func GenerateCmd() *cobra.Command` | [storage/tools/cmd/paxdb/benchmark/generate.go](../../../storage/tools/cmd/paxdb/benchmark/generate.go) |
| `go_exported` | `func DBIterationCmd() *cobra.Command` | [storage/tools/cmd/paxdb/benchmark/iteration.go](../../../storage/tools/cmd/paxdb/benchmark/iteration.go) |
| `go_exported` | `func DBRandomReadCmd() *cobra.Command` | [storage/tools/cmd/paxdb/benchmark/random_read.go](../../../storage/tools/cmd/paxdb/benchmark/random_read.go) |
| `go_exported` | `func DBReverseIterationCmd() *cobra.Command` | [storage/tools/cmd/paxdb/benchmark/reverse_iteration.go](../../../storage/tools/cmd/paxdb/benchmark/reverse_iteration.go) |
| `go_exported` | `ValidDBBackends = map[string]bool` | [storage/tools/cmd/paxdb/benchmark/types.go](../../../storage/tools/cmd/paxdb/benchmark/types.go) |
| `go_exported` | `func DBWriteCmd() *cobra.Command` | [storage/tools/cmd/paxdb/benchmark/write.go](../../../storage/tools/cmd/paxdb/benchmark/write.go) |
| `go_exported` | `func DumpDbCmd() *cobra.Command` | [storage/tools/cmd/paxdb/operations/dump_db.go](../../../storage/tools/cmd/paxdb/operations/dump_db.go) |
| `go_exported` | `func DumpFlatKVCmd() *cobra.Command` | [storage/tools/cmd/paxdb/operations/dump_flatkv.go](../../../storage/tools/cmd/paxdb/operations/dump_flatkv.go) |
| `go_exported` | `func DumpIAVLCmd() *cobra.Command` | [storage/tools/cmd/paxdb/operations/dump_iavl.go](../../../storage/tools/cmd/paxdb/operations/dump_iavl.go) |
| `go_exported` | `func ImportFlatKVFromMemiavlCmd() *cobra.Command` | [storage/tools/cmd/paxdb/operations/import_flatkv_from_memiavl.go](../../../storage/tools/cmd/paxdb/operations/import_flatkv_from_memiavl.go) |
| `go_exported` | `func MemiavlLatestVersionCmd() *cobra.Command` | [storage/tools/cmd/paxdb/operations/import_flatkv_from_memiavl.go](../../../storage/tools/cmd/paxdb/operations/import_flatkv_from_memiavl.go) |
| `go_exported` | `func MigrateEvmStatusCmd() *cobra.Command` | [storage/tools/cmd/paxdb/operations/migrate_evm_status.go](../../../storage/tools/cmd/paxdb/operations/migrate_evm_status.go) |
| `go_exported` | `func PruneCmd() *cobra.Command` | [storage/tools/cmd/paxdb/operations/prune.go](../../../storage/tools/cmd/paxdb/operations/prune.go) |
| `go_exported` | `func ReplayChangelogCmd() *cobra.Command` | [storage/tools/cmd/paxdb/operations/replay_changelog.go](../../../storage/tools/cmd/paxdb/operations/replay_changelog.go) |
| `go_exported` | `func StateSizeCmd() *cobra.Command` | [storage/tools/cmd/paxdb/operations/state_size.go](../../../storage/tools/cmd/paxdb/operations/state_size.go) |
| `go_exported` | `func TraceProfileReportCmd() *cobra.Command` | [storage/tools/cmd/paxdb/operations/trace_profile_report.go](../../../storage/tools/cmd/paxdb/operations/trace_profile_report.go) |
| `go_exported` | `type ContractSizeEntry struct` | [storage/tools/utils/dynamo.go](../../../storage/tools/utils/dynamo.go) |
| `go_exported` | `func NewDynamoDBClient(tableName, awsRegion string) (*DynamoDBClient, error)` | [storage/tools/utils/dynamo.go](../../../storage/tools/utils/dynamo.go) |
| `go_exported` | `type PrefixSize struct` | [storage/tools/utils/dynamo.go](../../../storage/tools/utils/dynamo.go) |
| `go_exported` | `type StateSizeAnalysis struct` | [storage/tools/utils/dynamo.go](../../../storage/tools/utils/dynamo.go) |
| `go_exported` | `func CalculatePercentile(latencies []time.Duration, percentile float64) time.Duration` | [storage/tools/utils/utils.go](../../../storage/tools/utils/utils.go) |
| `go_exported` | `func CreateFile(outputDir string, fileName string) (*os.File, error)` | [storage/tools/utils/utils.go](../../../storage/tools/utils/utils.go) |
| `go_exported` | `type KeyValuePair struct` | [storage/tools/utils/utils.go](../../../storage/tools/utils/utils.go) |
| `go_exported` | `func LoadAndShuffleKV(inputDir string, concurrency int) ([]KeyValuePair, error)` | [storage/tools/utils/utils.go](../../../storage/tools/utils/utils.go) |
| `file_format` | `func NewChangelogWAL(dir string, config Config) (ChangelogWAL, error)` | [storage/wal/changelog.go](../../../storage/wal/changelog.go) |
| `go_exported` | `type ChangelogWAL = GenericWAL[proto.ChangelogEntry]` | [storage/wal/changelog.go](../../../storage/wal/changelog.go) |
| `go_exported` | `func NewChangelogWAL(dir string, config Config) (ChangelogWAL, error)` | [storage/wal/changelog.go](../../../storage/wal/changelog.go) |
| `go_exported` | `type GenericWAL[T any] interface` | [storage/wal/types.go](../../../storage/wal/types.go) |
| `go_exported` | `type GenericWALProcessor[T any] interface` | [storage/wal/types.go](../../../storage/wal/types.go) |
| `go_exported` | `type MarshalFn[T any] func(entry T) ([]byte, error)` | [storage/wal/types.go](../../../storage/wal/types.go) |
| `go_exported` | `type UnmarshalFn[T any] func(data []byte) (T, error)` | [storage/wal/types.go](../../../storage/wal/types.go) |
| `go_exported` | `func GetLastEntryVersion(dir string) (index uint64, version int64, err error)` | [storage/wal/utils.go](../../../storage/wal/utils.go) |
| `go_exported` | `func GetLastIndex(dir string) (index uint64, err error)` | [storage/wal/utils.go](../../../storage/wal/utils.go) |
| `go_exported` | `func LogPath(dir string) string` | [storage/wal/utils.go](../../../storage/wal/utils.go) |
| `go_exported` | `func MockKVPairs(kvPairs ...string) []*paxdbproto.KVPair` | [storage/wal/utils.go](../../../storage/wal/utils.go) |
| `go_exported` | `type Config struct` | [storage/wal/wal.go](../../../storage/wal/wal.go) |
| `go_exported` | `func NewWAL[T any](` | [storage/wal/wal.go](../../../storage/wal/wal.go) |
| `go_exported` | `type WAL[T any] struct` | [storage/wal/wal.go](../../../storage/wal/wal.go) |
| `go_exported` | `func (walLog *WAL[T]) Close() error` | [storage/wal/wal.go](../../../storage/wal/wal.go) |
| `go_exported` | `func (walLog *WAL[T]) FirstOffset() (uint64, error)` | [storage/wal/wal.go](../../../storage/wal/wal.go) |
| `go_exported` | `func (walLog *WAL[T]) LastOffset() (uint64, error)` | [storage/wal/wal.go](../../../storage/wal/wal.go) |
| `go_exported` | `func (walLog *WAL[T]) ReadAt(index uint64) (T, error)` | [storage/wal/wal.go](../../../storage/wal/wal.go) |
| `go_exported` | `func (walLog *WAL[T]) Replay(start uint64, end uint64, processFn func(index uint64, entry T) error) error {` | [storage/wal/wal.go](../../../storage/wal/wal.go) |
| `go_exported` | `func (walLog *WAL[T]) TruncateAfter(index uint64) error` | [storage/wal/wal.go](../../../storage/wal/wal.go) |
| `go_exported` | `func (walLog *WAL[T]) TruncateAll() error` | [storage/wal/wal.go](../../../storage/wal/wal.go) |
| `go_exported` | `func (walLog *WAL[T]) TruncateBefore(index uint64) error` | [storage/wal/wal.go](../../../storage/wal/wal.go) |
| `go_exported` | `func (walLog *WAL[T]) Write(entry T) error` | [storage/wal/wal.go](../../../storage/wal/wal.go) |

## Node config keys

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func (cfg *Config) ValidateBasic() error` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `config_key` | `FlagSCSnapshotWriterLimit = "state-commit.sc-snapshot-writer-limit"` | [node/paxdb.go](../../../node/paxdb.go) |
| `config_key` | `async-write-buffer = {{ .StateCommit.FlatKVConfig.AsyncWriteBuffer }}` | [storage/config/toml.go](../../../storage/config/toml.go) |
| `config_key` | `fsync = {{ .StateCommit.FlatKVConfig.Fsync }}` | [storage/config/toml.go](../../../storage/config/toml.go) |
| `config_key` | `snapshot-interval = {{ .StateCommit.FlatKVConfig.SnapshotInterval }}` | [storage/config/toml.go](../../../storage/config/toml.go) |
| `config_key` | `snapshot-keep-recent = {{ .StateCommit.FlatKVConfig.SnapshotKeepRecent }}` | [storage/config/toml.go](../../../storage/config/toml.go) |
| `config_key` | `contract_state_checks = {{ .ETHReplay.ContractStateChecks }}` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `config_key` | `memory_cache_size = {{ .WASM.MemoryCacheSize }}` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `config_key` | `simulation_gas_limit = "{{ .WASM.SimulationGasLimit }}"` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `go_exported` | `func ParseFlatKVConfig(opts interface{ Get(string) interface{} }) (config.Config, error)` | [storage/config/flatkv_parse.go](../../../storage/config/flatkv_parse.go) |
| `config_key` | `sc-snapshot-writer-limit = {{ .StateCommit.MemIAVLConfig.SnapshotWriterLimit }}` | [storage/config/toml.go](../../../storage/config/toml.go) |
| `go_exported` | `type AutobahnFileConfig struct` | [consensus/config/autobahn.go](../../../consensus/config/autobahn.go) |
| `go_exported` | `func (fc *AutobahnFileConfig) ConsensusPersistentStateDir() (utils.Option[string], error)` | [consensus/config/autobahn.go](../../../consensus/config/autobahn.go) |
| `go_exported` | `func (fc *AutobahnFileConfig) Validate() error` | [consensus/config/autobahn.go](../../../consensus/config/autobahn.go) |
| `go_exported` | `type AutobahnValidator struct` | [consensus/config/autobahn.go](../../../consensus/config/autobahn.go) |
| `go_exported` | `func (av *AutobahnValidator) GetEVMRPC() utils.Option[*url.URL]` | [consensus/config/autobahn.go](../../../consensus/config/autobahn.go) |
| `go_exported` | `ErrPersistentStateDirBlank = errors.New("persistent_state_dir must not be blank")` | [consensus/config/autobahn.go](../../../consensus/config/autobahn.go) |
| `go_exported` | `type URL struct{ *url.URL }` | [consensus/config/autobahn.go](../../../consensus/config/autobahn.go) |
| `go_exported` | `func (u URL) MarshalText() ([]byte, error) { return []byte(u.String()), nil }` | [consensus/config/autobahn.go](../../../consensus/config/autobahn.go) |
| `go_exported` | `func (u *URL) UnmarshalText(text []byte) error` | [consensus/config/autobahn.go](../../../consensus/config/autobahn.go) |
| `go_exported` | `type BaseConfig struct` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg BaseConfig) ChainID() string` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg BaseConfig) DBDir() string` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg BaseConfig) GenesisFile() string` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg BaseConfig) LoadNodeKeyID() (types.NodeID, error)` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg BaseConfig) LoadOrGenNodeKeyID() (types.NodeID, error)` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg BaseConfig) NodeKeyFile() string` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg BaseConfig) ValidateBasic() error` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `type Config struct` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg *Config) DeprecatedFieldWarning() error` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg *Config) SetRoot(root string) *Config` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `type ConsensusConfig struct` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg *ConsensusConfig) DeprecatedFieldWarning() error` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg *ConsensusConfig) ValidateBasic() error` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg *ConsensusConfig) WaitForTxs() bool` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg *ConsensusConfig) WalFile() string` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func DefaultBaseConfig() BaseConfig` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func DefaultConfig() *Config` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func DefaultConsensusConfig() *ConsensusConfig` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func DefaultInstrumentationConfig() *InstrumentationConfig` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `DefaultLogLevel = "info"` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func DefaultMempoolConfig() *MempoolConfig` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func DefaultP2PConfig() *P2PConfig` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func DefaultPrivValidatorConfig() *PrivValidatorConfig` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func DefaultRPCConfig() *RPCConfig` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func DefaultSelfRemediationConfig() *SelfRemediationConfig` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func DefaultSequencerConfig() SequencerConfig` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func DefaultStateSyncConfig() *StateSyncConfig` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `DefaultTendermintDir = ".tendermint"` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func DefaultTxIndexConfig() *TxIndexConfig` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func DefaultValidatorConfig() *Config` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `FuzzModeDelay` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `FuzzModeDrop = iota` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `type InstrumentationConfig struct` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg *InstrumentationConfig) ValidateBasic() error` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `type MempoolConfig struct` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg *MempoolConfig) ToMempoolConfig() *mempoolcfg.Config` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg *MempoolConfig) ValidateBasic() error` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `ModeFull = "full"` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `ModeSeed = "seed"` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `ModeValidator = "validator"` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `type P2PConfig struct` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg *P2PConfig) ValidateBasic() error` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `type PrivValidatorConfig struct` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg *PrivValidatorConfig) AreSecurityOptionsPresent() bool` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg *PrivValidatorConfig) ClientCertificateFile() string` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg *PrivValidatorConfig) ClientKeyFile() string` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg *PrivValidatorConfig) KeyFile() string` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg *PrivValidatorConfig) RootCAFile() string` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg *PrivValidatorConfig) StateFile() string` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `type RPCConfig struct` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg RPCConfig) CertFile() string` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg *RPCConfig) IsCorsEnabled() bool` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg RPCConfig) IsTLSEnabled() bool` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg RPCConfig) KeyFile() string` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg *RPCConfig) ValidateBasic() error` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `type SelfRemediationConfig struct` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg *SelfRemediationConfig) ValidateBasic() error` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `type SequencerConfig struct` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg SequencerConfig) ProposeTimeout(round int32) time.Duration` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg SequencerConfig) ValidateBasic() error` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `type StateSyncConfig struct` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg *StateSyncConfig) TrustHashBytes() []byte` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func (cfg *StateSyncConfig) ValidateBasic() error` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func TestBaseConfig() BaseConfig` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func TestConfig() *Config` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func TestConsensusConfig() *ConsensusConfig` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func TestInstrumentationConfig() *InstrumentationConfig` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func TestMempoolConfig() *MempoolConfig` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func TestP2PConfig() *P2PConfig` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func TestRPCConfig() *RPCConfig` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func TestSelfRemediationConfig() *SelfRemediationConfig` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func TestStateSyncConfig() *StateSyncConfig` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `func TestTxIndexConfig() *TxIndexConfig` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `type TxIndexConfig struct` | [consensus/config/config.go](../../../consensus/config/config.go) |
| `go_exported` | `type DBContext struct` | [consensus/config/db.go](../../../consensus/config/db.go) |
| `go_exported` | `type DBProvider func(*DBContext) (dbm.DB, error)` | [consensus/config/db.go](../../../consensus/config/db.go) |
| `go_exported` | `func DefaultDBProvider(ctx *DBContext) (dbm.DB, error)` | [consensus/config/db.go](../../../consensus/config/db.go) |
| `go_exported` | `func ResolveDBDir(dbID string, baseDir string) string` | [consensus/config/db.go](../../../consensus/config/db.go) |
| `go_exported` | `func (cfg *Config) WriteToTemplate(path string) error` | [consensus/config/toml.go](../../../consensus/config/toml.go) |
| `go_exported` | `func EnsureRoot(rootDir string)` | [consensus/config/toml.go](../../../consensus/config/toml.go) |
| `go_exported` | `func ResetTestRoot(dir, testName string) (*Config, error)` | [consensus/config/toml.go](../../../consensus/config/toml.go) |
| `go_exported` | `func ResetTestRootWithChainID(dir, testName string, chainID string) (*Config, error)` | [consensus/config/toml.go](../../../consensus/config/toml.go) |
| `go_exported` | `func WriteConfigFile(rootDir string, config *Config) error` | [consensus/config/toml.go](../../../consensus/config/toml.go) |
| `config_key` | `eth_blocktest_enabled = {{ .ETHBlockTest.Enabled }}` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `config_key` | `eth_blocktest_test_data_path = "{{ .ETHBlockTest.TestDataPath }}"` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `config_key` | `eth_data_dir = "{{ .ETHReplay.EthDataDir }}"` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `config_key` | `eth_replay_enabled = {{ .ETHReplay.Enabled }}` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `config_key` | `eth_rpc = "{{ .ETHReplay.EthRPC }}"` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `config_key` | `evm_query_gas_limit = {{ .EvmQuery.GasLimit }}` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `config_key` | `baseapp.SetHaltHeight(cast.ToUint64(appOpts.Get(server.FlagHaltHeight))),` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `config_key` | `baseapp.SetHaltTime(cast.ToUint64(appOpts.Get(server.FlagHaltTime))),` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `config_key` | `baseapp.SetIndexEvents(cast.ToStringSlice(appOpts.Get(server.FlagIndexEvents))),` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `config_key` | `if cast.ToBool(appOpts.Get(server.FlagInterBlockCache))` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `config_key` | `cast.ToUint(appOpts.Get(server.FlagInvCheckPeriod)),` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `config_key` | `baseapp.SetMinRetainBlocks(cast.ToUint64(appOpts.Get(server.FlagMinRetainBlocks))),` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `config_key` | `baseapp.SetMinGasPrices(cast.ToString(appOpts.Get(server.FlagMinGasPrices))),` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `config_key` | `baseapp.SetOccEnabled(cast.ToBool(appOpts.Get(baseapp.FlagOccEnabled))),` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `config_key` | `snapshotDirectory := cast.ToString(appOpts.Get(server.FlagStateSyncSnapshotDir))` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `config_key` | `baseapp.SetTrace(cast.ToBool(appOpts.Get(server.FlagTrace))),` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `config_key` | `for _, h := range cast.ToIntSlice(appOpts.Get(server.FlagUnsafeSkipUpgrades))` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `env_var` | `IGNORE_SKIP_LIST` | [engine/tests/state_harness_test.go](../../../engine/tests/state_harness_test.go) |
| `env_var` | `STATE_TEST_DIR` | [engine/tests/state_harness_test.go](../../../engine/tests/state_harness_test.go) |
| `env_var` | `STATE_TEST_NAME` | [engine/tests/state_harness_test.go](../../../engine/tests/state_harness_test.go) |
| `env_var` | `USE_REGULAR_STORE` | [engine/tests/state_test.go](../../../engine/tests/state_test.go) |
| `env_var` | `BENCHMARK_CONFIG` | [node/benchmark/benchmark.go](../../../node/benchmark/benchmark.go) |
| `env_var` | `BENCHMARK_TXS_PER_BATCH` | [node/benchmark/generator.go](../../../node/benchmark/generator.go) |
| `config_key` | `flagSupplyEnabled = "light_invariance.supply_enabled"` | [node/invariance.go](../../../node/invariance.go) |
| `go_exported` | `func (app *App) LightInvarianceChecks(ctx context.Context, cms sdk.CommitMultiStore, config LightInvarianceConfig) (err error) {` | [node/invariance.go](../../../node/invariance.go) |
| `go_exported` | `func (app *App) LightInvarianceTotalSupply(ctx context.Context, cms sdk.CommitMultiStore)` | [node/invariance.go](../../../node/invariance.go) |
| `go_exported` | `var DefaultLightInvarianceConfig = LightInvarianceConfig` | [node/invariance.go](../../../node/invariance.go) |
| `go_exported` | `type LightInvarianceConfig struct` | [node/invariance.go](../../../node/invariance.go) |
| `go_exported` | `func ReadLightInvarianceConfig(opts servertypes.AppOptions) (LightInvarianceConfig, error)` | [node/invariance.go](../../../node/invariance.go) |
| `config_key` | `FlagSCAsyncCommitBuffer = "state-commit.sc-async-commit-buffer"` | [node/paxdb.go](../../../node/paxdb.go) |
| `config_key` | `FlagSCDirectory = "state-commit.sc-directory"` | [node/paxdb.go](../../../node/paxdb.go) |
| `config_key` | `FlagSCEnable = "state-commit.sc-enable"` | [node/paxdb.go](../../../node/paxdb.go) |
| `config_key` | `FlagSCHistoricalProofBurst = "state-commit.sc-historical-proof-burst"` | [node/paxdb.go](../../../node/paxdb.go) |
| `config_key` | `FlagSCHistoricalProofMaxInFlight = "state-commit.sc-historical-proof-max-inflight"` | [node/paxdb.go](../../../node/paxdb.go) |
| `config_key` | `FlagSCHistoricalProofRateLimit = "state-commit.sc-historical-proof-rate-limit"` | [node/paxdb.go](../../../node/paxdb.go) |
| `config_key` | `FlagSCSnapshotKeepRecent = "state-commit.sc-keep-recent"` | [node/paxdb.go](../../../node/paxdb.go) |
| `config_key` | `FlagSCKeysToMigratePerBlock = "state-commit.sc-keys-to-migrate-per-block"` | [node/paxdb.go](../../../node/paxdb.go) |
| `config_key` | `FlagSCSnapshotInterval = "state-commit.sc-snapshot-interval"` | [node/paxdb.go](../../../node/paxdb.go) |
| `config_key` | `FlagSCSnapshotMinTimeInterval = "state-commit.sc-snapshot-min-time-interval"` | [node/paxdb.go](../../../node/paxdb.go) |
| `config_key` | `FlagSCSnapshotPrefetchThreshold = "state-commit.sc-snapshot-prefetch-threshold"` | [node/paxdb.go](../../../node/paxdb.go) |
| `config_key` | `FlagSCSnapshotWriteRateMBps = "state-commit.sc-snapshot-write-rate-mbps"` | [node/paxdb.go](../../../node/paxdb.go) |
| `config_key` | `FlagSCWriteMode = "state-commit.sc-write-mode"` | [node/paxdb.go](../../../node/paxdb.go) |
| `config_key` | `FlagEVMSSDirectory = "state-store.evm-ss-db-directory"` | [node/paxdb.go](../../../node/paxdb.go) |
| `config_key` | `FlagEVMSSSeparateDBs = "state-store.evm-ss-separate-dbs"` | [node/paxdb.go](../../../node/paxdb.go) |
| `config_key` | `FlagEVMSSSplit = "state-store.evm-ss-split"` | [node/paxdb.go](../../../node/paxdb.go) |
| `config_key` | `FlagSSAsyncWriterBuffer = "state-store.ss-async-write-buffer"` | [node/paxdb.go](../../../node/paxdb.go) |
| `config_key` | `FlagSSBackend = "state-store.ss-backend"` | [node/paxdb.go](../../../node/paxdb.go) |
| `config_key` | `FlagSSDirectory = "state-store.ss-db-directory"` | [node/paxdb.go](../../../node/paxdb.go) |
| `config_key` | `FlagSSEnable = "state-store.ss-enable"` | [node/paxdb.go](../../../node/paxdb.go) |
| `config_key` | `FlagSSImportNumWorkers = "state-store.ss-import-num-workers"` | [node/paxdb.go](../../../node/paxdb.go) |
| `config_key` | `FlagSSKeepRecent = "state-store.ss-keep-recent"` | [node/paxdb.go](../../../node/paxdb.go) |
| `config_key` | `FlagSSPruneInterval = "state-store.ss-prune-interval"` | [node/paxdb.go](../../../node/paxdb.go) |
| `config_key` | `FlagSnapshotInterval = "state-sync.snapshot-interval"` | [node/paxdb.go](../../../node/paxdb.go) |
| `go_exported` | `FlagEVMSSDirectory = "state-store.evm-ss-db-directory"` | [node/paxdb.go](../../../node/paxdb.go) |
| `go_exported` | `FlagEVMSSSeparateDBs = "state-store.evm-ss-separate-dbs"` | [node/paxdb.go](../../../node/paxdb.go) |
| `go_exported` | `FlagEVMSSSplit = "state-store.evm-ss-split"` | [node/paxdb.go](../../../node/paxdb.go) |
| `go_exported` | `FlagSCAsyncCommitBuffer = "state-commit.sc-async-commit-buffer"` | [node/paxdb.go](../../../node/paxdb.go) |
| `go_exported` | `FlagSCDirectory = "state-commit.sc-directory"` | [node/paxdb.go](../../../node/paxdb.go) |
| `go_exported` | `FlagSCEnable = "state-commit.sc-enable"` | [node/paxdb.go](../../../node/paxdb.go) |
| `go_exported` | `FlagSCHistoricalProofBurst = "state-commit.sc-historical-proof-burst"` | [node/paxdb.go](../../../node/paxdb.go) |
| `go_exported` | `FlagSCHistoricalProofMaxInFlight = "state-commit.sc-historical-proof-max-inflight"` | [node/paxdb.go](../../../node/paxdb.go) |
| `go_exported` | `FlagSCHistoricalProofRateLimit = "state-commit.sc-historical-proof-rate-limit"` | [node/paxdb.go](../../../node/paxdb.go) |
| `go_exported` | `FlagSCKeysToMigratePerBlock = "state-commit.sc-keys-to-migrate-per-block"` | [node/paxdb.go](../../../node/paxdb.go) |
| `go_exported` | `FlagSCSnapshotInterval = "state-commit.sc-snapshot-interval"` | [node/paxdb.go](../../../node/paxdb.go) |
| `go_exported` | `FlagSCSnapshotKeepRecent = "state-commit.sc-keep-recent"` | [node/paxdb.go](../../../node/paxdb.go) |
| `go_exported` | `FlagSCSnapshotMinTimeInterval = "state-commit.sc-snapshot-min-time-interval"` | [node/paxdb.go](../../../node/paxdb.go) |
| `go_exported` | `FlagSCSnapshotPrefetchThreshold = "state-commit.sc-snapshot-prefetch-threshold"` | [node/paxdb.go](../../../node/paxdb.go) |
| `go_exported` | `FlagSCSnapshotWriteRateMBps = "state-commit.sc-snapshot-write-rate-mbps"` | [node/paxdb.go](../../../node/paxdb.go) |
| `go_exported` | `FlagSCSnapshotWriterLimit = "state-commit.sc-snapshot-writer-limit"` | [node/paxdb.go](../../../node/paxdb.go) |
| `go_exported` | `FlagSCWriteMode = "state-commit.sc-write-mode"` | [node/paxdb.go](../../../node/paxdb.go) |
| `go_exported` | `FlagSnapshotInterval = "state-sync.snapshot-interval"` | [node/paxdb.go](../../../node/paxdb.go) |
| `go_exported` | `FlagSSAsyncWriterBuffer = "state-store.ss-async-write-buffer"` | [node/paxdb.go](../../../node/paxdb.go) |
| `go_exported` | `FlagSSBackend = "state-store.ss-backend"` | [node/paxdb.go](../../../node/paxdb.go) |
| `go_exported` | `FlagSSDirectory = "state-store.ss-db-directory"` | [node/paxdb.go](../../../node/paxdb.go) |
| `go_exported` | `FlagSSEnable = "state-store.ss-enable"` | [node/paxdb.go](../../../node/paxdb.go) |
| `go_exported` | `FlagSSImportNumWorkers = "state-store.ss-import-num-workers"` | [node/paxdb.go](../../../node/paxdb.go) |
| `go_exported` | `FlagSSKeepRecent = "state-store.ss-keep-recent"` | [node/paxdb.go](../../../node/paxdb.go) |
| `go_exported` | `FlagSSPruneInterval = "state-store.ss-prune-interval"` | [node/paxdb.go](../../../node/paxdb.go) |
| `go_exported` | `var GigaKeys = []string{"evm", "bank"}` | [node/paxdb.go](../../../node/paxdb.go) |
| `go_exported` | `func SetupPaxDB(` | [node/paxdb.go](../../../node/paxdb.go) |
| `config_key` | `receiptConfig.KeepRecent = cast.ToInt(appOpts.Get(server.FlagMinRetainBlocks))` | [node/receipt_store_config.go](../../../node/receipt_store_config.go) |
| `env_var` | `UPGRADE_VERSION_LIST` | [node/upgrades.go](../../../node/upgrades.go) |
| `config_key` | `flagRSAsyncWriteBuffer = "receipt-store.async-write-buffer"` | [storage/config/receipt_config.go](../../../storage/config/receipt_config.go) |
| `config_key` | `flagRSMisnamedBackend = "receipt-store.backend"` | [storage/config/receipt_config.go](../../../storage/config/receipt_config.go) |
| `config_key` | `flagRSDBDirectory = "receipt-store.db-directory"` | [storage/config/receipt_config.go](../../../storage/config/receipt_config.go) |
| `config_key` | `flagRSPruneIntervalSeconds = "receipt-store.prune-interval-seconds"` | [storage/config/receipt_config.go](../../../storage/config/receipt_config.go) |
| `config_key` | `flagRSBackend = "receipt-store.rs-backend"` | [storage/config/receipt_config.go](../../../storage/config/receipt_config.go) |
| `env_var` | `SNAPSHOT_PATH` | [storage/state_db/bench/bench_sc_long_running_test.go](../../../storage/state_db/bench/bench_sc_long_running_test.go) |
| source | — | [node/genesis/genesis_test.go](../../../node/genesis/genesis_test.go) |

## Node CLI commands

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `strcmp(argv[1], "--migrate-snapshot-issuance") == 0)` | [cmd/layerx-genesis/lxp_genesis_build_cli.c](../../../cmd/layerx-genesis/lxp_genesis_build_cli.c) |
| `cli_command` | `int main(int argc, char **argv)` | [cmd/layerx-module-registry/main.c](../../../cmd/layerx-module-registry/main.c) |
| `cli_command` | `"submit" => true,` | [cmd/layerxctl/src/main.rs](../../../cmd/layerxctl/src/main.rs) |
| `go_exported` | `type CustomAppConfig struct` | [daemon/paxd/cmd/app_config.go](../../../daemon/paxd/cmd/app_config.go) |
| `go_exported` | `func NewCustomAppConfig(baseConfig *srvconfig.Config, evmConfig evmrpcconfig.Config) CustomAppConfig {` | [daemon/paxd/cmd/app_config.go](../../../daemon/paxd/cmd/app_config.go) |
| `go_exported` | `type WASMConfig struct` | [daemon/paxd/cmd/app_config.go](../../../daemon/paxd/cmd/app_config.go) |
| `cli_command` | `rootCmd.AddCommand(` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `cli_command` | `server.AddCommands(` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `cli_command` | `committed-fixture record --paxd PATH --out DIR` | [daemon/committed-fixture/main.go](../../../daemon/committed-fixture/main.go) |
| `cli_command` | `Use: "sequencer"` | [daemon/paxd/cmd/sequencer.go](../../../daemon/paxd/cmd/sequencer.go) |
| `cli_command` | `if (strcmp(key, "--asset") == 0) { bit = 1U; asset = value; }` | [cmd/layerx-module-registry/main.c](../../../cmd/layerx-module-registry/main.c) |
| `cli_command` | `else if (strcmp(key, "--currency") == 0) { bit = 4U; currency = value; }` | [cmd/layerx-module-registry/main.c](../../../cmd/layerx-module-registry/main.c) |
| `cli_command` | `if (strcmp(key, "--enable-module") == 0)` | [cmd/layerx-module-registry/main.c](../../../cmd/layerx-module-registry/main.c) |
| `cli_command` | `else if (strcmp(key, "--symbol") == 0) { bit = 2U; symbol = value; }` | [cmd/layerx-module-registry/main.c](../../../cmd/layerx-module-registry/main.c) |
| `cli_command` | `fn main() -> ExitCode` | [cmd/layerxctl/src/main.rs](../../../cmd/layerxctl/src/main.rs) |
| `cli_command` | `matches!(key.as_str(), "--to" \| "--data" \| "--block-height")` | [cmd/layerxctl/src/main.rs](../../../cmd/layerxctl/src/main.rs) |
| `cli_command` | `let common = matches!(key.as_str(), "--rpc-url" \| "--ca-cert");` | [cmd/layerxctl/src/main.rs](../../../cmd/layerxctl/src/main.rs) |
| `cli_command` | `key == "--transaction"` | [cmd/layerxctl/src/main.rs](../../../cmd/layerxctl/src/main.rs) |
| `cli_command` | `"read-state" => false,` | [cmd/layerxctl/src/main.rs](../../../cmd/layerxctl/src/main.rs) |
| `cli_command` | `Use: "blocktest",` | [daemon/paxd/cmd/blocktest.go](../../../daemon/paxd/cmd/blocktest.go) |
| `cli_command` | `cmd.Flags().String("block-test", "", "path to a block test json file")` | [daemon/paxd/cmd/blocktest.go](../../../daemon/paxd/cmd/blocktest.go) |
| `cli_command` | `cmd.Flags().String(flags.FlagChainID, "pax-chain", "chain ID")` | [daemon/paxd/cmd/blocktest.go](../../../daemon/paxd/cmd/blocktest.go) |
| `cli_command` | `cmd.Flags().String(flags.FlagHome, defaultNodeHome, "The database home directory")` | [daemon/paxd/cmd/blocktest.go](../../../daemon/paxd/cmd/blocktest.go) |
| `cli_command` | `cmd.Flags().String("test-name", "", "individual test name")` | [daemon/paxd/cmd/blocktest.go](../../../daemon/paxd/cmd/blocktest.go) |
| `go_exported` | `func BlocktestCmd(defaultNodeHome string) *cobra.Command` | [daemon/paxd/cmd/blocktest.go](../../../daemon/paxd/cmd/blocktest.go) |
| `cli_command` | `Use: "ethreplay",` | [daemon/paxd/cmd/ethreplay.go](../../../daemon/paxd/cmd/ethreplay.go) |
| `cli_command` | `cmd.Flags().String(flags.FlagChainID, "pax-chain", "chain ID")` | [daemon/paxd/cmd/ethreplay.go](../../../daemon/paxd/cmd/ethreplay.go) |
| `cli_command` | `cmd.Flags().String(flags.FlagHome, defaultNodeHome, "The database home directory")` | [daemon/paxd/cmd/ethreplay.go](../../../daemon/paxd/cmd/ethreplay.go) |
| `go_exported` | `func ReplayCmd(defaultNodeHome string) *cobra.Command` | [daemon/paxd/cmd/ethreplay.go](../../../daemon/paxd/cmd/ethreplay.go) |
| `cli_command` | `Use: "add-genesis-account [address_or_key_name] [coin][,[coin]]",` | [daemon/paxd/cmd/genaccounts.go](../../../daemon/paxd/cmd/genaccounts.go) |
| `cli_command` | `cmd.Flags().String(flagVestingAmt, "", "amount of coins for vesting accounts")` | [daemon/paxd/cmd/genaccounts.go](../../../daemon/paxd/cmd/genaccounts.go) |
| `cli_command` | `cmd.Flags().String(flags.FlagHome, defaultNodeHome, "The application home directory")` | [daemon/paxd/cmd/genaccounts.go](../../../daemon/paxd/cmd/genaccounts.go) |
| `go_exported` | `func AddGenesisAccountCmd(defaultNodeHome string) *cobra.Command` | [daemon/paxd/cmd/genaccounts.go](../../../daemon/paxd/cmd/genaccounts.go) |
| `cli_command` | `Use: "add-wasm-genesis-message",` | [daemon/paxd/cmd/genwasm.go](../../../daemon/paxd/cmd/genwasm.go) |
| `go_exported` | `func AddGenesisWasmMsgCmd(defaultNodeHome string) *cobra.Command` | [daemon/paxd/cmd/genwasm.go](../../../daemon/paxd/cmd/genwasm.go) |
| `cli_command` | `cmd.Flags().String(FlagMode, "full", "node mode: validator, full, seed, or archive")` | [daemon/paxd/cmd/init.go](../../../daemon/paxd/cmd/init.go) |
| `cli_command` | `cmd.Flags().String(cli.HomeFlag, defaultNodeHome, "node's home directory")` | [daemon/paxd/cmd/init.go](../../../daemon/paxd/cmd/init.go) |
| `cli_command` | `Use: "init [moniker]",` | [daemon/paxd/cmd/init.go](../../../daemon/paxd/cmd/init.go) |
| `go_exported` | `FlagMode = "mode"` | [daemon/paxd/cmd/init.go](../../../daemon/paxd/cmd/init.go) |
| `go_exported` | `FlagOverwrite = "overwrite"` | [daemon/paxd/cmd/init.go](../../../daemon/paxd/cmd/init.go) |
| `go_exported` | `FlagRecover = "recover"` | [daemon/paxd/cmd/init.go](../../../daemon/paxd/cmd/init.go) |
| `go_exported` | `func InitCmd(mbm module.BasicManager, defaultNodeHome string) *cobra.Command` | [daemon/paxd/cmd/init.go](../../../daemon/paxd/cmd/init.go) |
| `cli_command` | `Use: "get <logger>",` | [daemon/paxd/cmd/log.go](../../../daemon/paxd/cmd/log.go) |
| `cli_command` | `Use: "level",` | [daemon/paxd/cmd/log.go](../../../daemon/paxd/cmd/log.go) |
| `cli_command` | `Use: "list [prefix]",` | [daemon/paxd/cmd/log.go](../../../daemon/paxd/cmd/log.go) |
| `cli_command` | `Use: "log",` | [daemon/paxd/cmd/log.go](../../../daemon/paxd/cmd/log.go) |
| `cli_command` | `Use: "set <pattern> <level>",` | [daemon/paxd/cmd/log.go](../../../daemon/paxd/cmd/log.go) |
| `go_exported` | `func LogLevelCmd() *cobra.Command` | [daemon/paxd/cmd/log.go](../../../daemon/paxd/cmd/log.go) |
| `cli_command` | `cmd.PersistentFlags().String(flags.FlagChainID, "", "The network chain ID")` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `cli_command` | `Use: "paxd",` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `cli_command` | `Use: "query",` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `cli_command` | `Use: "tx",` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `env_var` | `PAX` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `go_exported` | `func NewRootCmd() (*cobra.Command, params.EncodingConfig)` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `go_exported` | `type Option func(*rootOptions)` | [daemon/paxd/cmd/root.go](../../../daemon/paxd/cmd/root.go) |
| `cli_command` | `cmd.Flags().String(flags.FlagChainID, "", "The network chain ID")` | [daemon/paxd/cmd/snapshot.go](../../../daemon/paxd/cmd/snapshot.go) |
| `cli_command` | `cmd.Flags().Int64("height", 0, "Height at which to create the snapshot (required)")` | [daemon/paxd/cmd/snapshot.go](../../../daemon/paxd/cmd/snapshot.go) |
| `cli_command` | `Use: "snapshot",` | [daemon/paxd/cmd/snapshot.go](../../../daemon/paxd/cmd/snapshot.go) |
| `go_exported` | `func SnapshotCmd() *cobra.Command` | [daemon/paxd/cmd/snapshot.go](../../../daemon/paxd/cmd/snapshot.go) |
| `cli_command` | `func main()` | [daemon/paxd/main.go](../../../daemon/paxd/main.go) |
| `cli_command` | `func main()` | [engine/executor/lib/gen/main.go](../../../engine/executor/lib/gen/main.go) |
| `cli_command` | `func main()` | [storage/db_engine/litt/benchmark/cmd/main.go](../../../storage/db_engine/litt/benchmark/cmd/main.go) |
| `cli_command` | `Name: "src",` | [storage/db_engine/litt/cli/litt_cli.go](../../../storage/db_engine/litt/cli/litt_cli.go) |
| `cli_command` | `Name: "force",` | [storage/db_engine/litt/cli/litt_cli.go](../../../storage/db_engine/litt/cli/litt_cli.go) |
| `cli_command` | `Name: "known-hosts",` | [storage/db_engine/litt/cli/litt_cli.go](../../../storage/db_engine/litt/cli/litt_cli.go) |
| `cli_command` | `Name: "litt",` | [storage/db_engine/litt/cli/litt_cli.go](../../../storage/db_engine/litt/cli/litt_cli.go) |
| `cli_command` | `Name: "benchmark",` | [storage/db_engine/litt/cli/litt_cli.go](../../../storage/db_engine/litt/cli/litt_cli.go) |
| `cli_command` | `Name: "debug",` | [storage/db_engine/litt/cli/litt_cli.go](../../../storage/db_engine/litt/cli/litt_cli.go) |
| `cli_command` | `Name: "pprof",` | [storage/db_engine/litt/cli/litt_cli.go](../../../storage/db_engine/litt/cli/litt_cli.go) |
| `cli_command` | `Name: "pprof-port",` | [storage/db_engine/litt/cli/litt_cli.go](../../../storage/db_engine/litt/cli/litt_cli.go) |
| `cli_command` | `Name: "ls",` | [storage/db_engine/litt/cli/litt_cli.go](../../../storage/db_engine/litt/cli/litt_cli.go) |
| `cli_command` | `Name: "prune",` | [storage/db_engine/litt/cli/litt_cli.go](../../../storage/db_engine/litt/cli/litt_cli.go) |
| `cli_command` | `Name: "max-age",` | [storage/db_engine/litt/cli/litt_cli.go](../../../storage/db_engine/litt/cli/litt_cli.go) |
| `cli_command` | `Name: "table",` | [storage/db_engine/litt/cli/litt_cli.go](../../../storage/db_engine/litt/cli/litt_cli.go) |
| `cli_command` | `Name: "push",` | [storage/db_engine/litt/cli/litt_cli.go](../../../storage/db_engine/litt/cli/litt_cli.go) |
| `cli_command` | `Name: "dst",` | [storage/db_engine/litt/cli/litt_cli.go](../../../storage/db_engine/litt/cli/litt_cli.go) |
| `cli_command` | `Name: "key",` | [storage/db_engine/litt/cli/litt_cli.go](../../../storage/db_engine/litt/cli/litt_cli.go) |
| `cli_command` | `Name: "no-gc",` | [storage/db_engine/litt/cli/litt_cli.go](../../../storage/db_engine/litt/cli/litt_cli.go) |
| `cli_command` | `Name: "port",` | [storage/db_engine/litt/cli/litt_cli.go](../../../storage/db_engine/litt/cli/litt_cli.go) |
| `cli_command` | `Name: "quiet",` | [storage/db_engine/litt/cli/litt_cli.go](../../../storage/db_engine/litt/cli/litt_cli.go) |
| `cli_command` | `Name: "threads",` | [storage/db_engine/litt/cli/litt_cli.go](../../../storage/db_engine/litt/cli/litt_cli.go) |
| `cli_command` | `Name: "throttle",` | [storage/db_engine/litt/cli/litt_cli.go](../../../storage/db_engine/litt/cli/litt_cli.go) |
| `cli_command` | `Name: "rebase",` | [storage/db_engine/litt/cli/litt_cli.go](../../../storage/db_engine/litt/cli/litt_cli.go) |
| `cli_command` | `Name: "preserve",` | [storage/db_engine/litt/cli/litt_cli.go](../../../storage/db_engine/litt/cli/litt_cli.go) |
| `cli_command` | `Name: "sync",` | [storage/db_engine/litt/cli/litt_cli.go](../../../storage/db_engine/litt/cli/litt_cli.go) |
| `cli_command` | `Name: "litt-binary",` | [storage/db_engine/litt/cli/litt_cli.go](../../../storage/db_engine/litt/cli/litt_cli.go) |
| `cli_command` | `Name: "period",` | [storage/db_engine/litt/cli/litt_cli.go](../../../storage/db_engine/litt/cli/litt_cli.go) |
| `cli_command` | `Name: "table-info",` | [storage/db_engine/litt/cli/litt_cli.go](../../../storage/db_engine/litt/cli/litt_cli.go) |
| `cli_command` | `Name: "unlock",` | [storage/db_engine/litt/cli/litt_cli.go](../../../storage/db_engine/litt/cli/litt_cli.go) |
| `cli_command` | `func main()` | [storage/db_engine/litt/cli/main.go](../../../storage/db_engine/litt/cli/main.go) |
| `cli_command` | `func main()` | [storage/ledger_db/block/blocksim/cmd/blocksim/main.go](../../../storage/ledger_db/block/blocksim/cmd/blocksim/main.go) |
| `cli_command` | `func main()` | [storage/ledger_db/block/blocksim/cmd/configure-logger/main.go](../../../storage/ledger_db/block/blocksim/cmd/configure-logger/main.go) |
| `cli_command` | `func main()` | [storage/state_db/bench/cryptosim/cmd/configure-logger/main.go](../../../storage/state_db/bench/cryptosim/cmd/configure-logger/main.go) |
| `cli_command` | `func main()` | [storage/state_db/bench/cryptosim/cmd/cryptosim/main.go](../../../storage/state_db/bench/cryptosim/cmd/cryptosim/main.go) |
| `cli_command` | `Use: "generate",` | [storage/tools/cmd/paxdb/benchmark/generate.go](../../../storage/tools/cmd/paxdb/benchmark/generate.go) |
| `cli_command` | `Use: "benchmark-iteration",` | [storage/tools/cmd/paxdb/benchmark/iteration.go](../../../storage/tools/cmd/paxdb/benchmark/iteration.go) |
| `cli_command` | `Use: "benchmark-read",` | [storage/tools/cmd/paxdb/benchmark/random_read.go](../../../storage/tools/cmd/paxdb/benchmark/random_read.go) |
| `cli_command` | `benchmarkReadCmd.PersistentFlags().StringP("db-backend", "d", "", "DB Backend")` | [storage/tools/cmd/paxdb/benchmark/random_read.go](../../../storage/tools/cmd/paxdb/benchmark/random_read.go) |
| `cli_command` | `benchmarkReadCmd.PersistentFlags().StringP("output-dir", "o", "", "Output Directory")` | [storage/tools/cmd/paxdb/benchmark/random_read.go](../../../storage/tools/cmd/paxdb/benchmark/random_read.go) |
| `cli_command` | `Use: "benchmark-reverse-iteration",` | [storage/tools/cmd/paxdb/benchmark/reverse_iteration.go](../../../storage/tools/cmd/paxdb/benchmark/reverse_iteration.go) |
| `cli_command` | `Use: "benchmark-write",` | [storage/tools/cmd/paxdb/benchmark/write.go](../../../storage/tools/cmd/paxdb/benchmark/write.go) |
| `cli_command` | `benchmarkWriteCmd.PersistentFlags().IntP("batch-size", "b", 1, "batch size for db writes")` | [storage/tools/cmd/paxdb/benchmark/write.go](../../../storage/tools/cmd/paxdb/benchmark/write.go) |
| `cli_command` | `benchmarkWriteCmd.PersistentFlags().StringP("db-backend", "d", "", "DB Backend")` | [storage/tools/cmd/paxdb/benchmark/write.go](../../../storage/tools/cmd/paxdb/benchmark/write.go) |
| `cli_command` | `benchmarkWriteCmd.PersistentFlags().StringP("output-dir", "o", "", "Output Directory")` | [storage/tools/cmd/paxdb/benchmark/write.go](../../../storage/tools/cmd/paxdb/benchmark/write.go) |
| `cli_command` | `Use: "paxdb",` | [storage/tools/cmd/paxdb/main.go](../../../storage/tools/cmd/paxdb/main.go) |
| `cli_command` | `Use: "dump-db",` | [storage/tools/cmd/paxdb/operations/dump_db.go](../../../storage/tools/cmd/paxdb/operations/dump_db.go) |
| `cli_command` | `Use: "dump-flatkv",` | [storage/tools/cmd/paxdb/operations/dump_flatkv.go](../../../storage/tools/cmd/paxdb/operations/dump_flatkv.go) |
| `cli_command` | `cmd.PersistentFlags().StringP("db-dir", "d", "", "FlatKV database directory")` | [storage/tools/cmd/paxdb/operations/dump_flatkv.go](../../../storage/tools/cmd/paxdb/operations/dump_flatkv.go) |
| `cli_command` | `Use: "dump-iavl",` | [storage/tools/cmd/paxdb/operations/dump_iavl.go](../../../storage/tools/cmd/paxdb/operations/dump_iavl.go) |
| `cli_command` | `cmd.PersistentFlags().StringP("db-dir", "d", "", "Database Directory")` | [storage/tools/cmd/paxdb/operations/dump_iavl.go](../../../storage/tools/cmd/paxdb/operations/dump_iavl.go) |
| `cli_command` | `cmd.PersistentFlags().Int64("height", 0, "Block Height")` | [storage/tools/cmd/paxdb/operations/dump_iavl.go](../../../storage/tools/cmd/paxdb/operations/dump_iavl.go) |
| `cli_command` | `cmd.PersistentFlags().StringP("output-dir", "o", "", "Output Directory")` | [storage/tools/cmd/paxdb/operations/dump_iavl.go](../../../storage/tools/cmd/paxdb/operations/dump_iavl.go) |
| `cli_command` | `cmd.Flags().Bool("force", false, "Overwrite existing committed FlatKV data")` | [storage/tools/cmd/paxdb/operations/import_flatkv_from_memiavl.go](../../../storage/tools/cmd/paxdb/operations/import_flatkv_from_memiavl.go) |
| `cli_command` | `cmd.Flags().Int64("height", 0, "memiavl version to import. 0 means latest")` | [storage/tools/cmd/paxdb/operations/import_flatkv_from_memiavl.go](../../../storage/tools/cmd/paxdb/operations/import_flatkv_from_memiavl.go) |
| `cli_command` | `cmd.Flags().String("home", "", "Pax home directory. Defaults to $HOME/.pax")` | [storage/tools/cmd/paxdb/operations/import_flatkv_from_memiavl.go](../../../storage/tools/cmd/paxdb/operations/import_flatkv_from_memiavl.go) |
| `cli_command` | `Use: "import-flatkv-from-memiavl",` | [storage/tools/cmd/paxdb/operations/import_flatkv_from_memiavl.go](../../../storage/tools/cmd/paxdb/operations/import_flatkv_from_memiavl.go) |
| `cli_command` | `Use: "memiavl-latest-version",` | [storage/tools/cmd/paxdb/operations/import_flatkv_from_memiavl.go](../../../storage/tools/cmd/paxdb/operations/import_flatkv_from_memiavl.go) |
| `cli_command` | `cmd.PersistentFlags().StringP("db-dir", "d", "", "FlatKV database directory")` | [storage/tools/cmd/paxdb/operations/migrate_evm_status.go](../../../storage/tools/cmd/paxdb/operations/migrate_evm_status.go) |
| `cli_command` | `Use: "migrate-evm-status",` | [storage/tools/cmd/paxdb/operations/migrate_evm_status.go](../../../storage/tools/cmd/paxdb/operations/migrate_evm_status.go) |
| `cli_command` | `Use: "prune",` | [storage/tools/cmd/paxdb/operations/prune.go](../../../storage/tools/cmd/paxdb/operations/prune.go) |
| `cli_command` | `Use: "replay-changelog",` | [storage/tools/cmd/paxdb/operations/replay_changelog.go](../../../storage/tools/cmd/paxdb/operations/replay_changelog.go) |
| `cli_command` | `cmd.PersistentFlags().String("aws-region", "us-east-2", "AWS region for DynamoDB")` | [storage/tools/cmd/paxdb/operations/state_size.go](../../../storage/tools/cmd/paxdb/operations/state_size.go) |
| `cli_command` | `cmd.PersistentFlags().StringP("db-dir", "d", "", "memIAVL database directory")` | [storage/tools/cmd/paxdb/operations/state_size.go](../../../storage/tools/cmd/paxdb/operations/state_size.go) |
| `cli_command` | `cmd.PersistentFlags().Int64("height", 0, "Block Height")` | [storage/tools/cmd/paxdb/operations/state_size.go](../../../storage/tools/cmd/paxdb/operations/state_size.go) |
| `cli_command` | `Use: "state-size",` | [storage/tools/cmd/paxdb/operations/state_size.go](../../../storage/tools/cmd/paxdb/operations/state_size.go) |
| `cli_command` | `cmd.Flags().Int64Var(&endBlock, "end-block", 0, "Ending block number")` | [storage/tools/cmd/paxdb/operations/trace_profile_report.go](../../../storage/tools/cmd/paxdb/operations/trace_profile_report.go) |
| `cli_command` | `cmd.Flags().Int64Var(&startBlock, "start-block", 0, "Starting block number")` | [storage/tools/cmd/paxdb/operations/trace_profile_report.go](../../../storage/tools/cmd/paxdb/operations/trace_profile_report.go) |
| `cli_command` | `Use: "trace-profile-report",` | [storage/tools/cmd/paxdb/operations/trace_profile_report.go](../../../storage/tools/cmd/paxdb/operations/trace_profile_report.go) |
| `cli_command` | `func main()` | [storage/tools/rpc_bench/main.go](../../../storage/tools/rpc_bench/main.go) |
| `cli_command` | `flag.IntVar(&blockCount, "blocks", 20, "number of recent blocks to sample")` | [storage/tools/rpc_bench/main.go](../../../storage/tools/rpc_bench/main.go) |
| `cli_command` | `flag.IntVar(&concurrency, "concurrency", 16, "number of concurrent workers")` | [storage/tools/rpc_bench/main.go](../../../storage/tools/rpc_bench/main.go) |
| `cli_command` | `flag.StringVar(&endpoint, "endpoint", "", "RPC endpoint URL (required)")` | [storage/tools/rpc_bench/main.go](../../../storage/tools/rpc_bench/main.go) |
| `cli_command` | `flag.IntVar(&requestsPer, "requests", 100, "requests per method per phase")` | [storage/tools/rpc_bench/main.go](../../../storage/tools/rpc_bench/main.go) |

## Node exported Go API

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func (app *App) RegisterLocalServices(node client.LocalClient, txConfig client.TxConfig)` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) Sequenced(ctx sdk.Context, height int64) bool` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) SetOpenBlock(ob OpenBlock)` | [node/app.go](../../../node/app.go) |
| `go_exported` | `type OpenBlock interface` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (pk *PrecompileKeepers) AnchorK() putils.AnchorKeeper {` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `func (app *App) SequencerSource() tmtypes.SequencerSource` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) SetCommitListener(fn func(height int64, appHash []byte) error) {` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) SetSequencerRPC(opt evmrpc.SequencerOption, rcpt evmrpc.SequencerReceiptOption) {` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) SetSequencerSource(src tmtypes.SequencerSource)` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (pk *PrecompileKeepers) LayerXAuthK() *layerxauthkeeper.Keeper` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `func (pk *PrecompileKeepers) LayerXLedgerK() *layerxledgerkeeper.Keeper` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `func GetCachedContext(ctx sdk.Context) (sdk.Context, sdk.CacheMultiStore)` | [modules/store/handler.go](../../../modules/store/handler.go) |
| `go_exported` | `type App struct` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) AppCodec() codec.Codec` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) BlacklistedAccAddrs() map[string]bool` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) CacheContext(ctx sdk.Context) (sdk.Context, sdk.CacheMultiStore)` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) ClearOptimisticProcessingInfo()` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) Close() error` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) DecodeTransactionsConcurrently(ctx sdk.Context, txs [][]byte) []sdk.Tx` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) DecodeTxBytesConcurrently(ctx context.Context, txs [][]byte) ([]sdk.Tx, error) {` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) DeliverTxWithResult(ctx sdk.Context, tx []byte, typedTx sdk.Tx) *abci.ExecTxResult {` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) ExecuteTxsConcurrently(ctx sdk.Context, txs [][]byte, typedTxs []sdk.Tx, firstTx int) ([]*abci.ExecTxResult, sdk.Context, error) {` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) FinalizeDecodedTransactionsConcurrently(ctx sdk.Context, typedTxs []sdk.Tx) {` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) GetBaseApp() *baseapp.BaseApp { return app.BaseApp }` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) GetDeliverTxEntry(ctx sdk.Context, txIndex int, bz []byte, tx sdk.Tx) (res *sdk.DeliverTxEntry) {` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) GetEVMMsg(tx sdk.Tx) (res *evmtypes.MsgEVMTransaction)` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) GetKey(storeKey string) *sdk.KVStoreKey` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) GetLegacyTxConfig() client.TxConfig` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) GetMemKey(storeKey string) *sdk.MemoryStoreKey` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) GetOptimisticProcessingInfo() OptimisticProcessingInfo` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) GetPrecompileKeepers() putils.Keepers` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) GetStateStore() paxdb.StateStore { return app.stateStore }` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) GetSubspace(moduleName string) paramstypes.Subspace` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) GetTKey(storeKey string) *sdk.TransientStoreKey` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) GetTxConfig() client.TxConfig` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) GetValidators() []abci.ValidatorUpdate` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) HandleClose() error` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) HandlePreCommit(ctx sdk.Context) error` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) InterfaceRegistry() types.InterfaceRegistry` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) LatestCtx() sdk.Context` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) LegacyAmino() *codec.LegacyAmino` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) LoadHeight(height int64) error` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) ModuleAccountAddrs() map[string]bool` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) Name() string { return app.BaseApp.Name() }` | [node/app.go](../../../node/app.go) |
| `go_exported` | `type AppOption func(*App)` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) PendingSnapshot() (sdk.Context, error)` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) ProcessTXsWithOCCGiga(ctx sdk.Context, txs [][]byte, typedTxs []sdk.Tx, firstTx int) ([]*abci.ExecTxResult, sdk.Context, error) {` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) ProcessTXsWithOCCV2(ctx sdk.Context, txs [][]byte, typedTxs []sdk.Tx, firstTx int) ([]*abci.ExecTxResult, sdk.Context) {` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) ProcessTxsSynchronousGiga(ctx sdk.Context, txs [][]byte, typedTxs []sdk.Tx, firstTx int) ([]*abci.ExecTxResult, error) {` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) ProcessTxsSynchronousV2(ctx sdk.Context, txs [][]byte, typedTxs []sdk.Tx, firstTx int) []*abci.ExecTxResult {` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) RegisterAPIRoutes(apiSvr *api.Server, apiConfig config.APIConfig)` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) RPCContextProvider(i int64) sdk.Context` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) SequencedTxRoot(txs [][]byte, typedTxs []sdk.Tx) [32]byte` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) SetStoreUpgradeHandlers()` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) SetTxDecoder(txDecoder sdk.TxDecoder)` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) SimulationManager() *module.SimulationManager` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func (app *App) SnapshotAwareRPCContextProvider() evmrpc.TraceContextProvider` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func DecodeSequenceBatch(tx []byte) (seqtypes.MsgSequenceBatch, error)` | [node/app.go](../../../node/app.go) |
| `go_exported` | `DefaultNodeHome string` | [node/app.go](../../../node/app.go) |
| `go_exported` | `EmittedPaxdVersionMetric = false` | [node/app.go](../../../node/app.go) |
| `go_exported` | `EmptyAppOptions []AppOption` | [node/app.go](../../../node/app.go) |
| `go_exported` | `EmptyWasmOpts []wasm.Option` | [node/app.go](../../../node/app.go) |
| `go_exported` | `EnableOCC = true` | [node/app.go](../../../node/app.go) |
| `go_exported` | `EnableSpecificWasmProposals = ""` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func EncodeSequenceBatch(m seqtypes.MsgSequenceBatch) ([]byte, error)` | [node/app.go](../../../node/app.go) |
| `go_exported` | `var ErrPendingServedBySequencer = errors.New("pending served by sequencer")` | [node/app.go](../../../node/app.go) |
| `go_exported` | `const EVMReceiptModuleID = lxpModuleReservedCount + 1` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func GetMaccPerms() map[string][]string` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func GetWasmEnabledProposals() []wasm.ProposalType` | [node/app.go](../../../node/app.go) |
| `go_exported` | `MinGasEVMTx = 21000` | [node/app.go](../../../node/app.go) |
| `go_exported` | `ModuleBasics = module.NewBasicManager(` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func New(` | [node/app.go](../../../node/app.go) |
| `go_exported` | `NewHeadsNotifierCapacity = 1` | [node/app.go](../../../node/app.go) |
| `go_exported` | `const PendingCtxHeight int64 = evmrpc.LatestCtxHeight - 1` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func RegisterSwaggerAPI(rtr *mux.Router)` | [node/app.go](../../../node/app.go) |
| `go_exported` | `WasmProposalsEnabled = "true"` | [node/app.go](../../../node/app.go) |
| `go_exported` | `func CreateRandomAccounts(numAccts int) []sdk.AccAddress {` | [node/apptesting/test_suite.go](../../../node/apptesting/test_suite.go) |
| `go_exported` | `func FundAccount(bankKeeper bankKeeper, ctx sdk.Context, addr sdk.AccAddress, amounts sdk.Coins) error {` | [node/apptesting/test_suite.go](../../../node/apptesting/test_suite.go) |
| `go_exported` | `func FundModuleAccount(bankKeeper bankKeeper, ctx sdk.Context, recipientMod string, amounts sdk.Coins) error {` | [node/apptesting/test_suite.go](../../../node/apptesting/test_suite.go) |
| `go_exported` | `func GenerateTestAddrs() (string, string) {` | [node/apptesting/test_suite.go](../../../node/apptesting/test_suite.go) |
| `go_exported` | `type KeeperTestHelper struct` | [node/apptesting/test_suite.go](../../../node/apptesting/test_suite.go) |
| `go_exported` | `func (s *KeeperTestHelper) AllocateRewardsToValidator(valAddr sdk.ValAddress, rewardAmt sdk.Int) {` | [node/apptesting/test_suite.go](../../../node/apptesting/test_suite.go) |
| `go_exported` | `func (s *KeeperTestHelper) BuildTx(` | [node/apptesting/test_suite.go](../../../node/apptesting/test_suite.go) |
| `go_exported` | `func (s *KeeperTestHelper) Commit() {` | [node/apptesting/test_suite.go](../../../node/apptesting/test_suite.go) |
| `go_exported` | `func (s *KeeperTestHelper) CreateTestContext() sdk.Context {` | [node/apptesting/test_suite.go](../../../node/apptesting/test_suite.go) |
| `go_exported` | `func (s *KeeperTestHelper) EndBlock() {` | [node/apptesting/test_suite.go](../../../node/apptesting/test_suite.go) |
| `go_exported` | `func (s *KeeperTestHelper) FundAcc(acc sdk.AccAddress, amounts sdk.Coins) {` | [node/apptesting/test_suite.go](../../../node/apptesting/test_suite.go) |
| `go_exported` | `func (s *KeeperTestHelper) Setup() {` | [node/apptesting/test_suite.go](../../../node/apptesting/test_suite.go) |
| `go_exported` | `func (s *KeeperTestHelper) SetupTokenFactory() {` | [node/apptesting/test_suite.go](../../../node/apptesting/test_suite.go) |
| `go_exported` | `func (s *KeeperTestHelper) SetupValidator(bondStatus stakingtypes.BondStatus) sdk.ValAddress {` | [node/apptesting/test_suite.go](../../../node/apptesting/test_suite.go) |
| `go_exported` | `func (app *App) BenchmarkLogger() *benchmark.Logger` | [node/benchmark.go](../../../node/benchmark.go) |
| `go_exported` | `func (app *App) EndBenchmarkBlockProcessing()` | [node/benchmark.go](../../../node/benchmark.go) |
| `go_exported` | `func (app *App) InitBenchmark(ctx context.Context, chainID string, evmChainID int64)` | [node/benchmark.go](../../../node/benchmark.go) |
| `go_exported` | `func (app *App) ProcessBenchmarkReceipts(ctx sdk.Context)` | [node/benchmark.go](../../../node/benchmark.go) |
| `go_exported` | `func (app *App) RecordBenchmarkCommitTime(duration time.Duration)` | [node/benchmark.go](../../../node/benchmark.go) |
| `go_exported` | `func (app *App) StartBenchmarkBlockProcessing()` | [node/benchmark.go](../../../node/benchmark.go) |
| `go_exported` | `type Manager struct` | [node/benchmark/benchmark.go](../../../node/benchmark/benchmark.go) |
| `go_exported` | `func (m *Manager) ProposalChannel() <-chan [][]byte {` | [node/benchmark/benchmark.go](../../../node/benchmark/benchmark.go) |
| `go_exported` | `func NewManager(ctx context.Context, txConfig client.TxConfig, chainID string, evmChainID int64) (*Manager, error) {` | [node/benchmark/benchmark.go](../../../node/benchmark/benchmark.go) |
| `go_exported` | `func LoadConfig(configPath string, evmChainID int64, paxChainID string) (*config.LoadConfig, error) {` | [node/benchmark/config.go](../../../node/benchmark/config.go) |
| `go_exported` | `func (g *Generator) Generate() [][]byte {` | [node/benchmark/generator.go](../../../node/benchmark/generator.go) |
| `go_exported` | `type Generator struct {` | [node/benchmark/generator.go](../../../node/benchmark/generator.go) |
| `go_exported` | `func (g *Generator) StartProposalChannel(ctx context.Context, benchLogger *Logger) <-chan [][]byte {` | [node/benchmark/generator.go](../../../node/benchmark/generator.go) |
| `go_exported` | `func NewGenerator(cfg *config.LoadConfig, txConfig client.TxConfig) (*Generator, error) {` | [node/benchmark/generator.go](../../../node/benchmark/generator.go) |
| `go_exported` | `type Phase int` | [node/benchmark/generator.go](../../../node/benchmark/generator.go) |
| `go_exported` | `PhaseLoad` | [node/benchmark/generator.go](../../../node/benchmark/generator.go) |
| `go_exported` | `PhaseSetup` | [node/benchmark/generator.go](../../../node/benchmark/generator.go) |
| `go_exported` | `PhaseWarmup Phase = iota` | [node/benchmark/generator.go](../../../node/benchmark/generator.go) |
| `go_exported` | `const WarmupBlocks = 3` | [node/benchmark/generator.go](../../../node/benchmark/generator.go) |
| `go_exported` | `type Logger struct` | [node/benchmark/logger.go](../../../node/benchmark/logger.go) |
| `go_exported` | `func (l *Logger) EndBlockProcessing() {` | [node/benchmark/logger.go](../../../node/benchmark/logger.go) |
| `go_exported` | `func (l *Logger) FlushLog() {` | [node/benchmark/logger.go](../../../node/benchmark/logger.go) |
| `go_exported` | `func (l *Logger) Increment(count int64, blocktime time.Time, height int64) {` | [node/benchmark/logger.go](../../../node/benchmark/logger.go) |
| `go_exported` | `func (l *Logger) RecordCommitTime(duration time.Duration) {` | [node/benchmark/logger.go](../../../node/benchmark/logger.go) |
| `go_exported` | `func (l *Logger) StartBlockProcessing() {` | [node/benchmark/logger.go](../../../node/benchmark/logger.go) |
| `go_exported` | `func (l *Logger) Start(ctx context.Context) {` | [node/benchmark/logger.go](../../../node/benchmark/logger.go) |
| `go_exported` | `func NewLogger() *Logger` | [node/benchmark/logger.go](../../../node/benchmark/logger.go) |
| `go_exported` | `AccountAddressPrefix = "pax"` | [node/const.go](../../../node/const.go) |
| `go_exported` | `AppName = "paxeer"` | [node/const.go](../../../node/const.go) |
| `go_exported` | `OptimisticProcessingTimeoutInSeconds = 5` | [node/const.go](../../../node/const.go) |
| `go_exported` | `func MakeEncodingConfig() params.EncodingConfig` | [node/encoding.go](../../../node/encoding.go) |
| `go_exported` | `func MakeLegacyEncodingConfig() params.EncodingConfig` | [node/encoding.go](../../../node/encoding.go) |
| `go_exported` | `func BlockTest(a *App, bt *ethtests.BlockTest, home string)` | [node/eth_replay.go](../../../node/eth_replay.go) |
| `go_exported` | `func IsWithdrawalAddress(addr common.Address, blocks []*ethtypes.Block) bool` | [node/eth_replay.go](../../../node/eth_replay.go) |
| `go_exported` | `func Replay(a *App)` | [node/eth_replay.go](../../../node/eth_replay.go) |
| `go_exported` | `func AddressFromValidatorsKey(key []byte) []byte` | [node/export.go](../../../node/export.go) |
| `go_exported` | `func (app *App) ExportAppStateAndValidators(` | [node/export.go](../../../node/export.go) |
| `go_exported` | `func (app *App) ExportAppToFileStateAndValidators(` | [node/export.go](../../../node/export.go) |
| `go_exported` | `const MockBalancesEnabled = true` | [node/mock_balances_buildtag.go](../../../node/mock_balances_buildtag.go) |
| `go_exported` | `const MockBalancesEnabled = false` | [node/mock_balances_buildtag_default.go](../../../node/mock_balances_buildtag_default.go) |
| `go_exported` | `func MakeEncodingConfig() EncodingConfig` | [node/params/amino.go](../../../node/params/amino.go) |
| `go_exported` | `func MakeLegacyEncodingConfig() EncodingConfig` | [node/params/amino.go](../../../node/params/amino.go) |
| `go_exported` | `BaseCoinUnit = "uhpx"` | [node/params/config.go](../../../node/params/config.go) |
| `go_exported` | `Bech32PrefixAccAddr = "pax"` | [node/params/config.go](../../../node/params/config.go) |
| `go_exported` | `Bech32PrefixAccPub = Bech32PrefixAccAddr + "pub"` | [node/params/config.go](../../../node/params/config.go) |
| `go_exported` | `Bech32PrefixConsAddr = Bech32PrefixAccAddr + "valcons"` | [node/params/config.go](../../../node/params/config.go) |
| `go_exported` | `Bech32PrefixConsPub = Bech32PrefixAccAddr + "valconspub"` | [node/params/config.go](../../../node/params/config.go) |
| `go_exported` | `Bech32PrefixValAddr = Bech32PrefixAccAddr + "valoper"` | [node/params/config.go](../../../node/params/config.go) |
| `go_exported` | `Bech32PrefixValPub = Bech32PrefixAccAddr + "valoperpub"` | [node/params/config.go](../../../node/params/config.go) |
| `go_exported` | `DefaultBondDenom = BaseCoinUnit` | [node/params/config.go](../../../node/params/config.go) |
| `go_exported` | `HumanCoinUnit = "hpx"` | [node/params/config.go](../../../node/params/config.go) |
| `go_exported` | `type NodeMode string` | [node/params/config.go](../../../node/params/config.go) |
| `go_exported` | `NodeModeArchive NodeMode = "archive"` | [node/params/config.go](../../../node/params/config.go) |
| `go_exported` | `NodeModeFull NodeMode = tmcfg.ModeFull // "full"` | [node/params/config.go](../../../node/params/config.go) |
| `go_exported` | `func (m NodeMode) IsFullnodeType() bool` | [node/params/config.go](../../../node/params/config.go) |
| `go_exported` | `NodeModeSeed NodeMode = tmcfg.ModeSeed // "seed"` | [node/params/config.go](../../../node/params/config.go) |
| `go_exported` | `NodeModeValidator NodeMode = tmcfg.ModeValidator // "validator"` | [node/params/config.go](../../../node/params/config.go) |
| `go_exported` | `func RegisterDenoms()` | [node/params/config.go](../../../node/params/config.go) |
| `go_exported` | `func SetAddressPrefixes()` | [node/params/config.go](../../../node/params/config.go) |
| `go_exported` | `func SetAppConfigByMode(config *srvconfig.Config, mode NodeMode)` | [node/params/config.go](../../../node/params/config.go) |
| `go_exported` | `func SetEVMConfigByMode(config *evmrpcconfig.Config, mode NodeMode)` | [node/params/config.go](../../../node/params/config.go) |
| `go_exported` | `func SetTendermintConfigByMode(config *tmcfg.Config)` | [node/params/config.go](../../../node/params/config.go) |
| `go_exported` | `UhpxExponent = 6` | [node/params/config.go](../../../node/params/config.go) |
| `go_exported` | `var UnsafeBypassCommitTimeoutOverride = false` | [node/params/config.go](../../../node/params/config.go) |
| `go_exported` | `type EncodingConfig struct` | [node/params/encoding.go](../../../node/params/encoding.go) |
| `go_exported` | `InitiallyBondedValidators = "initially_bonded_validators"` | [node/params/params.go](../../../node/params/params.go) |
| `go_exported` | `StakePerAccount = "stake_per_account"` | [node/params/params.go](../../../node/params/params.go) |
| `go_exported` | `func MakeEncodingConfig() EncodingConfig` | [node/params/proto.go](../../../node/params/proto.go) |
| `go_exported` | `func MakeLegacyEncodingConfig() EncodingConfig` | [node/params/proto.go](../../../node/params/proto.go) |
| `go_exported` | `DefaultWeightCommunitySpendProposal int = 5` | [node/params/weights.go](../../../node/params/weights.go) |
| `go_exported` | `DefaultWeightGrantAllowance int = 100` | [node/params/weights.go](../../../node/params/weights.go) |
| `go_exported` | `DefaultWeightMsgBeginRedelegate int = 100` | [node/params/weights.go](../../../node/params/weights.go) |
| `go_exported` | `DefaultWeightMsgCreateValidator int = 100` | [node/params/weights.go](../../../node/params/weights.go) |
| `go_exported` | `DefaultWeightMsgDelegate int = 100` | [node/params/weights.go](../../../node/params/weights.go) |
| `go_exported` | `DefaultWeightMsgDeposit int = 100` | [node/params/weights.go](../../../node/params/weights.go) |
| `go_exported` | `DefaultWeightMsgEditValidator int = 5` | [node/params/weights.go](../../../node/params/weights.go) |
| `go_exported` | `DefaultWeightMsgFundCommunityPool int = 50` | [node/params/weights.go](../../../node/params/weights.go) |
| `go_exported` | `DefaultWeightMsgMultiSend int = 10` | [node/params/weights.go](../../../node/params/weights.go) |
| `go_exported` | `DefaultWeightMsgSend int = 100` | [node/params/weights.go](../../../node/params/weights.go) |
| `go_exported` | `DefaultWeightMsgSetWithdrawAddress int = 50` | [node/params/weights.go](../../../node/params/weights.go) |
| `go_exported` | `DefaultWeightMsgUndelegate int = 100` | [node/params/weights.go](../../../node/params/weights.go) |
| `go_exported` | `DefaultWeightMsgUnjail int = 100` | [node/params/weights.go](../../../node/params/weights.go) |
| `go_exported` | `DefaultWeightMsgVote int = 67` | [node/params/weights.go](../../../node/params/weights.go) |
| `go_exported` | `DefaultWeightMsgVoteWeighted int = 33` | [node/params/weights.go](../../../node/params/weights.go) |
| `go_exported` | `DefaultWeightMsgWithdrawDelegationReward int = 50` | [node/params/weights.go](../../../node/params/weights.go) |
| `go_exported` | `DefaultWeightMsgWithdrawValidatorCommission int = 50` | [node/params/weights.go](../../../node/params/weights.go) |
| `go_exported` | `DefaultWeightParamChangeProposal int = 5` | [node/params/weights.go](../../../node/params/weights.go) |
| `go_exported` | `DefaultWeightRevokeAllowance int = 100` | [node/params/weights.go](../../../node/params/weights.go) |
| `go_exported` | `DefaultWeightTextProposal int = 5` | [node/params/weights.go](../../../node/params/weights.go) |
| `go_exported` | `func NewPrecompileKeepers(a *App) *PrecompileKeepers` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `type PrecompileKeepers struct` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `func (pk *PrecompileKeepers) AccountK() putils.AccountKeeper { return pk.AccountKeeper }` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `func (pk *PrecompileKeepers) BankK() putils.BankKeeper { return pk.BankKeeper }` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `func (pk *PrecompileKeepers) BankMS() putils.BankMsgServer { return pk.BankMsgServer }` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `func (pk *PrecompileKeepers) ChannelK() putils.ChannelKeeper { return pk.ChannelKeeper }` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `func (pk *PrecompileKeepers) ClientK() putils.ClientKeeper { return pk.ClientKeeper }` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `ConnectionK` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `DistributionK` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `func (pk *PrecompileKeepers) EVMK() putils.EVMKeeper { return pk.EVMKeeper }` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `func (pk *PrecompileKeepers) GovK() putils.GovKeeper { return pk.GovKeeper }` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `func (pk *PrecompileKeepers) GovMS() putils.GovMsgServer { return pk.GovMsgServer }` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `func (pk *PrecompileKeepers) LaunchpadK() *launchpadkeeper.Keeper` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `func (pk *PrecompileKeepers) LaunchpadV610K() *launchpadv610keeper.Keeper` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `func (pk *PrecompileKeepers) LayerXBridgeK() *layerxbridgekeeper.Keeper` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `func (pk *PrecompileKeepers) LayerXCustodyK() *layerxcustodykeeper.Keeper` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `func (pk *PrecompileKeepers) LayerXExchangeK() *layerxexchangekeeper.Keeper` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `func (pk *PrecompileKeepers) OracleK() putils.OracleKeeper { return pk.OracleKeeper }` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `func (pk *PrecompileKeepers) PerpsK() *perpskeeper.Keeper` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `func (pk *PrecompileKeepers) PriceFeedK() *pricefeedkeeper.Keeper` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `func (pk *PrecompileKeepers) SpotK() *spotkeeper.Keeper` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `func (pk *PrecompileKeepers) StakingK() putils.StakingKeeper { return pk.StakingKeeper }` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `func (pk *PrecompileKeepers) StakingQ() putils.StakingQuerier { return pk.StakingQuerier }` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `TransferK` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `func (pk *PrecompileKeepers) TxConfig() client.TxConfig { return pk.txConf }` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `func (pk *PrecompileKeepers) WasmdK() putils.WasmdKeeper { return pk.WasmdKeeper }` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `WasmdVK` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `func (pk *PrecompileKeepers) XWebK() *xwebkeeper.Keeper` | [node/precompiles.go](../../../node/precompiles.go) |
| `go_exported` | `func NewPaxTxPrioritizer(ek *evmkeeper.Keeper, uk *upgradekeeper.Keeper, pk *paramskeeper.Keeper) *PaxTxPrioritizer {` | [node/prioritizer.go](../../../node/prioritizer.go) |
| `go_exported` | `type PaxTxPrioritizer struct` | [node/prioritizer.go](../../../node/prioritizer.go) |
| `go_exported` | `func (s *PaxTxPrioritizer) GetTxPriorityHint(ctx sdk.Context, tx sdk.Tx) (_priorityHint int64, _err error) {` | [node/prioritizer.go](../../../node/prioritizer.go) |
| `go_exported` | `type Action struct` | [node/receipt.go](../../../node/receipt.go) |
| `go_exported` | `type AllowanceResponse struct` | [node/receipt.go](../../../node/receipt.go) |
| `go_exported` | `func (app *App) AddCosmosEventsToEVMReceiptIfApplicable(ctx sdk.Context, tx sdk.Tx, checksum [32]byte, response sdk.DeliverTxHookInput) error {` | [node/receipt.go](../../../node/receipt.go) |
| `go_exported` | `func (app *App) GetActionsFromWasmEvent(ctx sdk.Context, event abci.Event) ([]*Action, error) {` | [node/receipt.go](../../../node/receipt.go) |
| `go_exported` | `func (app *App) GetEvmAddressHash(ctx sdk.Context, addrStr string) (common.Hash, error)` | [node/receipt.go](../../../node/receipt.go) |
| `go_exported` | `var EmptyHash = common.HexToHash("0x0")` | [node/receipt.go](../../../node/receipt.go) |
| `go_exported` | `var ERC1155ApprovalForAllTopic = common.HexToHash("0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31")` | [node/receipt.go](../../../node/receipt.go) |
| `go_exported` | `var ERC1155TransferBatchTopic = common.HexToHash("0x4a39dc06d4c0dbc64b70af90fd698a233a518aa5d07e595d983b8c0526c8f7fb")` | [node/receipt.go](../../../node/receipt.go) |
| `go_exported` | `var ERC1155TransferSingleTopic = common.HexToHash("0xc3d58168c5ae7397731d063d5bbf3d657854427343f4c083240f7aacaa2d0f62")` | [node/receipt.go](../../../node/receipt.go) |
| `go_exported` | `var ERC1155URITopic = common.HexToHash("0x6bb7ff708619ba0610cba295a58592e0451dee2622938c8755667688daf3529b")` | [node/receipt.go](../../../node/receipt.go) |
| `go_exported` | `var ERC20ApprovalTopic = common.HexToHash("0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925")` | [node/receipt.go](../../../node/receipt.go) |
| `go_exported` | `var ERC20TransferTopic = common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")` | [node/receipt.go](../../../node/receipt.go) |
| `go_exported` | `var ERC721ApprovalTopic = common.HexToHash("0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925")` | [node/receipt.go](../../../node/receipt.go) |
| `go_exported` | `var ERC721ApproveAllTopic = common.HexToHash("0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31")` | [node/receipt.go](../../../node/receipt.go) |
| `go_exported` | `var ERC721TransferTopic = common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")` | [node/receipt.go](../../../node/receipt.go) |
| `go_exported` | `var ErrSyntheticReceiptTranslation = errors.New("synthetic EVM receipt translation failed")` | [node/receipt.go](../../../node/receipt.go) |
| `go_exported` | `func GetAttributeValue(event abci.Event, attribute string) (string, bool)` | [node/receipt.go](../../../node/receipt.go) |
| `go_exported` | `func GetEventsOfType(rdtx sdk.DeliverTxHookInput, ty string) (res []abci.Event)` | [node/receipt.go](../../../node/receipt.go) |
| `go_exported` | `var TrueHash = common.HexToHash("0x1")` | [node/receipt.go](../../../node/receipt.go) |
| `go_exported` | `func AddTestAddrs(app *App, ctx sdk.Context, accNum int, accAmt sdk.Int) []sdk.AccAddress` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func AddTestAddrsFromPubKeys(app *App, ctx sdk.Context, pubKeys []cryptotypes.PubKey, accAmt sdk.Int) {` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func AddTestAddrsIncremental(app *App, ctx sdk.Context, accNum int, accAmt sdk.Int) []sdk.AccAddress {` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func CheckBalance(t *testing.T, app *App, addr sdk.AccAddress, balances sdk.Coins)` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func ConvertAddrsToValAddrs(addrs []sdk.AccAddress) []sdk.ValAddress` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func CreateTestPubKeys(numPubKeys int) []cryptotypes.PubKey` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `var DefaultConsensusParams = &tmproto.ConsensusParams` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `const DefaultGenTxGas = 10000000` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func GenSequenceOfTxs(txGen client.TxConfig, msgs []sdk.Msg, accNums []uint64, initSeqNums []uint64, numToGenerate int, priv ...cryptotypes.PrivKey) ([]sdk.Tx, error) {` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func GenTx(gen client.TxConfig, msgs []sdk.Msg, feeAmt sdk.Coins, gas uint64, chainID string, accNums, accSeqs []uint64, priv ...cryptotypes.PrivKey) (sdk.Tx, error) {` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `type GenerateAccountStrategy func(int) []sdk.AccAddress` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func NewGigaTestWrapper(t *testing.T, tm time.Time, valPub cryptotypes.PubKey, enableEVMCustomPrecompiles bool, useOcc bool, baseAppOptions ...func(*bam.BaseApp)) *TestWrapper {` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func NewGigaTestWrapperWithRegularStore(t *testing.T, tm time.Time, valPub cryptotypes.PubKey, enableEVMCustomPrecompiles bool, useOcc bool, baseAppOptions ...func(*bam.BaseApp)) *TestWrapper {` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func NewPubKeyFromHex(pk string) (res cryptotypes.PubKey)` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func NewTestTx(msgs []sdk.Msg) TestTx` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func NewTestWrapper(tb testing.TB, tm time.Time, valPub cryptotypes.PubKey, enableEVMCustomPrecompiles bool, baseAppOptions ...func(*bam.BaseApp)) *TestWrapper {` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func NewTestWrapperWithSc(t *testing.T, tm time.Time, valPub cryptotypes.PubKey, enableEVMCustomPrecompiles bool, baseAppOptions ...func(*bam.BaseApp)) *TestWrapper {` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func Setup(tb testing.TB, isCheckTx bool, enableEVMCustomPrecompiles bool, overrideWasmGasMultiplier bool, baseAppOptions ...func(*bam.BaseApp)) (res *App) {` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func SetupTestingAppWithLevelDb(t *testing.T, isCheckTx bool, enableEVMCustomPrecompiles bool) (*App, func()) {` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func SetupWithAppOptsAndDefaultHome(isCheckTx bool, appOpts TestAppOpts, enableEVMCustomPrecompiles bool, overrideWasmGasMultiplier bool, baseAppOptions ...func(*bam.BaseApp)) (res *App) {` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func SetupWithDB(tb testing.TB, db dbm.DB, isCheckTx bool, enableEVMCustomPrecompiles bool, overrideWasmGasMultiplier bool, baseAppOptions ...func(*bam.BaseApp)) (res *App) {` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func SetupWithDefaultHome(isCheckTx bool, enableEVMCustomPrecompiles bool, overrideWasmGasMultiplier bool, baseAppOptions ...func(*bam.BaseApp)) (res *App) {` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func SetupWithGenesisAccounts(t *testing.T, genAccs []authtypes.GenesisAccount, balances ...banktypes.Balance) *App {` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func SetupWithGenesisValSet(t *testing.T, valSet *tmtypes.ValidatorSet, genAccs []authtypes.GenesisAccount, balances ...banktypes.Balance) *App {` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func SetupWithSc(t *testing.T, isCheckTx bool, enableEVMCustomPrecompiles bool, testAppOpts TestAppOpts, baseAppOptions ...func(*bam.BaseApp)) (res *App) {` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func SetupWithScReceiptFromOpts(t *testing.T, isCheckTx bool, enableEVMCustomPrecompiles bool, testAppOpts TestAppOpts, baseAppOptions ...func(*bam.BaseApp)) (res *App) {` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func SignCheckDeliver(` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func TestAddr(addr string, bech string) (sdk.AccAddress, error)` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `type TestAppOpts struct` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func (t TestAppOpts) Get(s string) interface{}` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `const TestContract = "TEST"` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `type TestTx struct` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func (t TestTx) GetGasEstimate() uint64` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func (t TestTx) GetMsgs() []sdk.Msg` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func (t TestTx) ValidateBasic() error` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `const TestUser = "pax1jdppe6fnj2q7hjsepty5crxtrryzhuqsrq7tpd"` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `type TestWrapper struct` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func (s *TestWrapper) BeginBlock()` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func (s *TestWrapper) EndBlock()` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `func (s *TestWrapper) FundAcc(acc sdk.AccAddress, amounts sdk.Coins)` | [node/test_helpers.go](../../../node/test_helpers.go) |
| `go_exported` | `type BlockProcessRequest struct` | [node/utils.go](../../../node/utils.go) |
| `go_exported` | `type OptimisticProcessingInfo struct` | [node/utils.go](../../../node/utils.go) |
| `go_exported` | `DefaultCodespace = "wasmbinding"` | [node/wasmbinding/bindings/errors.go](../../../node/wasmbinding/bindings/errors.go) |
| `go_exported` | `type BurnTokens struct` | [node/wasmbinding/bindings/msg.go](../../../node/wasmbinding/bindings/msg.go) |
| `go_exported` | `type CallEVM struct` | [node/wasmbinding/bindings/msg.go](../../../node/wasmbinding/bindings/msg.go) |
| `go_exported` | `type ChangeAdmin struct` | [node/wasmbinding/bindings/msg.go](../../../node/wasmbinding/bindings/msg.go) |
| `go_exported` | `type CreateDenom struct` | [node/wasmbinding/bindings/msg.go](../../../node/wasmbinding/bindings/msg.go) |
| `go_exported` | `type DelegateCallEVM struct` | [node/wasmbinding/bindings/msg.go](../../../node/wasmbinding/bindings/msg.go) |
| `go_exported` | `type MintTokens struct` | [node/wasmbinding/bindings/msg.go](../../../node/wasmbinding/bindings/msg.go) |
| `go_exported` | `type SetMetadata struct` | [node/wasmbinding/bindings/msg.go](../../../node/wasmbinding/bindings/msg.go) |
| `go_exported` | `func CustomEncoder(sender sdk.AccAddress, msg json.RawMessage, info wasmvmtypes.MessageInfo, codeInfo wasmtypes.CodeInfo) ([]sdk.Msg, error) {` | [node/wasmbinding/encoder.go](../../../node/wasmbinding/encoder.go) |
| `go_exported` | `type PaxWasmMessage struct` | [node/wasmbinding/encoder.go](../../../node/wasmbinding/encoder.go) |
| `go_exported` | `func CustomMessageHandler(` | [node/wasmbinding/message_plugin.go](../../../node/wasmbinding/message_plugin.go) |
| `go_exported` | `type CustomRouter struct` | [node/wasmbinding/message_plugin.go](../../../node/wasmbinding/message_plugin.go) |
| `go_exported` | `func (r *CustomRouter) Handler(msg sdk.Msg) baseapp.MsgServiceHandler` | [node/wasmbinding/message_plugin.go](../../../node/wasmbinding/message_plugin.go) |
| `go_exported` | `func NewQueryPlugin(oh *oraclewasm.OracleWasmQueryHandler, eh *epochwasm.EpochWasmQueryHandler, th *tokenfactorywasm.TokenFactoryWasmQueryHandler, evmh *evmwasm.EVMQueryHandler, sk stakingkeeper.Keeper) *QueryPlugin {` | [node/wasmbinding/queries.go](../../../node/wasmbinding/queries.go) |
| `go_exported` | `type QueryPlugin struct` | [node/wasmbinding/queries.go](../../../node/wasmbinding/queries.go) |
| `go_exported` | `func (qp QueryPlugin) HandleEpochQuery(ctx sdk.Context, queryData json.RawMessage) ([]byte, error) {` | [node/wasmbinding/queries.go](../../../node/wasmbinding/queries.go) |
| `go_exported` | `func (qp QueryPlugin) HandleEVMQuery(ctx sdk.Context, queryData json.RawMessage) (res []byte, err error) {` | [node/wasmbinding/queries.go](../../../node/wasmbinding/queries.go) |
| `go_exported` | `func (qp QueryPlugin) HandleOracleQuery(ctx sdk.Context, queryData json.RawMessage) ([]byte, error) {` | [node/wasmbinding/queries.go](../../../node/wasmbinding/queries.go) |
| `go_exported` | `func (qp QueryPlugin) HandleStakingExtQuery(ctx sdk.Context, queryData json.RawMessage) (res []byte, err error) {` | [node/wasmbinding/queries.go](../../../node/wasmbinding/queries.go) |
| `go_exported` | `func (qp QueryPlugin) HandleTokenFactoryQuery(ctx sdk.Context, queryData json.RawMessage) ([]byte, error) {` | [node/wasmbinding/queries.go](../../../node/wasmbinding/queries.go) |
| `go_exported` | `type StakingExtQuery struct` | [node/wasmbinding/queries.go](../../../node/wasmbinding/queries.go) |
| `go_exported` | `func (seq *StakingExtQuery) GetQueryType() StakingExtQueryType` | [node/wasmbinding/queries.go](../../../node/wasmbinding/queries.go) |
| `go_exported` | `type StakingExtQueryType string` | [node/wasmbinding/queries.go](../../../node/wasmbinding/queries.go) |
| `go_exported` | `type UnbondingDelegationEntry struct` | [node/wasmbinding/queries.go](../../../node/wasmbinding/queries.go) |
| `go_exported` | `type UnbondingDelegationsRequest struct` | [node/wasmbinding/queries.go](../../../node/wasmbinding/queries.go) |
| `go_exported` | `type UnbondingDelegationsResponse struct` | [node/wasmbinding/queries.go](../../../node/wasmbinding/queries.go) |
| `go_exported` | `UnbondingDelegationsType StakingExtQueryType = "staking_ext_unbonding_delegations"` | [node/wasmbinding/queries.go](../../../node/wasmbinding/queries.go) |
| `go_exported` | `func CustomQuerier(qp *QueryPlugin) func(ctx sdk.Context, request json.RawMessage) ([]byte, error) {` | [node/wasmbinding/query_plugin.go](../../../node/wasmbinding/query_plugin.go) |
| `go_exported` | `EpochRoute = "epoch"` | [node/wasmbinding/query_plugin.go](../../../node/wasmbinding/query_plugin.go) |
| `go_exported` | `EVMRoute = "evm"` | [node/wasmbinding/query_plugin.go](../../../node/wasmbinding/query_plugin.go) |
| `go_exported` | `OracleRoute = "oracle"` | [node/wasmbinding/query_plugin.go](../../../node/wasmbinding/query_plugin.go) |
| `go_exported` | `type PaxQueryWrapper struct` | [node/wasmbinding/query_plugin.go](../../../node/wasmbinding/query_plugin.go) |
| `go_exported` | `StakingExtRoute = "stakingext"` | [node/wasmbinding/query_plugin.go](../../../node/wasmbinding/query_plugin.go) |
| `go_exported` | `TokenFactoryRoute = "tokenfactory"` | [node/wasmbinding/query_plugin.go](../../../node/wasmbinding/query_plugin.go) |
| `go_exported` | `func RegisterCustomPlugins(` | [node/wasmbinding/wasm.go](../../../node/wasmbinding/wasm.go) |
| `go_exported` | `var XWebAttestors = mustXWebAttestors([]xwebAttestor` | [node/xweb_attestors.go](../../../node/xweb_attestors.go) |
| `go_exported` | `const XWebThreshold = uint32(3)` | [node/xweb_attestors.go](../../../node/xweb_attestors.go) |
| source | — | [node/wasm_abi.json](../../../node/wasm_abi.json) |

## Node metrics

| Kind | Declaration | Source |
|---|---|---|
| source | — | [storage/db_engine/pebbledb/mvcc/metrics.go](../../../storage/db_engine/pebbledb/mvcc/metrics.go) |
| source | — | [node/sequenced_metrics.go](../../../node/sequenced_metrics.go) |
| source | — | [node/sequencer/metrics.go](../../../node/sequencer/metrics.go) |
| source | — | [engine/deps/tasks/scheduler.go](../../../engine/deps/tasks/scheduler.go) |
| source | — | [engine/deps/xbank/keeper/send.go](../../../engine/deps/xbank/keeper/send.go) |
| source | — | [node/abci.go](../../../node/abci.go) |
| source | — | [node/legacyabci/begin_block.go](../../../node/legacyabci/begin_block.go) |
| source | — | [node/legacyabci/metrics.go](../../../node/legacyabci/metrics.go) |
| source | — | [node/wasmbinding/metrics.go](../../../node/wasmbinding/metrics.go) |
| source | — | [storage/common/metrics/phase_timer.go](../../../storage/common/metrics/phase_timer.go) |
| source | — | [storage/common/metrics/system_metrics.go](../../../storage/common/metrics/system_metrics.go) |
| source | — | [storage/db_engine/dbcache/cache_metrics.go](../../../storage/db_engine/dbcache/cache_metrics.go) |
| source | — | [storage/db_engine/litt/metrics/littdb_metrics.go](../../../storage/db_engine/litt/metrics/littdb_metrics.go) |
| source | — | [storage/db_engine/litt/util/cache_metrics.go](../../../storage/db_engine/litt/util/cache_metrics.go) |
| source | — | [storage/db_engine/pebbledb/pebble_metrics.go](../../../storage/db_engine/pebbledb/pebble_metrics.go) |
| source | — | [storage/ledger_db/block/blocksim/blocksim_metrics.go](../../../storage/ledger_db/block/blocksim/blocksim_metrics.go) |
| source | — | [storage/state_db/bench/cryptosim/cryptosim_metrics.go](../../../storage/state_db/bench/cryptosim/cryptosim_metrics.go) |
| source | — | [storage/state_db/sc/flatkv/metrics.go](../../../storage/state_db/sc/flatkv/metrics.go) |
| source | — | [storage/state_db/sc/memiavl/metrics.go](../../../storage/state_db/sc/memiavl/metrics.go) |
| source | — | [storage/state_db/sc/migration/migration_metrics.go](../../../storage/state_db/sc/migration/migration_metrics.go) |

## Genesis

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `int lxp_genesis_builder_cli_main(int argc, char **argv)` | [cmd/layerx-genesis/lxp_genesis_build_cli.c](../../../cmd/layerx-genesis/lxp_genesis_build_cli.c) |
| `go_exported` | `func ReadGenesisImportConfig(opts servertypes.AppOptions) (genesistypes.GenesisImportConfig, error) {` | [node/genesis.go](../../../node/genesis.go) |
| `file_format` | `if (status == LXP_OK && memcmp(magic, "LXGB", 4U) != 0)` | [cmd/layerx-genesis/lxp_genesis_build_cli.c](../../../cmd/layerx-genesis/lxp_genesis_build_cli.c) |
| `file_format` | `(void)memcpy(output, "LXGN", 4U);` | [cmd/layerx-genesis/lxp_genesis_cli.c](../../../cmd/layerx-genesis/lxp_genesis_cli.c) |
| `genesis_param` | `if genDoc.ChainID != chainID` | [daemon/paxd/cmd/init.go](../../../daemon/paxd/cmd/init.go) |
| `config_key` | `flagGenesisStreamImport = "genesis.stream-import"` | [node/genesis.go](../../../node/genesis.go) |
| `genesis_param` | `type GenesisState map[string]json.RawMessage` | [node/genesis.go](../../../node/genesis.go) |
| `genesis_param` | `distrGen := distrtypes.GenesisState` | [node/genesis.go](../../../node/genesis.go) |
| `go_exported` | `var DefaultGenesisConfig = genesistypes.GenesisImportConfig` | [node/genesis.go](../../../node/genesis.go) |
| `go_exported` | `type GenesisState map[string]json.RawMessage` | [node/genesis.go](../../../node/genesis.go) |
| `go_exported` | `func NewDefaultGenesisState(cdc codec.JSONCodec) GenesisState` | [node/genesis.go](../../../node/genesis.go) |
| source | — | [daemon/committed-fixture/vectors/vector-v1-receipted-positive.json](../../../daemon/committed-fixture/vectors/vector-v1-receipted-positive.json) |
| source | — | [daemon/paxd/cmd/genreceiptkey.go](../../../daemon/paxd/cmd/genreceiptkey.go) |
| source | — | [daemon/paxd/cmd/genseqleader.go](../../../daemon/paxd/cmd/genseqleader.go) |
| source | — | [node/genesis/genesis.go](../../../node/genesis/genesis.go) |

## Upgrade plans

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `const MergeUpgrade = pcommon.MergeUpgrade` | [node/upgrades.go](../../../node/upgrades.go) |
| `go_exported` | `const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"` | [node/upgrades.go](../../../node/upgrades.go) |
| `go_exported` | `const ActivationUpgrade = "v6.9"` | [node/upgrades.go](../../../node/upgrades.go) |
| `go_exported` | `func (app *App) LaunchpadV610Keeper() *launchpadv610keeper.Keeper` | [node/upgrades.go](../../../node/upgrades.go) |
| `go_exported` | `func (app *App) RefreshPrecompiles(ctx sdk.Context)` | [node/upgrades.go](../../../node/upgrades.go) |
| `go_exported` | `func (app *App) RegisterUpgradeHandlers()` | [node/upgrades.go](../../../node/upgrades.go) |
| `go_exported` | `var LatestUpgrade string` | [node/upgrades.go](../../../node/upgrades.go) |
| `go_exported` | `V610ExpeditedVotingPeriod = 20 * time.Minute` | [node/upgrades.go](../../../node/upgrades.go) |
| `go_exported` | `const V610Upgrade = "v6.10"` | [node/upgrades.go](../../../node/upgrades.go) |
| `go_exported` | `V610VotingPeriod = time.Hour` | [node/upgrades.go](../../../node/upgrades.go) |
| `go_exported` | `ChainIDPaxHardForkTest = "pax-hard-fork-test"` | [node/upgrades/fork_manager.go](../../../node/upgrades/fork_manager.go) |
| `go_exported` | `type HardForkHandler interface` | [node/upgrades/fork_manager.go](../../../node/upgrades/fork_manager.go) |
| `go_exported` | `type HardForkManager struct` | [node/upgrades/fork_manager.go](../../../node/upgrades/fork_manager.go) |
| `go_exported` | `func (hfm *HardForkManager) ExecuteForTargetHeight(ctx sdk.Context)` | [node/upgrades/fork_manager.go](../../../node/upgrades/fork_manager.go) |
| `go_exported` | `func (hfm *HardForkManager) RegisterHandler(handler HardForkHandler)` | [node/upgrades/fork_manager.go](../../../node/upgrades/fork_manager.go) |
| `go_exported` | `func (hfm *HardForkManager) TargetHeightReached(ctx sdk.Context) bool` | [node/upgrades/fork_manager.go](../../../node/upgrades/fork_manager.go) |
| `go_exported` | `func NewHardForkManager(chainID string) *HardForkManager` | [node/upgrades/fork_manager.go](../../../node/upgrades/fork_manager.go) |
| `go_exported` | `var SidioraProxyAddress = common.HexToAddress("0x21f7b20a555199fa73A238B1a91FD0f549068fEe")` | [node/upgrades_v611.go](../../../node/upgrades_v611.go) |
| `go_exported` | `const V6120Upgrade = "v6.12.0"` | [node/upgrades_v6120.go](../../../node/upgrades_v6120.go) |

## Admin service

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `if v := opts.Get("admin_server.admin_address"); v != nil` | [admin/config.go](../../../admin/config.go) |
| `config_key` | `if v := opts.Get("admin_server.admin_enabled"); v != nil` | [admin/config.go](../../../admin/config.go) |
| `go_exported` | `type Config struct` | [admin/config.go](../../../admin/config.go) |
| `go_exported` | ``const ConfigTemplate = ` `` | [admin/config.go](../../../admin/config.go) |
| `go_exported` | `var DefaultConfig = Config` | [admin/config.go](../../../admin/config.go) |
| `go_exported` | `DefaultEnabled = false` | [admin/config.go](../../../admin/config.go) |
| `go_exported` | `func ReadConfig(opts servertypes.AppOptions) (Config, error)` | [admin/config.go](../../../admin/config.go) |
| `go_exported` | `func StartServer(address string) (*grpc.Server, error)` | [admin/server.go](../../../admin/server.go) |
| `grpc_service_method` | `func (s *service) GetLogLevel(_ context.Context, req *types.GetLogLevelRequest) (*types.GetLogLevelResponse, error) {` | [admin/service.go](../../../admin/service.go) |
| `grpc_service_method` | `func (s *service) ListLoggers(_ context.Context, req *types.ListLoggersRequest) (*types.ListLoggersResponse, error) {` | [admin/service.go](../../../admin/service.go) |
| `grpc_service_method` | `func (s *service) SetLogLevel(_ context.Context, req *types.SetLogLevelRequest) (*types.SetLogLevelResponse, error) {` | [admin/service.go](../../../admin/service.go) |
| `go_exported` | `type AdminServiceClient interface` | [admin/types/admin.pb.go](../../../admin/types/admin.pb.go) |
| `go_exported` | `type GetLogLevelRequest struct` | [admin/types/admin.pb.go](../../../admin/types/admin.pb.go) |
| `go_exported` | `type GetLogLevelResponse struct` | [admin/types/admin.pb.go](../../../admin/types/admin.pb.go) |
| `go_exported` | `type ListLoggersRequest struct` | [admin/types/admin.pb.go](../../../admin/types/admin.pb.go) |
| `go_exported` | `type ListLoggersResponse struct` | [admin/types/admin.pb.go](../../../admin/types/admin.pb.go) |
| `go_exported` | `type LoggerInfo struct` | [admin/types/admin.pb.go](../../../admin/types/admin.pb.go) |
| `go_exported` | `func NewAdminServiceClient(cc grpc1.ClientConn) AdminServiceClient` | [admin/types/admin.pb.go](../../../admin/types/admin.pb.go) |
| `go_exported` | `func RegisterAdminServiceServer(s grpc1.Server, srv AdminServiceServer)` | [admin/types/admin.pb.go](../../../admin/types/admin.pb.go) |
| `go_exported` | `type SetLogLevelRequest struct` | [admin/types/admin.pb.go](../../../admin/types/admin.pb.go) |
| `go_exported` | `type SetLogLevelResponse struct` | [admin/types/admin.pb.go](../../../admin/types/admin.pb.go) |
| `go_exported` | `type UnimplementedAdminServiceServer struct` | [admin/types/admin.pb.go](../../../admin/types/admin.pb.go) |
| `grpc_service_method` | `ServiceName: "paxprotocol.paxchain.admin.v0.AdminService",` | [admin/types/admin.pb.go](../../../admin/types/admin.pb.go) |

<a id="sequencer-receipt"></a>

<a id="receipt-key-pin"></a>

<a id="rungs"></a>

<a id="sequencer-msg"></a>

<a id="sequencer-wal"></a>

<a id="sequencer-executor"></a>

<a id="sequencer-config"></a>

<a id="sequencer-module"></a>

<a id="errors"></a>

<a id="wiring-integration"></a>

<a id="sequencer-receipt-store"></a>

<a id="node-abci"></a>

<a id="node-ante"></a>

<a id="node-engine"></a>

<a id="node-storage"></a>

<a id="node-config"></a>

<a id="node-cli"></a>

<a id="node-go-api"></a>

<a id="node-genesis"></a>

<a id="node-upgrades"></a>

<a id="node-admin"></a>
