package app.vectra.android.feature

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.ui.Alignment
import androidx.compose.ui.draw.clip
import androidx.compose.ui.semantics.Role
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Dialog
import app.vectra.android.ui.ButtonKind
import app.vectra.android.ui.Note
import app.vectra.android.ui.NoteKind
import app.vectra.android.ui.V
import app.vectra.android.ui.VButton
import app.vectra.android.ui.VCard
import app.vectra.android.ui.VField
import app.vectra.android.ui.VIcons
import app.vectra.android.ui.VType
import app.vectra.core.model.VectraJson
import app.vectra.core.outbox.OutboxEntry
import app.vectra.core.outbox.OutboxStatus
import app.vectra.core.util.Format
import kotlinx.serialization.json.double
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive

/**
 * Stand erfassen. Die Plausibilität prüft allein der Server (ADR-021); der letzte bekannte Stand
 * dient als Orientierung, damit Tippfehler schon beim Erfassen auffallen.
 */
@Composable
fun AddReadingDialog(unit: String, lastKnown: String?, onDismiss: () -> Unit, onSave: (value: Double, note: String) -> Unit) {
    var value by remember { mutableStateOf("") }
    var note by remember { mutableStateOf("") }
    val parsed = Format.parseNumber(value)
    Dialog(onDismissRequest = onDismiss) {
        VCard(Modifier.fillMaxWidth()) {
            Text(if (unit == "h") "Betriebsstunden erfassen" else "Kilometerstand erfassen", style = VType.section, color = V.colors.text)
            if (lastKnown != null) Text("Letzter bekannter Stand: $lastKnown", style = VType.small, color = V.colors.muted)
            VField("Stand", value, { value = it }, VIcons.gauge, placeholder = "z. B. 143.520", keyboardType = KeyboardType.Decimal,
                trailing = unit, error = if (value.isNotBlank() && parsed == null) "Bitte eine Zahl eingeben." else null)
            VField("Notiz (optional)", note, { note = it }, VIcons.edit)
            Text("Zeitpunkt: jetzt. Ohne Netz wird der Eintrag gespeichert und später übertragen.", style = VType.caption, color = V.colors.muted)
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                VButton("Abbrechen", onDismiss, Modifier.weight(1f), kind = ButtonKind.Outline, height = 48.dp)
                VButton("Speichern", { parsed?.let { onSave(it, note) } }, Modifier.weight(1f), enabled = parsed != null, height = 48.dp)
            }
        }
    }
}

/** Zahlenwert eines Kilometerstand-Eintrags aus dem Outbox-Body (für die Korrektur vorbelegt). */
fun OutboxEntry.readingValue(): Double? = runCatching {
    VectraJson.parseToJsonElement(body).jsonObject["value"]!!.jsonObject["value"]!!.jsonPrimitive.double
}.getOrNull()

private enum class Answer { Confirm, Correct }

/**
 * Hinweis-Box bei Befunden, Konflikten oder Fehlern eines Eintrags aus der Outbox (ADR-010, ADR-021).
 * Öffnet sich von selbst, sobald die Synchronisierung eine Rückfrage hat. Der Nutzer antwortet:
 * Wert stimmt (mit Begründung), Wert korrigieren (neuer Wert wird neu geprüft) oder verwerfen.
 */
@Composable
fun PendingDialog(
    entry: OutboxEntry,
    onDismiss: () -> Unit,
    onConfirm: (codes: List<String>, reason: String) -> Unit,
    onDiscard: () -> Unit,
    onRetry: () -> Unit,
    onCorrect: ((value: Double) -> Unit)? = null,
) {
    val p = entry.problemOrNull()
    val canCorrect = onCorrect != null && entry.operation == "createOdometerReading"
    val confirmable = entry.status == OutboxStatus.NEEDS_CONFIRMATION
    var answer by remember(entry.id) { mutableStateOf(if (confirmable) Answer.Confirm else Answer.Correct) }
    var reason by remember(entry.id) { mutableStateOf("") }
    var corrected by remember(entry.id) { mutableStateOf(entry.readingValue()?.let { Format.number(it, 1).replace(".", "") } ?: "") }
    val parsed = Format.parseNumber(corrected)
    val c = V.colors
    Dialog(onDismissRequest = onDismiss) {
        VCard(Modifier.fillMaxWidth()) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                Icon(VIcons.alert, null, tint = if (confirmable) c.warn else c.bad, modifier = Modifier.size(24.dp))
                Text(
                    when (entry.status) {
                        OutboxStatus.NEEDS_CONFIRMATION -> "Bitte prüfen"
                        OutboxStatus.CONFLICT -> "Konflikt beim Synchronisieren"
                        OutboxStatus.FAILED -> "Nicht übernommen"
                        else -> "Noch nicht übertragen"
                    },
                    style = VType.section, color = c.text,
                )
            }
            Text(entry.summary, style = VType.bodyStrong, color = c.text)
            when (entry.status) {
                OutboxStatus.NEEDS_CONFIRMATION, OutboxStatus.CONFLICT, OutboxStatus.FAILED -> {
                    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        val msgs = p?.anomalies?.map { it.message ?: it.code }.orEmpty().ifEmpty {
                            listOf(p?.detail ?: p?.title ?: if (entry.status == OutboxStatus.CONFLICT) "Auf dem Server gibt es inzwischen einen anderen Stand." else "Der Server hat den Eintrag nicht angenommen.")
                        }
                        msgs.forEach { Note(if (confirmable) NoteKind.Warn else NoteKind.Bad, it) }
                    }
                    Text("Was stimmt?", style = VType.label, color = c.text)
                    if (confirmable) Choice("Der Wert stimmt so", answer == Answer.Confirm) { answer = Answer.Confirm }
                    if (canCorrect) Choice("Ich habe mich vertippt – Wert korrigieren", answer == Answer.Correct) { answer = Answer.Correct }
                    if (answer == Answer.Confirm && confirmable) {
                        VField("Begründung", reason, { reason = it }, VIcons.edit, placeholder = "z. B. Tacho getauscht")
                    }
                    if (answer == Answer.Correct && canCorrect) {
                        VField("Richtiger Stand", corrected, { corrected = it }, VIcons.gauge, keyboardType = KeyboardType.Decimal, trailing = "km",
                            error = if (corrected.isNotBlank() && parsed == null) "Bitte eine Zahl eingeben." else null)
                    }
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        VButton("Verwerfen", onDiscard, Modifier.weight(1f), kind = ButtonKind.Outline, height = 48.dp)
                        when {
                            answer == Answer.Confirm && confirmable ->
                                VButton("Bestätigen", { onConfirm(p?.anomalies?.map { it.code }.orEmpty(), reason) }, Modifier.weight(1f), enabled = reason.isNotBlank(), height = 48.dp)
                            answer == Answer.Correct && canCorrect ->
                                VButton("Korrigiert senden", { parsed?.let { onCorrect!!(it) } }, Modifier.weight(1f), enabled = parsed != null, height = 48.dp)
                            else -> VButton("Erneut senden", onRetry, Modifier.weight(1f), height = 48.dp)
                        }
                    }
                    VButton("Später entscheiden", onDismiss, Modifier.fillMaxWidth(), kind = ButtonKind.Outline, height = 44.dp)
                }
                else -> {
                    Text("Der Eintrag wird übertragen, sobald eine Verbindung besteht.", style = VType.small, color = c.muted)
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        VButton("Verwerfen", onDiscard, Modifier.weight(1f), kind = ButtonKind.Outline, height = 48.dp)
                        VButton("Schließen", onDismiss, Modifier.weight(1f), height = 48.dp)
                    }
                }
            }
        }
    }
}

@Composable
private fun Choice(text: String, selected: Boolean, onClick: () -> Unit) {
    val c = V.colors
    Row(
        Modifier.fillMaxWidth().clip(RoundedCornerShape(12.dp)).background(if (selected) c.infoBg else c.soft)
            .border(1.5.dp, if (selected) c.link else c.border, RoundedCornerShape(12.dp))
            .clickable(role = Role.RadioButton, onClick = onClick).padding(horizontal = 14.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Box(Modifier.size(18.dp).clip(RoundedCornerShape(9.dp)).border(2.dp, if (selected) c.link else c.muted, RoundedCornerShape(9.dp)), contentAlignment = Alignment.Center) {
            if (selected) Box(Modifier.size(8.dp).clip(RoundedCornerShape(4.dp)).background(c.link))
        }
        Text(text, style = VType.small, color = c.text)
    }
}

@Composable
fun ConfirmDialog(title: String, text: String, confirm: String, onDismiss: () -> Unit, onConfirm: () -> Unit) {
    Dialog(onDismissRequest = onDismiss) {
        VCard(Modifier.fillMaxWidth().padding(0.dp)) {
            Text(title, style = VType.section, color = V.colors.text)
            Text(text, style = VType.small, color = V.colors.muted)
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                VButton("Abbrechen", onDismiss, Modifier.weight(1f), kind = ButtonKind.Outline, height = 48.dp)
                VButton(confirm, onConfirm, Modifier.weight(1f), kind = ButtonKind.Navy, height = 48.dp)
            }
        }
    }
}
