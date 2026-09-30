package app.vectra.android.feature

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Dialog
import app.vectra.android.ui.ButtonKind
import app.vectra.android.ui.ChipKind
import app.vectra.android.ui.Note
import app.vectra.android.ui.NoteKind
import app.vectra.android.ui.StatusChip
import app.vectra.android.ui.V
import app.vectra.android.ui.VButton
import app.vectra.android.ui.VCard
import app.vectra.android.ui.VField
import app.vectra.android.ui.VIcons
import app.vectra.android.ui.VType
import app.vectra.core.model.DueStatus
import app.vectra.core.model.Trip
import app.vectra.core.model.TripCategory
import app.vectra.core.util.Format
import app.vectra.core.util.MoneyFormat

/** Rahmen der Erfassungsdialoge: Titel, Inhalt, Fehler bzw. Befunde, Knöpfe. */
@Composable
private fun FormDialog(
    title: String,
    error: FormError?,
    busy: Boolean,
    canSave: Boolean,
    onDismiss: () -> Unit,
    onSave: (Confirmation?) -> Unit,
    content: @Composable () -> Unit,
) {
    var reason by remember(error) { mutableStateOf("") }
    Dialog(onDismissRequest = onDismiss) {
        VCard(Modifier.fillMaxWidth()) {
            Column(Modifier.heightIn(max = 560.dp).verticalScroll(rememberScrollState()), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Text(title, style = VType.section, color = V.colors.text)
                content()
                if (error != null) {
                    if (error.confirmable) {
                        error.anomalies.forEach { Note(NoteKind.Warn, it.message ?: it.code) }
                        VField("Begründung, falls der Wert trotzdem stimmt", reason, { reason = it }, VIcons.edit, placeholder = "z. B. Tacho getauscht")
                    } else {
                        Note(NoteKind.Bad, error.message)
                    }
                }
            }
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                VButton("Abbrechen", onDismiss, Modifier.weight(1f), kind = ButtonKind.Outline, height = 48.dp)
                if (error != null && error.confirmable) {
                    val codes = error.anomalies.map { it.code }
                    VButton("Trotzdem speichern", { onSave(Confirmation(codes, reason.trim())) }, Modifier.weight(1f),
                        kind = ButtonKind.Navy, enabled = reason.isNotBlank() && canSave, loading = busy, height = 48.dp)
                } else {
                    VButton("Speichern", { onSave(null) }, Modifier.weight(1f), enabled = canSave, loading = busy, height = 48.dp)
                }
            }
        }
    }
}

/** Auswahl als Chips (Kategorie, Art, Typ). */
@Composable
private fun ChoiceRow(label: String, options: List<Pair<String, String>>, selected: String, onSelect: (String) -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
        Text(label, style = VType.label, color = V.colors.text)
        options.chunked(3).forEach { row ->
            Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                row.forEach { (k, l) ->
                    Row(Modifier.clip(RoundedCornerShape(14.dp)).clickable(role = Role.RadioButton) { onSelect(k) }, verticalAlignment = Alignment.CenterVertically) {
                        StatusChip(l, if (k == selected) ChipKind.Ok else ChipKind.Info, if (k == selected) VIcons.check else null)
                    }
                }
            }
        }
    }
}

/** Wartung als erledigt markieren (ohne Serviceeintrag). */
@Composable
fun CompleteDialog(item: DueStatus, lastKm: Double?, error: FormError?, busy: Boolean, onDismiss: () -> Unit, onSave: (km: Double?) -> Unit) {
    var km by remember { mutableStateOf(lastKm?.let { Format.number(it, 0).replace(".", "") } ?: "") }
    val parsed = Format.parseNumber(km)
    FormDialog("${item.title} erledigt", error, busy, km.isBlank() || parsed != null, onDismiss, { onSave(parsed) }) {
        Text("Datum: heute. Für Werkstattbesuche mit Rechnung besser einen Serviceeintrag anlegen – dort lässt sich die Wartung mit abhaken.", style = VType.small, color = V.colors.muted)
        VField("Kilometerstand (optional)", km, { km = it }, VIcons.gauge, keyboardType = KeyboardType.Decimal, trailing = "km",
            error = if (km.isNotBlank() && parsed == null) "Bitte eine Zahl eingeben." else null)
    }
}

/** Werkstattbesuch erfassen: Kostenpositionen Teile/Arbeit/Sonstiges, erledigte Wartungen. */
@Composable
fun AddServiceDialog(currency: String, lastKm: Double?, due: List<DueStatus>, error: FormError?, busy: Boolean, onDismiss: () -> Unit,
    onSave: (title: String, kind: String, km: Double?, parts: Long, labor: Long, other: Long, completes: List<String>, confirm: Confirmation?) -> Unit) {
    var title by remember { mutableStateOf("") }
    var kind by remember { mutableStateOf("maintenance") }
    var km by remember { mutableStateOf(lastKm?.let { Format.number(it, 0).replace(".", "") } ?: "") }
    var parts by remember { mutableStateOf("") }
    var labor by remember { mutableStateOf("") }
    var other by remember { mutableStateOf("") }
    var completes by remember { mutableStateOf(setOf<String>()) }
    val money = { s: String -> if (s.isBlank()) 0L else MoneyFormat.parse(s, currency) }
    val valid = title.isNotBlank() && Format.parseNumber(km) != null && listOf(parts, labor, other).all { money(it) != null }
    FormDialog("Serviceeintrag erfassen", error, busy, valid, onDismiss, { c ->
        onSave(title.trim(), kind, Format.parseNumber(km), money(parts) ?: 0L, money(labor) ?: 0L, money(other) ?: 0L, completes.toList(), c)
    }) {
        VField("Titel", title, { title = it }, VIcons.receipt, placeholder = "z. B. Inspektion 60.000 km")
        ChoiceRow("Art", listOf("maintenance" to "Wartung", "inspection" to "Inspektion", "repair" to "Reparatur", "upgrade" to "Nachrüstung"), kind) { kind = it }
        VField("Kilometerstand", km, { km = it }, VIcons.gauge, keyboardType = KeyboardType.Decimal, trailing = "km")
        VField("Teile", parts, { parts = it }, VIcons.euro, placeholder = "0,00", keyboardType = KeyboardType.Decimal, trailing = currency)
        VField("Arbeit", labor, { labor = it }, VIcons.euro, placeholder = "0,00", keyboardType = KeyboardType.Decimal, trailing = currency)
        VField("Sonstiges", other, { other = it }, VIcons.euro, placeholder = "0,00", keyboardType = KeyboardType.Decimal, trailing = currency)
        val open = due.filter { it.level != "completed" }
        if (open.isNotEmpty()) {
            Text("Erledigte Wartungen", style = VType.label, color = V.colors.text)
            open.forEach { d ->
                val on = d.itemId in completes
                Row(Modifier.fillMaxWidth().clip(RoundedCornerShape(10.dp)).clickable(role = Role.Checkbox) { completes = if (on) completes - d.itemId else completes + d.itemId }.padding(vertical = 4.dp),
                    verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    StatusChip(if (on) "erledigt" else "offen", if (on) ChipKind.Ok else ChipKind.Info, if (on) VIcons.check else null)
                    Text(d.title, style = VType.small, color = V.colors.text)
                }
            }
        }
    }
}

/** Sonstige Kosten erfassen (Datum: heute). */
@Composable
fun AddCostDialog(currency: String, error: FormError?, busy: Boolean, onDismiss: () -> Unit, onSave: (category: String, title: String, amountMinor: Long) -> Unit) {
    var category by remember { mutableStateOf("parking") }
    var title by remember { mutableStateOf("") }
    var amount by remember { mutableStateOf("") }
    val parsed = MoneyFormat.parse(amount, currency)
    FormDialog("Kosten erfassen", error, busy, parsed != null, onDismiss, { onSave(category, title.ifBlank { costCategoryLabel(category) }, parsed ?: 0L) }) {
        ChoiceRow("Kategorie", costCategories, category) { category = it }
        VField("Bezeichnung", title, { title = it }, VIcons.receipt, placeholder = costCategoryLabel(category))
        VField("Betrag", amount, { amount = it }, VIcons.euro, placeholder = "0,00", keyboardType = KeyboardType.Decimal, trailing = currency,
            error = if (amount.isNotBlank() && parsed == null) "Bitte einen Betrag eingeben." else null)
        Text("Datum: heute. Wiederkehrende Kosten wie die Kfz-Steuer legst du in der Web-App an.", style = VType.caption, color = V.colors.muted)
    }
}

/** Fahrt starten: Startstand (vorbelegt mit dem letzten Stand), Kategorie, Zweck. */
@Composable
fun StartTripDialog(categories: List<TripCategory>, lastKm: Double?, error: FormError?, busy: Boolean, onDismiss: () -> Unit,
    onSave: (km: Double, categoryId: String, purpose: String?, from: String?, confirm: Confirmation?) -> Unit) {
    val active = categories.filter { it.active }
    var km by remember { mutableStateOf(lastKm?.let { Format.number(it, 0).replace(".", "") } ?: "") }
    var cat by remember { mutableStateOf(active.firstOrNull()?.id ?: "") }
    var purpose by remember { mutableStateOf("") }
    var from by remember { mutableStateOf("") }
    val parsed = Format.parseNumber(km)
    val needsPurpose = active.firstOrNull { it.id == cat }?.purposeRequired == true
    FormDialog("Fahrt starten", error, busy, parsed != null && cat.isNotBlank() && (!needsPurpose || purpose.isNotBlank()), onDismiss, { c ->
        onSave(parsed ?: 0.0, cat, purpose.ifBlank { null }, from.ifBlank { null }, c)
    }) {
        VField("Startstand", km, { km = it }, VIcons.gauge, keyboardType = KeyboardType.Decimal, trailing = "km")
        ChoiceRow("Kategorie", active.map { it.id to it.name }, cat) { cat = it }
        VField(if (needsPurpose) "Zweck (Pflicht)" else "Zweck (optional)", purpose, { purpose = it }, VIcons.edit)
        VField("Start (optional)", from, { from = it }, VIcons.route)
    }
}

/** Laufende Fahrt beenden: Endstand und Ziel. */
@Composable
fun FinishTripDialog(trip: Trip, error: FormError?, busy: Boolean, onDismiss: () -> Unit, onSave: (km: Double, to: String?, confirm: Confirmation?) -> Unit) {
    var km by remember { mutableStateOf("") }
    var to by remember { mutableStateOf("") }
    val parsed = Format.parseNumber(km)
    val dist = parsed?.let { it - trip.startOdometer.value }
    FormDialog("Fahrt beenden", error, busy, parsed != null, onDismiss, { c -> onSave(parsed ?: 0.0, to.ifBlank { null }, c) }) {
        Text("Start bei " + Format.number(trip.startOdometer.value, 0) + " km", style = VType.small, color = V.colors.muted)
        VField("Endstand", km, { km = it }, VIcons.gauge, keyboardType = KeyboardType.Decimal, trailing = "km",
            error = if (dist != null && dist < 0) "Der Endstand liegt unter dem Startstand." else null)
        if (dist != null && dist >= 0) Text(Format.number(dist, 0) + " km Strecke", style = VType.small, color = V.colors.muted)
        VField("Ziel (optional)", to, { to = it }, VIcons.route)
    }
}

/** Nach der Dateiauswahl: Titel und Typ des Dokuments. */
@Composable
fun AddDocumentDialog(fileName: String, error: FormError?, busy: Boolean, onDismiss: () -> Unit, onSave: (title: String, docType: String) -> Unit) {
    var title by remember { mutableStateOf(fileName.substringBeforeLast('.')) }
    var type by remember { mutableStateOf("invoice") }
    FormDialog("Dokument hinzufügen", error, busy, title.isNotBlank(), onDismiss, { onSave(title.trim(), type) }) {
        Text(fileName, style = VType.small, color = V.colors.muted)
        VField("Titel", title, { title = it }, VIcons.doc)
        ChoiceRow("Typ", docTypes, type) { type = it }
    }
}
