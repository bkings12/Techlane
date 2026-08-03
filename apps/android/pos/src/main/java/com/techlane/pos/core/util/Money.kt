package com.techlane.pos.core.util

import java.text.DecimalFormat
import java.text.DecimalFormatSymbols
import java.util.Locale

private val groupedFormat = DecimalFormat("#,##0", DecimalFormatSymbols(Locale.US))
private val groupedDecimalFormat = DecimalFormat("#,##0.00", DecimalFormatSymbols(Locale.US))

/** "KES 1,500" — whole shillings unless the amount actually has cents. */
fun formatKes(amount: Double, withCurrency: Boolean = true): String {
    val body = if (amount % 1.0 == 0.0) groupedFormat.format(amount) else groupedDecimalFormat.format(amount)
    return if (withCurrency) "KES $body" else body
}

/** Groups the raw digit string a keypad produces: "12500" -> "12,500". */
fun groupDigits(digits: String): String {
    val value = digits.toLongOrNull() ?: return digits
    return groupedFormat.format(value)
}
