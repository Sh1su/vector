package app.vectra.android

import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.runtime.Composable
import androidx.compose.ui.ImageComposeScene
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.toComposeImageBitmap
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.dp
import app.vectra.android.feature.ApplyBookDialog
import app.vectra.android.feature.AssistantScreen
import app.vectra.android.feature.AssistantState
import app.vectra.android.feature.CostsScreen
import app.vectra.android.feature.CostsState
import app.vectra.android.feature.DocumentsScreen
import app.vectra.android.feature.DocumentsState
import app.vectra.android.feature.HomeScreen
import app.vectra.android.feature.MaintenanceScreen
import app.vectra.android.feature.MaintenanceState
import app.vectra.android.feature.ServiceScreen
import app.vectra.android.feature.StartTripDialog
import app.vectra.android.feature.FinishTripDialog
import app.vectra.android.feature.TachoCapture
import app.vectra.android.feature.TachoPhase
import app.vectra.android.feature.TachoPurpose
import app.vectra.core.model.DashboardReading
import app.vectra.android.feature.TripsScreen
import app.vectra.android.feature.TripsState
import app.vectra.android.feature.HomeState
import app.vectra.android.feature.LoginScreen
import app.vectra.android.feature.LoginState
import app.vectra.android.feature.MonthBar
import app.vectra.android.feature.MoreScreen
import app.vectra.android.feature.OdometerScreen
import app.vectra.android.feature.OdometerState
import app.vectra.android.feature.PendingDialog
import app.vectra.android.feature.SettingsScreen
import app.vectra.android.feature.SettingsState
import app.vectra.android.feature.VehiclesScreen
import app.vectra.android.feature.VehiclesState
import app.vectra.android.ui.Tab
import app.vectra.android.ui.TabBar
import app.vectra.android.ui.VectraTheme
import app.vectra.core.model.Anomaly
import app.vectra.core.model.AssistantMessage
import app.vectra.core.model.AssistantStatus
import app.vectra.core.model.ChatProvider
import app.vectra.core.model.CostCurrencyReport
import app.vectra.core.model.Proposal
import app.vectra.core.model.CostEntry
import app.vectra.core.model.CostGroup
import app.vectra.core.model.CostOccurrence
import app.vectra.core.model.CostReport
import app.vectra.core.model.DisplayValue
import app.vectra.core.model.DistanceShare
import app.vectra.core.model.DocumentMeta
import app.vectra.core.model.DueStatus
import app.vectra.core.model.MaintenanceBook
import app.vectra.core.model.MaintenanceBookItem
import app.vectra.core.model.Money
import app.vectra.core.model.OdometerReading
import app.vectra.core.model.QuantityInput
import app.vectra.core.model.ServiceEntry
import app.vectra.core.model.ServiceTotals
import app.vectra.core.model.Trip
import app.vectra.core.model.TripCategory
import app.vectra.core.model.TripReport
import app.vectra.core.model.OdometerValue
import app.vectra.core.model.Quantity
import app.vectra.core.model.Vehicle
import app.vectra.core.odometer.ReadingDraft
import app.vectra.core.outbox.OutboxStatus
import org.jetbrains.skia.EncodedImageFormat
import org.jetbrains.skia.Image as SkImage
import java.io.File
import java.time.Instant

// Rendert die Screens mit Beispieldaten als PNG (390 × 844 dp wie die Entwürfe), zur Sichtprüfung
// ohne Android-Gerät: gradle -Pvectra.uiCheck=true :uicheck:run --args=<Zielordner>

private val brandDir = File(System.getProperty("vectra.brandDir") ?: "../../web/public/brand")
private fun logo(name: String) = SkImage.makeFromEncoded(File(brandDir, name).readBytes()).toComposeImageBitmap()

private val golf = Vehicle("v1", "VW Golf VII", "B-VX 123", "VW", "Golf VII", 2017, myRole = "owner")
private val transit = Vehicle("v2", "Ford Transit Custom", "B-TC 88", "Ford", "Transit Custom", myRole = "editor")

private fun reading(id: String, km: Long, at: String, source: String, status: String = "valid") =
    OdometerReading(id, "v1", at, "Europe/Berlin", Quantity(km * 1000, "m", km.toDouble(), "km"), Quantity(km * 1000, "m"), source, status)

private val readings = listOf(
    reading("r1", 143_520, "2026-09-03T08:10:00Z", "fuel"),
    reading("r2", 143_108, "2026-08-31T15:00:00Z", "trip_end"),
    reading("r3", 142_920, "2026-08-28T07:30:00Z", "manual"),
    reading("r4", 142_825, "2026-08-12T09:00:00Z", "service"),
)
private val current = OdometerValue("2026-09-30T10:00:00Z", "exact", Quantity(143_520_000, "m"), Quantity(143_520_000, "m"))
private val p1 = ReadingDraft.toOutbox("v1", 142_900.0, "km", Instant.parse("2026-09-30T06:00:00Z")).copy(
    status = OutboxStatus.NEEDS_CONFIRMATION,
    problem = """{"type":"https://vectra.app/problems/plausibility","title":"Plausibilität","status":422,"anomalies":[{"code":"P1","confirmable":true,"message":"Der Stand ist kleiner als der vorige Messpunkt (143.520 km)."}]}""",
)
private val offline = ReadingDraft.toOutbox("v1", 143_600.0, "km")

private val books = listOf(
    MaintenanceBook("hyundai-tucson-nx4", "Hyundai", "Tucson", "NX4", "Hyundai Tucson (NX4) – Wartungsplan", "Richtwerte nach dem Hyundai-Wartungsplan; bitte mit dem Serviceheft abgleichen.",
        listOf(MaintenanceBookItem("inspection", "Inspektion", "service", 12, QuantityInput(15000.0, "km")), MaintenanceBookItem("hu", "Hauptuntersuchung (HU/AU)", "legal_inspection", 24, null, 36))),
    MaintenanceBook("leapmotor-b10", "Leapmotor", "B10", null, "Leapmotor B10 – Wartungsplan", "Richtwerte; bitte mit dem Serviceheft abgleichen.",
        listOf(MaintenanceBookItem("inspection", "Inspektion", "service", 24, QuantityInput(30000.0, "km")))),
)

private val due = listOf(
    DueStatus("m1", "v1", "HU/AU", "due", "time", "2026-10-20", 20),
    DueStatus("m2", "v1", "Ölwechsel", "upcoming", "distance", "2027-03-10", 161, distanceRemaining = DisplayValue(1300.0, "km")),
    DueStatus("m3", "v1", "Bremsflüssigkeit", "ok", "time", "2027-06-01", 244),
)
private fun eur(v: Long) = Money(v, "EUR")
private val services = listOf(
    ServiceEntry("s1", 1, "2026-03-10T11:00:00Z", "Europe/Berlin", "inspection", "Inspektion 45.000 km", "EUR", QuantityInput(45000.0, "km"), "Autohaus Muster",
        totals = ServiceTotals(eur(43250), eur(18000), eur(24000), eur(1250))),
    ServiceEntry("s2", 1, "2025-11-02T11:00:00Z", "Europe/Berlin", "repair", "Bremsbeläge vorne", "EUR", QuantityInput(41200.0, "km"), "Eigenleistung",
        totals = ServiceTotals(eur(8990), eur(8990), eur(0), eur(0))),
)
private val cats = listOf(TripCategory("c1", "Privat", "private"), TripCategory("c2", "Geschäftlich", "business", true), TripCategory("c3", "Arbeitsweg", "commute"))
private val tripList = listOf(
    Trip("t2", 1, "2026-09-02T10:00:00Z", "2026-09-02T10:30:00Z", "Europe/Berlin", QuantityInput(143400.0, "km"), QuantityInput(143420.0, "km"), "Büro", "Zuhause",
        categoryId = "c3", distance = DisplayValue(20.0, "km"), gapBefore = DisplayValue(62.0, "km")),
    Trip("t1", 1, "2026-09-01T06:00:00Z", "2026-09-01T06:45:00Z", "Europe/Berlin", QuantityInput(143300.0, "km"), QuantityInput(143338.0, "km"), "Berlin", "Potsdam",
        "Kunde Müller", "c2", distance = DisplayValue(38.0, "km"), startPhotoId = "f1", endPhotoId = "f2"),
)

@Composable
private fun Phone(tab: Tab?, content: @Composable () -> Unit) {
    Column(Modifier.fillMaxSize()) {
        Box(Modifier.weight(1f)) { content() }
        if (tab != null) TabBar(tab) {}
    }
}

fun main(args: Array<String>) {
    val out = File(args.firstOrNull() ?: "build/screens").apply { mkdirs() }
    val screens: Map<String, @Composable () -> Unit> = mapOf(
        "login" to { LoginScreen(LoginState(server = "vectra.example.org", email = "sam@example.org"), { Image(logo("vectra-logo.png"), "Vectra", Modifier.width(236.dp).height(172.dp)) }, {}, {}, {}, {}) },
        "home" to { Phone(Tab.Home) { HomeScreen(HomeState(golf, current, readings.take(3), loading = false), { Image(logo("vectra-schriftzug-negativ.png"), "Vectra", Modifier.width(109.dp).height(26.dp)) }, {}, {}, {}, {}, {}) } },
        "home-attention" to { Phone(Tab.Home) { HomeScreen(HomeState(golf, current, readings.take(2), listOf(p1, offline), loading = false), { Image(logo("vectra-schriftzug-negativ.png"), "Vectra", Modifier.width(109.dp).height(26.dp)) }, {}, {}, {}, {}, {}) } },
        "odometer" to { Phone(Tab.More) { OdometerScreen(OdometerState(golf, current, readings, listOf("Apr", "Mai", "Jun", "Jul", "Aug", "Sep").zip(listOf(910.0, 1240.0, 1480.0, 2210.0, 1030.0, 842.0)).map { MonthBar(it.first, it.second) }, listOf(p1, offline), loading = false), {}, {}, {}) } },
        "pending-dialog" to { PendingDialog(p1, {}, { _, _ -> }, {}, {}, onCorrect = {}) },
        "assistant" to { Phone(null) { AssistantScreen(AssistantState(status = AssistantStatus(true, true, false, ChatProvider("Anthropic (Claude)"), "claude-sonnet-4-5", true),
            conversationId = "c1", loading = false, messages = listOf(
                AssistantMessage("m1", "user", "Ich fahre jetzt los, Kilometerstand 52.340, Kundentermin bei Müller"),
                AssistantMessage("m2", "assistant", "Ich habe die Fahrt bei 52.340 km als Geschäftsfahrt vorbereitet – bitte bestätigen.",
                    listOf(Proposal("p1", "startTrip", "v1", "Fahrt starten bei 52.340 km · Geschäftlich · Kundentermin Müller", status = "pending"))),
                AssistantMessage("m3", "user", "Stand 40.000"),
                AssistantMessage("m4", "assistant", "Bitte bestätigen.", listOf(Proposal("p2", "createOdometerReading", "v1", "Kilometerstand 40.000 km für Octavia", status = "pending"))),
            ), anomalies = mapOf("p2" to listOf(Anomaly("P1", true, "Der Stand ist kleiner als der vorige Messpunkt (52.340 km)."))),
            input = "Bin angekommen, Stand 52.398"), {}, {}, {}, {}, {}, {}, { _, _ -> }, {}) } },
        "assistant-consent" to { Phone(null) { AssistantScreen(AssistantState(status = AssistantStatus(true, false, true, ChatProvider("Anthropic (Claude)"), "claude-sonnet-4-5", true), loading = false),
            {}, {}, {}, null, {}, {}, { _, _ -> }, {}) } },
        "book-dialog" to { ApplyBookDialog(books, golf.copy(make = "Hyundai", model = "Tucson", firstRegistration = "2024-05-15"), 21_800.0, null, false, {}, { _, _, _, _ -> }) },
        "vehicles" to { Phone(Tab.More) { VehiclesScreen(VehiclesState(listOf(golf, transit), mapOf("v1" to "143.520 km", "v2" to "88.210 km"), "v1", loading = false), {}, {}) } },
        "more" to { Phone(Tab.More) { MoreScreen {} } },
        "maintenance" to { Phone(Tab.Maintenance) { MaintenanceScreen(MaintenanceState(golf, due, services, loading = false), {}, {}, {}) } },
        "service" to { Phone(Tab.Maintenance) { ServiceScreen(MaintenanceState(golf, due, services, loading = false), {}, {}) } },
        "costs" to { Phone(Tab.Costs) { CostsScreen(CostsState(golf, 2026, CostReport("2026-01-01", "2026-12-31", DisplayValue(8200.0, "km"), 273,
            listOf(CostCurrencyReport("EUR", 103250, listOf(CostGroup("inspection", 43250), CostGroup("insurance", 60000)), DisplayValue(0.13, "EUR/km"), DisplayValue(3.78, "EUR/Tag")))),
            listOf(CostOccurrence("p1", "2026-04-15", "open", eur(18000), "Kfz-Steuer")), 0,
            listOf(CostEntry("e1", "insurance", "Versicherung 2026", "2026-01-02", eur(60000))), loading = false), {}, {}, {}) } },
        "trips" to { Phone(Tab.Trips) { TripsScreen(TripsState(golf, tripList, cats, TripReport("2026-09-01", "2026-09-30", DisplayValue(58.0, "km"),
            listOf(DistanceShare("c2", "Geschäftlich", DisplayValue(38.0, "km"), 1, 65.5), DistanceShare("c3", "Arbeitsweg", DisplayValue(20.0, "km"), 1, 34.5))), "September", loading = false), {}, {}, {}, {}, {}) } },
        "trips-open" to { Phone(Tab.Trips) { TripsScreen(TripsState(golf, listOf(Trip("t3", 1, "2026-09-21T05:58:00Z", null, "Europe/Berlin", QuantityInput(143612.0, "km"),
            categoryId = "c2", status = "open", startPhotoId = "f1")) + tripList, cats, null, "September", loading = false), {}, {}, {}, {}, {}) } },
        "trip-start" to { StartTripDialog(cats, 143_520.0, null, false, {}, { _, _, _, _, _ -> }) },
        "trip-start-tacho" to { StartTripDialog(cats, 143_520.0, null, false, {}, { _, _, _, _, _ -> }, tacho = TachoCapture(TachoPurpose.Start,
            java.time.Instant.parse("2026-09-21T05:58:00Z"), phase = TachoPhase.Ready, fileId = "f1", reading = DashboardReading("f1", true, QuantityInput(143_612.0, "km"),
                fuelLevelPercent = 75.0, confidence = "medium", notes = "Letzte Ziffer unscharf.", lastOdometer = QuantityInput(143_520.0, "km"),
                summary = "Tank 75 %, Reichweite 520 km, 18,5 °C"), message = "Bitte die Ziffern mit dem Foto vergleichen. Letzte Ziffer unscharf."), onRetake = {}) },
        "trip-finish-tacho" to { FinishTripDialog(tripList.first(), null, false, {}, { _, _, _ -> }, tacho = TachoCapture(TachoPurpose.Finish(tripList.first()),
            java.time.Instant.parse("2026-09-21T07:10:00Z"), phase = TachoPhase.Reading, fileId = "f2"), onRetake = {}) },
        "documents" to { Phone(Tab.More) { DocumentsScreen(DocumentsState(golf, listOf(DocumentMeta("d1", 1, "invoice", "record", "Rechnung Inspektion März", "2026-03-10", "Autohaus Muster", listOf("f1")),
            DocumentMeta("d2", 1, "registration", "other", "Zulassungsbescheinigung Teil I", null, null, listOf("f2", "f3"))), loading = false), {}, {}) } },
        "settings" to { Phone(Tab.More) { SettingsScreen(SettingsState("https://vectra.example.org", "Sam Beispiel", "sam@example.org", pendingCount = 2, clockSkewMinutes = 3, version = "0.1.0"), {}, {}, {}, {}) } },
    )
    for (dark in listOf(false, true)) {
        for ((name, content) in screens) {
            val scene = ImageComposeScene(width = 390 * 2, height = 844 * 2, density = Density(2f)) {
                VectraTheme(dark = dark) { content() }
            }
            val img = scene.render()
            val file = File(out, name + (if (dark) "-dark" else "") + ".png")
            file.writeBytes(img.encodeToData(EncodedImageFormat.PNG)!!.bytes)
            scene.close()
            println(file.path)
        }
    }
}

