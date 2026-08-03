package com.techlane.pos.data.printer

/**
 * The one payload this phase actually prints. Kept separate from
 * [PrinterRepository] so the exact bytes sent to the printer are directly
 * unit-testable without touching Bluetooth or Android at all.
 */
object PrinterTestPage {

    fun build(paperWidth: PaperWidth = PaperWidth.DEFAULT, printerName: String = "GOOJPRT MTP-II"): ByteArray {
        val columns = paperWidth.columnsAtFontA
        return EscPosEncoder()
            .reset()
            .alignCenter()
            .bold(true)
            .doubleSize(true)
            .text("TECHLANE")
            .doubleSize(false)
            .newLine()
            .text("PRINTER TEST SUCCESSFUL")
            .bold(false)
            .newLine()
            .alignLeft()
            .separator(columns)
            .text("Printer: $printerName")
            .text("Paper:   ${paperWidth.label}")
            .text("Mode:    Bluetooth ESC/POS")
            .separator(columns)
            .newLine()
            .text("Direct Bluetooth printing")
            .text("is working correctly.")
            .newLine()
            .alignCenter()
            .bold(true)
            .text("TECHLANE POS")
            .bold(false)
            .newLine()
            .text("*** READY TO PRINT ***")
            // No cut command — the MTP-II has no automatic cutter. This feed is
            // the only thing standing between "torn cleanly" and "lost the last line".
            .feed(4)
            .build()
    }
}
