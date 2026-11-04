import { stripLeadingSlash } from 'toolkit/utils/url';

// FIXME support multiline imports - https://base-goerli.blockscout.com/address/0x3442844D5d4938CA70f8C227dB88F6069C0b82A9?tab=contract

export default function getFullPathOfImportedFile(baseFilePath: string, importedFilePath: string, compilerRemappings?: Array<string>) {
  if (importedFilePath[0] !== '.') {
    return applyCompilerRemappings(baseFilePath, importedFilePath, compilerRemappings);
  }

  const baseFileChunks = stripLeadingSlash(baseFilePath).split('/');
  const importedFileChunks = importedFilePath.split('/');

  const result: Array<string> = baseFileChunks.slice(0, -1);

  for (let index = 0; index < importedFileChunks.length; index++) {
    const element = importedFileChunks[index];

    if (element === '.') {
      continue;
    }

    if (element === '..') {
      if (result.length === 0) {
        break;
      }
      result.pop();
      continue;
    }

    result.push(element);
  }

  if (result.length === 0) {
    return;
  }

  return applyCompilerRemappings(baseFilePath, result.join('/'), compilerRemappings);
}

interface Remapping {
  context?: string;
  prefix: string;
  target: string;
}

function formatCompilerRemappings(remappings: Array<string>): Array<Remapping> {
  return remappings.map((item) => {
    const [ source, ...target ] = item.split('=');
    const contextSeparator = source.indexOf(':');
    return {
      context: contextSeparator >= 0 ? source.slice(0, contextSeparator) : undefined,
      prefix: source.slice(contextSeparator + 1),
      target: target.join('='),
    };
  });
}

function applyCompilerRemappings(baseFilePath: string, importedFilePath: string, compilerRemappings: Array<string> = []) {
  const baseSourceUnit = stripLeadingSlash(baseFilePath);
  let bestMatch: Remapping | undefined;

  for (const remapping of formatCompilerRemappings(compilerRemappings)) {
    const context = remapping.context || '';
    if (!remapping.prefix || !baseSourceUnit.startsWith(stripLeadingSlash(context)) || !importedFilePath.startsWith(remapping.prefix)) {
      continue;
    }
    const bestContextLength = bestMatch?.context?.length || 0;
    if (context.length < bestContextLength ||
        (context.length === bestContextLength && bestMatch && remapping.prefix.length < bestMatch.prefix.length)) {
      continue;
    }
    bestMatch = remapping;
  }

  const result = bestMatch ? bestMatch.target + importedFilePath.slice(bestMatch.prefix.length) : importedFilePath;
  return result[0] === '/' ? result : '/' + result;
}
