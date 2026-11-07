import { Box } from '@chakra-ui/react';
import React from 'react';

import type { TokenTransfer } from 'types/api/tokenTransfer';

import { useMultichainContext } from 'lib/contexts/multichain';
import { getTokenTransferKey } from 'ui/shared/TokenTransfer/helpers';
import TokenTransferListItem from 'ui/shared/TokenTransfer/TokenTransferListItem';

interface Props {
  data: Array<TokenTransfer>;
  baseAddress?: string;
  showTxInfo?: boolean;
  enableTimeIncrement?: boolean;
  isLoading?: boolean;
}

export const TokenTransferList = ({ data, baseAddress, showTxInfo, enableTimeIncrement, isLoading }: Props) => {
  const multichainContext = useMultichainContext();
  const chainData = multichainContext?.chain;

  return (
    <Box>
      { data.map((item, index) => (
        <TokenTransferListItem
          key={ getTokenTransferKey(item, isLoading ? index : undefined) }
          { ...item }
          baseAddress={ baseAddress }
          showTxInfo={ showTxInfo }
          enableTimeIncrement={ enableTimeIncrement }
          isLoading={ isLoading }
          chainData={ chainData }
        />
      )) }
    </Box>
  );
};

export default React.memo(TokenTransferList);
