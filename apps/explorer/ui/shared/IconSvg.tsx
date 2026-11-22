import type { HTMLChakraProps } from '@chakra-ui/react';
import { chakra } from '@chakra-ui/react';
import { type IconName } from 'public/icons/name';
import React from 'react';

import config from 'configs/app';
import { Skeleton } from 'toolkit/chakra/skeleton';

export const href = config.app.spriteHash ? `/explorer/icons/sprite.${ config.app.spriteHash }.svg` : '/explorer/icons/sprite.svg';

export { IconName };

export interface Props extends HTMLChakraProps<'div'> {
  name: IconName;
  isLoading?: boolean;
}

const IconSvg = React.forwardRef(
  function IconSvg({ name, isLoading = false, ...props }: Props, ref: React.ForwardedRef<HTMLDivElement>) {
    return (
      <Skeleton loading={ isLoading } display="inline-block" flexShrink={ 0 } asChild { ...props } ref={ ref }>
        <chakra.svg w="100%" h="100%" fill="currentColor">
          <use href={ `${ href }#${ name }` } style={{ color: 'inherit', fill: 'currentColor' }}/>
        </chakra.svg>
      </Skeleton>
    );
  },
);

IconSvg.displayName = 'IconSvg';

export default IconSvg;
