// Nur zur Prüfung: übersetzt die plattformneutralen Compose-Quellen aus :app (Theme, Komponenten,
// Screens) mit Compose Multiplatform für die JVM. So lässt sich die UI auch ohne Android-SDK und
// ohne Google-Maven typprüfen. Aktivieren mit -Pvectra.uiCheck=true.
plugins {
    alias(libs.plugins.kotlin.jvm)
    alias(libs.plugins.kotlin.compose)
    id("org.jetbrains.compose") version "1.7.3"
}

java {
    sourceCompatibility = JavaVersion.VERSION_17
    targetCompatibility = JavaVersion.VERSION_17
}

kotlin {
    compilerOptions { jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17) }
    sourceSets["main"].kotlin.srcDirs(
        "src/main/kotlin",
        "../app/src/main/kotlin/app/vectra/android/ui",
        "../app/src/main/kotlin/app/vectra/android/feature",
    )
}

// Android-spezifische Dateien (Ressourcen-Zugriffe) ersetzt src/main/kotlin durch Stubs.
tasks.withType<org.jetbrains.kotlin.gradle.tasks.KotlinCompile>().configureEach {
    exclude("**/Fonts.kt")
}

dependencies {
    implementation(project(":core"))
    implementation(compose.desktop.currentOs)
    implementation(compose.material3)
}

tasks.register<JavaExec>("renderScreens") {
    group = "verification"
    description = "Rendert die Screens mit Beispieldaten als PNG nach build/screens"
    classpath = sourceSets["main"].runtimeClasspath
    mainClass.set("app.vectra.android.RenderKt")
    workingDir = projectDir
    args(layout.buildDirectory.dir("screens").get().asFile.absolutePath)
}
