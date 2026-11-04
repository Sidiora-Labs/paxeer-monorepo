import { useQueryClient } from '@tanstack/react-query';
import React from 'react';

import type { SocketMessage } from 'lib/socket/types';
import type { Block, BlockType, BlocksResponse } from 'types/api/block';

import { getResourceKey } from 'lib/api/useApiQuery';
import { useMultichainContext } from 'lib/contexts/multichain';
import useSocketChannel from 'lib/socket/useSocketChannel';
import useSocketMessage from 'lib/socket/useSocketMessage';

const OVERLOAD_COUNT = 75;

interface Params {
  type?: BlockType;
  isDisabled: boolean;
}

export default function useBlocksNewBlock({ type, isDisabled }: Params) {
  const queryClient = useQueryClient();
  const multichainContext = useMultichainContext();
  const [ showSocketAlert, setShowSocketAlert ] = React.useState(false);
  const [ newItemsCount, setNewItemsCount ] = React.useState(0);

  const handleNewBlockMessage: SocketMessage.NewBlock['handler'] = React.useCallback((payload) => {
    const queryKey = getResourceKey('general:blocks', {
      queryParams: { type },
      chainId: multichainContext?.chain?.id,
    });

    queryClient.setQueryData(queryKey, (prevData: BlocksResponse | undefined) => {
      const shouldAddToList = !type || type === payload.block.type;

      if (!prevData) {
        return {
          items: shouldAddToList ? [ payload.block ] : [],
          next_page_params: null,
        };
      }

      const matchesExistingBlock = (block: Block) =>
        block.hash === payload.block.hash ||
        ((!type || type === 'block') && payload.block.type === 'block' && block.type === 'block' && block.height === payload.block.height);
      const existingIndex = prevData.items.findIndex(matchesExistingBlock);

      if (existingIndex !== -1) {
        const items = prevData.items.flatMap((block, index) => {
          if (!matchesExistingBlock(block)) {
            return [ block ];
          }
          return shouldAddToList && index === existingIndex ? [ payload.block ] : [];
        });

        return { ...prevData, items };
      }

      if (!shouldAddToList) {
        return prevData;
      }

      if (prevData.items.length >= OVERLOAD_COUNT) {
        setNewItemsCount(prev => prev + 1);
        return prevData;
      }

      const newItems = [ payload.block, ...prevData.items ].sort((b1, b2) => b2.height - b1.height);
      return { ...prevData, items: newItems };
    });
  }, [ multichainContext?.chain?.id, queryClient, type ]);

  const handleSocketFailure = React.useCallback(() => {
    setShowSocketAlert(true);
  }, []);

  const channel = useSocketChannel({
    topic: 'blocks:new_block',
    onSocketClose: handleSocketFailure,
    onSocketError: handleSocketFailure,
    isDisabled,
  });
  useSocketMessage({ channel, event: 'new_block', handler: handleNewBlockMessage });

  return { newItemsCount, showSocketAlert };
}
