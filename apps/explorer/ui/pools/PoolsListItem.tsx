import { Box } from '@chakra-ui/react';
import React from 'react';

import type { Pool } from 'types/api/pools';

import getPoolLinks from 'lib/pools/getPoolLinks';
import { Image } from 'toolkit/chakra/image';
import { Link } from 'toolkit/chakra/link';
import { Skeleton } from 'toolkit/chakra/skeleton';
import { TruncatedText } from 'toolkit/components/truncation/TruncatedText';
import { spacing } from 'toolkit/theme/foundations/spacing';
import CopyToClipboard from 'ui/shared/CopyToClipboard';
import AddressEntity from 'ui/shared/entities/address/AddressEntity';
import PoolEntity from 'ui/shared/entities/pool/PoolEntity';
import ListItemMobileGrid from 'ui/shared/ListItemMobile/ListItemMobileGrid';

type Props = {
  item: Pool;
  isLoading?: boolean;
};

const PoolsListItem = ({ item, isLoading }: Props) => {
  const externalLinks = getPoolLinks(item);
  return (
    <Box containerType="inline-size" w="full" minW={ 0 }>
      <ListItemMobileGrid.Container
        gridTemplateColumns="minmax(0, 1fr)"
        css={{ [ `@container (min-width: ${ spacing['80'].value })` ]: { gridTemplateColumns: 'calc(var(--chakra-spacing-1) * 25) minmax(0, 1fr)' } }}
      >

        <ListItemMobileGrid.Label isLoading={ isLoading }>Pool</ListItemMobileGrid.Label>
        <ListItemMobileGrid.Value>
          <PoolEntity pool={ item } fontWeight="bold" isLoading={ isLoading }/>
        </ListItemMobileGrid.Value>

        { item.is_contract && (
          <>
            <ListItemMobileGrid.Label isLoading={ isLoading }>Contract</ListItemMobileGrid.Label>
            <ListItemMobileGrid.Value>
              <AddressEntity
                address={{ hash: item.pool_id }}
                noIcon
                link={{ variant: 'secondary' }}
                isLoading={ isLoading }
                truncation="tail"
              />
            </ListItemMobileGrid.Value>
          </>
        ) }

        { !item.is_contract && (
          <>
            <ListItemMobileGrid.Label isLoading={ isLoading }>Pool ID</ListItemMobileGrid.Label>
            <ListItemMobileGrid.Value display="flex" alignItems="center" gap={ 2 }>
              <TruncatedText text={ item.pool_id } minW={ 0 } maxW="full"/>
              <CopyToClipboard text={ item.pool_id }/>
            </ListItemMobileGrid.Value>
          </>
        ) }

        <ListItemMobileGrid.Label isLoading={ isLoading }>Liquidity</ListItemMobileGrid.Label>
        <ListItemMobileGrid.Value>
          <Skeleton loading={ isLoading }>
            ${ Number(item.liquidity).toLocaleString(undefined, { maximumFractionDigits: 2, notation: 'compact' }) }
          </Skeleton>
        </ListItemMobileGrid.Value>

        <ListItemMobileGrid.Label isLoading={ isLoading }>View in</ListItemMobileGrid.Label>
        <ListItemMobileGrid.Value>
          <Skeleton loading={ isLoading }>
            { externalLinks.map((link) => (
              <Link external href={ link.url } key={ link.url } display="inline-flex">
                <Image src={ link.image } alt={ link.title } boxSize={ 5 } mr={ 2 }/>
                { link.title }
              </Link>
            )) }
          </Skeleton>
        </ListItemMobileGrid.Value>
      </ListItemMobileGrid.Container>
    </Box>
  );
};

export default PoolsListItem;
