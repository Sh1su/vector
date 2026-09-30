buildscript {
    // Das Android-Gradle-Plugin muss im selben Classloader liegen wie das Kotlin-Plugin. Es wird nur
    // geladen, wenn settings.gradle.kts :app einbindet; so baut :core auch ohne Google-Maven.
    if (System.getProperty("vectra.withApp") == "true") {
        repositories {
            google()
            mavenCentral()
        }
        dependencies {
            classpath("com.android.tools.build:gradle:8.7.3")
        }
    }
}

plugins {
    alias(libs.plugins.kotlin.jvm) apply false
    alias(libs.plugins.kotlin.android) apply false
    alias(libs.plugins.kotlin.serialization) apply false
    alias(libs.plugins.kotlin.compose) apply false
    alias(libs.plugins.ksp) apply false
}
