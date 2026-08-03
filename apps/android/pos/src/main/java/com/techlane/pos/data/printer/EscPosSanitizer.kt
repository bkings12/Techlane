package com.techlane.pos.data.printer

/**
 * Adapts an ESC/POS document built for a different printer class before it
 * reaches the MTP-II.
 *
 * The backend renders one shared ESC/POS byte stream for receipts and intake
 * slips (`internal/receipts/escpos.go`) — the same content the web console's
 * counter till prints from. Printing it verbatim over Bluetooth would send
 * two things this printer cannot honour safely:
 *
 *  - `GS V` — a hardware paper-cut command. The backend's stream always ends
 *    with one, because it targets 80mm cutter-equipped counter printers. The
 *    MTP-II is a 58mm *portable* printer with no cutter at all, and the whole
 *    reason the original Bluetooth printing spec called this out explicitly
 *    is that forwarding a cut command to a printer that can't act on it is,
 *    at best, a no-op and at worst mishandled by cheaper clone firmware.
 *  - column width tuned for 80mm (42 columns) rather than 58mm (32 columns),
 *    which this sanitizer does not attempt to reflow — see [stripTrailingCut].
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
     *
     * Deliberately does *not* attempt to re-wrap the document's 80mm-tuned
     * line widths for 58mm paper — the printer's own firmware wraps or
     * truncates an over-width line, which is a cosmetic imperfection, not a
     * safety one. Reflowing correctly would mean parsing the byte stream's
     * text runs back out from around the alignment/style commands, which is
     * real complexity for a problem the shop hasn't reported yet; noted here
     * rather than solved speculatively.
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
