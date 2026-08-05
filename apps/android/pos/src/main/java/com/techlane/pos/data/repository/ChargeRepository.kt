package com.techlane.pos.data.repository

import com.techlane.pos.core.util.Msisdn
import com.techlane.pos.data.local.ChargeDao
import com.techlane.pos.data.local.ChargeRecordEntity
import com.techlane.pos.data.remote.ApiException
import com.techlane.pos.data.remote.OfflineException
import com.techlane.pos.data.remote.TechLaneApi
import com.techlane.pos.data.remote.dto.CheckoutRequest
import com.techlane.pos.data.remote.dto.CompleteSaleRequest
import com.techlane.pos.data.remote.dto.CreatePaymentRequest
import com.techlane.pos.data.remote.dto.PaymentDto
import com.techlane.pos.data.remote.dto.SaleItemInputDto
import com.techlane.pos.data.printer.PrinterRepository
import com.techlane.pos.data.remote.parseApiErrorMessage
import com.techlane.pos.data.remote.toAppException
import com.techlane.pos.domain.model.ChargeRequest
import com.techlane.pos.domain.model.ChargeTarget
import com.techlane.pos.domain.model.MpesaResult
import com.techlane.pos.domain.model.PaymentMethod
import com.techlane.pos.domain.model.StkFailure
import com.techlane.pos.domain.model.StkStage
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.FlowCollector
import kotlinx.coroutines.flow.emitAll
import kotlinx.coroutines.flow.flow
import java.util.UUID
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Runs a charge from "send the prompt" to a settled outcome.
 *
 * The contract the UI relies on: [charge] emits progress and always finishes on
 * exactly one of [StkStage.Paid], [StkStage.Failed] or [StkStage.TimedOut]. It
 * never leaves the caller guessing, because the screen that collects it blocks
 * the technician until it lands.
 */
@Singleton
class ChargeRepository @Inject constructor(
    private val api: TechLaneApi,
    private val chargeDao: ChargeDao,
    private val printers: PrinterRepository,
) {

    fun charge(request: ChargeRequest): Flow<StkStage> = flow {
        emit(StkStage.Sending)

        val phone = request.phone?.let(Msisdn::normalise)
        if (request.method == PaymentMethod.MpesaStk && phone == null) {
            emit(failure(StkFailure.InvalidNumber, "Enter a valid M-Pesa number before sending the prompt.", null))
            return@flow
        }

        val recordId = request.idempotencyKey
        writeRecord(request, recordId, status = "sent", paymentId = null, saleId = null)

        val checkout = try {
            api.checkout(
                body = CheckoutRequest(
                    branchId = request.branchId,
                    locationId = request.locationId,
                    items = listOf(request.target.toLineItem(request.amount)),
                    method = request.method.wire,
                    phone = phone,
                    // For STK the server picks the reference: Daraja caps it at
                    // 12 characters and rejects the longer labels staff pick.
                    // For Paybill it is the customer's own transaction code,
                    // which the server requires for mpesa_c2b.
                    accountReference = request.reference,
                ),
                correlationId = request.idempotencyKey,
                idempotencyKey = request.idempotencyKey,
            )
        } catch (e: Exception) {
            val mapped = e.toAppException()
            val (reason, message) = when (mapped) {
                is OfflineException -> StkFailure.NetworkError to mapped.message
                else -> MpesaResult.classify(null, mapped.message)
            }
            markRecord(recordId, "failed", null, message)
            emit(failure(reason, message, null))
            return@flow
        }

        val saleId = checkout.sale?.id
        val payment = checkout.payment
        markRecordIds(recordId, payment?.id, saleId)

        // Cash is settled the moment the server takes it — no prompt to wait on.
        if (request.method == PaymentMethod.Cash) {
            markRecord(recordId, "paid", null, null)
            saleId?.let(printers::autoPrintSaleReceiptIfEnabled)
            emit(StkStage.Paid(request.amount, null, saleId))
            return@flow
        }

        // Paybill is money the customer already sent, so there is nothing to
        // wait for either. The server only auto-completes cash and bank
        // tenders, so the sale is closed here explicitly — otherwise it would
        // sit in draft forever and never deduct stock.
        if (request.method == PaymentMethod.Paybill) {
            val finalSaleId = saleId?.also { id ->
                runCatching {
                    if (api.sale(id).status == "draft") {
                        api.completeSale(id, CompleteSaleRequest(request.locationId))
                    }
                }
            }
            markRecord(recordId, "paid", request.reference, null)
            finalSaleId?.let(printers::autoPrintSaleReceiptIfEnabled)
            emit(StkStage.Paid(request.amount, request.reference, finalSaleId))
            return@flow
        }

        if (payment == null) {
            val message = "The sale was created but M-Pesa did not accept the prompt. Check Payments before charging again."
            markRecord(recordId, "unknown", null, message)
            emit(failure(StkFailure.ProviderError, message, null))
            return@flow
        }

        if (payment.isSettled) {
            emitSettled(recordId, payment, saleId, request.locationId, request.amount)
            return@flow
        }
        if (payment.isFailed) {
            val (reason, message) = MpesaResult.classify(null, payment.status)
            markRecord(recordId, "failed", null, message)
            emit(failure(reason, message, payment.id))
            return@flow
        }

        emitAll(
            pollUntilResolved(
                recordId = recordId,
                paymentId = payment.id,
                saleId = saleId,
                locationId = request.locationId,
                fallbackAmount = request.amount,
            ),
        )
    }

    /**
     * Settles a repair job's balance.
     *
     * Deliberately does *not* go through `/pos/checkout`: the job already exists
     * and already carries its own total, so creating a sale for it would mint a
     * second document for the same money and double-count the day's takings.
     * `POST /payments` hangs the payment off the job instead, which is what the
     * web console does and what the handover check reads to decide whether a
     * device can be released.
     *
     * Everything after the prompt is sent is shared with a till charge — the
     * polling, the Daraja nudge, and the "not confirmed yet" handling are the
     * same problem regardless of what is being paid for.
     */
    fun chargeRepair(
        repairId: String,
        branchId: String,
        amount: Double,
        method: PaymentMethod,
        phone: String?,
        label: String,
        idempotencyKey: String,
        reference: String? = null,
        /**
         * Job code used as the C2B bill reference so Safaricom Paybill
         * confirmations can auto-match this payment. Required for Paybill.
         */
        jobCode: String? = null,
    ): Flow<StkStage> = flow {
        emit(StkStage.Sending)

        val normalised = phone?.let(Msisdn::normalise)
        if (method == PaymentMethod.MpesaStk && normalised == null) {
            emit(failure(StkFailure.InvalidNumber, "Enter a valid M-Pesa number before sending the prompt.", null))
            return@flow
        }
        // C2B needs a bill ref the customer typed (or we told them). The job
        // code is that ref — not an optional TransID paste.
        val billRef = when (method) {
            PaymentMethod.Paybill -> jobCode?.trim()?.takeIf { it.isNotEmpty() }
                ?: reference?.trim()?.takeIf { it.isNotEmpty() }
            else -> reference
        }
        if (method == PaymentMethod.Paybill && billRef.isNullOrBlank()) {
            emit(failure(StkFailure.Unknown, "Missing job code for Paybill account reference.", null))
            return@flow
        }

        val request = ChargeRequest(
            branchId = branchId,
            // A job payment touches no stock, so there is no location to post
            // against; the record keeps the same shape as a till charge purely
            // so Activity can show both in one list.
            locationId = "",
            amount = amount,
            method = method,
            phone = normalised,
            target = ChargeTarget.Service(id = null, name = label, price = amount),
            idempotencyKey = idempotencyKey,
            reference = billRef,
        )
        writeRecord(request, idempotencyKey, status = "sent", paymentId = null, saleId = null, repairId = repairId)

        val payment = try {
            api.createPayment(
                body = CreatePaymentRequest(
                    method = method.wire,
                    amount = amount,
                    payableType = "repair",
                    payableId = repairId,
                    branchId = branchId,
                    phone = normalised,
                    accountReference = billRef,
                ),
                correlationId = idempotencyKey,
                idempotencyKey = idempotencyKey,
            )
        } catch (e: Exception) {
            val mapped = e.toAppException()
            val (reason, message) = when (mapped) {
                is OfflineException -> StkFailure.NetworkError to mapped.message
                else -> MpesaResult.classify(null, mapped.message)
            }
            markRecord(idempotencyKey, "failed", null, message)
            emit(failure(reason, message, null))
            return@flow
        }

        markRecordIds(idempotencyKey, payment.id, null)

        // Cash settles immediately on the server (confirmed). Paybill/C2B stays
        // initiated until Safaricom confirms or staff match an unmatched row —
        // never treat "accepted" as paid.
        if (method == PaymentMethod.Cash) {
            markRecord(idempotencyKey, "paid", billRef, null)
            emit(StkStage.Paid(amount, billRef, null))
            return@flow
        }

        if (payment.isSettled) {
            emitSettled(idempotencyKey, payment, saleId = null, locationId = "", fallbackAmount = amount)
            return@flow
        }
        if (payment.isFailed) {
            val (reason, message) = MpesaResult.classify(null, payment.status)
            markRecord(idempotencyKey, "failed", null, message)
            emit(failure(reason, message, payment.id))
            return@flow
        }

        emitAll(
            pollUntilResolved(
                recordId = idempotencyKey,
                paymentId = payment.id,
                // No sale to complete or auto-print; the job's own receipt is
                // reprinted from Job Details instead.
                saleId = null,
                locationId = "",
                fallbackAmount = amount,
            ),
        )
    }

    /**
     * Creates an initiated C2B payment against a repair so an unmatched Paybill
     * row can be linked via MatchC2B. Does not poll — caller matches next.
     */
    suspend fun createRepairPaymentPending(
        repairId: String,
        branchId: String,
        amount: Double,
        jobCode: String,
        phone: String?,
    ): PaymentDto {
        val key = UUID.randomUUID().toString()
        return api.createPayment(
            body = CreatePaymentRequest(
                method = PaymentMethod.Paybill.wire,
                amount = amount,
                payableType = "repair",
                payableId = repairId,
                branchId = branchId,
                phone = phone?.let(Msisdn::normalise),
                accountReference = jobCode,
            ),
            correlationId = key,
            idempotencyKey = key,
        )
    }

    /**
     * Re-checks a prompt that was left unresolved — the "Check again" action
     * after a timeout.
     *
     * Everything it needs comes from the local record rather than from whatever
     * the screen still happens to hold, so this keeps working after the app is
     * killed and reopened with an unconfirmed charge sitting in Activity.
     */
    fun verify(recordId: String, locationId: String): Flow<StkStage> = flow {
        val record = chargeDao.byId(recordId)
        val paymentId = record?.paymentId
        if (record == null || paymentId == null) {
            emit(
                failure(
                    StkFailure.Unknown,
                    "We have no M-Pesa reference for this charge. Check Payments on the web console before charging again.",
                    null,
                ),
            )
            return@flow
        }
        emit(StkStage.Waiting(0, "Checking with M-Pesa…"))
        emitAll(
            pollUntilResolved(
                recordId = recordId,
                paymentId = paymentId,
                saleId = record.saleId,
                locationId = locationId,
                fallbackAmount = record.amount,
                window = SHORT_WINDOW_SECONDS,
            ),
        )
    }

    // ------------------------------------------------------------------ polling

    /**
     * Polls until the payment settles, fails, or the window closes.
     *
     * Two sources are used deliberately. GET /payments/{id} is cheap and sees
     * the Daraja callback the instant it lands, which is the common case. The
     * reconcile POST forces an STK Query at Safaricom and is what rescues a
     * prompt whose callback never arrives — but it is owner/manager-gated and
     * rate-sensitive, so it only fires every few ticks.
     */
    private fun pollUntilResolved(
        recordId: String,
        paymentId: String,
        saleId: String?,
        locationId: String,
        fallbackAmount: Double,
        window: Int = DEFAULT_WINDOW_SECONDS,
    ): Flow<StkStage> = flow {
        var elapsed = 0
        var tick = 0
        var mayReconcile = true
        var lastDetail: String? = null

        while (elapsed < window) {
            emit(StkStage.Waiting(elapsed, lastDetail ?: waitingDetail(elapsed)))
            delay(POLL_INTERVAL_MS)
            elapsed += (POLL_INTERVAL_MS / 1000).toInt()
            tick += 1

            val direct = runCatching { api.payment(paymentId) }.getOrNull()
            if (direct != null) {
                if (direct.isSettled) {
                    emitSettled(recordId, direct, saleId, locationId, fallbackAmount)
                    return@flow
                }
                if (direct.isFailed) {
                    val (reason, message) = MpesaResult.classify(null, direct.status)
                    markRecord(recordId, "failed", null, message)
                    emit(failure(reason, message, paymentId))
                    return@flow
                }
            }

            // Nudge Daraja periodically in case the callback is lost in transit.
            if (mayReconcile && tick % RECONCILE_EVERY_TICKS == 0) {
                when (val outcome = reconcile(paymentId)) {
                    is ReconcileOutcome.Settled -> {
                        emitSettled(recordId, outcome.payment, saleId, locationId, fallbackAmount)
                        return@flow
                    }
                    is ReconcileOutcome.Terminal -> {
                        markRecord(recordId, "failed", null, outcome.message)
                        emit(failure(outcome.reason, outcome.message, paymentId))
                        return@flow
                    }
                    is ReconcileOutcome.Pending -> lastDetail = outcome.detail ?: waitingDetail(elapsed)
                    // Non-privileged staff simply keep reading the payment record;
                    // the callback still settles it, just without our nudge.
                    ReconcileOutcome.NotPermitted -> mayReconcile = false
                    ReconcileOutcome.Unavailable -> Unit
                }
            } else {
                lastDetail = null
            }
        }

        // One last look before we call it: the callback often lands right at the edge.
        val last = runCatching { api.payment(paymentId) }.getOrNull()
        if (last?.isSettled == true) {
            emitSettled(recordId, last, saleId, locationId, fallbackAmount)
            return@flow
        }
        when (val outcome = reconcile(paymentId)) {
            is ReconcileOutcome.Settled -> {
                emitSettled(recordId, outcome.payment, saleId, locationId, fallbackAmount)
                return@flow
            }
            is ReconcileOutcome.Terminal -> {
                markRecord(recordId, "failed", null, outcome.message)
                emit(failure(outcome.reason, outcome.message, paymentId))
                return@flow
            }
            else -> Unit
        }
        markRecord(recordId, "pending", null, "Prompt not confirmed within ${window}s")
        emit(StkStage.TimedOut(paymentId))
    }

    private suspend fun FlowCollector<StkStage>.emitSettled(
        recordId: String,
        payment: PaymentDto,
        saleId: String?,
        locationId: String,
        fallbackAmount: Double,
    ) {
        emit(StkStage.Finalising)
        // The webhook normally completes the sale; this is the safety net for
        // when it raced us or never fired. Stock must not stay un-deducted.
        val finalSaleId = saleId?.also { id ->
            runCatching {
                val sale = api.sale(id)
                if (sale.status == "draft") {
                    api.completeSale(id, CompleteSaleRequest(locationId))
                }
            }
        }
        val receipt = payment.accountReference.takeIf { it.isNotBlank() }
        markRecord(recordId, "paid", receipt, null)
        finalSaleId?.let(printers::autoPrintSaleReceiptIfEnabled)
        emit(StkStage.Paid(payment.amount.takeIf { it > 0 } ?: fallbackAmount, receipt, finalSaleId))
    }

    private sealed interface ReconcileOutcome {
        data class Settled(val payment: PaymentDto) : ReconcileOutcome
        data class Pending(val detail: String?) : ReconcileOutcome
        data class Terminal(val reason: StkFailure, val message: String) : ReconcileOutcome
        data object NotPermitted : ReconcileOutcome
        data object Unavailable : ReconcileOutcome
    }

    private suspend fun reconcile(paymentId: String): ReconcileOutcome = try {
        val response = api.reconcileStk(paymentId)
        val body = response.body()
        when {
            response.isSuccessful && body != null && body.isSettled -> ReconcileOutcome.Settled(body)
            response.isSuccessful && body != null && body.isFailed -> {
                val (reason, message) = MpesaResult.classify(null, body.status)
                ReconcileOutcome.Terminal(reason, message)
            }
            response.isSuccessful -> ReconcileOutcome.Pending(null)
            // 202 is the backend's "customer still has the prompt open".
            response.code() == 202 -> ReconcileOutcome.Pending(
                parseApiErrorMessage(response.errorBody()?.string(), "Waiting for the customer…")
                    .takeIf { !MpesaResult.isStillProcessing(it) },
            )
            response.code() == 403 -> ReconcileOutcome.NotPermitted
            else -> {
                val raw = parseApiErrorMessage(response.errorBody()?.string(), "M-Pesa did not confirm the payment")
                if (MpesaResult.isStillProcessing(raw)) {
                    ReconcileOutcome.Pending(null)
                } else {
                    val (reason, message) = MpesaResult.classify(null, raw)
                    ReconcileOutcome.Terminal(reason, message)
                }
            }
        }
    } catch (e: Exception) {
        val mapped = e.toAppException()
        when {
            mapped is OfflineException -> ReconcileOutcome.Unavailable
            mapped is ApiException && MpesaResult.isStillProcessing(mapped.message) -> ReconcileOutcome.Pending(null)
            mapped is ApiException && MpesaResult.isTerminal(mapped.message) -> {
                val (reason, message) = MpesaResult.classify(null, mapped.message)
                ReconcileOutcome.Terminal(reason, message)
            }
            else -> ReconcileOutcome.Unavailable
        }
    }

    // ------------------------------------------------------------------ helpers

    private fun failure(reason: StkFailure, message: String, paymentId: String?) =
        StkStage.Failed(reason, message, paymentId)

    /** Keeps the wait screen honest about what is happening, second by second. */
    private fun waitingDetail(elapsed: Int): String = when {
        elapsed < 8 -> "Prompt sent. The customer's phone should be buzzing now."
        elapsed < 40 -> "Waiting for the customer to enter their M-Pesa PIN."
        elapsed < 80 -> "Still waiting. Ask them to check for the M-Pesa pop-up on their screen."
        else -> "Taking longer than usual — we're double-checking with M-Pesa."
    }

    private fun ChargeTarget.toLineItem(amount: Double): SaleItemInputDto = when (this) {
        is ChargeTarget.Product -> SaleItemInputDto(variantId = variantId, quantity = quantity)
        // Services and bare amounts are quick-sale lines: the server prices them
        // from what we send rather than looking up a variant.
        else -> SaleItemInputDto(description = label, quantity = 1, unitPrice = amount)
    }

    private suspend fun writeRecord(
        request: ChargeRequest,
        id: String,
        status: String,
        paymentId: String?,
        saleId: String?,
        repairId: String? = null,
    ) {
        val now = System.currentTimeMillis()
        chargeDao.upsert(
            ChargeRecordEntity(
                id = id,
                amount = request.amount,
                method = request.method.wire,
                phone = request.phone,
                label = request.target.label,
                status = status,
                saleId = saleId,
                paymentId = paymentId,
                receipt = null,
                failureReason = null,
                createdAt = now,
                updatedAt = now,
                repairId = repairId,
            ),
        )
    }

    private suspend fun markRecord(id: String, status: String, receipt: String?, reason: String?) {
        chargeDao.updateStatus(id, status, receipt, reason, System.currentTimeMillis())
    }

    private suspend fun markRecordIds(id: String, paymentId: String?, saleId: String?) {
        val existing = chargeDao.unresolved().firstOrNull { it.id == id } ?: return
        chargeDao.upsert(existing.copy(paymentId = paymentId, saleId = saleId, updatedAt = System.currentTimeMillis()))
    }

    fun observeRecent() = chargeDao.observeRecent()

    /** Rendered receipt HTML for a completed sale, ready for the print spooler. */
    suspend fun receiptHtml(saleId: String): Result<String> = runCatching {
        api.saleReceiptHtml(saleId).use { it.string() }
    }.recoverCatching { throw it.toAppException() }

    /** Sale id recorded against a local charge, for reprinting from Activity. */
    suspend fun saleIdFor(recordId: String): String? = chargeDao.byId(recordId)?.saleId

    /**
     * Sweeps every prompt this phone left hanging and writes back whatever the
     * server now says. Cheap (one GET per record) and read-only, so it is safe
     * to hang off a pull-to-refresh; the settle path stays with [charge].
     *
     * Returns how many changed, so the UI can say something useful.
     */
    suspend fun refreshUnresolved(): Int {
        var changed = 0
        chargeDao.unresolved().forEach { record ->
            val paymentId = record.paymentId ?: return@forEach
            val payment = runCatching { api.payment(paymentId) }.getOrNull() ?: return@forEach
            when {
                payment.isSettled -> {
                    markRecord(record.id, "paid", payment.accountReference.takeIf { it.isNotBlank() }, null)
                    changed++
                }
                payment.isFailed -> {
                    val (_, message) = MpesaResult.classify(null, payment.status)
                    markRecord(record.id, "failed", null, message)
                    changed++
                }
            }
        }
        return changed
    }

    private companion object {
        const val POLL_INTERVAL_MS = 2_000L

        /**
         * Safaricom expires an unanswered prompt at about 60s, but the callback
         * and our own confirmation can trail it. 120s covers a customer who has
         * to find their phone, without stranding the counter indefinitely.
         */
        const val DEFAULT_WINDOW_SECONDS = 120
        const val SHORT_WINDOW_SECONDS = 30
        const val RECONCILE_EVERY_TICKS = 4
    }
}
