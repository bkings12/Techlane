package com.techlane.pos

import com.techlane.pos.domain.model.AttentionLevel
import com.techlane.pos.domain.model.AttentionTarget
import com.techlane.pos.domain.model.DashboardRules
import com.techlane.pos.domain.model.JobStatus
import com.techlane.pos.domain.model.JobSummary
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * The dashboard has no backend of its own — it is a fold over the jobs already
 * cached from `/repairs`. These pin that fold, since a wrong count here is a
 * technician trusting a queue that lies about what is actually on the bench.
 */
class DashboardRulesTest {

    private fun job(
        status: JobStatus,
        technicianId: String? = null,
        promisedBy: Long? = null,
        customerWaiting: Boolean = false,
        awaitingApproval: Boolean = false,
        createdAt: Long = 0L,
        id: String = "job-${status.wire}-$technicianId-$promisedBy-$createdAt",
    ) = JobSummary(
        id = id,
        jobCode = "TL-$id",
        customerName = "Customer",
        customerPhone = null,
        deviceLabel = "Device",
        imei = null,
        serialNumber = null,
        status = status,
        technicianId = technicianId,
        technicianName = null,
        promisedBy = promisedBy,
        createdAt = createdAt,
        customerWaiting = customerWaiting,
        awaitingApproval = awaitingApproval,
        partsPending = status == JobStatus.WaitingParts,
        amountDue = 0.0,
    )

    // ------------------------------------------------------------------ tiles

    @Test
    fun `summary counts each queue independently`() {
        val jobs = listOf(
            job(JobStatus.Intake),
            job(JobStatus.Intake),
            job(JobStatus.WaitingParts),
            job(JobStatus.ReadyForPickup),
            job(JobStatus.Diagnosed, awaitingApproval = true),
            job(JobStatus.InProgress, technicianId = "me"),
            job(JobStatus.Collected, technicianId = "me"), // closed — must not count as "my jobs"
        )
        val summary = DashboardRules.summarise(jobs, meId = "me")

        assertEquals(1, summary.myJobs)
        assertEquals(2, summary.awaitingDiagnosis)
        assertEquals(1, summary.awaitingApproval)
        assertEquals(1, summary.waitingParts)
        assertEquals(1, summary.readyForCollection)
    }

    @Test
    fun `with no signed-in technician my jobs is zero rather than everyone's`() {
        val jobs = listOf(job(JobStatus.InProgress, technicianId = "someone-else"))
        assertEquals(0, DashboardRules.summarise(jobs, meId = null).myJobs)
    }

    // -------------------------------------------------------------- attention

    @Test
    fun `overdue and waiting-customer jobs are urgent`() {
        val now = 1_000_000L
        val overdue = job(JobStatus.InProgress, promisedBy = now - 60_000, createdAt = now)
        val attention = DashboardRules.attention(listOf(overdue), pendingSync = 0, stuckSync = 0, now = now)

        val overdueItem = attention.first { it.id == "overdue" }
        assertEquals(AttentionLevel.Urgent, overdueItem.level)
        assertEquals(1, overdueItem.count)
        assertTrue(overdueItem.target is AttentionTarget.Job)
    }

    @Test
    fun `a collected job never counts as overdue`() {
        val now = 1_000_000L
        val collected = job(JobStatus.Collected, promisedBy = now - 60_000, createdAt = now)
        val attention = DashboardRules.attention(listOf(collected), pendingSync = 0, stuckSync = 0, now = now)
        assertFalse(attention.any { it.id == "overdue" })
    }

    @Test
    fun `stuck sync actions outrank ordinary pending ones and both are not shown together`() {
        val attention = DashboardRules.attention(emptyList(), pendingSync = 3, stuckSync = 2)
        assertTrue(attention.any { it.id == "sync-stuck" && it.level == AttentionLevel.Warning })
        // A stuck action is also a pending one; showing both would double-count it.
        assertFalse(attention.any { it.id == "sync-pending" })
    }

    @Test
    fun `ordinary pending sync shows only when nothing is stuck`() {
        val attention = DashboardRules.attention(emptyList(), pendingSync = 3, stuckSync = 0)
        assertTrue(attention.any { it.id == "sync-pending" && it.level == AttentionLevel.Info })
    }

    @Test
    fun `jobs due within the window are flagged before they become overdue`() {
        val now = 1_000_000L
        val dueSoon = job(JobStatus.InProgress, promisedBy = now + 30 * 60_000, createdAt = now)
        val attention = DashboardRules.attention(listOf(dueSoon), pendingSync = 0, stuckSync = 0, now = now)
        assertTrue(attention.any { it.id == "due-soon" })
        assertFalse(attention.any { it.id == "overdue" })
    }

    @Test
    fun `ready jobs sitting uncollected past the stale window are surfaced`() {
        val now = System.currentTimeMillis()
        val staleReady = job(
            JobStatus.ReadyForPickup,
            createdAt = now - DashboardRules.UNCOLLECTED_STALE_MS - 1_000,
        )
        val attention = DashboardRules.attention(listOf(staleReady), pendingSync = 0, stuckSync = 0, now = now)
        assertTrue(attention.any { it.id == "uncollected" })
    }

    @Test
    fun `an empty board produces no attention items`() {
        assertTrue(DashboardRules.attention(emptyList(), pendingSync = 0, stuckSync = 0).isEmpty())
    }

    // ---------------------------------------------------------------- my jobs

    @Test
    fun `my jobs surfaces the customer-waiting job first regardless of promise time`() {
        val now = 1_000_000L
        val waiting = job(
            JobStatus.InProgress, technicianId = "me", customerWaiting = true,
            promisedBy = now + 60 * 60_000, createdAt = now, id = "waiting",
        )
        val soonerPromise = job(
            JobStatus.InProgress, technicianId = "me",
            promisedBy = now + 10 * 60_000, createdAt = now, id = "sooner",
        )
        val ranked = DashboardRules.myJobs(listOf(soonerPromise, waiting), meId = "me")
        assertEquals("waiting", ranked.first().id)
    }

    @Test
    fun `my jobs excludes closed jobs and other technicians' work`() {
        val mine = job(JobStatus.InProgress, technicianId = "me", id = "mine")
        val closed = job(JobStatus.Collected, technicianId = "me", id = "closed")
        val theirs = job(JobStatus.InProgress, technicianId = "them", id = "theirs")
        val ranked = DashboardRules.myJobs(listOf(mine, closed, theirs), meId = "me")
        assertEquals(listOf("mine"), ranked.map { it.id })
    }

    @Test
    fun `with no technician filter my jobs shows the whole open board`() {
        val a = job(JobStatus.InProgress, technicianId = "a", id = "a")
        val b = job(JobStatus.InProgress, technicianId = "b", id = "b")
        val ranked = DashboardRules.myJobs(listOf(a, b), meId = null)
        assertEquals(2, ranked.size)
    }

    // ----------------------------------------------------------- quick actions

    @Test
    fun `new repair only appears for roles that can create intake`() {
        assertFalse(DashboardRules.quickActions(canCreateIntake = false).any { it.label == "New repair" })
        assertTrue(DashboardRules.quickActions(canCreateIntake = true).any { it.label == "New repair" })
    }

    @Test
    fun `quick actions never grow into a giant grid`() {
        assertTrue(DashboardRules.quickActions(canCreateIntake = true).size <= 4)
    }
}
