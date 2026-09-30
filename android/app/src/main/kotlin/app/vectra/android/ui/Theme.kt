package app.vectra.android.ui

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp

// Farben und Typografie aus docs/phase-3/01-design-system.md (Vorlage „Vectra Brand“).

object Brand {
    val navy = Color(0xFF1E3A5F)
    val teal = Color(0xFF0EA5E9)
    val tealSoft = Color(0xFF7DD3FC)
    val ready = Color(0xFF10B981)
    val amber = Color(0xFFF59E0B)
    val ink = Color(0xFF0F172A)
    val onNavy = Color(0xFFF8FAFC)
    val onNavyMuted = Color(0xFFCBD5E1)
    val onNavySurface = Color(0x14F8FAFC) // rgba(248,250,252,.08)
    val onNavyLine = Color(0x40F8FAFC) // rgba(248,250,252,.25)
}

@Immutable
data class VColors(
    val bg: Color,
    val card: Color,
    val text: Color,
    val muted: Color,
    val border: Color,
    val soft: Color,
    val link: Color,
    val infoBg: Color,
    val ok: Color,
    val okBg: Color,
    val warn: Color,
    val warnBg: Color,
    val bad: Color,
    val badBg: Color,
    val hero: Color,
    val bar: Color,
    val tabActive: Color,
    val tabPill: Color,
    val dark: Boolean,
)

val LightColors = VColors(
    bg = Color(0xFFF8FAFC), card = Color.White, text = Color(0xFF0F172A), muted = Color(0xFF64748B),
    border = Color(0xFFE2E8F0), soft = Color(0xFFF1F5F9), link = Color(0xFF0369A1), infoBg = Color(0xFFE0F2FE),
    ok = Color(0xFF047857), okBg = Color(0xFFD1FAE5), warn = Color(0xFF92400E), warnBg = Color(0xFFFEF3C7),
    bad = Color(0xFFB91C1C), badBg = Color(0xFFFEE2E2), hero = Color(0xFF1E3A5F), bar = Color(0xFFCBD5E1),
    tabActive = Color(0xFF0369A1), tabPill = Color(0xFFE0F2FE), dark = false,
)

val DarkColors = VColors(
    bg = Color(0xFF0F172A), card = Color(0xFF1E293B), text = Color(0xFFF8FAFC), muted = Color(0xFF94A3B8),
    border = Color(0xFF334155), soft = Color(0xFF172136), link = Color(0xFF38BDF8), infoBg = Color(0x290EA5E9),
    ok = Color(0xFF34D399), okBg = Color(0x2910B981), warn = Color(0xFFFBBF24), warnBg = Color(0x29F59E0B),
    bad = Color(0xFFF87171), badBg = Color(0x29EF4444), hero = Color(0xFF172A45), bar = Color(0xFF334155),
    tabActive = Color(0xFF38BDF8), tabPill = Color(0x2E0EA5E9), dark = true,
)

val LocalVColors = staticCompositionLocalOf { LightColors }

object V {
    val colors: VColors
        @Composable get() = LocalVColors.current
}

/** Textstile der Vorlage: Poppins 600 für Titel und Kennzahlen, Inter für Text. */
object VType {
    val appTitle = TextStyle(fontFamily = VectraFonts.poppins, fontWeight = FontWeight.SemiBold, fontSize = 20.sp, lineHeight = 24.sp)
    val hero = TextStyle(fontFamily = VectraFonts.poppins, fontWeight = FontWeight.SemiBold, fontSize = 19.sp, lineHeight = 24.sp)
    val section = TextStyle(fontFamily = VectraFonts.poppins, fontWeight = FontWeight.SemiBold, fontSize = 16.sp, lineHeight = 22.sp)
    val figure = TextStyle(fontFamily = VectraFonts.poppins, fontWeight = FontWeight.SemiBold, fontSize = 21.sp, lineHeight = 28.sp, fontFeatureSettings = "tnum")
    val figureLarge = TextStyle(fontFamily = VectraFonts.poppins, fontWeight = FontWeight.SemiBold, fontSize = 32.sp, lineHeight = 40.sp, fontFeatureSettings = "tnum")
    val cardTitle = TextStyle(fontFamily = VectraFonts.poppins, fontWeight = FontWeight.SemiBold, fontSize = 18.sp, lineHeight = 24.sp)
    val body = TextStyle(fontFamily = VectraFonts.inter, fontWeight = FontWeight.Normal, fontSize = 15.sp, lineHeight = 22.sp)
    val bodyStrong = TextStyle(fontFamily = VectraFonts.inter, fontWeight = FontWeight.SemiBold, fontSize = 14.sp, lineHeight = 20.sp, fontFeatureSettings = "tnum")
    val label = TextStyle(fontFamily = VectraFonts.inter, fontWeight = FontWeight.SemiBold, fontSize = 14.sp, lineHeight = 20.sp)
    val button = TextStyle(fontFamily = VectraFonts.inter, fontWeight = FontWeight.SemiBold, fontSize = 15.sp, lineHeight = 20.sp)
    val small = TextStyle(fontFamily = VectraFonts.inter, fontWeight = FontWeight.Normal, fontSize = 13.sp, lineHeight = 19.sp)
    val caption = TextStyle(fontFamily = VectraFonts.inter, fontWeight = FontWeight.Normal, fontSize = 12.sp, lineHeight = 17.sp)
    val tab = TextStyle(fontFamily = VectraFonts.inter, fontWeight = FontWeight.SemiBold, fontSize = 11.sp, lineHeight = 14.sp)
    val input = TextStyle(fontFamily = VectraFonts.inter, fontWeight = FontWeight.Normal, fontSize = 16.sp, lineHeight = 22.sp)
}

@Composable
fun VectraTheme(dark: Boolean = isSystemInDarkTheme(), content: @Composable () -> Unit) {
    val c = if (dark) DarkColors else LightColors
    val scheme = if (dark) {
        darkColorScheme(primary = Brand.teal, onPrimary = Brand.ink, secondary = Brand.navy, background = c.bg, surface = c.card,
            onBackground = c.text, onSurface = c.text, onSurfaceVariant = c.muted, outline = c.border, error = c.bad)
    } else {
        lightColorScheme(primary = Brand.navy, onPrimary = Color.White, secondary = Brand.teal, background = c.bg, surface = c.card,
            onBackground = c.text, onSurface = c.text, onSurfaceVariant = c.muted, outline = c.border, error = c.bad)
    }
    val base = Typography()
    val typography = Typography(
        bodyLarge = VType.body, bodyMedium = VType.small, bodySmall = VType.caption,
        labelLarge = VType.button, titleLarge = VType.appTitle, titleMedium = VType.section,
        headlineSmall = base.headlineSmall.copy(fontFamily = VectraFonts.poppins, fontWeight = FontWeight.SemiBold),
    )
    CompositionLocalProvider(LocalVColors provides c) {
        MaterialTheme(colorScheme = scheme, typography = typography, content = content)
    }
}
