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

// --- Wartung, Service, Kosten, Fahrten, Dokumente (abgestimmt mit web/src/pages/*.tsx) ---

fun levelLabel(level: String): String = when (level) {
    "overdue" -> "Überfällig"
    "due" -> "Fällig"
    "upcoming" -> "Demnächst"
    "ok" -> "In Ordnung"
    "completed" -> "Erledigt"
    else -> "Noch offen"
}

fun levelChip(level: String): app.vectra.android.ui.ChipKind = when (level) {
    "overdue" -> app.vectra.android.ui.ChipKind.Bad
    "due" -> app.vectra.android.ui.ChipKind.Warn
    "ok", "completed" -> app.vectra.android.ui.ChipKind.Ok
    else -> app.vectra.android.ui.ChipKind.Info
}

private val isoDay = java.time.format.DateTimeFormatter.ofPattern("dd.MM.yyyy")

fun day(iso: String): String = runCatching { isoDay.format(java.time.LocalDate.parse(iso.take(10))) }.getOrDefault(iso)

/** Kurzbeschreibung der Fälligkeit, z. B. „am 10.03.2027 · noch 1.300 km“. */
fun app.vectra.core.model.DueStatus.dueText(): String {
    val parts = mutableListOf<String>()
    dueDate?.let { parts += (if ((daysRemaining ?: 0) < 0) "seit " else "am ") + day(it) }
    distanceRemaining?.let {
        parts += if (it.value < 0) Format.number(-it.value, 0) + " ${it.unit} drüber" else "noch " + Format.number(it.value, 0) + " ${it.unit}"
    }
    estimatedDueDate?.takeIf { estimated }?.let { parts += "voraussichtlich " + day(it) }
    return parts.joinToString(" · ").ifBlank { if (level == "unknown") "Letzte Durchführung im Web eintragen" else "" }
}

fun serviceKindLabel(kind: String): String = when (kind) {
    "maintenance" -> "Wartung"
    "inspection" -> "Inspektion"
    "repair" -> "Reparatur"
    "upgrade" -> "Nachrüstung"
    else -> kind
}

val costCategories: List<Pair<String, String>> = listOf(
    "fee" to "Gebühren", "parking" to "Parken", "toll" to "Maut", "care" to "Pflege", "tax" to "Steuer",
    "insurance" to "Versicherung", "financing" to "Finanzierung", "other" to "Sonstiges",
)

fun costCategoryLabel(key: String): String = when (key) {
    "energy" -> "Kraftstoff/Energie"
    "maintenance" -> "Wartung"
    "inspection" -> "Inspektion"
    "repair" -> "Reparatur"
    "upgrade" -> "Nachrüstung"
    else -> costCategories.firstOrNull { it.first == key }?.second ?: key
}

val docTypes: List<Pair<String, String>> = listOf(
    "invoice" to "Rechnung", "inspection_report" to "HU/TÜV-Bericht", "service_book" to "Checkheft", "workshop_report" to "Werkstattbericht",
    "registration" to "Zulassung", "insurance" to "Versicherung", "maintenance_manual" to "Wartungshandbuch", "owner_manual" to "Bedienungsanleitung",
    "technical_doc" to "Technische Dokumentation", "other" to "Sonstiges",
)

fun docTypeLabel(key: String): String = docTypes.firstOrNull { it.first == key }?.second ?: key
