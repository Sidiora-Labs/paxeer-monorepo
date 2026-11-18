import { Box, Flex, chakra } from '@chakra-ui/react';
import React from 'react';

import { Heading } from 'toolkit/chakra/heading';
import { Skeleton } from 'toolkit/chakra/skeleton';
import AddressQrCode from 'ui/address/details/AddressQrCode';
import TextAd from 'ui/shared/ad/TextAd';
import CopyToClipboard from 'ui/shared/CopyToClipboard';

type Props = {
  title: string;
  kind?: string;
  identifier?: string;
  identifierContent?: React.ReactNode;
  tags?: React.ReactNode;
  className?: string;
  beforeTitle?: React.ReactNode;
  afterTitle?: React.ReactNode;
  contentAfter?: React.ReactNode;
  secondRow?: React.ReactNode;
  isLoading?: boolean;
  withTextAd?: boolean;
};

const PageTitle = ({
  title, kind, identifier, identifierContent, tags, contentAfter, withTextAd, className,
  isLoading = false, afterTitle, beforeTitle, secondRow,
}: Props) => {
  return (
    <Flex className={ className } flexDir="column" rowGap={ 3 } mb={ 6 } minW={ 0 } maxW="100%" data-entity-header={ kind ? true : undefined }>
      { kind && <Box textStyle="label" color="text.muted" data-entity-kind-label>{ kind.toUpperCase() }</Box> }
      <Flex
        flexDir="row"
        flexWrap="wrap"
        rowGap={ 3 }
        columnGap={ 3 }
        alignItems="center"
      >
        <Flex
          maxW="100%"
          minW={ 0 }
          alignItems="center"
          flexWrap="wrap"
          columnGap={ 2 }
          rowGap={ 2 }
          data-title-row
        >
          { beforeTitle }
          <Skeleton
            loading={ isLoading }
            minW={ 0 }
            maxW="100%"
          >
            <Heading
              level="1"
              textStyle="h1"
              whiteSpace="normal"
              overflowWrap="anywhere"
            >
              { title }
            </Heading>
          </Skeleton>
          { afterTitle && (
            <Box minW={ 0 } maxW="100%" flexBasis={{ base: '100%', lg: 'auto' }} data-title-after>
              { afterTitle }
            </Box>
          ) }
        </Flex>
        { contentAfter }
        { withTextAd && <TextAd order={{ base: -1, lg: 100 }} mb={{ base: 6, lg: 0 }} ml="auto" w={{ base: '100%', lg: 'auto' }}/> }
      </Flex>
      { identifier && (
        <Flex alignItems="center" flexWrap="wrap" gap={ 2 } minW={ 0 } maxW="100%" fontFamily="mono" fontVariantNumeric="tabular-nums" data-entity-identifier>
          { identifierContent ?? (
            <>
              <Box overflowWrap="anywhere" minW={ 0 }>{ identifier }</Box>
              <CopyToClipboard text={ identifier } isLoading={ isLoading }/>
              <AddressQrCode hash={ identifier } isLoading={ isLoading }/>
            </>
          ) }
        </Flex>
      ) }
      { tags && (
        <Flex gap={ 2 } flexWrap="wrap" alignItems="center" data-entity-tags css={{ '& [data-scope="tag"]': { borderRadius: 'full' } }}>
          { tags }
        </Flex>
      ) }
      { secondRow && (
        <Skeleton
          loading={ isLoading }
          alignItems="center"
          minH={ 10 }
          overflow="hidden"
          display="flex"
          flexWrap={{ base: 'wrap', lg: 'nowrap' }}
          rowGap={ 2 }
          minW={ 0 }
          maxW="100%"
          data-title-second-row
          _empty={{ display: 'none' }}
        >
          { secondRow }
        </Skeleton>
      ) }
    </Flex>
  );
};

export default chakra(PageTitle);
