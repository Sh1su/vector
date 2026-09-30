package app.vectra.android.ui

import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import app.vectra.android.R

// Poppins für Überschriften und Kennzahlen, Inter für Fließtext (docs/phase-3/01-design-system.md).
// Beide unter SIL OFL 1.1 (assets/licenses), auf Latin reduziert.
object VectraFonts {
    val poppins = FontFamily(
        Font(R.font.poppins_500, FontWeight.Medium),
        Font(R.font.poppins_600, FontWeight.SemiBold),
    )
    val inter = FontFamily(
        Font(R.font.inter_400, FontWeight.Normal),
        Font(R.font.inter_500, FontWeight.Medium),
        Font(R.font.inter_600, FontWeight.SemiBold),
    )
}
