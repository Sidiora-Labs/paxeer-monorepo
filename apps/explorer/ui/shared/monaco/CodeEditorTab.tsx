import { Flex, chakra } from '@chakra-ui/react';
import React from 'react';

import { alt } from 'toolkit/utils/htmlEntities';
import useThemeColors from 'ui/shared/monaco/utils/useThemeColors';

import CodeEditorFileIcon from './CodeEditorFileIcon';
import CodeEditorMainFileIndicator from './CodeEditorMainFileIndicator';
import getFilePathParts from './utils/getFilePathParts';

interface Props {
  isActive?: boolean;
  isMainFile?: boolean;
  path: string;
  onClick: (path: string) => void;
  onClose: (path: string) => void;
  isCloseDisabled: boolean;
  tabsPathChunks: Array<Array<string>>;
}

const CodeEditorTab = ({ isActive, isMainFile, path, onClick, onClose, isCloseDisabled, tabsPathChunks }: Props) => {
  const [ fileName, folderName ] = getFilePathParts(path, tabsPathChunks);
  const themeColors = useThemeColors();

  const handleClick = React.useCallback(() => {
    onClick(path);
  }, [ onClick, path ]);

  const handleClose = React.useCallback((event: React.MouseEvent) => {
    event.stopPropagation();
    !isCloseDisabled && onClose(path);
  }, [ isCloseDisabled, onClose, path ]);

  return (
    <Flex
      pl={ 2.5 }
      pr={ 1 }
      textStyle="sm"
      minW={ 0 }
      maxW="100%"
      flexShrink={ 0 }
      bgColor={ isActive ? themeColors['tab.activeBackground'] : themeColors['tab.inactiveBackground'] }
      borderRightWidth="var(--chakra-border-widths-1)"
      borderRightColor={ themeColors['tab.border'] }
      borderBottomWidth="var(--chakra-border-widths-1)"
      borderBottomColor={ isActive ? 'transparent' : themeColors['tab.border'] }
      color={ isActive ? themeColors['tab.activeForeground'] : themeColors['tab.inactiveForeground'] }
      alignItems="center"
      fontWeight="normal"
      _hover={{
        '& .codicon-close': {
          visibility: 'visible',
        },
      }}
      _focusWithin={{ '& .codicon-close': { visibility: 'visible' } }}
      userSelect="none"
    >
      <chakra.button
        type="button"
        display="flex"
        alignItems="center"
        minW={ 0 }
        flexGrow={ 1 }
        onClick={ handleClick }
        aria-pressed={ Boolean(isActive) }
        _focusVisible={{ outline: 'var(--chakra-border-widths-2) solid', outlineColor: 'focusRing', outlineOffset: 'var(--chakra-spacing-0\\.5)' }}
      >
        <CodeEditorFileIcon mr={ 1 } fileName={ fileName }/>
        <chakra.span overflow="hidden" textOverflow="ellipsis" whiteSpace="nowrap">{ fileName }</chakra.span>
        { folderName && <chakra.span textStyle="xs" color="text.secondary" ml={ 1 }>{ folderName[0] === '.' ? '' : '...' }{ folderName }</chakra.span> }
        { isMainFile && <CodeEditorMainFileIndicator ml={ 2 }/> }
      </chakra.button>
      <chakra.button
        type="button"
        className="codicon codicon-close"
        boxSize={ 5 }
        flexShrink={ 0 }
        ml={ 1 }
        p={ 0.5 }
        title={ `Close ${ isActive ? `(${ alt }W)` : '' }` }
        aria-label="Close"
        onClick={ handleClose }
        borderRadius="sm"
        disabled={ isCloseDisabled }
        _disabled={{ opacity: 'control.disabled', cursor: 'not-allowed' }}
        _focusVisible={{ outline: 'var(--chakra-border-widths-2) solid', outlineColor: 'focusRing', outlineOffset: 'var(--chakra-spacing-0\\.5)' }}
        visibility={{ base: 'visible', lg: isActive ? 'visible' : 'hidden' }}
        color={ themeColors['icon.foreground'] }
        _hover={{ bgColor: isCloseDisabled ? 'transparent' : themeColors['custom.inputOption.hoverBackground'] }}
      />
    </Flex>
  );
};

export default React.memo(CodeEditorTab);
