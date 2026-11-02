import { accessSync, constants, statSync } from 'node:fs';
import { delimiter, resolve } from 'node:path';

function isExecutableFile(path: string): boolean {
  try {
    if (!statSync(path).isFile()) return false;
    accessSync(path, constants.X_OK);
    return true;
  } catch {
    return false;
  }
}

export function resolveAnvilBinary(
  env: Readonly<{ ANVIL_BIN?: string; PATH?: string; [key: string]: string | undefined }> = process.env,
): string {
  const ANVIL_BIN = env.ANVIL_BIN;
  if (ANVIL_BIN && isExecutableFile(ANVIL_BIN)) return ANVIL_BIN;

  for (const directory of env.PATH ? env.PATH.split(delimiter) : []) {
    const candidate = resolve(directory, 'anvil');
    if (isExecutableFile(candidate)) return candidate;
  }

  throw new Error('anvil binary unavailable: set ANVIL_BIN or PATH');
}
