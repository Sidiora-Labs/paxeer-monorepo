import { chakra } from '@chakra-ui/react';
import React from 'react';

import { route } from 'nextjs-routes';

import { useRewardsContext } from 'lib/contexts/rewards';
import useIsMobile from 'lib/hooks/useIsMobile';
import type { ButtonProps } from 'toolkit/chakra/button';
import { Button } from 'toolkit/chakra/button';
import durations from 'toolkit/theme/foundations/durations';
import { Tooltip } from 'ui/marketplace/NativeTooltip';
import IconSvg from 'ui/shared/IconSvg';

type Props = {
  size?: ButtonProps['size'];
  variant?: ButtonProps['variant'];
};

const RewardsButton = ({ variant = 'header', size }: Props) => {
  const { isInitialized, isAuth, openLoginModal, dailyRewardQuery, balancesQuery } = useRewardsContext();
  const isMobile = useIsMobile();
  const isLoading = !isInitialized || dailyRewardQuery.isLoading || balancesQuery.isLoading;

  return (
    <Tooltip
      content="Earn Merits for using Blockscout"
      openDelay={ Number.parseInt(durations['ultra-slow'].value, 10) }
      disabled={ isMobile || isLoading || isAuth }
    >
      <Button
        variant={ variant }
        selected={ !isLoading && isAuth }
        flexShrink={ 0 }
        as={ isAuth ? 'a' : 'button' }
        { ...(isAuth ? { href: route({ pathname: '/account/merits' }) } : {}) }
        onClick={ isAuth ? undefined : openLoginModal }
        aria-label={ isAuth ? 'Merits dashboard' : 'Earn Merits' }
        size={ size }
        px={{ base: 'calc(var(--chakra-spacing-1) * 2.5)', lg: 3 }}
        loading={ isLoading }
        _hover={{
          textDecoration: 'none',
        }}
      >
        <IconSvg
          name={ dailyRewardQuery.data?.available ? 'merits_with_dot' : 'merits' }
          boxSize={ variant === 'hero' ? 6 : 5 }
          flexShrink={ 0 }
        />
        <chakra.span
          display={{ base: 'none', md: 'inline' }}
          fontWeight={ isAuth ? 'bold' : 'semibold' }
        >
          { isAuth ? (balancesQuery.data?.total || 'N/A') : 'Merits' }
        </chakra.span>
      </Button>
    </Tooltip>
  );
};

export default RewardsButton;
