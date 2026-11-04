import React from 'react';
import { isAddress, isAddressEqual } from 'viem';

export default function useLoginAddressMismatch(address: string | undefined, profileAddress: string | null | undefined) {
  return React.useMemo(() => {
    if (!address || !profileAddress) {
      return false;
    }
    return !isAddress(address, { strict: false }) ||
      !isAddress(profileAddress, { strict: false }) ||
      !isAddressEqual(profileAddress, address);
  }, [ address, profileAddress ]);
}
