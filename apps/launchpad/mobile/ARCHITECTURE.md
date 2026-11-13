# Sidiora native architecture and build prerequisites

The application lives in `apps/launchpad/mobile`. Its [package manifest](package.json) pins React Native 0.86.0 and React 19.2.4. Android enables Hermes and the New Architecture, targets SDK 36 and requires Android 11/API 30 or later. The [iOS Podfile](ios/Podfile) and Xcode project target iOS 17 or later. The Sidiora platform projects and Gradle 9.3.1 wrapper are checked in; template licensing is retained in [TEMPLATE-LICENSE](TEMPLATE-LICENSE).

## Application and network boundaries

[App.tsx](src/App.tsx) loads native release configuration, validates it through [readConfiguration](src/runtime/service-client.ts), then mounts discovery, token, portfolio, watchlist, profile and notification screens. Chat and live screens depend on `optionalFeatures.chat` and `optionalFeatures.live`. Missing or invalid configuration displays an unavailable-service message.

Paxeer mainnet-beta is one BFT network with EVM chain ID 125 (`0x7d`). The kernel is part of the network and proposes blocks; a receipt is final when its block is committed. Public node JSON-RPC uses the HTTPS root of `api1.mainnet-beta.paxeer.network` through `api14.mainnet-beta.paxeer.network`, or `api-mainnet-beta.paxeer.network`; see the [public RPC reference](../../../docs/site/docs/reference/public-rpc.md). Sub-chains are tenant domains in the database. Launchpad's authenticated HTTP services and market streams are application interfaces on this network.

`PublicConfiguration` supplies service origins, `canonicalOrigin`, `identityURI`, `manifestURL`, `manifestDigest`, explicit optional-feature flags and `approvedPlaybackOrigins`. Authenticated `apiOrigin`, `identityOrigin` and `mediaOrigin` must equal the approved HTTPS canonical origin. The SDK validates transport configuration and generated contracts from `sdk/launchpad/packages`; [Metro](metro.config.cjs) watches that shared source tree. Manifest refresh checks chain ID 125 and the configured digest before execution.

## Native storage, sessions and privacy

[protected-storage.ts](src/platform/protected-storage.ts) calls the implemented `SecureSession` bridge in [Android](android/app/src/main/java/ag/sidiora/mobile/SecureSessionModule.kt) and [iOS](ios/Sidiora/SecureSession.swift). Android encrypts protected values with AES-GCM using an Android Keystore key; iOS uses Keychain. WalletConnect protocol state also uses this bridge. Wallet private keys remain with the external wallet.

Authenticated requests use a native cookie vault restricted to the configured HTTPS origin. The native transports manage access, refresh and CSRF cookies, reject redirects, and filter `Set-Cookie` from responses exposed to JavaScript. [MobileServices](src/runtime/service-client.ts) restores current identity through the session/refresh endpoints; [SessionController](src/runtime/session.ts) validates the server session rather than trusting a persisted session record. Account writes require current identity and wallet-signed operation authority. Logout attempts remote session/device revocation and clears local protected storage even when remote revocation fails; the failure remains visible and requires reconciliation.

[SnapshotCache](src/runtime/offline-cache.ts) limits public snapshots to 100 MiB and private snapshots to 20 MiB. Approved public snapshots may persist through native cache files; private snapshots remain in memory and are purged on authorization loss or logout. Offline snapshots are labeled and cannot authorize financial actions.

[LocalPrivacyController](src/platform/privacy.ts) implements optional biometric reopening and locks private screens on backgrounding. `PrivacyPreview` uses Android's `FLAG_SECURE` and an iOS background overlay. Local unlock does not replace server authorization.

## External wallet, return links and push

[WalletConnectNativeProvider](src/runtime/lifecycle.ts) uses WalletConnect SignClient with `eip155:125`, `personal_sign` and `eth_sendTransaction`. It requires `walletConnect.projectId`, `walletLink`, `rpcURL` and `iconURL`, validates chain identity, and invalidates execution on account or chain changes. Native approval and transaction construction live in [quote-transaction.ts](src/runtime/quote-transaction.ts); deployment role code checks live in [native-authority.ts](src/runtime/native-authority.ts).

[PendingOperations](src/runtime/deep-links.ts) persists a versioned journal with request ID, chain ID 125, start time, `/portfolio` return route, operation state, optional transaction hash and callback-consumed flag. A wallet callback only wakes reconciliation. Foreground execution requires online service access, a fresh session, manifest and matching unexpired quote, reconciled receipts and local unlock. Backgrounding or network loss invalidates those gates. Unknown outcomes require reconciliation before another submission; no financial mutation is queued offline. The retry-delay helper caps jitter at 30 seconds.

Canonical HTTPS navigation and wallet return links are validated against the configured origin. Deployment owners must supply the associated host and publish platform association records; intent filters and entitlements alone do not prove association registration.

[PushController and NativePlatformPush](src/runtime/push.ts) implement permission, token-change and registration boundaries using native APNs/FCM registration. Android requires `firebasePush.applicationId`, `senderId`, `apiKey` and `projectId`. Account registration also requires an injected `pushProofIssuer` and `deviceAuthorization` callback, or a supplied push adapter with that authority. The default entry point does not supply those authorities. A provider token alone cannot register an authorized device. Push payload validation permits only an unexpired notification reference leading to `/notifications`, rejects private payload fields, and resolves the notification against the current authenticated inbox. This does not establish end-to-end push delivery or device support.

## Native build prerequisites

Resolve the launchpad pnpm workspace and shared SDK dependencies before native builds. Android settings resolve the Gradle plugin from app-local `node_modules/@react-native/gradle-plugin`.

From the repository root, enter `apps/launchpad/mobile`. Android requires JDK 21 and SDK 36, build-tools 36.0.0 and NDK 27.1.12297006. The project declares AGP 8.12.0 and Kotlin 2.1.20:

```sh
cd apps/launchpad/mobile
export NATIVE_BUILD_CACHE="$(mktemp -d /tmp/sidiora-native-build.XXXXXX)"
GRADLE_USER_HOME="$NATIVE_BUILD_CACHE/gradle-user" ./android/gradlew -p android --project-cache-dir "$NATIVE_BUILD_CACHE/android-project" --no-daemon --max-workers=2 :app:assembleDebug
```

Before any Android release task, the operator must provide all inputs enforced by [app/build.gradle](android/app/build.gradle): `SIDIORA_ANDROID_KEYSTORE`, `SIDIORA_ANDROID_STORE_PASSWORD`, `SIDIORA_ANDROID_KEY_ALIAS`, `SIDIORA_ANDROID_KEY_PASSWORD`, `SIDIORA_SIGNING_OWNER`, `SIDIORA_ASSOCIATED_HOST` and `SIDIORA_PUBLIC_CONFIGURATION`. The keystore must exist; use an absolute path to the managed file. The public configuration is the JSON consumed by `readConfiguration`. Provision secrets through the managed signing environment. With those inputs available, from the same directory:

```sh
GRADLE_USER_HOME="$NATIVE_BUILD_CACHE/gradle-user" ./android/gradlew -p android --project-cache-dir "$NATIVE_BUILD_CACHE/android-project" --no-daemon --max-workers=2 :app:assembleRelease :app:bundleRelease
```

Release tasks fail when required inputs are absent; they do not provide an unsigned release fallback. Expected outputs under `android/app/build/outputs` include `apk/release/app-release.apk` and `bundle/release/app-release.aab`. Artifact production still requires signature inspection and device/store qualification. Keep Gradle user and project caches outside the source tree.

On macOS, use an Xcode, Ruby and CocoaPods toolchain compatible with the selected React Native version. From the repository root:

```sh
cd apps/launchpad/mobile/ios
pod install
xcodebuild -workspace Sidiora.xcworkspace -scheme Sidiora -configuration Release -sdk iphonesimulator -destination 'generic/platform=iOS Simulator' -derivedDataPath ../build/ios CODE_SIGNING_ALLOWED=NO build
xcodebuild -workspace Sidiora.xcworkspace -scheme Sidiora -configuration Release -destination 'generic/platform=iOS' -archivePath ../build/Sidiora.xcarchive CODE_SIGNING_ALLOWED=NO archive
```

Review CocoaPods project and lock changes. The simulator build and unsigned archive are compilation steps, not an installable signed IPA. The Release configuration uses manual signing inputs `SIDIORA_IOS_TEAM`, `SIDIORA_IOS_SIGNING_IDENTITY` and `SIDIORA_IOS_PROVISIONING_PROFILE`; configuration and entitlements also consume `SIDIORA_PUBLIC_CONFIGURATION`, `SIDIORA_ASSOCIATED_HOST` and `SIDIORA_APNS_ENVIRONMENT`. A signed archive/export requires the operator's approved profile, team, identity and export options. No Xcode project generator is required.

## Qualification boundary

Source tests live in [test](test). Their existence does not establish a signed Android/iOS build, working external wallet, push delivery or physical-device support. The [account UI guide](../qualification/account-ui/README.md) describes its browser and Android device runners. The [market UI guide](../qualification/market-ui/README.md) distinguishes arithmetic fixtures, integrated service observations and browser measurements. Tie evidence to the exact candidate, artifacts and environment exercised. Shared load/restore and final capacity acceptance remain separate release work.
