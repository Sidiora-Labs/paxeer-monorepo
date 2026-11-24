import { chakra } from '@chakra-ui/react';
import React from 'react';

import type { NavGroupItem } from 'types/client/navigation';

import NavLink from '../horizontal/NavLink';

type Props = {
  item: NavGroupItem;
  groupName: string;
  onNavLinkClick?: () => void;
};

const NavLinkGroup = ({ item, groupName, onNavLinkClick }: Props) => {
  const handleSummaryClick = React.useCallback((event: React.MouseEvent<HTMLElement>) => {
    const current = event.currentTarget.parentElement;
    if ('name' in HTMLDetailsElement.prototype || !(current instanceof HTMLDetailsElement) || current.open) return;
    current.closest('ul')?.querySelectorAll<HTMLDetailsElement>('details[open]').forEach(details => {
      if (details !== current && details.getAttribute('name') === groupName) details.open = false;
    });
  }, [ groupName ]);

  return (
    <chakra.li listStyleType="none" w="100%" borderBottomWidth="var(--chakra-border-widths-1)" borderColor="hairline">
      <chakra.details name={ groupName }>
        <chakra.summary
          onClick={ handleSummaryClick }
          cursor="pointer"
          py={ 3 }
          px={ 2 }
          borderRadius="control"
          color={ item.isActive ? 'accent' : 'text.primary' }
          _hover={{ bg: 'accent.soft' }}
          fontWeight="medium"
        >
          { item.text }
        </chakra.summary>
        <chakra.ul py={ 2 }>
          { item.subItems.flat().map((link) => <NavLink key={ link.text } item={ link } onClick={ onNavLinkClick }/>) }
        </chakra.ul>
      </chakra.details>
    </chakra.li>
  );
};

export default React.memo(NavLinkGroup);
