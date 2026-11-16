import { Text, Flex, Box, VStack } from '@chakra-ui/react';
import React from 'react';

import type { ArbitrumL2TxnBatchDAAnytrust } from 'types/api/arbitrumL2';

import dayjs from 'lib/date/dayjs';
import { TableBody, TableCell, TableColumnHeader, TableHeader, TableRow } from 'toolkit/chakra/table';
import CopyToClipboard from 'ui/shared/CopyToClipboard';
import DetailsTimestamp from 'ui/shared/DetailedInfo/DetailedInfoTimestamp';
import HashStringShorten from 'ui/shared/HashStringShorten';
import IconSvg from 'ui/shared/IconSvg';
import TextSeparator from 'ui/shared/TextSeparator';
import * as DetailedInfo from 'ui/tx/ActivityDetails';
import ActivityTable from 'ui/txs/ActivityTable';
import ActivityViews from 'ui/txs/ActivityViews';

type Props = {
  data: ArbitrumL2TxnBatchDAAnytrust;
};

const ArbitrumL2TxnBatchDetailsAnyTrustDA = ({ data }: Props) => {
  return (
    <>
      <DetailedInfo.ItemLabel
        hint="Aggregated BLS signature of AnyTrust committee members"
      >
        Signature
      </DetailedInfo.ItemLabel><DetailedInfo.ItemValue wordBreak="break-all" whiteSpace="break-spaces">
        { data.bls_signature }
      </DetailedInfo.ItemValue><DetailedInfo.ItemLabel
        hint="The hash of the data blob stored by the AnyTrust committee"
      >
        Data hash
      </DetailedInfo.ItemLabel>
      <DetailedInfo.ItemValue whiteSpace="pre-wrap" wordBreak="break-all" alignItems={{ base: 'flex-start', lg: 'center' }}>
        { data.data_hash }
        <CopyToClipboard text={ data.data_hash } ml={ 2 }/>
      </DetailedInfo.ItemValue>
      <DetailedInfo.ItemLabel
        hint="Expiration timeout for the data blob"
      >
        Timeout
      </DetailedInfo.ItemLabel><DetailedInfo.ItemValue multiRow>
        { dayjs(data.timeout) < dayjs() ?
          <DetailsTimestamp timestamp={ data.timeout }/> :
          (
            <>
              <DetailsTimestamp timestamp={ data.timeout } noRelativeTime/>
              <TextSeparator/>
              <Text color="negative">{ dayjs(data.timeout).diff(dayjs(), 'day') } days left</Text>
            </>
          ) }
      </DetailedInfo.ItemValue>
      <DetailedInfo.ItemLabel
        hint="Members of AnyTrust committee"
      >
        Signers
      </DetailedInfo.ItemLabel>
      <DetailedInfo.ItemValue fontSize="sm">
        <ActivityViews
          tableBreakpoint="sm"
          table={ (
            <ActivityTable label="AnyTrust signers" minW="xl">
              <TableHeader>
                <TableRow>
                  <TableColumnHeader>Key</TableColumnHeader>
                  <TableColumnHeader w="var(--chakra-spacing-24)">Trusted</TableColumnHeader>
                  <TableColumnHeader w="var(--chakra-spacing-40)">Proof</TableColumnHeader>
                </TableRow>
              </TableHeader>
              <TableBody>
                { data.signers.map(signer => (
                  <TableRow key={ signer.key }>
                    <TableCell>
                      <Flex minW={ 0 } alignItems="flex-start">
                        <Text wordBreak="break-all" whiteSpace="break-spaces">{ signer.key }</Text>
                        <CopyToClipboard text={ signer.key } ml={ 2 }/>
                      </Flex>
                    </TableCell>
                    <TableCell>
                      <IconSvg name={ signer.trusted ? 'check' : 'cross' } boxSize={ 6 } role="img" aria-label={ signer.trusted ? 'Trusted' : 'Not trusted' }/>
                    </TableCell>
                    <TableCell>
                      { signer.proof ? (
                        <Flex minW={ 0 } maxW="full" flexWrap="wrap" alignItems="center" gap={ 2 }>
                          <HashStringShorten hash={ signer.proof }/>
                          <CopyToClipboard text={ signer.proof }/>
                        </Flex>
                      ) : '-' }
                    </TableCell>
                  </TableRow>
                )) }
              </TableBody>
            </ActivityTable>
          ) }
          list={ (
            <Box backgroundColor="bg.surface" borderRadius="md" minW={ 0 }>
              { data.signers.map(signer => (
                <VStack padding={ 4 } key={ signer.key } gap={ 2 } alignItems="stretch">
                  <Flex w="full" justifyContent="space-between" alignItems="center">
                    <Text fontWeight="semibold">Key</Text>
                    <CopyToClipboard text={ signer.key }/>
                  </Flex>
                  <Text wordBreak="break-all" whiteSpace="break-spaces">{ signer.key }</Text>
                  <Flex w="full" flexWrap="wrap" gap={ 4 }>
                    <Flex alignItems="center" gap={ 2 }>
                      <Text fontWeight="semibold">Trusted</Text>
                      <IconSvg name={ signer.trusted ? 'check' : 'cross' } boxSize={ 6 } role="img" aria-label={ signer.trusted ? 'Trusted' : 'Not trusted' }/>
                    </Flex>
                    <Flex alignItems="center" minW={ 0 } maxW="full" flexWrap="wrap" gap={ 2 }>
                      <Text fontWeight="semibold">Proof</Text>
                      { signer.proof ? (
                        <Flex minW={ 0 } maxW="full" flexWrap="wrap" alignItems="center" gap={ 2 }>
                          <HashStringShorten hash={ signer.proof }/>
                          <CopyToClipboard text={ signer.proof }/>
                        </Flex>
                      ) : '-' }
                    </Flex>
                  </Flex>
                </VStack>
              )) }
            </Box>
          ) }
        />
      </DetailedInfo.ItemValue>
    </>
  );
};

export default ArbitrumL2TxnBatchDetailsAnyTrustDA;
