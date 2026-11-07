import { Box } from '@chakra-ui/react';
import React from 'react';

import { publicClient } from 'lib/web3/client';
import { BLOCK } from 'stubs/block';
import { TableBody } from 'toolkit/chakra/table';

import HomeTable from '../HomeTable';
import LatestBlocksItem from '../LatestBlocksItem';
import LatestBlocksFallback from './LatestBlocksFallback';
import { useHomeRpcDataContext } from './rpcDataContext';

interface Props {
  maxNum: number;
}

const LatestBlocksDegraded = ({ maxNum }: Props) => {

  const { blocks, isError, isLoading, enable } = useHomeRpcDataContext();

  React.useEffect(() => {
    enable(true, 'latest-blocks');
    return () => {
      enable(false, 'latest-blocks');
    };
  }, [ enable ]);

  if (isError || !publicClient) {
    return <LatestBlocksFallback/>;
  }

  const items = isLoading ? Array(maxNum).fill(BLOCK) : blocks.slice(0, maxNum);

  if (items.length === 0) {
    return <Box textStyle="sm">No latest blocks found.</Box>;
  }

  return (
    <>
      <Box display={{ base: 'block', lg: 'none' }}>
        { items.map((block, index) => <LatestBlocksItem key={ block.height + (isLoading ? String(index) : '') } block={ block } isLoading={ isLoading }/>) }
      </Box>
      <Box display={{ base: 'none', lg: 'block' }}>
        <HomeTable
          label="Latest blocks"
          columns={ [ { label: 'Block', width: '28%' }, { label: 'Hash / Transactions' }, { label: 'Reward', numeric: true, width: '25%' } ] }
        >
          <TableBody>
            { items.map((block, index) => <LatestBlocksItem key={ block.height + (isLoading ? String(index) : '') } block={ block } isLoading={ isLoading } view="table"/>) }
          </TableBody>
        </HomeTable>
      </Box>
    </>
  );
};

export default React.memo(LatestBlocksDegraded);
