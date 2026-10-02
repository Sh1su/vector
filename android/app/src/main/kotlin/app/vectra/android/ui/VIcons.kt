package app.vectra.android.ui

// Generiert aus web/src/components/Icon.tsx, ergänzt um more, shield und cloudoff aus dem Icon-Set
// der Vorlage „Vectra Brand“:
// Outline, 24er-Raster, runde Enden. Nicht von Hand ändern, sondern die Web-Quelle anpassen und neu erzeugen.

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.graphics.vector.addPathNodes
import androidx.compose.ui.unit.dp

private fun icon(name: String, vararg paths: Pair<String, Boolean>): ImageVector {
    val b = ImageVector.Builder(name = name, defaultWidth = 24.dp, defaultHeight = 24.dp, viewportWidth = 24f, viewportHeight = 24f)
    for ((d, filled) in paths) {
        if (filled) {
            b.addPath(pathData = addPathNodes(d), fill = SolidColor(Color.Black))
        } else {
            b.addPath(
                pathData = addPathNodes(d),
                stroke = SolidColor(Color.Black),
                strokeLineWidth = 1.8f,
                strokeLineCap = StrokeCap.Round,
                strokeLineJoin = StrokeJoin.Round,
            )
        }
    }
    return b.build()
}

object VIcons {
    val home: ImageVector by lazy { icon("home", "M3 11l9-7 9 7v9a1 1 0 0 1-1 1h-5v-6h-6v6H4a1 1 0 0 1-1-1z" to false) }
    val car: ImageVector by lazy { icon("car", "M5 11l2-5h10l2 5" to false, "M5 11h14a2 2 0 0 1 2 2v2a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2v-2a2 2 0 0 1 2 -2z" to false, "M6 17v2M18 17v2" to false, "M7 14h.01M17 14h.01" to false) }
    val route: ImageVector by lazy { icon("route", "M4 18a2 2 0 1 0 4 0a2 2 0 1 0 -4 0z" to false, "M16 6a2 2 0 1 0 4 0a2 2 0 1 0 -4 0z" to false, "M8 18h7a3 3 0 0 0 0-6H9a3 3 0 0 1 0-6h7" to false) }
    val gauge: ImageVector by lazy { icon("gauge", "M4 17a8 8 0 1 1 16 0" to false, "M12 17l4-5" to false, "M7 17h10" to false) }
    val fuel: ImageVector by lazy { icon("fuel", "M5 20V5a1 1 0 0 1 1-1h7a1 1 0 0 1 1 1v15" to false, "M4 20h11M5 10h9" to false, "M14 9h2a2 2 0 0 1 2 2v5a1.5 1.5 0 0 0 3 0V8l-3-3" to false) }
    val oil: ImageVector by lazy { icon("oil", "M12 3s6 7 6 11a6 6 0 0 1-12 0c0-4 6-11 6-11z" to false, "M9.5 14.5a2.5 2.5 0 0 0 2.5 2.5" to false) }
    val wrench: ImageVector by lazy { icon("wrench", "M14.5 6.5a4 4 0 0 0-5.3 5.3L4 17l3 3 5.2-5.2a4 4 0 0 0 5.3-5.3l-2.5 2.5-2.5-.5-.5-2.5z" to false) }
    val euro: ImageVector by lazy { icon("euro", "M17 6.5A7 7 0 1 0 17 17.5" to false, "M4 10h9M4 14h9" to false) }
    val doc: ImageVector by lazy { icon("doc", "M7 3h7l5 5v13H7z" to false, "M14 3v5h5M10 13h6M10 17h6" to false) }
    val sparkles: ImageVector by lazy { icon("sparkles", "M11 3l1.8 4.7 4.7 1.8-4.7 1.8L11 16l-1.8-4.7L4.5 9.5l4.7-1.8z" to false, "M18.5 14l.8 2.2 2.2.8-2.2.8-.8 2.2-.8-2.2-2.2-.8 2.2-.8z" to false) }
    val gear: ImageVector by lazy { icon("gear", "M9 12a3 3 0 1 0 6 0a3 3 0 1 0 -6 0z" to false, "M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z" to false) }
    val plus: ImageVector by lazy { icon("plus", "M12 5v14M5 12h14" to false) }
    val camera: ImageVector by lazy { icon("camera", "M4 8h3l2-3h6l2 3h3v11H4z" to false, "M8.5 13a3.5 3.5 0 1 0 7 0a3.5 3.5 0 1 0 -7 0z" to false) }
    val right: ImageVector by lazy { icon("right", "M9 6l6 6-6 6" to false) }
    val left: ImageVector by lazy { icon("left", "M15 6l-6 6 6 6" to false) }
    val down: ImageVector by lazy { icon("down", "M6 9l6 6 6-6" to false) }
    val check: ImageVector by lazy { icon("check", "M5 12l5 5L20 7" to false) }
    val alert: ImageVector by lazy { icon("alert", "M12 3l10 18H2z" to false, "M12 10v4M12 17.5v.5" to false) }
    val calendar: ImageVector by lazy { icon("calendar", "M5 5h14a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2v-12a2 2 0 0 1 2 -2z" to false, "M3 10h18M8 3v4M16 3v4" to false) }
    val clock: ImageVector by lazy { icon("clock", "M3 12a9 9 0 1 0 18 0a9 9 0 1 0 -18 0z" to false, "M12 7v5l3 2" to false) }
    val user: ImageVector by lazy { icon("user", "M8 8a4 4 0 1 0 8 0a4 4 0 1 0 -8 0z" to false, "M4 21a8 8 0 0 1 16 0" to false) }
    val users: ImageVector by lazy { icon("users", "M5.5 8a3.5 3.5 0 1 0 7 0a3.5 3.5 0 1 0 -7 0z" to false, "M2.5 20a6.5 6.5 0 0 1 13 0" to false, "M16 4.5a3.5 3.5 0 0 1 0 7M18 14a6.5 6.5 0 0 1 3.5 6" to false) }
    val lock: ImageVector by lazy { icon("lock", "M7 11h10a2 2 0 0 1 2 2v6a2 2 0 0 1 -2 2h-10a2 2 0 0 1 -2 -2v-6a2 2 0 0 1 2 -2z" to false, "M8 11V7a4 4 0 0 1 8 0v4" to false) }
    val bell: ImageVector by lazy { icon("bell", "M6 16v-5a6 6 0 0 1 12 0v5l2 2H4z" to false, "M10 20a2 2 0 0 0 4 0" to false) }
    val search: ImageVector by lazy { icon("search", "M4 11a7 7 0 1 0 14 0a7 7 0 1 0 -14 0z" to false, "M20 20l-4-4" to false) }
    val edit: ImageVector by lazy { icon("edit", "M4 20h4L19 9l-4-4L4 16z" to false, "M13.5 6.5l4 4" to false) }
    val info: ImageVector by lazy { icon("info", "M3 12a9 9 0 1 0 18 0a9 9 0 1 0 -18 0z" to false, "M12 11v5M12 7.5v.5" to false) }
    val moon: ImageVector by lazy { icon("moon", "M20 14.5A8 8 0 1 1 9.5 4a6.5 6.5 0 0 0 10.5 10.5z" to false) }
    val logout: ImageVector by lazy { icon("logout", "M15 4h4v16h-4M10 8l-4 4 4 4M6 12h10" to false) }
    val key: ImageVector by lazy { icon("key", "M4 15a4 4 0 1 0 8 0a4 4 0 1 0 -8 0z" to false, "M11 12l9-9M17 6l3 3" to false) }
    val receipt: ImageVector by lazy { icon("receipt", "M6 3h12v18l-3-2-3 2-3-2-3 2z" to false, "M9 8h6M9 12h6" to false) }
    val sync: ImageVector by lazy { icon("sync", "M20 11a8 8 0 0 0-14-5L4 8M4 4v4h4" to false, "M4 13a8 8 0 0 0 14 5l2-2M20 20v-4h-4" to false) }
    val close: ImageVector by lazy { icon("x", "M6 6l12 12M18 6L6 18" to false) }
    val garage: ImageVector by lazy { icon("garage", "M3 21V9l9-5 9 5v12" to false, "M7 21v-8h10v8M7 16h10" to false) }
    val scan: ImageVector by lazy { icon("scan", "M4 8V5a1 1 0 0 1 1-1h3M16 4h3a1 1 0 0 1 1 1v3M20 16v3a1 1 0 0 1-1 1h-3M8 20H5a1 1 0 0 1-1-1v-3" to false, "M8 12h8" to false) }
    val trend: ImageVector by lazy { icon("trend", "M3 17l6-6 4 4 8-8" to false, "M15 7h6v6" to false) }
    val mail: ImageVector by lazy { icon("mail", "M5 5h14a2 2 0 0 1 2 2v10a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2v-10a2 2 0 0 1 2 -2z" to false, "M3 7l9 6 9-6" to false) }
    val sun: ImageVector by lazy { icon("sun", "M8 12a4 4 0 1 0 8 0a4 4 0 1 0 -8 0z" to false, "M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" to false) }
    val play: ImageVector by lazy { icon("play", "M8 5v14l11-7z" to true) }
    val mic: ImageVector by lazy { icon("mic", "M12 3a3 3 0 0 1 3 3v5a3 3 0 0 1 -6 0v-5a3 3 0 0 1 3 -3z" to false, "M5 11a7 7 0 0 0 14 0M12 18v3M9 21h6" to false) }
    val send: ImageVector by lazy { icon("send", "M4 12l16-8-6 16-3-7z" to false, "M11 13l9-9" to false) }
    val more: ImageVector by lazy { icon("more", "M3.4 12a1.6 1.6 0 1 0 3.2 0a1.6 1.6 0 1 0 -3.2 0z" to true, "M10.4 12a1.6 1.6 0 1 0 3.2 0a1.6 1.6 0 1 0 -3.2 0z" to true, "M17.4 12a1.6 1.6 0 1 0 3.2 0a1.6 1.6 0 1 0 -3.2 0z" to true) }
    val shield: ImageVector by lazy { icon("shield", "M12 3l8 3v6c0 5-3.5 8-8 9-4.5-1-8-4-8-9V6z" to false, "M9 12l2 2 4-4" to false) }
    val cloudoff: ImageVector by lazy { icon("cloudoff", "M3 3l18 18" to false, "M8 8a5 5 0 0 0-1 10h10M20 16a4 4 0 0 0-5-6 6 6 0 0 0-4-3" to false) }
}
