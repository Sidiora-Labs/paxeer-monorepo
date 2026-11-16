import { Box } from '@chakra-ui/react';
import React from 'react';

import { IconButton } from 'toolkit/chakra/icon-button';
import { Tooltip } from 'toolkit/chakra/tooltip';

interface Props {
  onClick?: (event: React.MouseEvent<HTMLButtonElement>) => void;
  isActive: boolean;
  bg: string;
  value: string;
  label: string;
}

const SettingsSample = ({ label, value, bg, onClick, isActive }: Props) => (
  <Tooltip content={ label }>
    <IconButton minH="touchTarget" minW="touchTarget"
      type="button"
      variant="plain"
      aria-label={ label }
      aria-pressed={ isActive }
      aria-current={ isActive ? 'true' : undefined }
      data-value={ value }
      onClick={ onClick }
      borderRadius="control"
      bg={ isActive ? 'accent.soft' : 'transparent' }
      _hover={{ bg: 'accent.soft' }}
    >
      <Box
        aria-hidden="true"
        bg={ bg }
        boxSize={ 6 }
        borderRadius="full"
        borderWidth="var(--chakra-border-widths-2)"
        borderColor={ isActive ? 'accent' : 'border.strong' }
      />
    </IconButton>
  </Tooltip>
);

export default React.memo(SettingsSample);
