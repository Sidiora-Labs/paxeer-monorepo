package history

/*
#cgo CFLAGS: -I${SRCDIR}/../../../include -I${SRCDIR}/../../../build/generated
#include "layerx/lxp_snapshot.h"
#include "layerx/lxp_ledger.h"
#include "layerx/lxp_identity.h"
*/
import "C"

import (
	"bytes"
	"encoding/hex"
	"errors"
	"unsafe"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
)

// OwnedAccounts decodes the committed host registry with the kernel's own
// codec and ownership rules. A missing store or corrupt record is an error;
// neither may be mistaken for an empty registry.
func OwnedAccounts(ctx sdk.Context, did string, limit int) ([]string, error) {
	if did == "" || limit < 1 {
		return nil, errors.New("invalid native account enumeration")
	}
	var owner [32]byte
	didBytes := []byte(did)
	if C.lxp_did_id_derive((*C.uint8_t)(unsafe.Pointer(&didBytes[0])), C.size_t(len(didBytes)), (*C.uint8_t)(unsafe.Pointer(&owner[0]))) != C.LXP_OK {
		return nil, errors.New("invalid native account owner")
	}
	for _, key := range ctx.MultiStore().StoreKeys() {
		if key.Name() != keys.LayerXHostStoreKey {
			continue
		}
		iterator := sdk.KVStorePrefixIterator(ctx.KVStore(key), []byte{0x06})
		defer iterator.Close()
		accounts := []string{}
		for ; iterator.Valid(); iterator.Next() {
			if err := ctx.Context().Err(); err != nil {
				return nil, err
			}
			row := iterator.Value()
			storedKey := iterator.Key()
			if len(row) == 0 || len(storedKey) != 33 {
				return nil, errors.New("invalid native account record")
			}
			var decoded C.lx_account
			code := C.lx_account_record_decode((*C.uint8_t)(unsafe.Pointer(&row[0])), C.size_t(len(row)), &decoded)
			if code != C.LXP_OK || !bytes.Equal(C.GoBytes(unsafe.Pointer(&decoded.id[0]), 32), storedKey[1:]) {
				return nil, errors.New("invalid native account record")
			}
			var actual [32]byte
			code = C.lx_account_owner_did(&decoded, (*C.uint8_t)(unsafe.Pointer(&actual[0])))
			if code == C.LXP_ERR_UNKNOWN_FIELD {
				continue
			}
			if code != C.LXP_OK {
				return nil, errors.New("native account owner unavailable")
			}
			if actual != owner {
				continue
			}
			if len(accounts) == limit {
				return nil, errors.New("native account enumeration exceeds limit")
			}
			accounts = append(accounts, hex.EncodeToString(storedKey[1:]))
		}
		return accounts, nil
	}
	return nil, errors.New("native account registry is not mounted")
}
