# Vectra für Android

Kotlin, Jetpack Compose, Room, WorkManager. Architektur und Stand: `docs/phase-3/10-android.md`.

```bash
./gradlew :core:test                        # Kernmodul (ohne Android-SDK)
./gradlew :app:assembleDebug                # App (Android-SDK nötig, local.properties oder ANDROID_HOME)
./gradlew -Pvectra.uiCheck=true :uicheck:renderScreens   # Screens als PNG nach uicheck/build/screens
```

Integrationstest gegen ein laufendes Backend:

```bash
VECTRA_IT_URL=http://localhost:8080 VECTRA_IT_EMAIL=… VECTRA_IT_PASSWORD=… ./gradlew :core:test
```

Im Emulator erreicht die App ein lokales Backend unter `http://10.0.2.2:8080`. Das Backend muss dafür mit `VECTRA_COOKIE_SECURE=false` laufen, weil sichere Cookies sonst nur über HTTPS gesendet werden.
