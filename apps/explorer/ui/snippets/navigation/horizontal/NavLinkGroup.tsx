import { chakra } from '@chakra-ui/react';
import React from 'react';

import type { NavGroupItem } from 'types/client/navigation';

import { Button } from 'toolkit/chakra/button';
import { useDisclosure } from 'toolkit/hooks/useDisclosure';
import IconSvg from 'ui/shared/IconSvg';
import NativePopover from 'ui/snippets/overlays/NativePopover';

import LightningLabel from '../LightningLabel';
import { checkRouteHighlight } from '../utils';
import NavLink from './NavLink';

interface Props {
  item: NavGroupItem;
}

const NavLinkGroup = ({ item }: Props) => {
  const { open, onOpenChange, onClose, onToggle } = useDisclosure();
  const triggerRef = React.useRef<HTMLButtonElement>(null);
  const panelId = React.useId();
  const anchorName = `--navigation-${ panelId.replace(/[^\w-]/g, '') }`;
  const isHighlighted = checkRouteHighlight(item.subItems);

  return (
    <chakra.li listStyleType="none">

      <Button
        ref={ triggerRef }
        onClick={ onToggle }
        aria-controls={ panelId }
        aria-haspopup="dialog"
        css={{ anchorName }}
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
        <IconSvg
          aria-hidden={ true }
          name="arrows/east-mini"
          boxSize={ 4 }
          transform={ open ? 'rotate(90deg)' : 'rotate(-90deg)' }
        />
      </Button>
      <NativePopover
        id={ panelId }
        open={ open }
        onOpenChange={ onOpenChange }
        anchorRef={ triggerRef }
        anchorName={ anchorName }
        aria-label={ `${ item.text } navigation` }
        bg="raised"
        borderWidth="var(--chakra-border-widths-1)"
        borderColor="hairline"
        borderRadius="card"
        boxShadow="none"
        w="calc(var(--chakra-spacing-100) + var(--chakra-spacing-40))"
        maxW="calc(100dvw - var(--chakra-spacing-8))"
      >
        <chakra.div p={ 2 } containerType="inline-size" containerName="navigation-panel">
          <chakra.ul
            display="grid"
            gridTemplateColumns="repeat(auto-fit, minmax(min(100%, var(--chakra-spacing-40)), 1fr))"
            gap={ 1 }
          >
            { item.subItems.flat().map((link) => <NavLink key={ link.text } item={ link } onClick={ onClose }/>) }
          </chakra.ul>
        </chakra.div>
      </NativePopover>
    </chakra.li>
  );
};

export default React.memo(NavLinkGroup);
