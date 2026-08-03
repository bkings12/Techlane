package com.techlane.pos.data.printer

/**
 * A Bluetooth Classic device as the printer layer sees it — just enough to
 * show a technician what they're picking and to open an RFCOMM socket to it.
 * Never carries anything Android-framework-specific, so it stays trivial to
 * unit test and to fake in previews.
 */
data class PrinterDevice(
    val name: String,
    val address: String,
    /** True once Android has bonded with it — bonded devices connect far more
     *  reliably than an in-progress discovery result, so the picker leads with them. */
    val bonded: Boolean = false,
) {
    /** The label the settings screen and picker show for this device. */
    val displayName: String get() = name.ifBlank { "Unknown device" }
}

/** Paper width the printer takes. Only 58mm exists as hardware today, but the
 *  encoder's column math already keys off this rather than a hardcoded 32. */
enum class PaperWidth(val millimetres: Int, val columnsAtFontA: Int, val label: String) {
    MM_58(58, 32, "58mm"),
    MM_80(80, 48, "80mm"),
    ;

    companion object {
        val DEFAULT = MM_58

        fun fromMillimetres(mm: Int): PaperWidth = entries.firstOrNull { it.millimetres == mm } ?: DEFAULT
    }
}

/**
 * Where the connection currently stands. The Settings screen (and, later,
 * every payment/receipt flow) renders directly off this rather than inventing
 * its own notion of "is it working" — one state machine, everywhere printing
 * happens.
 */
sealed interface PrinterConnectionState {
    data object NotConfigured : PrinterConnectionState
    data object BluetoothDisabled : PrinterConnectionState
    data object PermissionRequired : PrinterConnectionState
    data object Idle : PrinterConnectionState
    data object Connecting : PrinterConnectionState
    data class Connected(val device: PrinterDevice) : PrinterConnectionState
    data object Printing : PrinterConnectionState
    data class PrintSuccess(val device: PrinterDevice) : PrinterConnectionState
    data class ConnectionFailed(val message: String) : PrinterConnectionState
    data class PrintFailed(val message: String) : PrinterConnectionState
}

/** Plain-language line for whatever [PrinterConnectionState] the UI is showing. */
fun PrinterConnectionState.statusLabel(): String = when (this) {
    PrinterConnectionState.NotConfigured -> "Not configured"
    PrinterConnectionState.BluetoothDisabled -> "Bluetooth is off"
    PrinterConnectionState.PermissionRequired -> "Bluetooth permission needed"
    PrinterConnectionState.Idle -> "Disconnected"
    PrinterConnectionState.Connecting -> "Connecting…"
    is PrinterConnectionState.Connected -> "Connected"
    PrinterConnectionState.Printing -> "Printing…"
    is PrinterConnectionState.PrintSuccess -> "Print successful"
    is PrinterConnectionState.ConnectionFailed -> "Connection failed"
    is PrinterConnectionState.PrintFailed -> "Print failed"
}

/** True for states where an operation is actively in flight. */
val PrinterConnectionState.isBusy: Boolean
    get() = this is PrinterConnectionState.Connecting || this is PrinterConnectionState.Printing

/** True once we're actually holding an open socket to the printer. */
val PrinterConnectionState.isConnected: Boolean
    get() = this is PrinterConnectionState.Connected

/** Surfaces the one-line reason something went wrong, never a raw exception. */
val PrinterConnectionState.errorMessage: String?
    get() = when (this) {
        is PrinterConnectionState.ConnectionFailed -> message
        is PrinterConnectionState.PrintFailed -> message
        else -> null
    }
