import { Icon, chakra } from '@chakra-ui/react';
import React from 'react';

// eslint-disable-next-line no-restricted-imports
import MeritsIconColored from 'icons/merits_colored.svg';

type Props = {
  className?: string;
};

const MeritsIcon = ({ className }: Props) => {
  return (
    <Icon className={ className }>
      <MeritsIconColored/>
    </Icon>
  );
};

export default chakra(MeritsIcon);
