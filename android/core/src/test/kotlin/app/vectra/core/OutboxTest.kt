package app.vectra.core

import app.vectra.core.net.RawResponse
import app.vectra.core.odometer.ReadingDraft
import app.vectra.core.outbox.InMemoryOutboxStore
import app.vectra.core.outbox.Outbox
import app.vectra.core.outbox.OutboxStatus
import app.vectra.core.outbox.Transport
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertFalse
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test
import java.io.IOException
import java.time.Instant

class OutboxTest {
    private val p1 = """{"type":"https://vectra.app/problems/plausibility","title":"Plausibilität","status":422,
        |"anomalies":[{"code":"P1","confirmable":true,"message":"kleiner als vorher"}]}""".trimMargin()
    private val future = """{"type":"x","title":"t","status":422,"anomalies":[{"code":"P4","confirmable":false}]}"""

    /** Transport, der je Aufruf die nächste Antwort liefert und die Aufrufe mitschreibt. */
    private class Scripted(vararg responses: () -> RawResponse) : Transport {
        val queue = ArrayDeque(responses.toList())
        val calls = mutableListOf<Triple<String, String?, Map<String, String>>>()
        override suspend fun send(method: String, path: String, body: String?, headers: Map<String, String>): RawResponse {
            calls += Triple(path, body, headers)
            return queue.removeFirst()()
        }
    }

    private fun ok() = { RawResponse(201, "{}", null, null) }
    private fun resp(status: Int, body: String) = { RawResponse(status, body, null, null) }
    private fun entry(vid: String, v: Double, at: Long) =
        ReadingDraft.toOutbox(vid, v, "km", Instant.parse("2026-09-29T06:00:00Z"), capturedAt = at)

    @Test
    fun `2xx entfernt den Eintrag`() = runTest {
        val store = InMemoryOutboxStore()
        val t = Scripted(ok())
        val o = Outbox(store, t)
        o.enqueue(entry("v1", 100.0, 1))
        val r = o.sync()
        assertEquals(1, r.sent.size)
        assertTrue(o.entries().isEmpty())
        assertEquals("application/json", t.calls[0].third["Content-Type"])
    }

    @Test
    fun `bestaetigbare 422 haelt das Fahrzeug an, andere Fahrzeuge laufen weiter`() = runTest {
        val o = Outbox(InMemoryOutboxStore(), Scripted(resp(422, p1), ok()))
        o.enqueue(entry("v1", 90.0, 1))
        o.enqueue(entry("v1", 95.0, 2))
        o.enqueue(entry("v2", 10.0, 3))
        val r = o.sync()
        assertEquals(1, r.sent.size)
        assertEquals(1, r.attention)
        val left = o.entries()
        assertEquals(listOf(OutboxStatus.NEEDS_CONFIRMATION, OutboxStatus.PENDING), left.map { it.status })
        assertEquals("P1", left[0].problemOrNull()!!.anomalies.single().code)
    }

    @Test
    fun `Bestaetigung schreibt Codes und Begruendung und sendet erneut`() = runTest {
        val t = Scripted(resp(422, p1), ok())
        val o = Outbox(InMemoryOutboxStore(), t)
        val e = entry("v1", 90.0, 1)
        o.enqueue(e)
        o.sync()
        o.confirm(e.id, listOf("P1"), "Tacho getauscht")
        val r = o.sync()
        assertEquals(1, r.sent.size)
        val body = Json.parseToJsonElement(t.calls[1].second!!).jsonObject
        assertEquals("P1", body["confirm_anomalies"]!!.jsonArray[0].jsonPrimitive.content)
        assertEquals("Tacho getauscht", body["anomaly_reason"]!!.jsonPrimitive.content)
        assertEquals(e.id, body["id"]!!.jsonPrimitive.content)
    }

    @Test
    fun `nicht bestaetigbare 422 und 403 sind fehlgeschlagen und halten nichts an`() = runTest {
        val o = Outbox(InMemoryOutboxStore(), Scripted(resp(422, future), resp(403, """{"type":"x","title":"t","status":403}"""), ok()))
        o.enqueue(entry("v1", 1.0, 1))
        o.enqueue(entry("v1", 2.0, 2))
        o.enqueue(entry("v1", 3.0, 3))
        val r = o.sync()
        assertEquals(1, r.sent.size)
        assertEquals(listOf(OutboxStatus.FAILED, OutboxStatus.FAILED), o.entries().map { it.status })
    }

    @Test
    fun `412 wird Konflikt`() = runTest {
        val o = Outbox(InMemoryOutboxStore(), Scripted(resp(412, """{"type":"x","title":"t","status":412,"current":{"version":3}}""")))
        o.enqueue(entry("v1", 1.0, 1).copy(baseVersion = "\"2\""))
        o.sync()
        val e = o.entries().single()
        assertEquals(OutboxStatus.CONFLICT, e.status)
        assertTrue(e.problemOrNull()!!.current != null)
    }

    @Test
    fun `Netzfehler und 5xx fuehren zur Wiederholung, der Eintrag bleibt`() = runTest {
        val t = Scripted({ throw IOException("offline") }, resp(503, ""), ok())
        val o = Outbox(InMemoryOutboxStore(), t)
        o.enqueue(entry("v1", 1.0, 1))
        assertTrue(o.sync().retry)
        assertTrue(o.sync().retry)
        val e = o.entries().single()
        assertEquals(OutboxStatus.PENDING, e.status)
        assertEquals(2, e.attempts)
        val r = o.sync()
        assertFalse(r.retry)
        assertEquals(1, r.sent.size)
        // Alle drei Versuche mit derselben Client-ID: der Server erkennt Wiederholungen (ADR-006).
        assertEquals(1, t.calls.map { Json.parseToJsonElement(it.second!!).jsonObject["id"] }.toSet().size)
    }

    @Test
    fun `401 bricht ab, ohne den Eintrag zu veraendern`() = runTest {
        val o = Outbox(InMemoryOutboxStore(), Scripted(resp(401, "")))
        o.enqueue(entry("v1", 1.0, 1))
        val r = o.sync()
        assertTrue(r.unauthorized)
        assertEquals(OutboxStatus.PENDING, o.entries().single().status)
    }

    @Test
    fun `Entwurf entspricht OdometerReadingCreate`() {
        val e = ReadingDraft.toOutbox("v1", 142950.0, "km", Instant.parse("2026-09-29T06:00:00.123Z"), java.time.ZoneId.of("Europe/Berlin"), note = " ")
        val body = Json.parseToJsonElement(e.body).jsonObject
        assertEquals("2026-09-29T06:00:00Z", body["occurred_at"]!!.jsonPrimitive.content)
        assertEquals("Europe/Berlin", body["time_zone"]!!.jsonPrimitive.content)
        assertEquals("km", body["value"]!!.jsonObject["unit"]!!.jsonPrimitive.content)
        assertFalse(body.containsKey("note"))
        assertFalse(body.containsKey("confirm_anomalies"))
        assertEquals("Kilometerstand 142.950 km", e.summary)
    }

    @Test
    fun `Wert korrigieren sendet mit neuer ID und ohne Bestaetigung`() = runTest {
        val t = Scripted(resp(422, p1), ok())
        val o = Outbox(InMemoryOutboxStore(), t)
        val e = entry("v1", 90.0, 1)
        o.enqueue(e)
        o.sync()
        assertEquals(OutboxStatus.NEEDS_CONFIRMATION, o.entries().single().status)
        val n = o.amendValue(e.id, 190.0, "Kilometerstand 190 km")!!
        val pending = o.entries().single()
        assertEquals(n.id, pending.id)
        assertTrue(pending.id != e.id)
        assertEquals(1L, pending.capturedAt)
        val body = Json.parseToJsonElement(pending.body).jsonObject
        assertEquals("190.0", body["value"]!!.jsonObject["value"]!!.jsonPrimitive.content)
        assertEquals(n.id, body["id"]!!.jsonPrimitive.content)
        assertFalse(body.containsKey("confirm_anomalies"))
        o.sync()
        assertTrue(o.entries().isEmpty())
        assertTrue(t.calls[1].second!!.contains(n.id))
    }
}
