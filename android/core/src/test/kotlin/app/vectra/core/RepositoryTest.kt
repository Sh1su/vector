package app.vectra.core

import app.vectra.core.net.ApiClient
import app.vectra.core.net.InMemoryCookieStorage
import app.vectra.core.net.SessionCookieJar
import app.vectra.core.outbox.InMemoryOutboxStore
import app.vectra.core.outbox.Outbox
import app.vectra.core.outbox.Transport
import app.vectra.core.repo.InMemoryJsonCache
import app.vectra.core.repo.VectraRepository
import kotlinx.coroutines.test.runTest
import okhttp3.mockwebserver.Dispatcher
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import okhttp3.mockwebserver.RecordedRequest
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertFalse
import org.junit.jupiter.api.Assertions.assertNull
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import java.io.IOException
import java.time.Instant
import java.time.YearMonth
import java.time.ZoneId

class RepositoryTest {
    private val vehicles = """{"items":[{"id":"v1","display_name":"Golf","my_role":"owner","usage_meter":"distance"}],"next_cursor":null}"""

    @Test
    fun `Netzfehler faellt auf den Cache zurueck, Serverfehler nicht`() = runTest {
        val server = MockWebServer()
        server.enqueue(MockResponse().setBody(vehicles))
        server.enqueue(MockResponse().setResponseCode(403).setBody("""{"type":"x","title":"Verboten","status":403}"""))
        server.start()
        val cache = InMemoryJsonCache()
        val api = ApiClient(server.url("/").toString(), SessionCookieJar(InMemoryCookieStorage()))
        val repo = VectraRepository(api, cache, Outbox(InMemoryOutboxStore(), Transport(api::raw)))

        assertFalse(repo.vehicles().offline)
        assertThrows<app.vectra.core.net.ApiException> { repo.vehicles() }
        server.shutdown()

        val cached = repo.vehicles()
        assertTrue(cached.offline)
        assertEquals("Golf", cached.value.single().displayName)
    }

    @Test
    fun `ohne Cache wird der Netzfehler weitergegeben`() = runTest {
        val server = MockWebServer()
        server.start()
        server.shutdown()
        val api = ApiClient(server.url("/").toString(), SessionCookieJar(InMemoryCookieStorage()))
        val repo = VectraRepository(api, InMemoryJsonCache(), Outbox(InMemoryOutboxStore(), Transport(api::raw)))
        assertThrows<IOException> { repo.vehicles() }
    }

    @Test
    fun `Monatsstrecken fragen Kalendermonate in der lokalen Zone ab`() = runTest {
        val server = MockWebServer()
        val seen = java.util.concurrent.ConcurrentLinkedQueue<String>()
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse {
                seen += request.requestUrl!!.queryParameter("from")!!
                return if (request.requestUrl!!.queryParameter("from") == "2026-07-31T22:00:00Z")
                    MockResponse().setBody("""{"status":"unknown"}""")
                else MockResponse().setBody("""{"status":"known","distance":{"canonical":1234000,"canonical_unit":"m"}}""")
            }
        }
        server.start()
        val api = ApiClient(server.url("/").toString(), SessionCookieJar(InMemoryCookieStorage()))
        val repo = VectraRepository(api, InMemoryJsonCache(), Outbox(InMemoryOutboxStore(), Transport(api::raw)))
        val m = repo.months("v1", 3, ZoneId.of("Europe/Berlin"), Instant.parse("2026-09-30T10:00:00Z"))
        assertEquals(listOf(YearMonth.of(2026, 7), YearMonth.of(2026, 8), YearMonth.of(2026, 9)), m.map { it.month })
        assertEquals(1234.0, m[0].km)
        assertNull(m[1].km)
        // 1. Juli 00:00 in Berlin = 30. Juni 22:00 UTC
        assertTrue(seen.contains("2026-06-30T22:00:00Z"), seen.toString())
        server.shutdown()
    }
}
