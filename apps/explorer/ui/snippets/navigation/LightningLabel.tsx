import { useBreakpointValue, chakra } from '@chakra-ui/react';
import React from 'react';

import IconSvg from 'ui/shared/IconSvg';

export const LIGHTNING_LABEL_CLASS_NAME = 'lightning-label';

interface Props {
  className?: string;
  iconColor?: string;
  isCollapsed?: boolean;
}

const LightningLabel = ({ className, iconColor, isCollapsed }: Props) => {
  const isLgScreen = useBreakpointValue({ base: false, lg: true, xl: false });

  const isExpanded = isCollapsed === false;

  const color = React.useMemo(() => {
    if (isCollapsed || (!isExpanded && isLgScreen)) {
      return (iconColor && iconColor !== 'transparent') ? iconColor : 'bg.primary';
    }
    return 'transparent';
  }, [ iconColor, isCollapsed, isExpanded, isLgScreen ]);

  return (
    <IconSvg aria-hidden={ true }
      className={ LIGHTNING_LABEL_CLASS_NAME + (className ? ` ${ className }` : '') }
      name="lightning_navbar"
      boxSize={ 4 }
      ml={{ base: 1, lg: isExpanded ? 1 : 0, xl: isCollapsed ? 0 : 1 }}
      position={{ lg: isExpanded ? 'relative' : 'absolute', xl: isCollapsed ? 'absolute' : 'relative' }}
      top={{ lg: isExpanded ? '0' : 'var(--chakra-spacing-2\\.5)', xl: isCollapsed ? 'var(--chakra-spacing-2\\.5)' : '0' }}
      right={{ lg: isExpanded ? '0' : 'var(--chakra-spacing-3\\.75)', xl: isCollapsed ? 'var(--chakra-spacing-3\\.75)' : '0' }}
      color={ color }
      transitionProperty="color, margin-left"
      _motionSafe={{ transitionDuration: 'normal' }}
      transitionTimingFunction="ease"
    />
  );
};

export default chakra(LightningLabel);
