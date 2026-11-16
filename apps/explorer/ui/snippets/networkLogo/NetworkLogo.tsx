import { chakra } from '@chakra-ui/react';
import React from 'react';

import { route } from 'nextjs-routes';

import config from 'configs/app';
import { useColorModeValue } from 'toolkit/chakra/color-mode';
import { Image } from 'toolkit/chakra/image';
import IconSvg from 'ui/shared/IconSvg';

import { INVERT_FILTER } from './consts';

const LogoFallback = () => {
  return (
    <IconSvg
      name="networks/logo-placeholder"
      width="var(--chakra-spacing-30)"
      height="var(--chakra-spacing-6)"
      color="accent"
      aria-label="Network logo placeholder"
    />
  );
};

type Props = {
  className?: string;
};

const NetworkLogo = ({ className }: Props) => {

  const logoSrc = useColorModeValue(config.UI.navigation.logo.default, config.UI.navigation.logo.dark || config.UI.navigation.logo.default);

  return (
    <chakra.a
      className={ className }
      href={ route({ pathname: '/' }) }
      aria-label="Link to main page"
      display="inline-flex"
      alignItems="center"
      minH="touchTarget"
      minW="touchTarget"
      flexShrink={ 0 }
    >
      <Image
        h="var(--chakra-spacing-6)"
        w="var(--chakra-spacing-30)"
        src={ logoSrc }
        alt={ `${ config.chain.name } network logo` }
        fallback={ <LogoFallback/> }
        filter={{ _dark: !config.UI.navigation.logo.dark ? INVERT_FILTER : undefined }}
        objectFit="contain"
        objectPosition="left"
      />
    </chakra.a>
  );
};

export default React.memo(chakra(NetworkLogo));
