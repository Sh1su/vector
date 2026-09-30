package app.vectra.core.odometer

import app.vectra.core.model.OdometerReadingCreate
import app.vectra.core.model.QuantityInput
import app.vectra.core.model.VectraJson
import app.vectra.core.outbox.OutboxEntry
import app.vectra.core.util.Format
import app.vectra.core.util.UuidV7
import java.time.Instant
import java.time.ZoneId
import java.time.temporal.ChronoUnit

/** Baut aus einer Erfassung auf dem Gerät den Outbox-Eintrag für `createOdometerReading`. */
object ReadingDraft {
    fun toOutbox(
        vehicleId: String,
        value: Double,
        unit: String,
        occurredAt: Instant = Instant.now(),
        zone: ZoneId = ZoneId.systemDefault(),
        note: String? = null,
        id: String = UuidV7.generate(),
        capturedAt: Long = System.currentTimeMillis(),
    ): OutboxEntry {
        val body = OdometerReadingCreate(
            id = id,
            occurredAt = occurredAt.truncatedTo(ChronoUnit.SECONDS).toString(),
            timeZone = zone.id,
            value = QuantityInput(value, unit),
            timePrecision = "exact",
            note = note?.takeIf { it.isNotBlank() },
        )
        return OutboxEntry(
            id = id,
            operation = "createOdometerReading",
            vehicleId = vehicleId,
            method = "POST",
            path = "/vehicles/$vehicleId/odometer/readings",
            body = VectraJson.encodeToString(OdometerReadingCreate.serializer(), body),
            summary = "Kilometerstand ${Format.number(value, 1)} $unit",
            capturedAt = capturedAt,
        )
    }
}
