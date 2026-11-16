import { Box } from '@chakra-ui/react';
import React from 'react';

import type { NavLink } from './types';

import { Link } from 'toolkit/chakra/link';
import IconSvg from 'ui/shared/IconSvg';

const UserProfileContentNavLink = ({ href, icon, text, onClick }: NavLink) => {
  return (
    <Link
      href={ href }
      display="flex"
      alignItems="center"
      columnGap={ 3 }
      py={ 'var(--chakra-spacing-3\\.5)' }
      onClick={ onClick }
      variant="menu"
    >
      <IconSvg aria-hidden={ true } name={ icon } boxSize={ 5 } flexShrink={ 0 }/>
      <Box textStyle="sm" fontWeight="medium">{ text }</Box>
    </Link>
  );
};

export default React.memo(UserProfileContentNavLink);
