package app.vectra.core.outbox

import app.vectra.core.model.Problem
import app.vectra.core.model.VectraJson
import app.vectra.core.net.RawResponse
import app.vectra.core.util.UuidV7
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.jsonObject
import java.io.IOException

/** Zustände eines Outbox-Eintrags nach ADR-021. */
enum class OutboxStatus { PENDING, SENDING, NEEDS_CONFIRMATION, CONFLICT, FAILED }

/**
 * Eine ausstehende Änderung. [id] ist die client-erzeugte UUIDv7 des Zielobjekts; eine Wiederholung
 * nach Netzabbruch führt deshalb nie zu einem Duplikat (ADR-006).
 */
data class OutboxEntry(
    val id: String,
    val operation: String,
    val vehicleId: String,
    val method: String,
    val path: String,
    val body: String,
    val summary: String,
    val capturedAt: Long,
    val baseVersion: String? = null,
    val idempotencyKey: String? = null,
    val status: OutboxStatus = OutboxStatus.PENDING,
    val attempts: Int = 0,
    val problem: String? = null,
) {
    fun problemOrNull(): Problem? = problem?.let { runCatching { VectraJson.decodeFromString(Problem.serializer(), it) }.getOrNull() }
}

interface OutboxStore {
    suspend fun all(): List<OutboxEntry>
    suspend fun get(id: String): OutboxEntry?
    suspend fun upsert(entry: OutboxEntry)
    suspend fun remove(id: String)
}

class InMemoryOutboxStore : OutboxStore {
    private val map = LinkedHashMap<String, OutboxEntry>()
    override suspend fun all() = synchronized(map) { map.values.toList() }
    override suspend fun get(id: String) = synchronized(map) { map[id] }
    override suspend fun upsert(entry: OutboxEntry) { synchronized(map) { map[entry.id] = entry } }
    override suspend fun remove(id: String) { synchronized(map) { map.remove(id) } }
}

fun interface Transport {
    suspend fun send(method: String, path: String, body: String?, headers: Map<String, String>): RawResponse
}

/** Bewertung einer Serverantwort (Tabelle „Antworten des Servers“ in ADR-021). */
sealed interface Outcome {
    data object Sent : Outcome
    data class NeedsConfirmation(val problem: Problem) : Outcome
    data class Conflict(val problem: Problem?) : Outcome
    data class Failed(val status: Int, val problem: Problem?) : Outcome
    data object Retry : Outcome
    data object Unauthorized : Outcome

    companion object {
        fun classify(r: RawResponse): Outcome {
            val p = r.problem()
            return when {
                r.isSuccess -> Sent
                r.status == 401 -> Unauthorized
                r.status == 422 && p != null && p.confirmableOnly -> NeedsConfirmation(p)
                r.status == 412 -> Conflict(p)
                r.status == 408 || r.status == 429 || r.status >= 500 -> Retry
                else -> Failed(r.status, p)
            }
        }
    }
}

data class SyncReport(
    val sent: List<Pair<OutboxEntry, RawResponse>> = emptyList(),
    val attention: Int = 0,
    val retry: Boolean = false,
    val unauthorized: Boolean = false,
)

/**
 * Sendet ausstehende Änderungen FIFO je Fahrzeug. Ein Eintrag, der Bestätigung braucht oder im
 * Konflikt steht, hält die späteren Einträge desselben Fahrzeugs an, weil die Plausibilität auf der
 * Reihenfolge beruht. Fehlgeschlagene Einträge halten nichts an.
 */
class Outbox(private val store: OutboxStore, private val transport: Transport) {
    private val mutex = Mutex()

    suspend fun enqueue(entry: OutboxEntry) = store.upsert(entry.copy(status = OutboxStatus.PENDING))

    suspend fun entries(): List<OutboxEntry> = store.all().sortedBy { it.capturedAt }

    suspend fun sync(): SyncReport = mutex.withLock {
        val sent = mutableListOf<Pair<OutboxEntry, RawResponse>>()
        var retry = false
        val blocked = mutableSetOf<String>()
        for (stale in store.all().filter { it.status == OutboxStatus.SENDING }) {
            store.upsert(stale.copy(status = OutboxStatus.PENDING))
        }
        for (e in store.all().sortedBy { it.capturedAt }) {
            if (e.vehicleId in blocked) continue
            when (e.status) {
                OutboxStatus.NEEDS_CONFIRMATION, OutboxStatus.CONFLICT -> { blocked += e.vehicleId; continue }
                OutboxStatus.FAILED -> continue
                else -> Unit
            }
            store.upsert(e.copy(status = OutboxStatus.SENDING))
            val headers = buildMap {
                put("Content-Type", "application/json")
                e.baseVersion?.let { put("If-Match", it) }
                e.idempotencyKey?.let { put("Idempotency-Key", it) }
            }
            val response = try {
                transport.send(e.method, e.path, e.body, headers)
            } catch (io: IOException) {
                // Kein Netz: nichts weiter versuchen, WorkManager wiederholt mit Backoff.
                store.upsert(e.copy(status = OutboxStatus.PENDING, attempts = e.attempts + 1))
                return@withLock finish(sent, retry = true)
            }
            when (val o = Outcome.classify(response)) {
                Outcome.Sent -> { store.remove(e.id); sent += e to response }
                is Outcome.NeedsConfirmation -> { store.upsert(e.copy(status = OutboxStatus.NEEDS_CONFIRMATION, problem = response.body)); blocked += e.vehicleId }
                is Outcome.Conflict -> { store.upsert(e.copy(status = OutboxStatus.CONFLICT, problem = response.body)); blocked += e.vehicleId }
                is Outcome.Failed -> store.upsert(e.copy(status = OutboxStatus.FAILED, problem = response.body.ifBlank { null }, attempts = e.attempts + 1))
                Outcome.Retry -> { store.upsert(e.copy(status = OutboxStatus.PENDING, attempts = e.attempts + 1)); blocked += e.vehicleId; retry = true }
                Outcome.Unauthorized -> {
                    store.upsert(e.copy(status = OutboxStatus.PENDING))
                    return@withLock finish(sent, retry = false, unauthorized = true)
                }
            }
        }
        finish(sent, retry)
    }

    private suspend fun finish(sent: List<Pair<OutboxEntry, RawResponse>>, retry: Boolean, unauthorized: Boolean = false) =
        SyncReport(sent, attention = store.all().count { it.status == OutboxStatus.NEEDS_CONFIRMATION || it.status == OutboxStatus.CONFLICT || it.status == OutboxStatus.FAILED }, retry = retry, unauthorized = unauthorized)

    /** Befunde bestätigen (ADR-010): Codes und Begründung in den Body schreiben und erneut einreihen. */
    suspend fun confirm(id: String, codes: List<String>, reason: String) = mutex.withLock {
        require(reason.isNotBlank()) { "Begründung fehlt" }
        val e = store.get(id) ?: return@withLock
        check(e.status == OutboxStatus.NEEDS_CONFIRMATION) { "Eintrag braucht keine Bestätigung" }
        val obj = VectraJson.parseToJsonElement(e.body).jsonObject.toMutableMap()
        obj["confirm_anomalies"] = JsonArray(codes.map { JsonPrimitive(it) })
        obj["anomaly_reason"] = JsonPrimitive(reason.trim())
        store.upsert(e.copy(body = JsonObject(obj).toString(), status = OutboxStatus.PENDING, problem = null))
    }

    /**
     * Wert korrigieren (Konfliktdialog: „Der Wert war falsch“). Der Eintrag bekommt eine neue ID – der
     * Server kennt die alte eventuell schon mit anderem Inhalt –, behält aber seinen Platz in der
     * Reihenfolge; Bestätigungen fallen weg, weil der neue Wert neu geprüft wird.
     */
    suspend fun amendValue(id: String, value: Double, summary: String, newId: String = UuidV7.generate()) = mutex.withLock {
        val e = store.get(id) ?: return@withLock null
        check(e.status != OutboxStatus.SENDING) { "Eintrag wird gerade gesendet" }
        val obj = VectraJson.parseToJsonElement(e.body).jsonObject.toMutableMap()
        val v = obj["value"]?.jsonObject?.toMutableMap() ?: error("Eintrag hat keinen Wert")
        v["value"] = JsonPrimitive(value)
        obj["value"] = JsonObject(v)
        obj["id"] = JsonPrimitive(newId)
        obj.remove("confirm_anomalies")
        obj.remove("anomaly_reason")
        store.remove(id)
        val n = e.copy(id = newId, body = JsonObject(obj).toString(), summary = summary, status = OutboxStatus.PENDING, problem = null, attempts = 0)
        store.upsert(n)
        n
    }

    /** Eintrag verwerfen, z. B. „Server übernehmen“ im Konfliktdialog oder ein fehlgeschlagener Eintrag. */
    suspend fun discard(id: String) = mutex.withLock { store.remove(id) }

    /** Fehlgeschlagenen Eintrag erneut versuchen, etwa nachdem eine Freigabe wieder erteilt wurde. */
    suspend fun retry(id: String) = mutex.withLock {
        store.get(id)?.let { store.upsert(it.copy(status = OutboxStatus.PENDING, problem = null)) }
    }
}
