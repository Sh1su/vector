package app.vectra.core.model

import kotlinx.serialization.json.Json

/** Gemeinsame JSON-Konfiguration: unbekannte Felder ignorieren, `null` beim Senden weglassen. */
val VectraJson: Json = Json {
    ignoreUnknownKeys = true
    explicitNulls = false
    encodeDefaults = true
    coerceInputValues = true
}
