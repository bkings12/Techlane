package com.techlane.pos.data.printer

import android.Manifest
import android.annotation.SuppressLint
import android.bluetooth.BluetoothAdapter
import android.bluetooth.BluetoothDevice
import android.bluetooth.BluetoothManager
import android.bluetooth.BluetoothSocket
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.pm.PackageManager
import android.os.Build
import androidx.core.content.ContextCompat
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.callbackFlow
import kotlinx.coroutines.withContext
import java.io.IOException
import java.util.UUID
import javax.inject.Inject
import javax.inject.Singleton

/** Raised for anything Bluetooth-specific; [PrinterRepository] turns this into UI copy. */
class PrinterConnectionException(message: String, cause: Throwable? = null) : Exception(message, cause)

/**
 * Bluetooth Classic / SPP-RFCOMM I/O — the only class in this feature that
 * touches `android.bluetooth.*` directly.
 *
 * Every socket this opens is short-lived: connect, do the one operation
 * asked for, close. Thermal SPP printers like the MTP-II sleep aggressively
 * and handle a held-open connection poorly, so "one socket per print" is
 * more reliable in practice than trying to keep a persistent connection warm.
 */
@Singleton
class BluetoothPrinterConnection @Inject constructor(
    @ApplicationContext private val context: Context,
) {
    private val adapter: BluetoothAdapter?
        get() = context.getSystemService(BluetoothManager::class.java)?.adapter

    fun isSupported(): Boolean = adapter != null

    fun isBluetoothEnabled(): Boolean = adapter?.isEnabled == true

    /**
     * API 31+ requires the BLUETOOTH_CONNECT runtime permission to read a
     * device's name or open a socket to it. Below that, BLUETOOTH is a
     * normal (install-time) permission and is always granted once declared.
     */
    fun hasConnectPermission(): Boolean =
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            ContextCompat.checkSelfPermission(context, Manifest.permission.BLUETOOTH_CONNECT) ==
                PackageManager.PERMISSION_GRANTED
        } else {
            true
        }

    fun hasScanPermission(): Boolean =
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            ContextCompat.checkSelfPermission(context, Manifest.permission.BLUETOOTH_SCAN) ==
                PackageManager.PERMISSION_GRANTED
        } else {
            true
        }

    /** Runtime permissions to request; empty below API 31, where none are runtime. */
    fun runtimePermissions(): Array<String> =
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            arrayOf(Manifest.permission.BLUETOOTH_CONNECT, Manifest.permission.BLUETOOTH_SCAN)
        } else {
            emptyArray()
        }

    /** Devices already paired through Android's own Bluetooth settings. */
    @SuppressLint("MissingPermission") // caller checks hasConnectPermission() first
    fun pairedDevices(): List<PrinterDevice> {
        if (!hasConnectPermission()) return emptyList()
        return adapter?.bondedDevices.orEmpty().map { it.toPrinterDevice(bonded = true) }
    }

    /**
     * Nearby, not-yet-paired devices, as they're found. The MTP-II must still
     * be paired via Android's Bluetooth settings before a socket to it will
     * hold (that's where the PIN exchange happens) — this lets a technician
     * at least see it and know its name/MAC before doing that.
     */
    @SuppressLint("MissingPermission") // caller checks hasScanPermission() first
    fun scanDevices(): Flow<PrinterDevice> = callbackFlow {
        val bluetoothAdapter = adapter
        if (bluetoothAdapter == null || !hasScanPermission()) {
            close()
            return@callbackFlow
        }
        val receiver = object : BroadcastReceiver() {
            override fun onReceive(receiverContext: Context, intent: Intent) {
                if (intent.action != BluetoothDevice.ACTION_FOUND) return
                val device = intent.getBluetoothDeviceExtra() ?: return
                trySend(device.toPrinterDevice(bonded = false))
            }
        }
        context.registerReceiver(receiver, IntentFilter(BluetoothDevice.ACTION_FOUND))
        // A discovery already in flight badly degrades the connect attempt
        // that follows, so always start from a clean slate.
        bluetoothAdapter.cancelDiscovery()
        bluetoothAdapter.startDiscovery()
        awaitClose {
            runCatching { context.unregisterReceiver(receiver) }
            runCatching { bluetoothAdapter.cancelDiscovery() }
        }
    }

    @SuppressLint("MissingPermission") // guarded by hasScanPermission() above
    fun stopScan() {
        if (!hasScanPermission()) return
        runCatching { adapter?.cancelDiscovery() }
    }

    /** Opens a socket and immediately closes it — "is this printer reachable?". */
    suspend fun verifyConnection(address: String): Result<Unit> = withSocket(address) { }

    /** Opens a socket, writes [payload], flushes, and closes — one full job. */
    suspend fun printBytes(address: String, payload: ByteArray): Result<Unit> = withSocket(address) { socket ->
        socket.outputStream.write(payload)
        socket.outputStream.flush()
    }

    private suspend fun withSocket(
        address: String,
        block: suspend (BluetoothSocket) -> Unit,
    ): Result<Unit> = withContext(Dispatchers.IO) {
        val bluetoothAdapter = adapter
            ?: return@withContext Result.failure(PrinterConnectionException("This device has no Bluetooth adapter."))
        if (!bluetoothAdapter.isEnabled) {
            return@withContext Result.failure(PrinterConnectionException("Bluetooth is turned off."))
        }
        if (!hasConnectPermission()) {
            return@withContext Result.failure(PrinterConnectionException("Bluetooth permission is required."))
        }
        val device = runCatching { bluetoothAdapter.getRemoteDevice(address) }.getOrNull()
            ?: return@withContext Result.failure(PrinterConnectionException("\"$address\" is not a valid Bluetooth address."))

        var socket: BluetoothSocket? = null
        try {
            // An in-progress discovery competes for the radio with the connect
            // attempt below and is a well-known cause of RFCOMM connects hanging.
            // Cancelling it is a courtesy, not a requirement, so a missing scan
            // permission (a separate grant from the connect permission already
            // checked above) just means we skip it rather than fail the print.
            if (hasScanPermission()) bluetoothAdapter.cancelDiscovery()
            socket = openSocket(device)
            block(socket)
            Result.success(Unit)
        } catch (e: IOException) {
            Result.failure(PrinterConnectionException(describeConnectionFailure(e), e))
        } catch (e: SecurityException) {
            Result.failure(PrinterConnectionException("Bluetooth permission is required.", e))
        } finally {
            closeQuietly(socket)
        }
    }

    /**
     * Opens the RFCOMM channel. Tries the standard SDP-based API first; if
     * that fails, falls back to the well-known `createRfcommSocket(1)`
     * reflection call.
     *
     * The fallback exists because a large share of inexpensive Chinese-made
     * SPP printers — the MTP-II's whole hardware class — ship SDP records
     * Android's service-record lookup handles inconsistently, while channel 1
     * (SPP's conventional channel) almost always works. This is a documented,
     * widely-used workaround for exactly this situation, not a hack specific
     * to this app.
     */
    @SuppressLint("MissingPermission") // caller checks hasConnectPermission() first
    private fun openSocket(device: BluetoothDevice): BluetoothSocket {
        val viaServiceRecord = runCatching {
            device.createRfcommSocketToServiceRecord(SPP_UUID).also { it.connect() }
        }
        viaServiceRecord.getOrNull()?.let { return it }

        val viaChannel = runCatching {
            @Suppress("DEPRECATION")
            (device.javaClass.getMethod("createRfcommSocket", Int::class.javaPrimitiveType))
                .invoke(device, 1) as BluetoothSocket
        }.mapCatching { socket -> socket.also { it.connect() } }

        return viaChannel.getOrElse { fallbackError ->
            throw (viaServiceRecord.exceptionOrNull() as? IOException)
                ?: (fallbackError as? IOException)
                ?: IOException("Could not open a connection to the printer.", fallbackError)
        }
    }

    private fun closeQuietly(socket: BluetoothSocket?) {
        if (socket == null) return
        runCatching { socket.outputStream.close() }
        runCatching { socket.close() }
    }

    private fun describeConnectionFailure(e: IOException): String {
        val message = e.message?.lowercase().orEmpty()
        return when {
            "timeout" in message -> "Timed out connecting to the printer. Make sure it's powered on and in range."
            "refused" in message -> "The printer refused the connection. Make sure it isn't connected to another phone."
            else -> "Could not connect to the printer. Make sure it's powered on, in range, and paired in Bluetooth settings."
        }
    }

    @Suppress("DEPRECATION")
    private fun Intent.getBluetoothDeviceExtra(): BluetoothDevice? =
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            getParcelableExtra(BluetoothDevice.EXTRA_DEVICE, BluetoothDevice::class.java)
        } else {
            getParcelableExtra(BluetoothDevice.EXTRA_DEVICE)
        }

    @SuppressLint("MissingPermission") // caller checks hasConnectPermission() before this is reachable
    private fun BluetoothDevice.toPrinterDevice(bonded: Boolean): PrinterDevice =
        PrinterDevice(name = name.orEmpty(), address = address, bonded = bonded)

    private companion object {
        /** The standard Serial Port Profile UUID — see the class doc for why a fallback exists too. */
        val SPP_UUID: UUID = UUID.fromString("00001101-0000-1000-8000-00805F9B34FB")
    }
}
