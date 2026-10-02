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
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.unit.dp
import app.vectra.android.ui.AppBar
import app.vectra.android.ui.Brand
import app.vectra.android.ui.ButtonKind
import app.vectra.android.ui.ChipKind
import app.vectra.android.ui.Loading
import app.vectra.android.ui.Note
import app.vectra.android.ui.NoteKind
import app.vectra.android.ui.StatusChip
import app.vectra.android.ui.V
import app.vectra.android.ui.VButton
import app.vectra.android.ui.VCard
import app.vectra.android.ui.VField
import app.vectra.android.ui.VIcons
import app.vectra.android.ui.VType
import app.vectra.core.model.Anomaly
import app.vectra.core.model.AssistantMessage
import app.vectra.core.model.Proposal

private val examples = listOf(
    "Ich fahre jetzt los, Kilometerstand 52.340, Kundentermin bei Müller",
    "Bin angekommen, Stand 52.398",
    "Lade die Wartungsintervalle für meinen Skoda Octavia nach",
    "Was ist als Nächstes fällig?",
)

/**
 * Assistent: sprechen oder schreiben, Einträge als Vorschlag bestätigen. Das Mikrofon nutzt die
 * Spracherkennung des Systems (onMic, Android-Teil); Diktier-Apps wie Wispr Flow tippen direkt ins Feld.
 */
@Composable
fun AssistantScreen(
    state: AssistantState,
    onBack: () -> Unit,
    onInput: (String) -> Unit,
    onSend: () -> Unit,
    onMic: (() -> Unit)?,
    onConsent: () -> Unit,
    onNew: () -> Unit,
    onConfirm: (Proposal, reason: String?) -> Unit,
    onReject: (Proposal) -> Unit,
) {
    val c = V.colors
    val s = state.status
    Column(Modifier.fillMaxSize().background(c.bg).imePadding()) {
        AppBar("Assistent", s?.takeIf { it.enabled }?.let { "${it.chatProvider?.name ?: "Claude"}${it.model?.let { m -> " · $m" } ?: ""}" } ?: "Sprechen und eintragen",
            onBack = onBack, action = if (s?.enabled == true && !s.consentRequired) VIcons.plus to "Neue Unterhaltung" else null, onAction = onNew)
        when {
            state.loading && s == null -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) { Loading() }
            s == null || !s.enabled -> Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                state.error?.let { Note(NoteKind.Bad, it) }
                Note(NoteKind.Info, "Der Assistent ist auf dem Server nicht eingerichtet. Ein Administrator aktiviert ihn mit einem Anthropic-API-Schlüssel (siehe deploy/.env.example).")
            }
            s.consentRequired -> ConsentCard(s.chatProvider?.name ?: "Anthropic (Claude)", s.model, s.webSearch, state.error, onConsent)
            else -> {
                val list = rememberLazyListState()
                val count = state.messages.size + (if (state.pendingText != null) 2 else 0)
                LaunchedEffect(count, state.pendingProposals.size) { if (count > 0) list.animateScrollToItem(count - 1) }
                LazyColumn(Modifier.weight(1f).fillMaxWidth(), state = list, contentPadding = PaddingValues(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                    if (state.messages.isEmpty() && state.pendingText == null) {
                        item { Note(NoteKind.Info, "Sag oder schreib, was du eintragen möchtest. Ich bereite es vor, du bestätigst.", icon = VIcons.sparkles) }
                        items(examples) { e ->
                            Text(e, style = VType.small, color = c.text, modifier = Modifier.clip(RoundedCornerShape(14.dp)).background(c.card)
                                .border(1.dp, c.border, RoundedCornerShape(14.dp)).clickable { onInput(e) }.padding(horizontal = 14.dp, vertical = 10.dp))
                        }
                    }
                    items(state.messages, key = { it.id ?: it.hashCode().toString() }) { m -> Bubble(m, state, onConfirm, onReject) }
                    state.pendingText?.let { t ->
                        item { Bubble(AssistantMessage(role = "user", text = t), state, onConfirm, onReject) }
                        item {
                            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                    Box(Modifier.size(8.dp).clip(RoundedCornerShape(4.dp)).background(Brand.teal))
                                    Text(state.progress ?: "Denke nach …", style = VType.small, color = c.muted)
                                }
                                state.pendingProposals.forEach { ProposalCard(it, state, onConfirm, onReject) }
                            }
                        }
                    }
                    state.error?.let { item { Note(NoteKind.Bad, it) } }
                }
                Composer(state.input, state.pendingText != null, onInput, onSend, onMic)
                Text("Antworten erzeugt von ${s.chatProvider?.name ?: "Anthropic (Claude)"} – gespeichert wird nur, was du bestätigst.",
                    style = VType.caption, color = c.muted, modifier = Modifier.padding(start = 16.dp, end = 16.dp, bottom = 8.dp))
            }
        }
    }
}

@Composable
private fun ConsentCard(provider: String, model: String?, webSearch: Boolean, error: String?, onConsent: () -> Unit) {
    Column(Modifier.padding(16.dp)) {
        VCard(Modifier.fillMaxWidth()) {
            Text("Assistent einschalten", style = VType.section, color = V.colors.text)
            Text("Der Assistent nutzt $provider${model?.let { " ($it)" } ?: ""}. Für jede Frage gehen deine Nachricht und die Daten, die er dafür nachschlägt, an diesen Anbieter. " +
                "Einträge speichert er nie selbst: Er bereitet sie vor, du bestätigst." +
                (if (webSearch) " Für Wartungsintervalle darf er im Web nach Herstellerangaben suchen – ohne persönliche Daten." else ""),
                style = VType.small, color = V.colors.muted)
            error?.let { Note(NoteKind.Bad, it) }
            VButton("Zustimmen und starten", onConsent, Modifier.fillMaxWidth(), icon = VIcons.check)
        }
    }
}

@Composable
private fun Bubble(m: AssistantMessage, state: AssistantState, onConfirm: (Proposal, String?) -> Unit, onReject: (Proposal) -> Unit) {
    val c = V.colors
    val mine = m.role == "user"
    Column(Modifier.fillMaxWidth(), horizontalAlignment = if (mine) Alignment.End else Alignment.Start, verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(m.text, style = VType.body, color = if (mine) Brand.onNavy else c.text,
            modifier = Modifier.widthIn(max = 320.dp).clip(RoundedCornerShape(16.dp)).background(if (mine) Brand.navy else c.card)
                .then(if (mine) Modifier else Modifier.border(1.dp, c.border, RoundedCornerShape(16.dp))).padding(horizontal = 14.dp, vertical = 10.dp))
        if (!mine) m.proposals.forEach { ProposalCard(it, state, onConfirm, onReject) }
    }
}

@Composable
private fun ProposalCard(p: Proposal, state: AssistantState, onConfirm: (Proposal, String?) -> Unit, onReject: (Proposal) -> Unit) {
    val an: List<Anomaly> = state.anomalies[p.id].orEmpty()
    var reason by remember(p.id) { mutableStateOf("") }
    val busy = state.busyProposal == p.id
    val (label, kind) = when (p.status) {
        "pending" -> "Bitte bestätigen" to ChipKind.Warn
        "confirmed" -> "Gespeichert" to ChipKind.Ok
        "rejected" -> "Verworfen" to ChipKind.Info
        else -> "Abgelaufen" to ChipKind.Info
    }
    VCard(Modifier.fillMaxWidth()) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Icon(when {
                p.operation.contains("Trip") -> VIcons.route
                p.operation.contains("Maint") -> VIcons.wrench
                p.operation.contains("Cost") -> VIcons.euro
                p.operation.contains("Service") -> VIcons.receipt
                else -> VIcons.gauge
            }, null, tint = V.colors.link, modifier = Modifier.size(22.dp))
            Text(p.summary ?: p.operation, style = VType.bodyStrong, color = V.colors.text, modifier = Modifier.weight(1f))
        }
        StatusChip(label, kind)
        if (p.status == "pending" && an.isNotEmpty()) {
            an.forEach { Note(NoteKind.Warn, it.message ?: it.code) }
            if (an.all { it.confirmable }) VField("Begründung, falls der Wert stimmt", reason, { reason = it }, VIcons.edit, placeholder = "z. B. Tacho getauscht")
            else Text("So lässt sich der Wert nicht speichern – bitte im Chat korrigieren.", style = VType.small, color = V.colors.bad)
        }
        if (p.status == "pending") {
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                VButton("Verwerfen", { onReject(p) }, Modifier.weight(1f), kind = ButtonKind.Outline, enabled = !busy, height = 44.dp)
                VButton(if (an.isEmpty()) "Bestätigen" else "Trotzdem speichern", { onConfirm(p, reason.trim().ifBlank { null }) }, Modifier.weight(1f),
                    enabled = !busy && (an.isEmpty() || (an.all { it.confirmable } && reason.isNotBlank())), loading = busy, height = 44.dp, icon = VIcons.check)
            }
        }
    }
}

@Composable
private fun Composer(text: String, busy: Boolean, onInput: (String) -> Unit, onSend: () -> Unit, onMic: (() -> Unit)?) {
    val c = V.colors
    Row(Modifier.fillMaxWidth().background(c.card).padding(12.dp), verticalAlignment = Alignment.Bottom, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        Box(Modifier.weight(1f).heightIn(min = 48.dp).clip(RoundedCornerShape(12.dp)).background(c.bg).border(1.5.dp, c.border, RoundedCornerShape(12.dp))
            .padding(horizontal = 12.dp, vertical = 12.dp)) {
            if (text.isEmpty()) Text(if (onMic != null) "Schreiben oder Mikrofon antippen …" else "Schreiben oder diktieren …", style = VType.input, color = c.muted)
            BasicTextField(text, onInput, textStyle = VType.input.copy(color = c.text), cursorBrush = SolidColor(c.link), maxLines = 5,
                keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Sentences),
                modifier = Modifier.fillMaxWidth().semantics { contentDescription = "Nachricht an den Assistenten" })
        }
        if (onMic != null) {
            Box(Modifier.size(48.dp).clip(RoundedCornerShape(12.dp)).border(1.5.dp, c.border, RoundedCornerShape(12.dp))
                .clickable(role = Role.Button, enabled = !busy, onClick = onMic), contentAlignment = Alignment.Center) {
                Icon(VIcons.mic, "Diktieren", tint = c.text, modifier = Modifier.size(22.dp))
            }
        }
        Box(Modifier.size(48.dp).clip(RoundedCornerShape(12.dp)).background(if (!busy && text.isNotBlank()) Brand.teal else Brand.teal.copy(alpha = 0.4f))
            .clickable(role = Role.Button, enabled = !busy && text.isNotBlank(), onClick = onSend), contentAlignment = Alignment.Center) {
            Icon(VIcons.send, "Senden", tint = Brand.ink, modifier = Modifier.size(22.dp))
        }
    }
}
