package app.vectra.android

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import android.graphics.BitmapFactory
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import app.vectra.android.feature.AssistantState
import app.vectra.android.feature.Confirmation
import app.vectra.android.feature.CostsState
import app.vectra.android.feature.DocumentsState
import app.vectra.android.feature.FormError
import app.vectra.android.feature.Freshness
import app.vectra.android.feature.MaintenanceState
import app.vectra.android.feature.TripsState
import app.vectra.android.feature.HomeState
import app.vectra.android.feature.LoginState
import app.vectra.android.feature.MonthBar
import app.vectra.android.feature.OdometerState
import app.vectra.android.feature.SettingsState
import app.vectra.android.feature.ThemeMode
import app.vectra.android.feature.VehiclesState
import app.vectra.android.sync.SyncWorker
import app.vectra.android.ui.Tab
import app.vectra.core.model.CompletionCreate
import app.vectra.core.model.CostEntryCreate
import app.vectra.core.model.CostItem
import app.vectra.core.model.CostOccurrence
import app.vectra.core.model.DocumentCreate
import app.vectra.core.model.DueStatus
import app.vectra.core.model.MaintenanceBookApply
import app.vectra.core.model.Money
import app.vectra.core.model.QuantityInput
import app.vectra.core.model.ServiceEntryCreate
import app.vectra.core.model.Trip
import app.vectra.core.model.TripFinish
import app.vectra.core.model.TripStart
import app.vectra.core.model.Vehicle
import app.vectra.core.repo.VectraRepository
import app.vectra.core.util.MoneyFormat
import app.vectra.core.util.UuidV7
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import java.time.Instant
import java.time.LocalDate
import java.time.ZoneId
import app.vectra.core.net.ApiClient
import app.vectra.core.net.ApiException
import app.vectra.core.net.Diagnosis
import app.vectra.core.outbox.OutboxEntry
import app.vectra.core.outbox.OutboxStatus
import app.vectra.core.util.Format
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.io.IOException
import java.text.SimpleDateFormat
import java.time.format.TextStyle
import java.util.Date
import java.util.Locale

sealed interface Route {
    data object Login : Route
    data class Main(val tab: Tab) : Route
    data object Odometer : Route
    data object Vehicles : Route
    data object Settings : Route
    data object Service : Route
    data object Documents : Route
    data object Assistant : Route
}

private val attention = setOf(OutboxStatus.NEEDS_CONFIRMATION, OutboxStatus.CONFLICT, OutboxStatus.FAILED)

/** Was nach der Dateiauswahl mit der Datei geschehen soll. */
sealed interface PickPurpose {
    data object Document : PickPurpose
    data class VehiclePhoto(val vehicleId: String) : PickPurpose
}

/** Vom System ausgewählte Datei (Android-Teil liest sie ein). */
class PickedFile(val name: String, val mediaType: String?, val bytes: ByteArray)

sealed interface DialogState {
    data object AddReading : DialogState
    data class Pending(val entry: OutboxEntry) : DialogState
    data object Logout : DialogState
    data class Complete(val item: DueStatus) : DialogState
    data object AddService : DialogState
    data object ApplyBook : DialogState
    data object AddCost : DialogState
    data object StartTrip : DialogState
    data class FinishTrip(val trip: Trip) : DialogState
    data class AddDocument(val file: PickedFile) : DialogState
}

/** Zustand der App: Navigation, Sitzung, geladene Daten und Dialoge. */
class MainViewModel(app: Application) : AndroidViewModel(app) {
    private val c = (app as VectraApp).container

    private val _stack = MutableStateFlow(if (c.hasSession) listOf<Route>(Route.Main(Tab.Home)) else listOf(Route.Login))
    val stack: StateFlow<List<Route>> = _stack.asStateFlow()

    val login = MutableStateFlow(LoginState(server = c.prefs.server, email = c.prefs.accountEmail))
    val theme = MutableStateFlow(c.prefs.theme)
    val vehicles = MutableStateFlow(VehiclesState())
    val home = MutableStateFlow(HomeState())
    val odometer = MutableStateFlow(OdometerState())
    val dialog = MutableStateFlow<DialogState?>(null)
    val maintenance = MutableStateFlow(MaintenanceState())
    val costs = MutableStateFlow(CostsState())
    val trips = MutableStateFlow(TripsState())
    val documents = MutableStateFlow(DocumentsState())
    val assistant = MutableStateFlow(AssistantState())
    val formError = MutableStateFlow<FormError?>(null)
    val busy = MutableStateFlow(false)
    var pickPurpose: PickPurpose = PickPurpose.Document
    private val photoCache = HashMap<String, ImageBitmap>()
    private val thumbCache = HashMap<String, ImageBitmap>()
    val pending: StateFlow<List<OutboxEntry>> = c.outboxStore.observe().stateIn(viewModelScope, SharingStarted.Eagerly, emptyList())

    private var refreshJob: Job? = null

    /** Kurzer Hinweis am unteren Rand (z. B. „Wieder online – synchronisiert“). */
    val notice = MutableStateFlow<String?>(null)
    val online: StateFlow<Boolean> = c.online

    /** Einträge, zu denen in dieser Sitzung schon eine Rückfrage gezeigt wurde. */
    private val alerted = HashSet<String>()

    init {
        if (c.hasSession) refresh()
        // Verbindung wieder da: sofort synchronisieren und neu laden.
        viewModelScope.launch {
            var was = c.online.value
            c.online.collect { now ->
                if (now && !was && c.hasSession && _stack.value.lastOrNull() != Route.Login) {
                    val n = pending.value.count { it.status == OutboxStatus.PENDING }
                    notice.value = if (n > 0) "Wieder online – $n ${if (n == 1) "Eintrag wird" else "Einträge werden"} übertragen" else "Wieder online – Daten aktualisiert"
                    syncNow()
                } else if (!now && was) {
                    notice.value = "Offline – Kilometerstände werden gespeichert und später übertragen"
                }
                was = now
            }
        }
        // Rückfragen der Synchronisierung (Befund, Konflikt, Fehler) als Hinweis-Box zeigen.
        viewModelScope.launch {
            combine(pending, dialog) { list, d -> list to d }.collect { (list, d) ->
                if (d != null || _stack.value.lastOrNull() == Route.Login) return@collect
                val next = list.firstOrNull { it.status in attention && it.id !in alerted } ?: return@collect
                alerted += next.id
                dialog.value = DialogState.Pending(next)
            }
        }
        // Sobald Einträge übertragen wurden, die Anzeige mit dem Serverstand auffrischen.
        viewModelScope.launch {
            var last = -1
            pending.collect { list ->
                if (last >= 0 && list.size < last) refresh()
                last = list.size
            }
        }
    }

    // --- Navigation ---

    val canGoBack: Boolean get() = _stack.value.size > 1 || (_stack.value.lastOrNull() as? Route.Main)?.tab.let { it != null && it != Tab.Home }

    fun back() = _stack.update { s ->
        when {
            s.size > 1 -> s.dropLast(1)
            (s.lastOrNull() as? Route.Main)?.tab != Tab.Home -> listOf(Route.Main(Tab.Home))
            else -> s
        }
    }

    fun selectTab(tab: Tab) { _stack.value = listOf(Route.Main(tab)) }
    fun open(route: Route) = _stack.update { it + route }

    // --- Anmeldung ---

    fun onLoginChange(transform: (LoginState) -> LoginState) = login.update { transform(it).copy(error = null) }

    fun submitLogin() {
        val s = login.value
        if (s.busy) return
        login.update { it.copy(busy = true, error = null) }
        viewModelScope.launch {
            val error = try {
                val base = ApiClient.normalizeBase(s.server).toString().removeSuffix("/api/v1/")
                if (base != c.prefs.server) {
                    c.signOut()
                    c.prefs.server = base
                }
                val api = c.repository()!!.api
                val account = api.login(s.email.trim(), s.password)
                // Wird die Sitzung nicht mitgesendet (Secure-Cookie über HTTP, Proxy), scheitert der nächste Aufruf.
                val sessionOk = try { api.me(); true } catch (e: ApiException) { if (e.status == 401) false else throw e }
                if (!sessionOk) {
                    Diagnosis.sessionNotSent(base)
                } else {
                    c.prefs.accountName = account.displayName
                    c.prefs.accountEmail = account.email
                    null
                }
            } catch (e: ApiException) {
                when (e.status) {
                    401 -> "E-Mail oder Passwort ist falsch."
                    429 -> "Zu viele Anmeldeversuche. Bitte warte kurz."
                    else -> e.message
                }
            } catch (e: IOException) {
                Diagnosis.network(e, c.prefs.server.ifBlank { s.server })
            } catch (e: IllegalArgumentException) {
                "Ungültige Server-Adresse."
            }
            if (error == null) {
                login.update { it.copy(busy = false, password = "") }
                SyncWorker.schedulePeriodic(getApplication())
                SyncWorker.syncNow(getApplication())
                _stack.value = listOf(Route.Main(Tab.Home))
                refresh()
            } else {
                login.update { it.copy(busy = false, error = error) }
            }
        }
    }

    fun logout() {
        dialog.value = null
        viewModelScope.launch {
            SyncWorker.cancelAll(getApplication())
            c.signOut()
            home.value = HomeState()
            odometer.value = OdometerState()
            maintenance.value = MaintenanceState()
            costs.value = CostsState()
            trips.value = TripsState()
            documents.value = DocumentsState()
            assistant.value = AssistantState()
            photoCache.clear()
            thumbCache.clear()
            vehicles.value = VehiclesState()
            login.update { LoginState(server = c.prefs.server) }
            _stack.value = listOf(Route.Login)
        }
    }

    // --- Daten ---

    private fun selected(list: List<Vehicle>): Vehicle? =
        list.firstOrNull { it.id == c.prefs.vehicleId } ?: list.firstOrNull { it.status == "active" } ?: list.firstOrNull()

    fun refresh() {
        val repo = c.repository() ?: return
        refreshJob?.cancel()
        refreshJob = viewModelScope.launch {
            home.update { it.copy(loading = true, error = null) }
            odometer.update { it.copy(loading = true, error = null) }
            vehicles.update { it.copy(loading = true, error = null) }
            try {
                val vs = repo.vehicles()
                val v = selected(vs.value)
                c.prefs.vehicleId = v?.id
                val fresh = Freshness(vs.offline, vs.fetchedAt?.let { stamp(it) })
                vehicles.update { it.copy(vehicles = vs.value, selectedId = v?.id, loading = false) }
                if (v == null) {
                    home.value = HomeState(loading = false, freshness = fresh)
                    odometer.value = OdometerState(loading = false, freshness = fresh)
                    return@launch
                }
                val cur = repo.current(v.id)
                val rs = repo.readings(v.id)
                vehicles.update { it.copy(currentKm = it.currentKm + (v.id to (cur.value.meterValue?.let { m -> Format.meter(m.canonical, v.meterUnit) } ?: "–"))) }
                val offline = vs.offline || cur.offline || rs.offline
                val f = Freshness(offline, if (offline) (rs.fetchedAt ?: vs.fetchedAt)?.let { stamp(it) } else null)
                val same = home.value.vehicle?.id == v.id
                home.value = HomeState(v, cur.value, rs.value.take(3), freshness = f, loading = false, photo = photoCache[v.id],
                    nextDue = home.value.nextDue.takeIf { same }, yearCosts = home.value.yearCosts.takeIf { same })
                viewModelScope.launch { loadModules(repo, v, vs.value) }
                odometer.value = OdometerState(v, cur.value, rs.value, odometer.value.months.takeIf { odometer.value.vehicle?.id == v.id } ?: emptyList(), freshness = f, loading = false)
                if (!offline) {
                    val months = repo.months(v.id).map { m ->
                        MonthBar(m.month.month.getDisplayName(TextStyle.SHORT, Locale.GERMANY).removeSuffix("."), m.km)
                    }
                    odometer.update { it.copy(months = months) }
                }
            } catch (e: ApiException) {
                if (e.status == 401) {
                    // Sitzung abgelaufen: neu anmelden, die Outbox bleibt erhalten.
                    login.update { it.copy(error = "Deine Sitzung ist abgelaufen. Bitte melde dich neu an.") }
                    _stack.value = listOf(Route.Login)
                } else fail(e.message ?: "Fehler ${e.status}")
            } catch (e: IOException) {
                fail("Keine Verbindung zum Server und noch keine Daten auf dem Gerät.")
            }
        }
    }

    // --- Wartung, Service, Kosten, Fahrten, Dokumente ---

    private fun <T> app.vectra.core.repo.Loaded<T>.fresh() = Freshness(offline, if (offline) fetchedAt?.let { stamp(it) } else null)

    private suspend fun loadModules(repo: VectraRepository, v: Vehicle, all: List<Vehicle>) = coroutineScope {
        val zone = runCatching { ZoneId.of(v.ownerTimeZone) }.getOrDefault(ZoneId.systemDefault())
        val today = LocalDate.now(zone)
        val year = today.year
        val monthFrom = today.withDayOfMonth(1)
        val monthTo = today.withDayOfMonth(today.lengthOfMonth())
        launch {
            runCatching {
                val due = repo.due(v.id)
                val services = repo.services(v.id)
                maintenance.value = MaintenanceState(v, due.value, services.value, due.fresh(), loading = false, books = maintenance.value.books)
                home.update { it.copy(nextDue = due.value.firstOrNull { d -> d.level != "unknown" } ?: due.value.firstOrNull()) }
            }.onFailure { e -> maintenance.update { it.copy(vehicle = v, loading = false, error = message(e)) } }
        }
        launch {
            runCatching {
                val report = repo.costReport(v.id, "$year-01-01", "$year-12-31")
                val occ = repo.occurrences(v.id)
                val entries = repo.costEntries(v.id)
                costs.value = CostsState(v, year, report.value, occ.value.items, occ.value.moreOpen, entries.value, report.fresh(), loading = false)
                val cur = report.value.currencies.firstOrNull()
                home.update { it.copy(yearCosts = cur?.let { c -> MoneyFormat.format(c.runningTotalMinor, c.currency) }) }
            }.onFailure { e -> costs.update { it.copy(vehicle = v, year = year, loading = false, error = message(e)) } }
        }
        launch {
            runCatching {
                val list = repo.trips(v.id)
                val cats = repo.tripCategories(v.id)
                val rep = runCatching { repo.tripReport(v.id, monthFrom.toString(), monthTo.toString()).value }.getOrNull()
                val label = today.month.getDisplayName(TextStyle.FULL, Locale.GERMANY)
                trips.value = TripsState(v, list.value, cats.value, rep, label, list.fresh(), loading = false)
            }.onFailure { e -> trips.update { it.copy(vehicle = v, loading = false, error = message(e)) } }
        }
        launch {
            runCatching {
                val docs = repo.documents(v.id)
                val files = repo.files(v.id)
                documents.update { DocumentsState(v, docs.value, files.value, it.thumbs.takeIf { _ -> it.vehicle?.id == v.id } ?: emptyMap(), freshness = docs.fresh(), loading = false) }
                val byId = files.value.associateBy { f -> f.id }
                val wanted = docs.value.mapNotNull { d -> d.fileIds.firstOrNull() }.filter { id -> byId[id]?.hasPreview == true }
                for (id in wanted) {
                    val bmp = thumbCache[id] ?: repo.api.preview(v.id, id, "thumbnail")?.let(::decode)?.also { b -> thumbCache[id] = b } ?: continue
                    documents.update { it.copy(thumbs = it.thumbs + (id to bmp)) }
                }
            }.onFailure { e -> documents.update { it.copy(vehicle = v, loading = false, error = message(e)) } }
        }
        // Fahrzeugfotos (Hauptbild, bereinigte Vorschau ohne EXIF)
        launch {
            for (veh in all) {
                val bmp = photoCache[veh.id] ?: runCatching {
                    val img = repo.images(veh.id).value.firstOrNull { it.primary } ?: return@runCatching null
                    repo.api.preview(veh.id, img.fileId, "preview")?.let(::decode)
                }.getOrNull() ?: continue
                photoCache[veh.id] = bmp
                vehicles.update { it.copy(photos = it.photos + (veh.id to bmp)) }
                if (veh.id == v.id) home.update { it.copy(photo = bmp) }
            }
        }
    }

    private fun decode(bytes: ByteArray): ImageBitmap? = BitmapFactory.decodeByteArray(bytes, 0, bytes.size)?.asImageBitmap()

    private fun message(e: Throwable): String = when (e) {
        is ApiException -> problemText(e)
        is IOException -> "Keine Verbindung zum Server."
        else -> e.message ?: "Unbekannter Fehler"
    }

    private fun problemText(e: ApiException): String {
        val p = e.problem
        val errs = p?.errors?.mapNotNull { it.message }?.filter { it.isNotBlank() }.orEmpty()
        return when {
            errs.isNotEmpty() -> errs.joinToString(" · ")
            p?.anomalies?.isNotEmpty() == true -> p.anomalies.mapNotNull { it.message }.joinToString(" ")
            else -> p?.detail ?: p?.title ?: "Fehler ${e.status}"
        }
    }

    fun openDialog(d: DialogState) {
        formError.value = null
        dialog.value = d
    }

    fun closeDialog() {
        formError.value = null
        dialog.value = null
    }

    /** Erfassen geht direkt an den Server (ohne Outbox); Fehler und Befunde erscheinen im Dialog. */
    private fun submit(block: suspend (VectraRepository, Vehicle) -> Unit) {
        val repo = c.repository() ?: return
        val v = home.value.vehicle ?: return
        if (busy.value) return
        busy.value = true
        formError.value = null
        viewModelScope.launch {
            try {
                block(repo, v)
                dialog.value = null
                refresh()
            } catch (e: ApiException) {
                formError.value = FormError(problemText(e), e.problem?.anomalies.orEmpty())
            } catch (e: IOException) {
                formError.value = FormError("Keine Verbindung. Dieser Eintrag lässt sich nur online speichern.")
            } catch (e: kotlinx.coroutines.CancellationException) {
                throw e
            } catch (e: Exception) {
                formError.value = FormError(message(e))
            } finally {
                busy.value = false
            }
        }
    }

    private fun zoneOf(v: Vehicle): String = runCatching { ZoneId.of(v.ownerTimeZone).id }.getOrDefault(ZoneId.systemDefault().id)
    private fun noonToday(v: Vehicle): String = LocalDate.now(ZoneId.of(zoneOf(v))).atTime(12, 0).atZone(ZoneId.of(zoneOf(v))).toInstant().toString()

    fun lastKm(): Double? = home.value.current?.meterValue?.canonical?.let { it / 1000.0 }

    fun complete(item: DueStatus, km: Double?) = submit { repo, v ->
        val today = LocalDate.now(ZoneId.of(zoneOf(v))).toString()
        repo.api.complete(v.id, item.itemId, CompletionCreate("done", today, km?.let { QuantityInput(it, "km") }), UuidV7.generate())
    }

    fun addService(title: String, kind: String, km: Double?, parts: Long, labor: Long, other: Long, completes: List<String>, confirm: Confirmation?) = submit { repo, v ->
        val items = listOf("parts" to parts, "labor" to labor, "other" to other).filter { it.second != 0L }.map { CostItem(it.first, amountMinor = it.second) }
            .ifEmpty { listOf(CostItem("labor", amountMinor = 0)) }
        repo.api.createServiceEntry(v.id, ServiceEntryCreate(UuidV7.generate(), noonToday(v), zoneOf(v), kind = kind, title = title, currency = v.defaultCurrency,
            odometer = km?.let { QuantityInput(it, "km") }, costItems = items, completes = completes,
            confirmAnomalies = confirm?.codes, anomalyReason = confirm?.reason))
    }

    fun addCost(category: String, title: String, amountMinor: Long) = submit { repo, v ->
        val today = LocalDate.now(ZoneId.of(zoneOf(v))).toString()
        repo.api.createCostEntry(v.id, CostEntryCreate(UuidV7.generate(), category, title, today, Money(amountMinor, v.defaultCurrency)))
    }

    fun confirmOccurrence(o: CostOccurrence) = submit { repo, v -> repo.api.confirmOccurrence(v.id, o.planId, o.dueOn) }

    fun startTrip(km: Double, categoryId: String, purpose: String?, from: String?, confirm: Confirmation?) = submit { repo, v ->
        repo.api.startTrip(v.id, TripStart(UuidV7.generate(), Instant.now().toString(), ZoneId.systemDefault().id, QuantityInput(km, "km"), categoryId, purpose, from,
            confirm?.codes, confirm?.reason))
    }

    fun finishTrip(trip: Trip, km: Double, to: String?, confirm: Confirmation?) = submit { repo, v ->
        repo.api.finishTrip(v.id, trip, TripFinish(Instant.now().toString(), QuantityInput(km, "km"), to, confirm?.codes, confirm?.reason))
    }

    /** Nach der Dateiauswahl: Fahrzeugfoto direkt hochladen, Dokumente erst nach Titel und Typ. */
    fun onPicked(file: PickedFile) {
        when (val p = pickPurpose) {
            PickPurpose.Document -> openDialog(DialogState.AddDocument(file))
            is PickPurpose.VehiclePhoto -> uploadVehiclePhoto(p.vehicleId, file)
        }
    }

    private fun uploadVehiclePhoto(vehicleId: String, file: PickedFile) {
        val repo = c.repository() ?: return
        vehicles.update { it.copy(uploading = true, error = null) }
        viewModelScope.launch {
            try {
                val meta = repo.api.upload(vehicleId, file.name, file.mediaType, file.bytes, "gallery")
                repo.api.setVehicleImage(vehicleId, meta.id)
                photoCache.remove(vehicleId)
                repo.api.preview(vehicleId, meta.id, "preview")?.let(::decode)?.let { bmp ->
                    photoCache[vehicleId] = bmp
                    vehicles.update { it.copy(photos = it.photos + (vehicleId to bmp)) }
                    if (home.value.vehicle?.id == vehicleId) home.update { it.copy(photo = bmp) }
                }
                vehicles.update { it.copy(uploading = false) }
            } catch (e: Exception) {
                vehicles.update { it.copy(uploading = false, error = message(e)) }
            }
        }
    }

    fun createDocument(file: PickedFile, title: String, docType: String) = submit { repo, v ->
        documents.update { it.copy(uploading = true, message = null) }
        try {
            val meta = repo.api.upload(v.id, file.name, file.mediaType, file.bytes, if (file.mediaType?.startsWith("image/") == true) "camera" else "upload")
            repo.api.createDocument(v.id, DocumentCreate(UuidV7.generate(), docType, title, listOf(meta.id)))
            documents.update { it.copy(message = if (meta.duplicateOf != null) "Die Datei war bereits vorhanden; das Dokument verweist auf sie." else "„$title“ gespeichert.") }
        } finally {
            documents.update { it.copy(uploading = false) }
        }
    }

    // --- Assistent (ADR-026, ADR-032): nur online; Einträge als Vorschlag ---

    fun openAssistant() {
        open(Route.Assistant)
        loadAssistant()
    }

    private fun loadAssistant() {
        val repo = c.repository() ?: return
        assistant.update { it.copy(loading = true, error = null) }
        viewModelScope.launch {
            try {
                val st = repo.api.assistantStatus()
                var conv = assistant.value.conversationId
                var msgs = assistant.value.messages
                if (st.enabled && !st.consentRequired && conv == null) {
                    conv = repo.api.conversations().firstOrNull()?.id
                    if (conv != null) msgs = repo.api.assistantMessages(conv)
                }
                assistant.update { it.copy(status = st, conversationId = conv, messages = msgs, loading = false) }
            } catch (e: Exception) {
                if (e is kotlinx.coroutines.CancellationException) throw e
                assistant.update { it.copy(loading = false, error = message(e)) }
            }
        }
    }

    fun assistantInput(text: String) = assistant.update { it.copy(input = text, error = null) }

    /** Ergebnis der Spracherkennung an die Eingabe anhängen. */
    fun onDictated(text: String) = assistant.update { s -> s.copy(input = listOf(s.input.trim(), text.trim()).filter { it.isNotEmpty() }.joinToString(" "), error = null) }

    fun assistantConsent() {
        val repo = c.repository() ?: return
        val name = assistant.value.status?.chatProvider?.name ?: "Anthropic (Claude)"
        viewModelScope.launch {
            try {
                repo.api.giveConsent(name)
                loadAssistant()
            } catch (e: Exception) {
                if (e is kotlinx.coroutines.CancellationException) throw e
                assistant.update { it.copy(error = message(e)) }
            }
        }
    }

    fun assistantNew() = assistant.update { it.copy(conversationId = null, messages = emptyList(), error = null, anomalies = emptyMap()) }

    fun assistantSend() {
        val repo = c.repository() ?: return
        val text = assistant.value.input.trim()
        if (text.isEmpty() || assistant.value.pendingText != null) return
        assistant.update { it.copy(input = "", pendingText = text, progress = null, pendingProposals = emptyList(), error = null) }
        viewModelScope.launch {
            try {
                val conv = assistant.value.conversationId ?: repo.api.createConversation(home.value.vehicle?.id).id!!
                assistant.update { it.copy(conversationId = conv) }
                repo.api.sendAssistantMessage(conv, text,
                    onStatus = { t -> assistant.update { it.copy(progress = t) } },
                    onProposal = { p -> assistant.update { it.copy(pendingProposals = it.pendingProposals + p) } })
                val msgs = repo.api.assistantMessages(conv)
                assistant.update { it.copy(messages = msgs, pendingText = null, progress = null, pendingProposals = emptyList()) }
            } catch (e: Exception) {
                if (e is kotlinx.coroutines.CancellationException) throw e
                assistant.update { it.copy(pendingText = null, progress = null, pendingProposals = emptyList(), input = text, error = message(e)) }
            }
        }
    }

    private fun replaceProposal(p: app.vectra.core.model.Proposal) = assistant.update { s ->
        s.copy(messages = s.messages.map { m -> m.copy(proposals = m.proposals.map { if (it.id == p.id) p else it }) })
    }

    fun confirmProposal(p: app.vectra.core.model.Proposal, reason: String?) {
        val repo = c.repository() ?: return
        val known = assistant.value.anomalies[p.id].orEmpty()
        assistant.update { it.copy(busyProposal = p.id, error = null) }
        viewModelScope.launch {
            try {
                val body = if (known.isNotEmpty() && reason != null) app.vectra.core.model.ProposalConfirm(known.map { it.code }, reason) else app.vectra.core.model.ProposalConfirm()
                val r = repo.api.confirmProposal(p.id, body)
                replaceProposal(r)
                assistant.update { it.copy(busyProposal = null, anomalies = it.anomalies - p.id) }
                refresh()
            } catch (e: ApiException) {
                val an = e.problem?.anomalies.orEmpty()
                assistant.update { it.copy(busyProposal = null, anomalies = if (an.isNotEmpty()) it.anomalies + (p.id to an) else it.anomalies,
                    error = if (an.isEmpty()) problemText(e) else null) }
            } catch (e: Exception) {
                if (e is kotlinx.coroutines.CancellationException) throw e
                assistant.update { it.copy(busyProposal = null, error = message(e)) }
            }
        }
    }

    fun rejectProposal(p: app.vectra.core.model.Proposal) {
        val repo = c.repository() ?: return
        viewModelScope.launch {
            try {
                replaceProposal(repo.api.rejectProposal(p.id))
            } catch (e: Exception) {
                if (e is kotlinx.coroutines.CancellationException) throw e
                assistant.update { it.copy(error = message(e)) }
            }
        }
    }

    fun assistantError(msg: String) = assistant.update { it.copy(error = msg) }

    private fun fail(msg: String) {
        home.update { it.copy(loading = false, error = msg) }
        odometer.update { it.copy(loading = false, error = msg) }
        vehicles.update { it.copy(loading = false, error = msg) }
    }

    private fun stamp(ms: Long): String = SimpleDateFormat("dd.MM.yyyy, HH:mm", Locale.GERMANY).format(Date(ms))

    fun selectVehicle(v: Vehicle) {
        c.prefs.vehicleId = v.id
        back()
        refresh()
    }

    // --- Erfassen und Outbox ---

    fun addReading(value: Double, note: String) {
        val v = home.value.vehicle ?: return
        dialog.value = null
        viewModelScope.launch {
            c.repository()?.addReading(v, value, note)
            SyncWorker.syncNow(getApplication())
        }
    }

    fun confirm(entry: OutboxEntry, codes: List<String>, reason: String) {
        dialog.value = null
        viewModelScope.launch {
            c.repository()?.outbox?.confirm(entry.id, codes, reason)
            SyncWorker.syncNow(getApplication())
        }
    }

    /** Konflikt-Antwort „Wert korrigieren“: neuer Stand ersetzt den Eintrag und wird neu geprüft. */
    fun correct(entry: OutboxEntry, value: Double) {
        dialog.value = null
        viewModelScope.launch {
            val unit = home.value.vehicle?.takeIf { it.id == entry.vehicleId }?.meterUnit ?: "km"
            c.repository()?.outbox?.amendValue(entry.id, value, "Kilometerstand ${Format.number(value, 1)} $unit")
            SyncWorker.syncNow(getApplication())
        }
    }

    fun dismissNotice() { notice.value = null }

    /** Wartungsbücher laden (einmal je Sitzung) und den Übernehmen-Dialog öffnen. */
    fun openBooks() {
        openDialog(DialogState.ApplyBook)
        if (maintenance.value.books.isNotEmpty()) return
        val repo = c.repository() ?: return
        viewModelScope.launch {
            try {
                val list = repo.api.maintenanceBooks()
                maintenance.update { it.copy(books = list) }
            } catch (e: Exception) {
                if (e is kotlinx.coroutines.CancellationException) throw e
                formError.value = FormError(message(e))
            }
        }
    }

    fun applyBook(bookId: String, sinceNew: Boolean, date: String, km: Double?) = submit { repo, v ->
        val r = repo.api.applyMaintenanceBook(v.id, bookId, MaintenanceBookApply(date, km?.let { QuantityInput(it, "km") }, sinceNew))
        notice.value = "${r.created.size} ${if (r.created.size == 1) "Wartung" else "Wartungen"} angelegt" +
            if (r.skipped.isNotEmpty()) ", ${r.skipped.size} schon vorhanden" else ""
    }

    fun discard(entry: OutboxEntry) {
        dialog.value = null
        viewModelScope.launch { c.repository()?.outbox?.discard(entry.id) }
    }

    fun retry(entry: OutboxEntry) {
        dialog.value = null
        viewModelScope.launch {
            c.repository()?.outbox?.retry(entry.id)
            SyncWorker.syncNow(getApplication())
        }
    }

    fun syncNow() {
        SyncWorker.syncNow(getApplication())
        refresh()
    }

    fun setTheme(mode: ThemeMode) {
        c.prefs.theme = mode
        theme.value = mode
    }

    fun settingsState(pendingCount: Int) = SettingsState(
        server = c.prefs.server,
        accountName = c.prefs.accountName,
        accountEmail = c.prefs.accountEmail,
        theme = theme.value,
        pendingCount = pendingCount,
        clockSkewMinutes = c.repository()?.api?.clockSkew?.toMinutes() ?: 0,
        version = BuildConfig.VERSION_NAME,
        online = c.online.value,
    )
}
