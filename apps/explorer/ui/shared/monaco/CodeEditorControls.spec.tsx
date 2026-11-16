// @vitest-environment jsdom

import React from 'react';

import { shift, cmd } from 'toolkit/utils/htmlEntities';
import { render } from 'ui/shared/layout/testWrapper';
import { expect, test, vi } from 'vitest';
import { fireEvent, screen, within } from 'vitest/lib';

import CodeEditorFileExplorer from './CodeEditorFileExplorer';
import CodeEditorSearchResultItem from './CodeEditorSearchResultItem';
import CodeEditorSideBar from './CodeEditorSideBar';
import CodeEditorTabs from './CodeEditorTabs';
import CoderEditorCollapseButton from './CoderEditorCollapseButton';

test('editorFileButtonsKeepSelectionAndCloseActions', function editorFileButtonsKeepSelectionAndCloseActions() {
  const selected: Array<string> = [];
  const closed: Array<string> = [];
  const props = {
    tabs: [ 'Token.sol', 'Owner.sol' ],
    activeTab: 'Token.sol',
    onTabSelect: (path: string) => selected.push(path),
    onTabClose: (path: string) => closed.push(path),
  };
  const view = render(<CodeEditorTabs { ...props }/>);
  const files = screen.getByRole('group', { name: 'Open files' });
  const owner = within(files).getByRole('button', { name: 'Owner.sol', pressed: false });
  owner.focus();
  expect(document.activeElement).toBe(owner);
  expect(owner.tagName).toBe('BUTTON');
  fireEvent.click(owner);
  expect(selected).toEqual([ 'Owner.sol' ]);
  fireEvent.click(within(files).getAllByRole('button', { name: 'Close' })[1]);
  expect(closed).toEqual([ 'Owner.sol' ]);
  expect(selected).toEqual([ 'Owner.sol' ]);

  view.rerender(<CodeEditorTabs { ...props } tabs={ [ 'Token.sol' ] }/>);
  const close = screen.getByRole('button', { name: 'Close' }) as HTMLButtonElement;
  expect(close.disabled).toBe(true);
  fireEvent.click(close);
  expect(closed).toEqual([ 'Owner.sol' ]);
});

test('editorCollapseButtonExposesStateAndHonorsDisabled', function editorCollapseButtonExposesStateAndHonorsDisabled() {
  let activations = 0;
  const onClick = vi.fn(() => activations++);
  const view = render(<CoderEditorCollapseButton label="Expand all" isCollapsed isDisabled onClick={ onClick }/>);
  const button = screen.getByRole('button', { name: 'Expand all' }) as HTMLButtonElement;
  expect(button.disabled).toBe(true);
  expect(button.getAttribute('aria-expanded')).toBe('false');
  fireEvent.click(button);
  expect(activations).toBe(0);

  view.rerender(<CoderEditorCollapseButton label="Collapse all" isCollapsed={ false } onClick={ onClick }/>);
  const enabled = screen.getByRole('button', { name: 'Collapse all' });
  enabled.focus();
  expect(document.activeElement).toBe(enabled);
  expect(enabled.getAttribute('aria-expanded')).toBe('true');
  fireEvent.click(enabled);
  expect(activations).toBe(1);
});

test('editorFileExplorerSelectsSourceThroughFocusableButtons', function editorFileExplorerSelectsSourceThroughFocusableButtons() {
  const indices: Array<number> = [];
  const onFileSelect = vi.fn((index: number) => indices.push(index));
  const setActionBarRenderer = vi.fn();
  render(
    <CodeEditorFileExplorer
      data={ [ { file_path: 'contracts/Token.sol', source_code: 'contract Token {}' } ] }
      selectedFile="contracts/Token.sol"
      onFileSelect={ onFileSelect }
      isActive
      setActionBarRenderer={ setActionBarRenderer }
    />,
  );
  const source = screen.getByRole('button', { name: 'Token.sol', pressed: true });
  expect(source.tagName).toBe('BUTTON');
  source.focus();
  expect(document.activeElement).toBe(source);
  fireEvent.click(source);
  expect(indices).toEqual([ 0 ]);
});

test('editorSearchResultKeepsItsSourceLocation', function editorSearchResultKeepsItsSourceLocation() {
  const locations: Array<string | null> = [];
  const onClick = vi.fn((event: React.MouseEvent) => {
    locations.push(event.currentTarget.getAttribute('data-file-path'), event.currentTarget.getAttribute('data-line-number'));
  });
  render(
    <CodeEditorSearchResultItem
      filePath="Token.sol"
      lineContent="contract Token {}"
      startLineNumber={ 7 }
      endLineNumber={ 7 }
      startColumn={ 10 }
      endColumn={ 15 }
      onClick={ onClick }
    />,
  );
  const result = screen.getByRole('button', { name: 'contract Token {}' });
  expect(result.tagName).toBe('BUTTON');
  result.focus();
  expect(document.activeElement).toBe(result);
  fireEvent.click(result);
  expect(locations).toEqual([ 'Token.sol', '7' ]);
});

test('editorSidebarKeepsControlsInKeyboardOrder', function editorSidebarKeepsControlsInKeyboardOrder() {
  const selected: Array<number> = [];
  const onFileSelect = vi.fn((index: number) => selected.push(index));
  render(
    <CodeEditorSideBar
      data={ [ { file_path: 'Token.sol', source_code: 'contract Token {}' } ] }
      selectedFile="Token.sol"
      onFileSelect={ onFileSelect }
      monaco={ undefined }
      editor={ undefined }
      models={ new Map() }
    />,
  );
  const toggle = screen.getByRole('button', { name: 'Open sidebar' });
  toggle.focus();
  fireEvent.click(toggle);
  const explorer = screen.getByRole('tab', { name: `File explorer (${ shift + cmd }E)` });
  expect(toggle.compareDocumentPosition(explorer) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  const tabs = explorer.closest('[role="tablist"]')!;
  expect(tabs.querySelector('button:not([role="tab"])')).toBeNull();
  explorer.focus();
  fireEvent.keyDown(explorer, { key: 'Escape' });
  expect(toggle.getAttribute('aria-expanded')).toBe('false');
  expect(document.activeElement).toBe(toggle);
  fireEvent.click(toggle);
  fireEvent.keyDown(toggle, { key: 'Escape' });
  expect(toggle.getAttribute('aria-expanded')).toBe('false');
  expect(selected).toEqual([]);
});
