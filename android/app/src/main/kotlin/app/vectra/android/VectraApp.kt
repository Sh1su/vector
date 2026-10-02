package app.vectra.android

import android.app.Application
import android.net.ConnectivityManager
import android.net.Network
import android.net.NetworkCapabilities
import app.vectra.android.data.AppContainer
import app.vectra.android.sync.SyncWorker

class VectraApp : Application() {
    lateinit var container: AppContainer
        private set

    override fun onCreate() {
        super.onCreate()
        container = AppContainer(this)
        if (container.hasSession) SyncWorker.schedulePeriodic(this)
        watchNetwork()
    }

    /** Sobald wieder eine Verbindung besteht, sofort synchronisieren (nicht erst beim nächsten Periodenlauf). */
    private fun watchNetwork() {
        val cm = getSystemService(ConnectivityManager::class.java) ?: return
        container.online.value = cm.getNetworkCapabilities(cm.activeNetwork)?.hasCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET) == true
        cm.registerDefaultNetworkCallback(object : ConnectivityManager.NetworkCallback() {
            override fun onAvailable(network: Network) {
                val wasOffline = !container.online.value
                container.online.value = true
                if (wasOffline && container.hasSession) SyncWorker.syncNow(this@VectraApp)
            }

            override fun onLost(network: Network) {
                container.online.value = false
            }
        })
    }
}
