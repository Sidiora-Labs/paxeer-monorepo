import type * as monaco from 'monaco-editor/esm/vs/editor/editor.api';

import type { SmartContractExternalLibrary } from 'types/api/contract';

export default function addExternalLibraryWarningDecoration(model: monaco.editor.ITextModel, libraries: Array<SmartContractExternalLibrary>) {
  const options: monaco.editor.IModelDecorationOptions = {
    isWholeLine: true,
    hoverMessage: [
      { value: '**This is an external library linked to the verified contract**' },
      // eslint-disable-next-line max-len
      { value: 'The linked library source code only affects the bytecode part with external `DELEGATECALL` to the library and it is not possible to automatically ensure that provided library is really the one deployed at specified address. If you want to be sure, check the source code of the library at the given address. (See [issue](https://github.com/blockscout/blockscout-rs/issues/532) for more details)',
      },
    ],
  };

  const names = libraries.map(getLibraryName(model)).filter((name): name is string => Boolean(name))
    .map((name) => name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|');

  if (!names) {
    return;
  }

  const declarationMatches = model.findMatches(`^[ \\t]*library[ \\t]+(?:${ names })\\s*\\{`, false, true, false, null, true);
  const decorations: Array<monaco.editor.IModelDeltaDecoration> = [];

  declarationMatches.forEach((firstLineMatch) => {
    const firstLine = firstLineMatch.range.startLineNumber;
    const openingBraceOffset = model.getOffsetAt({
      lineNumber: firstLineMatch.range.endLineNumber,
      column: firstLineMatch.range.endColumn,
    }) - 1;
    const lastLine = getLibraryEndLineNumber(model, openingBraceOffset);

    decorations.push({
      range: {
        startColumn: 1,
        endColumn: model.getLineMaxColumn(firstLine),
        startLineNumber: firstLine,
        endLineNumber: firstLine,
      },
      options: {
        ...options,
        className: '.risk-warning-primary',
        marginClassName: '.risk-warning-primary',
      },
    });

    if (lastLine !== undefined && lastLine > firstLine) {
      decorations.push({
        range: {
          startLineNumber: firstLine + 1,
          endLineNumber: lastLine,
          startColumn: 1,
          endColumn: model.getLineMaxColumn(lastLine),
        },
        options: {
          ...options,
          className: '.risk-warning',
          marginClassName: '.risk-warning',
        },
      });
    }
  });

  model.deltaDecorations([], decorations);
}

function getLibraryEndLineNumber(model: monaco.editor.ITextModel, openingBraceOffset: number) {
  const source = model.getValue().slice(openingBraceOffset);
  const tokens = /\/\*[\s\S]*?\*\/|\/\/[^\r\n]*|"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|[{}]/g;
  let depth = 0;
  for (const match of source.matchAll(tokens)) {
    if (match[0] === '{') {
      depth++;
    } else if (match[0] === '}') {
      depth--;
      if (depth === 0) {
        return model.getPositionAt(openingBraceOffset + match.index).lineNumber;
      }
    }
  }
}

const getLibraryName = (model: monaco.editor.ITextModel) => (library: SmartContractExternalLibrary) => {
  const containsFileName = library.name.includes(':');

  if (!containsFileName) {
    return library.name;
  }

  const [ fileName, libraryName ] = library.name.split(':');

  if (model.uri.path !== `/${ fileName }`) {
    return;
  }

  return libraryName;
};
