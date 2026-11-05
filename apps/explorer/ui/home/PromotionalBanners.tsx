import { Box, SimpleGrid, Text } from '@chakra-ui/react';
import { useQuery } from '@tanstack/react-query';
import React from 'react';

import type { HighlightsBannerConfig } from 'types/homepage';

import config from 'configs/app';
import useFetch from 'lib/hooks/useFetch';
import { useColorModeValue } from 'toolkit/chakra/color-mode';
import { Image } from 'toolkit/chakra/image';
import { Link } from 'toolkit/chakra/link';

const PromotionalBanner = ({ banner }: { banner: HighlightsBannerConfig }) => {
  const adaptive = (values: Array<string | undefined> | undefined): [string | undefined, string | undefined] => [ values?.[0], values?.[1] ?? values?.[0] ];
  const background = useColorModeValue(...adaptive(banner.background));
  const titleColor = useColorModeValue(...adaptive(banner.title_color));
  const descriptionColor = useColorModeValue(...adaptive(banner.description_color));
  const image = useColorModeValue(...adaptive(banner.side_img_url));

  const content = (
    <Box data-promotional-banner p={ 4 } borderRadius="md" h="100%" style={{ background }} display="flex" alignItems="center" gap={ 3 }>
      <Box flex="1">
        <Text data-banner-title fontWeight="700" style={{ color: titleColor }}>{ banner.title }</Text>
        <Text data-banner-description textStyle="sm" style={{ color: descriptionColor }}>{ banner.description }</Text>
      </Box>
      { image && <Image src={ image } alt={ banner.title } maxH="96px" maxW="30%" objectFit="contain"/> }
    </Box>
  );

  const href = banner.redirect_url || banner.page_path;
  return href ? <Link href={ href } external={ Boolean(banner.redirect_url) } noIcon display="block" variant="plain">{ content }</Link> : content;
};

const PromotionalBanners = () => {
  const fetch = useFetch();
  const path = config.UI.homepage.highlights;
  const { data } = useQuery({
    queryKey: [ 'homepage-promotional-banners', path ],
    queryFn: ({ signal }) => fetch<Array<HighlightsBannerConfig>, unknown>(new URL(path!, config.app.origin).href, { signal }),
    enabled: Boolean(path),
    staleTime: Infinity,
  });

  if (!Array.isArray(data) || data.length === 0) {
    return null;
  }

  return (
    <SimpleGrid data-label="home-promotional-banners" columns={{ base: 1, lg: Math.min(data.length, 3) }} gap={ 3 } mb={ 4 }>
      { data.map((banner, index) => <PromotionalBanner key={ `${ banner.title }-${ index }` } banner={ banner }/>) }
    </SimpleGrid>
  );
};

export default React.memo(PromotionalBanners);
