package com.techlane.pos.data.printer

import java.io.ByteArrayOutputStream
import java.nio.charset.Charset

/**
 * A small, deliberately narrow ESC/POS byte builder.
 *
 * This exists instead of a third-party ESC/POS library because the command
 * set a 58mm receipt actually needs — init, alignment, bold, double-size text,
 * line feeds — is about a dozen bytes total, and owning it means no dependency
 * to chase when a printer quirk needs a one-line fix. It is pure Kotlin (no
 * Android types), so it runs and is tested on the plain JVM.
 *
 * Every method returns `this` so a print job reads as one fluent sequence, and
 * every method is a thin, named wrapper around one or two control bytes so
 * adding QR codes, barcodes or a raster logo later is "add a method", not
 * "learn how this class works".
 */
class EscPosEncoder(
    /**
     * The codepage a raw byte maps to on the wire. ISO-8859-1 is a safe
     * default for ESC/POS: it is a 1-byte-per-character superset of ASCII, so
     * plain English/Swahili receipt text round-trips byte-for-byte without
     * needing a codepage-select command first. A shop that needs a currency
     * glyph outside Latin-1 will need a real codepage command added here —
     * intentionally not attempted until there's a printer in front of us to
     * verify it against.
     */
    private val charset: Charset = Charsets.ISO_8859_1,
) {
    private val buffer = ByteArrayOutputStream()

    /** ESC @ — resets the printer to its power-on state. Always call first. */
    fun reset(): EscPosEncoder = raw(0x1B, 0x40)

    fun alignLeft(): EscPosEncoder = align(0)
    fun alignCenter(): EscPosEncoder = align(1)
    fun alignRight(): EscPosEncoder = align(2)

    /** ESC a n — text justification for everything printed until changed. */
    private fun align(n: Int): EscPosEncoder = raw(0x1B, 0x61, n)

    /** ESC E n — bold on/off for everything printed until changed. */
    fun bold(on: Boolean): EscPosEncoder = raw(0x1B, 0x45, if (on) 1 else 0)

    /**
     * GS ! n — double width/height (or back to normal). Most 58mm printers,
     * the MTP-II included, support this even without supporting finer
     * magnification levels, so on/off is all this exposes.
     */
    fun doubleSize(on: Boolean): EscPosEncoder = raw(0x1D, 0x21, if (on) 0x11 else 0x00)

    /** Writes [line] followed by a line feed, in the codepage set on this encoder. */
    fun text(line: String = ""): EscPosEncoder {
        if (line.isNotEmpty()) buffer.write(line.toByteArray(charset))
        return newLine()
    }

    /** LF with no text — a blank line. */
    fun newLine(): EscPosEncoder = raw(0x0A)

    /** [count] blank lines, e.g. the final tear-off gap. Never a cut command —
     *  the MTP-II is a portable printer with no automatic cutter. */
    fun feed(count: Int): EscPosEncoder {
        repeat(count.coerceAtLeast(0)) { newLine() }
        return this
    }

    /** A full-width rule of repeated [char] sized to [columns] (paper width). */
    fun separator(columns: Int, char: Char = '-'): EscPosEncoder = text(char.toString().repeat(columns))

    private fun raw(vararg bytes: Int): EscPosEncoder {
        bytes.forEach { buffer.write(it) }
        return this
    }

    fun build(): ByteArray = buffer.toByteArray()
}
