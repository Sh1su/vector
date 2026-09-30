package app.vectra.android.feature

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import app.vectra.android.ui.BarButton
import app.vectra.android.ui.Brand
import app.vectra.android.ui.ButtonKind
import app.vectra.android.ui.ListRow
import app.vectra.android.ui.Loading
import app.vectra.android.ui.Note
import app.vectra.android.ui.NoteAction
import app.vectra.android.ui.NoteKind
import app.vectra.android.ui.SectionTitle
import app.vectra.android.ui.V
import app.vectra.android.ui.VButton
import app.vectra.android.ui.VIcons
import app.vectra.android.ui.VType
import app.vectra.core.util.Format

/**
 * Übersicht nach dem Entwurf „Dashboard“. Kennzahlen, die erst mit späteren Modulen kommen
 * (Service, Verbrauch, Öl), zeigen „–“ statt erfundener Werte.
 */
@Composable
fun HomeScreen(
    state: HomeState,
    wordmark: @Composable () -> Unit,
    onPickVehicle: () -> Unit,
    onOdometer: () -> Unit,
    onAddReading: () -> Unit,
    onOpenPending: () -> Unit,
    onRefresh: () -> Unit,
    onMaintenance: () -> Unit = {},
    onCosts: () -> Unit = {},
    onTrips: () -> Unit = {},
    onDocuments: () -> Unit = {},
) {
    val c = V.colors
    val v = state.vehicle
    LazyColumn(Modifier.fillMaxSize().background(c.bg), contentPadding = PaddingValues(bottom = 16.dp)) {
        item {
            Column(
                Modifier.fillMaxWidth().background(Brand.navy).statusBarsPadding().padding(start = 16.dp, end = 16.dp, top = 14.dp, bottom = 18.dp),
                verticalArrangement = Arrangement.spacedBy(14.dp),
            ) {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Box(Modifier.weight(1f)) { wordmark() }
                    Row(
                        Modifier.height(44.dp).clip(RoundedCornerShape(12.dp)).border(1.dp, Brand.onNavyLine, RoundedCornerShape(12.dp))
                            .clickable(role = Role.Button, onClick = onPickVehicle).padding(horizontal = 12.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(6.dp),
                    ) {
                        Icon(VIcons.car, null, tint = Brand.onNavy, modifier = Modifier.size(20.dp))
                        Text(v?.displayName ?: "Fahrzeug wählen", style = VType.label.copy(fontWeight = VType.body.fontWeight), color = Brand.onNavy,
                            maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f, fill = false))
                        Icon(VIcons.down, null, tint = Brand.onNavy, modifier = Modifier.size(16.dp))
                    }
                    BarButton(VIcons.sync, "Aktualisieren", Brand.onNavySurface, Brand.onNavy, onRefresh)
                }
                val attention = state.attention.size
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    Box(Modifier.size(32.dp).clip(CircleShape).background(if (attention == 0) Brand.ready else Brand.amber), contentAlignment = Alignment.Center) {
                        Icon(if (attention == 0) VIcons.check else VIcons.alert, null, tint = Brand.ink, modifier = Modifier.size(18.dp))
                    }
                    Text(
                        when {
                            v == null -> "Lege dein erstes Fahrzeug im Web an."
                            attention == 1 -> "1 Eintrag braucht deine Bestätigung."
                            attention > 1 -> "$attention Einträge brauchen deine Bestätigung."
                            else -> "Dein Auto ist bereit für die nächste Fahrt."
                        },
                        style = VType.hero, color = Brand.onNavy,
                    )
                }
                val km = state.current?.meterValue?.let { Format.meter(it.canonical, v?.meterUnit ?: "km") } ?: "–"
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    HeroTile(VIcons.gauge, if (v?.meterUnit == "h") "Betriebsstunden" else "Kilometerstand", km, Modifier.weight(1f), onClick = onOdometer)
                    HeroTile(VIcons.wrench, state.nextDue?.title ?: "Nächste Wartung", state.nextDue?.let { levelLabel(it.level) } ?: "–", Modifier.weight(1f), onClick = onMaintenance)
                }
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    HeroTile(VIcons.euro, "Kosten dieses Jahr", state.yearCosts ?: "–", Modifier.weight(1f), onClick = onCosts)
                    HeroTile(VIcons.route, "Fahrten", "Starten", Modifier.weight(1f), onClick = onTrips)
                }
            }
        }
        item {
            Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(16.dp)) {
                state.photo?.let { p ->
                    androidx.compose.foundation.Image(p, v?.displayName, Modifier.fillMaxWidth().height(160.dp).clip(RoundedCornerShape(16.dp)),
                        contentScale = androidx.compose.ui.layout.ContentScale.Crop)
                }
                if (state.freshness.offline) {
                    Note(NoteKind.Soft, "Offline – angezeigt wird der zuletzt geladene Stand" + (state.freshness.syncedAt?.let { " vom $it" } ?: "") + ".", icon = VIcons.cloudoff)
                }
                state.error?.let { Note(NoteKind.Bad, it) }
                if (state.attention.isNotEmpty()) {
                    Note(NoteKind.Warn, "Nicht alle Einträge konnten übertragen werden.") {
                        NoteAction("Ansehen", c.warn, onOpenPending)
                    }
                }
                Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                    SectionTitle("Schnell erfassen")
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        VButton("Kilometerstand", onAddReading, Modifier.weight(1f), icon = VIcons.gauge, enabled = v?.canEdit == true)
                        VButton("Verlauf", onOdometer, Modifier.weight(1f), kind = ButtonKind.Outline, icon = VIcons.clock, enabled = v != null)
                    }
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        QuickTile(VIcons.route, "Fahrt", Modifier.weight(1f), if (v != null) onTrips else null)
                        QuickTile(VIcons.wrench, "Wartung", Modifier.weight(1f), if (v != null) onMaintenance else null)
                        QuickTile(VIcons.euro, "Kosten", Modifier.weight(1f), if (v != null) onCosts else null)
                        QuickTile(VIcons.scan, "Beleg", Modifier.weight(1f), if (v?.canEdit == true) onDocuments else null)
                    }
                }
                SectionTitle("Zuletzt erfasst") {
                    Text("Verlauf", style = VType.label, color = c.link, modifier = Modifier.clip(RoundedCornerShape(6.dp)).clickable(onClick = onOdometer).padding(4.dp))
                }
                if (state.loading && state.recent.isEmpty()) Loading()
            }
        }
        items(state.pending, key = { "p" + it.id }) { e ->
            Box(Modifier.padding(horizontal = 16.dp, vertical = 4.dp)) {
                ListRow(VIcons.sync, e.summary, e.statusLabel(), onClick = onOpenPending)
            }
        }
        items(state.recent, key = { it.id }) { r ->
            Box(Modifier.padding(horizontal = 16.dp, vertical = 4.dp)) {
                ListRow(sourceIcon(r.source), r.meterText(v?.meterUnit ?: "km"), r.metaText(), onClick = onOdometer)
            }
        }
        if (!state.loading && state.recent.isEmpty() && state.pending.isEmpty() && v != null) {
            item { Box(Modifier.padding(horizontal = 16.dp)) { Note(NoteKind.Info, "Noch keine Messpunkte. Erfasse den aktuellen Stand, damit Vectra Strecken berechnen kann.") } }
        }
    }
}

@Composable
private fun HeroTile(icon: ImageVector, label: String, value: String, modifier: Modifier, onClick: (() -> Unit)? = null) {
    var m = modifier.heightIn(min = 76.dp).clip(RoundedCornerShape(16.dp)).background(Brand.onNavySurface)
    if (onClick != null) m = m.clickable(role = Role.Button, onClick = onClick)
    Column(m.padding(12.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
            Icon(icon, null, tint = Brand.tealSoft, modifier = Modifier.size(18.dp))
            Text(label, style = VType.small, color = Brand.onNavyMuted, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
        Text(value, style = VType.figure, color = if (value == "–") Brand.onNavyMuted else Brand.onNavy, maxLines = 1)
    }
}

@Composable
private fun QuickTile(icon: ImageVector, label: String, modifier: Modifier, onClick: (() -> Unit)?) {
    val c = V.colors
    val shape = RoundedCornerShape(14.dp)
    var m = modifier.height(64.dp).clip(shape).background(c.card).border(1.dp, c.border, shape)
    if (onClick != null) m = m.clickable(role = Role.Button, onClick = onClick)
    val alpha = if (onClick != null) 1f else 0.45f
    Column(m, horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(4.dp, Alignment.CenterVertically)) {
        Icon(icon, null, tint = c.link.copy(alpha = alpha), modifier = Modifier.size(22.dp))
        Text(label, style = VType.caption.copy(fontWeight = VType.label.fontWeight), color = c.text.copy(alpha = alpha))
    }
}

