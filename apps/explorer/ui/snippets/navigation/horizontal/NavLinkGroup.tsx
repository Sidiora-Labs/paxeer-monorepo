import { chakra } from '@chakra-ui/react';
import React from 'react';

import type { NavGroupItem } from 'types/client/navigation';

import { Button } from 'toolkit/chakra/button';
import { PopoverBody, PopoverContent, PopoverRoot, PopoverTrigger } from 'toolkit/chakra/popover';
import { useDisclosure } from 'toolkit/hooks/useDisclosure';
import IconSvg from 'ui/shared/IconSvg';

import LightningLabel from '../LightningLabel';
import { checkRouteHighlight } from '../utils';
import NavLink from './NavLink';

interface Props {
  item: NavGroupItem;
}

const NavLinkGroup = ({ item }: Props) => {
  const { open, onOpenChange, onClose } = useDisclosure();
  const isHighlighted = checkRouteHighlight(item.subItems);

  return (
    <chakra.li listStyleType="none">
      <PopoverRoot open={ open } onOpenChange={ onOpenChange } positioning={{ placement: 'bottom', offset: { mainAxis: 12 } }}>
        <PopoverTrigger>
          <Button
            data-label="nav-group"
            variant="plain"
            size="sm"
            px={ 2 }
            color={ item.isActive ? 'accent' : 'text.primary' }
            bg="transparent"
            _hover={{ bg: 'accent.soft' }}
            aria-label={ item.text }
            aria-expanded={ open }
            { ...(item.isActive ? { 'data-selected': true } : {}) }
            { ...(open ? { 'data-active': true } : {}) }
          >
            { item.text }
            { isHighlighted && <LightningLabel iconColor="accent"/> }
            <IconSvg name="arrows/east-mini" boxSize={ 4 } transform={ open ? 'rotate(90deg)' : 'rotate(-90deg)' }/>
          </Button>
        </PopoverTrigger>
        <PopoverContent
          aria-label={ `${ item.text } navigation` }
          bg="raised"
          borderWidth="1px"
          borderColor="hairline"
          borderRadius="8px"
          boxShadow="none"
          w="560px"
          maxW="calc(100vw - 32px)"
        >
          <PopoverBody p={ 2 }>
            <chakra.ul display="grid" gridTemplateColumns="repeat(2, minmax(0, 1fr))" gap={ 1 }>
              { item.subItems.flat().map((link) => <NavLink key={ link.text } item={ link } onClick={ onClose }/>) }
            </chakra.ul>
          </PopoverBody>
        </PopoverContent>
      </PopoverRoot>
    </chakra.li>
  );
};

export default React.memo(NavLinkGroup);
