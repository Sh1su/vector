package app.vectra.android.feature

import androidx.compose.foundation.layout.Arrangement
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
import app.vectra.core.outbox.OutboxEntry
import app.vectra.core.outbox.OutboxStatus
import app.vectra.core.util.Format

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

/**
 * Befunde bestätigen, Konflikt oder Fehler ansehen (ADR-010, ADR-021). Bestätigen verlangt eine
 * Begründung, sie wird im Audit-Protokoll gespeichert.
 */
@Composable
fun PendingDialog(entry: OutboxEntry, onDismiss: () -> Unit, onConfirm: (codes: List<String>, reason: String) -> Unit, onDiscard: () -> Unit, onRetry: () -> Unit) {
    var reason by remember { mutableStateOf("") }
    val p = entry.problemOrNull()
    Dialog(onDismissRequest = onDismiss) {
        VCard(Modifier.fillMaxWidth()) {
            Text(entry.summary, style = VType.section, color = V.colors.text)
            Text(entry.statusLabel().replaceFirstChar { it.uppercase() }, style = VType.small, color = V.colors.muted)
            when (entry.status) {
                OutboxStatus.NEEDS_CONFIRMATION -> {
                    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        p?.anomalies?.forEach { a -> Note(NoteKind.Warn, a.message ?: a.code) }
                    }
                    VField("Begründung", reason, { reason = it }, VIcons.edit, placeholder = "z. B. Tacho getauscht")
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        VButton("Verwerfen", onDiscard, Modifier.weight(1f), kind = ButtonKind.Outline, height = 48.dp)
                        VButton("Bestätigen", { onConfirm(p?.anomalies?.map { it.code }.orEmpty(), reason) }, Modifier.weight(1f),
                            enabled = reason.isNotBlank(), height = 48.dp)
                    }
                }
                OutboxStatus.CONFLICT, OutboxStatus.FAILED -> {
                    Note(NoteKind.Bad, p?.detail ?: p?.title ?: "Der Server hat den Eintrag nicht angenommen.")
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        VButton("Verwerfen", onDiscard, Modifier.weight(1f), kind = ButtonKind.Outline, height = 48.dp)
                        VButton("Erneut senden", onRetry, Modifier.weight(1f), height = 48.dp)
                    }
                }
                else -> {
                    Text("Der Eintrag wird übertragen, sobald eine Verbindung besteht.", style = VType.small, color = V.colors.muted)
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
