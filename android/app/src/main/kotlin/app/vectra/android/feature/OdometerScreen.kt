package app.vectra.android.feature

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import app.vectra.android.ui.AppBar
import app.vectra.android.ui.Brand
import app.vectra.android.ui.ChipKind
import app.vectra.android.ui.ListRow
import app.vectra.android.ui.Loading
import app.vectra.android.ui.Note
import app.vectra.android.ui.NoteAction
import app.vectra.android.ui.NoteKind
import app.vectra.android.ui.SectionTitle
import app.vectra.android.ui.StatusChip
import app.vectra.android.ui.V
import app.vectra.android.ui.VButton
import app.vectra.android.ui.VCard
import app.vectra.android.ui.VIcons
import app.vectra.android.ui.VType
import app.vectra.core.outbox.OutboxEntry
import app.vectra.core.outbox.OutboxStatus
import app.vectra.core.util.Format

/** Kilometerstand nach dem Entwurf „Kilometer“: aktueller Stand, Hinweise, Monatsbalken, Verlauf. */
@Composable
fun OdometerScreen(
    state: OdometerState,
    onBack: () -> Unit,
    onAdd: () -> Unit,
    onPending: (OutboxEntry) -> Unit,
) {
    val c = V.colors
    val v = state.vehicle
    val unit = v?.meterUnit ?: "km"
    Column(Modifier.fillMaxSize().background(c.bg)) {
        AppBar(
            title = if (unit == "h") "Betriebsstunden" else "Kilometerstand",
            sub = v?.displayName,
            onBack = onBack,
            primaryAction = if (v?.canEdit == true) VIcons.plus to "Stand erfassen" else null,
            onPrimaryAction = onAdd,
        )
        LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(16.dp), verticalArrangement = Arrangement.spacedBy(14.dp)) {
            item {
                VCard {
                    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        Icon(VIcons.gauge, null, tint = c.link, modifier = Modifier.size(18.dp))
                        Text("Aktueller Stand", style = VType.small, color = c.muted)
                    }
                    val cur = state.current
                    Text(cur?.meterValue?.let { Format.meter(it.canonical, unit) } ?: "–", style = VType.figureLarge, color = c.text)
                    val last = state.readings.firstOrNull()
                    if (last != null) {
                        Text("Zuletzt am ${Format.date(last.occurredAt, last.timeZone)} · ${sourceLabel(last.source).replaceFirstChar { it.lowercase() }.let { "aus $it" }}", style = VType.small, color = c.muted)
                    } else if (!state.loading) {
                        Text("Noch kein Messpunkt erfasst.", style = VType.small, color = c.muted)
                    }
                    if (v?.canEdit == true) {
                        VButton("Stand erfassen", onAdd, Modifier.fillMaxWidth(), icon = VIcons.edit, height = 48.dp)
                    }
                }
            }
            if (state.freshness.offline) {
                item { Note(NoteKind.Soft, "Offline – angezeigt wird der zuletzt geladene Stand.", icon = VIcons.cloudoff) }
            }
            state.error?.let { item { Note(NoteKind.Bad, it) } }
            items(state.pending.filter { it.status != OutboxStatus.PENDING && it.status != OutboxStatus.SENDING }, key = { "w" + it.id }) { e ->
                val p = e.problemOrNull()
                val text = when (e.status) {
                    OutboxStatus.NEEDS_CONFIRMATION -> e.summary + ": " + (p?.anomalies?.mapNotNull { it.message }?.joinToString(" ")?.ifBlank { null } ?: "Der Wert weicht vom bisherigen Verlauf ab.")
                    OutboxStatus.CONFLICT -> e.summary + ": Der Eintrag wurde inzwischen geändert."
                    else -> e.summary + ": " + (p?.detail ?: p?.title ?: "Der Server hat den Eintrag abgelehnt.")
                }
                Note(if (e.status == OutboxStatus.FAILED) NoteKind.Bad else NoteKind.Warn, text) {
                    NoteAction(if (e.status == OutboxStatus.NEEDS_CONFIRMATION) "Prüfen und bestätigen" else "Ansehen", if (e.status == OutboxStatus.FAILED) c.bad else c.warn) { onPending(e) }
                }
            }
            if (state.months.isNotEmpty()) {
                item {
                    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        SectionTitle(if (unit == "h") "Stunden pro Monat" else "Gefahrene km pro Monat")
                        MonthBars(state.months)
                    }
                }
            }
            item { SectionTitle("Verlauf") }
            if (state.loading && state.readings.isEmpty()) item { Loading() }
            items(state.pending.filter { it.status == OutboxStatus.PENDING || it.status == OutboxStatus.SENDING }, key = { "p" + it.id }) { e ->
                ListRow(VIcons.sync, e.summary, "Offline erfasst · wartet auf Sync", onClick = { onPending(e) })
            }
            val d = deltas(state.readings, unit)
            itemsIndexed(state.readings, key = { _, r -> r.id }) { i, r ->
                Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    ListRow(sourceIcon(r.source), r.meterText(unit), r.metaText(), d[i])
                    if (r.status == "confirmed_anomaly") {
                        Box(Modifier.padding(start = 8.dp)) { StatusChip("Abweichung bestätigt", ChipKind.Warn, VIcons.alert) }
                    }
                }
            }
        }
    }
}

@Composable
fun MonthBars(months: List<MonthBar>) {
    val c = V.colors
    val max = months.mapNotNull { it.km }.maxOrNull()?.takeIf { it > 0 } ?: 1.0
    Row(
        Modifier.fillMaxWidth().height(96.dp).padding(horizontal = 4.dp),
        horizontalArrangement = Arrangement.spacedBy(10.dp),
        verticalAlignment = Alignment.Bottom,
    ) {
        months.forEachIndexed { i, m ->
            val km = m.km
            Column(
                Modifier.weight(1f).fillMaxHeight().semantics { contentDescription = "${m.label}: " + (km?.let { Format.number(it, 0) + " km" } ?: "unbekannt") },
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(4.dp, Alignment.Bottom),
            ) {
                val h = if (km == null || km <= 0) 2.dp else (76 * (km / max)).coerceAtLeast(2.0).dp
                Box(Modifier.fillMaxWidth().height(h).clip(RoundedCornerShape(topStart = 6.dp, topEnd = 6.dp, bottomStart = 2.dp, bottomEnd = 2.dp))
                    .background(if (i == months.lastIndex) Brand.teal else c.bar))
                Text(m.label, style = VType.caption.copy(fontSize = VType.tab.fontSize), color = c.muted)
            }
        }
    }
}

