import { HStack, chakra, Separator } from '@chakra-ui/react';
import React from 'react';

import type { NavGroupItem, NavItem } from 'types/client/navigation';

import { isInternalItem } from 'lib/hooks/useNavItems';
import { Link } from 'toolkit/chakra/link';
import { Tooltip } from 'toolkit/chakra/tooltip';
import { useDisclosure } from 'toolkit/hooks/useDisclosure';
import IconSvg from 'ui/shared/IconSvg';

import LightningLabel from '../LightningLabel';
import { checkRouteHighlight } from '../utils';
import NavLink from './NavLink';
interface Props {
  item: NavGroupItem;
}

const NavLinkGroup = ({ item }: Props) => {
  const { open, onOpenChange, onToggle } = useDisclosure();

  const subItems = item.subItems.map((subItem) => {
    const items = Array.isArray(subItem) ? subItem : [ subItem ];
    const currentItems = items.filter((navItem) => {
      if (!isInternalItem(navItem)) {
        return true;
      }

      const pathname = navItem.nextRoute.pathname;
      return ![ '/paxeer-x/anchors', '/paxeer-x/receipts' ].some((retiredPath) =>
        pathname === retiredPath || pathname.startsWith(`${ retiredPath }/`),
      );
    });

    return Array.isArray(subItem) ? currentItems : currentItems[0];
  }).filter((subItem): subItem is NavItem | Array<NavItem> =>
    subItem !== undefined && (!Array.isArray(subItem) || subItem.length > 0),
  );

  const isHighlighted = checkRouteHighlight(subItems.flat());
  const hasGroups = subItems.some((subItem) => Array.isArray(subItem));

  const triggerProps = React.useMemo(() => ({ onClick: onToggle }), [ onToggle ]);

  if (subItems.length === 0) {
    return null;
  }

  const content = hasGroups ? (
    <HStack separator={ <Separator/> } alignItems="stretch">
      { subItems.map((subItem, index) => {
        if (!Array.isArray(subItem)) {
          return <NavLink key={ subItem.text } item={ subItem }/>;
        }

        return (
          <chakra.ul key={ index } display="flex" flexDir="column" rowGap={ 1 }>
            { subItem.map((navItem) => <NavLink key={ navItem.text } item={ navItem }/>) }
          </chakra.ul>
        );
      }) }
    </HStack>
  ) : (
    <chakra.ul display="flex" flexDir="column" rowGap={ 1 }>
      { subItems.map((subItem) => {
        if (Array.isArray(subItem)) {
          return null;
        }
        return <NavLink key={ subItem.text } item={ subItem }/>;
      }) }
    </chakra.ul>
  );

  return (
    <Tooltip
      variant="popover"
      content={ content }
      open={ open }
      onOpenChange={ onOpenChange }
      triggerProps={ triggerProps }
      lazyMount={ false }
      unmountOnExit={ false }
      positioning={{
        placement: 'bottom',
        offset: { mainAxis: 8 },
      }}
      interactive
    >
      <Link
        as="li"
        data-label="nav-group"
        listStyleType="none"
        display="flex"
        alignItems="center"
        px={ 2 }
        py={ 1.5 }
        textStyle="sm"
        fontWeight={ 500 }
        variant="navigation"
        // the scan layout marks the current group with the accent color alone, without a filled pill
        _selected={{ bg: 'transparent' }}
        { ...(item.isActive ? { 'data-selected': true } : {}) }
        { ...(open ? { 'data-active': true } : {}) }
        borderRadius="base"
      >
        { item.text }
        { isHighlighted && (
          <LightningLabel
            iconColor={ item.isActive ? 'link.navigation.bg.selected' : 'link.navigation.bg.group' }
            position={{ lg: 'static' }}
            ml={{ lg: '2px' }}
          />
        ) }
        <IconSvg name="arrows/east-mini" boxSize={ 5 } transform={ open ? 'rotate(90deg)' : 'rotate(-90deg)' } ml={ 1 }/>
      </Link>
    </Tooltip>
  );
};

export default React.memo(NavLinkGroup);
