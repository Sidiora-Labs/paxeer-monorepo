import { Flex, Text } from '@chakra-ui/react';
import type { MouseEvent } from 'react';
import React from 'react';

import type { MarketplaceApp } from 'types/client/marketplace';

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

interface Props extends MarketplaceApp {
  onInfoClick: (event: MouseEvent) => void;
  isFavorite: boolean;
  onFavoriteClick: () => void;
  isLoading: boolean;
  onAppClick: (event: MouseEvent, id: string) => void;
}

const FeaturedAppMobile = ({
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
}: Props) => {
  const categoriesLabel = categories.join(', ');

  const logoUrl = useColorModeValue(logo, logoDarkMode || logo);

  return (
    <LinkBox
      as="article"
      minW={ 0 }
      borderRadius="md"
      padding={{ base: 3, sm: 'var(--chakra-spacing-5)' }}
      role="group"
      background="accent.soft"
    >
      <Flex
        flexDirection="row"
        height="100%"
        alignContent="start"
        gap={ 4 }
      >
        <Flex
          flexDirection="column"
          alignItems="center"
          justifyContent="space-between"
        >
          <Skeleton
            loading={ isLoading }
            w={{ base: 'var(--chakra-spacing-16)', sm: 'var(--chakra-spacing-24)' }}
            h={{ base: 'var(--chakra-spacing-16)', sm: 'var(--chakra-spacing-24)' }}
            display="flex"
            alignItems="center"
            justifyContent="center"
          >
            <Image
              src={ isLoading ? undefined : logoUrl }
              alt={ `${ title } app icon` }
              borderRadius="card"
              boxSize="full"
              objectFit="contain"
            />
          </Skeleton>

          { !isLoading && (
            <Flex
              position={{ base: 'relative', sm: 'absolute' }}
              right={{ base: 0, sm: 'calc(var(--chakra-spacing-1) * 12.5)' }}
              top={{ base: 0, sm: 'var(--chakra-spacing-6)' }}
            >
              <Button
                variant="link"
                fontSize={{ base: 'xs', sm: 'sm' }}
                fontWeight="medium"
                paddingRight={{ sm: 2 }}
                onClick={ onInfoClick }
              >
                More info
              </Button>
            </Flex>
          ) }
        </Flex>

        <Flex flexDirection="column" gap={ 2 }>
          <Skeleton
            loading={ isLoading }
            paddingRight={{ base: 'calc(var(--chakra-spacing-1) * 6.25)', sm: 'calc(var(--chakra-spacing-1) * 27.5)' }}
            display="flex"
            alignItems="center"
          >
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
            textStyle="xs"
          >
            <span>{ categoriesLabel }</span>
          </Skeleton>

          <Skeleton
            loading={ isLoading }
            asChild
          >
            <Text lineClamp={ 3 } textStyle="xs">
              { shortDescription }
            </Text>
          </Skeleton>
        </Flex>

        { !isLoading && (
          <IconButton
            display="flex"
            alignItems="center"
            justifyContent="center"
            position="absolute"
            right={{ base: 1, sm: 'calc(var(--chakra-spacing-1) * 2.5)' }}
            top={{ base: 1, sm: 'calc(var(--chakra-spacing-1) * 4.5)' }}
            aria-label={ `Mark ${ title } as favorite` }
            aria-pressed={ isFavorite }
            title="Mark as favorite"
            variant="icon_background"
            size="md"
            onClick={ onFavoriteClick }
            selected={ isFavorite }
          >
            <FavoriteIcon isFavorite={ isFavorite }/>
          </IconButton>
        ) }
      </Flex>
    </LinkBox>
  );
};

export default React.memo(FeaturedAppMobile);
