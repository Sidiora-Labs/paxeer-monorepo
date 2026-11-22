import { chakra, Flex, Text } from '@chakra-ui/react';
import type { MouseEvent } from 'react';
import React, { useCallback } from 'react';

import type { MarketplaceApp } from 'types/client/marketplace';

import { Button } from 'toolkit/chakra/button';
import { useColorModeValue } from 'toolkit/chakra/color-mode';
import { Heading } from 'toolkit/chakra/heading';
import { IconButton } from 'toolkit/chakra/icon-button';
import { Image } from 'toolkit/chakra/image';
import { LinkBox } from 'toolkit/chakra/link';
import { Skeleton } from 'toolkit/chakra/skeleton';
import { isBrowser } from 'toolkit/utils/isBrowser';
import CopyToClipboard from 'ui/shared/CopyToClipboard';

import FavoriteIcon from './FavoriteIcon';
import getMarketplaceAppShareUrl from './getMarketplaceAppShareUrl';
import MarketplaceAppCardLink from './MarketplaceAppCardLink';
import MarketplaceAppGraphLinks from './MarketplaceAppGraphLinks';
import MarketplaceAppIntegrationIcon from './MarketplaceAppIntegrationIcon';
import Rating from './Rating/Rating';

interface Props extends MarketplaceApp {
  onInfoClick: (id: string) => void;
  isFavorite: boolean;
  onFavoriteClick: (id: string, isFavorite: boolean) => void;
  isLoading: boolean;
  onAppClick: (event: MouseEvent, id: string) => void;
  className?: string;
  graphLinks?: Array<{ title: string; url: string }>;
}

const MarketplaceAppCard = ({
  id,
  url,
  external,
  title,
  logo,
  logoDarkMode,
  shortDescription,
  categories,
  onInfoClick,
  isFavorite,
  onFavoriteClick,
  isLoading,
  internalWallet,
  onAppClick,
  className,
  rating,
  ratingsTotalCount,
  userRating,
  graphLinks,
}: Props) => {
  const categoriesLabel = categories.join(', ');

  const handleInfoClick = useCallback((event: MouseEvent) => {
    event.preventDefault();
    onInfoClick(id);
  }, [ onInfoClick, id ]);

  const handleFavoriteClick = useCallback(() => {
    onFavoriteClick(id, isFavorite);
  }, [ onFavoriteClick, id, isFavorite ]);

  const logoUrl = useColorModeValue(logo, logoDarkMode || logo);

  return (
    <LinkBox
      as="article"
      minW={ 0 }
      containerType="inline-size"
      bg="surface"
      className={ className }
      _hover={{
        boxShadow: isLoading ? 'none' : 'md',
      }}
      _focusWithin={{
        boxShadow: isLoading ? 'none' : 'md',
      }}
      borderRadius="base"
      padding={ 3 }
      borderWidth="1"
      borderColor="hairline"
    >
      <Flex
        flexDirection="column"
        height="100%"
        alignContent="start"
        gap={ 2 }
      >
        <Flex gap={ 4 } minW={ 0 }>
          <Skeleton
            loading={ isLoading }
            w="var(--chakra-spacing-16)"
            h="var(--chakra-spacing-16)"
            display="flex"
            alignItems="center"
            justifyContent="center"
            flexShrink={ 0 }
          >
            <Image
              src={ isLoading ? undefined : logoUrl }
              alt={ `${ title } app icon` }
              borderRadius="card"
              boxSize="full"
              objectFit="contain"
            />
          </Skeleton>

          <Flex flexDirection="column" gap={ 2 } pt={ 1 } minW={ 0 }>
            <Skeleton
              loading={ isLoading }
              display="inline-flex"
              alignItems="center"
            >
              <Heading level="3" textStyle="small">
                <MarketplaceAppCardLink
                  id={ id }
                  url={ url }
                  external={ external }
                  title={ title }
                  onClick={ onAppClick }
                  textStyle="sm"
                  fontWeight="semibold"
                />
              </Heading>
              <MarketplaceAppIntegrationIcon external={ external } internalWallet={ internalWallet }/>
              <MarketplaceAppGraphLinks
                links={ graphLinks }
                ml={ 2 }
                verticalAlign="middle"
              />
            </Skeleton>

            <Skeleton
              loading={ isLoading }
              color="text.secondary"
              fontSize="xs"
              lineHeight="label"
            >
              <span>{ categoriesLabel }</span>
            </Skeleton>
          </Flex>
        </Flex>

        <Skeleton
          loading={ isLoading }
          asChild
        >
          <Text lineClamp={ 2 } textStyle="sm">
            { shortDescription }
          </Text>
        </Skeleton>

        { !isLoading && (
          <Flex
            alignItems="center"
            justifyContent="space-between"
            flexWrap="wrap"
            gap={ 2 }
            marginTop="auto"
            position="relative"
            zIndex={ 1 }
          >
            <Button
              variant="link"
              textStyle="sm"
              fontWeight="medium"
              paddingRight={ 3 }
              h="full"
              onClick={ handleInfoClick }
            >
              Info
            </Button>
            <Flex alignItems="center" gap={ 3 }>
              <Rating
                appId={ id }
                rating={ rating }
                ratingsTotalCount={ ratingsTotalCount }
                userRating={ userRating }
                isLoading={ isLoading }
                source="Discovery"
              />
              <Flex gap={ 2 }>
                <IconButton
                  aria-label={ `${ isFavorite ? 'Remove' : 'Mark' } ${ title } ${ isFavorite ? 'from' : 'as' } favorite` }
                  aria-pressed={ isFavorite }
                  title="Mark as favorite"
                  variant="icon_background"
                  size="md"
                  onClick={ handleFavoriteClick }
                  selected={ isFavorite }
                >
                  <FavoriteIcon isFavorite={ isFavorite }/>
                </IconButton>
                <CopyToClipboard
                  text={ isBrowser() ? getMarketplaceAppShareUrl(id, window.location.origin) : '' }
                  type="share"
                  variant="icon_background"
                  size="md"
                  borderRadius="base"
                  ml={ 0 }
                  boxSize={ 8 }
                />
              </Flex>
            </Flex>
          </Flex>
        ) }
      </Flex>
    </LinkBox>
  );
};

export default React.memo(chakra(MarketplaceAppCard));
