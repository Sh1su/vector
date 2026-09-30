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
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.unit.dp
import app.vectra.android.ui.AppBar
import app.vectra.android.ui.ChipKind
import app.vectra.android.ui.Loading
import app.vectra.android.ui.Note
import app.vectra.android.ui.NoteKind
import app.vectra.android.ui.StatusChip
import app.vectra.android.ui.V
import app.vectra.android.ui.VIcons
import app.vectra.android.ui.VType
import app.vectra.core.model.Vehicle

/** Fahrzeugliste nach dem Entwurf „Fahrzeuge“. Antippen wählt das Fahrzeug für die App. */
@Composable
fun VehiclesScreen(state: VehiclesState, onBack: () -> Unit, onSelect: (Vehicle) -> Unit) {
    val c = V.colors
    Column(Modifier.fillMaxSize().background(c.bg)) {
        val n = state.vehicles.size
        AppBar("Fahrzeuge", if (n == 1) "1 Fahrzeug in deiner Garage" else "$n Fahrzeuge in deiner Garage", onBack = onBack)
        LazyColumn(contentPadding = PaddingValues(16.dp), verticalArrangement = Arrangement.spacedBy(14.dp)) {
            state.error?.let { item { Note(NoteKind.Bad, it) } }
            if (state.loading && n == 0) item { Loading() }
            items(state.vehicles, key = { it.id }) { v -> VehicleCard(v, state.currentKm[v.id], v.id == state.selectedId) { onSelect(v) } }
            item {
                Note(NoteKind.Info, "Fahrzeuge lassen sich teilen. Jede Person bekommt pro Fahrzeug eine Rolle: Eigentümer, Bearbeiter oder Leser. Neue Fahrzeuge legst du in der Web-App an.")
            }
        }
    }
}

@Composable
private fun VehicleCard(v: Vehicle, km: String?, selected: Boolean, onClick: () -> Unit) {
    val c = V.colors
    val shape = RoundedCornerShape(16.dp)
    Column(Modifier.fillMaxWidth().clip(shape).background(c.card).border(if (selected) 2.dp else 1.dp, if (selected) c.link else c.border, shape).clickable(onClick = onClick)) {
        Box(Modifier.fillMaxWidth().height(120.dp).background(if (c.dark) c.border else c.border), contentAlignment = Alignment.Center) {
            Icon(VIcons.car, null, tint = c.muted, modifier = Modifier.size(48.dp))
            Box(Modifier.align(Alignment.TopStart).padding(12.dp)) {
                when {
                    v.status != "active" -> StatusChip(if (v.status == "sold") "Verkauft" else "Stillgelegt", ChipKind.Info)
                    selected -> StatusChip("Ausgewählt", ChipKind.Ok, VIcons.check)
                    else -> StatusChip("Aktiv", ChipKind.Ok, VIcons.check)
                }
            }
        }
        Column(Modifier.padding(horizontal = 16.dp, vertical = 14.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                    Text(v.displayName, style = VType.cardTitle, color = c.text)
                    val sub = v.subtitle()
                    if (sub.isNotBlank()) Text(sub, style = VType.small, color = c.muted)
                }
                Icon(VIcons.right, null, tint = c.muted, modifier = Modifier.size(22.dp))
            }
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Row(Modifier.weight(1f), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    Icon(VIcons.gauge, null, tint = c.link, modifier = Modifier.size(18.dp))
                    Text(km ?: "–", style = VType.small.copy(fontFeatureSettings = "tnum"), color = c.muted)
                }
                Row(Modifier.weight(1f), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    Icon(VIcons.users, null, tint = c.link, modifier = Modifier.size(18.dp))
                    Text(roleLabel(v.myRole), style = VType.small, color = c.muted)
                }
            }
        }
    }
}
