package app.vectra.core

import app.vectra.core.util.Format
import app.vectra.core.util.MoneyFormat
import app.vectra.core.util.UuidV7
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertNull
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test

class UtilTest {
    @Test
    fun `UUIDv7 hat Version, Variante und Zeitstempel`() {
        val t = 1_790_000_000_000L
        val id = UuidV7.generate(t)
        assertTrue(Regex("^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$").matches(id), id)
        assertEquals(t, UuidV7.timestamp(id))
    }

    @Test
    fun `UUIDv7 sind nach Zeit sortierbar`() {
        val a = UuidV7.generate(1000)
        val b = UuidV7.generate(2000)
        assertTrue(a < b)
    }

    @Test
    fun `deutsche Zahlenformate`() {
        assertEquals("142.900", Format.group(142_900))
        assertEquals("142.900 km", Format.meter(142_900_400, "km"))
        assertEquals("1.234,5", Format.number(1234.5))
        assertEquals("6", Format.number(6.0))
        assertEquals("0,2", Format.number(0.25)) // half-even
        assertEquals("12,5 h", Format.meter(45_000, "h"))
    }

    @Test
    fun `Eingaben mit Komma, Punkt und Tausenderpunkten`() {
        assertEquals(142900.0, Format.parseNumber("142.900"))
        assertEquals(142900.5, Format.parseNumber("142.900,5"))
        assertEquals(1234.5, Format.parseNumber("1234.5"))
        assertEquals(12.5, Format.parseNumber("12,5"))
        assertNull(Format.parseNumber(""))
        assertNull(Format.parseNumber("-3"))
        assertNull(Format.parseNumber("abc"))
    }

    @Test
    fun `Datum in der Zeitzone der Erfassung`() {
        assertEquals("29.09.2026, 08:00", Format.dateTime("2026-09-29T06:00:00Z", "Europe/Berlin"))
    }

    @Test
    fun `Geldbeträge in kleinster Einheit`() {
        assertEquals("1.234,56 €", MoneyFormat.format(123_456, "EUR"))
        assertEquals("1.500 JPY", MoneyFormat.format(1_500, "JPY"))
        assertEquals(8_990L, MoneyFormat.parse("89,90", "EUR"))
        assertEquals(12_000L, MoneyFormat.parse("120", "EUR"))
        assertNull(MoneyFormat.parse("abc", "EUR"))
    }
}
