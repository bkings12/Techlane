package com.techlane.pos.feature.settings.printer

import android.bluetooth.BluetoothAdapter
import android.content.Intent
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Bluetooth
import androidx.compose.material.icons.outlined.Print
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Switch
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.techlane.pos.core.designsystem.component.TlBanner
import com.techlane.pos.core.designsystem.component.TlButton
import com.techlane.pos.core.designsystem.component.TlCard
import com.techlane.pos.core.designsystem.component.TlDangerButton
import com.techlane.pos.core.designsystem.component.TlDivider
import com.techlane.pos.core.designsystem.component.TlEmptyState
import com.techlane.pos.core.designsystem.component.TlScreen
import com.techlane.pos.core.designsystem.component.TlSecondaryButton
import com.techlane.pos.core.designsystem.component.TlStatusPill
import com.techlane.pos.core.designsystem.component.TlTone
import com.techlane.pos.core.designsystem.theme.PillShape
import com.techlane.pos.core.designsystem.theme.TlTheme
import com.techlane.pos.data.printer.PrinterConnectionState
import com.techlane.pos.data.printer.PrinterDevice
import com.techlane.pos.data.printer.errorMessage
import com.techlane.pos.data.printer.isConnected
import com.techlane.pos.data.printer.statusLabel

/**
 * Printer Settings — a page under Settings, not a feature of it. Everything
 * this screen does routes through [com.techlane.pos.data.printer.PrinterRepository];
 * a future payment screen reusing "Test Print"'s connect-and-write path calls
 * the same repository and needs none of what's on this screen.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PrinterSettingsScreen(
    onBack: () -> Unit,
    modifier: Modifier = Modifier,
    viewModel: PrinterSettingsViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val context = LocalContext.current

    val permissionLauncher = rememberLauncherForActivityResult(
        ActivityResultContracts.RequestMultiplePermissions(),
    ) { viewModel.retryAfterResolving(PickerBlockReason.PermissionRequired) }

    val enableBluetoothLauncher = rememberLauncherForActivityResult(
        ActivityResultContracts.StartActivityForResult(),
    ) { viewModel.retryAfterResolving(PickerBlockReason.BluetoothDisabled) }

    TlScreen(title = "Printer", onBack = onBack, modifier = modifier) {
        TlBanner(message = state.message, tone = TlTone.Success)
        TlBanner(message = state.connection.errorMessage, tone = TlTone.Danger)

        val device = state.preferences.device
        if (device == null) {
            TlCard {
                TlEmptyState(
                    title = "No printer configured",
                    subtitle = "Connect a Bluetooth thermal printer to print receipts directly from TechLane.",
                    icon = Icons.Outlined.Print,
                    action = {
                        TlButton(
                            text = "Select printer",
                            onClick = viewModel::openPicker,
                            modifier = Modifier.fillMaxWidth(),
                        )
                    },
                )
            }
        } else {
            TlCard {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Column {
                        Text(device.displayName, style = MaterialTheme.typography.titleMedium)
                        Text(
                            "${state.preferences.paperWidth.label} Bluetooth thermal printer",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                    Icon(
                        Icons.Outlined.Bluetooth,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.primary,
                    )
                }

                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(
                        "Status",
                        style = MaterialTheme.typography.labelMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                    ConnectionStatusPill(state.connection)
                }

                Row(horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm), modifier = Modifier.fillMaxWidth()) {
                    TlSecondaryButton(
                        text = "Select printer",
                        onClick = viewModel::openPicker,
                        modifier = Modifier.weight(1f),
                    )
                    TlButton(
                        text = if (state.connection is PrinterConnectionState.Printing) "Printing…" else "Test print",
                        onClick = viewModel::testPrint,
                        enabled = !state.isBusy,
                        loading = state.connection is PrinterConnectionState.Printing,
                        icon = Icons.Outlined.Print,
                        modifier = Modifier.weight(1f),
                    )
                }

                TlDivider()

                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Column(modifier = Modifier.weight(1f)) {
                        Text("Auto-print receipts", style = MaterialTheme.typography.bodyMedium)
                        Text(
                            "Not used yet — reserved for checkout.",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                    Switch(
                        checked = state.preferences.autoPrintEnabled,
                        onCheckedChange = viewModel::setAutoPrintEnabled,
                    )
                }

                TlDangerButton(
                    text = "Forget this printer",
                    onClick = viewModel::forgetPrinter,
                    modifier = Modifier.fillMaxWidth(),
                )
            }
        }
    }

    val picker = state.picker
    if (picker is DevicePickerState.Open) {
        LaunchedEffect(picker.blockedReason) {
            when (picker.blockedReason) {
                PickerBlockReason.PermissionRequired -> {
                    val permissions = viewModel.requiredPermissions()
                    if (permissions.isNotEmpty()) permissionLauncher.launch(permissions)
                }

                PickerBlockReason.BluetoothDisabled ->
                    enableBluetoothLauncher.launch(Intent(BluetoothAdapter.ACTION_REQUEST_ENABLE))

                PickerBlockReason.Unsupported, null -> Unit
            }
        }

        DevicePickerSheet(
            picker = picker,
            onSelect = viewModel::selectDevice,
            onDismiss = viewModel::closePicker,
        )
    }
}

@Composable
private fun ConnectionStatusPill(connection: PrinterConnectionState) {
    val tone = when (connection) {
        is PrinterConnectionState.Connected, is PrinterConnectionState.PrintSuccess -> TlTone.Success
        is PrinterConnectionState.ConnectionFailed, is PrinterConnectionState.PrintFailed -> TlTone.Danger
        PrinterConnectionState.Connecting, PrinterConnectionState.Printing -> TlTone.Info
        PrinterConnectionState.BluetoothDisabled, PrinterConnectionState.PermissionRequired -> TlTone.Warning
        PrinterConnectionState.NotConfigured, PrinterConnectionState.Idle -> TlTone.Neutral
    }
    TlStatusPill(text = connection.statusLabel(), tone = tone)
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun DevicePickerSheet(
    picker: DevicePickerState.Open,
    onSelect: (PrinterDevice) -> Unit,
    onDismiss: () -> Unit,
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        containerColor = MaterialTheme.colorScheme.surface,
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = TlTheme.spacing.xl)
                .padding(bottom = TlTheme.spacing.xxl),
            verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.md),
        ) {
            Text("Select printer", style = MaterialTheme.typography.titleLarge)

            when (picker.blockedReason) {
                PickerBlockReason.Unsupported -> Text(
                    "This device doesn't support Bluetooth, so it can't print over it.",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )

                PickerBlockReason.PermissionRequired -> Text(
                    "Waiting for Bluetooth permission…",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )

                PickerBlockReason.BluetoothDisabled -> Text(
                    "Waiting for Bluetooth to turn on…",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )

                null -> {
                    Text(
                        "Pair the printer in Android's Bluetooth settings first if it isn't listed below.",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                    if (picker.devices.isEmpty()) {
                        if (picker.scanning) {
                            Row(
                                horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm),
                                verticalAlignment = Alignment.CenterVertically,
                                modifier = Modifier.padding(vertical = TlTheme.spacing.lg),
                            ) {
                                CircularProgressIndicator(modifier = Modifier.size(18.dp), strokeWidth = 2.dp)
                                Text(
                                    "Looking for devices…",
                                    style = MaterialTheme.typography.bodyMedium,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                )
                            }
                        } else {
                            Text(
                                "No devices found yet.",
                                style = MaterialTheme.typography.bodyMedium,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                                modifier = Modifier.padding(vertical = TlTheme.spacing.lg),
                            )
                        }
                    } else {
                        LazyColumn(
                            modifier = Modifier.heightIn(max = 360.dp),
                            verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.xs),
                        ) {
                            items(picker.devices, key = { it.address }) { device ->
                                DeviceRow(device = device, onClick = { onSelect(device) })
                                TlDivider()
                            }
                        }
                        if (picker.scanning) {
                            Row(
                                horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm),
                                verticalAlignment = Alignment.CenterVertically,
                            ) {
                                CircularProgressIndicator(modifier = Modifier.size(14.dp), strokeWidth = 2.dp)
                                Text(
                                    "Still scanning…",
                                    style = MaterialTheme.typography.labelSmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun DeviceRow(device: PrinterDevice, onClick: () -> Unit) {
    Surface(onClick = onClick, modifier = Modifier.fillMaxWidth(), color = MaterialTheme.colorScheme.surface) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(vertical = TlTheme.spacing.md),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.md),
        ) {
            Surface(
                shape = PillShape,
                color = MaterialTheme.colorScheme.primary.copy(alpha = 0.12f),
                modifier = Modifier.size(36.dp),
            ) {
                Column(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalAlignment = Alignment.CenterHorizontally,
                ) {
                    Icon(
                        Icons.Outlined.Print,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.primary,
                        modifier = Modifier.padding(top = 9.dp).size(18.dp),
                    )
                }
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    device.displayName,
                    style = MaterialTheme.typography.titleSmall,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                Text(
                    device.address,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            if (device.bonded) {
                TlStatusPill(text = "Paired", tone = TlTone.Neutral, leadingDot = false)
            }
        }
    }
}
