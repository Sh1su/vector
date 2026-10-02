package app.vectra.core.repo

import app.vectra.core.model.CostEntry
import app.vectra.core.model.CostOccurrenceList
import app.vectra.core.model.CostReport
import app.vectra.core.model.DocumentMeta
import app.vectra.core.model.DueStatus
import app.vectra.core.model.FileMeta
import app.vectra.core.model.OdometerReading
import app.vectra.core.model.ServiceEntry
import app.vectra.core.model.Trip
import app.vectra.core.model.TripCategory
import app.vectra.core.model.TripReport
import app.vectra.core.model.VehicleImage
import app.vectra.core.model.OdometerValue
import app.vectra.core.model.Vehicle
import app.vectra.core.model.VectraJson
import app.vectra.core.net.ApiClient
import app.vectra.core.odometer.ReadingDraft
import app.vectra.core.outbox.Outbox
import app.vectra.core.outbox.OutboxEntry
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.coroutineScope
import kotlinx.serialization.KSerializer
import kotlinx.serialization.builtins.ListSerializer
import java.io.IOException
import java.time.Instant
import java.time.YearMonth
import java.time.ZoneId

/** Lesecache (ADR-021): JSON je Schlüssel mit Zeitpunkt des Abrufs. */
interface JsonCache {
    suspend fun get(key: String): Pair<String, Long>?
    suspend fun put(key: String, json: String)
    suspend fun clear()
}

class InMemoryJsonCache : JsonCache {
    private val map = HashMap<String, Pair<String, Long>>()
    override suspend fun get(key: String) = synchronized(map) { map[key] }
    override suspend fun put(key: String, json: String) { synchronized(map) { map[key] = json to System.currentTimeMillis() } }
    override suspend fun clear() { synchronized(map) { map.clear() } }
}

/** Ergebnis mit Herkunft: frisch vom Server oder aus dem Cache, weil kein Netz da war. */
data class Loaded<T>(val value: T, val offline: Boolean = false, val fetchedAt: Long? = null)

data class MonthDistance(val month: YearMonth, val km: Double?)

/**
 * Lesen mit Cache-Rückfall bei Netzfehlern, Schreiben immer über die Outbox. Fehlerantworten des
 * Servers (ApiException) werden nicht verschluckt; nur Netzfehler fallen auf den Cache zurück.
 */
class VectraRepository(val api: ApiClient, private val cache: JsonCache, val outbox: Outbox) {

    private suspend fun <T> cached(key: String, serializer: KSerializer<T>, fetch: suspend () -> T): Loaded<T> =
        try {
            val v = fetch()
            cache.put(key, VectraJson.encodeToString(serializer, v))
            Loaded(v)
        } catch (e: IOException) {
            val hit = cache.get(key) ?: throw e
            Loaded(VectraJson.decodeFromString(serializer, hit.first), offline = true, fetchedAt = hit.second)
        }

    suspend fun vehicles(): Loaded<List<Vehicle>> =
        cached("vehicles", ListSerializer(Vehicle.serializer())) { api.vehicles() }

    suspend fun current(vehicleId: String): Loaded<OdometerValue> =
        cached("odo:$vehicleId:current", OdometerValue.serializer()) { api.currentOdometer(vehicleId) }

    suspend fun readings(vehicleId: String, limit: Int = 50): Loaded<List<OdometerReading>> =
        cached("odo:$vehicleId:readings", ListSerializer(OdometerReading.serializer())) { api.readings(vehicleId, limit) }

    /** Gefahrene Strecke je Kalendermonat (ODO-05), die letzten [n] Monate einschließlich des laufenden. */
    suspend fun months(vehicleId: String, n: Int = 6, zone: ZoneId = ZoneId.systemDefault(), now: Instant = Instant.now()): List<MonthDistance> = coroutineScope {
        val thisMonth = YearMonth.from(now.atZone(zone))
        (n - 1 downTo 0).map { back ->
            val m = thisMonth.minusMonths(back.toLong())
            async {
                val from = m.atDay(1).atStartOfDay(zone).toInstant()
                val to = minOf(m.plusMonths(1).atDay(1).atStartOfDay(zone).toInstant(), now)
                val d = runCatching { api.distance(vehicleId, from, to) }.getOrNull()
                MonthDistance(m, if (d?.status == "known") d.distance?.canonical?.let { it / 1000.0 } else null)
            }
        }.awaitAll()
    }

    /** Stand erfassen: landet in der Outbox, der Sync überträgt ihn (auch offline). */
    suspend fun addReading(vehicle: Vehicle, value: Double, note: String?, at: Instant = Instant.now()): OutboxEntry {
        val entry = ReadingDraft.toOutbox(vehicle.id, value, vehicle.meterUnit, at, note = note)
        outbox.enqueue(entry)
        return entry
    }

    // --- Wartung, Service, Kosten, Fahrten, Dokumente: lesen mit Cache-Rückfall ---

    suspend fun due(vehicleId: String): Loaded<List<DueStatus>> =
        cached("due:$vehicleId", ListSerializer(DueStatus.serializer())) { api.dueStatus(vehicleId) }

    suspend fun services(vehicleId: String): Loaded<List<ServiceEntry>> =
        cached("service:$vehicleId", ListSerializer(ServiceEntry.serializer())) { api.serviceEntries(vehicleId) }

    suspend fun costReport(vehicleId: String, from: String, to: String): Loaded<CostReport> =
        cached("costs:$vehicleId:$from:$to", CostReport.serializer()) { api.costReport(vehicleId, from, to) }

    suspend fun occurrences(vehicleId: String): Loaded<CostOccurrenceList> =
        cached("occ:$vehicleId", CostOccurrenceList.serializer()) { api.costOccurrences(vehicleId) }

    suspend fun costEntries(vehicleId: String): Loaded<List<CostEntry>> =
        cached("costentries:$vehicleId", ListSerializer(CostEntry.serializer())) { api.costEntries(vehicleId) }

    suspend fun trips(vehicleId: String): Loaded<List<Trip>> =
        cached("trips:$vehicleId", ListSerializer(Trip.serializer())) { api.trips(vehicleId) }

    suspend fun tripCategories(vehicleId: String): Loaded<List<TripCategory>> =
        cached("tripcats:$vehicleId", ListSerializer(TripCategory.serializer())) { api.tripCategories(vehicleId) }

    suspend fun tripReport(vehicleId: String, from: String, to: String): Loaded<TripReport> =
        cached("tripreport:$vehicleId:$from", TripReport.serializer()) { api.tripReport(vehicleId, from, to) }

    suspend fun documents(vehicleId: String): Loaded<List<DocumentMeta>> =
        cached("docs:$vehicleId", ListSerializer(DocumentMeta.serializer())) { api.documents(vehicleId) }

    suspend fun files(vehicleId: String): Loaded<List<FileMeta>> =
        cached("files:$vehicleId", ListSerializer(FileMeta.serializer())) { api.files(vehicleId) }

    suspend fun images(vehicleId: String): Loaded<List<VehicleImage>> =
        cached("images:$vehicleId", ListSerializer(VehicleImage.serializer())) { api.vehicleImages(vehicleId) }

    suspend fun clear() = cache.clear()
}
