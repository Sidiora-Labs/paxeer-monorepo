// @vitest-environment jsdom

import { URI } from 'monaco-editor/esm/vs/base/common/uri';
import { TextModel } from 'monaco-editor/esm/vs/editor/common/model/textModel';
import type { editor } from 'monaco-editor/esm/vs/editor/editor.api';
import { StandaloneServices } from 'monaco-editor/esm/vs/editor/standalone/browser/standaloneServices';
import { IInstantiationService } from 'monaco-editor/esm/vs/platform/instantiation/common/instantiation';

import type { File, FileTree } from '../types';

import { expect, test } from 'vitest';

import addExternalLibraryWarningDecoration from './addExternalLibraryWarningDecoration';
import addFileImportDecorations from './addFileImportDecorations';
import composeFileTree from './composeFileTree';
import getFullPathOfImportedFile from './getFullPathOfImportedFile';

function createModel(source: string, path?: string): editor.ITextModel {
  return StandaloneServices.get(IInstantiationService).createInstance(
    TextModel, source, 'plaintext', TextModel.DEFAULT_CREATION_OPTIONS, path ? URI.file(path) : null,
  );
}

function libraryRegexAlternation() {
  const source = [
    '// L2 { is an unrelated mention of the second linked name',
    'contract Consumer {',
    '}',
    'library L1 {',
    '  function first() external {}',
    '}',
    'library L2 {',
    '  function second() external {}',
    '}',
    'library Math {}',
    'library $Math {}',
  ].join('\n');
  const model = createModel(source, '/Libraries.sol');
  try {
    addExternalLibraryWarningDecoration(model, [
      { name: 'Libraries.sol:L1', address_hash: '0x1' },
      { name: 'Libraries.sol:L2', address_hash: '0x2' },
      { name: '$Math', address_hash: '0x3' },
      { name: 'Other.sol:Consumer', address_hash: '0x4' },
    ]);
    const decorations = model.getAllDecorations();
    expect(decorations.filter(({ options }) => options.className?.trim() === 'risk-warning-primary')
      .map(({ range }) => range.startLineNumber).sort((a, b) => a - b)).toEqual([ 4, 7, 11 ]);
    expect(decorations.filter(({ options }) => options.className?.trim() === 'risk-warning')
      .map(({ range }) => [ range.startLineNumber, range.endLineNumber ])
      .sort((a, b) => a[0] - b[0])).toEqual([ [ 5, 6 ], [ 8, 9 ] ]);
    expect(decorations.every(({ options }) => {
      const messages = options.hoverMessage;
      return Array.isArray(messages) && messages[0].value.includes('external library');
    })).toBe(true);
  } finally {
    model.dispose();
  }
}

function libraryOneLineCrash() {
  const cases = [
    { source: 'library L { function f() external {} }\ncontract Next {}', endLine: 1 },
    {
      source: [
        '  library L {',
        '    function f() external {',
        '      string memory brace = "}"; // }',
        '      /* } { */ if (true) { }',
        '    }',
        '  }',
        'contract Next {}',
      ].join('\n'),
      endLine: 6,
    },
    { source: 'library L\n{\n  function f() external {}\n}\ncontract Next {}', endLine: 4 },
    { source: 'library L {\n  function f() external {}', endLine: 1 },
  ];
  for (const { source, endLine } of cases) {
    const model = createModel(source);
    try {
      addExternalLibraryWarningDecoration(model, [ { name: 'L', address_hash: '0x1' } ]);
      const decorations = model.getAllDecorations();
      expect(decorations).toHaveLength(endLine > 1 ? 2 : 1);
      expect(decorations.some(({ options, range }) => options.className?.trim() === 'risk-warning-primary' && range.startLineNumber === 1)).toBe(true);
      expect(Math.max(...decorations.map(({ range }) => range.endLineNumber))).toBe(endLine);
      for (const { range } of decorations) {
        expect(range.startLineNumber).toBeGreaterThanOrEqual(1);
        expect(range.endLineNumber).toBeLessThanOrEqual(model.getLineCount());
        expect(range.endColumn).toBeLessThanOrEqual(model.getLineMaxColumn(range.endLineNumber));
      }
    } finally {
      model.dispose();
    }
  }
}

function sourceTreePrototypeKeys() {
  const paths = [ '__proto__/x.sol', 'constructor/x.sol', 'result/x.sol', 'toString/x.sol', 'normal/constructor/result/__proto__/y.sol' ];
  const files = paths.map((filePath, index) => ({ file_path: filePath, source_code: `library L${ index } {}` }));
  files.push({ file_path: '/normal/spaces and $symbols.sol', source_code: 'contract C {}' });
  const objectProperties = Object.getOwnPropertyDescriptors(Object.prototype);
  const functionProperties = Object.getOwnPropertyDescriptors(Object);
  const tree = composeFileTree(files);
  const leaves: Array<File> = [];
  function collectFiles(nodes: FileTree) {
    for (const node of nodes) {
      if ('children' in node) {
        collectFiles(node.children);
      } else {
        leaves.push({ file_path: node.file_path, source_code: node.source_code });
      }
    }
  }
  collectFiles(tree);
  expect(tree.map(({ name }) => name)).toEqual([ '__proto__', 'constructor', 'normal', 'result', 'toString' ]);
  expect(leaves.sort((a, b) => a.file_path.localeCompare(b.file_path)))
    .toEqual([ ...files ].sort((a, b) => a.file_path.localeCompare(b.file_path)));
  expect(Object.getOwnPropertyDescriptors(Object.prototype)).toEqual(objectProperties);
  expect(Object.getOwnPropertyDescriptors(Object)).toEqual(functionProperties);
}

function remappingPrecedence() {
  const mappings = [ '@lib/=vendor/', '@lib/utils/=special/' ];
  for (const order of [ mappings, [ ...mappings ].reverse() ]) {
    expect(getFullPathOfImportedFile('/contracts/C.sol', '@lib/utils/X.sol', order)).toBe('/special/X.sol');
    expect(getFullPathOfImportedFile('/contracts/C.sol', '@lib/Y.sol', order)).toBe('/vendor/Y.sol');
  }
  const contextual = [
    '@lib/utils/=global-specific/',
    'contracts/:@lib/utils/=context-specific/',
    'contracts/deep/:@lib/=deep-context/',
  ];
  for (const order of [ contextual, [ ...contextual ].reverse() ]) {
    expect(getFullPathOfImportedFile('/contracts/deep/C.sol', '@lib/utils/X.sol', order)).toBe('/deep-context/utils/X.sol');
    expect(getFullPathOfImportedFile('/contracts/C.sol', '@lib/utils/X.sol', order)).toBe('/context-specific/X.sol');
    expect(getFullPathOfImportedFile('/contractsElsewhere/C.sol', '@lib/utils/X.sol', order)).toBe('/global-specific/X.sol');
  }
  expect(getFullPathOfImportedFile('/contracts/C.sol', '@lib/X.sol', [ 'contracts/C.sol:@lib/=file-context/' ]))
    .toBe('/file-context/X.sol');
  expect(getFullPathOfImportedFile('/contracts/C.sol', '@lib/X.sol', [ 'contracts:@lib/=prefix-context/' ]))
    .toBe('/prefix-context/X.sol');
  expect(getFullPathOfImportedFile('/C.sol', '@lib/X.sol', [ '@lib/=first/', '@lib/=last/' ])).toBe('/last/X.sol');
  expect(getFullPathOfImportedFile('/C.sol', '@lib/X.sol', [ '@lib/=last/', '@lib/=first/' ])).toBe('/first/X.sol');
  expect(getFullPathOfImportedFile('/C.sol', '@lib/X.sol', [ '@lib/=' ])).toBe('/X.sol');
  expect(getFullPathOfImportedFile('/contracts/C.sol', './utils/X.sol', [ 'contracts/utils/=mapped/' ])).toBe('/mapped/X.sol');

  const model = createModel('import "@lib/utils/X.sol";', '/contracts/C.sol');
  try {
    addFileImportDecorations(model);
    const [ decoration ] = model.getAllDecorations().filter(({ options }) => options.inlineClassName === 'import-link');
    expect(decoration).toBeDefined();
    const importPath = model.getValueInRange(decoration.range);
    expect(importPath).toBe('@lib/utils/X.sol');
    const resolvedPath = getFullPathOfImportedFile(model.uri.path, importPath, mappings);
    const files = [ { file_path: '/special/X.sol', source_code: 'library X {}' } ];
    expect(files.find(({ file_path: filePath }) => filePath === resolvedPath)?.source_code).toBe('library X {}');
  } finally {
    model.dispose();
  }
}

test(libraryRegexAlternation);
test(libraryOneLineCrash);
test(sourceTreePrototypeKeys);
test(remappingPrecedence);
