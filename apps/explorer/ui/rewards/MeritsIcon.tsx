import { Icon, chakra } from '@chakra-ui/react';
import React from 'react';

// This icon doesn't work properly when it is in the sprite
// Probably because of the gradient
// eslint-disable-next-line no-restricted-imports
import MeritsIconColored from 'icons/merits_colored.svg';

type Props = {
  className?: string;
};

const MeritsIcon = ({ className }: Props) => {
  return (
    <Icon className={ className } filter={ 'drop-shadow(0px 4px 2px var(--chakra-colors-rewards-icon-shadow))' }>
      <MeritsIconColored/>
    </Icon>
  );
};

export default chakra(MeritsIcon);
