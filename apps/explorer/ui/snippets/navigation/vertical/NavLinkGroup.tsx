import { Text, HStack, Box, VStack } from '@chakra-ui/react';
import React from 'react';

import type { NavGroupItem } from 'types/client/navigation';

import { Tooltip } from 'toolkit/chakra/tooltip';
import IconSvg from 'ui/shared/IconSvg';

import LightningLabel from '../LightningLabel';
import NavLinkIcon from '../NavLinkIcon';
import useNavLinkStyleProps from '../useNavLinkStyleProps';
import { checkRouteHighlight } from '../utils';
import NavLink from './NavLink';

type Props = {
  item: NavGroupItem;
  isCollapsed?: boolean;
};

const NavLinkGroup = ({ item, isCollapsed }: Props) => {
  const isExpanded = isCollapsed === false;

  const styleProps = useNavLinkStyleProps({ isCollapsed, isExpanded, isActive: item.isActive });

  const isHighlighted = checkRouteHighlight(item.subItems);

  const content = (
    <Box
      width="calc(var(--chakra-spacing-40) + var(--chakra-spacing-16) + var(--chakra-spacing-1))"
      top={{
        lg: isExpanded ? 'calc(-1 * var(--chakra-spacing-4))' : 0,
        xl: isCollapsed ? 0 : 'calc(-1 * var(--chakra-spacing-4))',
      }}
    >
      <Text
        color="text.secondary"
        fontSize="sm"
        mb={ 1 }
        display={{ lg: isExpanded ? 'none' : 'block', xl: isCollapsed ? 'block' : 'none' }}
      >
        { item.text }
      </Text>
      <VStack gap={ 1 } alignItems="start" as="ul">
        { item.subItems.map((subItem, index) => Array.isArray(subItem) ? (
          <Box
            key={ index }
            w="100%"
            as="ul"
            _notLast={{
              mb: 2,
              pb: 2,
              borderBottomWidth: 'var(--chakra-border-widths-1)',
              borderColor: 'border.divider',
            }}
          >
            { subItem.map(subSubItem => <NavLink key={ subSubItem.text } item={ subSubItem } isCollapsed={ false }/>) }
          </Box>
        ) :
          <NavLink key={ subItem.text } item={ subItem } isCollapsed={ false }/>,
        ) }
      </VStack>
    </Box>
  );

  return (
    <Box as="li" listStyleType="none" w="100%">
      <Tooltip
        content={ content }
        positioning={{ placement: 'right-start', offset: { crossAxis: 0, mainAxis: 8 } }}
        // should not be lazy to help google indexing pages
        lazyMount={ false }
        variant="popover"
        interactive
      >
        <Box
          { ...styleProps.itemProps }
          w={{
            lg: isExpanded ? 'calc(var(--chakra-spacing-40) + var(--chakra-spacing-5))' : 'var(--chakra-spacing-15)',
            xl: isCollapsed ? 'var(--chakra-spacing-15)' : 'calc(var(--chakra-spacing-40) + var(--chakra-spacing-5))',
          }}
          pl={{ lg: isExpanded ? 2 : 'var(--chakra-spacing-3\\.75)', xl: isCollapsed ? 'var(--chakra-spacing-3\\.75)' : 2 }}
          pr={{ lg: isExpanded ? 0 : 'var(--chakra-spacing-3\\.75)', xl: isCollapsed ? 'var(--chakra-spacing-3\\.75)' : 0 }}
          aria-label={ `${ item.text } link group` }
          position="relative"
          color={ item.isActive ? 'link.navigation.fg.selected' : 'link.navigation.fg' }
          bgColor={ item.isActive ? 'link.navigation.bg.selected' : 'link.navigation.bg' }
          _hover={{
            color: 'link.navigation.fg.hover',
          }}
          _open={{
            color: 'link.navigation.fg.hover',
          }}
          cursor="pointer"
        >
          <HStack gap={ 0 } overflow="hidden">
            <NavLinkIcon item={ item }/>
            <Text
              { ...styleProps.textProps }
              ml={ 3 }
              whiteSpace="nowrap"
            >
              { item.text }
            </Text>
            { isHighlighted && (
              <LightningLabel
                iconColor={ item.isActive ? 'link.navigation.bg.selected' : 'link.navigation.bg.group' }
                isCollapsed={ isCollapsed }
              />
            ) }
            <IconSvg aria-hidden={ true }
              name="arrows/east-mini"
              position="absolute"
              right="calc(var(--chakra-spacing-2) - var(--chakra-spacing-1) / 4)"
              transform="rotate(180deg)"
              boxSize={ 6 }
              opacity={{ lg: isExpanded ? '1' : '0', xl: isCollapsed ? '0' : '1' }}
              transitionProperty="opacity"
              _motionSafe={{ transitionDuration: 'normal' }}
              transitionTimingFunction="ease"
            />
          </HStack>
        </Box>
      </Tooltip>
    </Box>
  );
};

export default NavLinkGroup;
