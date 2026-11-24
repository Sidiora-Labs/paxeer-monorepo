import { readFileSync } from 'node:fs';
import { join } from 'node:path';

/** Apply the application's root tokens to DOM component tests. */
export function applySurfaceTestStyle() {
  const css = readFileSync(join(process.cwd(), 'src/app/tokens.css'), 'utf8');
  const root = css.match(/:root\s*\{([^}]+)\}/)?.[1];
  if (!root) throw new Error('Application root tokens are required');
  const style = document.createElement('style');
  style.textContent = `html { font-size: 16px; ${root} }`;
  document.head.appendChild(style);
}
