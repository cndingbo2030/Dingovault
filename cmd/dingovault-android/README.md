# Dingovault Android (gomobile)

This tree holds the **Go mobile library** consumed by a minimal Android shell.

## Layout

- `mobile/` — `gomobile bind` target (`.aar`). Initializes SQLite + indexer, exposes `Init` / `Shutdown` / `Invoke` (Wails-compatible JSON bridge), `SetEventSink`, plus `Version()` and `VaultPath(externalFilesDir)` for scoped storage under `…/Android/data/<app>/files/Dingovault/`. Embeds a copy of `demo-vault/` for first-run content.
- `android-shell/` — Kotlin **WebView** shell that loads `assets/dist` (Svelte `frontend`), injects `android-shim.js` for `window.go.bridge.App`, and produces **APK** / **AAB** (CI builds release artifacts with the debug keystore for attach-to-release; sign properly for Play).

## Local build (AAR)

Requires Android SDK + NDK (`ANDROID_HOME`), Go, and:

```bash
go install golang.org/x/mobile/cmd/gomobile@latest
go install golang.org/x/mobile/cmd/gobind@latest
gomobile init
gomobile bind -androidapi=24 -o dingovault-mobile.aar -target=android ./mobile
```

Copy `dingovault-mobile.aar` to `android-shell/app/libs/`, then from the **repo root**:

```bash
cd frontend && npm ci && npm run build:android
```

Gradle **preBuild** copies `frontend/dist` into `app/src/main/assets/dist` and patches `index.html` to load `../android-shim.js`. Then:

```bash
cd cmd/dingovault-android/android-shell
./gradlew assembleRelease
```

## CI

Release tags trigger `.github/workflows/release.yml`, which runs `gomobile bind` and Gradle **assembleRelease** / **bundleRelease**. The workflow pins **cmdline-tools**, exports **ANDROID_HOME** / **ANDROID_NDK_HOME** / **NDK_HOME**, symlinks **`ndk-bundle`** for gomobile, and uses semantic artifact names such as **`Dingovault-v1.4.x-Android-Mobile-Phone-Tablet.apk`** (see repo **`Makefile`** `RELEASE_*` / `make release-names`).
