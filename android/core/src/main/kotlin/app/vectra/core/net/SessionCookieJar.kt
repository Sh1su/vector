package app.vectra.core.net

import okhttp3.Cookie
import okhttp3.CookieJar
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull

/** Speicher für die Sitzungs-Cookies. Die App legt sie im privaten Speicher ab, Tests im RAM. */
interface CookieStorage {
    fun load(): List<String>
    fun save(cookies: List<String>)
}

class InMemoryCookieStorage : CookieStorage {
    private var data: List<String> = emptyList()
    override fun load() = data
    override fun save(cookies: List<String>) { data = cookies }
}

/**
 * Hält `vectra_session` und `vectra_csrf` (ADR-015). Übergangslösung, bis `POST /auth/token`
 * umgesetzt ist; danach nutzt die App ein API-Token mit Scopes.
 */
class SessionCookieJar(private val storage: CookieStorage, private val origin: () -> String? = { null }) : CookieJar {
    private val lock = Any()
    private var cookies: MutableList<Cookie> = mutableListOf()
    private var loaded = false

    /**
     * Lädt die gespeicherten Cookies. Ohne Anfrage-URL dient die Server-Adresse ([origin]) als Bezug,
     * damit die App nach einem Neustart sofort erkennt, dass sie noch angemeldet ist.
     */
    private fun ensureLoaded(url: HttpUrl?) {
        if (loaded) return
        val base = url ?: origin()?.toHttpUrlOrNull() ?: return
        cookies = storage.load().mapNotNull { Cookie.parse(base, it) }.toMutableList()
        loaded = true
    }

    override fun saveFromResponse(url: HttpUrl, cookies: List<Cookie>) = synchronized(lock) {
        ensureLoaded(url)
        for (c in cookies) {
            this.cookies.removeAll { it.name == c.name && it.domain == c.domain && it.path == c.path }
            if (c.expiresAt > System.currentTimeMillis()) this.cookies.add(c)
        }
        storage.save(this.cookies.map { it.toString() })
    }

    override fun loadForRequest(url: HttpUrl): List<Cookie> = synchronized(lock) {
        ensureLoaded(url)
        val now = System.currentTimeMillis()
        cookies.removeAll { it.expiresAt <= now }
        cookies.filter { it.matches(url) }
    }

    fun value(name: String): String? = synchronized(lock) {
        ensureLoaded(null)
        val now = System.currentTimeMillis()
        cookies.firstOrNull { it.name == name && it.expiresAt > now }?.value
    }

    fun clear() = synchronized(lock) {
        cookies.clear()
        loaded = true
        storage.save(emptyList())
    }

    val hasSession: Boolean get() = value(SESSION_COOKIE) != null

    companion object {
        const val SESSION_COOKIE = "vectra_session"
        const val CSRF_COOKIE = "vectra_csrf"
    }
}
