package app.vectra.android.data

import android.content.Context
import app.vectra.core.net.ApiClient
import app.vectra.core.net.SessionCookieJar
import app.vectra.core.outbox.Outbox
import app.vectra.core.outbox.Transport
import app.vectra.core.repo.JsonCache
import app.vectra.core.repo.VectraRepository
import kotlinx.coroutines.flow.MutableStateFlow

class RoomJsonCache(private val dao: CacheDao) : JsonCache {
    override suspend fun get(key: String) = dao.get(key)?.let { it.json to it.updatedAt }
    override suspend fun put(key: String, json: String) = dao.put(CacheRow(key, json, System.currentTimeMillis()))
    override suspend fun clear() = dao.clear()
}

/** Einfache manuelle Abhängigkeitsverwaltung: eine Instanz je Prozess, gehalten von VectraApp. */
class AppContainer(context: Context) {
    val prefs = Prefs(context)
    val db = VectraDb.create(context)
    val outboxStore = RoomOutboxStore(db.outbox())
    private val cookies = SessionCookieJar(prefs) { prefs.server.takeIf { it.isNotBlank() } }

    /** Netzverbindung laut System; VectraApp hält den Wert aktuell. */
    val online = MutableStateFlow(true)

    @Volatile private var current: Pair<String, VectraRepository>? = null

    /** Repository für den eingestellten Server; wird bei einem Serverwechsel neu aufgebaut. */
    fun repository(): VectraRepository? {
        val server = prefs.server.takeIf { it.isNotBlank() } ?: return null
        current?.let { (s, r) -> if (s == server) return r }
        val api = ApiClient(server, cookies)
        val repo = VectraRepository(api, RoomJsonCache(db.cache()), Outbox(outboxStore, Transport(api::raw)))
        current = server to repo
        return repo
    }

    val hasSession: Boolean get() = prefs.server.isNotBlank() && cookies.hasSession

    suspend fun signOut() {
        runCatching { repository()?.api?.logout() }
        cookies.clear()
        db.outbox().clear()
        db.cache().clear()
        prefs.clearSession()
    }
}
