package app.vectra.core.model

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement

// Datentransferobjekte nach api/openapi.yaml. Nur die Felder, die die App nutzt; unbekannte Felder
// werden beim Lesen ignoriert (siehe VectraJson).

@Serializable
data class Quantity(
    val canonical: Long,
    @SerialName("canonical_unit") val canonicalUnit: String,
    @SerialName("input_value") val inputValue: Double? = null,
    @SerialName("input_unit") val inputUnit: String? = null,
)

@Serializable
data class QuantityInput(val value: Double, val unit: String)

@Serializable
data class DisplayValue(val value: Double, val unit: String)

@Serializable
data class Anomaly(
    val code: String,
    val confirmable: Boolean,
    val message: String? = null,
    @SerialName("related_id") val relatedId: String? = null,
)

@Serializable
data class FieldError(val pointer: String? = null, val code: String? = null, val message: String? = null)

/** RFC 9457 Problem Details (ADR-013). */
@Serializable
data class Problem(
    val type: String = "about:blank",
    val title: String = "",
    val status: Int = 0,
    val detail: String? = null,
    @SerialName("request_id") val requestId: String? = null,
    val errors: List<FieldError> = emptyList(),
    val anomalies: List<Anomaly> = emptyList(),
    val current: JsonElement? = null,
) {
    val confirmableOnly: Boolean get() = anomalies.isNotEmpty() && anomalies.all { it.confirmable }
}

@Serializable
data class Page<T>(val items: List<T>, @SerialName("next_cursor") val nextCursor: String? = null)

@Serializable
data class LoginRequest(val email: String, val password: String)

@Serializable
data class Account(
    val id: String,
    val email: String,
    @SerialName("display_name") val displayName: String,
    @SerialName("is_admin") val isAdmin: Boolean = false,
)

@Serializable
data class Vehicle(
    val id: String,
    @SerialName("display_name") val displayName: String,
    @SerialName("license_plate") val licensePlate: String? = null,
    val make: String? = null,
    val model: String? = null,
    @SerialName("model_year") val modelYear: Int? = null,
    @SerialName("body_type") val bodyType: String = "car",
    @SerialName("usage_meter") val usageMeter: String = "distance",
    val status: String = "active",
    @SerialName("my_role") val myRole: String = "viewer",
    val version: Long = 0,
) {
    val canEdit: Boolean get() = myRole == "owner" || myRole == "editor"
    val meterUnit: String get() = if (usageMeter == "engine_hours") "h" else "km"
}

@Serializable
data class OdometerReading(
    val id: String,
    @SerialName("vehicle_id") val vehicleId: String,
    @SerialName("occurred_at") val occurredAt: String,
    @SerialName("time_zone") val timeZone: String,
    @SerialName("meter_value") val meterValue: Quantity,
    val total: Quantity,
    val source: String = "manual",
    val status: String = "valid",
    val note: String = "",
    @SerialName("confirmed_anomalies") val confirmedAnomalies: List<Anomaly> = emptyList(),
    val version: Long = 0,
)

@Serializable
data class OdometerReadingCreate(
    val id: String,
    @SerialName("occurred_at") val occurredAt: String,
    @SerialName("time_zone") val timeZone: String,
    val value: QuantityInput,
    @SerialName("time_precision") val timePrecision: String? = null,
    val note: String? = null,
    @SerialName("confirm_anomalies") val confirmAnomalies: List<String>? = null,
    @SerialName("anomaly_reason") val anomalyReason: String? = null,
)

@Serializable
data class OdometerValue(
    val at: String,
    val kind: String,
    @SerialName("meter_value") val meterValue: Quantity? = null,
    val total: Quantity? = null,
    val display: DisplayValue? = null,
    @SerialName("reading_id") val readingId: String? = null,
)

@Serializable
data class OdometerDistance(
    val status: String,
    val distance: Quantity? = null,
    val display: DisplayValue? = null,
)
