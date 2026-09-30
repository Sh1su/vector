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
import app.vectra.android.feature.HomeScreen
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
import app.vectra.core.model.OdometerReading
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
        "pending-dialog" to { PendingDialog(p1, {}, { _, _ -> }, {}, {}) },
        "vehicles" to { Phone(Tab.More) { VehiclesScreen(VehiclesState(listOf(golf, transit), mapOf("v1" to "143.520 km", "v2" to "88.210 km"), "v1", loading = false), {}, {}) } },
        "more" to { Phone(Tab.More) { MoreScreen {} } },
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

