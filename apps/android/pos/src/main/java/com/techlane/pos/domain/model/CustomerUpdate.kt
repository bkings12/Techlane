package com.techlane.pos.domain.model

/**
 * Suggested wording for a customer update, chosen by job status.
 *
 * These are starting points a technician edits, never fixed strings: the shop's
 * voice is its own, and a message that reads as a robot template costs trust.
 * Sending goes through the backend (`POST /sms/send`) so it lands in the same
 * notification log as everything else — the handset never sends an SMS itself.
 */
data class CustomerUpdateTemplate(
    val id: String,
    val label: String,
    val build: (CustomerUpdateContext) -> String,
)

data class CustomerUpdateContext(
    val customerFirstName: String,
    val deviceLabel: String,
    val jobCode: String,
    val shopName: String = "TechLane",
)

object CustomerUpdateTemplates {

    fun forStatus(status: JobStatus): List<CustomerUpdateTemplate> = when (status) {
        JobStatus.Intake -> listOf(received, diagnosisStarted)
        JobStatus.Diagnosed -> listOf(diagnosisInconclusive, estimateReady)
        JobStatus.WaitingParts -> listOf(waitingParts)
        JobStatus.InProgress -> listOf(repairUnderway)
        JobStatus.ReadyForPickup, JobStatus.Completed -> listOf(readyForCollection)
        JobStatus.Cancelled, JobStatus.Unrepairable -> listOf(closedNoRepair)
        JobStatus.Collected -> listOf(thanks)
    }

    private val received = CustomerUpdateTemplate("received", "Device received") { c ->
        "Hello ${c.customerFirstName}, we have received your ${c.deviceLabel} at ${c.shopName} " +
            "under job ${c.jobCode}. We will update you once the assessment begins."
    }

    private val diagnosisStarted = CustomerUpdateTemplate("diagnosis_started", "Assessment started") { c ->
        "Hello ${c.customerFirstName}, our technician has started assessing your ${c.deviceLabel}. " +
            "We will share the findings and the cost as soon as the check is complete."
    }

    private val diagnosisInconclusive =
        CustomerUpdateTemplate("diagnosis_inconclusive", "Diagnosis not conclusive") { c ->
            "Hello ${c.customerFirstName}, we have completed the initial diagnosis of your ${c.deviceLabel}. " +
                "Further testing is required, so the diagnosis is not yet conclusive. We will continue the " +
                "assessment and update you once we have more information."
        }

    private val estimateReady = CustomerUpdateTemplate("estimate_ready", "Estimate ready") { c ->
        "Hello ${c.customerFirstName}, the assessment of your ${c.deviceLabel} is complete and we have " +
            "prepared a repair estimate. Please confirm whether we should proceed."
    }

    private val waitingParts = CustomerUpdateTemplate("waiting_parts", "Waiting for parts") { c ->
        "Hello ${c.customerFirstName}, the repair of your ${c.deviceLabel} is on hold while we wait for a " +
            "part to arrive. We will resume as soon as it is in and keep you posted."
    }

    private val repairUnderway = CustomerUpdateTemplate("repair_underway", "Repair underway") { c ->
        "Hello ${c.customerFirstName}, work on your ${c.deviceLabel} is underway. We will let you know " +
            "the moment it is ready for collection."
    }

    private val readyForCollection = CustomerUpdateTemplate("ready", "Ready for collection") { c ->
        "Hello ${c.customerFirstName}, your ${c.deviceLabel} is ready for collection at ${c.shopName}. " +
            "Please carry your job number ${c.jobCode}."
    }

    private val closedNoRepair = CustomerUpdateTemplate("closed", "Closed without repair") { c ->
        "Hello ${c.customerFirstName}, we were unable to proceed with the repair of your ${c.deviceLabel}. " +
            "Please collect the device at your convenience and speak to us about the options."
    }

    private val thanks = CustomerUpdateTemplate("thanks", "Thank you") { c ->
        "Hello ${c.customerFirstName}, thank you for choosing ${c.shopName}. Please get in touch if " +
            "anything at all is not right with your ${c.deviceLabel}."
    }
}
