package keyring

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/99designs/keyring"
	"github.com/pkg/errors"
)

const operationJournalKey = ".keyring-operation-v1"

// operationJournal is stored by the secret backend before changing either view.
// Replay is idempotent; the original public and private key record is retained.
type operationJournal struct {
	Version    int    `json:"version"`
	Operation  string `json:"operation"`
	NameKey    string `json:"name_key"`
	AddressKey string `json:"address_key"`
	Data       []byte `json:"data"`
}

// PendingOperationError reports a durable intent which must be recovered before
// another operation can proceed. Reopen or retry after fixing the storage error.
type PendingOperationError struct {
	Operation string
	NameKey   string
	Err       error
}

func (e *PendingOperationError) Error() string {
	return fmt.Sprintf("keyring %s pending for %q: %v", e.Operation, e.NameKey, e.Err)
}
func (e *PendingOperationError) Unwrap() error { return e.Err }

func (ks keystore) recoverOperation() error {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	_, err := ks.recoverOperationLocked()
	return err
}

func (ks keystore) recoverOperationLocked() (*operationJournal, error) {
	item, err := ks.db.Get(operationJournalKey)
	if errors.Is(err, keyring.ErrKeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, &EntryError{Entry: operationJournalKey, Err: err}
	}
	var op operationJournal
	if err := json.Unmarshal(item.Data, &op); err != nil {
		return nil, &EntryError{Entry: operationJournalKey, Err: errors.New("invalid operation journal")}
	}
	info, err := unmarshalInfo(op.Data)
	if err != nil {
		return nil, &EntryError{Entry: operationJournalKey, Err: errors.New("invalid journal key information")}
	}
	if err := validateInfo(info); err != nil {
		return nil, &EntryError{Entry: operationJournalKey, Err: err}
	}
	if op.Version != 1 || (op.Operation != "create" && op.Operation != "delete") || op.NameKey != infoKey(info.GetName()) || op.AddressKey != addrHexKeyAsString(info.GetAddress()) {
		return nil, &EntryError{Entry: operationJournalKey, Err: errors.New("inconsistent operation journal")}
	}
	if err := ks.applyOperation(&op); err != nil {
		return nil, err
	}
	return &op, nil
}

func (ks keystore) persistOperation(operation string, info Info) error {
	op := &operationJournal{Version: 1, Operation: operation, NameKey: infoKey(info.GetName()), AddressKey: addrHexKeyAsString(info.GetAddress()), Data: marshalInfo(info)}
	data, err := json.Marshal(op)
	if err != nil {
		return err
	}
	if err := ks.db.Set(keyring.Item{Key: operationJournalKey, Data: data}); err != nil {
		return err
	}
	return ks.applyOperation(op)
}

func (ks keystore) applyOperation(op *operationJournal) error {
	pending := func(err error) error {
		return &PendingOperationError{Operation: op.Operation, NameKey: op.NameKey, Err: err}
	}
	if op.Operation == "create" {
		if err := ks.db.Set(keyring.Item{Key: op.NameKey, Data: op.Data}); err != nil {
			return pending(err)
		}
		if err := ks.db.Set(keyring.Item{Key: op.AddressKey, Data: []byte(op.NameKey)}); err != nil {
			return pending(err)
		}
	} else {
		if err := ks.removeIfPresent(op.AddressKey); err != nil {
			return pending(err)
		}
		if err := ks.removeIfPresent(op.NameKey); err != nil {
			return pending(err)
		}
	}
	if err := ks.removeIfPresent(operationJournalKey); err != nil {
		return pending(err)
	}
	return nil
}

func (ks keystore) removeIfPresent(key string) error {
	err := ks.db.Remove(key)
	if errors.Is(err, keyring.ErrKeyNotFound) || os.IsNotExist(err) {
		return nil
	}
	return err
}
