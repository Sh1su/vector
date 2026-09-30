package app.vectra.core.model

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

// DTOs für Wartung, Service, Kosten, Fahrten und Dokumente (api/openapi.yaml), nur genutzte Felder.

@Serializable
data class Money(@SerialName("amount_minor") val amountMinor: Long, val currency: String)

// --- Wartung ---

@Serializable
data class DueStatus(
    @SerialName("item_id") val itemId: String,
    @SerialName("vehicle_id") val vehicleId: String? = null,
    val title: String,
    val level: String,
    @SerialName("reason_trigger") val reasonTrigger: String? = null,
    @SerialName("due_date") val dueDate: String? = null,
    @SerialName("days_remaining") val daysRemaining: Int? = null,
    @SerialName("due_total") val dueTotal: Quantity? = null,
    @SerialName("distance_remaining") val distanceRemaining: DisplayValue? = null,
    @SerialName("estimated_due_date") val estimatedDueDate: String? = null,
    val estimated: Boolean = false,
)

@Serializable
data class CompletionCreate(
    val kind: String,
    @SerialName("completed_on") val completedOn: String,
    @SerialName("completed_odometer") val completedOdometer: QuantityInput? = null,
    val reason: String? = null,
)

// --- Service ---

@Serializable
data class CostItem(val kind: String, val label: String? = null, @SerialName("amount_minor") val amountMinor: Long)

@Serializable
data class ServiceTotals(val total: Money, val parts: Money, val labor: Money, val other: Money)

@Serializable
data class ServiceEntry(
    val id: String,
    val version: Long = 0,
    @SerialName("occurred_at") val occurredAt: String,
    @SerialName("time_zone") val timeZone: String,
    val kind: String,
    val title: String,
    val currency: String,
    val odometer: QuantityInput? = null,
    @SerialName("provider_name") val providerName: String? = null,
    @SerialName("cost_unknown") val costUnknown: Boolean = false,
    val totals: ServiceTotals? = null,
    @SerialName("completes_maintenance_item_ids") val completes: List<String> = emptyList(),
)

@Serializable
data class ServiceEntryCreate(
    val id: String,
    @SerialName("occurred_at") val occurredAt: String,
    @SerialName("time_zone") val timeZone: String,
    @SerialName("time_precision") val timePrecision: String = "date_only",
    val kind: String,
    val title: String,
    val currency: String,
    val odometer: QuantityInput? = null,
    @SerialName("provider_name") val providerName: String? = null,
    @SerialName("cost_unknown") val costUnknown: Boolean = false,
    @SerialName("cost_items") val costItems: List<CostItem> = emptyList(),
    @SerialName("completes_maintenance_item_ids") val completes: List<String> = emptyList(),
    @SerialName("confirm_anomalies") val confirmAnomalies: List<String>? = null,
    @SerialName("anomaly_reason") val anomalyReason: String? = null,
)

// --- Kosten ---

@Serializable
data class CostGroup(val key: String, @SerialName("amount_minor") val amountMinor: Long)

@Serializable
data class CostCurrencyReport(
    val currency: String,
    @SerialName("running_total_minor") val runningTotalMinor: Long,
    val groups: List<CostGroup> = emptyList(),
    @SerialName("per_distance") val perDistance: DisplayValue? = null,
    @SerialName("per_day") val perDay: DisplayValue? = null,
)

@Serializable
data class CostReport(
    val from: String,
    val to: String,
    val distance: DisplayValue? = null,
    @SerialName("ownership_days") val ownershipDays: Int? = null,
    val currencies: List<CostCurrencyReport> = emptyList(),
    val hints: List<String> = emptyList(),
)

@Serializable
data class CostOccurrence(
    @SerialName("plan_id") val planId: String,
    @SerialName("due_on") val dueOn: String,
    val state: String,
    val amount: Money,
    val title: String = "",
)

@Serializable
data class CostOccurrenceList(val items: List<CostOccurrence>, @SerialName("more_open") val moreOpen: Int = 0)

@Serializable
data class CostEntry(
    val id: String,
    val category: String,
    val title: String,
    @SerialName("incurred_on") val incurredOn: String,
    val amount: Money,
)

@Serializable
data class CostEntryCreate(
    val id: String,
    val category: String,
    val title: String,
    @SerialName("incurred_on") val incurredOn: String,
    val amount: Money,
)

// --- Fahrten ---

@Serializable
data class TripCategory(val id: String, val name: String, val kind: String, @SerialName("purpose_required") val purposeRequired: Boolean = false, val active: Boolean = true)

@Serializable
data class Trip(
    val id: String,
    val version: Long = 0,
    @SerialName("started_at") val startedAt: String,
    @SerialName("ended_at") val endedAt: String? = null,
    @SerialName("time_zone") val timeZone: String,
    @SerialName("start_odometer") val startOdometer: QuantityInput,
    @SerialName("end_odometer") val endOdometer: QuantityInput? = null,
    @SerialName("start_location") val startLocation: String? = null,
    @SerialName("end_location") val endLocation: String? = null,
    val purpose: String? = null,
    @SerialName("category_id") val categoryId: String,
    val status: String = "closed",
    val distance: DisplayValue? = null,
    @SerialName("gap_before") val gapBefore: DisplayValue? = null,
)

@Serializable
data class TripStart(
    val id: String,
    @SerialName("started_at") val startedAt: String,
    @SerialName("time_zone") val timeZone: String,
    @SerialName("start_odometer") val startOdometer: QuantityInput,
    @SerialName("category_id") val categoryId: String,
    val purpose: String? = null,
    @SerialName("start_location") val startLocation: String? = null,
    @SerialName("confirm_anomalies") val confirmAnomalies: List<String>? = null,
    @SerialName("anomaly_reason") val anomalyReason: String? = null,
)

@Serializable
data class TripFinish(
    @SerialName("ended_at") val endedAt: String,
    @SerialName("end_odometer") val endOdometer: QuantityInput,
    @SerialName("end_location") val endLocation: String? = null,
    @SerialName("confirm_anomalies") val confirmAnomalies: List<String>? = null,
    @SerialName("anomaly_reason") val anomalyReason: String? = null,
)

@Serializable
data class DistanceShare(val key: String, val label: String? = null, val distance: DisplayValue, val trips: Int, @SerialName("share_pct") val sharePct: Double)

@Serializable
data class TripReport(
    val from: String,
    val to: String,
    val total: DisplayValue,
    @SerialName("by_category") val byCategory: List<DistanceShare> = emptyList(),
    val unassigned: DisplayValue? = null,
)

// --- Dokumente und Dateien ---

@Serializable
data class FileDerivative(val kind: String, @SerialName("media_type") val mediaType: String)

@Serializable
data class FileMeta(
    val id: String,
    @SerialName("original_name") val originalName: String,
    @SerialName("media_type") val mediaType: String,
    @SerialName("size_bytes") val sizeBytes: Long,
    @SerialName("received_at") val receivedAt: String,
    val derivatives: List<FileDerivative> = emptyList(),
    @SerialName("duplicate_of") val duplicateOf: String? = null,
) {
    val hasPreview: Boolean get() = derivatives.any { it.kind == "thumbnail" }
}

@Serializable
data class DocumentMeta(
    val id: String,
    val version: Long = 0,
    @SerialName("doc_type") val docType: String,
    val nature: String = "other",
    val title: String,
    @SerialName("document_date") val documentDate: String? = null,
    val issuer: String? = null,
    @SerialName("file_ids") val fileIds: List<String> = emptyList(),
)

@Serializable
data class DocumentCreate(
    val id: String,
    @SerialName("doc_type") val docType: String,
    val title: String,
    @SerialName("file_ids") val fileIds: List<String>,
    @SerialName("document_date") val documentDate: String? = null,
)

@Serializable
data class VehicleImage(val id: String, @SerialName("file_id") val fileId: String, val primary: Boolean = false)

@Serializable
data class VehicleImageCreate(@SerialName("file_id") val fileId: String, val primary: Boolean = true)
