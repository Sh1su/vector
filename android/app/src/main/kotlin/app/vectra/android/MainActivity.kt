package app.vectra.android

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.viewModels
import androidx.compose.foundation.Image
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.width
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import app.vectra.android.feature.AddReadingDialog
import app.vectra.android.feature.ComingSoonScreen
import app.vectra.android.feature.ConfirmDialog
import app.vectra.android.feature.HomeScreen
import app.vectra.android.feature.LoginScreen
import app.vectra.android.feature.MoreScreen
import app.vectra.android.feature.MoreTarget
import app.vectra.android.feature.OdometerScreen
import app.vectra.android.feature.PendingDialog
import app.vectra.android.feature.SettingsScreen
import app.vectra.android.feature.ThemeMode
import app.vectra.android.feature.VehiclesScreen
import app.vectra.android.ui.Tab
import app.vectra.android.ui.TabBar
import app.vectra.android.ui.V
import app.vectra.android.ui.VectraTheme
import app.vectra.core.outbox.OutboxStatus

class MainActivity : ComponentActivity() {
    private val vm: MainViewModel by viewModels()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            val mode by vm.theme.collectAsStateWithLifecycle()
            val dark = when (mode) {
                ThemeMode.System -> isSystemInDarkTheme()
                ThemeMode.Light -> false
                ThemeMode.Dark -> true
            }
            VectraTheme(dark = dark) { AppRoot(vm) }
        }
    }
}

@Composable
private fun AppRoot(vm: MainViewModel) {
    val stack by vm.stack.collectAsStateWithLifecycle()
    val pending by vm.pending.collectAsStateWithLifecycle()
    val dialog by vm.dialog.collectAsStateWithLifecycle()
    val route = stack.last()

    BackHandler(enabled = vm.canGoBack) { vm.back() }

    if (route == Route.Login) {
        val s by vm.login.collectAsStateWithLifecycle()
        val logo = if (V.colors.dark) R.drawable.brand_logo_negative else R.drawable.brand_logo
        LoginScreen(
            s,
            logo = { Image(painterResource(logo), "Vectra – Intelligent Vehicle Management", Modifier.width(236.dp).height(172.dp)) },
            onServer = { v -> vm.onLoginChange { it.copy(server = v) } },
            onEmail = { v -> vm.onLoginChange { it.copy(email = v) } },
            onPassword = { v -> vm.onLoginChange { it.copy(password = v) } },
            onSubmit = vm::submitLogin,
        )
        return
    }

    // Unterseiten zeigen den Tab, von dem aus sie geöffnet wurden.
    val activeTab = (stack.first() as? Route.Main)?.tab ?: Tab.Home
    Column(Modifier.fillMaxSize()) {
        Box(Modifier.weight(1f)) {
            when (route) {
                is Route.Main -> when (route.tab) {
                    Tab.Home -> {
                        val s by vm.home.collectAsStateWithLifecycle()
                        HomeScreen(
                            s.copy(pending = pending.filter { it.vehicleId == s.vehicle?.id }),
                            wordmark = { Image(painterResource(R.drawable.brand_wordmark_negative), "Vectra", Modifier.width(109.dp).height(26.dp)) },
                            onPickVehicle = { vm.open(Route.Vehicles) },
                            onOdometer = { vm.open(Route.Odometer) },
                            onAddReading = { vm.dialog.value = DialogState.AddReading },
                            onOpenPending = { vm.open(Route.Odometer) },
                            onRefresh = vm::syncNow,
                        )
                    }
                    Tab.Trips -> ComingSoonScreen("Fahrten", "Das Fahrtenbuch mit Start, Ziel und Zweck folgt in Iteration 3. Kilometerstände erfasst du bereits unter Übersicht.")
                    Tab.Maintenance -> ComingSoonScreen("Wartung", "Wartungspläne, Fälligkeiten und die Servicehistorie folgen in Iteration 2.")
                    Tab.Costs -> ComingSoonScreen("Kosten", "Kostenübersicht und wiederkehrende Kosten folgen in Iteration 2.")
                    Tab.More -> MoreScreen { t ->
                        when (t) {
                            MoreTarget.Vehicles -> vm.open(Route.Vehicles)
                            MoreTarget.Odometer -> vm.open(Route.Odometer)
                            MoreTarget.Settings -> vm.open(Route.Settings)
                        }
                    }
                }
                Route.Odometer -> {
                    val s by vm.odometer.collectAsStateWithLifecycle()
                    OdometerScreen(
                        s.copy(pending = pending.filter { it.vehicleId == s.vehicle?.id }),
                        onBack = vm::back,
                        onAdd = { vm.dialog.value = DialogState.AddReading },
                        onPending = { vm.dialog.value = DialogState.Pending(it) },
                    )
                }
                Route.Vehicles -> {
                    val s by vm.vehicles.collectAsStateWithLifecycle()
                    VehiclesScreen(s, onBack = vm::back, onSelect = vm::selectVehicle)
                }
                Route.Settings -> {
                    val themeMode by vm.theme.collectAsStateWithLifecycle()
                    SettingsScreen(
                        vm.settingsState(pending.size).copy(theme = themeMode),
                        onBack = vm::back,
                        onTheme = vm::setTheme,
                        onSync = vm::syncNow,
                        onLogout = { if (pending.isEmpty()) vm.logout() else vm.dialog.value = DialogState.Logout },
                    )
                }
                Route.Login -> Unit
            }
        }
        TabBar(activeTab, vm::selectTab)
    }

    when (val d = dialog) {
        DialogState.AddReading -> {
            val h by vm.home.collectAsStateWithLifecycle()
            val v = h.vehicle
            if (v != null) {
                AddReadingDialog(
                    unit = v.meterUnit,
                    lastKnown = h.current?.meterValue?.let { app.vectra.core.util.Format.meter(it.canonical, v.meterUnit) },
                    onDismiss = { vm.dialog.value = null },
                    onSave = vm::addReading,
                )
            }
        }
        is DialogState.Pending -> {
            // Den aktuellen Stand des Eintrags zeigen, falls er sich seit dem Öffnen geändert hat.
            val entry = pending.firstOrNull { it.id == d.entry.id }
            if (entry == null) LaunchedEffect(d) { vm.dialog.value = null }
            else PendingDialog(
                entry,
                onDismiss = { vm.dialog.value = null },
                onConfirm = { codes, reason -> vm.confirm(entry, codes, reason) },
                onDiscard = { vm.discard(entry) },
                onRetry = { vm.retry(entry) },
            )
        }
        DialogState.Logout -> ConfirmDialog(
            "Abmelden?",
            "${pending.count { it.status != OutboxStatus.FAILED }} Einträge sind noch nicht übertragen. Beim Abmelden werden sie und alle Daten auf diesem Gerät gelöscht.",
            "Abmelden",
            onDismiss = { vm.dialog.value = null },
            onConfirm = vm::logout,
        )
        null -> Unit
    }
}
