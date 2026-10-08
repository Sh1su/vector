package app.vectra.android.data

import android.content.Context
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Matrix
import android.media.ExifInterface
import android.net.Uri
import androidx.core.content.FileProvider
import java.io.ByteArrayOutputStream
import java.io.File

/** Aufnahme und Aufbereitung von Tachofotos (Kamera-App des Systems). */
object TachoImage {
    private const val MAX_EDGE = 2048

    /** Neue Zieldatei für die Kamera-App im Cache und ihre content://-URI. */
    fun newTarget(context: Context): Pair<File, Uri> {
        val dir = File(context.cacheDir, "tacho").apply { mkdirs() }
        dir.listFiles()?.forEach { it.delete() } // ältere, nicht mehr gebrauchte Aufnahmen
        val file = File(dir, "tacho-${System.currentTimeMillis()}.jpg")
        return file to FileProvider.getUriForFile(context, context.packageName + ".files", file)
    }

    /**
     * Liest das Foto, dreht es nach EXIF richtig herum und verkleinert es auf höchstens
     * 2048 px Kantenlänge (JPEG). Das spart Datenvolumen und entfernt EXIF samt GPS.
     */
    fun prepare(file: File): ByteArray? = runCatching {
        if (!file.exists() || file.length() == 0L) return@runCatching null
        val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
        BitmapFactory.decodeFile(file.path, bounds)
        var sample = 1
        while (maxOf(bounds.outWidth, bounds.outHeight) / (sample * 2) >= MAX_EDGE) sample *= 2
        val raw = BitmapFactory.decodeFile(file.path, BitmapFactory.Options().apply { inSampleSize = sample }) ?: return@runCatching null
        val rotation = when (ExifInterface(file.path).getAttributeInt(ExifInterface.TAG_ORIENTATION, ExifInterface.ORIENTATION_NORMAL)) {
            ExifInterface.ORIENTATION_ROTATE_90 -> 90f
            ExifInterface.ORIENTATION_ROTATE_180 -> 180f
            ExifInterface.ORIENTATION_ROTATE_270 -> 270f
            else -> 0f
        }
        val scale = minOf(1f, MAX_EDGE.toFloat() / maxOf(raw.width, raw.height))
        val m = Matrix().apply { postScale(scale, scale); postRotate(rotation) }
        val bmp = if (rotation == 0f && scale == 1f) raw else Bitmap.createBitmap(raw, 0, 0, raw.width, raw.height, m, true)
        ByteArrayOutputStream().use { out ->
            bmp.compress(Bitmap.CompressFormat.JPEG, 88, out)
            out.toByteArray()
        }
    }.getOrNull().also { file.delete() }
}
