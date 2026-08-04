package com.techlane.pos.domain.model

/** What the customer is paying for. Optional — a bare amount is a valid charge. */
sealed interface ChargeTarget {

    /** No line chosen: the prompt is just "an amount". */
    data object None : ChargeTarget

    /** A stock item. Priced and decremented server-side from the catalog. */
    data class Product(
        val variantId: String,
        val name: String,
        val unitPrice: Double,
        val quantity: Int = 1,
        val availableQty: Int = 0,
    ) : ChargeTarget

    /** Labour. Priced by whoever is at the counter, not by the catalog. */
    data class Service(
        val id: String?,
        val name: String,
        val price: Double?,
    ) : ChargeTarget

    /** Receipt line text. */
    val label: String
        get() = when (this) {
            None -> "Quick payment"
            is Product -> if (quantity > 1) "$name × $quantity" else name
            is Service -> name
        }

    /** Amount the target implies, or null when the technician must type one. */
    val suggestedAmount: Double?
        get() = when (this) {
            None -> null
            is Product -> unitPrice * quantity
            is Service -> price
        }
}

/**
 * Wire values mirror `internal/payments` — `mpesa_c2b` is the backend's name
 * for a Paybill/till payment the customer made themselves, so we adopt it
 * rather than inventing a "paybill" the server would reject.
 */
enum class PaymentMethod(val wire: String, val display: String) {
    MpesaStk("mpesa_stk", "M-Pesa prompt"),
    Cash("cash", "Cash"),
    Paybill("mpesa_c2b", "Paybill"),
    ;

    /** Only STK pushes a prompt and then waits on Daraja. */
    val isPrompted: Boolean get() = this == MpesaStk

    /** Paybill is money already received; we are recording it, not collecting it. */
    val needsReference: Boolean get() = this == Paybill

    /** Receipt wording, matching what the printed slip should say. */
    val receiptLabel: String
        get() = when (this) {
            MpesaStk -> "M-PESA STK"
            Cash -> "CASH"
            Paybill -> "M-PESA PAYBILL"
        }
}

/**
 * Basic shape check for an M-Pesa transaction code (e.g. QHK7T9XXX): ten
 * alphanumerics, which is what Safaricom issues. Deliberately permissive about
 * case and surrounding spaces — a technician copying a code off a customer's
 * phone should not be fighting the field.
 */
object MpesaReference {
    private val PATTERN = Regex("^[A-Z0-9]{10}$")

    fun normalise(input: String): String = input.trim().uppercase()

    fun isValid(input: String): Boolean = PATTERN.matches(normalise(input))

    /** Null when acceptable, otherwise the reason to show under the field. */
    fun validationError(input: String): String? = when {
        input.isBlank() -> "Enter the M-Pesa code from the customer's message."
        !isValid(input) -> "That doesn't look like an M-Pesa code — they are 10 letters and numbers."
        else -> null
    }
}

/** One charge attempt as the UI knows it. */
data class ChargeRequest(
    val branchId: String,
    val locationId: String,
    val amount: Double,
    val method: PaymentMethod,
    val phone: String?,
    val target: ChargeTarget,
    /**
     * Stable across retries of the same network call so a flaky connection
     * cannot turn one charge into two sales.
     */
    val idempotencyKey: String,
    /** M-Pesa transaction code, for a Paybill payment the customer already made. */
    val reference: String? = null,
)
