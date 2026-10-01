package app.vectra.android.feature

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.dp
import app.vectra.android.ui.AppBar
import app.vectra.android.ui.ButtonKind
import app.vectra.android.ui.Note
import app.vectra.android.ui.NoteKind
import app.vectra.android.ui.SectionTitle
import app.vectra.android.ui.V
import app.vectra.android.ui.VButton
import app.vectra.android.ui.VCard
import app.vectra.android.ui.VIcons
import app.vectra.android.ui.VType

@Composable
fun SettingsScreen(state: SettingsState, onBack: () -> Unit, onTheme: (ThemeMode) -> Unit, onSync: () -> Unit, onLogout: () -> Unit) {
    val c = V.colors
    Column(Modifier.fillMaxSize().background(c.bg)) {
        AppBar("Einstellungen", state.accountEmail, onBack = onBack)
        Column(Modifier.verticalScroll(rememberScrollState()).padding(16.dp), verticalArrangement = Arrangement.spacedBy(16.dp)) {
            VCard(Modifier.fillMaxWidth()) {
                SectionTitle("Konto")
                KeyValue("Name", state.accountName)
                KeyValue("E-Mail", state.accountEmail)
                KeyValue("Server", state.server)
                Text("Du bleibst angemeldet – bis zu einem Jahr, solange du die App mindestens alle 90 Tage öffnest. Geräte beendest du in der Web-App unter Einstellungen.",
                    style = VType.caption, color = c.muted)
            }
            VCard(Modifier.fillMaxWidth()) {
                SectionTitle("Darstellung")
                Row(Modifier.fillMaxWidth().clip(RoundedCornerShape(12.dp)).background(c.soft).padding(4.dp), horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                    ThemeMode.entries.forEach { m ->
                        val on = m == state.theme
                        Box(
                            Modifier.weight(1f).height(40.dp).clip(RoundedCornerShape(10.dp)).background(if (on) c.card else Color.Transparent)
                                .then(if (on) Modifier.border(1.dp, c.border, RoundedCornerShape(10.dp)) else Modifier)
                                .clickable(role = Role.RadioButton) { onTheme(m) },
                            contentAlignment = Alignment.Center,
                        ) { Text(m.label, style = VType.label, color = if (on) c.text else c.muted) }
                    }
                }
            }
            VCard(Modifier.fillMaxWidth()) {
                SectionTitle("Synchronisation")
                KeyValue("Verbindung", if (state.online) "online" else "offline")
                Text(
                    if (state.pendingCount == 0) "Alle Einträge sind übertragen." else "${state.pendingCount} Einträge warten auf Übertragung.",
                    style = VType.small, color = c.muted,
                )
                Text("Sobald wieder eine Verbindung besteht, überträgt die App automatisch. Rückfragen (z. B. ein ungewöhnlicher Kilometerstand) erscheinen als Hinweis zum Beantworten.",
                    style = VType.caption, color = c.muted)
                if (kotlin.math.abs(state.clockSkewMinutes) >= 2) {
                    Note(NoteKind.Warn, "Die Uhr dieses Geräts weicht um ${kotlin.math.abs(state.clockSkewMinutes)} Minuten von der Serverzeit ab. Einträge mit Zeitpunkt in der Zukunft lehnt der Server ab.")
                }
                VButton("Jetzt synchronisieren", onSync, Modifier.fillMaxWidth(), kind = ButtonKind.Outline, icon = VIcons.sync, height = 48.dp)
            }
            VButton("Abmelden", onLogout, Modifier.fillMaxWidth(), kind = ButtonKind.Navy, icon = VIcons.logout)
            Text("Vectra für Android ${state.version} · Schriften Poppins und Inter unter SIL Open Font License 1.1", style = VType.caption, color = c.muted)
        }
    }
}

@Composable
private fun KeyValue(k: String, v: String) {
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
        Text(k, style = VType.small, color = V.colors.muted, modifier = Modifier.weight(0.35f))
        Text(v, style = VType.small.copy(fontWeight = VType.label.fontWeight), color = V.colors.text, modifier = Modifier.weight(0.65f))
    }
}

/** Platzhalter für Bereiche späterer Iterationen (Fahrten, Wartung, Kosten). */
@Composable
fun ComingSoonScreen(title: String, text: String) {
    Column(Modifier.fillMaxSize().background(V.colors.bg)) {
        AppBar(title, "Folgt in einer der nächsten Versionen")
        Column(Modifier.padding(16.dp)) { Note(NoteKind.Info, text) }
    }
}
