package com.techlane.pos.data.repository

import com.techlane.pos.data.local.ChargeDao
import com.techlane.pos.data.local.JobDao
import com.techlane.pos.data.session.PreferencesStore
import com.techlane.pos.domain.model.DashboardData
import com.techlane.pos.domain.model.DashboardRules
import com.techlane.pos.domain.model.JobStatus
import com.techlane.pos.domain.model.RecentActivity
import com.techlane.pos.domain.model.TimelineKind
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Assembles the dashboard from what the device already holds.
 *
 * No dashboard endpoint was added. `/repairs` already returns status, promise
 * time, technician and authorization for every job, and [JobRepository] caches
 * all of it — so the counts here are a fold over Room rather than a second
 * server-side aggregation that could quietly disagree with the board.
 *
 * The practical consequence is that the dashboard works with no connection at
 * all, which is the state a back-room bench is usually in.
 */
@Singleton
class DashboardRepository @Inject constructor(
    private val jobs: JobRepository,
    private val jobDao: JobDao,
    private val chargeDao: ChargeDao,
    private val prefs: PreferencesStore,
) {

    fun observe(): Flow<DashboardData> = combine(
        jobs.observeJobs(),
        prefs.preferences.map { it.userId },
        jobDao.observeOutboxCount(),
        jobDao.observeStuck(OUTBOX_STUCK_THRESHOLD).map { it.size },
        recentActivity(),
    ) { allJobs, meId, pending, stuck, activity ->
        DashboardData(
            summary = DashboardRules.summarise(allJobs, meId),
            attention = DashboardRules.attention(allJobs, pending, stuck),
            myJobs = DashboardRules.myJobs(allJobs, meId),
            activity = activity,
            pendingSync = pending,
        )
    }

    /**
     * Recent activity, merged from the two local records that actually know what
     * this device has been doing: job status events and charges taken here.
     * There is no server activity feed, and inventing one would duplicate the
     * per-job timeline the web console already renders.
     */
    private fun recentActivity(): Flow<List<RecentActivity>> = combine(
        jobDao.observeAll(),
        chargeDao.observeRecent(),
    ) { jobRows, charges ->
        val fromJobs = jobRows
            .sortedByDescending { it.createdAt }
            .take(12)
            .map { job ->
                val status = JobStatus.fromWire(job.status)
                RecentActivity(
                    id = "job-${job.id}-${job.status}",
                    at = job.createdAt,
                    title = when (status) {
                        JobStatus.Intake -> "Job received"
                        JobStatus.Diagnosed -> "Diagnosis updated"
                        JobStatus.WaitingParts -> "Waiting on parts"
                        JobStatus.InProgress -> "Repair started"
                        JobStatus.ReadyForPickup -> "Device marked ready"
                        JobStatus.Completed -> "Repair completed"
                        JobStatus.Collected -> "Device collected"
                        JobStatus.Cancelled, JobStatus.Unrepairable -> "Job closed"
                    },
                    detail = listOfNotNull(job.jobCode, job.customerName).joinToString(" · "),
                    jobId = job.id,
                    kind = when (status) {
                        JobStatus.Intake -> TimelineKind.Received
                        JobStatus.Collected -> TimelineKind.Collected
                        JobStatus.Diagnosed -> TimelineKind.Diagnosis
                        else -> TimelineKind.StatusChange
                    },
                )
            }

        val fromCharges = charges.filter { it.status == "paid" }.take(6).map { charge ->
            RecentActivity(
                id = "charge-${charge.id}",
                at = charge.updatedAt,
                title = "Sale completed",
                detail = charge.label,
                jobId = null,
                kind = TimelineKind.CustomerNotified,
            )
        }

        (fromJobs + fromCharges).sortedByDescending { it.at }.take(10)
    }

    /** Pulls the board; the dashboard recomputes itself from the cache. */
    suspend fun refresh(): Result<Unit> = runCatching {
        val branchId = prefs.preferences.first().branchId
        jobs.refreshTechnicians()
        jobs.refreshJobs(branchId = branchId).getOrThrow()
        Unit
    }
}
