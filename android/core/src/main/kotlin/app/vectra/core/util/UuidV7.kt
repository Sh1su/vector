package app.vectra.core.util

import java.security.SecureRandom

/** Client-erzeugte UUIDv7 (ADR-006): 48 Bit Millisekunden, Version 7, Variante 10, Rest zufällig. */
object UuidV7 {
    private val random = SecureRandom()

    fun generate(nowMillis: Long = System.currentTimeMillis()): String {
        val bytes = ByteArray(16)
        random.nextBytes(bytes)
        for (i in 0 until 6) bytes[i] = (nowMillis ushr (8 * (5 - i))).toByte()
        bytes[6] = ((bytes[6].toInt() and 0x0f) or 0x70).toByte()
        bytes[8] = ((bytes[8].toInt() and 0x3f) or 0x80).toByte()
        val hex = bytes.joinToString("") { "%02x".format(it) }
        return "${hex.substring(0, 8)}-${hex.substring(8, 12)}-${hex.substring(12, 16)}-${hex.substring(16, 20)}-${hex.substring(20)}"
    }

    /** Zeitstempel einer UUIDv7 in Millisekunden. */
    fun timestamp(uuid: String): Long = uuid.replace("-", "").substring(0, 12).toLong(16)
}
