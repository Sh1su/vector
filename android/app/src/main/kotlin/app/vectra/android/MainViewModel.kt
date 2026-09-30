package app.vectra.android

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import app.vectra.android.feature.Freshness
import app.vectra.android.feature.HomeState
import app.vectra.android.feature.LoginState
import app.vectra.android.feature.MonthBar
import app.vectra.android.feature.OdometerState
import app.vectra.android.feature.SettingsState
import app.vectra.android.feature.ThemeMode
import app.vectra.android.feature.VehiclesState
import app.vectra.android.sync.SyncWorker
import app.vectra.android.ui.Tab
import app.vectra.core.model.Vehicle
import app.vectra.core.net.ApiClient
import app.vectra.core.net.ApiException
import app.vectra.core.outbox.OutboxEntry
import app.vectra.core.util.Format
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
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
}

sealed interface DialogState {
    data object AddReading : DialogState
    data class Pending(val entry: OutboxEntry) : DialogState
    data object Logout : DialogState
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
    val pending: StateFlow<List<OutboxEntry>> = c.outboxStore.observe().stateIn(viewModelScope, SharingStarted.Eagerly, emptyList())

    private var refreshJob: Job? = null

    init {
        if (c.hasSession) refresh()
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
                val account = c.repository()!!.api.login(s.email.trim(), s.password)
                c.prefs.accountName = account.displayName
                c.prefs.accountEmail = account.email
                null
            } catch (e: ApiException) {
                when (e.status) {
                    401 -> "E-Mail oder Passwort ist falsch."
                    429 -> "Zu viele Anmeldeversuche. Bitte warte kurz."
                    else -> e.message
                }
            } catch (e: IOException) {
                "Server nicht erreichbar. Prüfe die Adresse und deine Verbindung."
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
                home.value = HomeState(v, cur.value, rs.value.take(3), freshness = f, loading = false)
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
    )
}
