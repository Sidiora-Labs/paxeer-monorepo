declare module 'monaco-editor/esm/vs/base/common/uri' {
  export { Uri as URI } from 'monaco-editor/esm/vs/editor/editor.api';
}

declare module 'monaco-editor/esm/vs/editor/common/model/textModel' {
  import type { editor, Uri } from 'monaco-editor/esm/vs/editor/editor.api';

  interface TextModelCreationOptions {
    isForSimpleWidget: boolean;
    tabSize: number;
    indentSize: number | 'tabSize';
    insertSpaces: boolean;
    detectIndentation: boolean;
    defaultEOL: editor.DefaultEndOfLine;
    trimAutoWhitespace: boolean;
    largeFileOptimizations: boolean;
    bracketPairColorizationOptions: editor.BracketPairColorizationOptions;
  }

  export interface TextModel extends editor.ITextModel {}
  export class TextModel {
    static readonly DEFAULT_CREATION_OPTIONS: TextModelCreationOptions;
    constructor(source: string, languageId: string, creationOptions: TextModelCreationOptions, associatedResource: Uri | null);
  }
}

declare module 'monaco-editor/esm/vs/platform/instantiation/common/instantiation' {
  export interface ServiceIdentifier<T> {
    (...args: Array<unknown>): void;
    readonly type: T;
  }

  export interface InstantiationService {
    createInstance<T, Args extends Array<unknown>>(ctor: new (...args: Args) => T, ...args: Args): T;
  }

  export const IInstantiationService: ServiceIdentifier<InstantiationService>;
}

declare module 'monaco-editor/esm/vs/editor/standalone/browser/standaloneServices' {
  import type { ServiceIdentifier } from 'monaco-editor/esm/vs/platform/instantiation/common/instantiation';

  export const StandaloneServices: {
    get<T>(id: ServiceIdentifier<T>): T;
  };
}
