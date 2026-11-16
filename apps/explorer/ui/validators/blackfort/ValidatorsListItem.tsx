import React from 'react';

import type { ValidatorBlackfort } from 'types/api/validators';

import { Skeleton } from 'toolkit/chakra/skeleton';
import AddressEntity from 'ui/shared/entities/address/AddressEntity';
import ListItemMobileGrid from 'ui/shared/ListItemMobile/ListItemMobileGrid';
import NativeCoinValue from 'ui/shared/value/NativeCoinValue';

interface Props {
  data: ValidatorBlackfort;
  isLoading?: boolean;
}

const ValidatorsListItem = ({ data, isLoading }: Props) => {

  return (
    <ListItemMobileGrid.Container gridTemplateColumns="minmax(0, 1fr)" borderColor="hairline"
      css={{ '& a': { display: 'inline-flex', alignItems: 'center' },
        '& a, & button': { minWidth: 'touchTarget', minHeight: 'touchTarget', _motionReduce: { transitionProperty: 'none' } } }}>

      <ListItemMobileGrid.Label isLoading={ isLoading }>Validator</ListItemMobileGrid.Label>
      <ListItemMobileGrid.Value>
        <AddressEntity
          isLoading={ isLoading }
          address={ data.address }
          truncation="constant"
        />
      </ListItemMobileGrid.Value>

      { data.name && (
        <>
          <ListItemMobileGrid.Label isLoading={ isLoading }>Name</ListItemMobileGrid.Label>
          <ListItemMobileGrid.Value>
            <Skeleton loading={ isLoading } whiteSpace="normal" overflowWrap="anywhere">{ data.name }</Skeleton>
          </ListItemMobileGrid.Value>
        </>
      ) }

      { data.commission != null && (
        <>
          <ListItemMobileGrid.Label isLoading={ isLoading }>Commission</ListItemMobileGrid.Label>
          <ListItemMobileGrid.Value>
            <Skeleton loading={ isLoading }>
              { `${ data.commission / 100 }%` }
            </Skeleton>
          </ListItemMobileGrid.Value>
        </>
      ) }

      { data.self_bonded_amount != null && (
        <>
          <ListItemMobileGrid.Label isLoading={ isLoading }>Stake</ListItemMobileGrid.Label>
          <ListItemMobileGrid.Value>
            <NativeCoinValue
              amount={ data.self_bonded_amount }
              loading={ isLoading }
            />
          </ListItemMobileGrid.Value>
        </>
      ) }

      { data.delegated_amount != null && (
        <>
          <ListItemMobileGrid.Label isLoading={ isLoading }>Delegated amount</ListItemMobileGrid.Label>
          <ListItemMobileGrid.Value>
            <NativeCoinValue
              amount={ data.delegated_amount }
              loading={ isLoading }
            />
          </ListItemMobileGrid.Value>
        </>
      ) }

    </ListItemMobileGrid.Container>
  );
};

export default React.memo(ValidatorsListItem);
