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
    fun `Uhrabweichung aus dem Date-Header`() = runTest {
        server.enqueue(MockResponse().setBody("{}").setHeader("Date", "Mon, 01 Jan 2024 00:00:00 GMT"))
        api.raw("GET", "/health")
        assertTrue(api.clockSkew.toDays() > 365)
    }
}
