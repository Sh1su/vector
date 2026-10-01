package app.vectra.core.net

import app.vectra.core.model.Account
import app.vectra.core.model.CompletionCreate
import app.vectra.core.model.CostEntry
import app.vectra.core.model.CostEntryCreate
import app.vectra.core.model.CostOccurrenceList
import app.vectra.core.model.CostReport
import app.vectra.core.model.DocumentCreate
import app.vectra.core.model.DocumentMeta
import app.vectra.core.model.DueStatus
import app.vectra.core.model.FileMeta
import app.vectra.core.model.ServiceEntry
import app.vectra.core.model.ServiceEntryCreate
import app.vectra.core.model.Trip
import app.vectra.core.model.TripCategory
import app.vectra.core.model.TripFinish
import app.vectra.core.model.TripReport
import app.vectra.core.model.TripStart
import app.vectra.core.model.VehicleImage
import app.vectra.core.model.VehicleImageCreate
import app.vectra.core.model.LoginRequest
import app.vectra.core.model.MaintenanceBook
import app.vectra.core.model.MaintenanceBookApply
import app.vectra.core.model.MaintenanceBookApplyResult
import app.vectra.core.model.MaintenanceBookPage
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
import okhttp3.MediaType.Companion.toMediaTypeOrNull
import okhttp3.MultipartBody
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

    suspend fun login(email: String, password: String, clientKind: String = "android"): Account =
        call("POST", "/auth/login", Account.serializer(), encode(LoginRequest(email, password, clientKind)))

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

    // --- Wartung ---

    suspend fun dueStatus(vehicleId: String): List<DueStatus> =
        call("GET", "/vehicles/$vehicleId/maintenance/status", Page.serializer(DueStatus.serializer())).items

    suspend fun complete(vehicleId: String, itemId: String, body: CompletionCreate, idempotencyKey: String): Unit {
        val r = raw("POST", "/vehicles/$vehicleId/maintenance-items/$itemId/completions", encode(body), mapOf("Idempotency-Key" to idempotencyKey))
        if (!r.isSuccess) throw ApiException(r.status, r.problem())
    }

    suspend fun maintenanceBooks(): List<MaintenanceBook> =
        call("GET", "/maintenance-books", MaintenanceBookPage.serializer()).items

    suspend fun applyMaintenanceBook(vehicleId: String, bookId: String, body: MaintenanceBookApply): MaintenanceBookApplyResult =
        call("POST", "/vehicles/$vehicleId/maintenance-books/$bookId/apply", MaintenanceBookApplyResult.serializer(), encode(body))

    // --- Service ---

    suspend fun serviceEntries(vehicleId: String, limit: Int = 50): List<ServiceEntry> =
        call("GET", "/vehicles/$vehicleId/service-entries?limit=$limit", Page.serializer(ServiceEntry.serializer())).items

    suspend fun createServiceEntry(vehicleId: String, body: ServiceEntryCreate): ServiceEntry =
        call("POST", "/vehicles/$vehicleId/service-entries", ServiceEntry.serializer(), encode(body))

    // --- Kosten ---

    suspend fun costReport(vehicleId: String, from: String, to: String): CostReport =
        call("GET", "/vehicles/$vehicleId/cost-report?from=$from&to=$to&group_by=category", CostReport.serializer())

    suspend fun costOccurrences(vehicleId: String): CostOccurrenceList =
        call("GET", "/vehicles/$vehicleId/cost-occurrences", CostOccurrenceList.serializer())

    suspend fun confirmOccurrence(vehicleId: String, planId: String, dueOn: String): CostEntry =
        call("POST", "/vehicles/$vehicleId/cost-plans/$planId/occurrences/$dueOn/confirm", CostEntry.serializer(), "{}")

    suspend fun costEntries(vehicleId: String, limit: Int = 30): List<CostEntry> =
        call("GET", "/vehicles/$vehicleId/cost-entries?limit=$limit", Page.serializer(CostEntry.serializer())).items

    suspend fun createCostEntry(vehicleId: String, body: CostEntryCreate): CostEntry =
        call("POST", "/vehicles/$vehicleId/cost-entries", CostEntry.serializer(), encode(body))

    // --- Fahrten ---

    suspend fun trips(vehicleId: String, limit: Int = 50): List<Trip> =
        call("GET", "/vehicles/$vehicleId/trips?limit=$limit", Page.serializer(Trip.serializer())).items

    suspend fun tripCategories(vehicleId: String): List<TripCategory> =
        call("GET", "/vehicles/$vehicleId/trip-categories", Page.serializer(TripCategory.serializer())).items

    suspend fun tripReport(vehicleId: String, from: String, to: String): TripReport =
        call("GET", "/vehicles/$vehicleId/trip-report?from=$from&to=$to", TripReport.serializer())

    suspend fun startTrip(vehicleId: String, body: TripStart): Trip =
        call("POST", "/vehicles/$vehicleId/trips/start", Trip.serializer(), encode(body))

    suspend fun finishTrip(vehicleId: String, trip: Trip, body: TripFinish): Trip =
        call("POST", "/vehicles/$vehicleId/trips/${trip.id}/finish", Trip.serializer(), encode(body), mapOf("If-Match" to "\"${trip.version}\""))

    // --- Dokumente, Dateien, Fahrzeugbilder ---

    suspend fun documents(vehicleId: String): List<DocumentMeta> =
        call("GET", "/vehicles/$vehicleId/documents?limit=200", Page.serializer(DocumentMeta.serializer())).items

    suspend fun files(vehicleId: String): List<FileMeta> =
        call("GET", "/vehicles/$vehicleId/files?limit=200", Page.serializer(FileMeta.serializer())).items

    suspend fun createDocument(vehicleId: String, body: DocumentCreate): DocumentMeta =
        call("POST", "/vehicles/$vehicleId/documents", DocumentMeta.serializer(), encode(body))

    suspend fun vehicleImages(vehicleId: String): List<VehicleImage> =
        call("GET", "/vehicles/$vehicleId/images", Page.serializer(VehicleImage.serializer())).items

    suspend fun setVehicleImage(vehicleId: String, fileId: String): VehicleImage =
        call("POST", "/vehicles/$vehicleId/images", VehicleImage.serializer(), encode(VehicleImageCreate(fileId, primary = true)))

    /** Datei hochladen (multipart, Felder vor der Datei). Bei Dublette liefert der Server die vorhandene Datei (DO-02). */
    suspend fun upload(vehicleId: String, name: String, mediaType: String?, bytes: ByteArray, captureSource: String = "gallery"): FileMeta =
        withContext(Dispatchers.IO) {
            val body = MultipartBody.Builder().setType(MultipartBody.FORM)
                .addFormDataPart("capture_source", captureSource)
                .addFormDataPart("file", name, bytes.toRequestBody(mediaType?.toMediaTypeOrNull()))
                .build()
            val b = Request.Builder().url((apiBase.toString().trimEnd('/') + "/vehicles/$vehicleId/files").toHttpUrl())
                .header("Accept", "application/json, application/problem+json").post(body)
            cookies.value(SessionCookieJar.CSRF_COOKIE)?.let { b.header("X-CSRF-Token", it) }
            http.newCall(b.build()).execute().use { resp ->
                val text = resp.body?.string().orEmpty()
                if (!resp.isSuccessful) throw ApiException(resp.code, runCatching { VectraJson.decodeFromString(Problem.serializer(), text) }.getOrNull())
                VectraJson.decodeFromString(FileMeta.serializer(), text)
            }
        }

    /** Vorschaubild (JPEG ohne EXIF) als Bytes; null, wenn es keines gibt. */
    suspend fun preview(vehicleId: String, fileId: String, size: String = "thumbnail"): ByteArray? =
        withContext(Dispatchers.IO) {
            val b = Request.Builder().url((apiBase.toString().trimEnd('/') + "/vehicles/$vehicleId/files/$fileId/preview?size=$size").toHttpUrl())
            http.newCall(b.build()).execute().use { resp -> if (resp.isSuccessful) resp.body?.bytes() else null }
        }

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
