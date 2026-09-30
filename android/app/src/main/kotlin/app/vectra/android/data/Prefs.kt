package app.vectra.android.data

import android.content.Context
import app.vectra.android.feature.ThemeMode
import app.vectra.core.net.CookieStorage

/**
 * Einstellungen im privaten App-Speicher (nicht gesichert, siehe data_extraction_rules).
 * Die Sitzungs-Cookies sind eine Übergangslösung bis zum API-Token (`POST /auth/token`), das dann im
 * Android-Keystore verschlüsselt abgelegt wird.
 */
class Prefs(context: Context) : CookieStorage {
    private val sp = context.getSharedPreferences("vectra", Context.MODE_PRIVATE)

    var server: String
        get() = sp.getString("server", "") ?: ""
        set(v) = sp.edit().putString("server", v).apply()

    var accountName: String
        get() = sp.getString("account_name", "") ?: ""
        set(v) = sp.edit().putString("account_name", v).apply()

    var accountEmail: String
        get() = sp.getString("account_email", "") ?: ""
        set(v) = sp.edit().putString("account_email", v).apply()

    var vehicleId: String?
        get() = sp.getString("vehicle_id", null)
        set(v) = sp.edit().putString("vehicle_id", v).apply()

    var theme: ThemeMode
        get() = runCatching { ThemeMode.valueOf(sp.getString("theme", ThemeMode.System.name)!!) }.getOrDefault(ThemeMode.System)
        set(v) = sp.edit().putString("theme", v.name).apply()

    override fun load(): List<String> = sp.getStringSet("cookies", emptySet())!!.toList()
    override fun save(cookies: List<String>) = sp.edit().putStringSet("cookies", cookies.toSet()).apply()

    fun clearSession() {
        sp.edit().remove("cookies").remove("account_name").remove("account_email").remove("vehicle_id").apply()
    }
}
