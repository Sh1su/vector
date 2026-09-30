package app.vectra.android.feature

import androidx.compose.foundation.Image
import androidx.compose.foundation.background
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
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.unit.dp
import app.vectra.android.ui.AppBar
import app.vectra.android.ui.Brand
import app.vectra.android.ui.ButtonKind
import app.vectra.android.ui.ChipKind
import app.vectra.android.ui.ListRow
import app.vectra.android.ui.Loading
import app.vectra.android.ui.Note
import app.vectra.android.ui.NoteKind
import app.vectra.android.ui.SectionTitle
import app.vectra.android.ui.StatusChip
import app.vectra.android.ui.V
import app.vectra.android.ui.VButton
import app.vectra.android.ui.VCard
import app.vectra.android.ui.VIcons
import app.vectra.android.ui.VType
import app.vectra.core.model.CostOccurrence
import app.vectra.core.model.DueStatus
import app.vectra.core.model.Trip
import app.vectra.core.util.Format
import app.vectra.core.util.MoneyFormat

@Composable
private fun StatusNotes(freshness: Freshness, error: String?) {
    if (freshness.offline) Note(NoteKind.Soft, "Offline – angezeigt wird der zuletzt geladene Stand" + (freshness.syncedAt?.let { " vom $it" } ?: "") + ". Erfassen geht nur mit Verbindung.", icon = VIcons.cloudoff)
    error?.let { Note(NoteKind.Bad, it) }
}

// ---------- Wartung ----------

/** Wartung: Fälligkeiten nach Dringlichkeit, Erledigen, Zugang zur Servicehistorie. */
@Composable
fun MaintenanceScreen(state: MaintenanceState, onComplete: (DueStatus) -> Unit, onService: () -> Unit, onRefresh: () -> Unit) {
    val c = V.colors
    val v = state.vehicle
    Column(Modifier.fillMaxSize().background(c.bg)) {
        AppBar("Wartung", v?.displayName, action = VIcons.sync to "Aktualisieren", onAction = onRefresh, primaryAction = VIcons.receipt to "Servicehistorie", onPrimaryAction = onService)
        LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            item { StatusNotes(state.freshness, state.error) }
            val urgent = state.due.count { it.level == "overdue" || it.level == "due" }
            item {
                VCard {
                    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        Box(Modifier.size(36.dp).clip(RoundedCornerShape(18.dp)).background(if (urgent == 0) Brand.ready else Brand.amber), contentAlignment = Alignment.Center) {
                            Icon(if (urgent == 0) VIcons.check else VIcons.wrench, null, tint = Brand.ink, modifier = Modifier.size(20.dp))
                        }
                        Text(
                            when {
                                state.due.isEmpty() && !state.loading -> "Noch keine Wartungen geplant. Lege sie in der Web-App an."
                                urgent == 0 -> "Alles im grünen Bereich."
                                urgent == 1 -> "1 Wartung ist fällig."
                                else -> "$urgent Wartungen sind fällig."
                            },
                            style = VType.bodyStrong, color = c.text, modifier = Modifier.weight(1f),
                        )
                    }
                }
            }
            if (state.loading && state.due.isEmpty()) item { Loading() }
            items(state.due, key = { it.itemId }) { d ->
                VCard(onClick = if (v?.canEdit == true && d.level != "completed") ({ onComplete(d) }) else null) {
                    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        Icon(VIcons.wrench, null, tint = c.link, modifier = Modifier.size(20.dp))
                        Text(d.title, style = VType.bodyStrong, color = c.text, modifier = Modifier.weight(1f))
                        StatusChip(levelLabel(d.level), levelChip(d.level))
                    }
                    val t = d.dueText()
                    if (t.isNotBlank()) Text(t, style = VType.small, color = c.muted)
                    if (v?.canEdit == true && d.level != "completed") Text("Antippen, um als erledigt zu markieren", style = VType.caption, color = c.link)
                }
            }
            item { VButton("Servicehistorie", onService, Modifier.fillMaxWidth(), kind = ButtonKind.Outline, icon = VIcons.receipt, height = 48.dp) }
        }
    }
}

/** Servicehistorie: Werkstattbesuche mit Summen; neue Einträge mit Teilen/Arbeit/Sonstigem. */
@Composable
fun ServiceScreen(state: MaintenanceState, onBack: () -> Unit, onAdd: () -> Unit) {
    val c = V.colors
    val v = state.vehicle
    Column(Modifier.fillMaxSize().background(c.bg)) {
        AppBar("Servicehistorie", v?.displayName, onBack = onBack, primaryAction = if (v?.canEdit == true) VIcons.plus to "Eintrag erfassen" else null, onPrimaryAction = onAdd)
        LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            item { StatusNotes(state.freshness, state.error) }
            if (state.loading && state.services.isEmpty()) item { Loading() }
            if (!state.loading && state.services.isEmpty()) item { Note(NoteKind.Info, "Noch keine Serviceeinträge. Erfasse Werkstattbesuche mit Kosten und Kilometerstand.") }
            items(state.services, key = { it.id }) { e ->
                val meta = listOfNotNull(day(e.occurredAt), serviceKindLabel(e.kind), e.odometer?.let { Format.number(it.value, 0) + " " + it.unit }, e.providerName).joinToString(" · ")
                ListRow(if (e.kind == "repair") VIcons.alert else VIcons.wrench, e.title, meta,
                    e.totals?.let { MoneyFormat.format(it.total.amountMinor, it.total.currency) } ?: "Kosten unbekannt")
            }
        }
    }
}

// ---------- Kosten ----------

/** Kosten: Jahressumme, je km/Tag, Kategorien, anstehende Vorkommen, sonstige Kosten. */
@Composable
fun CostsScreen(state: CostsState, onConfirm: (CostOccurrence) -> Unit, onAdd: () -> Unit, onRefresh: () -> Unit) {
    val c = V.colors
    val v = state.vehicle
    val cur = state.report?.currencies?.firstOrNull()
    Column(Modifier.fillMaxSize().background(c.bg)) {
        AppBar("Kosten", v?.displayName, action = VIcons.sync to "Aktualisieren", onAction = onRefresh,
            primaryAction = if (v?.canEdit == true) VIcons.plus to "Kosten erfassen" else null, onPrimaryAction = onAdd)
        LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            item { StatusNotes(state.freshness, state.error) }
            item {
                VCard {
                    Text("Kosten ${state.year}", style = VType.small, color = c.muted)
                    Text(cur?.let { MoneyFormat.format(it.runningTotalMinor, it.currency) } ?: "–", style = VType.figureLarge, color = c.text)
                    val unit = cur?.currency.orEmpty()
                    Row(horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                        Text("je km: " + (cur?.perDistance?.let { Format.number(it.value, 2) + " " + unit } ?: "–"), style = VType.small, color = c.muted)
                        Text("je Tag: " + (cur?.perDay?.let { Format.number(it.value, 2) + " " + unit } ?: "–"), style = VType.small, color = c.muted)
                    }
                    val groups = cur?.groups.orEmpty()
                    val max = groups.maxOfOrNull { kotlin.math.abs(it.amountMinor) }?.takeIf { it > 0 } ?: 1L
                    groups.forEach { g ->
                        Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                            Row { Text(costCategoryLabel(g.key), style = VType.small, color = c.text, modifier = Modifier.weight(1f)); Text(MoneyFormat.format(g.amountMinor, unit), style = VType.small, color = c.text) }
                            Box(Modifier.fillMaxWidth().height(8.dp).clip(RoundedCornerShape(4.dp)).background(c.soft)) {
                                Box(Modifier.fillMaxWidth((kotlin.math.abs(g.amountMinor).toFloat() / max).coerceIn(0.02f, 1f)).height(8.dp).clip(RoundedCornerShape(4.dp)).background(Brand.teal))
                            }
                        }
                    }
                }
            }
            if (state.occurrences.isNotEmpty()) {
                item { SectionTitle("Anstehende Kosten") }
                items(state.occurrences, key = { it.planId + it.dueOn }) { o ->
                    VCard {
                        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                            Column(Modifier.weight(1f)) {
                                Text(o.title, style = VType.bodyStrong, color = c.text)
                                Text((if (o.state == "open") "fällig seit " else "fällig am ") + day(o.dueOn), style = VType.caption, color = c.muted)
                            }
                            Text(MoneyFormat.format(o.amount.amountMinor, o.amount.currency), style = VType.bodyStrong, color = c.text)
                        }
                        if (v?.canEdit == true) VButton("Bezahlt", { onConfirm(o) }, Modifier.fillMaxWidth(), kind = ButtonKind.Navy, height = 44.dp)
                    }
                }
                if (state.moreOpen > 0) item { Text("${state.moreOpen} weitere offen", style = VType.caption, color = c.muted) }
            }
            item { SectionTitle("Sonstige Kosten") }
            if (state.loading && state.entries.isEmpty()) item { Loading() }
            if (!state.loading && state.entries.isEmpty()) item { Note(NoteKind.Info, "Werkstattkosten erscheinen automatisch aus der Servicehistorie. Hier erfasst du Parken, Maut, Versicherung & Co.") }
            items(state.entries, key = { it.id }) { e ->
                ListRow(VIcons.euro, e.title, costCategoryLabel(e.category) + " · " + day(e.incurredOn), MoneyFormat.format(e.amount.amountMinor, e.amount.currency))
            }
        }
    }
}

// ---------- Fahrten ----------

/** Fahrten: laufende Fahrt beenden oder neue starten, Liste mit Strecke und Lücken, Monatssumme. */
@Composable
fun TripsScreen(state: TripsState, onStart: () -> Unit, onFinish: (Trip) -> Unit, onRefresh: () -> Unit) {
    val c = V.colors
    val v = state.vehicle
    val open = state.open
    val cats = state.categories.associateBy { it.id }
    Column(Modifier.fillMaxSize().background(c.bg)) {
        AppBar("Fahrten", v?.displayName, action = VIcons.sync to "Aktualisieren", onAction = onRefresh)
        LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            item { StatusNotes(state.freshness, state.error) }
            item {
                Column(Modifier.fillMaxWidth().clip(RoundedCornerShape(16.dp)).background(Brand.navy).padding(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                    if (open != null) {
                        Text("Fahrt läuft seit " + Format.dateTime(open.startedAt, open.timeZone), style = VType.hero, color = Brand.onNavy)
                        Text("Start bei " + Format.number(open.startOdometer.value, 0) + " " + open.startOdometer.unit + (cats[open.categoryId]?.let { " · " + it.name } ?: ""), style = VType.small, color = Brand.onNavyMuted)
                        if (v?.canEdit == true) VButton("Fahrt beenden", { onFinish(open) }, Modifier.fillMaxWidth(), icon = VIcons.check)
                    } else {
                        Text("Im ${state.monthLabel}: " + (state.report?.total?.let { Format.number(it.value, 0) + " " + it.unit } ?: "–"), style = VType.hero, color = Brand.onNavy)
                        state.report?.byCategory?.take(3)?.forEach { s ->
                            Text("${s.label ?: s.key}: ${Format.number(s.distance.value, 0)} ${s.distance.unit} · ${Format.number(s.sharePct, 0)} %", style = VType.small, color = Brand.onNavyMuted)
                        }
                        if (v?.canEdit == true) VButton("Fahrt starten", onStart, Modifier.fillMaxWidth(), icon = VIcons.play)
                    }
                }
            }
            item { SectionTitle("Fahrtenliste") }
            if (state.loading && state.trips.isEmpty()) item { Loading() }
            if (!state.loading && state.trips.isEmpty()) item { Note(NoteKind.Info, "Noch keine Fahrten. Die Strecke berechnet Vectra aus Start- und Endstand.") }
            items(state.trips, key = { it.id }) { t ->
                Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    t.gapBefore?.let { Note(NoteKind.Warn, Format.number(it.value, 0) + " ${it.unit} nicht als Fahrt erfasst") }
                    val title = listOfNotNull(t.startLocation, t.endLocation).joinToString(" → ").ifBlank { t.purpose ?: "Fahrt" }
                    val meta = Format.dateTime(t.startedAt, t.timeZone) + (cats[t.categoryId]?.let { " · " + it.name } ?: "") + if (t.status == "open") " · läuft" else ""
                    ListRow(VIcons.route, title, meta, t.distance?.let { Format.number(it.value, 1) + " " + it.unit } ?: "–")
                }
            }
        }
    }
}

// ---------- Dokumente ----------

/** Fahrzeugakte: Dokumente mit Vorschaubild, Datei hochladen (Foto oder PDF). */
@Composable
fun DocumentsScreen(state: DocumentsState, onBack: () -> Unit, onUpload: () -> Unit) {
    val c = V.colors
    val v = state.vehicle
    val files = state.files.associateBy { it.id }
    Column(Modifier.fillMaxSize().background(c.bg)) {
        AppBar("Dokumente", v?.displayName, onBack = onBack, primaryAction = if (v?.canEdit == true) VIcons.plus to "Dokument hinzufügen" else null, onPrimaryAction = onUpload)
        LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            item { StatusNotes(state.freshness, state.error) }
            state.message?.let { item { Note(NoteKind.Info, it) } }
            if (v?.canEdit == true) item { VButton(if (state.uploading) "Lädt hoch …" else "Foto oder PDF hochladen", onUpload, Modifier.fillMaxWidth(), icon = VIcons.scan, loading = state.uploading) }
            if (state.loading && state.documents.isEmpty()) item { Loading() }
            if (!state.loading && state.documents.isEmpty()) item { Note(NoteKind.Info, "Die Fahrzeugakte ist leer. Lade Rechnungen, HU-Berichte, Zulassung oder das Handbuch hoch.") }
            items(state.documents, key = { it.id }) { d ->
                val first = d.fileIds.firstOrNull()
                val thumb = first?.let { state.thumbs[it] }
                VCard {
                    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                        Box(Modifier.size(56.dp).clip(RoundedCornerShape(10.dp)).background(c.soft), contentAlignment = Alignment.Center) {
                            if (thumb != null) Image(thumb, null, Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
                            else Icon(VIcons.doc, null, tint = c.muted, modifier = Modifier.size(26.dp))
                        }
                        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                            Text(d.title, style = VType.bodyStrong, color = c.text)
                            StatusChip(docTypeLabel(d.docType), ChipKind.Info)
                            val meta = listOfNotNull(d.documentDate?.let { day(it) }, d.issuer, "${d.fileIds.size} " + if (d.fileIds.size == 1) "Datei" else "Dateien").joinToString(" · ")
                            Text(meta, style = VType.caption, color = c.muted)
                            if (first != null && files[first]?.hasPreview == false) Text(files[first]?.originalName ?: "", style = VType.caption, color = c.muted)
                        }
                    }
                }
            }
        }
    }
}
