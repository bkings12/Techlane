package com.techlane.pos.sync

import android.content.Context
import androidx.hilt.work.HiltWorker
import androidx.work.BackoffPolicy
import androidx.work.Constraints
import androidx.work.CoroutineWorker
import androidx.work.ExistingWorkPolicy
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import com.techlane.pos.data.repository.JobRepository
import com.techlane.pos.data.session.SecureTokenStore
import dagger.assisted.Assisted
import dagger.assisted.AssistedInject
import java.util.concurrent.TimeUnit

/**
 * Flushes the job outbox once there is a connection.
 *
 * Queued repair work is the technician's actual output, so this retries with
 * backoff rather than giving up; [JobRepository.syncOutbox] is where an action
 * the server will never accept gets parked for a human instead.
 */
@HiltWorker
class JobSyncWorker @AssistedInject constructor(
    @Assisted context: Context,
    @Assisted params: WorkerParameters,
    private val jobs: JobRepository,
    private val tokens: SecureTokenStore,
) : CoroutineWorker(context, params) {

    override suspend fun doWork(): Result {
        if (tokens.refreshToken == null) return Result.success()
        val remaining = runCatching { jobs.syncOutbox() }.getOrElse { return Result.retry() }
        return if (remaining > 0) Result.retry() else Result.success()
    }

    companion object {
        const val NAME = "techlane-pos-job-sync"

        /**
         * Enqueued after every offline mutation. KEEP rather than REPLACE so a
         * burst of edits does not keep pushing the flush further out.
         */
        fun enqueue(context: Context) {
            val request = OneTimeWorkRequestBuilder<JobSyncWorker>()
                .setConstraints(
                    Constraints.Builder().setRequiredNetworkType(NetworkType.CONNECTED).build(),
                )
                .setBackoffCriteria(BackoffPolicy.EXPONENTIAL, 30, TimeUnit.SECONDS)
                .build()
            WorkManager.getInstance(context)
                .enqueueUniqueWork(NAME, ExistingWorkPolicy.KEEP, request)
        }
    }
}
