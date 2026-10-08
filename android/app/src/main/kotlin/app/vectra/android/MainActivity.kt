package app.vectra.android

import android.content.ActivityNotFoundException
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Bundle
import android.provider.OpenableColumns
import android.speech.RecognizerIntent
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
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.width
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import app.vectra.android.data.TachoImage
import app.vectra.android.feature.TachoPurpose
import java.io.File
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import app.vectra.android.ui.Note
import app.vectra.android.ui.NoteKind
import app.vectra.android.ui.VIcons
import kotlinx.coroutines.delay
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import app.vectra.android.feature.AddCostDialog
import app.vectra.android.feature.AddDocumentDialog
import app.vectra.android.feature.AddReadingDialog
import app.vectra.android.feature.AddServiceDialog
import app.vectra.android.feature.ApplyBookDialog
import app.vectra.android.feature.AssistantScreen
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
    // Tachofoto über die Kamera-App; der Pfad überlebt eine Neuerstellung der Activity.
    var tachoPath by rememberSaveable { mutableStateOf<String?>(null) }
    val camera = rememberLauncherForActivityResult(ActivityResultContracts.TakePicture()) { ok ->
        vm.onTachoFile(ok, tachoPath?.let(::File))
        tachoPath = null
    }
    fun takeTacho(purpose: TachoPurpose) {
        val (file, uri) = TachoImage.newTarget(context)
        vm.pendingTacho = purpose
        tachoPath = file.path
        try {
            camera.launch(uri)
        } catch (e: ActivityNotFoundException) {
            vm.pendingTacho = null
            vm.notice.value = "Auf diesem Gerät ist keine Kamera-App verfügbar."
        }
    }
    // Spracherkennung des Systems (offline je nach Gerät); das Ergebnis landet im Eingabefeld des Assistenten.
    val speech = rememberLauncherForActivityResult(ActivityResultContracts.StartActivityForResult()) { r ->
        r.data?.getStringArrayListExtra(RecognizerIntent.EXTRA_RESULTS)?.firstOrNull()?.let(vm::onDictated)
    }
    fun dictate() {
        val intent = Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH)
            .putExtra(RecognizerIntent.EXTRA_LANGUAGE_MODEL, RecognizerIntent.LANGUAGE_MODEL_FREE_FORM)
            .putExtra(RecognizerIntent.EXTRA_LANGUAGE, "de-DE")
            .putExtra(RecognizerIntent.EXTRA_PROMPT, "Was möchtest du eintragen?")
        try {
            speech.launch(intent)
        } catch (e: ActivityNotFoundException) {
            vm.assistantError("Auf diesem Gerät ist keine Spracherkennung verfügbar. Diktier-Apps wie Wispr Flow funktionieren über die Tastatur.")
        }
    }

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
                            onAssistant = vm::openAssistant,
                        )
                    }
                    Tab.Trips -> {
                        val s by vm.trips.collectAsStateWithLifecycle()
                        TripsScreen(s, onStart = { vm.openDialog(DialogState.StartTrip) }, onFinish = { vm.openDialog(DialogState.FinishTrip(it)) }, onRefresh = vm::syncNow,
                            onStartPhoto = { takeTacho(TachoPurpose.Start) }, onFinishPhoto = { takeTacho(TachoPurpose.Finish(it)) })
                    }
                    Tab.Maintenance -> {
                        val s by vm.maintenance.collectAsStateWithLifecycle()
                        MaintenanceScreen(s, onComplete = { vm.openDialog(DialogState.Complete(it)) }, onService = { vm.open(Route.Service) }, onRefresh = vm::syncNow, onBook = vm::openBooks)
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
                            MoreTarget.Assistant -> vm.openAssistant()
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
                Route.Assistant -> {
                    val s by vm.assistant.collectAsStateWithLifecycle()
                    AssistantScreen(
                        s, onBack = vm::back, onInput = vm::assistantInput, onSend = vm::assistantSend, onMic = { dictate() },
                        onConsent = vm::assistantConsent, onNew = vm::assistantNew, onConfirm = vm::confirmProposal, onReject = vm::rejectProposal,
                    )
                }
                Route.Documents -> {
                    val s by vm.documents.collectAsStateWithLifecycle()
                    DocumentsScreen(s, onBack = vm::back, onUpload = { pickDocument() })
                }
                Route.Settings -> {
                    val themeMode by vm.theme.collectAsStateWithLifecycle()
                    val online by vm.online.collectAsStateWithLifecycle()
                    SettingsScreen(
                        vm.settingsState(pending.size).copy(theme = themeMode, online = online),
                        onBack = vm::back,
                        onTheme = vm::setTheme,
                        onSync = vm::syncNow,
                        onLogout = { if (pending.isEmpty()) vm.logout() else vm.dialog.value = DialogState.Logout },
                    )
                }
                Route.Login -> Unit
            }
            val notice by vm.notice.collectAsStateWithLifecycle()
            notice?.let { msg ->
                LaunchedEffect(msg) { delay(4000); vm.dismissNotice() }
                Note(NoteKind.Info, msg, Modifier.align(Alignment.BottomCenter).padding(16.dp), icon = VIcons.sync)
            }
        }
        if (route != Route.Assistant) TabBar(activeTab, vm::selectTab)
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
                onCorrect = { value -> vm.correct(entry, value) },
            )
        }
        is DialogState.Complete -> CompleteDialog(d.item, vm.lastKm(), formError, busy, onDismiss = vm::closeDialog, onSave = { km -> vm.complete(d.item, km) })
        DialogState.AddService -> {
            val m by vm.maintenance.collectAsStateWithLifecycle()
            AddServiceDialog(m.vehicle?.defaultCurrency ?: "EUR", vm.lastKm(), m.due, formError, busy, onDismiss = vm::closeDialog, onSave = vm::addService)
        }
        DialogState.ApplyBook -> {
            val m by vm.maintenance.collectAsStateWithLifecycle()
            ApplyBookDialog(m.books, m.vehicle, vm.lastKm(), formError, busy, onDismiss = vm::closeDialog, onSave = vm::applyBook)
        }
        DialogState.AddCost -> {
            val cs by vm.costs.collectAsStateWithLifecycle()
            AddCostDialog(cs.vehicle?.defaultCurrency ?: "EUR", formError, busy, onDismiss = vm::closeDialog, onSave = vm::addCost)
        }
        DialogState.StartTrip -> {
            val t by vm.trips.collectAsStateWithLifecycle()
            val tacho by vm.tacho.collectAsStateWithLifecycle()
            // Bei neuem Foto den Dialog neu aufbauen, damit Eingaben zur neuen Aufnahme passen.
            key(tacho?.capturedAt) {
                StartTripDialog(t.categories, vm.lastKm(), formError, busy, onDismiss = vm::closeDialog, onSave = vm::startTrip,
                    tacho = tacho, onRetake = { takeTacho(TachoPurpose.Start) })
            }
        }
        is DialogState.FinishTrip -> {
            val tacho by vm.tacho.collectAsStateWithLifecycle()
            key(tacho?.capturedAt) {
                FinishTripDialog(d.trip, formError, busy, onDismiss = vm::closeDialog, onSave = { km, to, c -> vm.finishTrip(d.trip, km, to, c) },
                    tacho = tacho, onRetake = { takeTacho(TachoPurpose.Finish(d.trip)) })
            }
        }
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
