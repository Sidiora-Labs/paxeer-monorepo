import { Icon } from '@chakra-ui/react';
import React from 'react';

import InfoIcon from 'icons/info.svg';

import type { IconButtonProps } from '../../chakra/icon-button';
import { IconButton } from '../../chakra/icon-button';
import type { TooltipProps } from '../../chakra/tooltip';
import { Tooltip } from '../../chakra/tooltip';

export interface HintProps extends IconButtonProps {
  label: string | React.ReactNode;
  tooltipProps?: Partial<TooltipProps>;
  isLoading?: boolean;
  as?: React.ElementType;
  inline?: boolean;
}

export const Hint = React.memo(({ label, tooltipProps, isLoading, boxSize = 5, inline, ...rest }: HintProps) => {
  return (
    <Tooltip
      content={ label }
      positioning={{ placement: 'top' }}
      { ...tooltipProps }
    >
      <IconButton
        aria-label="hint"
        boxSize={ boxSize }
        data-touch-inline={ inline || undefined }
        css={ inline ? { '--touch-line-size': typeof boxSize === 'number' ? `var(--chakra-sizes-${ boxSize })` : boxSize } : undefined }
        loadingSkeleton={ isLoading }
        borderRadius="sm"
        variant="icon_secondary"
        { ...rest }
      >
        <Icon boxSize={ boxSize }>
          <InfoIcon/>
        </Icon>
      </IconButton>
    </Tooltip>
  );
});
