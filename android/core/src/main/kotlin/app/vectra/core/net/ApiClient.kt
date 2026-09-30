package app.vectra.core.net

import app.vectra.core.model.Account
import app.vectra.core.model.LoginRequest
import app.vectra.core.model.OdometerDistance
import app.vectra.core.model.OdometerReading
import app.vectra.core.model.OdometerReadingCreate
import app.vectra.core.model.OdometerValue
import app.vectra.core.model.Page
import app.vectra.core.model.Problem
import app.vectra.core.model.Vehicle
import app.vectra.core.model.VectraJson
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.KSerializer
import kotlinx.serialization.serializer
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import java.io.IOException
import java.net.URLEncoder
import java.time.Duration
import java.time.Instant
import java.time.ZonedDateTime
import java.time.format.DateTimeFormatter

/** Antwort mit Status und Rohtext; Grundlage für Outbox und typisierte Aufrufe. */
class RawResponse(val status: Int, val body: String, val etag: String?, val serverDate: Instant?) {
    val isSuccess: Boolean get() = status in 200..299
    fun problem(): Problem? = runCatching { VectraJson.decodeFromString(Problem.serializer(), body) }.getOrNull()
}

/** Fehlerantwort des Servers (4xx/5xx) als Problem Details. */
class ApiException(val status: Int, val problem: Problem?) :
    Exception(problem?.detail ?: problem?.title ?: "HTTP $status")

/**
 * HTTP-Client für die Vectra-API (`/api/v1`). Netzfehler kommen als [IOException],
 * Fehlerantworten als [ApiException].
 */
class ApiClient(
    baseUrl: String,
    val cookies: SessionCookieJar,
    client: OkHttpClient = OkHttpClient(),
) {
    val apiBase: HttpUrl = normalizeBase(baseUrl)
    private val http = client.newBuilder()
        .cookieJar(cookies)
        .callTimeout(Duration.ofSeconds(30))
        .build()

    /** Letzte gemessene Abweichung der Geräteuhr gegenüber dem Server (ADR-021: Warnung ab 2 min). */
    @Volatile var clockSkew: Duration = Duration.ZERO
        private set

    suspend fun raw(method: String, path: String, body: String? = null, headers: Map<String, String> = emptyMap()): RawResponse =
        withContext(Dispatchers.IO) {
            val url = (apiBase.toString().trimEnd('/') + path).toHttpUrl()
            val builder = Request.Builder().url(url).header("Accept", "application/json, application/problem+json")
            headers.forEach { (k, v) -> builder.header(k, v) }
            if (method != "GET" && method != "HEAD") {
                cookies.value(SessionCookieJar.CSRF_COOKIE)?.let { builder.header("X-CSRF-Token", it) }
            }
            val contentType = headers["Content-Type"] ?: "application/json"
            val reqBody = when {
                body != null -> body.toRequestBody(contentType.toMediaType())
                method == "POST" || method == "PATCH" || method == "PUT" -> ByteArray(0).toRequestBody(null)
                else -> null
            }
            builder.method(method, reqBody)
            http.newCall(builder.build()).execute().use { resp ->
                val date = resp.header("Date")?.let { runCatching { ZonedDateTime.parse(it, DateTimeFormatter.RFC_1123_DATE_TIME).toInstant() }.getOrNull() }
                if (date != null) clockSkew = Duration.between(date, Instant.now())
                RawResponse(resp.code, resp.body?.string().orEmpty(), resp.header("ETag"), date)
            }
        }

    private suspend fun <T> call(method: String, path: String, out: KSerializer<T>, body: String? = null, headers: Map<String, String> = emptyMap()): T {
        val r = raw(method, path, body, headers)
        if (!r.isSuccess) throw ApiException(r.status, r.problem())
        return VectraJson.decodeFromString(out, r.body)
    }

    private inline fun <reified T> encode(v: T): String = VectraJson.encodeToString(serializer<T>(), v)

    // --- Identity ---

    suspend fun login(email: String, password: String): Account =
        call("POST", "/auth/login", Account.serializer(), encode(LoginRequest(email, password)))

    suspend fun logout() {
        runCatching { raw("POST", "/auth/logout") }
        cookies.clear()
    }

    suspend fun me(): Account = call("GET", "/me", Account.serializer())

    // --- Vehicles ---

    suspend fun vehicles(): List<Vehicle> =
        call("GET", "/vehicles?limit=100", Page.serializer(Vehicle.serializer())).items

    // --- Odometer ---

    suspend fun currentOdometer(vehicleId: String): OdometerValue =
        call("GET", "/vehicles/$vehicleId/odometer/current", OdometerValue.serializer())

    suspend fun readings(vehicleId: String, limit: Int = 50): List<OdometerReading> =
        call("GET", "/vehicles/$vehicleId/odometer/readings?limit=$limit", Page.serializer(OdometerReading.serializer())).items

    suspend fun distance(vehicleId: String, from: Instant, to: Instant): OdometerDistance =
        call("GET", "/vehicles/$vehicleId/odometer/distance?from=${q(from.toString())}&to=${q(to.toString())}", OdometerDistance.serializer())

    suspend fun createReading(vehicleId: String, body: OdometerReadingCreate): OdometerReading =
        call("POST", "/vehicles/$vehicleId/odometer/readings", OdometerReading.serializer(), encode(body))

    private fun q(s: String) = URLEncoder.encode(s, Charsets.UTF_8)

    companion object {
        /** Akzeptiert „https://host“, „https://host/“ oder „https://host/api/v1“. */
        fun normalizeBase(input: String): HttpUrl {
            var s = input.trim().trimEnd('/')
            if (!s.startsWith("http://") && !s.startsWith("https://")) s = "https://$s"
            if (!s.endsWith("/api/v1")) s += "/api/v1"
            return "$s/".toHttpUrl()
        }
    }
}
