import React, {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from 'react';
import {
  AccessibilityInfo,
  ActivityIndicator,
  findNodeHandle,
  FlatList,
  Pressable,
  ScrollView,
  KeyboardAvoidingView,
  Platform,
  SafeAreaView,
  StatusBar,
  Text as NativeText,
  type TextProps,
  TextInput,
  View,
} from 'react-native';
import { surface, useSurface } from './surface.ts';
export { useSurface } from './surface.ts';
import {
  useResource as useResourceState,
  offlineSnapshotLabel,
} from './runtime/resource.ts';
import type { MobileServices } from './runtime/service-client.ts';
import type { SessionController } from './runtime/session.ts';
import type { SnapshotCache } from './runtime/offline-cache.ts';
import type {
  ForegroundLifecycle,
  WalletExecutionProvider,
} from './runtime/lifecycle.ts';
import type { PendingOperations } from './runtime/deep-links.ts';
import type { MarketSession } from './runtime/market-session.ts';
import type { StreamState } from '../../../../sdk/launchpad/packages/sdk/src/stream.ts';
import type { LocalPrivacyController } from './platform/privacy.ts';
import type { NativeApprovalController } from './runtime/quote-transaction.ts';
import type { PushController } from './runtime/push.ts';
export interface Runtime {
  services: MobileServices;
  session: SessionController;
  cache: SnapshotCache;
  lifecycle: ForegroundLifecycle;
  pending: PendingOperations;
  wallet?: WalletExecutionProvider;
  approval?: NativeApprovalController;
  push?: PushController;
  market?: MarketSession;
  marketState: StreamState;
  measurementState?: { snapshotHealthy: boolean };
  marketData: Map<string, unknown>;
  privacy: LocalPrivacyController;
  routeState: Map<string, RouteMemory>;
  changed(): void;
  navigate(route: Route): void;
}
export type RouteMemory = {
  inputs: Record<string, string>;
  scrollOffset: number;
};
export type Route =
  | {
      name:
        | 'discovery'
        | 'portfolio'
        | 'watchlists'
        | 'notifications'
        | 'chat'
        | 'live';
    }
  | { name: 'token' | 'profile'; address: string };
export const Text = React.forwardRef<NativeText, TextProps>(
  function SurfaceText(props, ref) {
    return (
      <NativeText
        allowFontScaling
        textBreakStrategy="balanced"
        lineBreakStrategyIOS="push-out"
        {...props}
        ref={ref}
      />
    );
  },
);
function useReducedMotion() {
  const [reduced, setReduced] = useState(true);
  useEffect(() => {
    let active = true;
    const listener = AccessibilityInfo.addEventListener(
      'reduceMotionChanged',
      (value) => {
        if (active) setReduced(value);
      },
    );
    void AccessibilityInfo.isReduceMotionEnabled().then((value) => {
      if (active) setReduced(value);
    });
    return () => {
      active = false;
      listener.remove();
    };
  }, []);
  return reduced;
}
const Chrome = createContext<{ header?: ReactNode; footer?: ReactNode }>({});
export function ScreenChrome({
  header,
  footer,
  children,
}: {
  header?: ReactNode;
  footer?: ReactNode;
  children: ReactNode;
}) {
  return (
    <Chrome.Provider value={{ header, footer }}>{children}</Chrome.Provider>
  );
}
export function SurfaceFrame({ children }: { children: ReactNode }) {
  const { ui, color, scheme } = useSurface();
  return (
    <SafeAreaView style={ui.safeArea}>
      <StatusBar
        barStyle={scheme === 'dark' ? 'light-content' : 'dark-content'}
        backgroundColor={color.background}
      />
      <KeyboardAvoidingView
        style={ui.page}
        behavior={Platform.OS === 'ios' ? 'padding' : 'height'}
      >
        {children}
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}
export function Button({
  label,
  onPress,
  disabled = false,
  selected,
}: {
  label: string;
  onPress(): void;
  disabled?: boolean;
  selected?: boolean;
}) {
  const { ui, color } = useSurface();
  const [focused, setFocused] = useState(false);
  const [hovered, setHovered] = useState(false);
  const [busy, setBusy] = useState(false);
  const reducedMotion = useReducedMotion();
  const running = useRef(false);
  const unavailable = disabled || busy;
  const press = async () => {
    if (unavailable || running.current) return;
    running.current = true;
    setBusy(true);
    try {
      await onPress();
    } finally {
      running.current = false;
      setBusy(false);
    }
  };
  return (
    <Pressable
      onFocus={() => setFocused(true)}
      onBlur={() => setFocused(false)}
      onHoverIn={() => setHovered(true)}
      onHoverOut={() => setHovered(false)}
      accessibilityRole="button"
      accessibilityLabel={label}
      aria-busy={busy}
      aria-selected={selected}
      accessibilityState={{
        disabled: unavailable,
        busy,
        ...(selected === undefined ? {} : { selected }),
      }}
      disabled={unavailable}
      onPress={() => void press()}
      style={({ pressed }) => [
        ui.button,
        hovered && ui.buttonHover,
        (pressed || selected) && ui.buttonPressed,
        unavailable && ui.buttonDisabled,
        focused && ui.buttonFocus,
      ]}
    >
      <Text style={[ui.buttonText, unavailable && ui.buttonDisabledText]}>
        {label}
      </Text>
      {busy && (
        <ActivityIndicator
          color={color.onDisabled}
          style={ui.buttonProgress}
          animating={!reducedMotion}
          hidesWhenStopped={false}
        />
      )}
    </Pressable>
  );
}
export function Heading({
  children,
  focusOnMount = false,
}: {
  children: ReactNode;
  focusOnMount?: boolean;
}) {
  const { ui } = useSurface();
  const ref = useRef<NativeText>(null);
  useEffect(() => {
    if (!focusOnMount) return;
    const frame = requestAnimationFrame(() => {
      const target = findNodeHandle(ref.current);
      if (target) AccessibilityInfo.setAccessibilityFocus(target);
    });
    return () => cancelAnimationFrame(frame);
  }, [focusOnMount]);
  return (
    <Text ref={ref} accessibilityRole="header" style={ui.heading}>
      {children}
    </Text>
  );
}
export function Card({ children }: { children: ReactNode }) {
  const { ui } = useSurface();
  return <View style={ui.card}>{children}</View>;
}
export function Copyable({ label, value }: { label: string; value: string }) {
  const { ui } = useSurface();
  return (
    <View style={ui.field}>
      <Text style={ui.subtle}>{label}</Text>
      <Text
        selectable
        accessibilityLabel={`${label}: ${value}`}
        style={/^-?\d+$/.test(value) ? ui.number : ui.text}
      >
        {value}
      </Text>
    </View>
  );
}
export function Field({
  label,
  value,
  onChange,
  keyboardType = 'default',
  multiline = false,
}: {
  label: string;
  value: string;
  onChange(value: string): void;
  keyboardType?: 'default' | 'decimal-pad';
  multiline?: boolean;
}) {
  const { ui, color } = useSurface();
  const [focused, setFocused] = useState(false);
  return (
    <View style={ui.field}>
      <Text style={ui.subtle}>{label}</Text>
      <TextInput
        accessibilityLabel={label}
        value={value}
        onChangeText={onChange}
        keyboardType={keyboardType}
        autoCorrect={false}
        importantForAutofill="noExcludeDescendants"
        autoCapitalize="none"
        allowFontScaling
        multiline={multiline}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        selectionColor={color.focus}
        placeholderTextColor={color.muted}
        style={[ui.input, multiline && ui.multiline, focused && ui.inputFocus]}
      />
    </View>
  );
}
function memory(runtime: Runtime, key: string): RouteMemory {
  let value = runtime.routeState.get(key);
  if (!value) value = { inputs: {}, scrollOffset: 0 };
  runtime.routeState.delete(key);
  runtime.routeState.set(key, value);
  while (runtime.routeState.size > 128)
    runtime.routeState.delete(runtime.routeState.keys().next().value!);
  return value;
}
export function useRetainedInput(
  runtime: Runtime,
  key: string,
  field: string,
  initial = '',
) {
  const [value, setValue] = useState(
    () => memory(runtime, key).inputs[field] ?? initial,
  );
  useEffect(() => {
    setValue(memory(runtime, key).inputs[field] ?? initial);
  }, [runtime, key, field]);
  return [
    value,
    (next: string) => {
      if (next.length > 8192)
        throw new Error('Route input exceeds local retention bound');
      memory(runtime, key).inputs[field] = next;
      setValue(next);
    },
  ] as const;
}
export function Page({
  title,
  children,
  runtime,
  stateKey,
}: {
  title: string;
  children: ReactNode;
  runtime?: Runtime;
  stateKey?: string;
}) {
  const { ui } = useSurface();
  const chrome = useContext(Chrome);
  const ref = useRef<ScrollView>(null);
  const restored = useRef<string | undefined>(undefined);
  const retained = runtime ? memory(runtime, stateKey ?? title) : undefined;
  return (
    <ScrollView
      ref={ref}
      accessibilityLabel={title}
      contentOffset={{
        x: surface.space.none,
        y: retained?.scrollOffset ?? surface.space.none,
      }}
      onScroll={(event) => {
        if (retained) retained.scrollOffset = event.nativeEvent.contentOffset.y;
      }}
      scrollEventThrottle={surface.timing.scroll}
      onLayout={() => {
        const key = stateKey ?? title;
        if (retained && restored.current !== key) {
          restored.current = key;
          ref.current?.scrollTo({ y: retained.scrollOffset, animated: false });
        }
      }}
      keyboardDismissMode={Platform.OS === 'ios' ? 'interactive' : 'none'}
      keyboardShouldPersistTaps="handled"
      contentInsetAdjustmentBehavior="automatic"
      style={ui.page}
      contentContainerStyle={ui.content}
    >
      {chrome.header}
      <Heading key={stateKey ?? title} focusOnMount>
        {title}
      </Heading>
      {children}
      {chrome.footer}
    </ScrollView>
  );
}
export function ListPage<T>({
  runtime,
  stateKey,
  title,
  data,
  row,
  keyOf,
  header,
  empty = 'No items available.',
}: {
  runtime: Runtime;
  stateKey: string;
  title: string;
  data: readonly T[];
  row(item: T): ReactNode;
  keyOf(item: T): string;
  header?: ReactNode;
  empty?: string;
}) {
  const { ui } = useSurface();
  const chrome = useContext(Chrome);
  const restored = useRef<string | undefined>(undefined);
  const ref = useRef<FlatList<T>>(null),
    retained = memory(runtime, stateKey);
  return (
    <FlatList
      ref={ref}
      accessibilityLabel={title}
      data={data}
      keyExtractor={keyOf}
      renderItem={({ item }) => <View style={ui.listRow}>{row(item)}</View>}
      ListHeaderComponent={
        <View style={ui.listHeader}>
          {chrome.header}
          <Heading key={stateKey ?? title} focusOnMount>
            {title}
          </Heading>
          {header}
        </View>
      }
      ListFooterComponent={<>{chrome.footer}</>}
      ListEmptyComponent={<Text style={ui.text}>{empty}</Text>}
      initialNumToRender={8}
      maxToRenderPerBatch={10}
      windowSize={7}
      keyboardDismissMode={Platform.OS === 'ios' ? 'interactive' : 'none'}
      keyboardShouldPersistTaps="handled"
      contentInsetAdjustmentBehavior="automatic"
      style={ui.page}
      contentContainerStyle={ui.content}
      contentOffset={{ x: surface.space.none, y: retained.scrollOffset }}
      onScroll={(event) => {
        retained.scrollOffset = event.nativeEvent.contentOffset.y;
      }}
      scrollEventThrottle={surface.timing.scroll}
      onLayout={() => {
        if (restored.current !== stateKey) {
          restored.current = stateKey;
          ref.current?.scrollToOffset({
            offset: retained.scrollOffset,
            animated: false,
          });
        }
      }}
      maintainVisibleContentPosition={{ minIndexForVisible: 0 }}
    />
  );
}
export function ErrorNotice({ message }: { message: string }) {
  const { ui } = useSurface();
  useEffect(() => {
    void AccessibilityInfo.announceForAccessibility(message);
  }, [message]);
  return (
    <Text
      accessibilityRole="alert"
      accessibilityLiveRegion="polite"
      style={ui.error}
    >
      {message}
    </Text>
  );
}
export function useResource<T>(
  runtime: Runtime,
  key: string,
  read: (signal: AbortSignal) => Promise<T>,
  privateData = false,
) {
  return useResourceState(runtime, key, read, privateData, () =>
    runtime.lifecycle.networkLost(),
  );
}
export function ResourceStatus({
  loading,
  error,
  offline,
  storedAt,
  reload,
}: {
  loading: boolean;
  error?: string;
  offline: boolean;
  storedAt?: number;
  reload(): void;
}) {
  const { ui, color } = useSurface();
  const reducedMotion = useReducedMotion();
  return (
    <View style={ui.status}>
      {loading && (
        <ActivityIndicator
          color={color.accent}
          accessibilityLabel="Loading current data"
          accessibilityState={{ busy: loading }}
          aria-busy={loading}
          animating={loading && !reducedMotion}
          hidesWhenStopped={false}
        />
      )}
      {offline && <Text style={ui.text}>{offlineSnapshotLabel(storedAt)}</Text>}
      {error && (
        <>
          <ErrorNotice message={error} />
          <Button label="Retry" onPress={reload} />
        </>
      )}
    </View>
  );
}
export async function showAction(
  action: () => Promise<unknown>,
  failure: (value: string) => void,
  done?: () => void,
) {
  try {
    await action();
    done?.();
  } catch (error) {
    const message = error instanceof Error ? error.message : '';
    failure(
      message &&
        message.length <= 256 &&
        !/(?:0x[0-9a-f]{40,}|https?:|jwt|token=|signature|payload|\r|\n)/i.test(
          message,
        )
        ? message
        : 'Operation failed. Refresh current authorization and retry explicitly.',
    );
  }
}
