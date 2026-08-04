package com.techlane.pos.data.remote.dto

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

// Mirrors internal/repair models. Every field is optional-with-default so a
// backend that grows a column never crashes an installed handset.

@Serializable
data class RepairJobDto(
    val id: String,
    @SerialName("job_code") val jobCode: String = "",
    @SerialName("job_number") val jobNumber: Int = 0,
    @SerialName("pickup_code") val pickupCode: String = "",
    @SerialName("branch_id") val branchId: String = "",
    @SerialName("customer_id") val customerId: String? = null,
    @SerialName("customer_name") val customerName: String? = null,
    @SerialName("device_id") val deviceId: String = "",
    @SerialName("technician_id") val technicianId: String? = null,
    val status: String = "intake",
    @SerialName("problem_summary") val problemSummary: String = "",
    @SerialName("service_type") val serviceType: String = "",
    @SerialName("labor_amount") val laborAmount: Double = 0.0,
    @SerialName("sale_lines_total") val saleLinesTotal: Double = 0.0,
    @SerialName("sale_lines") val saleLines: List<JobSaleLineDto> = emptyList(),
    @SerialName("amount_due") val amountDue: Double = 0.0,
    @SerialName("balance_due") val balanceDue: Double = 0.0,
    @SerialName("paid_total") val paidTotal: Double = 0.0,
    @SerialName("created_at") val createdAt: String? = null,
    @SerialName("promised_by") val promisedBy: String? = null,
    @SerialName("customer_waiting") val customerWaiting: Boolean = false,
    @SerialName("pending_estimate_total") val pendingEstimateTotal: Double? = null,
    @SerialName("approved_estimate_total") val approvedEstimateTotal: Double? = null,
    @SerialName("authorized_amount") val authorizedAmount: Double? = null,
    val authorization: WorkAuthorizationDto? = null,
    val customer: RepairCustomerDto? = null,
    val device: RepairDeviceDto? = null,
    val timeline: List<StatusEventDto> = emptyList(),
)

@Serializable
data class WorkAuthorizationDto(
    @SerialName("authorized_at") val authorizedAt: String? = null,
    @SerialName("authorized_by") val authorizedBy: String? = null,
    val source: String = "",
    @SerialName("authorized_amount") val authorizedAmount: Double? = null,
)

@Serializable
data class RepairCustomerDto(
    val id: String? = null,
    @SerialName("full_name") val fullName: String? = null,
    val phone: String? = null,
    val email: String? = null,
)

@Serializable
data class RepairDeviceDto(
    val id: String? = null,
    val kind: String = "",
    val brand: String? = null,
    val model: String? = null,
    val imei: String? = null,
    @SerialName("serial_number") val serialNumber: String? = null,
)

@Serializable
data class StatusEventDto(
    val status: String = "",
    val note: String? = null,
    val at: String? = null,
    val by: String? = null,
)

@Serializable
data class JobSaleLineDto(
    val id: String? = null,
    @SerialName("variant_id") val variantId: String? = null,
    val description: String = "",
    val quantity: Int = 1,
    @SerialName("unit_price") val unitPrice: Double = 0.0,
    @SerialName("line_total") val lineTotal: Double = 0.0,
)

@Serializable
data class RepairNoteDto(
    val id: String,
    val note: String = "",
    @SerialName("created_at") val createdAt: String? = null,
    @SerialName("created_by") val createdBy: String? = null,
    @SerialName("author_name") val authorName: String? = null,
)

@Serializable
data class RepairAttachmentDto(
    val id: String,
    @SerialName("repair_job_id") val repairJobId: String = "",
    @SerialName("file_name") val fileName: String = "",
    @SerialName("content_type") val contentType: String = "",
    @SerialName("size_bytes") val sizeBytes: Int = 0,
    @SerialName("created_at") val createdAt: String? = null,
)

@Serializable
data class RepairEstimateDto(
    val id: String,
    @SerialName("total_amount") val totalAmount: Double = 0.0,
    val status: String = "",
    val notes: String? = null,
    @SerialName("created_at") val createdAt: String? = null,
)

@Serializable
data class TechnicianDto(
    val id: String,
    @SerialName("display_name") val displayName: String = "",
    val email: String = "",
    val status: String = "",
)

// ------------------------------------------------------------------ requests

@Serializable
data class ChangeStatusRequest(
    val status: String,
    val note: String? = null,
    @SerialName("closure_reason") val closureReason: String? = null,
    @SerialName("variance_reason") val varianceReason: String? = null,
    @SerialName("labor_amount") val laborAmount: Double? = null,
)

@Serializable
data class AssignRequest(@SerialName("technician_id") val technicianId: String)

@Serializable
data class AddNoteRequest(val note: String)

@Serializable
data class CreateEstimateRequest(
    @SerialName("total_amount") val totalAmount: Double,
    val notes: String? = null,
)

@Serializable
data class AuthorizeWorkRequest(
    val note: String,
    val amount: Double? = null,
)

@Serializable
data class AddAttachmentRequest(
    @SerialName("file_name") val fileName: String,
    @SerialName("content_type") val contentType: String,
    @SerialName("data_base64") val dataBase64: String,
)

@Serializable
data class AddSaleLineRequest(
    @SerialName("variant_id") val variantId: String,
    @SerialName("location_id") val locationId: String,
    val quantity: Int,
)

@Serializable
data class SendSmsRequest(
    val phone: String,
    val body: String,
    @SerialName("repair_job_id") val repairJobId: String? = null,
    @SerialName("customer_id") val customerId: String? = null,
)

// ---------------------------------------------------------------- intake

/**
 * Mirrors the anonymous request struct on `POST /repairs/intake`. Nulls are
 * omitted by the serializer's explicitNulls=false, so an absent optional stays
 * absent on the wire rather than becoming an explicit null the backend has to
 * distinguish from "not supplied".
 */
@Serializable
data class IntakeRequest(
    @SerialName("branch_id") val branchId: String,
    @SerialName("problem_summary") val problemSummary: String,
    @SerialName("device_kind") val deviceKind: String,
    val anonymous: Boolean = false,
    @SerialName("customer_id") val customerId: String? = null,
    @SerialName("customer_name") val customerName: String? = null,
    @SerialName("customer_phone") val customerPhone: String? = null,
    val brand: String? = null,
    val model: String? = null,
    val imei: String? = null,
    @SerialName("serial_number") val serialNumber: String? = null,
    @SerialName("condition_tags") val conditionTags: List<String> = emptyList(),
    @SerialName("estimate_labor_amount") val estimateLaborAmount: Double? = null,
    @SerialName("estimate_parts_amount") val estimatePartsAmount: Double? = null,
    @SerialName("technician_id") val technicianId: String? = null,
    /** ISO-8601 UTC. Optional — a job with no promise is never overdue. */
    @SerialName("promised_by") val promisedBy: String? = null,
)

/** `POST /repairs/intake` creates customer, device, job and estimate in one go. */
@Serializable
data class IntakeResultDto(
    val repair: RepairJobDto? = null,
)

/** A customer as the intake lookup needs them — phone is the search key. */
@Serializable
data class CustomerDto(
    val id: String,
    @SerialName("full_name") val fullName: String = "",
    val phone: String? = null,
    val email: String? = null,
)
