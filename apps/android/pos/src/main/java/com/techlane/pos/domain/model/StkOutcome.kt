package com.techlane.pos.domain.model

/**
 * How far along an STK prompt is. The UI blocks on this until it reaches a
 * terminal case, so every branch here must eventually resolve to Paid or Failed
 * (or TimedOut, which is Failed the technician can retry).
 */
sealed interface StkStage {
    /** Creating the sale and asking Daraja to push the prompt. */
    data object Sending : StkStage

    /** Prompt is on the customer's handset; we are polling for the result. */
    data class Waiting(
        val secondsElapsed: Int,
        val detail: String? = null,
    ) : StkStage

    /** Payment confirmed; finishing the sale (stock, receipt) before we let go. */
    data object Finalising : StkStage

    data class Paid(
        val amount: Double,
        val receipt: String?,
        val saleId: String?,
    ) : StkStage

    data class Failed(
        val reason: StkFailure,
        val message: String,
        val paymentId: String?,
    ) : StkStage

    /** Prompt neither confirmed nor rejected inside the window — verify manually. */
    data class TimedOut(val paymentId: String?) : StkStage
}

enum class StkFailure {
    CancelledByCustomer,
    InsufficientFunds,
    WrongPin,
    NoResponse,
    SubscriberLocked,
    InvalidNumber,
    ProviderError,
    NetworkError,
    NotConfigured,
    Unknown,
}

/**
 * Daraja result codes, mapped to something a technician can act on. Safaricom's
 * own ResultDesc is often either blank or a sentence of internal jargon, so we
 * lead with our wording and keep theirs as the sub-line.
 */
object MpesaResult {

    fun classify(resultCode: String?, resultDesc: String?): Pair<StkFailure, String> {
        val code = resultCode?.trim().orEmpty()
        val desc = resultDesc?.trim().orEmpty()
        val lower = desc.lowercase()

        val byCode = when (code) {
            "1032" -> StkFailure.CancelledByCustomer to "Customer cancelled the prompt"
            "1037" -> StkFailure.NoResponse to "No response from the phone — it may be off, out of range, or the prompt was ignored"
            "1", "1001" -> if (code == "1") {
                StkFailure.InsufficientFunds to "Not enough money in the customer's M-Pesa"
            } else {
                StkFailure.SubscriberLocked to "Another M-Pesa transaction is already running on that number — wait a moment and retry"
            }
            "2001" -> StkFailure.WrongPin to "Wrong M-Pesa PIN entered"
            "1019" -> StkFailure.NoResponse to "The prompt expired before the customer responded"
            "1025", "9999" -> StkFailure.ProviderError to "M-Pesa had a system error — retry the prompt"
            "1036", "1050" -> StkFailure.ProviderError to "M-Pesa is temporarily unavailable — retry in a moment"
            "SFC_IC0003", "17" -> StkFailure.ProviderError to "M-Pesa rejected the request — check the shop's paybill settings"
            else -> null
        }
        if (byCode != null) return byCode

        return when {
            lower.contains("cancel") -> StkFailure.CancelledByCustomer to "Customer cancelled the prompt"
            lower.contains("insufficient") -> StkFailure.InsufficientFunds to "Not enough money in the customer's M-Pesa"
            lower.contains("wrong pin") || lower.contains("invalid pin") ->
                StkFailure.WrongPin to "Wrong M-Pesa PIN entered"
            lower.contains("ds timeout") || lower.contains("timeout") || lower.contains("timed out") ->
                StkFailure.NoResponse to "No response from the phone before the prompt expired"
            lower.contains("unable to lock subscriber") ->
                StkFailure.SubscriberLocked to "Another M-Pesa transaction is already running on that number — wait a moment and retry"
            lower.contains("invalid msisdn") || lower.contains("invalid phone") ->
                StkFailure.InvalidNumber to "That phone number is not a valid M-Pesa number"
            lower.contains("credentials not configured") || lower.contains("not configured") ->
                StkFailure.NotConfigured to "M-Pesa is not set up for this shop yet — check Payment settings on the web console"
            desc.isNotBlank() -> StkFailure.Unknown to desc
            else -> StkFailure.Unknown to "M-Pesa did not complete the payment"
        }
    }

    /**
     * True when a server error means "keep waiting" rather than "this is over".
     * Mirrors the backend's own pending detection in ReconcileSTKPayment.
     */
    fun isStillProcessing(message: String?): Boolean {
        val m = message?.lowercase().orEmpty()
        return m.contains("stk pending") ||
            m.contains("under processing") ||
            m.contains("being processed") ||
            m.contains("still processing") ||
            m.contains("4999") ||
            m.contains("500.001.1001")
    }

    /** True when the message names an outcome we should stop polling on. */
    fun isTerminal(message: String?): Boolean {
        if (isStillProcessing(message)) return false
        val m = message?.lowercase().orEmpty()
        return listOf(
            "1032", "1037", "2001", "1019", "1025",
            "cancelled", "canceled", "ds timeout", "request cancelled",
            "insufficient", "wrong pin", "not paid",
        ).any { m.contains(it) }
    }

    /** Advice line shown under a failure so staff know the next move. */
    fun recovery(failure: StkFailure): String = when (failure) {
        StkFailure.CancelledByCustomer -> "Send the prompt again, or take cash instead."
        StkFailure.InsufficientFunds -> "Ask the customer to top up, or split the payment."
        StkFailure.WrongPin -> "Send the prompt again so they can re-enter the PIN."
        StkFailure.NoResponse -> "Check the phone is on and has network, then resend."
        StkFailure.SubscriberLocked -> "Wait about a minute for their other transaction to clear, then resend."
        StkFailure.InvalidNumber -> "Confirm the number with the customer and try again."
        StkFailure.ProviderError -> "This is on M-Pesa's side — resend, or take cash and reconcile later."
        StkFailure.NetworkError -> "Check this phone's internet connection, then check the payment status again."
        StkFailure.NotConfigured -> "An owner or manager needs to finish M-Pesa setup before STK works."
        StkFailure.Unknown -> "Check the payment status before charging again — do not double-charge."
    }
}
