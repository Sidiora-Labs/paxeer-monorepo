// @vitest-environment jsdom
import { act, cleanup, renderHook } from '@testing-library/react';
import { URI } from 'monaco-editor/esm/vs/base/common/uri';
import { TextModel } from 'monaco-editor/esm/vs/editor/common/model/textModel';
import { editor } from 'monaco-editor/esm/vs/editor/editor.api';
import { StandaloneServices } from 'monaco-editor/esm/vs/editor/standalone/browser/standaloneServices';
import { IInstantiationService } from 'monaco-editor/esm/vs/platform/instantiation/common/instantiation';

import type { File } from './types';

import { afterEach, expect, test } from 'vitest';

import type { EditorModelServices, EditorModels } from './useCodeEditorModels';
import useCodeEditorModels from './useCodeEditorModels';
import useCodeEditorSearch from './useCodeEditorSearch';

const createdModels: Array<editor.ITextModel> = [];
const modelServices: EditorModelServices = {
  createModel(source, language, path) {
    const model: editor.ITextModel = StandaloneServices.get(IInstantiationService).createInstance(
      TextModel, source, language, TextModel.DEFAULT_CREATION_OPTIONS, URI.parse(path),
    );
    createdModels.push(model);
    return model;
  },
  setModelLanguage: editor.setModelLanguage,
};

afterEach(() => {
  cleanup();
  createdModels.forEach((model) => {
    if (!model.isDisposed()) {
      model.dispose();
    }
  });
  createdModels.length = 0;
});

function contractFiles(name: string): Array<File> {
  return [
    { file_path: `/contracts/${ name }.sol`, source_code: `contract ${ name } { /* needle ${ name } */ }` },
    { file_path: '/contracts/Token.sol', source_code: `library Token { /* needle ${ name } */ }` },
  ];
}

function requireModel(models: EditorModels, path: string): editor.ITextModel {
  const model = models.get(path);
  expect(model).toBeDefined();
  if (!model) {
    throw new Error(`Missing owned model: ${ path }`);
  }
  return model;
}

function highlights(model: editor.ITextModel) {
  return model.getAllDecorations().filter(({ options }) => options.className === 'highlight');
}

function modelIdentity(model: editor.ITextModel): string {
  const namespace: unknown = JSON.parse(model.uri.authority);
  expect(Array.isArray(namespace)).toBe(true);
  if (!Array.isArray(namespace) || typeof namespace[0] !== 'string') {
    throw new Error('The model namespace must contain its chain and contract identity');
  }
  return namespace[0];
}

function editorModelLifecycle() {
  const firstFiles = contractFiles('First');
  const secondFiles = contractFiles('Second');
  const firstIdentity = JSON.stringify([ '125', '0x1111111111111111111111111111111111111111' ]);
  const secondIdentity = JSON.stringify([ '125', '0x2222222222222222222222222222222222222222' ]);
  const first = renderHook(() => useCodeEditorModels(firstFiles, 'plaintext', firstIdentity));
  act(() => first.result.current.initialize(modelServices));
  const firstToken = requireModel(first.result.current.models, '/contracts/Token.sol');
  const second = renderHook(({ data, identity }) => useCodeEditorModels(data, 'plaintext', identity), {
    initialProps: { data: secondFiles, identity: secondIdentity },
  });
  act(() => second.result.current.initialize(modelServices));
  const secondToken = requireModel(second.result.current.models, '/contracts/Token.sol');
  expect(firstToken.getValue()).toBe(firstFiles[1].source_code);
  expect(secondToken.getValue()).toBe(secondFiles[1].source_code);
  expect(firstToken.uri.toString()).not.toBe(secondToken.uri.toString());
  expect(firstToken.uri.path).toBe('/contracts/Token.sol');
  expect(modelIdentity(firstToken)).toBe(firstIdentity);
  expect(modelIdentity(secondToken)).toBe(secondIdentity);

  const updatedFiles = secondFiles.map((file) => ({ ...file, source_code: `${ file.source_code }\n// updated` }));
  second.rerender({ data: updatedFiles, identity: secondIdentity });
  expect(requireModel(second.result.current.models, '/contracts/Token.sol')).toBe(secondToken);
  updatedFiles.forEach((file) => expect(requireModel(second.result.current.models, file.file_path).getValue()).toBe(file.source_code));
  expect(firstToken.getValue()).toBe(firstFiles[1].source_code);

  const otherChainIdentity = JSON.stringify([ '126', '0x2222222222222222222222222222222222222222' ]);
  const previousModels = Array.from(second.result.current.models.values());
  second.rerender({ data: updatedFiles, identity: otherChainIdentity });
  previousModels.forEach((model) => expect(model.isDisposed()).toBe(true));
  const otherChainToken = requireModel(second.result.current.models, '/contracts/Token.sol');
  expect(otherChainToken.uri.toString()).not.toBe(secondToken.uri.toString());
  expect(modelIdentity(otherChainToken)).toBe(otherChainIdentity);
  expect(otherChainToken.getValue()).toBe(updatedFiles[1].source_code);

  second.rerender({ data: [ updatedFiles[0] ], identity: otherChainIdentity });
  expect(otherChainToken.isDisposed()).toBe(true);
  expect(second.result.current.models.size).toBe(1);
  const remaining = requireModel(second.result.current.models, updatedFiles[0].file_path);
  second.unmount();
  expect(remaining.isDisposed()).toBe(true);
  expect(firstToken.isDisposed()).toBe(false);

  const firstModels = Array.from(first.result.current.models.values());
  first.unmount();
  firstModels.forEach((model) => expect(model.isDisposed()).toBe(true));
  const reopened = renderHook(() => useCodeEditorModels(updatedFiles, 'plaintext', secondIdentity));
  act(() => reopened.result.current.initialize(modelServices));
  expect(requireModel(reopened.result.current.models, '/contracts/Token.sol').getValue()).toBe(updatedFiles[1].source_code);
  expect(new Set(createdModels.map((model) => model.uri.toString())).size).toBe(createdModels.length);
  reopened.unmount();
  createdModels.forEach((model) => expect(model.isDisposed()).toBe(true));
}

function editorSearchScope() {
  const firstFiles = contractFiles('First');
  const secondFiles = contractFiles('Second');
  const first = renderHook(() => useCodeEditorModels(firstFiles, 'plaintext', JSON.stringify([ '125', 'first' ])));
  const second = renderHook(() => useCodeEditorModels(secondFiles, 'plaintext', JSON.stringify([ '125', 'second' ])));
  act(() => {
    first.result.current.initialize(modelServices);
    second.result.current.initialize(modelServices);
  });
  const search = renderHook(({ models, term }) => useCodeEditorSearch(models, term, false, false, false), {
    initialProps: { models: first.result.current.models, term: 'needle' },
  });
  expect(search.result.current.map((file) => file.file_path)).toEqual(firstFiles.map((file) => file.file_path));
  first.result.current.models.forEach((model) => expect(highlights(model)).toHaveLength(1));
  second.result.current.models.forEach((model) => expect(highlights(model)).toHaveLength(0));

  search.rerender({ models: second.result.current.models, term: 'needle' });
  expect(search.result.current.map((file) => file.file_path)).toEqual(secondFiles.map((file) => file.file_path));
  first.result.current.models.forEach((model) => {
    expect(model.isDisposed()).toBe(false);
    expect(highlights(model)).toHaveLength(0);
  });
  search.result.current.forEach((result) => {
    const fileIndex = secondFiles.findIndex((file) => file.file_path === result.file_path);
    expect(fileIndex).toBeGreaterThanOrEqual(0);
    expect(result.matches).toHaveLength(1);
    expect(result.matches[0].lineContent).toBe(secondFiles[fileIndex].source_code);
    const model = requireModel(second.result.current.models, result.file_path);
    expect(model.getValueInRange(result.matches[0])).toBe('needle');
    expect(highlights(model)).toHaveLength(1);
  });
  search.rerender({ models: second.result.current.models, term: '' });
  expect(search.result.current).toEqual([]);
  second.result.current.models.forEach((model) => expect(highlights(model)).toHaveLength(0));
  search.rerender({ models: second.result.current.models, term: 'needle' });
  second.result.current.models.forEach((model) => expect(highlights(model)).toHaveLength(1));
  search.unmount();
  second.result.current.models.forEach((model) => expect(highlights(model)).toHaveLength(0));

  const disposedModels = second.result.current.models;
  const disposedSearch = renderHook(() => useCodeEditorSearch(disposedModels, 'needle', false, false, false));
  second.unmount();
  expect(() => disposedSearch.unmount()).not.toThrow();
}

function editorWholeWord() {
  for (const term of [ 'target', 'true', 'return', 'route', 'user' ]) {
    const identifiers = [ term, `${ term }t`, `t${ term }`, `${ term }r`, `r${ term }`, `${ term }u`, `u${ term }`, `${ term }e`, `e${ term }`,
      `${ term }_x`, `_${ term }`, `1${ term }`, `${ term }2` ].join(' ');
    const punctuation = `(${ term });${ term },${ term }.${ term }+${ term }/${ term }`;
    const files = [ { file_path: '/contracts/Token.sol', source_code: `${ identifiers }\n${ punctuation }` } ];
    const viewer = renderHook(() => useCodeEditorModels(files, 'plaintext', JSON.stringify([ '125', term ])));
    act(() => viewer.result.current.initialize(modelServices));
    const model = requireModel(viewer.result.current.models, files[0].file_path);
    const search = renderHook(({ wholeWord }) => useCodeEditorSearch(viewer.result.current.models, term, false, false, wholeWord), {
      initialProps: { wholeWord: true },
    });
    expect(search.result.current).toHaveLength(1);
    const matches = search.result.current[0].matches;
    expect(matches.map(({ startLineNumber }) => startLineNumber)).toEqual([ 1, 2, 2, 2, 2, 2, 2 ]);
    expect(matches[0].startColumn).toBe(1);
    matches.forEach((match) => expect(model.getValueInRange(match)).toBe(term));
    expect(highlights(model)).toHaveLength(7);
    search.rerender({ wholeWord: false });
    expect(search.result.current[0].matches).toHaveLength(19);
    expect(highlights(model)).toHaveLength(19);
    search.unmount();
    expect(highlights(model)).toHaveLength(0);
    viewer.unmount();
    expect(model.isDisposed()).toBe(true);
  }
}

test('editorModelLifecycle', () => {
  expect.hasAssertions();
  editorModelLifecycle();
});
test('editorSearchScope', () => {
  expect.hasAssertions();
  editorSearchScope();
});
test('editorWholeWord', () => {
  expect.hasAssertions();
  editorWholeWord();
});
