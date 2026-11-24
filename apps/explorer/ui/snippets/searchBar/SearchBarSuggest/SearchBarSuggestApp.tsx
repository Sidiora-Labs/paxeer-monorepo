import { Flex, Text, Grid } from '@chakra-ui/react';
import { useRouter } from 'next/router';
import React from 'react';

import type { MarketplaceApp } from 'types/client/marketplace';

import { route } from 'nextjs-routes';

import highlightText from 'lib/highlightText';
import { useColorModeValue } from 'toolkit/chakra/color-mode';
import { Image } from 'toolkit/chakra/image';
import IconSvg from 'ui/shared/IconSvg';

import { resultLayout } from './resultLayout';
import SearchBarSuggestItemLink from './SearchBarSuggestItemLink';
interface Props {
  data: MarketplaceApp;
  isMobile: boolean | undefined;
  searchTerm: string;
  onClick: (event: React.MouseEvent<HTMLAnchorElement>) => void;
}

const SearchBarSuggestApp = ({ data, searchTerm, onClick }: Props) => {
  const router = useRouter();
  const logo = (
    <Image
      borderRadius="base"
      boxSize={ 5 }
      src={ useColorModeValue(data.logo, data.logoDarkMode || data.logo) }
      alt={ `${ data.title } app icon` }
    />
  );

  const content = (
    <Grid gap={ 2 } css={ resultLayout }>
      <Flex alignItems="center" gap={ 2 } minW={ 0 }>
        { logo }
        <Text fontWeight="bold" overflow="hidden" whiteSpace="nowrap" textOverflow="ellipsis">
          <span dangerouslySetInnerHTML={{ __html: highlightText(data.title, searchTerm) }}/>
        </Text>
        { data.external && <IconSvg aria-hidden={ true } name="link_external" color="icon.secondary" boxSize={ 3 } flexShrink={ 0 }/> }
      </Flex>
      <Text color="text.secondary" overflowWrap="anywhere">{ data.description }</Text>
    </Grid>
  );

  return (
    <SearchBarSuggestItemLink
      onClick={ onClick }
      href={ data.external ? route({ pathname: '/apps', query: { selectedAppId: data.id } }) : route({ pathname: '/apps/[id]', query: { id: data.id } }) }
      shallow={ data.external && router.pathname === '/apps' }
    >
      { content }
    </SearchBarSuggestItemLink>
  );
};

export default React.memo(SearchBarSuggestApp);
