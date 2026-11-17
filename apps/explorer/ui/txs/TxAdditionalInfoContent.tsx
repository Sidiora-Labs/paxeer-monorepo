import { Box, Text, Flex } from '@chakra-ui/react';
import BigNumber from 'bignumber.js';
import React from 'react';

import type { Transaction } from 'types/api/transaction';

import { route } from 'nextjs/routes';

import config from 'configs/app';
import { useMultichainContext } from 'lib/contexts/multichain';
import { currencyUnits } from 'lib/units';
import { Link } from 'toolkit/chakra/link';
import BlobEntity from 'ui/shared/entities/blob/BlobEntity';
import TxStatus from 'ui/shared/statusTag/TxStatus';
import TextSeparator from 'ui/shared/TextSeparator';
import TxFee from 'ui/shared/tx/TxFee';
import Utilization from 'ui/shared/Utilization/Utilization';
import NativeCoinValue from 'ui/shared/value/NativeCoinValue';

const TxAdditionalInfoContent = ({ tx }: { tx: Transaction }) => {
  const multichainContext = useMultichainContext();

  const sectionProps = {
    borderBottomWidth: 'var(--chakra-border-widths-1)', borderBottomStyle: 'solid',
    borderColor: 'border.divider',
    paddingBottom: 4,
  };

  const sectionTitleProps = {
    color: 'text.secondary',
    fontWeight: 'semibold',
    marginBottom: 3,
  };

  return (
    <>
      <TxStatus status={ tx.status } errorText={ tx.status === 'error' ? tx.result : undefined } mb={ 3 }/>
      { tx.blob_versioned_hashes && tx.blob_versioned_hashes.length > 0 && (
        <Box { ...sectionProps } mb={ 4 }>
          <Flex alignItems="center" justifyContent="space-between">
            <Text { ...sectionTitleProps }>Blobs: { tx.blob_versioned_hashes.length }</Text>
            { tx.blob_versioned_hashes.length > 3 && (
              <Link
                href={ route({ pathname: '/tx/[hash]', query: { hash: tx.hash, tab: 'blobs' } }) }
                mb={ 3 }
              >
                view all
              </Link>
            ) }
          </Flex>
          <Flex flexDir="column" rowGap={ 3 }>
            { tx.blob_versioned_hashes.slice(0, 3).map((hash, index) => (
              <Flex key={ hash } columnGap={ 2 }>
                <Box fontWeight="medium">{ index + 1 }</Box>
                <BlobEntity hash={ hash } noIcon/>
              </Flex>
            )) }
          </Flex>
        </Box>
      ) }
      <Box { ...sectionProps } mb={ 4 }>
        <Text { ...sectionTitleProps }>Value</Text>
        <NativeCoinValue
          amount={ tx.value }
          exchangeRate={ tx.exchange_rate }
          historicalExchangeRate={ tx.historic_exchange_rate }
          noTooltip
        />
      </Box>
      { !config.UI.views.tx.hiddenFields?.tx_fee && (tx.stability_fee !== undefined || tx.fee.value !== null) && (
        <Box { ...sectionProps } mb={ 4 }>
          <Text { ...sectionTitleProps }>Transaction fee</Text>
          <TxFee tx={ tx } rowGap={ 0 } noTooltip/>
        </Box>
      ) }
      { tx.gas_used !== null && (
        <Box { ...sectionProps } mb={ 4 }>
          <Text { ...sectionTitleProps }>Gas limit & usage by transaction</Text>
          <Flex flexWrap="wrap" alignItems="center" gap={ 2 }>
            <Text>{ BigNumber(tx.gas_used).toFormat() }</Text>
            <TextSeparator/>
            <Text>{ BigNumber(tx.gas_limit).toFormat() }</Text>
            <Utilization ml={ 4 } value={ Number(BigNumber(tx.gas_used).dividedBy(BigNumber(tx.gas_limit)).toFixed(2)) }/>
          </Flex>
        </Box>
      ) }
      { !config.UI.views.tx.hiddenFields?.gas_fees &&
        (tx.base_fee_per_gas !== null || tx.max_fee_per_gas !== null || tx.max_priority_fee_per_gas !== null) && (
        <Box { ...sectionProps } mb={ 4 }>
          <Text { ...sectionTitleProps }>Gas fees ({ currencyUnits.gwei })</Text>
          { tx.base_fee_per_gas !== null && (
            <Box>
              <Text as="span" fontWeight="medium">Base: </Text>
              <NativeCoinValue
                amount={ tx.base_fee_per_gas }
                units="gwei"
                unitsTooltip="wei"
                noSymbol
                fontWeight="bold"
              />
            </Box>
          ) }
          { tx.max_fee_per_gas !== null && (
            <Box mt={ 1 }>
              <Text as="span" fontWeight="medium">Max: </Text>
              <NativeCoinValue
                amount={ tx.max_fee_per_gas }
                units="gwei"
                unitsTooltip="wei"
                noSymbol
                fontWeight="bold"
              />
            </Box>
          ) }
          { tx.max_priority_fee_per_gas !== null && (
            <Box mt={ 1 }>
              <Text as="span" fontWeight="medium">Max priority: </Text>
              <NativeCoinValue
                amount={ tx.max_priority_fee_per_gas }
                units="gwei"
                unitsTooltip="wei"
                noSymbol
                fontWeight="bold"
              />
            </Box>
          ) }
        </Box>
      ) }
      { !(tx.blob_versioned_hashes && tx.blob_versioned_hashes.length > 0) && (
        <Box { ...sectionProps } mb={ 4 }>
          <Text { ...sectionTitleProps }>Others</Text>
          <Box>
            <Text as="span" fontWeight="medium">Txn type: </Text>
            <Text fontWeight="semibold" as="span">{ tx.type }</Text>
            { tx.type === 2 && <Text fontWeight="normal" as="span" ml={ 1 } color="text.secondary">(EIP-1559)</Text> }
          </Box>
          <Box mt={ 1 }>
            <Text as="span" fontWeight="medium">Nonce: </Text>
            <Text fontWeight="semibold" as="span">{ tx.nonce }</Text>
          </Box>
          <Box mt={ 1 }>
            <Text as="span" fontWeight="medium">Position: </Text>
            <Text fontWeight="semibold" as="span">{ tx.position }</Text>
          </Box>
        </Box>
      ) }
      <Link href={ route({ pathname: '/tx/[hash]', query: { hash: tx.hash } }, multichainContext) }>More details</Link>
    </>
  );
};

export default React.memo(TxAdditionalInfoContent);
