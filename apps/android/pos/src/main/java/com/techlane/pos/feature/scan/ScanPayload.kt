package com.techlane.pos.feature.scan

/**
 * What a scanned code turned out to be.
 *
 * The one format that genuinely exists today is the intake-slip QR the backend
 * already prints — `techlane://repair-pickup/<PICKUP_CODE>` from
 * internal/repair/pickup.go. Everything else here is recognised loosely and
 * handed on as a plain search term, so the scanner is not coupled to a payload
 * structure that has not been agreed yet.
 *
 * When device barcodes and inventory SKUs get their own printed formats, they
 * become new [ScanResult] variants; nothing in the camera layer has to change.
 */
sealed interface ScanResult {

    /** The QR printed on a TechLane intake slip. Resolvable to exactly one job. */
    data class RepairPickup(val code: String) : ScanResult

    /** A 15-digit IMEI. Unambiguous enough to treat as a device lookup. */
    data class DeviceIdentifier(val value: String) : ScanResult

    /** Anything else legible: passed to search rather than guessed at. */
    data class Unknown(val raw: String) : ScanResult
}

object ScanPayloads {

    private const val PICKUP_SCHEME = "techlane://repair-pickup/"

    fun parse(raw: String): ScanResult {
        val value = raw.trim()
        if (value.isEmpty()) return ScanResult.Unknown("")

        if (value.startsWith(PICKUP_SCHEME, ignoreCase = true)) {
            val code = value.removePrefix(PICKUP_SCHEME).trim('/').uppercase()
            if (code.isNotBlank()) return ScanResult.RepairPickup(code)
        }

        // Pickup codes are printed as PK-XXXXXX and are often keyed in by hand.
        if (value.uppercase().matches(Regex("^PK-?[A-Z0-9]{4,8}$"))) {
            return ScanResult.RepairPickup(value.uppercase().replace("-", "").let {
                if (it.startsWith("PK")) it else "PK$it"
            })
        }

        val digits = value.filter(Char::isDigit)
        if (digits.length == 15 && digits == value) return ScanResult.DeviceIdentifier(digits)

        return ScanResult.Unknown(value)
    }
}
