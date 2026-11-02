//go:build cgo

package lxphost

/*
#include "layerx/lxp_authority.h"
*/
import "C"

const AuthorityKindOwner uint8 = C.LXP_AUTHORITY_OWNER

func AuthorityHash(kind uint8, grantID, verifiedKey [32]byte) ([32]byte, error) {
	var hash [32]byte
	if err := result(C.lxp_authority_hash(C.lxp_authority_kind(kind), bytes32(&grantID), bytes32(&verifiedKey), bytes32(&hash))); err != nil {
		return [32]byte{}, err
	}
	return hash, nil
}
