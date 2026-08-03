package com.techlane.pos

import com.techlane.pos.domain.model.JobBoardSummary
import com.techlane.pos.domain.model.JobSort
import com.techlane.pos.domain.model.JobStatus
import com.techlane.pos.domain.model.JobSummary
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

private const val HOUR = 60 * 60 * 1000L

private fun job(
    id: String,
    status: JobStatus = JobStatus.InProgress,
    createdAt: Long = 0L,
    promisedBy: Long? = null,
    customerWaiting: Boolean = false,
) = JobSummary(
    id = id,
    jobCode = "JOB-$id",
    customerName = "Customer $id",
    customerPhone = null,
    deviceLabel = "Device",
    imei = null,
    serialNumber = null,
    status = status,
    technicianId = null,
    technicianName = null,
    promisedBy = promisedBy,
    createdAt = createdAt,
    customerWaiting = customerWaiting,
    awaitingApproval = false,
    partsPending = false,
    amountDue = 0.0,
)

class JobBoardSummaryTest {

    @Test
    fun `counts each queue independently`() {
        val summary = JobBoardSummary.from(
            listOf(
                job("1", JobStatus.InProgress),
                job("2", JobStatus.InProgress),
                job("3", JobStatus.WaitingParts),
                job("4", JobStatus.ReadyForPickup),
            ),
        )

        assertEquals(2, summary.onBench)
        assertEquals(1, summary.waitingParts)
        assertEquals(1, summary.ready)
    }

    @Test
    fun `open excludes collected and written-off jobs`() {
        val summary = JobBoardSummary.from(
            listOf(
                job("1", JobStatus.InProgress),
                job("2", JobStatus.Collected),
                job("3", JobStatus.Cancelled),
                job("4", JobStatus.Unrepairable),
            ),
        )

        assertEquals(1, summary.open)
    }

    @Test
    fun `a past promise on an open job counts as overdue`() {
        val summary = JobBoardSummary.from(
            listOf(job("1", JobStatus.InProgress, promisedBy = System.currentTimeMillis() - HOUR)),
        )

        assertEquals(1, summary.overdue)
    }

    @Test
    fun `a past promise on a collected job is not overdue`() {
        // The device is already back with the customer — nothing is owed, so
        // flagging it red would be noise on the board forever.
        val summary = JobBoardSummary.from(
            listOf(job("1", JobStatus.Collected, promisedBy = System.currentTimeMillis() - HOUR)),
        )

        assertEquals(0, summary.overdue)
    }

    @Test
    fun `an empty board reports empty`() {
        assertTrue(JobBoardSummary.from(emptyList()).isEmpty)
    }
}

class JobSortTest {

    @Test
    fun `newest puts the most recent job first`() {
        val sorted = JobSort.Newest.sort(listOf(job("old", createdAt = 1), job("new", createdAt = 99)))

        assertEquals("new", sorted.first().id)
    }

    @Test
    fun `oldest reverses that`() {
        val sorted = JobSort.Oldest.sort(listOf(job("new", createdAt = 99), job("old", createdAt = 1)))

        assertEquals("old", sorted.first().id)
    }

    @Test
    fun `promise date orders soonest first and sinks jobs with no promise`() {
        val sorted = JobSort.PromiseDate.sort(
            listOf(
                job("none", createdAt = 5),
                job("later", createdAt = 5, promisedBy = 5_000),
                job("sooner", createdAt = 5, promisedBy = 1_000),
            ),
        )

        assertEquals(listOf("sooner", "later", "none"), sorted.map { it.id })
    }

    @Test
    fun `a waiting walk-in floats above everything regardless of sort`() {
        val sorted = JobSort.Oldest.sort(
            listOf(
                job("oldest", createdAt = 1),
                job("waiting", createdAt = 99, customerWaiting = true),
            ),
        )

        assertEquals("waiting", sorted.first().id)
    }

    @Test
    fun `overdue outranks the chosen ordering but not a waiting walk-in`() {
        val past = System.currentTimeMillis() - HOUR
        val sorted = JobSort.Newest.sort(
            listOf(
                job("newest", createdAt = 99),
                job("overdue", createdAt = 1, promisedBy = past),
                job("waiting", createdAt = 1, customerWaiting = true),
            ),
        )

        assertEquals(listOf("waiting", "overdue", "newest"), sorted.map { it.id })
    }

    @Test
    fun `sorting never drops or duplicates a job`() {
        val jobs = listOf(job("a", createdAt = 3), job("b", createdAt = 1), job("c", createdAt = 2))

        JobSort.entries.forEach { sort ->
            val sorted = sort.sort(jobs)
            assertEquals("$sort changed the job count", jobs.size, sorted.size)
            assertEquals("$sort lost a job", jobs.map { it.id }.toSet(), sorted.map { it.id }.toSet())
        }
    }
}
