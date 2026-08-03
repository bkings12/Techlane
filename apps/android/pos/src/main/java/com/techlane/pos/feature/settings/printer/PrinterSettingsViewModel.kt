package com.techlane.pos.feature.settings.printer

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.techlane.pos.data.printer.PaperWidth
import com.techlane.pos.data.printer.PrinterAvailability
import com.techlane.pos.data.printer.PrinterConnectionState
import com.techlane.pos.data.printer.PrinterDevice
import com.techlane.pos.data.printer.PrinterPreferences
import com.techlane.pos.data.printer.PrinterRepository
import com.techlane.pos.data.printer.isBusy
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

/** Why the picker can't scan/connect yet, and what the screen should offer to fix it. */
enum class PickerBlockReason { Unsupported, PermissionRequired, BluetoothDisabled }

/** What the device-picker sheet is doing, if it's open at all. */
sealed interface DevicePickerState {
    data object Closed : DevicePickerState
    data class Open(
        val scanning: Boolean = false,
        val devices: List<PrinterDevice> = emptyList(),
        val blockedReason: PickerBlockReason? = null,
    ) : DevicePickerState
}

data class PrinterSettingsUiState(
    val preferences: PrinterPreferences = PrinterPreferences(),
    val connection: PrinterConnectionState = PrinterConnectionState.NotConfigured,
    val picker: DevicePickerState = DevicePickerState.Closed,
    val message: String? = null,
) {
    val isBusy: Boolean get() = connection.isBusy
}

@HiltViewModel
class PrinterSettingsViewModel @Inject constructor(
    private val printers: PrinterRepository,
) : ViewModel() {

    private val picker = MutableStateFlow<DevicePickerState>(DevicePickerState.Closed)
    private val message = MutableStateFlow<String?>(null)
    private var scanJob: Job? = null

    val state: StateFlow<PrinterSettingsUiState> = combine(
        printers.preferences,
        printers.connectionState,
        picker,
        message,
    ) { prefs, connection, pickerState, msg ->
        PrinterSettingsUiState(prefs, connection, pickerState, msg)
    }.stateIn(viewModelScope, SharingStarted.WhileSubscribed(5_000), PrinterSettingsUiState())

    /** The permissions the UI must request before the picker can actually scan/connect. */
    fun requiredPermissions(): Array<String> = printers.requiredPermissions()

    /**
     * Entry point for "Select Printer". Checks Bluetooth support, permission
     * and the radio being on up front, so the picker either opens ready to
     * use or explains exactly what's missing instead of opening empty.
     */
    fun openPicker() {
        when (printers.checkAvailability()) {
            PrinterAvailability.Unsupported ->
                picker.value = DevicePickerState.Open(blockedReason = PickerBlockReason.Unsupported)
            PrinterAvailability.PermissionRequired ->
                picker.value = DevicePickerState.Open(blockedReason = PickerBlockReason.PermissionRequired)
            PrinterAvailability.BluetoothDisabled ->
                picker.value = DevicePickerState.Open(blockedReason = PickerBlockReason.BluetoothDisabled)
            PrinterAvailability.Ready -> {
                picker.value = DevicePickerState.Open(devices = printers.pairedDevices())
                startScan()
            }
        }
    }

    /** Called once the caller has walked the user through the permission prompt or enabling Bluetooth. */
    fun retryAfterResolving(reason: PickerBlockReason) {
        val current = picker.value
        if (current !is DevicePickerState.Open || current.blockedReason != reason) return
        openPicker()
    }

    private fun startScan() {
        scanJob?.cancel()
        scanJob = viewModelScope.launch {
            val open = picker.value as? DevicePickerState.Open ?: return@launch
            picker.value = open.copy(scanning = true)
            printers.scanDevices().collect { found ->
                val current = picker.value as? DevicePickerState.Open ?: return@collect
                if (current.devices.any { it.address == found.address }) return@collect
                picker.value = current.copy(devices = current.devices + found)
            }
        }
    }

    fun closePicker() {
        scanJob?.cancel()
        scanJob = null
        printers.stopScan()
        picker.value = DevicePickerState.Closed
    }

    fun selectDevice(device: PrinterDevice) {
        scanJob?.cancel()
        scanJob = null
        printers.stopScan()
        picker.value = DevicePickerState.Closed
        viewModelScope.launch {
            printers.selectPrinter(device)
        }
    }

    fun forgetPrinter() {
        viewModelScope.launch {
            printers.forgetPrinter()
            message.value = "Printer removed"
        }
    }

    fun testPrint() {
        viewModelScope.launch { printers.printTestPage() }
    }

    fun setPaperWidth(width: PaperWidth) {
        viewModelScope.launch { printers.setPaperWidth(width) }
    }

    fun setAutoPrintEnabled(enabled: Boolean) {
        viewModelScope.launch { printers.setAutoPrintEnabled(enabled) }
    }

    fun dismissMessage() {
        message.value = null
    }
}
