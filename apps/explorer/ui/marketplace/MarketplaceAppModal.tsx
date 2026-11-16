import { Box, Flex, Text } from '@chakra-ui/react';
import React, { useCallback } from 'react';

import type { MarketplaceApp } from 'types/client/marketplace';

import { route } from 'nextjs-routes';

import config from 'configs/app';
import { Badge } from 'toolkit/chakra/badge';
import { Button } from 'toolkit/chakra/button';
import { useColorModeValue } from 'toolkit/chakra/color-mode';
import { Heading } from 'toolkit/chakra/heading';
import { IconButton } from 'toolkit/chakra/icon-button';
import { Image } from 'toolkit/chakra/image';
import { Link } from 'toolkit/chakra/link';
import { nbsp } from 'toolkit/utils/htmlEntities';
import { isBrowser } from 'toolkit/utils/isBrowser';
import { makePrettyLink } from 'toolkit/utils/url';
import { DialogBody, DialogCloseTrigger, DialogContent, DialogFooter, DialogRoot, DialogTitle } from 'ui/marketplace/NativeDialog';
import CopyToClipboard from 'ui/shared/CopyToClipboard';
import type { IconName } from 'ui/shared/IconSvg';
import IconSvg from 'ui/shared/IconSvg';

import FavoriteIcon from './FavoriteIcon';
import getMarketplaceAppShareUrl from './getMarketplaceAppShareUrl';
import MarketplaceAppGraphLinks from './MarketplaceAppGraphLinks';
import MarketplaceAppIntegrationIcon from './MarketplaceAppIntegrationIcon';
import Rating from './Rating/Rating';

const feature = config.features.marketplace;
const isRatingEnabled = feature.isEnabled && 'api' in feature;

type Props = {
  onClose: () => void;
  isFavorite: boolean;
  onFavoriteClick: (id: string, isFavorite: boolean, source: 'App modal') => void;
  data: MarketplaceApp;
  graphLinks?: Array<{ title: string; url: string }>;
};

const MarketplaceAppModal = ({
  onClose,
  isFavorite,
  onFavoriteClick,
  data,
  graphLinks,
}: Props) => {
  const {
    id,
    title,
    url,
    external,
    author,
    description,
    site,
    github,
    telegram,
    twitter,
    discord,
    logo,
    logoDarkMode,
    categories,
    rating,
    ratingsTotalCount,
    userRating,
    internalWallet,
  } = data;

  const socialLinks = [
    telegram ? {
      icon: 'social/telegram_filled' as IconName,
      url: telegram,
    } : null,
    twitter ? {
      icon: 'social/twitter_filled' as IconName,
      url: twitter,
    } : null,
    discord ? {
      icon: 'social/discord_filled' as IconName,
      url: discord,
    } : null,
  ].filter(Boolean);

  if (github) {
    if (Array.isArray(github)) {
      github.forEach((url) => socialLinks.push({ icon: 'social/github_filled', url }));
    } else {
      socialLinks.push({ icon: 'social/github_filled', url: github });
    }
  }

  const handleOpenChange = React.useCallback(({ open }: { open: boolean }) => {
    if (!open) {
      onClose();
    }
  }, [ onClose ]);

  const handleFavoriteClick = useCallback(() => {
    onFavoriteClick(id, isFavorite, 'App modal');
  }, [ onFavoriteClick, id, isFavorite ]);

  const logoUrl = useColorModeValue(logo, logoDarkMode || logo);

  return (
    <DialogRoot
      open={ Boolean(data.id) }
      onOpenChange={ handleOpenChange }
      size={{ lgDown: 'full', lg: 'md' }}
    >
      <DialogContent>
        <Box
          display="grid"
          gridTemplateColumns="auto minmax(0, 1fr)"
          marginBottom={{ base: 6, md: 8 }}
        >
          <Flex
            alignItems="center"
            justifyContent="center"
            w={{ base: 'var(--chakra-spacing-18)', md: 'calc(var(--chakra-spacing-1) * 36)' }}
            h={{ base: 'var(--chakra-spacing-18)', md: 'calc(var(--chakra-spacing-1) * 36)' }}
            marginRight={{ base: 6, md: 8 }}
            gridRow={{ base: '1 / 3', md: '1 / 5' }}
          >
            <Image
              src={ logoUrl }
              alt={ `${ title } app icon` }
              borderRadius="md"
              boxSize="full"
              objectFit="contain"
            />
          </Flex>

          <Flex alignItems="center" flexWrap="wrap" gap={ 1 } minW={ 0 } mb={{ md: 2 }} gridColumn={ 2 }>
            <DialogTitle asChild>
              <Heading
                level="2"
                fontWeight="medium"
                whiteSpace="normal"
                overflowWrap="anywhere"
                mr={ 2 }
              >
                { title }
              </Heading>
            </DialogTitle>
            <MarketplaceAppIntegrationIcon external={ external } internalWallet={ internalWallet }/>
            <MarketplaceAppGraphLinks links={ graphLinks } ml={ 2 }/>
            <DialogCloseTrigger ml="auto"/>
          </Flex>

          <Text
            color="text.secondary"
            gridColumn={ 2 }
            textStyle={{ base: 'sm', md: 'md' }}
            fontWeight="normal"
            overflowWrap="anywhere"
          >
            By{ nbsp }{ author }
          </Text>

          { isRatingEnabled && (
            <Box
              gridColumn={{ base: '1 / 3', md: 2 }}
              marginTop={{ base: 6, md: 3 }}
              py={{ base: 0, md: 1.5 }}
              width="fit-content"
            >
              <Rating
                appId={ id }
                rating={ rating }
                ratingsTotalCount={ ratingsTotalCount }
                userRating={ userRating }
                fullView
                source="App modal"
                popoverContentProps={{ portalled: false, zIndex: 'modal' }}
              />
            </Box>
          ) }

          <Box
            gridColumn={{ base: '1 / 3', md: 2 }}
            marginTop={{ base: 6, md: 3 }}
          >
            <Flex flexWrap="wrap" gap={ 6 }>
              <Flex width={{ base: '100%', md: 'auto' }} gap={ 2 }>
                <Button size="sm" asChild>
                  <Link href={ external ? url : route({ pathname: '/apps/[id]', query: { id: data.id } }) } external={ external } noIcon>
                    Launch app
                  </Link>
                </Button>

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
                  ml={ 0 }
                  boxSize={ 8 }
                  borderRadius="base"
                />
              </Flex>
            </Flex>
          </Box>
        </Box>

        <DialogBody mb={ 6 }>
          <Text>{ description }</Text>
        </DialogBody>

        <DialogFooter
          display="flex"
          flexDirection={{ base: 'column', md: 'row' }}
          justifyContent={{ base: 'flex-start', md: 'space-between' }}
          alignItems="flex-start"
          gap={ 3 }
        >
          <Flex gap={ 2 } flexWrap="wrap">
            { categories.map((category) => (
              <Badge
                colorPalette="blue"
                key={ category }
              >
                { category }
              </Badge>
            )) }
          </Flex>

          <Flex alignItems="center" flexWrap="wrap" minW={ 0 } gap={ 3 } my="calc(var(--chakra-spacing-1) * 0.5)">
            { site && (
              <Link
                external
                href={ site }
                display="flex"
                alignItems="center"
                textStyle="sm"
              >
                <IconSvg
                  name="link"
                  display="inline"
                  verticalAlign="baseline"
                  boxSize="calc(var(--chakra-spacing-1) * 4.5)"
                  marginRight={ 2 }
                />

                <Text
                  color="inherit"
                  whiteSpace="nowrap"
                  overflow="hidden"
                  textOverflow="ellipsis"
                >
                  { makePrettyLink(site)?.domain }
                </Text>
              </Link>
            ) }

            { socialLinks.map(({ icon, url }) => (
              <Link
                aria-label={ `Link to ${ url }` }
                title={ url }
                key={ url }
                href={ url }
                display="flex"
                alignItems="center"
                justifyContent="center"
                external
                noIcon
                flexShrink={ 0 }
              >
                <IconSvg
                  name={ icon }
                  color="icon.secondary"
                  boxSize={ 5 }
                />
              </Link>
            )) }
          </Flex>
        </DialogFooter>
      </DialogContent>
    </DialogRoot>
  );
};

export default MarketplaceAppModal;
