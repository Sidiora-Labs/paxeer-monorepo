package deployment

import (
	"context"
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

func runAttestationReads(ctx context.Context, limit int, reads []func()) {
	if limit <= 1 {
		for _, read := range reads {
			if ctx.Err() != nil {
				return
			}
			read()
		}
		return
	}
	var workers sync.WaitGroup
	jobs := make(chan func())
	for range min(limit, len(reads)) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for read := range jobs {
				if ctx.Err() == nil {
					read()
				}
			}
		}()
	}
	for _, read := range reads {
		select {
		case <-ctx.Done():
			close(jobs)
			workers.Wait()
			return
		case jobs <- read:
		}
	}
	close(jobs)
	workers.Wait()
}

func attestIdentity(ctx context.Context, r Reader, m Manifest, name string, block common.Hash) Identity {
	role := m.Roles[name]
	var err error
	id := Identity{Role: name, Address: role.Address}
	add := func(code string) { id.Issues = append(id.Issues, Issue{code, name}) }
	addr := common.HexToAddress(role.Address)
	if role.Kind == valuePrecompile {
		// Precompile code is not EVM bytecode: the identity is the fixed address.
		if !validAddress(role.Address) || addr != common.HexToAddress(LaunchpadPrecompileAddress) {
			add(valueIdentityUnverified)
		}
	} else {
		id.RuntimeHash, err = codeHash(ctx, r, addr, block)
		if err != nil {
			add(valueCodeReadFailed)
		} else if !role.AcceptsRuntimeHash(id.RuntimeHash) {
			add("unknown_runtime_hash")
		}
	}
	var impl common.Address
	switch role.Kind {
	case valueErc1967, valueBeaconProxy:
		slot := ImplementationSlot
		if role.Kind == valueBeaconProxy {
			slot = BeaconSlot
		}
		var b []byte
		err = ctx.Err()
		if err == nil {
			b, err = r.StorageAtHash(ctx, addr, slot, block)
		}
		if err == nil {
			impl, err = wordAddress(b)
		}
		if err != nil || impl == (common.Address{}) {
			add(valueIdentityUnverified)
		} else if role.Kind == valueBeaconProxy {
			id.BeaconAddress = impl.Hex()
			if impl != common.HexToAddress(role.BeaconAddress) {
				add("beacon_mismatch")
			}
			beaconHash, e := codeHash(ctx, r, impl, block)
			known := false
			for _, candidate := range m.Roles {
				if candidate.Kind == valueBeacon && common.HexToAddress(candidate.Address) == impl && equalHash(candidate.RuntimeHash, beaconHash) {
					known = true
				}
			}
			if e != nil || !known {
				add("beacon_identity_unverified")
			}
			impl, err = implementationAt(ctx, r, impl, "implementation()", block)
			if err != nil {
				add(valueIdentityUnverified)
			}
		}
	case valueBeacon:
		impl, err = implementationAt(ctx, r, addr, "implementation()", block)
		if err != nil {
			add(valueIdentityUnverified)
		}
	}
	if hasImplementation(role.Kind) {
		id.ImplementationAddress = impl.Hex()
		if impl != common.HexToAddress(role.ImplementationAddress) {
			add("implementation_mismatch")
		}
		if impl != (common.Address{}) {
			id.ImplementationHash, err = codeHash(ctx, r, impl, block)
			if err != nil {
				add("implementation_read_failed")
			} else if !equalHash(id.ImplementationHash, role.ImplementationHash) {
				add("unknown_implementation_hash")
			}
		}
	}
	id.Verified = len(id.Issues) == 0
	return id
}

type attestationGetter struct {
	address   common.Address
	signature string
}

type attestationWord struct {
	value []byte
	err   error
}

func (w attestationWord) address() (common.Address, error) {
	if w.err != nil {
		return common.Address{}, w.err
	}
	return wordAddress(w.value)
}

func readAttestationGetters(ctx context.Context, r Reader, block common.Hash, limit int, getters []attestationGetter) []attestationWord {
	words := make([]attestationWord, len(getters))
	reads := make([]func(), len(getters))
	for i, getter := range getters {
		reads[i] = func() {
			words[i].value, words[i].err = callWord(ctx, r, getter.address, getter.signature, block)
		}
	}
	runAttestationReads(ctx, limit, reads)
	return words
}
