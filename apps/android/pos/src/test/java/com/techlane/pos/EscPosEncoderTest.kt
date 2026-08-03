package com.techlane.pos

import com.techlane.pos.data.printer.EscPosEncoder
import com.techlane.pos.data.printer.PaperWidth
import com.techlane.pos.data.printer.PrinterTestPage
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * These pin the exact bytes sent to the printer. A thermal printer has no
 * screen to tell you your command sequence was wrong — the only feedback is
 * a receipt that looks broken in someone's hand — so the byte-level contract
 * here is worth being strict about.
 */
class EscPosEncoderTest {

    @Test
    fun `reset emits ESC at-sign`() {
        assertEquals(listOf(0x1B, 0x40), EscPosEncoder().reset().build().toIntList())
    }

    @Test
    fun `alignment emits ESC a with the correct code per direction`() {
        assertEquals(listOf(0x1B, 0x61, 0), EscPosEncoder().alignLeft().build().toIntList())
        assertEquals(listOf(0x1B, 0x61, 1), EscPosEncoder().alignCenter().build().toIntList())
        assertEquals(listOf(0x1B, 0x61, 2), EscPosEncoder().alignRight().build().toIntList())
    }

    @Test
    fun `bold on and off emit ESC E with 1 and 0`() {
        assertEquals(listOf(0x1B, 0x45, 1), EscPosEncoder().bold(true).build().toIntList())
        assertEquals(listOf(0x1B, 0x45, 0), EscPosEncoder().bold(false).build().toIntList())
    }

    @Test
    fun `double size on and off emit GS exclamation with the width+height and reset bytes`() {
        assertEquals(listOf(0x1D, 0x21, 0x11), EscPosEncoder().doubleSize(true).build().toIntList())
        assertEquals(listOf(0x1D, 0x21, 0x00), EscPosEncoder().doubleSize(false).build().toIntList())
    }

    @Test
    fun `text writes the line as bytes followed by a line feed`() {
        val bytes = EscPosEncoder().text("HELLO").build()
        assertEquals("HELLO\n", String(bytes, Charsets.ISO_8859_1))
    }

    @Test
    fun `an empty text call is just a line feed`() {
        val bytes = EscPosEncoder().text().build()
        assertEquals(listOf(0x0A), bytes.toIntList())
    }

    @Test
    fun `newLine emits a single LF`() {
        assertEquals(listOf(0x0A), EscPosEncoder().newLine().build().toIntList())
    }

    @Test
    fun `feed emits exactly that many line feeds and never a cut command`() {
        val bytes = EscPosEncoder().feed(4).build()
        assertEquals(List(4) { 0x0A }, bytes.toIntList())
        // GS V (0x1D 0x56) is the ESC/POS paper-cut command — the MTP-II has
        // no cutter, so this must never appear anywhere this encoder produces.
        assertFalse(bytes.toIntList().windowed(2).any { it == listOf(0x1D, 0x56) })
    }

    @Test
    fun `feed with a negative count is treated as zero rather than throwing`() {
        assertEquals(0, EscPosEncoder().feed(-3).build().size)
    }

    @Test
    fun `separator repeats the character to the requested column count then feeds`() {
        val bytes = EscPosEncoder().separator(32).build()
        assertEquals("-".repeat(32) + "\n", String(bytes, Charsets.ISO_8859_1))
    }

    @Test
    fun `a full sequence concatenates every command in call order`() {
        val bytes = EscPosEncoder()
            .reset()
            .alignCenter()
            .bold(true)
            .text("HI")
            .bold(false)
            .build()

        val expected = listOf(0x1B, 0x40, 0x1B, 0x61, 1, 0x1B, 0x45, 1) +
            "HI\n".toByteArray(Charsets.ISO_8859_1).map { it.toInt() and 0xFF } +
            listOf(0x1B, 0x45, 0)
        assertEquals(expected, bytes.toIntList())
    }

    // -------------------------------------------------------------- test page

    @Test
    fun `the test page never sends a cut command`() {
        val bytes = PrinterTestPage.build(PaperWidth.MM_58, "GOOJPRT MTP-II")
        assertFalse(bytes.toIntList().windowed(2).any { it == listOf(0x1D, 0x56) })
    }

    @Test
    fun `the test page starts with a reset and contains the required lines`() {
        val bytes = PrinterTestPage.build(PaperWidth.MM_58, "GOOJPRT MTP-II")
        assertEquals(listOf(0x1B, 0x40), bytes.toIntList().take(2))

        val text = String(bytes, Charsets.ISO_8859_1)
        assertTrue(text.contains("TECHLANE"))
        assertTrue(text.contains("PRINTER TEST SUCCESSFUL"))
        assertTrue(text.contains("Printer: GOOJPRT MTP-II"))
        assertTrue(text.contains("Paper:   58mm"))
        assertTrue(text.contains("Mode:    Bluetooth ESC/POS"))
        assertTrue(text.contains("Direct Bluetooth printing"))
        assertTrue(text.contains("is working correctly."))
        assertTrue(text.contains("TECHLANE POS"))
        assertTrue(text.contains("*** READY TO PRINT ***"))
    }

    @Test
    fun `the test page ends with several blank feed lines for a clean tear`() {
        val bytes = PrinterTestPage.build()
        val text = String(bytes, Charsets.ISO_8859_1)
        // The last non-blank content line, followed by nothing but feeds to the end.
        val afterReady = text.substringAfter("*** READY TO PRINT ***\n")
        assertEquals("\n".repeat(4), afterReady)
    }

    @Test
    fun `the test page separator width matches the paper's column count`() {
        val bytes58 = PrinterTestPage.build(PaperWidth.MM_58)
        val bytes80 = PrinterTestPage.build(PaperWidth.MM_80)
        assertTrue(String(bytes58, Charsets.ISO_8859_1).contains("-".repeat(PaperWidth.MM_58.columnsAtFontA)))
        assertTrue(String(bytes80, Charsets.ISO_8859_1).contains("-".repeat(PaperWidth.MM_80.columnsAtFontA)))
    }

    private fun ByteArray.toIntList(): List<Int> = map { it.toInt() and 0xFF }
}
