import { Flex, Text } from '@chakra-ui/react';
import type { MouseEvent } from 'react';
import React, { useCallback } from 'react';

import type { MarketplaceApp } from 'types/client/marketplace';

import useIsMobile from 'lib/hooks/useIsMobile';
import * as mixpanel from 'lib/mixpanel/index';
import { Button } from 'toolkit/chakra/button';
import { useColorModeValue } from 'toolkit/chakra/color-mode';
import { Heading } from 'toolkit/chakra/heading';
import { IconButton } from 'toolkit/chakra/icon-button';
import { Image } from 'toolkit/chakra/image';
import { LinkBox } from 'toolkit/chakra/link';
import { Skeleton } from 'toolkit/chakra/skeleton';

import FavoriteIcon from '../FavoriteIcon';
import MarketplaceAppCardLink from '../MarketplaceAppCardLink';
import MarketplaceAppIntegrationIcon from '../MarketplaceAppIntegrationIcon';
import FeaturedAppMobile from './FeaturedAppMobile';

type FeaturedAppProps = {
  app: MarketplaceApp;
  isFavorite: boolean;
  isLoading: boolean;
  onInfoClick: (id: string) => void;
  onFavoriteClick: (id: string, isFavorite: boolean, source: 'Banner') => void;
  onAppClick: (event: MouseEvent, id: string) => void;
};

const FeaturedApp = ({
  app, isFavorite, isLoading, onAppClick,
  onInfoClick, onFavoriteClick,
}: FeaturedAppProps) => {
  const isMobile = useIsMobile();

  const { id, url, external, title, logo, logoDarkMode, shortDescription, categories, internalWallet } = app;
  const logoUrl = useColorModeValue(logo, logoDarkMode || logo);
  const categoriesLabel = categories.join(', ');

  const handleInfoClick = useCallback((event: MouseEvent) => {
    event.preventDefault();
    mixpanel.logEvent(mixpanel.EventTypes.PAGE_WIDGET, { Type: 'More button', Info: id, Source: 'Banner' });
    onInfoClick(id);
  }, [ onInfoClick, id ]);

  const handleFavoriteClick = useCallback(() => {
    onFavoriteClick(id, isFavorite, 'Banner');
  }, [ onFavoriteClick, id, isFavorite ]);

  if (isMobile) {
    return (
      <FeaturedAppMobile
        { ...app }
        onInfoClick={ handleInfoClick }
        isFavorite={ isFavorite }
        onFavoriteClick={ handleFavoriteClick }
        isLoading={ isLoading }
        onAppClick={ onAppClick }
      />
    );
  }

  return (
    <LinkBox as="article" minW={ 0 }>
      <Flex
        gap={ 4 }
        borderRadius="md"
        minH="var(--chakra-spacing-24)"
        padding={ 3 }
        background="accent.soft"
      >
        <Skeleton
          loading={ isLoading }
          w="calc(var(--chakra-spacing-1) * 19)"
          h="calc(var(--chakra-spacing-1) * 19)"
          display="flex"
          alignItems="center"
          justifyContent="center"
        >
          <Image
            src={ isLoading ? undefined : logoUrl }
            alt={ `${ title } app icon` }
            borderRadius="md"
            boxSize="full"
            objectFit="contain"
          />
        </Skeleton>

        <Flex flexDirection="column" flex={ 1 } gap={ 1 } minW={ 0 }>
          <Flex alignItems="center" gap={ 3 }>
            <Skeleton loading={ isLoading } display="flex" alignItems="center">
              <Heading level="3">
                <MarketplaceAppCardLink
                  id={ id }
                  url={ url }
                  external={ external }
                  title={ title }
                  onClick={ onAppClick }
                />
              </Heading>
              <MarketplaceAppIntegrationIcon external={ external } internalWallet={ internalWallet }/>
            </Skeleton>

            <Skeleton
              loading={ isLoading }
              color="text.secondary"
              fontSize="xs"
              flex={ 1 }
            >
              <span>{ categoriesLabel }</span>
            </Skeleton>

            { !isLoading && (
              <Button
                variant="link"
                fontSize="sm"
                fontWeight="medium"
                onClick={ handleInfoClick }
              >
                More info
              </Button>
            ) }

            { !isLoading && (
              <IconButton
                aria-label={ `Mark ${ title } as favorite` }
                aria-pressed={ isFavorite }
                title="Mark as favorite"
                variant="icon_background"
                size="md"
                onClick={ handleFavoriteClick }
                selected={ isFavorite }
              >
                <FavoriteIcon isFavorite={ isFavorite }/>
              </IconButton>
            ) }
          </Flex>

          <Skeleton
            loading={ isLoading }
            asChild
          >
            <Text lineClamp={ 2 } textStyle="sm">
              { shortDescription }
            </Text>
          </Skeleton>
        </Flex>
      </Flex>
    </LinkBox>
  );
};

export default FeaturedApp;
