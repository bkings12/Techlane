package com.techlane.pos.domain.model

/**
 * The dashboard is an operational answer to "what do I do next?", not a
 * reporting surface. Every number here is a queue a technician can act on, and
 * every one of them is derived from the jobs already cached on the device —
 * there is no dashboard endpoint, because `/repairs` already carries all of it
 * and a second aggregation would be one more thing to keep in step.
 */
data class DashboardSummary(
    val myJobs: Int = 0,
    val awaitingDiagnosis: Int = 0,
    val awaitingApproval: Int = 0,
    val waitingParts: Int = 0,
    val readyForCollection: Int = 0,
) {
    /** Cards map straight onto board filters so a tap lands somewhere useful. */
    fun tiles(): List<DashboardTile> = listOf(
        DashboardTile("My jobs", myJobs, JobFilter.Mine),
        DashboardTile("Awaiting diagnosis", awaitingDiagnosis, JobFilter.AwaitingDiagnosis),
        DashboardTile("Awaiting approval", awaitingApproval, JobFilter.AwaitingApproval),
        DashboardTile("Waiting parts", waitingParts, JobFilter.WaitingParts),
        DashboardTile("Ready", readyForCollection, JobFilter.Ready),
    )
}

data class DashboardTile(val label: String, val count: Int, val filter: JobFilter)

/** Severity drives colour. Reserved for things that genuinely cost the shop. */
enum class AttentionLevel { Info, Warning, Urgent }

/**
 * One actionable line in "Needs attention".
 *
 * [target] is where tapping goes. A row with nothing to open is noise, so
 * everything here resolves to a filter or a specific job.
 */
data class AttentionItem(
    val id: String,
    val count: Int,
    val label: String,
    val level: AttentionLevel,
    val target: AttentionTarget,
)

sealed interface AttentionTarget {
    data class Board(val filter: JobFilter) : AttentionTarget
    data class Job(val jobId: String) : AttentionTarget
    /** Pending local changes — resolved by connectivity, not by navigation. */
    data object Sync : AttentionTarget
}

/** A line on the dashboard's recent-activity list. */
data class RecentActivity(
    val id: String,
    val at: Long,
    val title: String,
    val detail: String?,
    val jobId: String?,
    val kind: TimelineKind,
)

/** Contextual shortcuts. Kept to four so this stays a bar, not a grid. */
enum class QuickAction(val label: String) {
    Scan("Scan"),
    FindJob("Find job"),
    NewSale("New sale"),
    NewIntake("New repair"),
}

data class DashboardData(
    val summary: DashboardSummary = DashboardSummary(),
    val attention: List<AttentionItem> = emptyList(),
    val myJobs: List<JobSummary> = emptyList(),
    val activity: List<RecentActivity> = emptyList(),
    val pendingSync: Int = 0,
) {
    val isEmpty: Boolean
        get() = summary.tiles().all { it.count == 0 } && myJobs.isEmpty() && activity.isEmpty()
}

/**
 * Builds the dashboard from the jobs this device already has.
 *
 * Pure and separately testable on purpose: the thresholds below are judgement
 * calls about what "needs attention" means in a repair shop, and they should be
 * easy to argue with and change.
 */
object DashboardRules {

    /** A promise inside this window is close enough to chase. */
    const val DUE_SOON_MS = 2 * 60 * 60 * 1000L

    /** A finished device sitting this long is a conversation, not a queue. */
    const val UNCOLLECTED_STALE_MS = 3 * 24 * 60 * 60 * 1000L

    fun summarise(jobs: List<JobSummary>, meId: String?): DashboardSummary = DashboardSummary(
        myJobs = jobs.count { it.technicianId != null && it.technicianId == meId && it.status.isOpen },
        awaitingDiagnosis = jobs.count { it.status == JobStatus.Intake },
        awaitingApproval = jobs.count { it.awaitingApproval && it.status.isOpen },
        waitingParts = jobs.count { it.status == JobStatus.WaitingParts },
        readyForCollection = jobs.count { it.status == JobStatus.ReadyForPickup },
    )

    fun attention(
        jobs: List<JobSummary>,
        pendingSync: Int,
        stuckSync: Int,
        now: Long = System.currentTimeMillis(),
    ): List<AttentionItem> = buildList {
        val overdue = jobs.filter { it.status.isOpen && it.promisedBy != null && it.promisedBy < now }
        if (overdue.isNotEmpty()) {
            add(
                AttentionItem(
                    id = "overdue",
                    count = overdue.size,
                    label = if (overdue.size == 1) "Repair overdue" else "Repairs overdue",
                    level = AttentionLevel.Urgent,
                    target = if (overdue.size == 1) {
                        AttentionTarget.Job(overdue.first().id)
                    } else {
                        AttentionTarget.Board(JobFilter.All)
                    },
                ),
            )
        }

        val waiting = jobs.filter { it.isUrgent }
        if (waiting.isNotEmpty()) {
            add(
                AttentionItem(
                    id = "customer-waiting",
                    count = waiting.size,
                    label = if (waiting.size == 1) "Customer waiting at the counter" else "Customers waiting",
                    level = AttentionLevel.Urgent,
                    target = if (waiting.size == 1) {
                        AttentionTarget.Job(waiting.first().id)
                    } else {
                        AttentionTarget.Board(JobFilter.All)
                    },
                ),
            )
        }

        // A prompt still queued after several attempts will not fix itself.
        if (stuckSync > 0) {
            add(
                AttentionItem(
                    id = "sync-stuck",
                    count = stuckSync,
                    label = if (stuckSync == 1) "Change the server rejected" else "Changes the server rejected",
                    level = AttentionLevel.Warning,
                    target = AttentionTarget.Sync,
                ),
            )
        }

        val dueSoon = jobs.filter {
            it.status.isOpen && it.promisedBy != null &&
                it.promisedBy in now..(now + DUE_SOON_MS)
        }
        if (dueSoon.isNotEmpty()) {
            add(
                AttentionItem(
                    id = "due-soon",
                    count = dueSoon.size,
                    label = "Due within two hours",
                    level = AttentionLevel.Warning,
                    target = AttentionTarget.Board(JobFilter.All),
                ),
            )
        }

        val approvals = jobs.filter { it.awaitingApproval && it.status.isOpen }
        if (approvals.isNotEmpty()) {
            add(
                AttentionItem(
                    id = "approvals",
                    count = approvals.size,
                    label = "Waiting customer approval",
                    level = AttentionLevel.Warning,
                    target = AttentionTarget.Board(JobFilter.AwaitingApproval),
                ),
            )
        }

        val diagnosis = jobs.filter { it.status == JobStatus.Intake }
        if (diagnosis.isNotEmpty()) {
            add(
                AttentionItem(
                    id = "diagnosis",
                    count = diagnosis.size,
                    label = "Awaiting diagnosis",
                    level = AttentionLevel.Info,
                    target = AttentionTarget.Board(JobFilter.AwaitingDiagnosis),
                ),
            )
        }

        val parts = jobs.filter { it.status == JobStatus.WaitingParts }
        if (parts.isNotEmpty()) {
            add(
                AttentionItem(
                    id = "parts",
                    count = parts.size,
                    label = "Waiting on parts",
                    level = AttentionLevel.Info,
                    target = AttentionTarget.Board(JobFilter.WaitingParts),
                ),
            )
        }

        val uncollected = jobs.filter {
            it.status == JobStatus.ReadyForPickup && now - it.createdAt > UNCOLLECTED_STALE_MS
        }
        if (uncollected.isNotEmpty()) {
            add(
                AttentionItem(
                    id = "uncollected",
                    count = uncollected.size,
                    label = "Ready but not collected",
                    level = AttentionLevel.Info,
                    target = AttentionTarget.Board(JobFilter.Ready),
                ),
            )
        }

        // Ordinary queued work is shown by the sync chip, not escalated here.
        if (pendingSync > 0 && stuckSync == 0) {
            add(
                AttentionItem(
                    id = "sync-pending",
                    count = pendingSync,
                    label = if (pendingSync == 1) "Change waiting to sync" else "Changes waiting to sync",
                    level = AttentionLevel.Info,
                    target = AttentionTarget.Sync,
                ),
            )
        }
    }

    /**
     * My open jobs, most urgent first: customers standing at the counter, then
     * anything overdue, then whatever is promised soonest.
     */
    fun myJobs(jobs: List<JobSummary>, meId: String?, limit: Int = 4): List<JobSummary> = jobs
        .filter { it.status.isOpen && (meId == null || it.technicianId == meId) }
        .sortedWith(
            compareByDescending<JobSummary> { it.isUrgent }
                .thenByDescending { it.isOverdue }
                .thenBy { it.promisedBy ?: Long.MAX_VALUE }
                .thenByDescending { it.createdAt },
        )
        .take(limit)

    fun quickActions(canCreateIntake: Boolean): List<QuickAction> = buildList {
        add(QuickAction.Scan)
        add(QuickAction.FindJob)
        add(QuickAction.NewSale)
        if (canCreateIntake) add(QuickAction.NewIntake)
    }
}
