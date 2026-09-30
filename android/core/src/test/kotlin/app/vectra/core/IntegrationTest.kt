package app.vectra.core

import app.vectra.core.model.VectraJson
import app.vectra.core.net.ApiClient
import app.vectra.core.net.InMemoryCookieStorage
import app.vectra.core.net.SessionCookieJar
import app.vectra.core.odometer.ReadingDraft
import app.vectra.core.outbox.InMemoryOutboxStore
import app.vectra.core.outbox.Outbox
import app.vectra.core.outbox.OutboxStatus
import app.vectra.core.outbox.Transport
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.condition.EnabledIfEnvironmentVariable
import java.time.Instant

/**
 * Gegen ein laufendes Backend (VECTRA_IT_URL, VECTRA_IT_EMAIL, VECTRA_IT_PASSWORD).
 * Legt ein Testfahrzeug an und löscht es am Ende wieder.
 */
@EnabledIfEnvironmentVariable(named = "VECTRA_IT_URL", matches = ".+")
class IntegrationTest {
    @Test
    fun `Outbox gegen den Server - anlegen, Wiederholung, Bestaetigung`() = runTest {
        val api = ApiClient(System.getenv("VECTRA_IT_URL"), SessionCookieJar(InMemoryCookieStorage()))
        api.login(System.getenv("VECTRA_IT_EMAIL") ?: "sam@example.org", System.getenv("VECTRA_IT_PASSWORD") ?: "Sicheres-Passwort-2026")

        val created = api.raw("POST", "/vehicles", """{"display_name":"Android-IT ${System.nanoTime()}","body_type":"car","usage_meter":"distance","energy_carriers":["petrol"]}""",
            mapOf("Content-Type" to "application/json"))
        assertEquals(201, created.status, created.body)
        val vid = VectraJson.parseToJsonElement(created.body).jsonObject["id"]!!.jsonPrimitive.content
        try {
            val outbox = Outbox(InMemoryOutboxStore(), Transport(api::raw))
            val t0 = Instant.now().minusSeconds(7200)
            val first = ReadingDraft.toOutbox(vid, 1000.0, "km", t0, capturedAt = 1)
            outbox.enqueue(first)
            outbox.enqueue(ReadingDraft.toOutbox(vid, 900.0, "km", t0.plusSeconds(3600), capturedAt = 2))
            val r1 = outbox.sync()
            assertEquals(1, r1.sent.size)
            val pending = outbox.entries().single()
            assertEquals(OutboxStatus.NEEDS_CONFIRMATION, pending.status)
            assertEquals("P1", pending.problemOrNull()!!.anomalies.single().code)

            // Wiederholung des ersten Eintrags (z. B. nach verlorener Antwort) ergibt kein Duplikat.
            outbox.enqueue(first.copy(capturedAt = 0))
            outbox.confirm(pending.id, listOf("P1"), "Tachotausch")
            val r2 = outbox.sync()
            assertEquals(listOf(200, 201), r2.sent.map { it.second.status })
            assertTrue(outbox.entries().isEmpty())

            val readings = api.readings(vid)
            assertEquals(2, readings.size)
            assertEquals("confirmed_anomaly", readings.first { it.meterValue.canonical == 900_000L }.status)
            assertEquals(900_000L, api.currentOdometer(vid).meterValue!!.canonical)
        } finally {
            val v = api.raw("GET", "/vehicles/$vid")
            api.raw("DELETE", "/vehicles/$vid", headers = mapOf("If-Match" to (v.etag ?: "*")))
        }
    }
}
