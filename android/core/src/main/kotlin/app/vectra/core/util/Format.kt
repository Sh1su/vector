package app.vectra.core.util

import java.math.BigDecimal
import java.math.RoundingMode
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.util.Locale

/** Deutsche Anzeigeformate, abgestimmt mit der Web-App (lib/format.ts). */
object Format {
    private val de = Locale.GERMANY

    /** Ganzzahl mit Tausenderpunkten, z. B. 142.900. */
    fun group(value: Long): String = String.format(de, "%,d", value)

    /** Zahl mit höchstens [decimals] Nachkommastellen, deutsches Komma. */
    fun number(value: Double, decimals: Int = 1): String {
        val bd = BigDecimal.valueOf(value).setScale(decimals, RoundingMode.HALF_EVEN).stripTrailingZeros()
        val scale = maxOf(bd.scale(), 0)
        return String.format(de, "%,.${scale}f", bd)
    }

    /** Zählerstand aus kanonischen Metern bzw. Sekunden. */
    fun meter(canonical: Long, unit: String): String = when (unit) {
        "h" -> number(canonical / 3600.0, 1) + " h"
        else -> group(Math.floorDiv(canonical, 1000L)) + " km"
    }

    private val dateFmt = DateTimeFormatter.ofPattern("dd.MM.yyyy", de)
    private val dateTimeFmt = DateTimeFormatter.ofPattern("dd.MM.yyyy, HH:mm", de)

    fun date(iso: String, zone: String): String = dateFmt.format(Instant.parse(iso).atZone(ZoneId.of(zone)))
    fun dateTime(iso: String, zone: String): String = dateTimeFmt.format(Instant.parse(iso).atZone(ZoneId.of(zone)))

    /** Eingabe mit deutschem oder englischem Dezimaltrennzeichen, Tausenderpunkte erlaubt ("142.900" → 142900). */
    fun parseNumber(input: String): Double? {
        val s = input.trim().replace(" ", "")
        if (s.isEmpty()) return null
        val normalized = when {
            s.contains(',') -> s.replace(".", "").replace(',', '.')
            Regex("""^\d{1,3}(\.\d{3})+$""").matches(s) -> s.replace(".", "")
            else -> s
        }
        return normalized.toDoubleOrNull()?.takeIf { it >= 0 && it.isFinite() }
    }
}

/** Geldbeträge aus kleinster Einheit (ADR-029), abgestimmt mit web/src/lib/money.ts. */
object MoneyFormat {
    private val zero = setOf("JPY", "KRW", "ISK", "CLP", "VND", "PYG", "UGX", "XAF", "XOF", "XPF", "RWF", "KMF", "GNF", "DJF", "VUV")
    private val three = setOf("BHD", "KWD", "OMR", "JOD", "TND", "LYD", "IQD")

    fun digits(currency: String): Int = if (currency in zero) 0 else if (currency in three) 3 else 2

    fun format(minor: Long, currency: String): String {
        val d = digits(currency)
        val v = java.math.BigDecimal.valueOf(minor, d)
        val symbol = if (currency == "EUR") "€" else currency
        return String.format(java.util.Locale.GERMANY, "%,.${d}f", v) + " " + symbol
    }

    /** „89,90“ oder „89.90“ → 8990 (bei EUR); null bei ungültiger Eingabe. */
    fun parse(input: String, currency: String): Long? {
        val v = Format.parseNumber(input) ?: return null
        return java.math.BigDecimal.valueOf(v).movePointRight(digits(currency)).setScale(0, java.math.RoundingMode.HALF_UP).toLong()
    }
}
