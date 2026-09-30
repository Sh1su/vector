// Vectra Android. Das Modul :core (reines Kotlin/JVM) baut überall; :app braucht ein Android-SDK
// und Google-Maven und wird nur eingebunden, wenn ein SDK gefunden wird.
pluginManagement {
    repositories {
        google {
            content {
                includeGroupByRegex("com\\.android.*")
                includeGroupByRegex("com\\.google\\.android.*")
                includeGroupByRegex("com\\.google\\.testing.*")
                includeGroupByRegex("androidx.*")
            }
        }
        mavenCentral()
        gradlePluginPortal()
    }
}

dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        google {
            content {
                includeGroupByRegex("com\\.android.*")
                includeGroupByRegex("com\\.google\\.android.*")
                includeGroupByRegex("com\\.google\\.testing.*")
                includeGroupByRegex("androidx.*")
            }
        }
        mavenCentral()
    }
}

rootProject.name = "vectra-android"
include(":core")

// Nur zur Typprüfung der Compose-Oberfläche ohne Android-SDK (siehe uicheck/build.gradle.kts).
if (providers.gradleProperty("vectra.uiCheck").orNull == "true") include(":uicheck")

val localProps = java.util.Properties().apply {
    val f = file("local.properties")
    if (f.exists()) f.inputStream().use { load(it) }
}
val sdkDir = localProps.getProperty("sdk.dir") ?: System.getenv("ANDROID_HOME") ?: System.getenv("ANDROID_SDK_ROOT")
val withApp = sdkDir != null && file(sdkDir).exists() && providers.gradleProperty("vectra.coreOnly").orNull != "true"
// Die Wurzel-Build-Datei lädt das Android-Gradle-Plugin nur, wenn :app dabei ist.
System.setProperty("vectra.withApp", withApp.toString())
if (withApp) {
    include(":app")
} else {
    logger.lifecycle("Vectra: kein Android-SDK gefunden, baue ohne :app")
}
