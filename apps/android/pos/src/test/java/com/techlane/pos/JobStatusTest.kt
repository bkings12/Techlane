package com.techlane.pos

import com.techlane.pos.domain.model.ClosureReason
import com.techlane.pos.domain.model.JobStatus
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * These pin the Android status model to `internal/repair/status.go`.
 *
 * The risk this guards is drift: if the backend adds a transition and this
 * table does not, the technician is shown an option the server will refuse —
 * and if it removes one, an action silently disappears from the bench.
 */
class JobStatusTest {

    @Test
    fun `wire values match the backend constants`() {
        assertEquals("intake", JobStatus.Intake.wire)
        assertEquals("diagnosed", JobStatus.Diagnosed.wire)
        assertEquals("waiting_parts", JobStatus.WaitingParts.wire)
        assertEquals("in_progress", JobStatus.InProgress.wire)
        assertEquals("ready_for_pickup", JobStatus.ReadyForPickup.wire)
        assertEquals("completed", JobStatus.Completed.wire)
        assertEquals("collected", JobStatus.Collected.wire)
        assertEquals("cancelled", JobStatus.Cancelled.wire)
        assertEquals("unrepairable", JobStatus.Unrepairable.wire)
    }

    @Test
    fun `transitions mirror allowedTransitions in status_go`() {
        assertEquals(
            listOf(
                JobStatus.Diagnosed,
                JobStatus.WaitingParts,
                JobStatus.InProgress,
                JobStatus.Cancelled,
                JobStatus.Unrepairable,
            ),
            JobStatus.Intake.allowedNext,
        )
        assertEquals(
            listOf(JobStatus.InProgress, JobStatus.Cancelled, JobStatus.Unrepairable),
            JobStatus.WaitingParts.allowedNext,
        )
        assertEquals(
            listOf(
                JobStatus.ReadyForPickup,
                JobStatus.WaitingParts,
                JobStatus.Cancelled,
                JobStatus.Unrepairable,
            ),
            JobStatus.InProgress.allowedNext,
        )
        assertEquals(listOf(JobStatus.Collected), JobStatus.Completed.allowedNext)
        assertTrue(JobStatus.Collected.allowedNext.isEmpty())
    }

    @Test
    fun `a completed job cannot jump back to the bench`() {
        assertFalse(JobStatus.InProgress in JobStatus.Completed.allowedNext)
        assertFalse(JobStatus.Diagnosed in JobStatus.Completed.allowedNext)
    }

    @Test
    fun `closure and open states classify the way the board expects`() {
        assertTrue(JobStatus.Cancelled.isClosure)
        assertTrue(JobStatus.Unrepairable.isClosure)
        assertFalse(JobStatus.Collected.isClosure)
        assertFalse(JobStatus.Collected.isOpen)
        assertTrue(JobStatus.InProgress.isOpen)
    }

    @Test
    fun `irreversible statuses ask for confirmation`() {
        listOf(
            JobStatus.ReadyForPickup,
            JobStatus.Completed,
            JobStatus.Collected,
            JobStatus.Cancelled,
            JobStatus.Unrepairable,
        ).forEach { assertTrue("${it.label} should confirm", it.needsConfirmation) }
        assertFalse(JobStatus.Diagnosed.needsConfirmation)
    }

    @Test
    fun `closure reasons are scoped to their status`() {
        val cancelled = ClosureReason.forStatus(JobStatus.Cancelled)
        assertTrue(cancelled.isNotEmpty())
        assertTrue(cancelled.all { it.status == JobStatus.Cancelled })
        assertTrue(
            ClosureReason.forStatus(JobStatus.Unrepairable)
                .any { it.wire == "beyond_economical_repair" },
        )
    }

    @Test
    fun `unknown wire values fall back rather than crash`() {
        assertEquals(JobStatus.Intake, JobStatus.fromWire("something_new"))
        assertEquals(JobStatus.Intake, JobStatus.fromWire(null))
        assertEquals(JobStatus.InProgress, JobStatus.fromWire("IN_PROGRESS"))
    }
}
