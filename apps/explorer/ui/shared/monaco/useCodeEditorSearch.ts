import React from 'react';

import type { SearchResult } from './types';

import type { EditorModels } from './useCodeEditorModels';

const WORD_SEPARATORS = '`~!@#$%^&*()-=+[{]}\\|;:\'",.<>/?';

export default function useCodeEditorSearch(
  models: EditorModels,
  term: string,
  isMatchRegex: boolean,
  isMatchCase: boolean,
  isMatchWholeWord: boolean,
) {
  const [ results, setResults ] = React.useState<Array<SearchResult>>([]);

  React.useEffect(() => {
    const decorations: Array<() => void> = [];
    const nextResults: Array<SearchResult> = [];
    models.forEach((model, filePath) => {
      const matches = term ? model.findMatches(term, false, isMatchRegex, isMatchCase, isMatchWholeWord ? WORD_SEPARATORS : null, false) : [];
      const ids = model.deltaDecorations([], matches.map(({ range }) => ({ range, options: { className: 'highlight' } })));
      decorations.push(() => {
        if (!model.isDisposed()) {
          model.deltaDecorations(ids, []);
        }
      });
      if (matches.length) {
        nextResults.push({
          file_path: filePath,
          matches: matches.map(({ range }) => ({ ...range, lineContent: model.getLineContent(range.startLineNumber) })),
        });
      }
    });
    setResults(nextResults);

    return () => decorations.forEach((clear) => clear());
  }, [ models, term, isMatchRegex, isMatchCase, isMatchWholeWord ]);

  return results;
}
