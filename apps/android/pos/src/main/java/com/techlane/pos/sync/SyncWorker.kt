package com.techlane.pos.sync

import android.content.Context
import androidx.hilt.work.HiltWorker
import androidx.work.Constraints
import androidx.work.CoroutineWorker
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.NetworkType
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import com.techlane.pos.data.repository.ShopRepository
import com.techlane.pos.data.session.PreferencesStore
import com.techlane.pos.data.session.SecureTokenStore
import dagger.assisted.Assisted
import dagger.assisted.AssistedInject
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.flow.first
import java.util.concurrent.TimeUnit
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Keeps the offline catalog warm in the background so the product picker is
 * useful the moment a technician opens it, even on a bad shop connection.
 */
@HiltWorker
class CatalogSyncWorker @AssistedInject constructor(
    @Assisted context: Context,
    @Assisted params: WorkerParameters,
    private val shop: ShopRepository,
    private val prefs: PreferencesStore,
    private val tokens: SecureTokenStore,
) : CoroutineWorker(context, params) {

    override suspend fun doWork(): Result {
        if (tokens.refreshToken == null) return Result.success()
        val locationId = prefs.preferences.first().locationId ?: return Result.success()
        return shop.syncCatalog(locationId).fold(
            onSuccess = { Result.success() },
            // Retry rather than fail: a shop that was offline at 09:00 is usually
            // back by 09:15, and WorkManager's backoff handles that for us.
            onFailure = { Result.retry() },
        )
    }

    companion object {
        const val NAME = "techlane-pos-catalog-sync"
    }
}

@Singleton
class SyncScheduler @Inject constructor(
    @ApplicationContext private val context: Context,
) {
    fun schedulePeriodicSync() {
        // 30 minutes, not 6 hours — this is the last-resort net for a phone left
        // idle or backgrounded; the charge screen also syncs on its own whenever
        // it's opened or resumed (see QuickChargeViewModel.syncCatalogIfStale).
        val request = PeriodicWorkRequestBuilder<CatalogSyncWorker>(30, TimeUnit.MINUTES)
            .setConstraints(
                Constraints.Builder()
                    .setRequiredNetworkType(NetworkType.CONNECTED)
                    .build(),
            )
            .build()
        // UPDATE (not KEEP) so phones that already had the old 6-hour job
        // scheduled from an earlier install pick up the new interval too.
        WorkManager.getInstance(context).enqueueUniquePeriodicWork(
            CatalogSyncWorker.NAME,
            ExistingPeriodicWorkPolicy.UPDATE,
            request,
        )
    }
}
