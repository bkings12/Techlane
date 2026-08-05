package com.techlane.pos

import com.techlane.pos.domain.model.ApprovalMethod
import com.techlane.pos.domain.model.JobAction
import com.techlane.pos.domain.model.JobCustomer
import com.techlane.pos.domain.model.JobDetail
import com.techlane.pos.domain.model.JobDevice
import com.techlane.pos.domain.model.JobNote
import com.techlane.pos.domain.model.JobPhoto
import com.techlane.pos.domain.model.JobStatus
import com.techlane.pos.domain.model.PhotoKind
import com.techlane.pos.domain.model.PhotoMetadata
import com.techlane.pos.domain.model.TimelineKind
import com.techlane.pos.domain.model.WorkApproval
import com.techlane.pos.feature.scan.ScanPayloads
import com.techlane.pos.feature.scan.ScanResult
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class JobWorkflowTest {

    private fun detail(
        status: JobStatus,
        approvedAt: Long? = null,
        notes: List<JobNote> = emptyList(),
        photos: List<JobPhoto> = emptyList(),
    ) = JobDetail(
        id = "job-1",
        jobCode = "TL-1042",
        status = status,
        customer = JobCustomer("c1", "Ayub Otieno", "254712345678"),
        device = JobDevice(null, "laptop", "Apple", "MacBook A2337", null, "C02XYZ"),
        problemSummary = "Will not power on",
        technicianId = null,
        technicianName = null,
        createdAt = 1_000L,
        promisedBy = null,
        customerWaiting = false,
        approval = WorkApproval(approvedAt, null, ApprovalMethod.ManagerOverride, 8000.0),
        amountDue = 8000.0,
        balanceDue = 8000.0,
        laborAmount = 8000.0,
        notes = notes,
        estimates = emptyList(),
        parts = emptyList(),
        photos = photos,
        statusEvents = emptyList(),
    )

    // ------------------------------------------------------- authorization gate

    @Test
    fun `unapproved intake and diagnosed jobs are blocked from the bench`() {
        assertTrue(detail(JobStatus.Intake).needsApprovalBeforeBench)
        assertTrue(detail(JobStatus.Diagnosed).needsApprovalBeforeBench)
    }

    @Test
    fun `an approved job is not blocked`() {
        assertFalse(detail(JobStatus.Diagnosed, approvedAt = 5_000L).needsApprovalBeforeBench)
    }

    @Test
    fun `a job already past the bench is not re-gated`() {
        // waiting_parts means work was authorised before the parts detour; the
        // server does not re-check, and neither should the UI.
        assertFalse(detail(JobStatus.WaitingParts).needsApprovalBeforeBench)
        assertFalse(detail(JobStatus.InProgress).needsApprovalBeforeBench)
    }

    // -------------------------------------------------------- contextual actions

    @Test
    fun `actions change with status and never show everything at once`() {
        val diagnosing = JobAction.forStatus(JobStatus.Intake, needsApproval = true)
        assertTrue(JobAction.AddDiagnosis in diagnosing)
        assertTrue(diagnosing.size <= 3)

        val diagnosed = JobAction.forStatus(JobStatus.Diagnosed, needsApproval = true)
        assertTrue(JobAction.SendEstimate in diagnosed)
        assertTrue(JobAction.RecordApproval in diagnosed)

        val waitingParts = JobAction.forStatus(JobStatus.WaitingParts, needsApproval = false)
        assertTrue(JobAction.AddPart in waitingParts)
        assertTrue(JobAction.ResumeRepair in waitingParts)

        val bench = JobAction.forStatus(JobStatus.InProgress, needsApproval = false)
        assertTrue(JobAction.MarkReady in bench)

        val ready = JobAction.forStatus(JobStatus.ReadyForPickup, needsApproval = false, balanceDue = 8000.0)
        assertTrue(JobAction.TakePayment in ready)
        assertFalse(JobAction.MarkCollected in ready)

        val paidReady = JobAction.forStatus(JobStatus.ReadyForPickup, needsApproval = false, balanceDue = 0.0)
        assertTrue(JobAction.MarkCollected in paidReady)
        assertFalse(JobAction.TakePayment in paidReady)
    }

    @Test
    fun `zero balance ready job can be collected`() {
        val job = detail(JobStatus.ReadyForPickup).copy(balanceDue = 0.0)
        assertTrue(job.canCollect)
        assertEquals(8000.0, job.paidTotal, 0.001)
    }

    @Test
    fun `outstanding balance blocks collection`() {
        val job = detail(JobStatus.ReadyForPickup)
        assertFalse(job.canCollect)
        assertEquals(0.0, job.paidTotal, 0.001)
    }

    @Test
    fun `an approved diagnosed job is offered work, not more approval chasing`() {
        val actions = JobAction.forStatus(JobStatus.Diagnosed, needsApproval = false)
        assertFalse(JobAction.SendEstimate in actions)
        assertTrue(JobAction.AddPart in actions)
    }

    // ------------------------------------------------------------------ timeline

    @Test
    fun `timeline merges notes and photos newest first`() {
        val job = detail(
            JobStatus.InProgress,
            notes = listOf(JobNote("n1", "Board-level testing required", "James", 3_000L)),
            photos = listOf(
                JobPhoto("p1", "p1", PhotoKind.Diagnosis, null, null, 2_000L, uploaded = true),
                JobPhoto("p2", "p2", PhotoKind.Diagnosis, null, null, 2_100L, uploaded = true),
            ),
        )
        val timeline = job.timeline()

        assertEquals(TimelineKind.Diagnosis, timeline.first().kind)
        assertTrue(timeline.map { it.at }.zipWithNext().all { (a, b) -> a >= b })
        // Photos taken in the same hour collapse into one line rather than spamming.
        assertEquals(1, timeline.count { it.kind == TimelineKind.Photo })
        assertTrue(timeline.first { it.kind == TimelineKind.Photo }.title.contains("2"))
    }

    @Test
    fun `latest diagnosis is the newest note`() {
        val job = detail(
            JobStatus.Diagnosed,
            notes = listOf(
                JobNote("n1", "First look", null, 1_000L),
                JobNote("n2", "Confirmed board fault", null, 9_000L),
            ),
        )
        assertEquals("Confirmed board fault", job.latestDiagnosis?.text)
    }

    // ------------------------------------------------------------ photo metadata

    @Test
    fun `photo kind and caption survive the filename round trip`() {
        val encoded = PhotoMetadata.encode(PhotoKind.Intake, "Cracked top-right corner", 1234L)
        val decoded = PhotoMetadata.decode(encoded)
        assertEquals(PhotoKind.Intake, decoded.kind)
        assertEquals("Cracked top-right corner", decoded.caption)
    }

    @Test
    fun `a foreign filename decodes to a sane default instead of failing`() {
        val decoded = PhotoMetadata.decode("IMG_20260101_120000.jpg")
        assertEquals(PhotoKind.Progress, decoded.kind)
        assertNull(decoded.caption)
    }

    @Test
    fun `an empty caption does not become an empty string`() {
        val decoded = PhotoMetadata.decode(PhotoMetadata.encode(PhotoKind.Completion, null))
        assertEquals(PhotoKind.Completion, decoded.kind)
        assertNull(decoded.caption)
    }

    // -------------------------------------------------------------------- scan

    @Test
    fun `the intake slip QR resolves to a pickup code`() {
        val result = ScanPayloads.parse("techlane://repair-pickup/PKAB12CD")
        assertTrue(result is ScanResult.RepairPickup)
        assertEquals("PKAB12CD", (result as ScanResult.RepairPickup).code)
    }

    @Test
    fun `a hand-typed pickup code is accepted`() {
        val result = ScanPayloads.parse("pk-ab12cd")
        assertTrue(result is ScanResult.RepairPickup)
    }

    @Test
    fun `a 15-digit IMEI is treated as a device identifier`() {
        val result = ScanPayloads.parse("356938035643809")
        assertTrue(result is ScanResult.DeviceIdentifier)
    }

    @Test
    fun `anything else is passed through for search rather than guessed at`() {
        val result = ScanPayloads.parse("SKU-SCREEN-A2337")
        assertTrue(result is ScanResult.Unknown)
        assertEquals("SKU-SCREEN-A2337", (result as ScanResult.Unknown).raw)
    }
}
