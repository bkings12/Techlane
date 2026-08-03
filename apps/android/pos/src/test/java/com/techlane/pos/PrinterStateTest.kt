package com.techlane.pos

import com.techlane.pos.data.printer.PaperWidth
import com.techlane.pos.data.printer.PrinterConnectionState
import com.techlane.pos.data.printer.PrinterDevice
import com.techlane.pos.data.printer.PrinterPreferences
import com.techlane.pos.data.printer.errorMessage
import com.techlane.pos.data.printer.isBusy
import com.techlane.pos.data.printer.isConnected
import com.techlane.pos.data.printer.statusLabel
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class PrinterStateTest {

    private val device = PrinterDevice(name = "GOOJPRT MTP-II", address = "66:22:33:AA:BB:CC", bonded = true)

    @Test
    fun `status labels match the words the settings screen is required to show`() {
        assertEquals("Not configured", PrinterConnectionState.NotConfigured.statusLabel())
        assertEquals("Disconnected", PrinterConnectionState.Idle.statusLabel())
        assertEquals("Connected", PrinterConnectionState.Connected(device).statusLabel())
    }

    @Test
    fun `busy states are exactly connecting and printing`() {
        assertTrue(PrinterConnectionState.Connecting.isBusy)
        assertTrue(PrinterConnectionState.Printing.isBusy)
        assertFalse(PrinterConnectionState.Idle.isBusy)
        assertFalse(PrinterConnectionState.Connected(device).isBusy)
        assertFalse(PrinterConnectionState.PrintSuccess(device).isBusy)
    }

    @Test
    fun `isConnected is true only for the Connected state`() {
        assertTrue(PrinterConnectionState.Connected(device).isConnected)
        assertFalse(PrinterConnectionState.PrintSuccess(device).isConnected)
        assertFalse(PrinterConnectionState.Idle.isConnected)
    }

    @Test
    fun `errorMessage surfaces only for the two failure states`() {
        assertEquals("printer off", PrinterConnectionState.ConnectionFailed("printer off").errorMessage)
        assertEquals("ribbon jam", PrinterConnectionState.PrintFailed("ribbon jam").errorMessage)
        assertNull(PrinterConnectionState.Connected(device).errorMessage)
        assertNull(PrinterConnectionState.Idle.errorMessage)
    }

    @Test
    fun `paper width falls back to the 58mm default for an unknown value`() {
        assertEquals(PaperWidth.MM_58, PaperWidth.fromMillimetres(58))
        assertEquals(PaperWidth.MM_80, PaperWidth.fromMillimetres(80))
        assertEquals(PaperWidth.DEFAULT, PaperWidth.fromMillimetres(112))
    }

    @Test
    fun `preferences report configured only once an address is saved`() {
        assertFalse(PrinterPreferences().isConfigured)
        assertFalse(PrinterPreferences(name = "GOOJPRT MTP-II").isConfigured)
        assertTrue(PrinterPreferences(name = "GOOJPRT MTP-II", address = "66:22:33:AA:BB:CC").isConfigured)
    }

    @Test
    fun `preferences derive a device only when an address is present`() {
        assertNull(PrinterPreferences().device)
        val prefs = PrinterPreferences(name = "GOOJPRT MTP-II", address = "66:22:33:AA:BB:CC")
        assertEquals(PrinterDevice("GOOJPRT MTP-II", "66:22:33:AA:BB:CC"), prefs.device)
    }

    @Test
    fun `a blank device name falls back to a readable placeholder`() {
        assertEquals("Unknown device", PrinterDevice(name = "", address = "00:11:22:33:44:55").displayName)
        assertEquals("GOOJPRT MTP-II", device.displayName)
    }
}
