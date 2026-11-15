// @vitest-environment jsdom

import { ChakraProvider, defaultSystem } from '@chakra-ui/react';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import React from 'react';
import { useController, useForm } from 'react-hook-form';

import { afterEach, expect, test } from 'vitest';

import { DragAndDropArea } from './DragAndDropArea';
import { FileInput } from './FileInput';
import { convertFileEntryToFile, FileSelectionReadError, getAllFileEntries } from './utils';

type EnumerationCallback = () => void;

class SelectionStore implements FileSystem {
  readonly name = 'dropped-selection';
  readonly root: FileSystemDirectoryEntry;
  readonly entries = new Map<string, FileSystemEntry>();
  readonly files = new Map<string, File>();
  readonly denied = new Set<string>();
  readonly directoryFailures = new Map<string, { page: number; error: DOMException }>();
  readonly afterEnumeration = new Map<string, EnumerationCallback>();
  readonly reads: Array<string> = [];

  constructor() {
    this.root = this.directory('/');
  }

  private parent(path: string, success?: FileSystemEntryCallback) {
    const parentPath = path.slice(0, path.lastIndexOf('/')) || '/';
    success?.(this.directory(parentPath));
  }

  private lookup(directory: string, path: string | null | undefined, isFile: boolean,
    success?: FileSystemEntryCallback, error?: ErrorCallback) {
    const fullPath = path?.startsWith('/') ? path : `${ directory === '/' ? '' : directory }/${ path || '' }`;
    const entry = this.entries.get(fullPath);
    if (!entry) {
      error?.(new DOMException('Entry does not exist', 'NotFoundError'));
    } else if (entry.isFile !== isFile) {
      error?.(new DOMException('Entry has a different type', 'TypeMismatchError'));
    } else {
      success?.(entry);
    }
  }

  directory(path: string): FileSystemDirectoryEntry {
    const existing = this.entries.get(path);
    if (existing) {
      if (!('createReader' in existing)) {
        throw new Error(`Not a directory: ${ path }`);
      }
      return existing as FileSystemDirectoryEntry;
    }
    const entry: FileSystemDirectoryEntry = {
      filesystem: this,
      fullPath: path,
      name: path.split('/').pop() || '',
      isDirectory: true,
      isFile: false,
      getParent: (success) => this.parent(path, success),
      getDirectory: (child, _options, success, error) => this.lookup(path, child, false, success, error),
      getFile: (child, _options, success, error) => this.lookup(path, child, true, success, error),
      createReader: () => {
        let page = 0;
        const children = Array.from(this.entries.values()).filter((child) =>
          child.fullPath !== path && (child.fullPath.slice(0, child.fullPath.lastIndexOf('/')) || '/') === path,
        );
        return {
          readEntries: (success, error) => {
            const currentPage = page++;
            queueMicrotask(() => {
              this.reads.push(`${ path }:${ currentPage }`);
              const failure = this.directoryFailures.get(path);
              if (failure?.page === currentPage) {
                error?.(failure.error);
              } else {
                const entries = children.slice(currentPage, currentPage + 1);
                success(entries);
                if (entries.length === 0) {
                  this.afterEnumeration.get(path)?.();
                }
              }
            });
          },
        };
      },
    };
    this.entries.set(path, entry);
    if (path !== '/') {
      this.directory(path.slice(0, path.lastIndexOf('/')) || '/');
    }
    return entry;
  }

  write(path: string, content: string): FileSystemFileEntry {
    this.directory(path.slice(0, path.lastIndexOf('/')) || '/');
    const file = new File([ content ], path.split('/').pop() || '', { type: 'text/plain', lastModified: 1234 });
    this.files.set(path, file);
    const entry: FileSystemFileEntry = {
      filesystem: this,
      fullPath: path,
      name: file.name,
      isDirectory: false,
      isFile: true,
      getParent: (success) => this.parent(path, success),
      file: (success, error) => queueMicrotask(() => {
        this.reads.push(path);
        const current = this.files.get(path);
        if (this.denied.has(path)) {
          error?.(new DOMException('Permission denied', 'NotReadableError'));
        } else if (!current) {
          error?.(new DOMException('File was removed', 'NotFoundError'));
        } else {
          success(current);
        }
      }),
    };
    this.entries.set(path, entry);
    return entry;
  }
}

function selectionItems(...entries: Array<FileSystemEntry>): DataTransferItemList {
  // jsdom has no DataTransfer implementation; this is the browser callback boundary.
  return entries.map((entry) => ({ webkitGetAsEntry: () => entry })) as unknown as DataTransferItemList;
}

const priorFile = new File([ 'previous selection' ], 'prior.txt');

function SelectionForm() {
  const { control } = useForm<{ files: Array<File> }>({ defaultValues: { files: [ priorFile ] } });
  const { field } = useController({ control, name: 'files' });
  return React.createElement(React.Fragment, null,
    React.createElement(FileInput<{ files: Array<File> }, 'files'>, {
      field,
      multiple: true,
      children: ({ onChange }) => React.createElement(DragAndDropArea, {
        onDrop: onChange,
        fullFilePath: true,
        children: 'Drop files',
      }),
    }),
    React.createElement('output', { 'aria-label': 'Selected files' }, field.value.map((file) => file.name).join(',')),
  );
}

function renderSelection() {
  render(React.createElement(ChakraProvider, { value: defaultSystem, children: React.createElement(SelectionForm) }));
  return screen.getByText('Drop files');
}

function dropSelection(area: HTMLElement, ...entries: Array<FileSystemEntry>) {
  fireEvent.dragEnter(area);
  expect(area.getAttribute('data-drag-over')).toBe('true');
  fireEvent.drop(area, { dataTransfer: { items: selectionItems(...entries) } });
}

afterEach(cleanup);

async function directoryReadRejectsAfterFirstPage() {
  const store = new SelectionStore();
  store.write('/contracts/first.sol', 'first');
  store.write('/contracts/second.sol', 'second');
  const cause = new DOMException('Directory access lost', 'NotReadableError');
  store.directoryFailures.set('/contracts', { page: 1, error: cause });

  await expect(getAllFileEntries(selectionItems(store.directory('/contracts')))).rejects.toMatchObject({
    name: 'FileSelectionReadError', operation: 'directory', path: '/contracts', cause,
  });
  expect(store.reads).toEqual([ '/contracts:0', '/contracts:1' ]);
}

async function directoryReadPartialRefusesSelectionAndPreservesPriorFiles() {
  const store = new SelectionStore();
  const loose = store.write('/loose.txt', 'loose');
  store.write('/contracts/first.sol', 'first');
  store.write('/contracts/second.sol', 'second');
  store.directoryFailures.set('/contracts', { page: 1, error: new DOMException('Access lost', 'NotReadableError') });
  const area = renderSelection();

  dropSelection(area, loose, store.directory('/contracts'));
  const alert = await screen.findByRole('alert');
  expect(alert.textContent).toContain('Could not read directory "/contracts".');
  expect(alert.textContent).toContain('Selection incomplete; no dropped files were added.');
  expect(screen.getByRole('status', { name: 'Selected files' }).textContent).toBe('prior.txt');
  expect(area.hasAttribute('data-drag-over')).toBe(false);
  expect(store.reads).toEqual([ '/contracts:0', '/contracts:1' ]);

  store.directoryFailures.clear();
  dropSelection(area, loose, store.directory('/contracts'));
  await waitFor(() => expect(screen.getByRole('status', { name: 'Selected files' }).textContent)
    .toBe('prior.txt,loose.txt,contracts/first.sol,contracts/second.sol'));
  expect(screen.queryByRole('alert')).toBeNull();
  expect(area.hasAttribute('data-drag-over')).toBe(false);
}

async function fileEntryErrorRejectsRemovedAndDeniedFiles() {
  for (const failure of [ 'removed', 'denied' ]) {
    const store = new SelectionStore();
    store.write('/contracts/source.sol', 'source');
    const entries = await getAllFileEntries(selectionItems(store.directory('/contracts')));
    const entry = entries[0];
    if (failure === 'removed') {
      store.files.delete(entry.fullPath);
    } else {
      store.denied.add(entry.fullPath);
    }

    await expect(convertFileEntryToFile(entry, true)).rejects.toBeInstanceOf(FileSelectionReadError);
    await expect(convertFileEntryToFile(entry, true)).rejects.toMatchObject({
      operation: 'file', path: '/contracts/source.sol',
      cause: { name: failure === 'removed' ? 'NotFoundError' : 'NotReadableError' },
    });
  }
}

async function fileEntryErrorReleasesDropAndShowsAffectedPath() {
  for (const failure of [ 'removed', 'denied' ]) {
    const store = new SelectionStore();
    store.write('/contracts/readable.sol', 'readable');
    const unreadable = store.write('/contracts/source.sol', 'source');
    store.afterEnumeration.set('/contracts', () => {
      if (failure === 'removed') {
        store.files.delete(unreadable.fullPath);
      } else {
        store.denied.add(unreadable.fullPath);
      }
    });
    const area = renderSelection();

    dropSelection(area, store.directory('/contracts'));
    const alert = await screen.findByRole('alert');
    expect(alert.textContent).toContain('Could not read file "/contracts/source.sol".');
    expect(alert.textContent).toContain('Selection incomplete; no dropped files were added.');
    expect(screen.getByRole('status', { name: 'Selected files' }).textContent).toBe('prior.txt');
    expect(area.hasAttribute('data-drag-over')).toBe(false);
    expect(store.reads).toContain('/contracts/readable.sol');
    expect(store.reads).toContain('/contracts/source.sol');

    store.write(unreadable.fullPath, 'restored');
    store.denied.clear();
    store.afterEnumeration.clear();
    dropSelection(area, store.directory('/contracts'));
    await waitFor(() => expect(screen.getByRole('status', { name: 'Selected files' }).textContent)
      .toBe('prior.txt,contracts/readable.sol,contracts/source.sol'));
    expect(screen.queryByRole('alert')).toBeNull();
    expect(area.hasAttribute('data-drag-over')).toBe(false);
    cleanup();
  }
}

async function completeNestedSelectionPreservesFilesAndMetadata() {
  const store = new SelectionStore();
  store.write('/contracts/first.sol', 'first');
  store.write('/contracts/nested/second.sol', 'second');
  store.directory('/contracts/empty');
  const entries = await getAllFileEntries(selectionItems(store.directory('/contracts')));
  const files = await Promise.all(entries.map((entry) => convertFileEntryToFile(entry, true)));

  expect(files.map((file) => file.name)).toEqual([ 'contracts/first.sol', 'contracts/nested/second.sol' ]);
  expect(files.map((file) => [ file.size, file.type, file.lastModified ])).toEqual([
    [ 5, 'text/plain', 1234 ], [ 6, 'text/plain', 1234 ],
  ]);
  expect(await convertFileEntryToFile(entries[0])).toBe(store.files.get('/contracts/first.sol'));
}

test(directoryReadRejectsAfterFirstPage.name, directoryReadRejectsAfterFirstPage);
test(directoryReadPartialRefusesSelectionAndPreservesPriorFiles.name, directoryReadPartialRefusesSelectionAndPreservesPriorFiles);
test(fileEntryErrorRejectsRemovedAndDeniedFiles.name, fileEntryErrorRejectsRemovedAndDeniedFiles);
test(fileEntryErrorReleasesDropAndShowsAffectedPath.name, fileEntryErrorReleasesDropAndShowsAffectedPath);
test(completeNestedSelectionPreservesFilesAndMetadata.name, completeNestedSelectionPreservesFilesAndMetadata);
