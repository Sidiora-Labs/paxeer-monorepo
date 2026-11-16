import { chakra } from '@chakra-ui/react';
import React from 'react';

import useThemeColors from './utils/useThemeColors';

interface Props {
  onClick: () => void;
  label: string;
  isDisabled?: boolean;
  isCollapsed?: boolean;
}

const CoderEditorCollapseButton = ({ onClick, label, isDisabled, isCollapsed }: Props) => {
  const themeColors = useThemeColors();

  return (
    <chakra.button
      type="button"
      ml="auto"
      alignSelf="center"
      className={ isCollapsed ? 'codicon codicon-search-expand-results' : 'codicon codicon-collapse-all' }
      disabled={ isDisabled }
      aria-expanded={ isCollapsed === undefined ? undefined : !isCollapsed }
      _disabled={{ opacity: 'control.disabled', cursor: 'not-allowed' }}
      boxSize="touchTarget"
      p={ 0.5 }
      borderRadius="sm"
      _before={{
        content: isCollapsed ? '"\\eb95"' : '"\\eac5"',
      }}
      _hover={{
        bgColor: themeColors['custom.inputOption.hoverBackground'],
      }}
      _focusVisible={{
        outline: 'var(--chakra-border-widths-2) solid',
        outlineColor: 'focusRing',
        outlineOffset: 'var(--chakra-spacing-0\\.5)',
      }}
      onClick={ onClick }
      cursor="pointer"
      title={ label }
      aria-label={ label }
    />
  );
};

export default React.memo(CoderEditorCollapseButton);
