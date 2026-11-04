import type * as monaco from 'monaco-editor/esm/vs/editor/editor.api';
import React from 'react';

import type { File, Monaco } from './types';

export type EditorModels = ReadonlyMap<string, monaco.editor.ITextModel>;
export interface EditorModelServices {
  createModel: (source: string, language: string, path: string) => monaco.editor.ITextModel;
  setModelLanguage: Monaco['editor']['setModelLanguage'];
}

export default function useCodeEditorModels(data: Array<File>, language: string, identity: string) {
  const viewerId = React.useId();
  const namespace = JSON.stringify([ identity, viewerId ]);
  const ownedModels = React.useRef(new Map<string, monaco.editor.ITextModel>());
  const [ instance, setInstance ] = React.useState<EditorModelServices>();
  const [ models, setModels ] = React.useState<EditorModels>(new Map());

  const modelPath = React.useCallback((filePath: string) => {
    return `contract-source://${ encodeURIComponent(namespace) }${ filePath.split('/').map(encodeURIComponent).join('/') }`;
  }, [ namespace ]);

  const synchronize = React.useCallback((services: EditorModelServices) => {
    const nextModels = new Map<string, monaco.editor.ITextModel>();
    const nextOwnedModels = new Map<string, monaco.editor.ITextModel>();
    data.forEach((file) => {
      const path = modelPath(file.file_path);
      const model = ownedModels.current.get(path) ?? services.createModel(file.source_code, language, path);
      if (model.getValue() !== file.source_code) {
        model.setValue(file.source_code);
      }
      if (model.getLanguageId() !== language) {
        services.setModelLanguage(model, language);
      }
      nextModels.set(file.file_path, model);
      nextOwnedModels.set(path, model);
    });
    ownedModels.current.forEach((model, path) => {
      if (!nextOwnedModels.has(path)) {
        model.dispose();
      }
    });
    ownedModels.current = nextOwnedModels;
    setModels(nextModels);
  }, [ data, language, modelPath ]);

  const initialize = React.useCallback((services: EditorModelServices) => {
    synchronize(services);
    setInstance(services);
  }, [ synchronize ]);

  React.useEffect(() => {
    if (instance) {
      synchronize(instance);
    }
  }, [ instance, synchronize ]);

  React.useEffect(() => () => {
    ownedModels.current.forEach((model) => model.dispose());
    ownedModels.current.clear();
  }, []);

  return { models, modelPath, initialize };
}
