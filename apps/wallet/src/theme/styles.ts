import { DEFAULT_SELECTION } from './catalogue';
import { resolveTheme } from './resolve';
import { DEFAULT_ENVIRONMENT } from './schema';

export function themeStyleSheet(): string {
  const dark = resolveTheme(DEFAULT_SELECTION, DEFAULT_ENVIRONMENT).properties;
  const light = resolveTheme(
    { ...DEFAULT_SELECTION, theme: 'light' },
    DEFAULT_ENVIRONMENT
  ).properties;
  const declarations = (properties: Record<string, string>) =>
    Object.entries(properties)
      .map(([name, value]) => `${name}:${value}`)
      .join(';');
  const lightColors = Object.fromEntries(
    Object.entries(light).filter(
      ([name]) => name.startsWith('--color-') || name.startsWith('--elevation-')
    )
  );
  const pairedColors = Object.fromEntries(
    Object.entries(lightColors)
      .filter(([name]) => name.startsWith('--color-') && light[name] !== dark[name])
      .map(([name, value]) => [name, `light-dark(${value},${dark[name]})`])
  );
  return `:root{${declarations(dark)}}
@media(prefers-color-scheme:light){:root:not([data-theme]){${declarations(lightColors)}}}
@supports(color:light-dark(white,black)){:root:not([data-theme]){${declarations(pairedColors)}}}`;
}
