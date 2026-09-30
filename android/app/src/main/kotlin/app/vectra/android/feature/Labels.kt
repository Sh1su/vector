package app.vectra.android.feature

import androidx.compose.ui.graphics.vector.ImageVector
import app.vectra.android.ui.VIcons
import app.vectra.core.model.OdometerReading
import app.vectra.core.model.Vehicle
import app.vectra.core.outbox.OutboxEntry
import app.vectra.core.outbox.OutboxStatus
import app.vectra.core.util.Format

// Texte und Zuordnungen, abgestimmt mit der Web-App (web/src/lib/odometer.ts, format.ts).

fun sourceLabel(source: String): String = when (source) {
    "manual" -> "Manuell erfasst"
    "fuel" -> "Tankvorgang"
    "oil" -> "Ölmessung"
    "trip_start" -> "Fahrtbeginn"
    "trip_end" -> "Fahrtende"
    "service" -> "Service"
    "import" -> "Import"
    "assistant" -> "Assistent"
    else -> source
}

fun sourceIcon(source: String): ImageVector = when (source) {
    "fuel" -> VIcons.fuel
    "oil" -> VIcons.oil
    "trip_start", "trip_end" -> VIcons.route
    "service" -> VIcons.wrench
    "import" -> VIcons.sync
    "assistant" -> VIcons.sparkles
    else -> VIcons.gauge
}

fun roleLabel(role: String): String = when (role) {
    "owner" -> "Eigentümer"
    "editor" -> "Bearbeiter"
    else -> "Leser"
}

fun Vehicle.subtitle(): String = listOfNotNull(licensePlate, listOfNotNull(make, model).joinToString(" ").ifBlank { null }).joinToString(" · ")

fun OdometerReading.meterText(unit: String): String = Format.meter(meterValue.canonical, unit)

fun OdometerReading.metaText(): String = Format.date(occurredAt, timeZone) + " · " + sourceLabel(source)

/** Zuwachs gegenüber dem vorigen gültigen Messpunkt (Liste neueste zuerst). */
fun deltas(readings: List<OdometerReading>, unit: String): List<String?> = readings.mapIndexed { i, r ->
    val prev = readings.getOrNull(i + 1) ?: return@mapIndexed null
    val d = r.total.canonical - prev.total.canonical
    if (unit == "h") (if (d >= 0) "+" else "−") + Format.number(Math.abs(d) / 3600.0) + " h"
    else (if (d >= 0) "+" else "−") + Format.group(Math.abs(d) / 1000) + " km"
}

fun OutboxEntry.statusLabel(): String = when (status) {
    OutboxStatus.PENDING, OutboxStatus.SENDING -> "wartet auf Sync"
    OutboxStatus.NEEDS_CONFIRMATION -> "braucht Bestätigung"
    OutboxStatus.CONFLICT -> "Konflikt"
    OutboxStatus.FAILED -> "nicht gesendet"
}
