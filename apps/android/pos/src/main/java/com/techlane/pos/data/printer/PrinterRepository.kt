package com.techlane.pos.data.printer

import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.first
import javax.inject.Inject
import javax.inject.Singleton

/** What the picker should do before it opens, decided without touching the radio. */
sealed interface PrinterAvailability {
    data object Ready : PrinterAvailability
    data object Unsupported : PrinterAvailability
    data object PermissionRequired : PrinterAvailability
    data object BluetoothDisabled : PrinterAvailability
}

/**
 * The one door into printing. Everything above this — today, the Settings
 * screen; later, Quick Charge, POS checkout, repair payments, and reprint —
 * calls through here and never imports `android.bluetooth.*` or touches an
 * ESC/POS byte directly. That boundary is the whole point of this phase:
 * prove the Bluetooth link works behind an API shape that doesn't have to
 * change when a payment screen starts calling `printReceipt(receipt)`.
 *
 * [connectionState] is the single status a caller needs: it already folds in
 * "is a printer even saved" (`PrinterConnectionState.NotConfigured`), so nothing
 * downstream has to separately check preferences before trusting it.
 */
@Singleton
class PrinterRepository @Inject constructor(
    private val connection: BluetoothPrinterConnection,
    private val preferencesStore: PrinterPreferencesStore,
) {
    /** Outcome of the last connect/print action taken this process; null before any action. */
    private val liveStatus = MutableStateFlow<PrinterConnectionState?>(null)

    val preferences: Flow<PrinterPreferences> = preferencesStore.preferences

    /** Resolved status: "not configured" always wins, otherwise the last live outcome, else disconnected. */
    val connectionState: Flow<PrinterConnectionState> = combine(preferencesStore.preferences, liveStatus) { prefs, live ->
        when {
            !prefs.isConfigured -> PrinterConnectionState.NotConfigured
            live != null -> live
            else -> PrinterConnectionState.Idle
        }
    }

    fun isBluetoothSupported(): Boolean = connection.isSupported()

    fun isBluetoothEnabled(): Boolean = connection.isBluetoothEnabled()

    fun hasRequiredPermissions(): Boolean = connection.hasConnectPermission() && connection.hasScanPermission()

    fun requiredPermissions(): Array<String> = connection.runtimePermissions()

    /** What the device picker should check for before it opens. */
    fun checkAvailability(): PrinterAvailability = when {
        !connection.isSupported() -> PrinterAvailability.Unsupported
        !hasRequiredPermissions() -> PrinterAvailability.PermissionRequired
        !connection.isBluetoothEnabled() -> PrinterAvailability.BluetoothDisabled
        else -> PrinterAvailability.Ready
    }

    fun pairedDevices(): List<PrinterDevice> = connection.pairedDevices()

    /** Nearby, not-yet-paired devices as they're discovered. Call [stopScan] when done. */
    fun scanDevices(): Flow<PrinterDevice> = connection.scanDevices()

    fun stopScan() = connection.stopScan()

    /**
     * Saves [device] as the shop's printer and verifies it's actually
     * reachable before calling it connected — a save that silently can't
     * connect is worse than no save at all, since it hides the problem until
     * the next receipt.
     */
    suspend fun selectPrinter(device: PrinterDevice) {
        liveStatus.value = PrinterConnectionState.Connecting
        preferencesStore.savePrinter(device)
        connection.verifyConnection(device.address)
            .onSuccess { liveStatus.value = PrinterConnectionState.Connected(device) }
            .onFailure { error -> liveStatus.value = PrinterConnectionState.ConnectionFailed(error.friendlyMessage()) }
    }

    /** Forgets the saved printer. Status immediately reflects NotConfigured. */
    suspend fun forgetPrinter() {
        preferencesStore.clearPrinter()
        liveStatus.value = null
    }

    suspend fun setPaperWidth(width: PaperWidth) = preferencesStore.setPaperWidth(width)

    suspend fun setAutoPrintEnabled(enabled: Boolean) = preferencesStore.setAutoPrintEnabled(enabled)

    /**
     * Prints the fixed proof-of-life page from requirement #7. This is
     * deliberately the only thing this repository prints today — see the
     * class doc for what plugs in here next.
     */
    suspend fun printTestPage() {
        val prefs = preferencesStore.preferences.first()
        val device = prefs.device
        if (device == null) {
            liveStatus.value = PrinterConnectionState.ConnectionFailed("No printer configured yet.")
            return
        }
        liveStatus.value = PrinterConnectionState.Printing
        val payload = PrinterTestPage.build(prefs.paperWidth, device.displayName)
        connection.printBytes(device.address, payload)
            .onSuccess { liveStatus.value = PrinterConnectionState.PrintSuccess(device) }
            .onFailure { error -> liveStatus.value = PrinterConnectionState.PrintFailed(error.friendlyMessage()) }
    }

    private fun Throwable.friendlyMessage(): String =
        message?.takeIf { it.isNotBlank() } ?: "Something went wrong talking to the printer."
}
