package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	seiconfig "github.com/sei-protocol/sei-config"
	"github.com/sei-protocol/seictl/sidecar/engine"
	"github.com/sei-protocol/seictl/sidecar/rpc"
	seis3 "github.com/sei-protocol/seictl/sidecar/s3"
	"github.com/sei-protocol/seilog"
)

var exportLog = seilog.NewLogger("seictl", "task", "result-export")

const (
	exportStateFile = ".sei-sidecar-last-export.json"
	defaultPageSize = 1000
)

var defaultRPCEndpoint = fmt.Sprintf("http://localhost:%d", seiconfig.PortRPC)

// errSinkWrite tags a failure writing to the export sink (the gzip/upload pipe),
// distinguishing a downstream/S3 cause from a genuine producer fault (RPC,
// marshaling, ctx). When the sink fails mid-stream the real cause is the upload
// error, so the write error must not be returned as if the producer failed.
var errSinkWrite = errors.New("writing to export sink")

// ResultExportRequest holds the parameters for the result-export task.
type ResultExportRequest struct {
	Bucket      string `json:"bucket"`
	Prefix      string `json:"prefix"`
	Region      string `json:"region"`
	RPCEndpoint string `json:"rpcEndpoint"`

	// CanonicalRPC enables comparison mode. When set, the exporter compares
	// local block execution against this canonical RPC endpoint and completes
	// when app-hash divergence is detected.
	CanonicalRPC string `json:"canonicalRpc"`

	// MigrationMode tunes comparison for an AppHash-breaking migration shadow
	// (e.g. memiavl->flatkv): AppHash divergence from canonical is expected
	// every block, so it is treated as informational and the verdict keys on
	// execution-results equivalence (LastResultsHash + gas + per-tx receipts).
	MigrationMode bool `json:"migrationMode,omitempty"`

	// ContinueOnDivergence selects survey mode: the comparison records each
	// divergent block to the compare page and keeps going instead of halting on
	// the first divergence. Default false preserves the production tripwire. The
	// comparator's verdict is unchanged — every field is compared authentically;
	// this only decides whether a divergence stops the run. Classifying benign vs
	// real divergences is the downstream `seictl report` step's job. Has no
	// effect outside comparison mode (it requires CanonicalRPC).
	ContinueOnDivergence bool `json:"continueOnDivergence,omitempty"`

	// ShadowEVMRPC and CanonicalEVMRPC are the EVM JSON-RPC endpoints for the
	// shadow and canonical chains. When both are set, Layer 2 (logical state
	// diff) is enabled, comparing storage/code/nonce for the keys each block
	// touched. These are EVM JSON-RPC (eth_*), distinct from the CometBFT RPC
	// used for Layers 0/1.
	ShadowEVMRPC    string `json:"shadowEvmRpc,omitempty"`
	CanonicalEVMRPC string `json:"canonicalEvmRpc,omitempty"`

	// TraceRPC is the EVM JSON-RPC endpoint used for prestate traces
	// (debug_traceBlockByNumber) to derive each block's touched keys. Defaults
	// to CanonicalEVMRPC. Requires the debug_ namespace enabled on that node.
	TraceRPC string `json:"traceRpc,omitempty"`
}

type exportIdentity struct {
	ChainID     string `json:"chainId"`
	RequestHash string `json:"requestHash"`
}

type exportState struct {
	Identity           exportIdentity `json:"identity"`
	LastExportedHeight int64          `json:"lastExportedHeight"`
}

func (e *ResultExporter) exportIdentity(cfg ResultExportRequest) exportIdentity {
	if cfg.RPCEndpoint == "" {
		cfg.RPCEndpoint = defaultRPCEndpoint
	}
	cfg.Prefix = normalizePrefix(cfg.Prefix)
	data, _ := json.Marshal(cfg)
	return exportIdentity{ChainID: e.chainID, RequestHash: recoveryHash(data)}
}

func exportFailure(operation string, err error, retryable bool) error {
	return &engine.TaskError{Task: "result-export", Operation: operation, Message: err.Error(), Cause: err.Error(), Retryable: retryable,
		Hint: "this is a one-shot task; the controller must submit a new task ID to retry after resolving the cause"}
}

// ResultExporter queries the local seid RPC for block results and uploads
// them in compressed NDJSON pages to S3.
type ResultExporter struct {
	homeDir           string
	chainID           string
	podName           string
	s3UploaderFactory seis3.UploaderFactory
}

// NewResultExporter creates an exporter targeting the given home directory.
// chainID and podName label shadow comparison metrics; pass empty strings
// if the exporter is only used in non-comparison mode.
func NewResultExporter(homeDir, chainID, podName string, factory seis3.UploaderFactory) *ResultExporter {
	if factory == nil {
		factory = seis3.DefaultUploaderFactory
	}
	return &ResultExporter{homeDir: homeDir, chainID: chainID, podName: podName, s3UploaderFactory: factory}
}

func (e *ResultExporter) Handler() engine.TaskHandler {
	return engine.TypedHandler(func(ctx context.Context, cfg ResultExportRequest) error {
		if cfg.Bucket == "" {
			return fmt.Errorf("result-export: missing required param 'bucket'")
		}
		if cfg.Region == "" {
			return fmt.Errorf("result-export: missing required param 'region'")
		}
		if cfg.RPCEndpoint == "" {
			cfg.RPCEndpoint = defaultRPCEndpoint
		}
		if cfg.CanonicalRPC != "" {
			return e.ExportAndCompare(ctx, cfg)
		}
		return e.Export(ctx, cfg)
	})
}

// Export queries the local node for block results and uploads pages to S3.
// Each invocation exports as many complete pages as are available since the
// last export height. The state file tracks progress across invocations.
func (e *ResultExporter) Export(ctx context.Context, cfg ResultExportRequest) error {
	unlock, err := lockRecovery(ctx, e.homeDir, ".sei-sidecar-export-lock")
	if err != nil {
		return exportFailure("checkpoint", err, true)
	}
	defer unlock()
	if cfg.RPCEndpoint == "" {
		cfg.RPCEndpoint = defaultRPCEndpoint
	}
	latestHeight, chainID, err := e.queryExportSource(ctx, cfg.RPCEndpoint)
	if err != nil {
		return exportFailure("source", err, true)
	}
	bound := *e
	bound.chainID = chainID
	e = &bound
	last, err := e.readCheckpoint(cfg)
	if err != nil {
		return err
	}
	startHeight := last.LastExportedHeight + 1
	if startHeight > latestHeight {
		return exportFailure("no-progress", fmt.Errorf("no new complete page: checkpoint=%d latest=%d", last.LastExportedHeight, latestHeight), true)
	}

	rpcClient := rpc.NewClient(cfg.RPCEndpoint, nil)
	prefix := normalizePrefix(cfg.Prefix)

	availableBlocks := latestHeight - startHeight + 1
	fullPages := int(availableBlocks) / defaultPageSize

	if fullPages == 0 {
		exportLog.Debug("not enough blocks for a full page yet",
			"available", availableBlocks,
			"page-size", defaultPageSize)
		return exportFailure("no-progress", fmt.Errorf("only %d of %d blocks available for the next complete page", availableBlocks, defaultPageSize), true)
	}

	uploader, err := e.s3UploaderFactory(ctx, cfg.Region)
	if err != nil {
		return fmt.Errorf("building S3 uploader: %w", err)
	}

	for page := 0; page < fullPages; page++ {
		pageStart := startHeight + int64(page*defaultPageSize)
		pageEnd := pageStart + int64(defaultPageSize) - 1

		exportLog.Info("exporting result page",
			"start", pageStart,
			"end", pageEnd,
			"bucket", cfg.Bucket)

		if err := e.exportPage(ctx, rpcClient, uploader, cfg.Bucket, cfg.Region, prefix, pageStart, pageEnd); err != nil {
			var taskErr *engine.TaskError
			if errors.As(err, &taskErr) {
				return fmt.Errorf("exporting page %d-%d: %w", pageStart, pageEnd, err)
			}
			return exportFailure("source", fmt.Errorf("exporting page %d-%d: %w", pageStart, pageEnd, err), true)
		}

		last.LastExportedHeight = pageEnd
		if err := e.writeExportState(last); err != nil {
			return exportFailure("checkpoint", err, true)
		}

	}

	lastExported := startHeight + int64(fullPages*defaultPageSize) - 1
	exportLog.Info("export complete",
		"pages", fullPages,
		"last-exported", lastExported,
		"latest-available", latestHeight)

	return nil
}

// exportPage collects block results for [start, end] and streams a gzipped
// NDJSON file to S3.
func (e *ResultExporter) exportPage(
	ctx context.Context,
	client *rpc.Client,
	uploader seis3.Uploader,
	bucket, region, prefix string,
	start, end int64,
) error {
	key := fmt.Sprintf("%s%d-%d.ndjson.gz", prefix, start, end)

	var collectErr error
	_, uploadErr := seis3.StreamGzipFunc(ctx, uploader, bucket, key, func(w io.Writer) error {
		collectErr = e.collectResults(ctx, client, w, start, end)
		return collectErr
	})
	switch {
	case collectErr != nil && !errors.Is(collectErr, errSinkWrite):
		// Genuine producer fault (block_results RPC, marshaling, ctx cancel) —
		// not an S3 problem; return it as-is, never mislabeled with S3 hints.
		return collectErr
	case uploadErr != nil:
		// Upload failed, including mid-stream (which also shows up as a tagged
		// errSinkWrite in collectErr). Keep the S3 classification + Retryable hint.
		return seis3.ClassifyS3Error("result-export", bucket, key, region, uploadErr)
	case collectErr != nil:
		// A sink write failed without the upload reporting an error — surface it.
		return collectErr
	}
	return nil
}

// collectResults queries block_results for each height and writes one NDJSON
// line per block to w. Each line is a JSON object with height, time, and the
// raw block_results response. gzip/pipe/checksum are owned by the s3 helper.
func (e *ResultExporter) collectResults(ctx context.Context, client *rpc.Client, w io.Writer, start, end int64) error {
	for h := start; h <= end; h++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		result, err := queryBlockResults(ctx, client, h)
		if err != nil {
			return fmt.Errorf("querying block_results at height %d: %w", h, err)
		}

		record := map[string]any{
			"height":        h,
			"exported_at":   time.Now().UTC().Format(time.RFC3339),
			"block_results": result,
		}

		line, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("marshaling result at height %d: %w", h, err)
		}
		line = append(line, '\n')

		if _, err := w.Write(line); err != nil {
			return fmt.Errorf("%w at height %d: %w", errSinkWrite, h, err)
		}
	}

	return nil
}

func queryLatestHeight(ctx context.Context, rpcEndpoint string) (int64, error) {
	c := rpc.NewStatusClient(rpcEndpoint, nil)
	h, err := c.LatestHeight(ctx)
	if err != nil {
		return 0, err
	}
	if h <= 0 {
		return 0, fmt.Errorf("latest_block_height is %d, node may still be syncing", h)
	}
	return h, nil
}

func queryBlockResults(ctx context.Context, client *rpc.Client, height int64) (json.RawMessage, error) {
	body, err := client.GetRaw(ctx, fmt.Sprintf("/block_results?height=%d", height))
	if err != nil {
		return nil, err
	}
	var envelope struct {
		JSONRPC string          `json:"jsonrpc"`
		Error   json.RawMessage `json:"error"`
		Result  json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("invalid block_results response: %w", err)
	}
	if envelope.JSONRPC == "2.0" && ((len(envelope.Error) > 0 && string(envelope.Error) != "null") || len(envelope.Result) == 0 || string(envelope.Result) == "null") {
		return nil, fmt.Errorf("block_results returned a JSON-RPC source failure: %s", body)
	}
	return json.RawMessage(body), nil
}

func (e *ResultExporter) readExportState() (exportState, error) {
	data, err := readRecoveryFile(e.homeDir, exportStateFile)
	if errors.Is(err, os.ErrNotExist) {
		return e.bootstrapExportState()
	}
	if err != nil {
		return exportState{}, err
	}
	var state exportState
	if err := json.Unmarshal(data, &state); err != nil {
		return exportState{}, err
	}
	if state.LastExportedHeight < 0 || state.Identity.RequestHash == "" {
		return exportState{}, fmt.Errorf("unbound or invalid export checkpoint; explicit backfill required")
	}
	return state, nil
}

func (e *ResultExporter) readCheckpoint(cfg ResultExportRequest) (exportState, error) {
	state, err := e.readExportState()
	if err != nil {
		return exportState{}, exportFailure("checkpoint", err, false)
	}
	identity := e.exportIdentity(cfg)
	if state.Identity.RequestHash != "" && state.Identity != identity {
		return exportState{}, exportFailure("checkpoint-identity", fmt.Errorf("source, chain or destination changed; explicit backfill into a separate export home is required"), false)
	}
	state.Identity = identity
	return state, nil
}

// Snapshot baselines apply only when no bound checkpoint exists. A destination
// change is refused above, never bootstrapped past missing destination pages.
func (e *ResultExporter) bootstrapExportState() (exportState, error) {
	data, err := readRecoveryFile(e.homeDir, SnapshotHeightFile)
	if errors.Is(err, os.ErrNotExist) {
		return exportState{}, nil
	}
	if err != nil {
		return exportState{}, err
	}
	h, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil || h < 0 {
		return exportState{}, fmt.Errorf("invalid snapshot height %q", strings.TrimSpace(string(data)))
	}
	return exportState{LastExportedHeight: h}, nil
}

func (e *ResultExporter) writeExportState(state exportState) error {
	if state.Identity.RequestHash == "" || state.LastExportedHeight < 0 {
		return fmt.Errorf("refusing unbound or invalid export checkpoint")
	}
	return saveRecovery(e.homeDir, exportStateFile, state)
}

func (e *ResultExporter) queryExportSource(ctx context.Context, endpoint string) (int64, string, error) {
	client := rpc.NewClient(endpoint, nil)
	client.SetTimeout(500 * time.Millisecond)
	raw, err := client.Get(ctx, "/status")
	if err != nil {
		return 0, "", err
	}
	var status rpc.StatusResult
	if err := json.Unmarshal(raw, &status); err != nil {
		return 0, "", err
	}
	if status.NodeInfo.Network == "" || (e.chainID != "" && status.NodeInfo.Network != e.chainID) {
		return 0, "", fmt.Errorf("source chain %q does not match configured chain %q", status.NodeInfo.Network, e.chainID)
	}
	h, err := strconv.ParseInt(status.SyncInfo.LatestBlockHeight, 10, 64)
	if err != nil {
		return 0, "", err
	}
	if h <= 0 {
		return 0, "", fmt.Errorf("source has no committed blocks")
	}
	return h, status.NodeInfo.Network, nil
}
