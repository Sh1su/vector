package app.vectra.core

import app.vectra.core.net.ApiClient
import app.vectra.core.net.ApiException
import app.vectra.core.net.InMemoryCookieStorage
import app.vectra.core.net.SessionCookieJar
import kotlinx.coroutines.test.runTest
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertNull
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows

class ApiClientTest {
    private lateinit var server: MockWebServer
    private lateinit var api: ApiClient

    @BeforeEach
    fun setUp() {
        server = MockWebServer()
        server.start()
        api = ApiClient(server.url("/").toString(), SessionCookieJar(InMemoryCookieStorage()))
    }

    @AfterEach
    fun tearDown() = server.shutdown()

    @Test
    fun `Basis-URL wird normalisiert`() {
        assertEquals("https://vectra.example/api/v1/", ApiClient.normalizeBase("vectra.example/").toString())
        assertEquals("http://10.0.2.2:8080/api/v1/", ApiClient.normalizeBase("http://10.0.2.2:8080/api/v1").toString())
    }

    @Test
    fun `Login speichert Cookies, danach sendet der Client das CSRF-Token`() = runTest {
        server.enqueue(MockResponse().setBody("""{"id":"a","email":"sam@example.org","display_name":"Sam","extra":1}""")
            .addHeader("Set-Cookie", "vectra_session=s1; Path=/; HttpOnly")
            .addHeader("Set-Cookie", "vectra_csrf=c1; Path=/"))
        server.enqueue(MockResponse().setResponseCode(201).setBody(
            """{"id":"r","vehicle_id":"v","occurred_at":"2026-09-29T06:00:00Z","time_zone":"Europe/Berlin",
               "meter_value":{"canonical":1000,"canonical_unit":"m"},"total":{"canonical":1000,"canonical_unit":"m"}}"""))
        val me = api.login("sam@example.org", "pw")
        assertEquals("Sam", me.displayName)
        assertTrue(api.cookies.hasSession)
        val login = server.takeRequest()
        assertEquals("/api/v1/auth/login", login.path)
        assertNull(login.getHeader("X-CSRF-Token"))
        assertTrue(login.body.readUtf8().contains("\"client_kind\":\"android\""))

        api.createReading("v", app.vectra.core.model.OdometerReadingCreate("r", "2026-09-29T06:00:00Z", "Europe/Berlin", app.vectra.core.model.QuantityInput(1.0, "km")))
        val post = server.takeRequest()
        assertEquals("c1", post.getHeader("X-CSRF-Token"))
        assertTrue(post.getHeader("Cookie")!!.contains("vectra_session=s1"))
        assertTrue(!post.body.readUtf8().contains("null"))
    }

    @Test
    fun `Fehler kommen als Problem Details`() = runTest {
        server.enqueue(MockResponse().setResponseCode(404).addHeader("Content-Type", "application/problem+json")
            .setBody("""{"type":"https://vectra.app/problems/not-found","title":"Nicht gefunden","status":404,"detail":"Fahrzeug nicht gefunden."}"""))
        val e = assertThrows<ApiException> { api.currentOdometer("x") }
        assertEquals(404, e.status)
        assertEquals("Fahrzeug nicht gefunden.", e.message)
    }

    @Test
    fun `Tachofoto hochladen, auswerten und Fahrt mit Foto starten`() = runTest {
        server.enqueue(MockResponse().setResponseCode(201).setBody(
            """{"id":"f1","original_name":"tacho.jpg","media_type":"image/jpeg","size_bytes":3,"received_at":"2026-09-21T06:00:00Z"}"""))
        server.enqueue(MockResponse().setBody(
            """{"file_id":"f1","readable":true,"odometer":{"value":98312,"unit":"km"},"fuel_level_percent":75,"warning_lights":[],
               "confidence":"high","notes":"","captured_at":"2026-09-21T05:58:00Z","last_odometer":{"value":98000,"unit":"km"},
               "summary":"Tank 75 %","unknown_field":1}"""))
        server.enqueue(MockResponse().setResponseCode(201).setBody(
            """{"id":"t1","version":1,"started_at":"2026-09-21T05:58:00Z","time_zone":"Europe/Berlin","start_odometer":{"value":98312,"unit":"km"},
               "category_id":"c","status":"open","start_photo_id":"f1"}"""))
        val meta = api.upload("v", "tacho.jpg", "image/jpeg", byteArrayOf(1, 2, 3), "camera", java.time.Instant.parse("2026-09-21T05:58:00Z"))
        val up = server.takeRequest().body.readUtf8()
        assertTrue(up.contains("captured_at_client") && up.contains("2026-09-21T05:58:00Z") && up.contains("camera"))
        val r = api.readDashboard("v", meta.id)
        assertEquals("/api/v1/vehicles/v/files/f1/dashboard-reading", server.takeRequest().path)
        assertEquals(98312.0, r.odometer?.value)
        assertEquals(75.0, r.fuelLevelPercent)
        val t = api.startTrip("v", app.vectra.core.model.TripStart("t1", r.capturedAt!!, "Europe/Berlin", r.odometer!!, "c", startPhotoId = meta.id, note = "Start: " + r.summary))
        val body = server.takeRequest().body.readUtf8()
        assertTrue(body.contains("\"start_photo_id\":\"f1\"") && body.contains("Start: Tank 75 %"))
        assertEquals("f1", t.startPhotoId)
    }

    @Test
    fun `Uhrabweichung aus dem Date-Header`() = runTest {
        server.enqueue(MockResponse().setBody("{}").setHeader("Date", "Mon, 01 Jan 2024 00:00:00 GMT"))
        api.raw("GET", "/health")
        assertTrue(api.clockSkew.toDays() > 365)
    }

    @Test
    fun `Nach einem Neustart ist die gespeicherte Sitzung sofort bekannt`() = runTest {
        val storage = InMemoryCookieStorage()
        val first = SessionCookieJar(storage)
        val a = ApiClient(server.url("/").toString(), first)
        server.enqueue(MockResponse().setBody("""{"id":"a","email":"sam@example.org","display_name":"Sam"}""")
            .addHeader("Set-Cookie", "vectra_session=s1; Path=/; Max-Age=31536000; HttpOnly")
            .addHeader("Set-Cookie", "vectra_csrf=c1; Path=/; Max-Age=31536000"))
        a.login("sam@example.org", "pw")
        // Neuer Prozess: neue Jar auf demselben Speicher, noch keine Anfrage
        val restarted = SessionCookieJar(storage) { server.url("/").toString() }
        assertTrue(restarted.hasSession)
        assertEquals("c1", restarted.value(SessionCookieJar.CSRF_COOKIE))
        // Ohne Server-Adresse bleibt sie unbekannt, bis eine Anfrage sie lädt
        assertTrue(!SessionCookieJar(storage).hasSession)
    }

    @Test
    fun `Assistent-Antwort als Server-Sent Events`() = runTest {
        server.enqueue(MockResponse().setHeader("Content-Type", "text/event-stream").setBody(
            "event: status\ndata: {\"text\":\"Lese Fahrzeuge …\"}\n\n" +
                "event: proposal\ndata: {\"id\":\"p1\",\"operation\":\"startTrip\",\"vehicle_id\":\"v1\",\"summary\":\"Fahrt starten\",\"body\":{},\"status\":\"pending\",\"expires_at\":\"2026-10-01T12:00:00Z\"}\n\n" +
                "event: message\ndata: {\"id\":\"m1\",\"role\":\"assistant\",\"text\":\"Bitte bestätigen.\",\"proposals\":[]}\n\n"))
        val status = mutableListOf<String>()
        val proposals = mutableListOf<String>()
        val m = api.sendAssistantMessage("c1", "Ich fahre los", { status += it }, { proposals += it.id })
        assertEquals("Bitte bestätigen.", m.text)
        assertEquals(listOf("Lese Fahrzeuge …"), status)
        assertEquals(listOf("p1"), proposals)
        assertEquals("/api/v1/assistant/conversations/c1/messages", server.takeRequest().path)

        server.enqueue(MockResponse().setHeader("Content-Type", "text/event-stream").setBody(
            "event: error\ndata: {\"type\":\"x\",\"title\":\"KI-Anbieter\",\"status\":502,\"detail\":\"nicht erreichbar\"}\n\n"))
        val e = assertThrows<ApiException> { api.sendAssistantMessage("c1", "x", {}, {}) }
        assertEquals(502, e.status)
    }
}
