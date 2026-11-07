package memiavl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/types"
)

var (
	// Pipeline channel size - controls how many operations can be queued
	// Memory footprint (worst case, all 3 channels full in snapshot.go):
	// - kvChan: 500k × ~140 bytes (key+value avg) = ~70 MiB
	// - leafChan: 500k × ~48 bytes (fixed size) = ~24 MiB
	// - branchChan: 500k × ~45 bytes (fixed size) = ~22.5 MiB
	// Total: ~116 MiB for channel buffers
	// Profiling shows peak usage of ~440k nodes with 1GB total overhead.
	// Smaller buffer causes frequent blocking and context switch overhead.
	nodeChanSize = 500000

	// bufio.Writer buffer size - 128 MiB balances performance and memory usage
	// Large buffer reduces syscalls and improves throughput for sequential writes
	bufIOSize = 128 << 20 // 128 MiB
)

type MultiTreeImporter struct {
	dir         string
	snapshotDir string
	height      int64
	importer    *TreeImporter
	fileLock    FileLock
	ctx         context.Context // Context for cancellation support
	err         error
	closed      bool
}

func NewMultiTreeImporter(dir string, height uint64) (*MultiTreeImporter, error) {
	if height > math.MaxUint32 {
		return nil, fmt.Errorf("version overflows uint32: %d", height)
	}

	var fileLock FileLock
	fileLock, err := LockFile(filepath.Join(dir, LockFileName))
	if err != nil {
		return nil, fmt.Errorf("fail to lock db: %w", err)
	}

	return &MultiTreeImporter{
		dir:         dir,
		height:      int64(height),
		snapshotDir: snapshotName(int64(height)),
		fileLock:    fileLock,
		ctx:         context.Background(), // Default to background context for backward compatibility
	}, nil
}

func (mti *MultiTreeImporter) tmpDir() string {
	return filepath.Join(mti.dir, mti.snapshotDir+"-tmp")
}

func (mti *MultiTreeImporter) Add(item interface{}) error {
	switch item := item.(type) {
	case *types.SnapshotNode:
		return mti.addNode(item)
	case string:
		return mti.AddModule(item)
	default:
		if mti.err == nil {
			mti.err = fmt.Errorf("unknown item type: %T", item)
		}
		return mti.err
	}
}

func (mti *MultiTreeImporter) AddModule(name string) error {
	if mti.closed {
		return fmt.Errorf("importer is closed")
	}
	if mti.err != nil {
		return mti.err
	}
	if mti.importer != nil {
		if err := mti.importer.Close(); err != nil {
			mti.err = err
			return err
		}
	}
	mti.importer = NewTreeImporter(mti.ctx, filepath.Join(mti.tmpDir(), name), mti.height)
	return nil
}

// AddNode preserves the Importer interface; errors are retained for Close.
func (mti *MultiTreeImporter) AddNode(node *types.SnapshotNode) {
	mti.addNode(node)
}

func (mti *MultiTreeImporter) addNode(node *types.SnapshotNode) error {
	if mti.closed {
		return fmt.Errorf("importer is closed")
	}
	if mti.err != nil {
		return mti.err
	}
	if mti.importer == nil {
		mti.err = fmt.Errorf("snapshot node received before module")
		return mti.err
	}
	mti.err = mti.importer.Add(node)
	return mti.err
}

func (mti *MultiTreeImporter) Close() (returnErr error) {
	if mti.closed {
		return mti.err
	}
	mti.closed = true
	defer func() {
		if returnErr != nil {
			returnErr = errors.Join(returnErr, os.RemoveAll(mti.tmpDir()))
		}
		returnErr = errors.Join(returnErr, mti.fileLock.Unlock(), mti.fileLock.Destroy())
		mti.err = returnErr
	}()
	if mti.importer != nil {
		err := mti.importer.Close()
		mti.importer = nil
		if mti.err == nil {
			mti.err = err
		}
	}
	if mti.err != nil {
		return mti.err
	}

	tmpDir := mti.tmpDir()
	if err := updateMetadataFile(tmpDir, mti.height); err != nil {
		return err
	}
	if err := os.Rename(tmpDir, filepath.Join(mti.dir, mti.snapshotDir)); err != nil {
		return err
	}
	return updateCurrentSymlink(mti.dir, mti.snapshotDir)
}

// TreeImporter imports a single memiavl tree from a state-sync snapshot.
type TreeImporter struct {
	nodesChan chan *types.SnapshotNode
	done      chan struct{}
	ctx       context.Context
	mu        sync.RWMutex
	closed    bool
	err       error // Written before done is closed; read only after done.
}

func NewTreeImporter(ctx context.Context, dir string, version int64) *TreeImporter {
	ai := &TreeImporter{
		nodesChan: make(chan *types.SnapshotNode, nodeChanSize),
		done:      make(chan struct{}),
		ctx:       ctx,
	}
	go func() {
		ai.err = doImport(ctx, dir, version, ai.nodesChan)
		close(ai.done)
	}()
	return ai
}

func (ai *TreeImporter) Add(node *types.SnapshotNode) error {
	ai.mu.RLock()
	defer ai.mu.RUnlock()
	select {
	case <-ai.done:
		if ai.err != nil {
			return ai.err
		}
		return fmt.Errorf("importer is closed")
	case <-ai.ctx.Done():
		return ai.ctx.Err()
	default:
	}
	if ai.closed {
		return fmt.Errorf("importer is closed")
	}
	select {
	case <-ai.done:
		return ai.err
	case <-ai.ctx.Done():
		return ai.ctx.Err()
	case ai.nodesChan <- node:
		return nil
	}
}

func (ai *TreeImporter) Close() error {
	ai.mu.Lock()
	if !ai.closed {
		ai.closed = true
		close(ai.nodesChan)
	}
	ai.mu.Unlock()
	<-ai.done
	return ai.err
}

// doImport a stream of `types.SnapshotNode`s into a new snapshot.
func doImport(ctx context.Context, dir string, version int64, nodes <-chan *types.SnapshotNode) (returnErr error) {
	if version < 0 || version > int64(math.MaxUint32) {
		return fmt.Errorf("version under/overflows uint32: %d", version)
	}

	return writeSnapshot(ctx, dir, uint32(version), func(w *snapshotWriter) (uint32, error) {
		ctx := w.ctx
		i := &importer{
			w:           w,
			leavesStack: make([]uint32, 0),
			nodeStack:   make([]*MemNode, 0),
		}

		i.version = uint32(version)
		for {
			if err := ctx.Err(); err != nil {
				return 0, fmt.Errorf("import cancelled: %w", err)
			}
			select {
			case <-ctx.Done():
				return 0, fmt.Errorf("import cancelled: %w", ctx.Err())
			case node, ok := <-nodes:
				if !ok {
					goto complete
				}
				if err := i.Add(node); err != nil {
					return 0, err
				}
			}
		}
	complete:

		switch len(i.leavesStack) {
		case 0:
			return 0, nil
		case 1:
			return i.w.leafCounter, nil
		default:
			return 0, fmt.Errorf("invalid node structure, found stack size %v after imported", len(i.leavesStack))
		}
	})
}

type importer struct {
	w           *snapshotWriter
	version     uint32
	boundsStack [][2][]byte
	lastKey     []byte
	hasLeaf     bool

	// keep track of how many leaves has been written before the pending nodes
	leavesStack []uint32
	// keep track of the pending nodes
	nodeStack []*MemNode
}

func (i *importer) Add(n *types.SnapshotNode) error {
	if n == nil {
		return fmt.Errorf("nil snapshot node")
	}
	if n.Height < 0 {
		return fmt.Errorf("negative node height: %d", n.Height)
	}
	if n.Version < 0 || n.Version > math.MaxUint32 || n.Version > int64(i.version) {
		return fmt.Errorf("node version under/overflows uint32: %d", n.Version)
	}
	version := uint32(n.Version)

	if n.Height == 0 {
		if i.hasLeaf && bytes.Compare(i.lastKey, n.Key) >= 0 {
			return fmt.Errorf("snapshot leaves are not strictly ordered")
		}
		node := &MemNode{
			height:  0,
			size:    1,
			version: version,
			key:     n.Key,
			value:   n.Value,
		}
		nodeHash := node.Hash()
		if err := i.w.writeLeaf(node.version, node.key, node.value, nodeHash); err != nil {
			return err
		}
		i.leavesStack = append(i.leavesStack, i.w.leafCounter)
		i.nodeStack = append(i.nodeStack, node)
		i.boundsStack = append(i.boundsStack, [2][]byte{n.Key, n.Key})
		i.lastKey = n.Key
		i.hasLeaf = true
		return nil
	}

	// Validate the complete post-order pair before indexing or constructing a branch.
	if len(i.nodeStack) < 2 || len(i.leavesStack) != len(i.nodeStack) {
		return fmt.Errorf("branch requires two child subtrees")
	}
	keyLeaf := i.leavesStack[len(i.leavesStack)-2]
	leftNode := i.nodeStack[len(i.nodeStack)-2]
	rightNode := i.nodeStack[len(i.nodeStack)-1]

	leftBounds := i.boundsStack[len(i.boundsStack)-2]
	rightBounds := i.boundsStack[len(i.boundsStack)-1]
	if int(n.Height) != 1+max(int(leftNode.height), int(rightNode.height)) ||
		int(leftNode.height)-int(rightNode.height) > 1 || int(rightNode.height)-int(leftNode.height) > 1 {
		return fmt.Errorf("invalid branch or child heights")
	}
	if bytes.Compare(leftBounds[1], n.Key) >= 0 || !bytes.Equal(n.Key, rightBounds[0]) {
		return fmt.Errorf("invalid branch key ordering")
	}
	if version < leftNode.version || version < rightNode.version {
		return fmt.Errorf("branch version precedes child version")
	}

	node := &MemNode{
		height:  uint8(n.Height),
		size:    leftNode.size + rightNode.size,
		version: version,
		key:     n.Key,
		left:    leftNode,
		right:   rightNode,
	}
	nodeHash := node.Hash()

	// remove unnecessary reference to avoid memory leak
	node.left = nil
	node.right = nil

	pt := len(i.nodeStack) - 2
	if pt < 0 || pt > math.MaxUint8 {
		return fmt.Errorf("preTrees out of range: %d", pt)
	}
	preTrees := uint8(pt)
	if node.size < 0 || node.size > math.MaxUint32 {
		return fmt.Errorf("node size under/overflows uint32: %d", node.size)
	}
	if err := i.w.writeBranch(node.version, uint32(node.size), node.height, preTrees, keyLeaf, nodeHash); err != nil {
		return err
	}

	i.leavesStack = i.leavesStack[:len(i.leavesStack)-2]
	i.leavesStack = append(i.leavesStack, i.w.leafCounter)

	i.nodeStack = i.nodeStack[:len(i.nodeStack)-2]
	i.nodeStack = append(i.nodeStack, node)
	i.boundsStack = i.boundsStack[:len(i.boundsStack)-2]
	i.boundsStack = append(i.boundsStack, [2][]byte{leftBounds[0], rightBounds[1]})
	return nil
}

func updateMetadataFile(dir string, height int64) (returnErr error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	storeInfos := make([]proto.StoreInfo, 0, len(entries))
	opts := Options{Config: Config{SnapshotPrefetchThreshold: 0}}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		snapshot, err := OpenSnapshot(filepath.Join(dir, name), opts)
		if err != nil {
			return err
		}
		defer func() {
			if err := snapshot.Close(); returnErr == nil {
				returnErr = err
			}
		}()
		storeInfos = append(storeInfos, proto.StoreInfo{
			Name: name,
			CommitId: proto.CommitID{
				Version: height,
				Hash:    snapshot.RootHash(),
			},
		})
	}
	metadata := proto.MultiTreeMetadata{
		CommitInfo: &proto.CommitInfo{
			Version:    height,
			StoreInfos: storeInfos,
		},
		// initial version should correspond to the first rlog entry
		InitialVersion: height + 1,
	}
	bz, err := metadata.Marshal()
	if err != nil {
		return err
	}
	return WriteFileSync(filepath.Join(dir, MetadataFileName), bz)
}
