package sender

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/paxeer-network/pax-load/types"
)

type TxsWriter struct {
	gasPerBlock     uint64
	nextHeight      uint64
	txsDir          string
	blocksGenerated uint64
	numBlocks       uint64
	bufferGas       uint64
	txBuffer        []*types.LoadTx
	startHeight     uint64
	initialized     bool
}

func NewTxsWriter(gasPerBlock uint64, txsDir string, startHeight uint64, numBlocks uint64) *TxsWriter {
	return &TxsWriter{
		gasPerBlock: gasPerBlock,
		nextHeight:  startHeight,
		startHeight: startHeight,
		txsDir:      txsDir,
		numBlocks:   numBlocks,
		txBuffer:    make([]*types.LoadTx, 0),
	}
}

var ErrWriterComplete = errors.New("transaction writer completed requested blocks")

func (w *TxsWriter) Send(ctx context.Context, tx *types.LoadTx) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if tx == nil || tx.EthTx == nil {
		return errors.New("transaction is missing its Ethereum payload")
	}
	gas := tx.EthTx.Gas()
	if gas > w.gasPerBlock {
		return fmt.Errorf("transaction gas %d exceeds block gas limit %d", gas, w.gasPerBlock)
	}
	if err := w.resume(); err != nil {
		return err
	}
	if w.blocksGenerated >= w.numBlocks {
		return ErrWriterComplete
	}
	if gas > w.gasPerBlock-w.bufferGas {
		if err := w.Flush(); err != nil {
			return err
		}
		if w.blocksGenerated >= w.numBlocks {
			return ErrWriterComplete
		}
	}
	w.txBuffer = append(w.txBuffer, tx)
	w.bufferGas += gas
	return nil
}

type TxWriteData struct {
	TxPayloads [][]byte `json:"tx_payloads"`
}

type txWriterManifest struct {
	Version     int    `json:"version"`
	StartHeight uint64 `json:"start_height"`
}

const txWriterManifestName = "sender-manifest.json"

func syncTxsDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func makeTxsDir(path string) error {
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err == nil {
		if !info.IsDir() {
			return &os.PathError{Op: "mkdir", Path: path, Err: errors.New("not a directory")}
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	parent := filepath.Dir(path)
	if err := makeTxsDir(parent); err != nil {
		return err
	}
	if err := os.Mkdir(path, 0755); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	return syncTxsDir(parent)
}

// A linked, synced temporary inode publishes complete bytes without replacing a
// previous batch. Identical bytes allow retry after an interrupted publication.
func publishTxsFile(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".sender-tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if err := file.Chmod(0644); err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Link(file.Name(), path); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		existing, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if !bytes.Equal(existing, data) {
			return fmt.Errorf("conflicting transaction batch %s: %w", path, os.ErrExist)
		}
	}
	return syncTxsDir(filepath.Dir(path))
}

func (w *TxsWriter) batchPath(height uint64) string {
	return filepath.Join(w.txsDir, fmt.Sprintf("%d_txs.json", height))
}

func (w *TxsWriter) pendingPath(height uint64) string {
	return filepath.Join(w.txsDir, fmt.Sprintf("%d_txs.pending", height))
}

// Staging is durable before the batch becomes visible to transaction consumers.
// On restart the manifest and pending files allow publication to finish.
func (w *TxsWriter) stageBatch(data []byte) error {
	return publishTxsFile(w.pendingPath(w.nextHeight), data)
}

func (w *TxsWriter) publishBatch(height uint64, data []byte) error {
	if err := os.Link(w.pendingPath(height), w.batchPath(height)); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		existing, readErr := os.ReadFile(w.batchPath(height))
		if readErr != nil {
			return readErr
		}
		if !bytes.Equal(existing, data) {
			return fmt.Errorf("conflicting transaction batch %s: %w", w.batchPath(height), os.ErrExist)
		}
	}
	if err := syncTxsDir(w.txsDir); err != nil {
		return err
	}
	if err := os.Remove(w.pendingPath(height)); err != nil {
		return err
	}
	return syncTxsDir(w.txsDir)
}

func (w *TxsWriter) resume() error {
	if w.initialized {
		return nil
	}
	if err := makeTxsDir(w.txsDir); err != nil {
		return err
	}
	entries, err := os.ReadDir(w.txsDir)
	if err != nil {
		return err
	}
	heights := make(map[uint64]bool)
	for _, entry := range entries {
		name := entry.Name()
		suffix := "_txs.json"
		if strings.HasSuffix(name, "_txs.pending") {
			suffix = "_txs.pending"
		} else if !strings.HasSuffix(name, suffix) {
			continue
		}
		number := strings.TrimSuffix(name, suffix)
		height, err := strconv.ParseUint(number, 10, 64)
		if err != nil || strconv.FormatUint(height, 10) != number {
			return fmt.Errorf("invalid transaction batch name %q", name)
		}
		heights[height] = true
	}
	manifestPath := filepath.Join(w.txsDir, txWriterManifestName)
	data, err := os.ReadFile(manifestPath)
	if errors.Is(err, os.ErrNotExist) {
		if len(heights) != 0 {
			return fmt.Errorf("transaction directory has batches without a resume manifest: %w", os.ErrExist)
		}
		data, err = json.Marshal(txWriterManifest{Version: 1, StartHeight: w.startHeight})
		if err != nil {
			return err
		}
		if err := publishTxsFile(manifestPath, data); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	var manifest txWriterManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("decode writer manifest: %w", err)
	}
	if manifest.Version != 1 || manifest.StartHeight != w.startHeight {
		return fmt.Errorf("writer manifest conflicts with start height %d: %w", w.startHeight, os.ErrExist)
	}
	ordered := make([]uint64, 0, len(heights))
	for height := range heights {
		ordered = append(ordered, height)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	if uint64(len(ordered)) > w.numBlocks {
		return fmt.Errorf("recovered batches exceed requested block count %d", w.numBlocks)
	}
	next := w.startHeight
	for _, height := range ordered {
		if height != next {
			return fmt.Errorf("writer recovery expected height %d, found %d", next, height)
		}
		data, err := os.ReadFile(w.pendingPath(height))
		pending := err == nil
		if errors.Is(err, os.ErrNotExist) {
			data, err = os.ReadFile(w.batchPath(height))
		}
		if err != nil {
			return err
		}
		var batch TxWriteData
		if err := json.Unmarshal(data, &batch); err != nil {
			return fmt.Errorf("decode batch %d: %w", height, err)
		}
		if batch.TxPayloads == nil {
			return fmt.Errorf("batch %d has no transaction payloads", height)
		}
		if pending {
			if err := w.publishBatch(height, data); err != nil {
				return err
			}
		}
		next++
	}
	w.nextHeight = next
	w.blocksGenerated = uint64(len(ordered))
	w.initialized = true
	return nil
}

func (w *TxsWriter) Flush() error {
	if err := w.resume(); err != nil {
		return err
	}
	if len(w.txBuffer) == 0 {
		return nil
	}
	if w.blocksGenerated >= w.numBlocks {
		return ErrWriterComplete
	}
	txData := TxWriteData{TxPayloads: make([][]byte, 0, len(w.txBuffer))}
	for _, tx := range w.txBuffer {
		txData.TxPayloads = append(txData.TxPayloads, tx.Payload)
	}
	data, err := json.Marshal(txData)
	if err != nil {
		return err
	}
	// Refuse a stale writer before creating a recovery record for another
	// workload's already-published height.
	if existing, err := os.ReadFile(w.batchPath(w.nextHeight)); err == nil {
		if !bytes.Equal(existing, data) {
			return fmt.Errorf("conflicting transaction batch %s: %w", w.batchPath(w.nextHeight), os.ErrExist)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := w.stageBatch(data); err != nil {
		return err
	}
	if err := w.publishBatch(w.nextHeight, data); err != nil {
		return err
	}
	log.Printf("Flushed %d transactions to %s", len(w.txBuffer), w.batchPath(w.nextHeight))
	w.txBuffer = make([]*types.LoadTx, 0)
	w.bufferGas = 0
	w.nextHeight++
	w.blocksGenerated++
	return nil
}
