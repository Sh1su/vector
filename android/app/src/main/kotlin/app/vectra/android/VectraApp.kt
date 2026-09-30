package app.vectra.android

import android.app.Application
import app.vectra.android.data.AppContainer
import app.vectra.android.sync.SyncWorker

class VectraApp : Application() {
    lateinit var container: AppContainer
        private set

    override fun onCreate() {
        super.onCreate()
        container = AppContainer(this)
        if (container.hasSession) SyncWorker.schedulePeriodic(this)
    }
}
