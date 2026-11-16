import { Box, Flex } from '@chakra-ui/react';
import React from 'react';

import type { Route } from 'nextjs-routes';
import { route } from 'nextjs-routes';

import { Link } from 'toolkit/chakra/link';

interface Props {
  title: string;
  href: Route;
  actions?: React.ReactNode;
  note?: React.ReactNode;
  children: React.ReactNode;
}

const HomeTableSection = ({ title, href, actions, note, children }: Props) => {
  const id = React.useId();
  return (
    <Box as="section" aria-labelledby={ id } data-scan-table-card data-home-table minW={ 0 }
      css={{ '& a': { display: 'inline-flex', alignItems: 'center' },
        '& a, & button': { minWidth: 'touchTarget', minHeight: 'touchTarget', _motionReduce: { transitionProperty: 'none' } } }}>
      <Flex data-header alignItems="center" justifyContent="space-between" flexWrap="wrap" gap={ 2 } mb={ 2 }>
        <Box as="h2" id={ id } data-title textStyle="label">{ title }</Box>
        <Flex alignItems="center" gap={ 2 }>
          { actions }
          <Link href={ route(href) } textStyle="small" minH="touchTarget" display="inline-flex" alignItems="center"
            aria-label={ `View all ${ title.replace('Latest ', '').toLowerCase() }` }>View all</Link>
        </Flex>
      </Flex>
      { note && <Box data-note textStyle="small" mb={ 2 } color="text.muted">{ note }</Box> }
      <Box data-body overflowX="auto" maxW="100%" borderTopWidth="1" borderColor="border.divider"
        role="region" aria-labelledby={ id } tabIndex={ 0 }>{ children }</Box>
    </Box>
  );
};

export default HomeTableSection;
