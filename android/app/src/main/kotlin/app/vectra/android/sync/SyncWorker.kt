package app.vectra.android.sync

import android.content.Context
import androidx.work.BackoffPolicy
import androidx.work.Constraints
import androidx.work.CoroutineWorker
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.ExistingWorkPolicy
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import app.vectra.android.VectraApp
import java.util.concurrent.TimeUnit

/**
 * Sendet die Outbox (ADR-021): einmalig nach jeder Erfassung und periodisch alle 15 Minuten, jeweils
 * nur mit Netz und mit exponentiellem Backoff bei Netz- oder Serverfehlern.
 */
class SyncWorker(context: Context, params: WorkerParameters) : CoroutineWorker(context, params) {
    override suspend fun doWork(): Result {
        val container = (applicationContext as VectraApp).container
        val repo = container.repository() ?: return Result.success()
        if (!container.hasSession) return Result.success()
        val report = repo.outbox.sync()
        return when {
            report.unauthorized -> Result.failure()
            report.retry -> Result.retry()
            else -> Result.success()
        }
    }

    companion object {
        private val online = Constraints.Builder().setRequiredNetworkType(NetworkType.CONNECTED).build()

        fun syncNow(context: Context) {
            val request = OneTimeWorkRequestBuilder<SyncWorker>()
                .setConstraints(online)
                .setBackoffCriteria(BackoffPolicy.EXPONENTIAL, 30, TimeUnit.SECONDS)
                .build()
            // APPEND_OR_REPLACE: Einträge, die während eines laufenden Syncs entstehen, gehen im nächsten Lauf mit.
            WorkManager.getInstance(context).enqueueUniqueWork("sync", ExistingWorkPolicy.APPEND_OR_REPLACE, request)
        }

        fun schedulePeriodic(context: Context) {
            val request = PeriodicWorkRequestBuilder<SyncWorker>(15, TimeUnit.MINUTES).setConstraints(online).build()
            WorkManager.getInstance(context).enqueueUniquePeriodicWork("sync-periodic", ExistingPeriodicWorkPolicy.KEEP, request)
        }

        fun cancelAll(context: Context) {
            WorkManager.getInstance(context).cancelUniqueWork("sync")
            WorkManager.getInstance(context).cancelUniqueWork("sync-periodic")
        }
    }
}
