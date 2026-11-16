type Props = {
  isExpanded?: boolean;
  isCollapsed?: boolean;
  isActive?: boolean;
};

export default function useNavLinkStyleProps({ isExpanded, isCollapsed, isActive }: Props) {
  return {
    itemProps: {
      variant: 'navigation' as const,
      py: 'calc(var(--chakra-spacing-2) + var(--chakra-spacing-1) / 4)',
      display: 'flex',
      ...(isActive ? { 'data-selected': true } : {}),
      borderRadius: 'base',
      transitionProperty: 'width, padding',
      transitionDuration: 'normal',
      transitionTimingFunction: 'ease',
    },
    textProps: {
      variant: 'inherit',
      fontSize: 'sm',
      lineHeight: 'body',
      opacity: { base: '1', lg: isExpanded ? '1' : '0', xl: isCollapsed ? '0' : '1' },
      transitionProperty: 'opacity',
      transitionDuration: 'normal',
      transitionTimingFunction: 'ease',
    },
  };
}
