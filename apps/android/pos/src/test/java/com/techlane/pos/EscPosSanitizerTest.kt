package com.techlane.pos

import com.techlane.pos.data.printer.EscPosSanitizer
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Test

/**
 * The backend's shared ESC/POS receipt (`internal/receipts/escpos.go`) always
 * ends with a hardware cut command — correct for the 80mm counter printers it
 * targets, unsafe to forward to the MTP-II, which has no cutter. These pin
 * that the sanitizer actually removes it rather than passing it through.
 */
class EscPosSanitizerTest {

    @Test
    fun `strips the exact partial-cut sequence the backend emits`() {
        val body = "RECEIPT\n".toByteArray(Charsets.ISO_8859_1)
        val withCut = body + byteArrayOf(0x1D, 0x56, 0x01)

        val sanitized = EscPosSanitizer.stripTrailingCut(withCut, feedLines = 0)

        assertArrayEquals(body, sanitized)
    }

    @Test
    fun `also strips the full-cut variant`() {
        val body = "RECEIPT\n".toByteArray(Charsets.ISO_8859_1)
        val withCut = body + byteArrayOf(0x1D, 0x56, 0x00)

        assertArrayEquals(body, EscPosSanitizer.stripTrailingCut(withCut, feedLines = 0))
    }

    @Test
    fun `appends the requested number of feed lines after stripping`() {
        val withCut = "X".toByteArray() + byteArrayOf(0x1D, 0x56, 0x01)
        val sanitized = EscPosSanitizer.stripTrailingCut(withCut, feedLines = 4)

        assertEquals("X" + "\n".repeat(4), String(sanitized, Charsets.ISO_8859_1))
    }

    @Test
    fun `bytes with no cut command are only feed-padded, not corrupted`() {
        val body = "NO CUT HERE".toByteArray(Charsets.ISO_8859_1)
        val sanitized = EscPosSanitizer.stripTrailingCut(body, feedLines = 2)

        assertEquals("NO CUT HERE\n\n", String(sanitized, Charsets.ISO_8859_1))
    }

    @Test
    fun `never leaves a cut command anywhere in the output`() {
        val withCut = "A".repeat(50).toByteArray() + byteArrayOf(0x1D, 0x56, 0x01)
        val sanitized = EscPosSanitizer.stripTrailingCut(withCut)
        val ints = sanitized.map { it.toInt() and 0xFF }

        assertFalse(ints.windowed(3).any { it == listOf(0x1D, 0x56, 0x01) })
        assertFalse(ints.windowed(3).any { it == listOf(0x1D, 0x56, 0x00) })
    }

    @Test
    fun `bytes shorter than a cut command are left alone rather than throwing`() {
        val tiny = byteArrayOf(0x1D)
        assertArrayEquals(tiny + byteArrayOf(0x0A), EscPosSanitizer.stripTrailingCut(tiny, feedLines = 1))
    }

    @Test
    fun `a three-byte sequence that only coincidentally starts like a cut is untouched`() {
        // 0x1D 0x56 0x02 is not a cut command this backend emits — must not be stripped.
        val bytes = byteArrayOf(0x1D, 0x56, 0x02)
        assertArrayEquals(bytes, EscPosSanitizer.stripTrailingCut(bytes, feedLines = 0))
    }
}
