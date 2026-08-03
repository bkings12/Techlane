package com.techlane.pos.data.remote

import com.techlane.pos.data.remote.dto.AddAttachmentRequest
import com.techlane.pos.data.remote.dto.AddNoteRequest
import com.techlane.pos.data.remote.dto.AddSaleLineRequest
import com.techlane.pos.data.remote.dto.AssignRequest
import com.techlane.pos.data.remote.dto.AuthorizeWorkRequest
import com.techlane.pos.data.remote.dto.BranchDto
import com.techlane.pos.data.remote.dto.ChangeStatusRequest
import com.techlane.pos.data.remote.dto.CreateEstimateRequest
import com.techlane.pos.data.remote.dto.JobSaleLineDto
import com.techlane.pos.data.remote.dto.RepairAttachmentDto
import com.techlane.pos.data.remote.dto.RepairEstimateDto
import com.techlane.pos.data.remote.dto.RepairJobDto
import com.techlane.pos.data.remote.dto.RepairNoteDto
import com.techlane.pos.data.remote.dto.SendSmsRequest
import com.techlane.pos.data.remote.dto.TechnicianDto
import com.techlane.pos.data.remote.dto.CatalogItemDto
import com.techlane.pos.data.remote.dto.CheckoutRequest
import com.techlane.pos.data.remote.dto.CheckoutResponse
import com.techlane.pos.data.remote.dto.CompleteSaleRequest
import com.techlane.pos.data.remote.dto.IntakePresetDto
import com.techlane.pos.data.remote.dto.ItemsEnvelope
import com.techlane.pos.data.remote.dto.LoginRequest
import com.techlane.pos.data.remote.dto.LoginResponse
import com.techlane.pos.data.remote.dto.MeDto
import com.techlane.pos.data.remote.dto.MfaVerifyRequest
import com.techlane.pos.data.remote.dto.PaymentDto
import com.techlane.pos.data.remote.dto.PaymentSettingsDto
import com.techlane.pos.data.remote.dto.RefreshRequest
import com.techlane.pos.data.remote.dto.SaleDto
import com.techlane.pos.data.remote.dto.StockLocationDto
import com.techlane.pos.data.remote.dto.TokenPairDto
import okhttp3.ResponseBody
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.Header
import retrofit2.http.POST
import retrofit2.http.Path
import retrofit2.http.Query

interface TechLaneApi {

    @POST("auth/login")
    suspend fun login(@Body body: LoginRequest): LoginResponse

    @POST("auth/mfa/verify")
    suspend fun verifyMfa(@Body body: MfaVerifyRequest): TokenPairDto

    @POST("auth/refresh")
    suspend fun refresh(@Body body: RefreshRequest): TokenPairDto

    @GET("me")
    suspend fun me(): MeDto

    @GET("branches")
    suspend fun branches(): ItemsEnvelope<BranchDto>

    @GET("stock-locations")
    suspend fun stockLocations(@Query("branch_id") branchId: String): ItemsEnvelope<StockLocationDto>

    @GET("catalog")
    suspend fun catalog(@Query("location_id") locationId: String?): ItemsEnvelope<CatalogItemDto>

    @GET("intake-presets")
    suspend fun intakePresets(@Query("kind") kind: String): ItemsEnvelope<IntakePresetDto>

    @GET("payments/settings")
    suspend fun paymentSettings(): PaymentSettingsDto

    /**
     * Creates the sale and asks for payment in one call. For mpesa_stk the sale
     * stays draft until the prompt is confirmed — the server completes it from
     * the Daraja callback, and [completeSale] is only our safety net.
     */
    @POST("pos/checkout")
    suspend fun checkout(
        @Body body: CheckoutRequest,
        @Header("Idempotency-Key") idempotencyKey: String,
    ): CheckoutResponse

    @GET("payments/{id}")
    suspend fun payment(@Path("id") id: String): PaymentDto

    /**
     * Nudges Daraja's STK Query when the callback is late. Returns 202 while the
     * customer still has the prompt open, which is why this is a raw [Response]:
     * "not finished yet" is not an error worth throwing over.
     */
    @POST("payments/{id}/mpesa/reconcile")
    suspend fun reconcileStk(@Path("id") id: String): Response<PaymentDto>

    @GET("sales/{id}")
    suspend fun sale(@Path("id") id: String): SaleDto

    @POST("sales/{id}/complete")
    suspend fun completeSale(@Path("id") id: String, @Body body: CompleteSaleRequest): SaleDto

    // ------------------------------------------------------------- repair jobs

    @GET("repairs")
    suspend fun repairs(
        @Query("status") status: String? = null,
        @Query("technician_id") technicianId: String? = null,
        @Query("branch_id") branchId: String? = null,
        @Query("q") query: String? = null,
    ): ItemsEnvelope<RepairJobDto>

    @GET("repairs/{id}")
    suspend fun repair(@Path("id") id: String): RepairJobDto

    /** Resolves the QR printed on an intake slip (techlane://repair-pickup/CODE). */
    @GET("repairs/by-pickup-code")
    suspend fun repairByPickupCode(@Query("code") code: String): RepairJobDto

    @POST("repairs/{id}/status")
    suspend fun changeRepairStatus(
        @Path("id") id: String,
        @Body body: ChangeStatusRequest,
    ): RepairJobDto

    @POST("repairs/{id}/assign")
    suspend fun assignRepair(@Path("id") id: String, @Body body: AssignRequest): RepairJobDto

    @GET("repairs/{id}/notes")
    suspend fun repairNotes(@Path("id") id: String): ItemsEnvelope<RepairNoteDto>

    @POST("repairs/{id}/notes")
    suspend fun addRepairNote(@Path("id") id: String, @Body body: AddNoteRequest): RepairNoteDto

    @GET("repairs/{id}/estimates")
    suspend fun repairEstimates(@Path("id") id: String): ItemsEnvelope<RepairEstimateDto>

    @POST("repairs/{id}/estimates")
    suspend fun createRepairEstimate(
        @Path("id") id: String,
        @Body body: CreateEstimateRequest,
    ): RepairEstimateDto

    /** Manager go-ahead when there is no customer-approved estimate. */
    @POST("repairs/{id}/authorize-work")
    suspend fun authorizeRepairWork(
        @Path("id") id: String,
        @Body body: AuthorizeWorkRequest,
    ): RepairJobDto

    @GET("repairs/{id}/attachments")
    suspend fun repairAttachments(@Path("id") id: String): ItemsEnvelope<RepairAttachmentDto>

    @POST("repairs/{id}/attachments")
    suspend fun addRepairAttachment(
        @Path("id") id: String,
        @Body body: AddAttachmentRequest,
    ): RepairAttachmentDto

    @GET("repairs/{id}/attachments/{attachmentId}/content")
    suspend fun repairAttachmentContent(
        @Path("id") id: String,
        @Path("attachmentId") attachmentId: String,
    ): ResponseBody

    /**
     * Pre-rendered ESC/POS bytes for the job's final receipt / intake slip —
     * the same content and wording the web console prints from, so a reprint
     * from the phone never drifts from what the shop's other till would
     * produce. Built for an 80mm cutter-equipped printer, so
     * [com.techlane.pos.data.printer.EscPosSanitizer] adapts it before it
     * reaches a 58mm printer with no cutter.
     */
    @GET("repairs/{id}/receipt.escpos")
    suspend fun repairReceiptEscPos(@Path("id") id: String): ResponseBody

    @GET("repairs/{id}/intake-slip.escpos")
    suspend fun repairIntakeSlipEscPos(@Path("id") id: String): ResponseBody

    @GET("repairs/{id}/sale-lines")
    suspend fun repairSaleLines(@Path("id") id: String): ItemsEnvelope<JobSaleLineDto>

    @POST("repairs/{id}/sale-lines")
    suspend fun addRepairSaleLine(
        @Path("id") id: String,
        @Body body: AddSaleLineRequest,
    ): JobSaleLineDto

    @DELETE("repairs/{id}/sale-lines/{lineId}")
    suspend fun removeRepairSaleLine(
        @Path("id") id: String,
        @Path("lineId") lineId: String,
    ): Response<Unit>

    @GET("technicians")
    suspend fun technicians(): ItemsEnvelope<TechnicianDto>

    /** Customer updates go through the shop's notification log, not the handset. */
    @POST("sms/send")
    suspend fun sendSms(@Body body: SendSmsRequest): Response<Unit>

    /**
     * Rendered receipt for a completed sale. Returned as a raw body rather than
     * a typed String: the JSON converter would otherwise try to parse the HTML.
     */
    @GET("sales/{id}/receipt.html")
    suspend fun saleReceiptHtml(@Path("id") id: String): ResponseBody

    /** See [repairReceiptEscPos] — same rationale, for a POS/Quick Charge sale. */
    @GET("sales/{id}/receipt.escpos")
    suspend fun saleReceiptEscPos(@Path("id") id: String): ResponseBody

    @GET("sales")
    suspend fun sales(
        @Query("branch_id") branchId: String?,
        @Query("limit") limit: Int = 25,
    ): ItemsEnvelope<SaleDto>
}
