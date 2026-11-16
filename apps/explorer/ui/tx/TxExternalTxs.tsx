import { Box, Flex } from '@chakra-ui/react';
import React from 'react';

import config from 'configs/app';
import { Button } from 'toolkit/chakra/button';
import { Image } from 'toolkit/chakra/image';
import { PopoverBody, PopoverContent, PopoverRoot, PopoverTrigger } from 'toolkit/chakra/popover';
import TxEntity from 'ui/shared/entities/tx/TxEntity';

const externalTxFeature = config.features.externalTxs;

interface Props {
  data: Array<string>;
}

const TxExternalTxs: React.FC<Props> = ({ data }) => {
  if (!externalTxFeature.isEnabled) {
    return null;
  }

  const content = (
    <Box textStyle="sm">
      <Flex alignItems="center" gap={ 2 } fontSize="md" mb={ 3 }>
        <Image src={ externalTxFeature.chainLogoUrl } alt={ externalTxFeature.chainName } width={ 5 } height={ 5 }/>
        { externalTxFeature.chainName } transaction{ data.length > 1 ? 's' : '' }
      </Flex>
      <Flex
        role="region"
        aria-label={ `${ externalTxFeature.chainName } transaction hashes` }
        tabIndex={ 0 } flexDirection="column" gap={ 2 } w="100%"
        maxHeight="var(--chakra-spacing-100)" overflowY="auto"
      >
        { data.map((txHash) => (
          <TxEntity
            key={ txHash }
            hash={ txHash }
            href={ externalTxFeature.explorerUrlTemplate.replace('{hash}', txHash) }
            link={{ external: true }}
            noCopy
            noTooltip
          />
        )) }
      </Flex>
    </Box>
  );

  return (
    <PopoverRoot autoFocus positioning={{ placement: 'bottom-end' }}>
      <PopoverTrigger>
        <Button
          type="button"
          aria-label={ `${ data.length } ${ externalTxFeature.chainName } txn${ data.length > 1 ? 's' : '' }` }
          variant="link"
          display="inline-flex"
          alignItems="center"
          gap={ 2 }
          textDecorationLine="underline"
          textDecorationStyle="dashed"
        >
          <Image src={ externalTxFeature.chainLogoUrl } alt={ externalTxFeature.chainName } boxSize={ 5 }/>
          { `${ data.length } ${ externalTxFeature.chainName } txn${ data.length > 1 ? 's' : '' }` }
        </Button>
      </PopoverTrigger>
      <PopoverContent
        aria-label={ `${ externalTxFeature.chainName } transactions` }
        w={{ base: 'var(--chakra-spacing-75)', lg: 'var(--chakra-spacing-100)' }}
        maxW="calc(100dvw - var(--chakra-spacing-8))"
        maxH="min(var(--available-height, 100dvh), calc(100dvh - var(--chakra-spacing-8)))"
        overflowY="auto"
        overscrollBehavior="contain"
      >
        <PopoverBody>{ content }</PopoverBody>
      </PopoverContent>
    </PopoverRoot>
  );
};

export default TxExternalTxs;
