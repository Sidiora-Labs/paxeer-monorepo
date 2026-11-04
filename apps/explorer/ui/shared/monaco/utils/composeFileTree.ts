import type { File, FileTree } from '../types';

import { stripLeadingSlash } from 'toolkit/utils/url';

import sortFileTree from './sortFileTree';

export default function composeFileTree(files: Array<File>) {
  const result: FileTree = [];
  type Level = {
    result: FileTree;
    children: Map<string, Level>;
  };
  const level: Level = { result, children: new Map() };

  files.forEach((file) => {
    const path = stripLeadingSlash(file.file_path);
    const segments = path.split('/');

    segments.reduce((acc, segment, currentIndex, array) => {
      let child = acc.children.get(segment);
      if (!child) {
        child = { result: [], children: new Map() };
        acc.children.set(segment, child);
        acc.result.push({
          name: segment,
          ...(currentIndex === array.length - 1 ? file : { children: child.result }),
        });
      }
      acc.result.sort(sortFileTree);

      return child;
    }, level);
  });

  return result.sort(sortFileTree);
}
