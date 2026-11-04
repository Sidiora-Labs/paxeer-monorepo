import type { TokenInstance } from 'types/api/token';

import config from 'configs/app';
import { useMultichainContext } from 'lib/contexts/multichain';

export default function useNftMediaIdentity(data: TokenInstance) {
  const multichainContext = useMultichainContext();
  return [ multichainContext?.chain.id || config.chain.id, data.token.address_hash, data.id ];
}
