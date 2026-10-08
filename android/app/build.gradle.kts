plugins {
    id("com.android.application")
    alias(libs.plugins.kotlin.android)
    alias(libs.plugins.kotlin.compose)
    alias(libs.plugins.kotlin.serialization)
    alias(libs.plugins.ksp)
}

android {
    namespace = "app.vectra.android"
    compileSdk = 35

    defaultConfig {
        applicationId = "app.vectra.android"
        minSdk = 26
        targetSdk = 35
        // In der CI: fortlaufende Laufnummer, damit sich jede neue APK als Update installieren lässt.
        versionCode = System.getenv("VECTRA_VERSION_CODE")?.toIntOrNull() ?: 1
        versionName = System.getenv("VECTRA_VERSION_NAME") ?: "0.1.0"
    }

    // Release-Signatur aus Umgebungsvariablen (CI-Secrets). Ohne Schlüssel bleibt die Release-APK
    // unsigniert und lässt sich nicht installieren; dann gilt die Debug-APK.
    val keystore = System.getenv("VECTRA_KEYSTORE_FILE")?.let { file(it) }?.takeIf { it.exists() }
    signingConfigs {
        if (keystore != null) {
            create("release") {
                storeFile = keystore
                storePassword = System.getenv("VECTRA_KEYSTORE_PASSWORD")
                keyAlias = System.getenv("VECTRA_KEY_ALIAS")
                keyPassword = System.getenv("VECTRA_KEY_PASSWORD") ?: System.getenv("VECTRA_KEYSTORE_PASSWORD")
            }
        }
    }

    buildTypes {
        release {
            // R8 bleibt aus, bis die Release-Variante auf einem Gerät geprüft ist.
            isMinifyEnabled = false
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
            if (keystore != null) signingConfig = signingConfigs.getByName("release")
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    buildFeatures {
        compose = true
        buildConfig = true
    }

    packaging {
        resources.excludes += setOf("/META-INF/{AL2.0,LGPL2.1}", "META-INF/versions/9/previous-compilation-data.bin")
    }
}

kotlin {
    compilerOptions { jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17) }
}

ksp {
    arg("room.schemaLocation", "$projectDir/schemas")
}

// Room schreibt und liest schemas/…/1.json; laufen Debug- und Release-KSP parallel, liest einer
// eine halb geschriebene Datei („Empty schema file“). Deshalb nacheinander.
tasks.configureEach {
    if (name == "kspReleaseKotlin") mustRunAfter("kspDebugKotlin")
}

dependencies {
    implementation(project(":core"))
    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.activity.compose)
    implementation(libs.androidx.lifecycle.runtime.compose)
    implementation(libs.androidx.lifecycle.viewmodel.compose)
    implementation(platform(libs.compose.bom))
    implementation(libs.compose.ui)
    implementation(libs.compose.foundation)
    implementation(libs.compose.material3)
    implementation(libs.compose.ui.tooling.preview)
    debugImplementation(libs.compose.ui.tooling)
    implementation(libs.room.runtime)
    implementation(libs.room.ktx)
    ksp(libs.room.compiler)
    implementation(libs.work.runtime)
    implementation(libs.coroutines.android)
}
