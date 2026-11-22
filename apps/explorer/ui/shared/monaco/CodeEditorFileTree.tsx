import type { AccordionItemProps } from '@chakra-ui/react';
import { Box, chakra } from '@chakra-ui/react';
import React from 'react';

import type { FileTree } from './types';

import { AccordionItem, AccordionItemContent, AccordionItemTrigger, AccordionRoot } from 'toolkit/chakra/accordion';
import IconSvg from 'ui/shared/IconSvg';

import CodeEditorFileIcon from './CodeEditorFileIcon';
import CodeEditorMainFileIndicator from './CodeEditorMainFileIndicator';
import useThemeColors from './utils/useThemeColors';

interface Props {
  tree: FileTree;
  level?: number;
  isCollapsed?: boolean;
  onItemClick: (event: React.MouseEvent) => void;
  selectedFile: string;
  mainFile?: string;
}

const CodeEditorFileTree = ({ tree, level = 0, onItemClick, isCollapsed, selectedFile, mainFile }: Props) => {
  const [ value, setValue ] = React.useState<Array<string>>(isCollapsed ? [] : tree.map((item) => item.name));

  const handleValueChange = React.useCallback(({ value }: { value: Array<string> }) => {
    setValue(value);
  }, []);

  const itemProps: Partial<AccordionItemProps> = {
    borderWidth: 0,
    cursor: 'pointer',
    textStyle: 'sm',
    _last: {
      borderBottomWidth: 0,
    },
  };
  const themeColors = useThemeColors();

  return (
    <AccordionRoot multiple value={ value } onValueChange={ handleValueChange } noAnimation>
      {
        tree.map((leaf, index) => {
          const leafName = <chakra.span overflow="hidden" textOverflow="ellipsis" whiteSpace="nowrap">{ leaf.name }</chakra.span>;
          const isExpanded = value.includes(leaf.name);

          if ('children' in leaf) {
            return (
              <AccordionItem key={ index } value={ leaf.name } { ...itemProps }>
                <AccordionItemTrigger
                  pr={ 2 }
                  py="0"
                  pl={ `calc(var(--chakra-spacing-2) * ${ level + 1 })` }
                  _hover={{ bgColor: themeColors['custom.list.hoverBackground'] }}
                  textStyle="sm"
                  transitionDuration="0"
                  noIndicator
                >
                  <Box
                    className="codicon codicon-tree-item-expanded"
                    transform="rotate(-90deg)"
                    _groupExpanded={{
                      transform: 'rotate(0deg)',
                    }}
                    boxSize={ 4 }
                    mr={ 0.5 }
                  />
                  <IconSvg
                    name={ isExpanded ? 'monaco/folder-open' : 'monaco/folder' }
                    boxSize={ 4 }
                    mr={ 1 }
                  />
                  { leafName }
                </AccordionItemTrigger>
                <AccordionItemContent p="0">
                  <CodeEditorFileTree
                    tree={ leaf.children }
                    level={ level + 1 }
                    onItemClick={ onItemClick }
                    isCollapsed={ isCollapsed }
                    selectedFile={ selectedFile }
                    mainFile={ mainFile }
                  />
                </AccordionItemContent>
              </AccordionItem>
            );
          }

          return (
            <chakra.button
              type="button"
              key={ index }
              textStyle="sm"
              w="100%"
              minW={ 0 }
              textAlign="left"
              aria-pressed={ selectedFile === leaf.file_path }
              pl={ `calc(var(--chakra-spacing-7) + var(--chakra-spacing-2) * ${ level })` }
              pr={ 2 }
              onClick={ onItemClick }
              data-file-path={ leaf.file_path }
              display="flex"
              position="relative"
              alignItems="center"
              overflow="hidden"
              _hover={{
                bgColor: selectedFile === leaf.file_path ?
                  themeColors['list.inactiveSelectionBackground'] : themeColors['custom.list.hoverBackground'],
              }}
              _focusVisible={{
                outline: 'var(--chakra-border-widths-2) solid',
                outlineColor: 'focusRing',
                outlineOffset: 'calc(-1 * var(--chakra-spacing-0\\.5))',
              }}
              bgColor={ selectedFile === leaf.file_path ? themeColors['list.inactiveSelectionBackground'] : 'none' }
            >
              { mainFile === leaf.file_path && (
                <CodeEditorMainFileIndicator
                  position="absolute"
                  top="50%"
                  transform="translateY(-50%)"
                  left={ `calc(var(--chakra-spacing-3) + var(--chakra-spacing-2) * ${ level })` }
                />
              ) }
              <CodeEditorFileIcon fileName={ leaf.name } mr={ 1 }/>
              { leafName }
            </chakra.button>
          );
        })
      }
    </AccordionRoot>
  );
};

export default React.memo(CodeEditorFileTree);
