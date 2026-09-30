package app.vectra.android

import android.content.Context
import android.net.Uri
import android.os.Bundle
import android.provider.OpenableColumns
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
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
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import app.vectra.android.feature.AddCostDialog
import app.vectra.android.feature.AddDocumentDialog
import app.vectra.android.feature.AddReadingDialog
import app.vectra.android.feature.AddServiceDialog
import app.vectra.android.feature.CompleteDialog
import app.vectra.android.feature.ConfirmDialog
import app.vectra.android.feature.CostsScreen
import app.vectra.android.feature.DocumentsScreen
import app.vectra.android.feature.FinishTripDialog
import app.vectra.android.feature.MaintenanceScreen
import app.vectra.android.feature.ServiceScreen
import app.vectra.android.feature.StartTripDialog
import app.vectra.android.feature.TripsScreen
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

/** Liest eine vom System gewählte Datei (Name, Typ, Inhalt); große Dateien prüft der Server. */
private fun readPicked(context: Context, uri: Uri): PickedFile? = runCatching {
    val cr = context.contentResolver
    val name = cr.query(uri, arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)?.use { c ->
        if (c.moveToFirst()) c.getString(0) else null
    } ?: "datei"
    val bytes = cr.openInputStream(uri)?.use { it.readBytes() } ?: return null
    PickedFile(name, cr.getType(uri), bytes)
}.getOrNull()

@Composable
private fun AppRoot(vm: MainViewModel) {
    val stack by vm.stack.collectAsStateWithLifecycle()
    val pending by vm.pending.collectAsStateWithLifecycle()
    val dialog by vm.dialog.collectAsStateWithLifecycle()
    val formError by vm.formError.collectAsStateWithLifecycle()
    val busy by vm.busy.collectAsStateWithLifecycle()
    val route = stack.last()
    val context = LocalContext.current
    val picker = rememberLauncherForActivityResult(ActivityResultContracts.GetContent()) { uri ->
        uri?.let { readPicked(context, it) }?.let(vm::onPicked)
    }
    fun pickDocument() { vm.pickPurpose = PickPurpose.Document; picker.launch("*/*") }

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
                            onMaintenance = { vm.selectTab(Tab.Maintenance) },
                            onCosts = { vm.selectTab(Tab.Costs) },
                            onTrips = { vm.selectTab(Tab.Trips) },
                            onDocuments = { vm.open(Route.Documents) },
                        )
                    }
                    Tab.Trips -> {
                        val s by vm.trips.collectAsStateWithLifecycle()
                        TripsScreen(s, onStart = { vm.openDialog(DialogState.StartTrip) }, onFinish = { vm.openDialog(DialogState.FinishTrip(it)) }, onRefresh = vm::syncNow)
                    }
                    Tab.Maintenance -> {
                        val s by vm.maintenance.collectAsStateWithLifecycle()
                        MaintenanceScreen(s, onComplete = { vm.openDialog(DialogState.Complete(it)) }, onService = { vm.open(Route.Service) }, onRefresh = vm::syncNow)
                    }
                    Tab.Costs -> {
                        val s by vm.costs.collectAsStateWithLifecycle()
                        CostsScreen(s, onConfirm = vm::confirmOccurrence, onAdd = { vm.openDialog(DialogState.AddCost) }, onRefresh = vm::syncNow)
                    }
                    Tab.More -> MoreScreen { t ->
                        when (t) {
                            MoreTarget.Vehicles -> vm.open(Route.Vehicles)
                            MoreTarget.Odometer -> vm.open(Route.Odometer)
                            MoreTarget.Service -> vm.open(Route.Service)
                            MoreTarget.Documents -> vm.open(Route.Documents)
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
                    VehiclesScreen(s, onBack = vm::back, onSelect = vm::selectVehicle, onPhoto = { v ->
                        vm.pickPurpose = PickPurpose.VehiclePhoto(v.id)
                        picker.launch("image/*")
                    })
                }
                Route.Service -> {
                    val s by vm.maintenance.collectAsStateWithLifecycle()
                    ServiceScreen(s, onBack = vm::back, onAdd = { vm.openDialog(DialogState.AddService) })
                }
                Route.Documents -> {
                    val s by vm.documents.collectAsStateWithLifecycle()
                    DocumentsScreen(s, onBack = vm::back, onUpload = { pickDocument() })
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
        is DialogState.Complete -> CompleteDialog(d.item, vm.lastKm(), formError, busy, onDismiss = vm::closeDialog, onSave = { km -> vm.complete(d.item, km) })
        DialogState.AddService -> {
            val m by vm.maintenance.collectAsStateWithLifecycle()
            AddServiceDialog(m.vehicle?.defaultCurrency ?: "EUR", vm.lastKm(), m.due, formError, busy, onDismiss = vm::closeDialog, onSave = vm::addService)
        }
        DialogState.AddCost -> {
            val cs by vm.costs.collectAsStateWithLifecycle()
            AddCostDialog(cs.vehicle?.defaultCurrency ?: "EUR", formError, busy, onDismiss = vm::closeDialog, onSave = vm::addCost)
        }
        DialogState.StartTrip -> {
            val t by vm.trips.collectAsStateWithLifecycle()
            StartTripDialog(t.categories, vm.lastKm(), formError, busy, onDismiss = vm::closeDialog, onSave = vm::startTrip)
        }
        is DialogState.FinishTrip -> FinishTripDialog(d.trip, formError, busy, onDismiss = vm::closeDialog, onSave = { km, to, c -> vm.finishTrip(d.trip, km, to, c) })
        is DialogState.AddDocument -> AddDocumentDialog(d.file.name, formError, busy, onDismiss = vm::closeDialog, onSave = { title, type -> vm.createDocument(d.file, title, type) })
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
