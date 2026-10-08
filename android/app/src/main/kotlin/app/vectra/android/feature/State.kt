package app.vectra.android.feature

import androidx.compose.ui.graphics.ImageBitmap
import app.vectra.core.model.Anomaly
import app.vectra.core.model.CostEntry
import app.vectra.core.model.CostOccurrence
import app.vectra.core.model.CostReport
import app.vectra.core.model.DocumentMeta
import app.vectra.core.model.DueStatus
import app.vectra.core.model.FileMeta
import app.vectra.core.model.ServiceEntry
import app.vectra.core.model.Trip
import app.vectra.core.model.TripCategory
import app.vectra.core.model.TripReport
import app.vectra.core.model.OdometerReading
import app.vectra.core.model.OdometerValue
import app.vectra.core.model.Vehicle
import app.vectra.core.outbox.OutboxEntry

// Zustände der Screens. Die Screens selbst sind zustandslos und plattformneutral; ViewModels und
// Datenhaltung liegen im Android-Teil (app/src/main/kotlin/app/vectra/android/{data,sync}).

data class LoginState(
    val server: String = "",
    val email: String = "",
    val password: String = "",
    val busy: Boolean = false,
    val error: String? = null,
)

data class MonthBar(val label: String, val km: Double?)

/** Kennzeichnet, ob die Anzeige aus dem Cache stammt (offline). */
data class Freshness(val offline: Boolean = false, val syncedAt: String? = null)

data class HomeState(
    val vehicle: Vehicle? = null,
    val current: OdometerValue? = null,
    val recent: List<OdometerReading> = emptyList(),
    val pending: List<OutboxEntry> = emptyList(),
    val freshness: Freshness = Freshness(),
    val loading: Boolean = true,
    val error: String? = null,
    val photo: ImageBitmap? = null,
    val nextDue: DueStatus? = null,
    val yearCosts: String? = null,
) {
    val attention: List<OutboxEntry> get() = pending.filter { it.status != app.vectra.core.outbox.OutboxStatus.PENDING && it.status != app.vectra.core.outbox.OutboxStatus.SENDING }
}

data class OdometerState(
    val vehicle: Vehicle? = null,
    val current: OdometerValue? = null,
    val readings: List<OdometerReading> = emptyList(),
    val months: List<MonthBar> = emptyList(),
    val pending: List<OutboxEntry> = emptyList(),
    val freshness: Freshness = Freshness(),
    val loading: Boolean = true,
    val error: String? = null,
)

data class VehiclesState(
    val vehicles: List<Vehicle> = emptyList(),
    val currentKm: Map<String, String> = emptyMap(),
    val selectedId: String? = null,
    val loading: Boolean = true,
    val error: String? = null,
    val photos: Map<String, ImageBitmap> = emptyMap(),
    val uploading: Boolean = false,
)

enum class ThemeMode(val label: String) { System("System"), Light("Hell"), Dark("Dunkel") }

data class SettingsState(
    val server: String = "",
    val accountName: String = "",
    val accountEmail: String = "",
    val theme: ThemeMode = ThemeMode.System,
    val pendingCount: Int = 0,
    val clockSkewMinutes: Long = 0,
    val version: String = "",
    val online: Boolean = true,
)

/** Befunde eines Outbox-Eintrags für den Bestätigungsdialog (ADR-010, ADR-021). */
data class ConfirmRequest(val entry: OutboxEntry, val anomalies: List<Anomaly>)

/** Fehler eines Formulars; bestätigbare Befunde (ADR-010) erlauben „Trotzdem speichern“ mit Begründung. */
data class FormError(val message: String, val anomalies: List<Anomaly> = emptyList()) {
    val confirmable: Boolean get() = anomalies.isNotEmpty() && anomalies.all { it.confirmable }
}

/** Bestätigung von Befunden beim erneuten Speichern. */
data class Confirmation(val codes: List<String>, val reason: String)

data class MaintenanceState(
    val vehicle: Vehicle? = null,
    val due: List<DueStatus> = emptyList(),
    val services: List<ServiceEntry> = emptyList(),
    val freshness: Freshness = Freshness(),
    val loading: Boolean = true,
    val error: String? = null,
    val books: List<app.vectra.core.model.MaintenanceBook> = emptyList(),
)

data class CostsState(
    val vehicle: Vehicle? = null,
    val year: Int = 0,
    val report: CostReport? = null,
    val occurrences: List<CostOccurrence> = emptyList(),
    val moreOpen: Int = 0,
    val entries: List<CostEntry> = emptyList(),
    val freshness: Freshness = Freshness(),
    val loading: Boolean = true,
    val error: String? = null,
)

data class TripsState(
    val vehicle: Vehicle? = null,
    val trips: List<Trip> = emptyList(),
    val categories: List<TripCategory> = emptyList(),
    val report: TripReport? = null,
    val monthLabel: String = "",
    val freshness: Freshness = Freshness(),
    val loading: Boolean = true,
    val error: String? = null,
) {
    val open: Trip? get() = trips.firstOrNull { it.status == "open" }
}

data class DocumentsState(
    val vehicle: Vehicle? = null,
    val documents: List<DocumentMeta> = emptyList(),
    val files: List<FileMeta> = emptyList(),
    val thumbs: Map<String, ImageBitmap> = emptyMap(),
    val uploading: Boolean = false,
    val message: String? = null,
    val freshness: Freshness = Freshness(),
    val loading: Boolean = true,
    val error: String? = null,
)

/** Chat mit dem Assistenten (ADR-026: Einträge nur als Vorschlag, Bestätigung durch den Nutzer). */
data class AssistantState(
    val status: app.vectra.core.model.AssistantStatus? = null,
    val conversationId: String? = null,
    val messages: List<app.vectra.core.model.AssistantMessage> = emptyList(),
    val pendingText: String? = null,
    val progress: String? = null,
    val pendingProposals: List<app.vectra.core.model.Proposal> = emptyList(),
    val input: String = "",
    val loading: Boolean = true,
    val error: String? = null,
    /** Befunde je Vorschlag, die vor dem Speichern eine Begründung brauchen. */
    val anomalies: Map<String, List<app.vectra.core.model.Anomaly>> = emptyMap(),
    val busyProposal: String? = null,
)

/** Wofür ein Tachofoto aufgenommen wird: Fahrt starten oder eine laufende Fahrt beenden. */
sealed interface TachoPurpose {
    data object Start : TachoPurpose
    data class Finish(val trip: Trip) : TachoPurpose
}

enum class TachoPhase { Uploading, Reading, Ready, Manual, Failed }

/**
 * Tachofoto zur Fahrt: hochladen, auswerten lassen, Werte im Dialog vorbelegen.
 * [capturedAt] ist der Zeitpunkt der Aufnahme und gilt als Start- bzw. Endzeit der Fahrt.
 */
data class TachoCapture(
    val purpose: TachoPurpose,
    val capturedAt: java.time.Instant,
    val preview: ImageBitmap? = null,
    val phase: TachoPhase = TachoPhase.Uploading,
    val fileId: String? = null,
    val reading: app.vectra.core.model.DashboardReading? = null,
    val message: String? = null,
) {
    /** Erkannter Gesamtstand in km (Meilen werden umgerechnet). */
    val km: Double? get() = reading?.odometer?.let { if (it.unit == "mi") it.value * 1.609344 else it.value }
}
