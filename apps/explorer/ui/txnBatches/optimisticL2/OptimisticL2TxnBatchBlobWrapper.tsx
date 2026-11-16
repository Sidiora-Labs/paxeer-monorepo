import { chakra, Grid } from '@chakra-ui/react';
import React from 'react';

interface Props {
  children: React.ReactNode;
  className?: string;
  isLoading: boolean;
}

const OptimisticL2TxnBatchBlobWrapper = ({ children, className, isLoading }: Props) => {
  return (
    <Grid
      className={ className }
      columnGap={ 3 }
      rowGap="var(--chakra-spacing-2\.5)"
      p={ 4 }
      bgColor="bg.surface"
      gridTemplateColumns="minmax(0, max-content) minmax(0, 1fr)"
      borderRadius="base"
      w="100%"
      minH={ isLoading ? 'var(--chakra-spacing-36)' : undefined }
      textStyle="sm"
      css={{ '& > *': { minW: 0, overflowWrap: 'anywhere' } }}
    >
      { isLoading ? null : children }
    </Grid>
  );
};

export default React.memo(chakra(OptimisticL2TxnBatchBlobWrapper));
