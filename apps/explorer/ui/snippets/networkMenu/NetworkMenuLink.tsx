import { Box, Text, chakra } from '@chakra-ui/react';
import React from 'react';

import type { FeaturedNetwork } from 'types/networks';

import { useColorModeValue } from 'toolkit/chakra/color-mode';
import { Image } from 'toolkit/chakra/image';
import IconSvg from 'ui/shared/IconSvg';

interface Props extends FeaturedNetwork {
  isActive?: boolean;
  isMobile?: boolean;
}

const NetworkMenuLink = ({ title, icon, isActive: isActiveProp, url, invertIconInDarkMode }: Props) => {
  const darkModeFilter = { filter: 'brightness(0) invert(1)' };
  const style = useColorModeValue({}, invertIconInDarkMode ? darkModeFilter : {});

  const iconEl = icon ? (
    <Image
      w="var(--chakra-spacing-5)"
      h="var(--chakra-spacing-5)"
      src={ icon }
      alt={ `${ title } network icon` }
      style={ style }
    />
  ) : (
    <IconSvg aria-hidden={ true }
      name="networks/icon-placeholder"
      boxSize="var(--chakra-spacing-5)"
      color="icon.secondary"
    />
  );

  const isActive = (() => {
    if (isActiveProp !== undefined) {
      return isActiveProp;
    }

    try {
      const itemOrigin = new URL(url).origin;
      const currentOrigin = window.location.origin;

      return itemOrigin === currentOrigin;
    } catch (error) {
      return false;
    }
  })();

  return (
    <Box as="li" listStyleType="none">
      <chakra.a
        display="flex"
        href={ url }
        target="_blank"
        rel="noopener noreferrer"
        px={ 2 }
        py={ 2 }
        opacity={ isActive ? 0.6 : 1 }
        alignItems="center"
        cursor="pointer"
        pointerEvents={ isActive ? 'none' : 'initial' }
        borderRadius="base"
        _hover={{ color: isActive ? 'text.primary' : 'hover' }}
      >
        { iconEl }
        <Text
          marginLeft={ 2 }
          color="inherit"
          fontSize="sm"
          lineHeight="body"
        >
          { title }
        </Text>
        { isActive && (
          <IconSvg aria-hidden={ true }
            name="check"
            boxSize="var(--chakra-spacing-5)"
            marginLeft="auto"
          />
        ) }
      </chakra.a>
    </Box>
  );
};

export default React.memo(NetworkMenuLink);
