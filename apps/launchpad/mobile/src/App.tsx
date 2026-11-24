import React, { useEffect, useReducer, useRef, useState } from 'react';
import NetInfo from '@react-native-community/netinfo';
import {
  AccessibilityInfo,
  AppState,
  Linking,
  Platform,
  View,
} from 'react-native';
import {
  MobileServices,
  readConfiguration,
  type PublicConfiguration,
} from './runtime/service-client.ts';
import { MarketSession } from './runtime/market-session.ts';
import { SnapshotCache } from './runtime/offline-cache.ts';
import { SessionController } from './runtime/session.ts';
import { PendingOperations } from './runtime/deep-links.ts';
import {
  ForegroundLifecycle,
  WalletConnectNativeProvider,
  type WalletExecutionProvider,
} from './runtime/lifecycle.ts';
import {
  createNativeQuoteTransactionBuilder,
  NativeApprovalController,
} from './runtime/quote-transaction.ts';
import { type PlatformPush, type DeviceProofIssuer } from './runtime/push.ts';
import { createApplicationPush } from './runtime/push-bootstrap.ts';
import {
  protectedStorage,
  releaseConfiguration,
  publicSnapshotStorage,
  nativePush,
  consumeNotificationLink,
} from './platform/protected-storage.ts';
import {
  LocalPrivacyController,
  setPrivatePreview,
} from './platform/privacy.ts';
import {
  Button,
  Text,
  ErrorNotice,
  useSurface,
  Page,
  ScreenChrome,
  SurfaceFrame,
  type Runtime,
  type Route,
  showAction,
} from './navigation.tsx';
import { Discovery } from './screens/discovery.tsx';
import { Token } from './screens/token.tsx';
import { Portfolio } from './screens/portfolio.tsx';
import { Watchlists } from './screens/watchlists.tsx';
import { Profile } from './screens/profile.tsx';
import { Notifications } from './screens/notifications.tsx';
import { Chat } from './screens/chat.tsx';
import { Live } from './screens/live.tsx';
export default function App({
  configuration,
  wallet,
  push,
  pushProofIssuer,
  deviceAuthorization,
}: {
  configuration?: PublicConfiguration;
  wallet?: WalletExecutionProvider;
  push?: PlatformPush;
  pushProofIssuer?: DeviceProofIssuer;
  deviceAuthorization?: MobileServices['deviceAuthorization'];
} = {}) {
  const { ui } = useSurface();
  const [config, setConfig] = useState<PublicConfiguration>(),
    [error, setError] = useState<string>();
  const [configurationRevision, retryConfiguration] = useReducer(
    (n) => n + 1,
    0,
  );
  useEffect(() => {
    let active = true;
    void Promise.resolve()
      .then(() => configuration ?? releaseConfiguration())
      .then((value) => {
        if (active) setConfig(readConfiguration(value));
      })
      .catch(() => {
        if (active)
          setError(
            'Release service configuration is unavailable. This build cannot connect to services.',
          );
      });
    return () => {
      active = false;
    };
  }, [configuration, configurationRevision]);
  return config ? (
    <ConnectedApp
      config={config}
      wallet={wallet}
      push={push}
      pushProofIssuer={pushProofIssuer}
      deviceAuthorization={deviceAuthorization}
    />
  ) : (
    <SurfaceFrame>
      <Page title="Sidiora">
        {error ? (
          <>
            <ErrorNotice message={error} />
            <Button
              label="Retry configuration"
              onPress={() => {
                setError(undefined);
                retryConfiguration();
              }}
            />
          </>
        ) : (
          <Text style={ui.text}>
            Opening secure service configuration…
          </Text>
        )}
      </Page>
    </SurfaceFrame>
  );
}
function ConnectedApp({
  config,
  wallet,
  push,
  pushProofIssuer,
  deviceAuthorization,
}: {
  config: PublicConfiguration;
  wallet?: WalletExecutionProvider;
  push?: PlatformPush;
  pushProofIssuer?: DeviceProofIssuer;
  deviceAuthorization?: MobileServices['deviceAuthorization'];
}) {
  const { ui } = useSurface();
  const [, render] = useReducer((n) => n + 1, 0),
    [route, setRoute] = useState<Route>({ name: 'discovery' }),
    [error, setError] = useState<string>();
  const runtimeRef = useRef<Runtime | null>(null);
  if (!runtimeRef.current) {
    const privacy = new LocalPrivacyController(render),
      cache = new SnapshotCache(
        100 * 1024 * 1024,
        20 * 1024 * 1024,
        publicSnapshotStorage,
      ),
      session = new SessionController(protectedStorage, cache),
      services = new MobileServices(config, () => session.token),
      pending = new PendingOperations(
        protectedStorage,
        config.canonicalOrigin,
      ),
      lifecycle = new ForegroundLifecycle(
        services,
        session,
        pending,
        render,
        () => !privacy.enabled || privacy.unlocked,
      );
    runtimeRef.current = {
      cache,
      session,
      services,
      pending,
      lifecycle,
      wallet:
        wallet ??
        (config.walletConnect
          ? new WalletConnectNativeProvider(
              config,
              () => {
                runtimeRef.current?.lifecycle.background();
                runtimeRef.current?.market?.background();
                void (async () => {
                  const current = runtimeRef.current;
                  if (!current) return;
                  try {
                    await current.session.logout(services);
                  } finally {
                    const cleanup = await Promise.allSettled([
                      current.pending.clearForAccountSwitch(),
                      current.approval?.purgeForAccountSwitch(),
                    ]);
                    current.routeState.clear();
                    current.marketData.clear();
                    render();
                    if (
                      cleanup.some(
                        (result) => result.status === 'rejected',
                      )
                    )
                      throw new Error(
                        'Protected account cleanup unavailable',
                      );
                  }
                })().catch(() =>
                  setError(
                    'Wallet account or chain changed. Local account data cleared; remote device revocation needs reconnect.',
                  ),
                );
                render();
              },
              createNativeQuoteTransactionBuilder(services),
            )
          : undefined),
      marketState: 'idle',
      measurementState: { snapshotHealthy: false },
      marketData: new Map(),
      privacy,
      routeState: new Map(),
      changed: render,
      navigate: setRoute,
    };
    const value = runtimeRef.current;
    value.push = createApplicationPush({
      session,
      services,
      wallet: value.wallet,
      native: nativePush,
      unlocked: () => !privacy.enabled || privacy.unlocked,
      changed: render,
      platform: push,
      proofIssuer: pushProofIssuer,
      authorizeDevice: deviceAuthorization,
    });
    session.onInvalidated = () => {
      value.routeState.clear();
      value.marketData.clear();
      privacy.lock();
      lifecycle.background();
      value.market?.background();
      render();
    };
    if (value.wallet && config.walletConnect) {
      value.approval = new NativeApprovalController(
        services,
        session,
        pending,
        value.wallet,
        render,
        () => !privacy.enabled || privacy.unlocked,
      );
      lifecycle.additionalExecutionAllowed = () =>
        !value.approval?.blocking;
      lifecycle.reconcileFinancial = async (signal) =>
        value.approval?.hasOutstanding
          ? value.approval.reconcile(signal)
          : undefined;
    }
    let updateTimer: ReturnType<typeof setTimeout> | undefined,
      lastAnnouncement = 0;
    const notify = () => {
      if (!updateTimer)
        updateTimer = setTimeout(() => {
          updateTimer = undefined;
          render();
        }, 250);
    };
    value.market = new MarketSession(services, {
      snapshot: (snapshot) => {
        value.measurementState!.snapshotHealthy = snapshot.topics.every(
          (topic) => topic.watermark?.integrity === 'healthy',
        );
        if (config.measurement)
          console.info(
            'SIDIORA_MEASUREMENT ' +
              JSON.stringify({
                runId: config.measurement.runId,
                kind: 'snapshot',
                at: Date.now(),
                topics: snapshot.topics.map((topic) => topic.topic),
                healthy: snapshot.topics.every(
                  (topic) => topic.watermark?.integrity === 'healthy',
                ),
              }),
          );
        value.marketData.clear();
        for (const topic of snapshot.topics)
          value.marketData.set(topic.topic, topic.payload);
        notify();
      },
      event: (event) => {
        if (event.watermark?.integrity !== 'healthy')
          value.measurementState!.snapshotHealthy = false;
        value.marketData.set(event.topic, event.payload);
        notify();
        if (Date.now() - lastAnnouncement >= 5000) {
          lastAnnouncement = Date.now();
          void AccessibilityInfo.announceForAccessibility(
            'Market data updated',
          );
        }
      },
      state: (state) => {
        if (config.measurement)
          console.info(
            'SIDIORA_MEASUREMENT ' +
              JSON.stringify({
                runId: config.measurement.runId,
                kind: 'stream-state',
                at: Date.now(),
                state,
              }),
          );
        value.marketState = state;
        notify();
      },
    });
  }
  const runtime = runtimeRef.current;
  useEffect(() => {
    let active = true;
    const restoration = (async () => {
      await runtime.pending.restore();
      await runtime.approval?.restore();
      await runtime.privacy.restore();
    })();
    const initialization = restoration.then(async () => {
      await runtime.session.restore(runtime.services);
      await runtime.lifecycle.foreground();
      if (active) render();
    });
    void initialization.catch(() => {
      if (active)
        setError(
          'Session or deployment refresh failed. Public browsing remains available; execution is disabled.',
        );
    });
    const state = AppState.addEventListener('change', (value) => {
      if (value === 'active')
        void runtime.lifecycle
          .foreground()
          .catch(() =>
            setError(
              'Foreground refresh failed. Execution remains disabled.',
            ),
          );
      else {
        runtime.privacy.lock();
        runtime.lifecycle.background();
        runtime.market?.background();
      }
    });
    const receive = (url: string) => {
      void (async () => {
        await restoration;
        if (!active || url.length > 2048) return;
        const parsed = new URL(url);
        if (parsed.username || parsed.password)
          throw new Error('Invalid wallet return authority');
        if (
          parsed.protocol === 'sidiora:' &&
          parsed.host === 'wallet' &&
          parsed.pathname === '/connect' &&
          !parsed.search &&
          !parsed.hash
        ) {
          await runtime.lifecycle.foreground();
          return;
        }
        if (parsed.pathname === '/wallet/connect') {
          if (
            parsed.origin !== config.canonicalOrigin ||
            parsed.search ||
            parsed.hash
          )
            throw new Error('Invalid wallet session return');
          await runtime.lifecycle.foreground();
          return;
        }
        if (parsed.pathname === '/wallet/return') {
          await runtime.pending.receive(url);
          await runtime.lifecycle.foreground();
          setRoute({ name: 'portfolio' });
        } else setRoute(runtime.pending.route(url));
      })().catch(() =>
        setError('Invalid or unrelated application link.'),
      );
    };
    const links = Linking.addEventListener('url', (event) =>
      receive(event.url),
    );
    void Promise.all([Linking.getInitialURL(), consumeNotificationLink()])
      .then(([url, notification]) => {
        if (active && (url || notification))
          receive(url || notification!);
      })
      .catch(() =>
        setError(
          'Application link unavailable. Open notifications from navigation.',
        ),
      );
    return () => {
      active = false;
      state.remove();
      links.remove();
      runtime.lifecycle.background();
      runtime.market?.background();
      runtime.session.cancel();
      runtime.push?.dispose();
    };
  }, [runtime]);
  useEffect(() => {
    const topics = config.measurement
      ? [...config.measurement.topics]
      : route.name === 'token'
        ? [
            `market.price.${route.address}`,
            `market.stats.${route.address}`,
            `market.trades.${route.address}`,
          ]
        : ['market.rankings.trending'];
    const start = () => void runtime.market?.foreground(topics);
    if (AppState.currentState === 'active') start();
    const listener = AppState.addEventListener('change', (state) => {
      if (state === 'active') start();
      else runtime.market?.background();
    });
    return () => {
      listener.remove();
      runtime.market?.background();
    };
  }, [runtime, route.name, route.name === 'token' ? route.address : '']);
  useEffect(() => {
    void setPrivatePreview(
      [
        'portfolio',
        'watchlists',
        'chat',
        'profile',
        'notifications',
      ].includes(route.name),
    ).catch(() =>
      setError(
        'Native preview privacy unavailable. Private screen release qualification is blocked.',
      ),
    );
  }, [route.name]);
  useEffect(() => {
    let reachable: boolean | undefined;
    return NetInfo.addEventListener((state) => {
      const next =
        state.isConnected === true && state.isInternetReachable !== false;
      if (!next) {
        runtime.lifecycle.networkLost();
        runtime.market?.background();
        setError(
          'Network unavailable. Financial actions are disabled; reconnect and refresh before signing.',
        );
      } else if (
        reachable === false &&
        AppState.currentState === 'active'
      ) {
        void runtime.lifecycle
          .foreground()
          .then(() => {
            const topics = config.measurement
              ? [...config.measurement.topics]
              : route.name === 'token'
                ? [
                    `market.price.${route.address}`,
                    `market.stats.${route.address}`,
                    `market.trades.${route.address}`,
                  ]
                : ['market.rankings.trending'];
            return runtime.market?.foreground(topics);
          })
          .catch(() =>
            setError(
              'Service unreachable. Check network or captive portal; execution remains disabled.',
            ),
          );
      }
      reachable = next;
    });
  }, [runtime, route.name, route.name === 'token' ? route.address : '']);
  useEffect(() => {
    if (!config.measurement) return;
    const timer = setInterval(() => {
      console.info(
        'SIDIORA_MEASUREMENT ' +
          JSON.stringify({
            runId: config.measurement!.runId,
            kind: 'snapshot',
            at: Date.now(),
            topics: [...runtime.marketData.keys()],
            healthy:
              runtime.marketState === 'live' &&
              runtime.measurementState?.snapshotHealthy === true,
          }),
      );
      console.info(
        'SIDIORA_MEASUREMENT ' +
          JSON.stringify({
            runId: config.measurement!.runId,
            kind: 'stream-state',
            at: Date.now(),
            state: runtime.marketState,
          }),
      );
    }, 5000);
    return () => clearInterval(timer);
  }, [runtime, config]);
  const login = async () => {
    if (!runtime.wallet)
      throw new Error(
        'A supported native external wallet adapter is required',
      );
    const signal = runtime.session.signal,
      connected = await runtime.wallet.connect(signal),
      challenge = (
        await runtime.services.challenge(connected.address, signal)
      ).data.challenge;
    if (
      challenge.purpose !== 'login' ||
      challenge.wallet.toLowerCase() !==
        connected.address.toLowerCase() ||
      challenge.uri !== config.identityURI ||
      challenge.chainId !== '125' ||
      challenge.domain !== new URL(config.identityURI).host ||
      Date.parse(challenge.expiresAt) <= Date.now()
    )
      throw new Error('Invalid wallet sign-in challenge');
    const signature = await runtime.wallet.signMessage(
      challenge.message,
      signal,
    );
    const response = await runtime.services.login(
      challenge.challengeId,
      connected.address,
      signature,
      connected.proofKind,
      `Sidiora ${Platform.OS}`,
      signal,
    );
    const identity = await runtime.services.currentSession(signal);
    await runtime.session.establish({
      session: response.data.session,
      identity,
    });
    await runtime.lifecycle.foreground();
    runtime.changed();
  };
  const privateRoute = [
      'portfolio',
      'watchlists',
      'profile',
      'notifications',
      'chat',
    ].includes(route.name),
    localLocked = runtime.privacy.enabled && !runtime.privacy.unlocked;
  const names = [
    'discovery',
    'portfolio',
    'watchlists',
    'notifications',
    'chat',
    'live',
  ] as const;
  return (
    <SurfaceFrame>
      <ScreenChrome
        header={
          <>
            <View style={ui.navigation}>
              {names
                .filter(
                  (name) =>
                    name !== 'chat' || config.optionalFeatures.chat,
                )
                .filter(
                  (name) =>
                    name !== 'live' || config.optionalFeatures.live,
                )
                .map((name) => (
                  <Button
                    key={name}
                    selected={route.name === name}
                    label={
                      name === 'discovery'
                        ? 'Discover'
                        : name[0].toUpperCase() + name.slice(1)
                    }
                    onPress={() => setRoute({ name })}
                  />
                ))}
            </View>
            {error && (
              <View style={ui.stack}>
                <ErrorNotice message={error} />
                <Button
                  label="Refresh services and authorization"
                  onPress={() =>
                    showAction(
                      async () => {
                        if (!runtime.session.current)
                          await runtime.session.restore(runtime.services);
                        await runtime.lifecycle.foreground();
                      },
                      setError,
                      () => {
                        setError(undefined);
                        render();
                      },
                    )
                  }
                />
              </View>
            )}
          </>
        }
        footer={
          <View style={ui.footer}>
            {runtime.session.account ? (
              <>
                <Button
                  label="Reconnect external wallet"
                  onPress={() =>
                    showAction(
                      async () => {
                        if (!runtime.wallet)
                          throw new Error('Native wallet unavailable');
                        const connected = await runtime.wallet.connect(
                          runtime.session.signal,
                        );
                        if (
                          connected.address.toLowerCase() !==
                          runtime.session.account?.toLowerCase()
                        )
                          throw new Error(
                            'Connected wallet differs from current account; sign out before switching',
                          );
                        await runtime.lifecycle.foreground();
                      },
                      setError,
                      render,
                    )
                  }
                />
                <Button
                  label="My profile"
                  onPress={() =>
                    setRoute({
                      name: 'profile',
                      address: runtime.session.account!,
                    })
                  }
                />
                <Button
                  label="Sign out and purge private data"
                  onPress={() =>
                    showAction(
                      async () => {
                        runtime.lifecycle.background();
                        runtime.market?.background();
                        runtime.marketData.clear();
                        try {
                          await runtime.session.logout(runtime.services);
                        } finally {
                          const results = await Promise.allSettled([
                            runtime.wallet?.disconnect?.(),
                            runtime.pending.clearForAccountSwitch(),
                            runtime.approval?.purgeForAccountSwitch(),
                          ]);
                          runtime.routeState.clear();
                          runtime.privacy.reset();
                          setRoute({ name: 'discovery' });
                          render();
                          if (
                            results.some(
                              (result) => result.status === 'rejected',
                            )
                          )
                            throw new Error(
                              'Sign out completed. Reopen the app to retry protected storage cleanup.',
                            );
                        }
                      },
                      setError,
                      render,
                    )
                  }
                />
              </>
            ) : (
              <Button
                label="Sign in with external wallet"
                disabled={!runtime.wallet}
                onPress={() => showAction(login, setError, render)}
              />
            )}
            <Button
              label={
                runtime.privacy.enabled
                  ? 'Disable biometric reopening'
                  : 'Enable biometric reopening'
              }
              onPress={() =>
                showAction(
                  () =>
                    runtime.privacy.setEnabled(!runtime.privacy.enabled),
                  setError,
                )
              }
            />
            {runtime.cache.persistenceError && (
              <ErrorNotice message={runtime.cache.persistenceError} />
            )}
            <Button
              label="Purge snapshots"
              onPress={() =>
                showAction(() => runtime.cache.clear(), setError, render)
              }
            />
          </View>
        }
      >
        {privateRoute && (!runtime.session.token || localLocked) ? (
          <Page title={route.name[0].toUpperCase() + route.name.slice(1)}>
            <Text style={ui.text}>
              {!runtime.session.token
                ? 'Sign in to view private account content.'
                : 'Private screens are locked on this device.'}
            </Text>
            {runtime.session.token && localLocked && (
              <Button
                label="Unlock private screens"
                onPress={() =>
                  showAction(() => runtime.privacy.unlock(), setError)
                }
              />
            )}
          </Page>
        ) : (
          <>
            {route.name === 'discovery' && (
              <Discovery runtime={runtime} />
            )}
            {route.name === 'token' && (
              <Token
                key={route.address}
                runtime={runtime}
                address={route.address}
              />
            )}
            {route.name === 'portfolio' && (
              <Portfolio runtime={runtime} />
            )}
            {route.name === 'watchlists' && (
              <Watchlists runtime={runtime} />
            )}
            {route.name === 'profile' && (
              <Profile
                key={route.address}
                runtime={runtime}
                address={route.address}
              />
            )}
            {route.name === 'notifications' && (
              <Notifications runtime={runtime} />
            )}
            {route.name === 'chat' && config.optionalFeatures.chat && (
              <Chat runtime={runtime} />
            )}
            {((route.name === 'chat' && !config.optionalFeatures.chat) ||
              (route.name === 'live' &&
                !config.optionalFeatures.live)) && (
              <Page title="Unavailable">
                <Text style={ui.text}>
                  This feature is unavailable in this deployment.
                </Text>
                <Button
                  label="Back to discovery"
                  onPress={() => setRoute({ name: 'discovery' })}
                />
              </Page>
            )}
            {route.name === 'live' && config.optionalFeatures.live && (
              <Live runtime={runtime} />
            )}
          </>
        )}
      </ScreenChrome>
    </SurfaceFrame>
  );
}
