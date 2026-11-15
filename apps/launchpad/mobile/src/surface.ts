import { Platform, StatusBar, StyleSheet, useColorScheme } from 'react-native';

export const surface = {
  space: { none: 0, xs: 4, sm: 8, md: 12, lg: 16, xl: 24, xxl: 32 },
  radius: { control: 8, card: 12 },
  border: { focus: 2 },
  size: { touch: 48, content: 768, videoRatio: 16 / 9, full: '100%' },
  layout: { grow: 1, shrink: 1 },
  timing: { scroll: 100 },
  type: {
    heading: { fontSize: 26, lineHeight: 34, fontWeight: '700' },
    body: { fontSize: 16, lineHeight: 24, fontWeight: '400' },
    label: { fontSize: 16, lineHeight: 24, fontWeight: '600' },
    caption: { fontSize: 14, lineHeight: 20, fontWeight: '400' },
  },
  schemes: {
    light: {
      background: '#f4f6f8',
      panel: '#ffffff',
      text: '#101418',
      muted: '#46535f',
      input: '#e6ecf0',
      edge: '#657582',
      accent: '#235e38',
      onAccent: '#ffffff',
      hover: '#1c4e2e',
      pressed: '#133b22',
      focus: '#265ed0',
      disabled: '#d9e0e5',
      onDisabled: '#47545e',
      error: '#a51d2d',
    },
    dark: {
      background: '#101418',
      panel: '#202830',
      text: '#f4f6f8',
      muted: '#c1c8cf',
      input: '#303b45',
      edge: '#8998a5',
      accent: '#c3eacf',
      onAccent: '#10251a',
      hover: '#a8dcb8',
      pressed: '#93c9a4',
      focus: '#a8c7ff',
      disabled: '#36434e',
      onDisabled: '#c1c8cf',
      error: '#ffb3bb',
    },
  },
} as const;

function styles(scheme: keyof typeof surface.schemes) {
  const color = surface.schemes[scheme];
  const { space, type, layout } = surface;
  const flexibleText = { minWidth: space.none, flexShrink: layout.shrink };
  return StyleSheet.create({
    page: { flex: layout.grow, backgroundColor: color.background },
    safeArea: {
      flex: layout.grow,
      backgroundColor: color.background,
      paddingTop:
        Platform.OS === 'android'
          ? (StatusBar.currentHeight ?? space.none)
          : space.none,
    },
    content: {
      padding: space.lg,
      paddingBottom: space.xxl,
      gap: space.lg,
      width: surface.size.full,
      maxWidth: surface.size.content,
      alignSelf: 'center',
    },
    heading: { ...flexibleText, ...type.heading, color: color.text },
    text: { ...flexibleText, ...type.body, color: color.text },
    subtle: { ...flexibleText, ...type.caption, color: color.muted },
    number: {
      ...flexibleText,
      ...type.body,
      color: color.text,
      fontVariant: ['tabular-nums'],
      textAlign: 'right',
    },
    card: {
      minWidth: space.none,
      backgroundColor: color.panel,
      padding: space.lg,
      gap: space.md,
      borderRadius: surface.radius.card,
    },
    button: {
      minHeight: surface.size.touch,
      minWidth: surface.size.touch,
      maxWidth: surface.size.full,
      flexShrink: layout.shrink,
      justifyContent: 'center',
      paddingLeft: space.lg,
      paddingRight: space.xxl,
      paddingVertical: space.md,
      backgroundColor: color.accent,
      borderRadius: surface.radius.control,
      borderWidth: surface.border.focus,
      borderColor: color.accent,
    },
    buttonProgress: { position: 'absolute', right: space.sm, top: space.md },
    buttonHover: { backgroundColor: color.hover, borderColor: color.hover },
    buttonPressed: {
      backgroundColor: color.pressed,
      borderColor: color.pressed,
    },
    buttonFocus: { borderColor: color.focus },
    buttonText: { ...flexibleText, ...type.label, color: color.onAccent },
    buttonDisabled: {
      backgroundColor: color.disabled,
      borderColor: color.edge,
    },
    buttonDisabledText: { color: color.onDisabled },
    input: {
      minHeight: surface.size.touch,
      minWidth: space.none,
      padding: space.md,
      ...type.body,
      color: color.text,
      backgroundColor: color.input,
      borderRadius: surface.radius.control,
      borderWidth: surface.border.focus,
      borderColor: color.edge,
    },
    inputFocus: { borderColor: color.focus },
    multiline: { textAlignVertical: 'top' },
    field: { minWidth: space.none, gap: space.xs },
    row: {
      flexDirection: 'row',
      gap: space.md,
      flexWrap: 'wrap',
      minWidth: space.none,
    },
    stack: { gap: space.md },
    listRow: { marginBottom: space.lg },
    listHeader: { gap: space.lg, paddingBottom: space.lg },
    navigation: { flexDirection: 'row', flexWrap: 'wrap', gap: space.sm },
    footer: { gap: space.md, paddingTop: space.lg },
    error: { ...flexibleText, ...type.body, color: color.error },
    status: {
      minHeight: surface.size.touch,
      justifyContent: 'center',
      gap: space.md,
    },
    video: {
      width: surface.size.full,
      aspectRatio: surface.size.videoRatio,
      backgroundColor: color.background,
    },
  });
}
const schemeStyles = { light: styles('light'), dark: styles('dark') };
export function useSurface() {
  const scheme = useColorScheme() === 'dark' ? 'dark' : 'light';
  return { ui: schemeStyles[scheme], color: surface.schemes[scheme], scheme };
}
