import type { HTMLChakraProps } from '@chakra-ui/react';
import { Box, chakra, Flex } from '@chakra-ui/react';
import React from 'react';

import type { File } from './types';

import useDebounce from 'lib/hooks/useDebounce';
import { AccordionRoot } from 'toolkit/chakra/accordion';
import { Input } from 'toolkit/chakra/input';
import { InputGroup } from 'toolkit/chakra/input-group';

import CodeEditorSearchSection from './CodeEditorSearchSection';
import CoderEditorCollapseButton from './CoderEditorCollapseButton';
import type { EditorModels } from './useCodeEditorModels';
import useCodeEditorSearch from './useCodeEditorSearch';
import useThemeColors from './utils/useThemeColors';

interface Props {
  data: Array<File>;
  models: EditorModels;
  onFileSelect: (index: number, lineNumber?: number) => void;
  isInputStuck: boolean;
  isActive: boolean;
  setActionBarRenderer: React.Dispatch<React.SetStateAction<(() => React.JSX.Element) | undefined>>;
  defaultValue: string;
}

const CodeEditorSearch = ({ models, data, onFileSelect, isInputStuck, isActive, setActionBarRenderer, defaultValue }: Props) => {
  const [ searchTerm, changeSearchTerm ] = React.useState('');
  const [ expandedSections, setExpandedSections ] = React.useState<Array<string>>([]);
  const [ isMatchCase, setMatchCase ] = React.useState(false);
  const [ isMatchWholeWord, setMatchWholeWord ] = React.useState(false);
  const [ isMatchRegex, setMatchRegex ] = React.useState(false);

  const themeColors = useThemeColors();

  const debouncedSearchTerm = useDebounce(searchTerm, 300);
  const searchResults = useCodeEditorSearch(models, debouncedSearchTerm, isMatchRegex, isMatchCase, isMatchWholeWord);

  const handleMatchCaseChange = React.useCallback(() => {
    setMatchCase((prev) => !prev);
  }, []);

  const handleMatchWholeWordChange = React.useCallback(() => {
    setMatchWholeWord((prev) => !prev);
  }, []);

  const handleMatchRegexChange = React.useCallback(() => {
    setMatchRegex((prev) => !prev);
  }, []);

  React.useEffect(() => {
    changeSearchTerm(defaultValue);
  }, [ defaultValue ]);

  React.useEffect(() => {
    setExpandedSections(searchResults.map((item) => item.file_path));
  }, [ searchResults ]);

  const handleSearchTermChange = React.useCallback((event: React.ChangeEvent<HTMLInputElement>) => {
    changeSearchTerm(event.target.value);
  }, []);

  const handleResultItemClick = React.useCallback((filePath: string, lineNumber: number) => {
    const fileIndex = data.findIndex((item) => item.file_path === filePath);
    if (fileIndex > -1) {
      onFileSelect(fileIndex, Number(lineNumber));
    }
  }, [ data, onFileSelect ]);

  const handleAccordionStateChange = React.useCallback(({ value }: { value: Array<string> }) => {
    setExpandedSections(value);
  }, []);

  const handleToggleCollapseClick = React.useCallback(() => {
    if (expandedSections.length === 0) {
      setExpandedSections(searchResults.map((item) => item.file_path));
    } else {
      setExpandedSections([]);
    }
  }, [ expandedSections.length, searchResults ]);

  const renderActionBar = React.useCallback(() => {
    return (
      <CoderEditorCollapseButton
        onClick={ handleToggleCollapseClick }
        label={ expandedSections.length === 0 ? 'Expand all' : 'Collapse all' }
        isDisabled={ searchResults.length === 0 }
        isCollapsed={ expandedSections.length === 0 }
      />
    );
  }, [ expandedSections.length, handleToggleCollapseClick, searchResults.length ]);

  React.useEffect(() => {
    isActive && setActionBarRenderer(() => renderActionBar);
  }, [ isActive, renderActionBar, setActionBarRenderer ]);

  const buttonProps: HTMLChakraProps<'button'> = {
    type: 'button',
    boxSize: 5,
    p: 0.25,
    cursor: 'pointer',
    borderRadius: 'control',
    borderWidth: 'var(--chakra-border-widths-1)',
    borderColor: 'transparent',
    color: 'text.primary',
    _focusVisible: { outline: 'var(--chakra-border-widths-2) solid', outlineColor: 'focusRing', outlineOffset: 'var(--chakra-spacing-0\\.5)' },
  };

  const searchResultNum = (() => {
    if (!debouncedSearchTerm) {
      return null;
    }

    const totalResults = searchResults.map(({ matches }) => matches.length).reduce((result, item) => result + item, 0);

    if (!totalResults) {
      return (
        <Box px={ 2 } textStyle="sm" mb={ 2 } role="status">
          No results found. Review your settings for configured exclusions.
        </Box>
      );
    }

    return (
      <Box px={ 2 } textStyle="sm" mb={ 2 } role="status">
        { totalResults } result{ totalResults > 1 ? 's' : '' } in { searchResults.length } file{ searchResults.length > 1 ? 's' : '' }
      </Box>
    );
  })();

  const inputEndElement = (
    <>
      <chakra.button
        { ...buttonProps }
        className="codicon codicon-case-sensitive"
        onClick={ handleMatchCaseChange }
        bgColor={ isMatchCase ? themeColors['custom.inputOption.activeBackground'] : 'transparent' }
        _hover={{ bgColor: isMatchCase ? themeColors['custom.inputOption.activeBackground'] : themeColors['custom.inputOption.hoverBackground'] }}
        title="Match Case"
        aria-label="Match Case"
        aria-pressed={ isMatchCase }
      />
      <chakra.button
        { ...buttonProps }
        className="codicon codicon-whole-word"
        bgColor={ isMatchWholeWord ? themeColors['custom.inputOption.activeBackground'] : 'transparent' }
        onClick={ handleMatchWholeWordChange }
        _hover={{ bgColor: isMatchWholeWord ? themeColors['custom.inputOption.activeBackground'] : themeColors['custom.inputOption.hoverBackground'] }}
        title="Match Whole Word"
        aria-label="Match Whole Word"
        aria-pressed={ isMatchWholeWord }
      />
      <chakra.button
        { ...buttonProps }
        className="codicon codicon-regex"
        bgColor={ isMatchRegex ? themeColors['custom.inputOption.activeBackground'] : 'transparent' }
        onClick={ handleMatchRegexChange }
        _hover={{ bgColor: isMatchRegex ? themeColors['custom.inputOption.activeBackground'] : themeColors['custom.inputOption.hoverBackground'] }}
        title="Use Regular Expression"
        aria-label="Use Regular Expression"
        aria-pressed={ isMatchRegex }
      />
    </>
  );

  return (
    <>
      <Box
        px={ 2 }
        position="sticky"
        top="calc(var(--chakra-spacing-8) + var(--chakra-spacing-3) / 4)" css={{ '@media (max-width: 834px)': { top: 'var(--chakra-sizes-touch-target)' } }}
        left="0"
        zIndex="2"
        bgColor={ themeColors['sideBar.background'] }
        pb={ 2 }
        boxShadow={ isInputStuck ? 'md' : 'none' }
      >
        <InputGroup>
          <Input
            size="xs"
            onChange={ handleSearchTermChange }
            value={ searchTerm }
            placeholder="Search"
            aria-label="Search"
            name="contract-code-search"
            autoComplete="off"
            spellCheck={ false }
            color={ themeColors['input.foreground'] }
            bgColor={ themeColors['input.background'] }
            borderRadius="none"
            height="auto"
            textStyle="sm"
            borderWidth="var(--chakra-border-widths-1)"
            borderColor={ themeColors['input.background'] }
            py={ 0.5 }
            px={ 1 }
            transitionDuration="0"
            _focus={{
              borderColor: themeColors.focusBorder,
            }}
          />

        </InputGroup>
        <Flex gap={ 1 } flexWrap="wrap">{ inputEndElement }</Flex>
      </Box>
      { searchResultNum }
      <AccordionRoot
        key={ debouncedSearchTerm }
        multiple
        value={ expandedSections }
        onValueChange={ handleAccordionStateChange }
        noAnimation
      >
        { searchResults.map((item) => <CodeEditorSearchSection key={ item.file_path } data={ item } onItemClick={ handleResultItemClick }/>) }
      </AccordionRoot>
    </>
  );
};

export default React.memo(CodeEditorSearch);
