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

enum class PaymentMethod(val wire: String, val display: String) {
    MpesaStk("mpesa_stk", "M-Pesa prompt"),
    Cash("cash", "Cash"),
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
)
