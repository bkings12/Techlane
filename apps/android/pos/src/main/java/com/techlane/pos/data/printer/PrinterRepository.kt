package com.techlane.pos.data.printer

import com.techlane.pos.data.remote.TechLaneApi
import com.techlane.pos.data.remote.toAppException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import okhttp3.ResponseBody
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
 * The one door into printing. Quick Charge, repair reprints, and Settings all
 * call through here and never import `android.bluetooth.*` or touch an
 * ESC/POS byte directly.
 *
 * [connectionState] is the single status a caller needs: it already folds in
 * "is a printer even saved" (`PrinterConnectionState.NotConfigured`), so nothing
 * downstream has to separately check preferences before trusting it.
 */
@Singleton
class PrinterRepository @Inject constructor(
    private val connection: BluetoothPrinterConnection,
    private val preferencesStore: PrinterPreferencesStore,
    private val api: TechLaneApi,
) {
    /** Outcome of the last connect/print action taken this process; null before any action. */
    private val liveStatus = MutableStateFlow<PrinterConnectionState?>(null)

    /**
     * Owns auto-print jobs so they outlive whatever screen triggered the
     * charge that started them — a technician who swipes away from Quick
     * Charge the instant "Paid" appears shouldn't cancel the receipt.
     */
    private val backgroundScope = CoroutineScope(SupervisorJob() + Dispatchers.IO)

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

    /** The fixed proof-of-life page from the initial Bluetooth printing phase. */
    suspend fun printTestPage(): Result<Unit> {
        val prefs = preferencesStore.preferences.first()
        val device = prefs.device ?: return notConfigured()
        liveStatus.value = PrinterConnectionState.Printing
        val payload = PrinterTestPage.build(prefs.paperWidth, device.displayName)
        return connection.printBytes(device.address, payload).reportResult(device)
    }

    /** A Quick Charge / POS sale receipt, for the customer at the counter. */
    suspend fun printSaleReceipt(saleId: String): Result<Unit> =
        fetchAndPrint { paper -> api.saleReceiptEscPos(saleId, paper) }

    /** A repair's final receipt — what the customer gets when they collect the device. */
    suspend fun printRepairReceipt(repairId: String): Result<Unit> =
        fetchAndPrint { paper -> api.repairReceiptEscPos(repairId, paper) }

    /** The slip handed over at intake, reprintable from the job if the original was lost or unreadable. */
    suspend fun printIntakeSlip(repairId: String): Result<Unit> =
        fetchAndPrint { paper -> api.repairIntakeSlipEscPos(repairId, paper) }

    /**
     * Best-effort background print for "Auto-print receipts". Fire-and-forget
     * by design: a charge that already succeeded must not be reported as
     * failed to the technician just because the printer was off, so this
     * never surfaces its error back through the charge flow — only through
     * the shared [connectionState], the same place a manual reprint's outcome
     * shows up.
     */
    fun autoPrintSaleReceiptIfEnabled(saleId: String) {
        backgroundScope.launch {
            val prefs = preferencesStore.preferences.first()
            if (prefs.isConfigured && prefs.autoPrintEnabled) printSaleReceipt(saleId)
        }
    }

    /**
     * The intake slip for a job that was just booked.
     *
     * Reports its outcome through [onOutcome] rather than swallowing it,
     * because unlike a sale receipt this one is the customer's proof that the
     * shop has their device — the counter needs to know whether paper actually
     * came out. Still never throws: a dead printer must not turn a booked job
     * into an error.
     */
    fun autoPrintIntakeSlipIfEnabled(repairId: String, onOutcome: (String) -> Unit) {
        backgroundScope.launch {
            val prefs = preferencesStore.preferences.first()
            when {
                !prefs.isConfigured -> onOutcome("Receipt ready — printer not connected")
                !prefs.autoPrintEnabled -> onOutcome("Receipt ready")
                else -> {
                    onOutcome("Printing receipt…")
                    printIntakeSlip(repairId).fold(
                        { onOutcome("Intake receipt printed") },
                        { onOutcome(it.message ?: "Could not print the receipt") },
                    )
                }
            }
        }
    }

    private suspend fun fetchAndPrint(fetch: suspend (paper: String) -> ResponseBody): Result<Unit> {
        val prefs = preferencesStore.preferences.first()
        val device = prefs.device ?: return notConfigured()
        liveStatus.value = PrinterConnectionState.Printing
        return try {
            val rendered = fetch(prefs.paperWidth.millimetres.toString()).use { it.bytes() }
            val payload = EscPosSanitizer.stripTrailingCut(rendered)
            connection.printBytes(device.address, payload).reportResult(device)
        } catch (e: Exception) {
            val message = e.toAppException().message ?: "Could not load the receipt to print."
            liveStatus.value = PrinterConnectionState.PrintFailed(message)
            Result.failure(PrinterConnectionException(message, e))
        }
    }

    private fun notConfigured(): Result<Unit> {
        val message = "No printer configured yet."
        liveStatus.value = PrinterConnectionState.ConnectionFailed(message)
        return Result.failure(PrinterConnectionException(message))
    }

    private fun Result<Unit>.reportResult(device: PrinterDevice): Result<Unit> = also { result ->
        result
            .onSuccess { liveStatus.value = PrinterConnectionState.PrintSuccess(device) }
            .onFailure { error -> liveStatus.value = PrinterConnectionState.PrintFailed(error.friendlyMessage()) }
    }

    private fun Throwable.friendlyMessage(): String =
        message?.takeIf { it.isNotBlank() } ?: "Something went wrong talking to the printer."
}
