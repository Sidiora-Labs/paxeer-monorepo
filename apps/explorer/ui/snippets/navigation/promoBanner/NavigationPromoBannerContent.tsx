import { HStack, Text, Box } from '@chakra-ui/react';

import config from 'configs/app';
import { Image } from 'toolkit/chakra/image';

import useNavLinkStyleProps from '../useNavLinkStyleProps';

const promoBanner = config.UI.navigation.promoBanner;

type Props = {
  isCollapsed?: boolean;
  isHorizontalNavigation?: boolean;
};

const NavigationPromoBannerContent = ({ isCollapsed, isHorizontalNavigation }: Props) => {
  const isExpanded = isCollapsed === false;
  const navLinkStyleProps = useNavLinkStyleProps({ isCollapsed, isExpanded });

  if (!promoBanner) {
    return null;
  }

  return 'text' in promoBanner ? (
    <HStack
      { ...navLinkStyleProps.itemProps }
      minW={ isHorizontalNavigation ? 'auto' : 'full' }
      maxW={ isHorizontalNavigation ? 'auto' : 'full' }
      w={ isHorizontalNavigation ? 'auto' : 'calc(var(--chakra-spacing-40) + var(--chakra-spacing-5))' }
      gap={ 2 }
      overflow="hidden"
      whiteSpace="nowrap"
      py={ isHorizontalNavigation ? 1.5 : 2 }
      px={ isHorizontalNavigation ? 1.5 : {
        base: 3,
        lg: isExpanded ? 3 : 'var(--chakra-spacing-3\\.75)',
        xl: isCollapsed ? 'var(--chakra-spacing-3\\.75)' : 3,
      } }
      bgColor={{ _light: promoBanner.bg_color.light, _dark: promoBanner.bg_color.dark }}
    >
      <Image
        src={ promoBanner.img_url }
        alt="Promo banner icon"
        boxSize={ isHorizontalNavigation ? 'var(--chakra-spacing-5)' : 'var(--chakra-spacing-7\\.5)' }
      />
      { !isHorizontalNavigation && (
        <Text
          { ...navLinkStyleProps.textProps }
          fontWeight="medium"
          color={{ _light: promoBanner.text_color.light, _dark: promoBanner.text_color.dark }}
          overflow="hidden"
        >
          { promoBanner.text }
        </Text>
      ) }
    </HStack>
  ) : (
    <Box
      position="relative"
      minH={ isHorizontalNavigation ? 'auto' : 'var(--chakra-spacing-15)' }
    >
      <Image
        src={ promoBanner.img_url.small }
        alt="Promo banner small"
        boxSize={ isHorizontalNavigation ? 'var(--chakra-spacing-8)' : 'var(--chakra-spacing-15)' }
        borderRadius={ isHorizontalNavigation ? 'sm' : 'base' }
        position={ isHorizontalNavigation ? undefined : 'absolute' }
        top={ isHorizontalNavigation ? undefined : 'calc(50% - var(--chakra-spacing-7) - var(--chakra-spacing-1) / 2)' }
        left={ isHorizontalNavigation ? undefined : 'calc(50% - var(--chakra-spacing-7) - var(--chakra-spacing-1) / 2)' }
        opacity={ isHorizontalNavigation ? 1 : { base: 0, lg: isExpanded ? 0 : 1, xl: isCollapsed ? 1 : 0 } }
        transitionProperty="opacity"
        _motionSafe={{ transitionDuration: 'normal' }}
        transitionTimingFunction="ease"
      />
      <Image
        display={ isHorizontalNavigation ? 'none' : 'block' }
        src={ promoBanner.img_url.large }
        alt="Promo banner large"
        w="full"
        maxW={{ base: 'full', lg: 'calc(var(--chakra-spacing-40) + var(--chakra-spacing-5))' }}
        borderRadius="base"
        aspectRatio={ 2 / 1 }
        opacity={{ base: 1, lg: isExpanded ? 1 : 0, xl: isCollapsed ? 0 : 1 }}
        transitionProperty="opacity"
        _motionSafe={{ transitionDuration: 'normal' }}
        transitionTimingFunction="ease"
      />
    </Box>
  );
};

export default NavigationPromoBannerContent;
