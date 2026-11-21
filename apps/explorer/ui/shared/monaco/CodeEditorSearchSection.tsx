import { Box } from '@chakra-ui/react';
import React from 'react';

import type { SearchResult } from './types';

import { AccordionItem, AccordionItemContent, AccordionItemTrigger } from 'toolkit/chakra/accordion';

import CodeEditorFileIcon from './CodeEditorFileIcon';
import CodeEditorSearchResultItem from './CodeEditorSearchResultItem';
import getFileName from './utils/getFileName';
import useThemeColors from './utils/useThemeColors';

interface Props {
  data: SearchResult;
  onItemClick: (filePath: string, lineNumber: number) => void;
}

const CodeEditorSearchSection = ({ data, onItemClick }: Props) => {
  const fileName = getFileName(data.file_path);

  const handleFileLineClick = React.useCallback((event: React.MouseEvent) => {
    const lineNumber = Number((event.currentTarget as HTMLDivElement).getAttribute('data-line-number'));
    if (!Object.is(lineNumber, NaN)) {
      onItemClick(data.file_path, Number(lineNumber));
    }
  }, [ data.file_path, onItemClick ]);

  const themeColors = useThemeColors();

  return (
    <AccordionItem value={ data.file_path } borderWidth={ 0 } _last={{ borderBottomWidth: 0 }}>
      <AccordionItemTrigger
        py={ 0 }
        px={ 2 }
        _hover={{ bgColor: themeColors['custom.list.hoverBackground'] }}
        textStyle="sm"
        transitionDuration="0"
        alignItems="center"
        noIndicator
      >
        <Box
          className="codicon codicon-tree-item-expanded"
          transform="rotate(-90deg)"
          _groupExpanded={{
            transform: 'rotate(0deg)',
          }}
          // transform={ isExpanded ? 'rotate(0deg)' : 'rotate(-90deg)' }
          boxSize={ 5 }
          py={ 0.5 }
          flexShrink={ 0 }
        />
        <CodeEditorFileIcon mr={ 1 } fileName={ fileName }/>
        <Box
          mr={ 2 }
          whiteSpace="nowrap"
          overflow="hidden"
          textOverflow="ellipsis"
          textAlign="left"
        >
          { fileName }
        </Box>
        <Box
          className="monaco-count-badge"
          ml="auto"
          bgColor={ themeColors['badge.background'] }
          flexShrink={ 0 }
        >
          { data.matches.length }
        </Box>
      </AccordionItemTrigger>
      <AccordionItemContent p={ 0 }>
        { data.matches.map((match) => (
          <CodeEditorSearchResultItem
            key={ data.file_path + '_' + match.startLineNumber + '_' + match.startColumn }
            filePath={ data.file_path }
            onClick={ handleFileLineClick }
            { ...match }
          />
        ),
        ) }
      </AccordionItemContent>
    </AccordionItem>
  );
};

export default React.memo(CodeEditorSearchSection);
