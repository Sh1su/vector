package app.vectra.android.feature

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.unit.dp
import app.vectra.android.ui.AppBar
import app.vectra.android.ui.IconTile
import app.vectra.android.ui.V
import app.vectra.android.ui.VIcons
import app.vectra.android.ui.VType

enum class MoreTarget { Vehicles, Odometer, Settings }

private data class Tile(val icon: ImageVector, val label: String, val desc: String, val target: MoreTarget?)

private val tiles = listOf(
    Tile(VIcons.car, "Fahrzeuge", "Stammdaten, Fotos, Freigaben", MoreTarget.Vehicles),
    Tile(VIcons.gauge, "Kilometerstand", "Verlauf mit Herkunft und Tachofoto", MoreTarget.Odometer),
    Tile(VIcons.fuel, "Kraftstoff", "Tankvorgänge und Verbrauch", null),
    Tile(VIcons.oil, "Öl", "Ölstand, Nachfüllen, Verbrauch", null),
    Tile(VIcons.receipt, "Servicehistorie", "Werkstatt, Teile, Arbeit, Belege", null),
    Tile(VIcons.doc, "Dokumente", "Handbuch, Rechnungen, HU-Berichte", null),
    Tile(VIcons.gear, "Einstellungen", "Konto, Darstellung, Synchronisation", MoreTarget.Settings),
)

/** „Mehr“ nach dem Entwurf: Kacheln in zwei Spalten. Noch nicht umgesetzte Bereiche sind gedimmt. */
@Composable
fun MoreScreen(onOpen: (MoreTarget) -> Unit) {
    val c = V.colors
    Column(Modifier.fillMaxSize().background(c.bg)) {
        AppBar("Mehr", "Alle Bereiche von Vectra")
        Column(Modifier.verticalScroll(rememberScrollState()).padding(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            tiles.chunked(2).forEach { row ->
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    row.forEach { t -> MoreTile(t, Modifier.weight(1f)) { t.target?.let(onOpen) } }
                    if (row.size == 1) Column(Modifier.weight(1f)) {}
                }
            }
        }
    }
}

@Composable
private fun MoreTile(t: Tile, modifier: Modifier, onClick: () -> Unit) {
    val c = V.colors
    val shape = RoundedCornerShape(16.dp)
    var m = modifier.heightIn(min = 104.dp).clip(shape).background(c.card).border(1.dp, c.border, shape)
    if (t.target != null) m = m.clickable(onClick = onClick) else m = m.alpha(0.5f)
    Column(m.padding(12.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
        IconTile(t.icon)
        Text(t.label, style = VType.label.copy(fontSize = VType.button.fontSize), color = c.text)
        Text(if (t.target == null) "Folgt in einer der nächsten Versionen" else t.desc, style = VType.caption, color = c.muted)
    }
}

