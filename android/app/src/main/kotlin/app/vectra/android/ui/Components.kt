package app.vectra.android.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

// Bausteine der App-Entwürfe (AppBar, TabBar, Karten, Buttons, Felder, Hinweise).

/** App-Leiste: Navy, 88 dp, Zurück oder Bildmarke links, bis zu zwei Aktionen rechts. */
@Composable
fun AppBar(
    title: String,
    sub: String? = null,
    onBack: (() -> Unit)? = null,
    logo: (@Composable () -> Unit)? = null,
    action: Pair<ImageVector, String>? = null,
    onAction: () -> Unit = {},
    primaryAction: Pair<ImageVector, String>? = null,
    onPrimaryAction: () -> Unit = {},
) {
    Row(
        Modifier.fillMaxWidth().background(Brand.navy).statusBarsPadding()
            .height(88.dp).padding(start = 12.dp, end = 16.dp, top = 16.dp, bottom = 16.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        if (onBack != null) {
            BarButton(VIcons.left, "Zurück", Brand.onNavySurface, Brand.onNavy, onBack)
        } else if (logo != null) {
            logo()
        }
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(title, style = VType.appTitle, color = Brand.onNavy, maxLines = 1, overflow = TextOverflow.Ellipsis)
            if (sub != null) Text(sub, style = VType.small, color = Brand.onNavyMuted, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
        action?.let { (icon, label) -> BarButton(icon, label, Brand.onNavySurface, Brand.onNavy, onAction) }
        primaryAction?.let { (icon, label) -> BarButton(icon, label, Brand.teal, Brand.ink, onPrimaryAction) }
    }
}

@Composable
fun BarButton(icon: ImageVector, label: String, bg: Color, fg: Color, onClick: () -> Unit) {
    Box(
        Modifier.size(44.dp).clip(RoundedCornerShape(12.dp)).background(bg)
            .clickable(role = Role.Button, onClickLabel = label, onClick = onClick)
            .semantics { contentDescription = label },
        contentAlignment = Alignment.Center,
    ) { Icon(icon, contentDescription = null, tint = fg, modifier = Modifier.size(22.dp)) }
}

enum class Tab(val label: String) { Home("Übersicht"), Trips("Fahrten"), Maintenance("Wartung"), Costs("Kosten"), More("Mehr") }

private fun Tab.icon(): ImageVector = when (this) {
    Tab.Home -> VIcons.home
    Tab.Trips -> VIcons.route
    Tab.Maintenance -> VIcons.wrench
    Tab.Costs -> VIcons.euro
    Tab.More -> VIcons.more
}

/** Tab-Leiste: fünf Ziele, aktives Ziel mit Pille. */
@Composable
fun TabBar(active: Tab, onSelect: (Tab) -> Unit) {
    val c = V.colors
    Column(Modifier.fillMaxWidth().background(c.card)) {
        Box(Modifier.fillMaxWidth().height(1.dp).background(c.border))
        Row(Modifier.fillMaxWidth().navigationBarsPadding().height(72.dp).padding(start = 4.dp, end = 4.dp, top = 4.dp, bottom = 8.dp)) {
            for (t in Tab.entries) {
                val on = t == active
                val color = if (on) c.tabActive else c.muted
                Column(
                    Modifier.weight(1f).clip(RoundedCornerShape(12.dp))
                        .clickable(role = Role.Tab, onClickLabel = t.label) { onSelect(t) }
                        .padding(vertical = 2.dp),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(4.dp, Alignment.CenterVertically),
                ) {
                    Box(
                        Modifier.width(52.dp).height(30.dp).clip(CircleShape).background(if (on) c.tabPill else Color.Transparent),
                        contentAlignment = Alignment.Center,
                    ) { Icon(t.icon(), contentDescription = null, tint = color, modifier = Modifier.size(22.dp)) }
                    Text(t.label, style = VType.tab, color = color, maxLines = 1)
                }
            }
        }
    }
}

@Composable
fun VCard(modifier: Modifier = Modifier, onClick: (() -> Unit)? = null, padding: PaddingValues = PaddingValues(16.dp), content: @Composable ColumnScope.() -> Unit) {
    val c = V.colors
    val base = modifier.clip(RoundedCornerShape(16.dp)).background(c.card).border(1.dp, c.border, RoundedCornerShape(16.dp))
    Column(
        (if (onClick != null) base.clickable(onClick = onClick) else base).padding(padding),
        verticalArrangement = Arrangement.spacedBy(12.dp),
        content = content,
    )
}

enum class ButtonKind { Accent, Navy, Outline }

/** Buttons der Vorlage: Teal mit dunklem Text, Navy mit weißem Text oder mit Rahmen. */
@Composable
fun VButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    kind: ButtonKind = ButtonKind.Accent,
    icon: ImageVector? = null,
    enabled: Boolean = true,
    loading: Boolean = false,
    height: Dp = 52.dp,
) {
    val c = V.colors
    val (bg, fg) = when (kind) {
        ButtonKind.Accent -> Brand.teal to Brand.ink
        ButtonKind.Navy -> (if (c.dark) Brand.teal else Brand.navy) to (if (c.dark) Brand.ink else Color.White)
        ButtonKind.Outline -> c.card to c.text
    }
    val shape = RoundedCornerShape(14.dp)
    var m = modifier.height(height).clip(shape).background(if (enabled) bg else bg.copy(alpha = 0.5f))
    if (kind == ButtonKind.Outline) m = m.border(BorderStroke(1.5.dp, c.border), shape)
    Row(
        m.clickable(enabled = enabled && !loading, role = Role.Button, onClick = onClick).padding(horizontal = 16.dp),
        horizontalArrangement = Arrangement.spacedBy(8.dp, Alignment.CenterHorizontally),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (loading) {
            CircularProgressIndicator(color = fg, strokeWidth = 2.dp, modifier = Modifier.size(18.dp))
        } else if (icon != null) {
            Icon(icon, contentDescription = null, tint = fg, modifier = Modifier.size(20.dp))
        }
        Text(text, style = VType.button, color = fg, maxLines = 1)
    }
}

/** Eingabefeld 52 dp mit Icon links und Beschriftung darüber (Login-Entwurf). */
@Composable
fun VField(
    label: String,
    value: String,
    onValueChange: (String) -> Unit,
    icon: ImageVector,
    modifier: Modifier = Modifier,
    placeholder: String = "",
    keyboardType: KeyboardType = KeyboardType.Text,
    password: Boolean = false,
    error: String? = null,
    trailing: String? = null,
) {
    val c = V.colors
    Column(modifier, verticalArrangement = Arrangement.spacedBy(6.dp)) {
        Text(label, style = VType.label, color = c.text)
        Row(
            Modifier.fillMaxWidth().height(52.dp).clip(RoundedCornerShape(12.dp)).background(c.card)
                .border(1.5.dp, if (error != null) c.bad else c.border, RoundedCornerShape(12.dp)).padding(horizontal = 14.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            Icon(icon, contentDescription = null, tint = c.muted, modifier = Modifier.size(20.dp))
            Box(Modifier.weight(1f)) {
                if (value.isEmpty()) Text(placeholder, style = VType.input, color = c.muted)
                BasicTextField(
                    value = value,
                    onValueChange = onValueChange,
                    singleLine = true,
                    textStyle = VType.input.copy(color = c.text),
                    cursorBrush = SolidColor(c.link),
                    keyboardOptions = KeyboardOptions(keyboardType = if (password) KeyboardType.Password else keyboardType),
                    visualTransformation = if (password) PasswordVisualTransformation() else VisualTransformation.None,
                    modifier = Modifier.fillMaxWidth().semantics { contentDescription = label },
                )
            }
            if (trailing != null) Text(trailing, style = VType.label, color = c.muted)
        }
        if (error != null) Text(error, style = VType.small, color = c.bad)
    }
}

enum class NoteKind { Info, Warn, Bad, Soft }

/** Hinweisfläche mit Icon (Info-, Warn- und Fehlerboxen der Entwürfe). */
@Composable
fun Note(kind: NoteKind, text: String, modifier: Modifier = Modifier, icon: ImageVector? = null, actions: (@Composable RowScope.() -> Unit)? = null) {
    val c = V.colors
    val (bg, fg, textColor) = when (kind) {
        NoteKind.Info -> Triple(c.infoBg, c.link, c.muted)
        NoteKind.Soft -> Triple(c.soft, c.link, c.muted)
        NoteKind.Warn -> Triple(c.warnBg, c.warn, c.warn)
        NoteKind.Bad -> Triple(c.badBg, c.bad, c.bad)
    }
    Row(
        modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).background(bg).padding(horizontal = 14.dp, vertical = 12.dp),
        horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Icon(icon ?: if (kind == NoteKind.Warn || kind == NoteKind.Bad) VIcons.alert else VIcons.info, contentDescription = null, tint = fg, modifier = Modifier.size(20.dp))
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Text(text, style = if (kind == NoteKind.Warn || kind == NoteKind.Bad) VType.small.copy(fontWeight = VType.label.fontWeight) else VType.small, color = textColor)
            if (actions != null) Row(horizontalArrangement = Arrangement.spacedBy(14.dp), content = actions)
        }
    }
}

@Composable
fun NoteAction(text: String, color: Color, onClick: () -> Unit) {
    Text(text, style = VType.small.copy(fontWeight = VType.label.fontWeight), color = color,
        modifier = Modifier.clip(RoundedCornerShape(6.dp)).clickable(role = Role.Button, onClick = onClick).padding(vertical = 4.dp))
}

enum class ChipKind { Ok, Warn, Bad, Info }

@Composable
fun StatusChip(text: String, kind: ChipKind, icon: ImageVector? = null) {
    val c = V.colors
    val (bg, fg) = when (kind) {
        ChipKind.Ok -> c.okBg to c.ok
        ChipKind.Warn -> c.warnBg to c.warn
        ChipKind.Bad -> c.badBg to c.bad
        ChipKind.Info -> c.infoBg to c.link
    }
    Row(
        Modifier.height(28.dp).clip(CircleShape).background(bg).padding(horizontal = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        if (icon != null) Icon(icon, contentDescription = null, tint = fg, modifier = Modifier.size(16.dp))
        Text(text, style = VType.small.copy(fontWeight = VType.label.fontWeight), color = fg)
    }
}

@Composable
fun SectionTitle(text: String, modifier: Modifier = Modifier, trailing: (@Composable () -> Unit)? = null) {
    Row(modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
        Text(text, style = VType.section, color = V.colors.text, modifier = Modifier.weight(1f))
        trailing?.invoke()
    }
}

/** Listenzeile mit Icon links, zwei Textzeilen und einem Wert rechts (Verlauf im Kilometer-Entwurf). */
@Composable
fun ListRow(icon: ImageVector, title: String, meta: String, trailing: String? = null, trailingColor: Color? = null, onClick: (() -> Unit)? = null) {
    val c = V.colors
    val shape = RoundedCornerShape(12.dp)
    var m = Modifier.fillMaxWidth().heightIn(min = 56.dp).clip(shape).background(c.card).border(1.dp, c.border, shape)
    if (onClick != null) m = m.clickable(onClick = onClick)
    Row(m.padding(horizontal = 12.dp, vertical = 10.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
        Icon(icon, contentDescription = null, tint = c.link, modifier = Modifier.size(20.dp))
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(1.dp)) {
            Text(title, style = VType.bodyStrong, color = c.text)
            Text(meta, style = VType.caption, color = c.muted)
        }
        if (trailing != null) Text(trailing, style = VType.small.copy(fontWeight = VType.label.fontWeight, fontFeatureSettings = "tnum"), color = trailingColor ?: c.muted)
    }
}

@Composable
fun IconTile(icon: ImageVector, size: Dp = 38.dp) {
    val c = V.colors
    Box(Modifier.size(size).clip(RoundedCornerShape(12.dp)).background(c.infoBg), contentAlignment = Alignment.Center) {
        Icon(icon, contentDescription = null, tint = c.link, modifier = Modifier.size(22.dp))
    }
}

@Composable
fun Loading(modifier: Modifier = Modifier) {
    Box(modifier.fillMaxWidth().padding(32.dp), contentAlignment = Alignment.Center) {
        CircularProgressIndicator(color = V.colors.link, strokeWidth = 2.5.dp, modifier = Modifier.size(28.dp))
    }
}

@Composable
fun Gap(h: Dp) = Spacer(Modifier.height(h))
