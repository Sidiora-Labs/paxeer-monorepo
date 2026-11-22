import { Box, Flex, chakra } from '@chakra-ui/react';
import { throttle } from 'es-toolkit';
import type * as monaco from 'monaco-editor/esm/vs/editor/editor.api';
import React from 'react';

import type { File, Monaco } from './types';

import type { TabsTriggerProps } from 'toolkit/chakra/tabs';
import { TabsContent, TabsList, TabsRoot, TabsTrigger } from 'toolkit/chakra/tabs';
import { shift, cmd } from 'toolkit/utils/htmlEntities';

import CodeEditorFileExplorer from './CodeEditorFileExplorer';
import CodeEditorSearch from './CodeEditorSearch';
import type { EditorModels } from './useCodeEditorModels';
import useThemeColors from './utils/useThemeColors';

interface Props {
  monaco: Monaco | undefined;
  editor: monaco.editor.IStandaloneCodeEditor | undefined;
  data: Array<File>;
  models: EditorModels;
  onFileSelect: (index: number, lineNumber?: number) => void;
  selectedFile: string;
  mainFile?: string;
}

const CodeEditorSideBar = ({ onFileSelect, data, monaco, editor, selectedFile, mainFile, models }: Props) => {

  const [ isStuck, setIsStuck ] = React.useState(false);
  const [ isDrawerOpen, setIsDrawerOpen ] = React.useState(false);
  const [ activeTab, setActiveTab ] = React.useState('explorer');
  const [ searchValue, setSearchValue ] = React.useState('');
  const [ actionBarRenderer, setActionBarRenderer ] = React.useState<() => React.JSX.Element>();
  const sidebarId = React.useId();
  const toggleRef = React.useRef<HTMLButtonElement>(null);

  const themeColors = useThemeColors();

  const tabProps: Partial<TabsTriggerProps> = {
    fontFamily: 'heading',
    textTransform: 'uppercase',
    textStyle: 'label',
    color: themeColors['tab.inactiveForeground'],
    _selected: {
      color: themeColors['tab.activeForeground'],
    },
    px: 0,
  };

  const handleScrollThrottled = React.useRef(throttle((event: React.SyntheticEvent) => {
    setIsStuck((event.target as HTMLDivElement).scrollTop > 0);
  }, 100));

  const handleFileSelect = React.useCallback((index: number, lineNumber?: number) => {
    isDrawerOpen && setIsDrawerOpen(false);
    onFileSelect(index, lineNumber);
  }, [ isDrawerOpen, onFileSelect, setIsDrawerOpen ]);

  const handleSideBarButtonClick = React.useCallback(() => {
    setIsDrawerOpen((prev) => !prev);
  }, []);

  const handleKeyDown = React.useCallback((event: React.KeyboardEvent) => {
    if (event.key === 'Escape' && isDrawerOpen) {
      event.stopPropagation();
      setIsDrawerOpen(false);
      toggleRef.current?.focus();
    }
  }, [ isDrawerOpen ]);

  const handleTabChange = React.useCallback(({ value }: { value: string }) => {
    setActiveTab(value);
  }, []);

  React.useEffect(() => {
    if (editor && monaco) {
      editor.addAction({
        id: 'file-explorer',
        label: 'Show File Explorer',
        keybindings: [
          monaco.KeyMod.CtrlCmd | monaco.KeyMod.Shift | monaco.KeyCode.KeyE,
        ],
        contextMenuGroupId: 'navigation',
        contextMenuOrder: 1.5,
        run: function() {
          setActiveTab('explorer');
        },
      });

      editor.addAction({
        id: 'search-in-files',
        label: 'Show Search in Files',
        keybindings: [
          monaco.KeyMod.CtrlCmd | monaco.KeyMod.Shift | monaco.KeyCode.KeyF,
        ],
        contextMenuGroupId: 'navigation',
        contextMenuOrder: 1.6,
        run: function(editor) {
          setActiveTab('search');
          const selection = editor.getSelection();
          const selectedValue = selection ? editor.getModel()?.getValueInRange(selection) : '';
          setSearchValue(selectedValue || '');
        },
      });
    }
  }, [ editor, monaco ]);

  return (
    <>
      <chakra.button
        type="button"
        ref={ toggleRef }
        boxSize={ 6 }
        p={ 1 }
        position="absolute"
        display={{ base: 'block', lg: 'none' }}
        right={ isDrawerOpen ? 'min(calc(100% - var(--chakra-spacing-6)), var(--chakra-spacing-62\\.5))' : 0 }
        css={{ '@media (max-width: 834px)': { right: isDrawerOpen ? 'min(calc(100% - var(--chakra-sizes-touch-target)), var(--chakra-spacing-62\\.5))' : 0 } }}
        top="50%"
        transform="translateY(-50%)"
        backgroundColor={ themeColors['sideBar.background'] }
        borderTopLeftRadius="control"
        borderBottomLeftRadius="control"
        boxShadow="md"
        onClick={ handleSideBarButtonClick }
        onKeyDown={ handleKeyDown }
        zIndex="3"
        title={ isDrawerOpen ? 'Close sidebar' : 'Open sidebar' }
        aria-label={ isDrawerOpen ? 'Close sidebar' : 'Open sidebar' }
        aria-expanded={ isDrawerOpen }
        aria-controls={ sidebarId }
        _focusVisible={{ outline: 'var(--chakra-border-widths-2) solid', outlineColor: 'focusRing', outlineOffset: 'calc(-1 * var(--chakra-spacing-0\\.5))' }}
      >
        <Box
          className="codicon codicon-tree-item-expanded"
          transform={ isDrawerOpen ? 'rotate(-90deg)' : 'rotate(+90deg)' }
          boxSize={ 4 }
        />
      </chakra.button>
      <Box
        id={ sidebarId }
        w={ 'var(--chakra-spacing-62\\.5)' }
        maxW="100%"
        flexShrink={ 0 }
        bgColor={ themeColors['sideBar.background'] }
        textStyle="sm"
        overflowY="scroll"
        onScroll={ handleScrollThrottled.current }
        position={{ base: 'absolute', lg: 'relative' }}
        right={ 0 }
        visibility={{ base: isDrawerOpen ? 'visible' : 'hidden', lg: 'visible' }}
        top={{ base: 0, lg: undefined }}
        h="100%"
        pb={ 6 }
        boxShadow={{ base: isDrawerOpen ? 'md' : 'none', lg: 'none' }}
        zIndex={{ base: '2', lg: undefined }}
        onKeyDown={ handleKeyDown }
        borderTopRightRadius="md"
        borderBottomRightRadius="md"
      >
        <TabsRoot unmountOnExit={ false } variant="unstyled" size="free" value={ activeTab } onValueChange={ handleTabChange }>
          <Flex
            columnGap={ 3 }
            position="sticky"
            top={ 0 }
            left={ 0 }
            bgColor={ themeColors['sideBar.background'] }
            zIndex="1"
            px={ 2 }
            flexWrap="wrap"
            alignItems="center"
            boxShadow={ isStuck ? 'md' : 'none' }
            borderTopRightRadius="md"
          >
            <TabsList columnGap={ 3 } flexWrap="wrap">
              <TabsTrigger value="explorer" { ...tabProps } title={ `File explorer (${ shift + cmd }E)` }>Explorer</TabsTrigger>
              <TabsTrigger value="search" { ...tabProps } title={ `Search in files (${ shift + cmd }F)` }>Search</TabsTrigger>
            </TabsList>
            { actionBarRenderer?.() }
          </Flex>
          <TabsContent value="explorer" p={ 0 }>
            <CodeEditorFileExplorer
              data={ data }
              onFileSelect={ handleFileSelect }
              selectedFile={ selectedFile }
              mainFile={ mainFile }
              isActive={ activeTab === 'explorer' }
              setActionBarRenderer={ setActionBarRenderer }
            />
          </TabsContent>
          <TabsContent value="search" p={ 0 }>
            <CodeEditorSearch
              data={ data }
              onFileSelect={ handleFileSelect }
              models={ models }
              isInputStuck={ isStuck }
              isActive={ activeTab === 'search' }
              setActionBarRenderer={ setActionBarRenderer }
              defaultValue={ searchValue }
            />
          </TabsContent>
        </TabsRoot>
      </Box>
    </>
  );
};

export default React.memo(CodeEditorSideBar);
