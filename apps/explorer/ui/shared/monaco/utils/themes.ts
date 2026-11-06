import type * as monaco from 'monaco-editor/esm/vs/editor/editor.api';

import { LIGHT, DARK } from 'toolkit/theme/foundations/palette';

function createEditorTheme(palette: typeof LIGHT, base: monaco.editor.BuiltinTheme): monaco.editor.IStandaloneThemeData {
  return {
    base,
    inherit: true,
    rules: [ { token: 'predefined', foreground: palette.negative } ],
    colors: {
      'editor.background': palette.surface,
      'editorWidget.background': palette.raised,
      'tab.activeBackground': palette.surface,
      'tab.inactiveBackground': palette.canvas,
      'tab.activeForeground': palette.textPrimary,
      'tab.inactiveForeground': palette.textSecondary,
      'tab.border': palette.raised,
      'icon.foreground': palette.textMuted,
      'input.foreground': palette.textPrimary,
      'input.background': palette.raised,
      'list.inactiveSelectionBackground': palette.raised,
      'breadcrumbs.foreground': palette.textSecondary,
      'badge.background': palette.raised,
      'sideBar.background': palette.canvas,
      focusBorder: palette.accent,
      'custom.list.hoverBackground': palette.hairline,
      'custom.findMatchHighlightBackground': palette.positiveSoft,
      'custom.inputOption.activeBackground': palette.accentSoft,
      'custom.inputOption.hoverBackground': palette.hairline,
      'custom.fileLink.hoverForeground': palette.accent,
      'custom.riskWarning.primaryBackground': palette.warningSoft,
      'custom.riskWarning.background': palette.warningSoft,
      'custom.mainContract.header': palette.accentSoft,
      'custom.mainContract.body': palette.surface,
    },
  };
}

export const light = createEditorTheme(LIGHT, 'vs');
export const dark = createEditorTheme(DARK, 'vs-dark');
