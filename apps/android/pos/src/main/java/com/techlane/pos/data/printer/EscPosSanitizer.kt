package com.techlane.pos.data.printer

/**
 * Adapts an ESC/POS document built for a different printer class before it
 * reaches the MTP-II.
 *
 * The backend renders one shared ESC/POS byte stream for receipts and intake
 * slips (`internal/receipts/escpos.go`), already laid out at the requested
 * paper's column width (see the `paper` query param [PrinterRepository]
 * passes on every `.escpos` call) — the same content the web console's
 * counter till prints from. Printing it verbatim over Bluetooth would still
 * send one thing this printer cannot honour safely: `GS V`, a hardware
 * paper-cut command. The backend's stream always ends with one, because it
 * also targets 80mm cutter-equipped counter printers. The MTP-II is a 58mm
 * *portable* printer with no cutter at all, and the whole reason the original
 * Bluetooth printing spec called this out explicitly is that forwarding a cut
 * command to a printer that can't act on it is, at best, a no-op and at worst
 * mishandled by cheaper clone firmware.
 */
object EscPosSanitizer {

    /** The exact 3-byte sequences `internal/receipts/escpos.go` can emit today. */
    private val KNOWN_CUT_COMMANDS: List<ByteArray> = listOf(
        byteArrayOf(0x1D, 0x56, 0x00), // GS V 0 — full cut
        byteArrayOf(0x1D, 0x56, 0x01), // GS V 1 — partial cut (what the backend actually sends)
    )

    /**
     * Drops a trailing cut command, if the bytes end with one, and replaces it
     * with [feedLines] blank lines so there is still enough paper past the
     * last printed line for a clean manual tear.
     */
    fun stripTrailingCut(bytes: ByteArray, feedLines: Int = 4): ByteArray {
        val cutLength = KNOWN_CUT_COMMANDS.firstOrNull { bytes.endsWith(it) }?.size
        val body = if (cutLength != null) bytes.copyOfRange(0, bytes.size - cutLength) else bytes
        return body + ByteArray(feedLines.coerceAtLeast(0)) { 0x0A }
    }

    private fun ByteArray.endsWith(suffix: ByteArray): Boolean {
        if (suffix.size > size) return false
        val start = size - suffix.size
        for (i in suffix.indices) {
            if (this[start + i] != suffix[i]) return false
        }
        return true
    }
}
