package evm

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"

	dbm "github.com/tendermint/tm-db"

	commonevm "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/ss/backend"
)

var _ types.StateStore = (*EVMStateStore)(nil)

// EVMStateStore manages either a single MVCC DB for all EVM data or one DB per
// EVM sub-type, depending on config. In both modes, the logical store key and
// key encoding remain unchanged.
type EVMStateStore struct {
	subDBs      map[EVMStoreType]types.StateStore
	managedDBs  []types.StateStore
	dir         string
	separateDBs bool
	closed      atomic.Bool
}

// NewEVMStateStore opens either a single unified MVCC DB for all EVM state
// or one MVCC DB per EVM sub-type.
func NewEVMStateStore(dir string, ssConfig config.StateStoreConfig) (*EVMStateStore, error) {
	if ssConfig.AsyncWriteBuffer < 0 {
		return nil, fmt.Errorf("EVMStateStore: AsyncWriteBuffer must be non-negative")
	}
	if ssConfig.KeepRecent < 0 {
		return nil, fmt.Errorf("EVMStateStore: KeepRecent must be non-negative")
	}
	opener := backend.ResolveBackend(ssConfig.Backend)

	store := &EVMStateStore{
		subDBs:      make(map[EVMStoreType]types.StateStore, NumEVMStoreTypes),
		dir:         dir,
		separateDBs: ssConfig.SeparateEVMSubDBs,
	}

	if ssConfig.SeparateEVMSubDBs {
		for _, storeType := range AllEVMStoreTypes() {
			dbDir := filepath.Join(dir, StoreTypeName(storeType))
			subCfg := subDBConfig(ssConfig, dbDir)
			db, err := opener(dbDir, subCfg)
			if err != nil {
				return nil, errors.Join(
					fmt.Errorf("failed to open EVM MVCC DB for %s: %w", StoreTypeName(storeType), err),
					store.Close(),
				)
			}
			store.subDBs[storeType] = db
			store.managedDBs = append(store.managedDBs, db)
		}
		return store, nil
	}

	cfg := subDBConfig(ssConfig, dir)
	db, err := opener(dir, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to open unified EVM MVCC DB: %w", err)
	}
	store.managedDBs = append(store.managedDBs, db)
	for _, storeType := range AllEVMStoreTypes() {
		store.subDBs[storeType] = db
	}

	return store, nil
}

func subDBConfig(parent config.StateStoreConfig, dbDir string) config.StateStoreConfig {
	cfg := parent
	cfg.DBDirectory = dbDir
	cfg.UseDefaultComparer = true
	return cfg
}

func (s *EVMStateStore) primaryDB() types.StateStore {
	if len(s.managedDBs) == 0 {
		return nil
	}
	return s.managedDBs[0]
}

func (s *EVMStateStore) routeKey(key []byte) types.StateStore {
	storeType, _ := commonevm.ParseEVMKey(key)
	if storeType == StoreEmpty {
		return nil
	}
	return s.subDBs[storeType]
}

func (s *EVMStateStore) Get(_ string, version int64, key []byte) ([]byte, error) {
	db := s.routeKey(key)
	if db == nil {
		return nil, nil
	}
	return db.Get(EVMStoreKey, version, key)
}

func (s *EVMStateStore) Has(_ string, version int64, key []byte) (bool, error) {
	db := s.routeKey(key)
	if db == nil {
		return false, nil
	}
	return db.Has(EVMStoreKey, version, key)
}

func (s *EVMStateStore) Iterator(_ string, version int64, start, end []byte) (dbm.Iterator, error) {
	// Resolve visibility in reverse: the backend's forward cursor can skip a
	// live key after seeking past a key written only at a later version.
	source, err := s.ReverseIterator(EVMStoreKey, version, start, end)
	if err != nil {
		return nil, err
	}
	itr := &forwardEVMIterator{start: bytes.Clone(start), end: bytes.Clone(end)}
	for ; source.Valid(); source.Next() {
		itr.pairs = append(itr.pairs, evmIteratorPair{
			key: bytes.Clone(source.Key()), value: bytes.Clone(source.Value()),
		})
	}
	if err := errors.Join(source.Error(), source.Close()); err != nil {
		return nil, err
	}
	itr.index = len(itr.pairs) - 1
	return itr, nil
}

type evmIteratorPair struct {
	key, value []byte
}

type forwardEVMIterator struct {
	pairs      []evmIteratorPair
	start, end []byte
	index      int
}

var _ dbm.Iterator = (*forwardEVMIterator)(nil)

func (itr *forwardEVMIterator) Domain() ([]byte, []byte) {
	return itr.start, itr.end
}

func (itr *forwardEVMIterator) Valid() bool {
	return itr.index >= 0 && itr.index < len(itr.pairs)
}

func (itr *forwardEVMIterator) assertIsValid() {
	if !itr.Valid() {
		panic("iterator is invalid")
	}
}

func (itr *forwardEVMIterator) Next() {
	itr.assertIsValid()
	itr.index--
}

func (itr *forwardEVMIterator) Key() []byte {
	itr.assertIsValid()
	return itr.pairs[itr.index].key
}

func (itr *forwardEVMIterator) Value() []byte {
	itr.assertIsValid()
	return itr.pairs[itr.index].value
}

func (itr *forwardEVMIterator) Error() error {
	return nil
}

func (itr *forwardEVMIterator) Close() error {
	itr.index = -1
	itr.pairs = nil
	return nil
}

func (s *EVMStateStore) ReverseIterator(_ string, version int64, start, end []byte) (dbm.Iterator, error) {
	return s.mergedIterator(version, start, end)
}

func (s *EVMStateStore) mergedIterator(version int64, start, end []byte) (dbm.Iterator, error) {
	itr := &mergedEVMIterator{start: bytes.Clone(start), end: bytes.Clone(end), current: -1}
	// Bounds describe a logical range, not a complete routable key. Legacy
	// keys can share any family prefix, so every DB must participate.
	for _, db := range s.managedDBs {
		child, err := db.ReverseIterator(EVMStoreKey, version, start, end)
		if err != nil {
			return nil, errors.Join(err, itr.Close())
		}
		itr.children = append(itr.children, child)
	}
	itr.selectCurrent()
	return itr, nil
}

type mergedEVMIterator struct {
	children   []dbm.Iterator
	start, end []byte
	current    int
	err        error
	closeErr   error
	closed     bool
}

var _ dbm.Iterator = (*mergedEVMIterator)(nil)

func (itr *mergedEVMIterator) selectCurrent() {
	itr.current = -1
	for i, child := range itr.children {
		valid := child.Valid()
		if err := child.Error(); err != nil {
			itr.err = err
			itr.current = -1
			return
		}
		if !valid {
			continue
		}
		if itr.current < 0 {
			itr.current = i
			continue
		}
		cmp := bytes.Compare(child.Key(), itr.children[itr.current].Key())
		if cmp > 0 {
			itr.current = i
		}
	}
}

func (itr *mergedEVMIterator) Domain() ([]byte, []byte) {
	return itr.start, itr.end
}

func (itr *mergedEVMIterator) Valid() bool {
	return !itr.closed && itr.err == nil && itr.current >= 0
}

func (itr *mergedEVMIterator) assertIsValid() {
	if !itr.Valid() {
		panic("iterator is invalid")
	}
}

func (itr *mergedEVMIterator) Next() {
	itr.assertIsValid()
	itr.children[itr.current].Next()
	itr.selectCurrent()
}

func (itr *mergedEVMIterator) Key() []byte {
	itr.assertIsValid()
	return itr.children[itr.current].Key()
}

func (itr *mergedEVMIterator) Value() []byte {
	itr.assertIsValid()
	return itr.children[itr.current].Value()
}

func (itr *mergedEVMIterator) Error() error {
	return itr.err
}

func (itr *mergedEVMIterator) Close() error {
	if !itr.closed {
		itr.closed = true
		itr.current = -1
		for _, child := range itr.children {
			itr.closeErr = errors.Join(itr.closeErr, child.Close())
		}
	}
	return itr.closeErr
}

func (s *EVMStateStore) RawIterate(storeKey string, fn func([]byte, []byte, int64) bool) (bool, error) {
	if s.closed.Load() {
		return false, fmt.Errorf("EVMStateStore: database is closed")
	}
	for i, db := range s.managedDBs {
		stopped, err := db.RawIterate(storeKey, fn)
		if err != nil {
			return stopped, fmt.Errorf("EVM raw traversal of database %d failed: %w", i, err)
		}
		if stopped {
			return true, nil
		}
	}
	return false, nil
}

func (s *EVMStateStore) GetLatestVersion() int64 {
	var minVersion int64 = -1
	for _, db := range s.managedDBs {
		if v := db.GetLatestVersion(); minVersion < 0 || v < minVersion {
			minVersion = v
		}
	}
	if minVersion < 0 {
		return 0
	}
	return minVersion
}

func (s *EVMStateStore) SetLatestVersion(version int64) error {
	if s.closed.Load() {
		return fmt.Errorf("EVMStateStore: database is closed")
	}
	for _, db := range s.managedDBs {
		if err := db.SetLatestVersion(version); err != nil {
			return err
		}
	}
	return nil
}

func (s *EVMStateStore) GetEarliestVersion() int64 {
	var maxVersion int64
	for _, db := range s.managedDBs {
		maxVersion = max(maxVersion, db.GetEarliestVersion())
	}
	return maxVersion
}

func (s *EVMStateStore) SetEarliestVersion(version int64, ignoreVersion bool) error {
	if s.closed.Load() {
		return fmt.Errorf("EVMStateStore: database is closed")
	}
	for _, db := range s.managedDBs {
		if err := db.SetEarliestVersion(version, ignoreVersion); err != nil {
			return err
		}
	}
	return nil
}

func (s *EVMStateStore) ApplyChangesetSync(version int64, changesets []*proto.NamedChangeSet) error {
	if !s.separateDBs {
		db := s.primaryDB()
		if db == nil {
			return nil
		}
		evmChangesets := filterEVMChangesets(changesets)
		if len(evmChangesets) == 0 {
			return nil
		}
		return db.ApplyChangesetSync(version, evmChangesets)
	}

	grouped := s.groupBySubType(changesets)
	if len(grouped) == 0 {
		return nil
	}
	return s.applyGrouped(version, grouped, false)
}

func (s *EVMStateStore) ApplyChangesetAsync(version int64, changesets []*proto.NamedChangeSet) error {
	if !s.separateDBs {
		db := s.primaryDB()
		if db == nil {
			return nil
		}
		evmChangesets := filterEVMChangesets(changesets)
		if len(evmChangesets) == 0 {
			return nil
		}
		return db.ApplyChangesetAsync(version, evmChangesets)
	}

	grouped := s.groupBySubType(changesets)
	if len(grouped) == 0 {
		return nil
	}
	return s.applyGrouped(version, grouped, true)
}

func (s *EVMStateStore) groupBySubType(changesets []*proto.NamedChangeSet) map[EVMStoreType][]*proto.KVPair {
	grouped := make(map[EVMStoreType][]*proto.KVPair, NumEVMStoreTypes)
	for _, cs := range changesets {
		if cs.Name != EVMStoreKey {
			continue
		}
		for _, kvPair := range cs.Changeset.Pairs {
			storeType, _ := commonevm.ParseEVMKey(kvPair.Key)
			if storeType == StoreEmpty {
				continue
			}
			grouped[storeType] = append(grouped[storeType], &proto.KVPair{
				Key:    kvPair.Key,
				Value:  kvPair.Value,
				Delete: kvPair.Delete,
			})
		}
	}
	return grouped
}

func (s *EVMStateStore) applyGrouped(version int64, grouped map[EVMStoreType][]*proto.KVPair, async bool) error {
	if len(grouped) == 1 {
		for storeType, pairs := range grouped {
			return s.applyToSubDB(storeType, version, pairs, async)
		}
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(grouped))

	for storeType, pairs := range grouped {
		wg.Add(1)
		go func(st EVMStoreType, p []*proto.KVPair) {
			defer wg.Done()
			if err := s.applyToSubDB(st, version, p, async); err != nil {
				errCh <- err
			}
		}(storeType, pairs)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		return err
	}
	return nil
}

func (s *EVMStateStore) applyToSubDB(storeType EVMStoreType, version int64, pairs []*proto.KVPair, async bool) error {
	db := s.subDBs[storeType]
	if db == nil {
		return nil
	}
	cs := []*proto.NamedChangeSet{
		{
			Name:      EVMStoreKey,
			Changeset: proto.ChangeSet{Pairs: pairs},
		},
	}
	if async {
		return db.ApplyChangesetAsync(version, cs)
	}
	return db.ApplyChangesetSync(version, cs)
}

func (s *EVMStateStore) Import(version int64, ch <-chan types.SnapshotNode) error {
	if s.closed.Load() {
		return fmt.Errorf("EVMStateStore: database is closed")
	}
	if !s.separateDBs {
		db := s.primaryDB()
		if db == nil {
			return nil
		}
		filtered := make(chan types.SnapshotNode, 100)
		stop := make(chan struct{})
		finished := make(chan struct{})
		go func() {
			defer close(finished)
			defer close(filtered)
			for {
				select {
				case <-stop:
					return
				case node, ok := <-ch:
					if !ok {
						return
					}
					if node.StoreKey == EVMStoreKey {
						select {
						case filtered <- node:
						case <-stop:
							return
						}
					}
				}
			}
		}()
		err := db.Import(version, filtered)
		close(stop)
		<-finished
		// Composite import owns the input's close; drain it so an early backend
		// return cannot block its router before it can finish the snapshot.
		for range ch {
		}
		return err
	}

	const flushThreshold = 10000
	grouped := make(map[EVMStoreType][]*proto.KVPair, NumEVMStoreTypes)
	pending := 0

	flush := func() error {
		if len(grouped) == 0 {
			return nil
		}
		if err := s.applyGrouped(version, grouped, false); err != nil {
			return err
		}
		grouped = make(map[EVMStoreType][]*proto.KVPair, NumEVMStoreTypes)
		pending = 0
		return nil
	}

	for node := range ch {
		storeType, _ := commonevm.ParseEVMKey(node.Key)
		if storeType == StoreEmpty {
			continue
		}
		grouped[storeType] = append(grouped[storeType], &proto.KVPair{
			Key:   node.Key,
			Value: node.Value,
		})
		pending++
		if pending >= flushThreshold {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	return flush()
}

func (s *EVMStateStore) Prune(version int64) error {
	if len(s.managedDBs) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(s.managedDBs))

	for _, db := range s.managedDBs {
		wg.Add(1)
		go func(db types.StateStore) {
			defer wg.Done()
			if err := db.Prune(version); err != nil {
				errCh <- err
			}
		}(db)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		return err
	}
	return nil
}

func (s *EVMStateStore) Close() error {
	s.closed.Store(true)
	var lastErr error
	for _, db := range s.managedDBs {
		if err := db.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func filterEVMChangesets(changesets []*proto.NamedChangeSet) []*proto.NamedChangeSet {
	filtered := make([]*proto.NamedChangeSet, 0, len(changesets))
	for _, cs := range changesets {
		if cs.Name == EVMStoreKey {
			filtered = append(filtered, cs)
		}
	}
	return filtered
}
