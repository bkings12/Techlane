package com.techlane.pos.domain.model

/** A sale row for the history list — deliberately small, mirrors JobSummary. */
data class SaleSummary(
    val id: String,
    val reference: String,
    val customerName: String?,
    val total: Double,
    val paymentMethod: String,
    val status: String,
    val createdAt: Long?,
    /** First item's name, for a one-line summary on the row ("Oraimo Charger", "+2 more"). */
    val itemSummary: String?,
    val itemCount: Int,
) {
    val isCompleted: Boolean get() = status == "completed"
    val isReversed: Boolean get() = status == "reversed"
}

data class SaleLineItem(
    val description: String,
    val quantity: Int,
    val unitPrice: Double,
    val lineTotal: Double,
    /** Null unless the caller has reports.read — see SaleDto's doc comment. */
    val unitCost: Double?,
    val margin: Double?,
) {
    val hasCostInfo: Boolean get() = unitCost != null
}

/** Everything Sale Details renders. Loaded from GET sales/{id} or the offline cache. */
data class SaleDetail(
    val id: String,
    val reference: String,
    val status: String,
    val channel: String,
    val branchName: String?,
    val cashierName: String?,
    val customerName: String?,
    val customerPhone: String?,
    val createdAt: Long?,
    val items: List<SaleLineItem>,
    val subtotal: Double,
    val taxTotal: Double,
    val discountTotal: Double,
    val total: Double,
    val paidTotal: Double,
    val balanceDue: Double,
    val paymentMethod: String,
    val paymentStatus: String,
    val paymentReference: String?,
    /** Set only for a repair-payment row surfaced from Activity — see ChargeRecordEntity.repairId.
     *  A genuine sales.sales row is never repair-linked (repair-attached line items are billed
     *  as payable_type='repair' and never produce a sales.sales row at all), so this stays null
     *  for every real SaleDetail; it exists so the UI has one type to render either case through. */
    val relatedJobId: String? = null,
    val relatedJobLabel: String? = null,
    /** True when this came from the local cache rather than a fresh network fetch. */
    val fromCache: Boolean = false,
) {
    val isCompleted: Boolean get() = status == "completed"
    val paymentIsSettled: Boolean get() = paymentStatus == "allocated" || paymentStatus == "confirmed"
}

/** Filters for the sales history list — kept flat, one param per backend query param. */
data class SalesFilter(
    val query: String = "",
    val method: String? = null,
    val status: String? = null,
    val fromEpochDay: Long? = null,
    val toEpochDay: Long? = null,
) {
    val isEmpty: Boolean get() = query.isBlank() && method == null && status == null && fromEpochDay == null && toEpochDay == null
}

/**
 * Where tapping an Activity row should go. A repair payment never produces a
 * sales.sales row (it's billed as payable_type='repair'), so it must route to
 * the job, never to a fabricated Sale Details — see ChargeRecordEntity.repairId.
 */
sealed interface ChargeRowDestination {
    data class Sale(val saleId: String) : ChargeRowDestination
    data class Job(val repairId: String) : ChargeRowDestination
    data object None : ChargeRowDestination
}

fun chargeRowDestination(saleId: String?, repairId: String?): ChargeRowDestination = when {
    saleId != null -> ChargeRowDestination.Sale(saleId)
    repairId != null -> ChargeRowDestination.Job(repairId)
    else -> ChargeRowDestination.None
}

fun paymentMethodLabel(wire: String): String = when (wire) {
    "mpesa_stk" -> "M-Pesa"
    "mpesa_c2b" -> "Paybill"
    "cash" -> "Cash"
    "bank" -> "Bank"
    "" -> "—"
    else -> wire.replaceFirstChar(Char::uppercase)
}
