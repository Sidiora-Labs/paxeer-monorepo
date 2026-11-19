import {
  Text,
  chakra,
  Box,
  VStack,
} from '@chakra-ui/react';
import React from 'react';

import { IconButton } from 'toolkit/chakra/icon-button';
import { Link } from 'toolkit/chakra/link';
import { PopoverRoot, PopoverTrigger, PopoverContent, PopoverBody } from 'ui/marketplace/NativePopover';
import IconSvg from 'ui/shared/IconSvg';

interface Props {
  className?: string;
  links?: Array<{ title: string; url: string }>;
}

const MarketplaceAppGraphLinks = ({ className, links }: Props) => {

  if (!links || links.length === 0) {
    return null;
  }

  const content = (
    <VStack gap={ 4 } align="start" textStyle="sm" w="full">
      <Text>{ `This dapp uses ${ links.length > 1 ? 'several subgraphs' : 'a subgraph' } powered by The Graph` }</Text>
      { links.map(link => (
        <Link external key={ link.url } href={ link.url }>{ link.title }</Link>
      )) }
    </VStack>
  );

  return (
    <Box position="relative" className={ className } display="inline-flex" alignItems="center">
      <PopoverRoot autoFocus>
        <PopoverTrigger>
          <IconButton aria-label="View subgraph links" variant="icon_secondary" size="md">
            <IconSvg name="brands/graph" boxSize={ 5 }/>
          </IconButton>
        </PopoverTrigger>
        <PopoverContent w="var(--chakra-spacing-75)" maxW="calc(100dvw - var(--chakra-spacing-8))">
          <PopoverBody>{ content }</PopoverBody>
        </PopoverContent>
      </PopoverRoot>
    </Box>
  );
};

export default React.memo(chakra(MarketplaceAppGraphLinks));
