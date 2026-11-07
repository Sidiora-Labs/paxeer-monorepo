import { Box, chakra } from '@chakra-ui/react';
import React from 'react';

import type { NavGroupItem } from 'types/client/navigation';

import { Button } from 'toolkit/chakra/button';
import IconSvg from 'ui/shared/IconSvg';

import NavLink from '../horizontal/NavLink';

type Props = {
  item: NavGroupItem;
  onClick: (text: string) => void;
  isExpanded?: boolean;
  onNavLinkClick?: () => void;
};

const NavLinkGroup = ({ item, onClick, isExpanded, onNavLinkClick }: Props) => {
  const panelId = React.useId();
  const handleClick = React.useCallback(() => onClick(item.text), [ onClick, item.text ]);

  return (
    <chakra.li listStyleType="none" w="100%" borderBottomWidth="1px" borderColor="hairline">
      <Button
        variant="plain"
        _hover={{ bg: 'accent.soft' }}
        w="100%"
        justifyContent="space-between"
        color={ item.isActive ? 'accent' : 'text.primary' }
        onClick={ handleClick }
        aria-expanded={ Boolean(isExpanded) }
        aria-controls={ panelId }
      >
        { item.text }
        <IconSvg name="arrows/east-mini" boxSize={ 5 } transform={ isExpanded ? 'rotate(90deg)' : 'rotate(-90deg)' }/>
      </Button>
      <Box id={ panelId } hidden={ !isExpanded } py={ 2 }>
        <chakra.ul>
          { item.subItems.flat().map((link) => <NavLink key={ link.text } item={ link } onClick={ onNavLinkClick }/>) }
        </chakra.ul>
      </Box>
    </chakra.li>
  );
};

export default React.memo(NavLinkGroup);
