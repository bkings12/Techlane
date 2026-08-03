package com.techlane.pos.core.util

/**
 * Safaricom STK needs a 2547XXXXXXXX / 2541XXXXXXXX MSISDN. Staff type whatever
 * the customer reads out — 07…, +2547…, 7…, with spaces. Normalising on-device
 * (and showing the result before we send) prevents a prompt going to a number
 * that merely looks right.
 */
object Msisdn {

    /** Returns the 12-digit MSISDN, or null when the input can't be one. */
    fun normalise(raw: String): String? {
        val digits = raw.filter { it.isDigit() }
        val national = when {
            digits.length == 12 && digits.startsWith("254") -> digits.removePrefix("254")
            digits.length == 10 && digits.startsWith("0") -> digits.removePrefix("0")
            digits.length == 9 -> digits
            // 00254… international prefix
            digits.length == 14 && digits.startsWith("00254") -> digits.removePrefix("00254")
            else -> return null
        }
        if (national.length != 9) return null
        // Kenyan mobile prefixes are 7xx (Safaricom/Airtel) and 1xx (Safaricom).
        if (national[0] != '7' && national[0] != '1') return null
        return "254$national"
    }

    fun isValid(raw: String): Boolean = normalise(raw) != null

    /** "254712345678" -> "0712 345 678" for display back to the technician. */
    fun formatLocal(raw: String): String {
        val msisdn = normalise(raw) ?: return raw
        val national = msisdn.removePrefix("254")
        return "0${national.substring(0, 3)} ${national.substring(3, 6)} ${national.substring(6)}"
    }

    /** Masked form for receipts and history rows: "0712 *** 678". */
    fun mask(raw: String): String {
        val msisdn = normalise(raw) ?: return raw
        val national = msisdn.removePrefix("254")
        return "0${national.substring(0, 3)} *** ${national.substring(6)}"
    }
}
