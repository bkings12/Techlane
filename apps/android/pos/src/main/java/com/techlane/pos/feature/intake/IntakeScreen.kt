package com.techlane.pos.feature.intake

import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.techlane.pos.core.designsystem.component.TlBanner
import com.techlane.pos.core.designsystem.component.TlButton
import com.techlane.pos.core.designsystem.component.TlCard
import com.techlane.pos.core.designsystem.component.TlPhoneField
import com.techlane.pos.core.designsystem.component.TlScreen
import com.techlane.pos.core.designsystem.component.TlSectionHeader
import com.techlane.pos.core.designsystem.component.TlTextField
import com.techlane.pos.core.designsystem.component.TlTone
import com.techlane.pos.core.designsystem.theme.PillShape
import com.techlane.pos.core.designsystem.theme.TlTheme
import com.techlane.pos.domain.model.DeviceKind

/**
 * Booking a walk-in. The Save button lives in the footer bar so it is reachable
 * the moment the form is valid, without scrolling past the optional fields —
 * a counter with a queue should never have to hunt for it.
 */
@Composable
fun IntakeScreen(
    onBack: () -> Unit,
    onJobCreated: (String) -> Unit,
    modifier: Modifier = Modifier,
    viewModel: IntakeViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val technicians by viewModel.technicians.collectAsStateWithLifecycle()

    // Navigation is driven off the created id rather than the button click, so
    // a job is never "left behind" if the screen recomposes mid-save.
    LaunchedEffect(state.createdJobId) {
        state.createdJobId?.let(onJobCreated)
    }

    TlScreen(
        title = "New intake",
        subtitle = "Book a device in",
        modifier = modifier,
        onBack = onBack,
        footer = {
            TlBanner(message = state.error, tone = TlTone.Danger)
            TlBanner(
                message = state.validationHint.takeIf { state.error == null },
                tone = TlTone.Warning,
            )
            TlButton(
                text = if (state.saving) "Booking…" else "Book the job",
                onClick = viewModel::save,
                enabled = state.canSave,
                loading = state.saving,
                large = true,
                modifier = Modifier.fillMaxWidth(),
            )
        },
    ) {
        TlSectionHeader(title = "Customer")
        TlCard {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text("Walk-in (no details)", style = MaterialTheme.typography.titleSmall)
                Switch(checked = state.anonymous, onCheckedChange = viewModel::setAnonymous)
            }
            if (state.anonymous) {
                Text(
                    "The pickup code on the intake slip will be the only way to claim this device.",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            } else {
                TlTextField(
                    value = state.customerName,
                    onValueChange = viewModel::setCustomerName,
                    label = "Name",
                    placeholder = "Who is dropping it off?",
                )
                TlPhoneField(
                    value = state.customerPhone,
                    onValueChange = viewModel::setCustomerPhone,
                    imeAction = ImeAction.Next,
                )
            }
        }

        TlSectionHeader(title = "Device")
        TlCard {
            DeviceKindRow(current = state.deviceKind, onSelect = viewModel::setDeviceKind)
            TlTextField(
                value = state.brand,
                onValueChange = viewModel::setBrand,
                label = "Brand",
                placeholder = "Samsung, Apple, HP…",
            )
            TlTextField(
                value = state.model,
                onValueChange = viewModel::setModel,
                label = "Model",
                placeholder = "A2337, Galaxy S21…",
            )
            TlTextField(
                value = state.identifier,
                onValueChange = viewModel::setIdentifier,
                label = state.deviceKind.identifierLabel,
                placeholder = "Optional, but makes the device unmistakable",
            )
        }

        TlSectionHeader(title = "Fault")
        TlCard {
            TlTextField(
                value = state.problem,
                onValueChange = viewModel::setProblem,
                label = "What is wrong?",
                placeholder = "Cracked screen, won't charge, no display…",
                singleLine = false,
                modifier = Modifier.heightIn(min = 100.dp),
            )
        }

        TlSectionHeader(title = "Optional", subtitle = "Can all be set later from the job")
        TlCard {
            TlTextField(
                value = state.estimateLabour,
                onValueChange = viewModel::setEstimateLabour,
                label = "Rough labour estimate (KES)",
                placeholder = "Leave blank until diagnosed",
                keyboardType = KeyboardType.Number,
            )
            if (technicians.isNotEmpty()) {
                Text(
                    "Assign a technician",
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                Row(
                    modifier = Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()),
                    horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm),
                ) {
                    ChoicePill(
                        label = "Unassigned",
                        selected = state.technicianId == null,
                        onClick = { viewModel.setTechnician(null) },
                    )
                    technicians.forEach { technician ->
                        ChoicePill(
                            label = technician.displayName,
                            selected = state.technicianId == technician.id,
                            onClick = { viewModel.setTechnician(technician.id) },
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun DeviceKindRow(current: DeviceKind, onSelect: (DeviceKind) -> Unit) {
    Row(
        modifier = Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()),
        horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm),
    ) {
        DeviceKind.entries.forEach { kind ->
            ChoicePill(label = kind.label, selected = kind == current, onClick = { onSelect(kind) })
        }
    }
}

@Composable
private fun ChoicePill(label: String, selected: Boolean, onClick: () -> Unit) {
    Surface(
        onClick = onClick,
        shape = PillShape,
        color = if (selected) MaterialTheme.colorScheme.primaryContainer else MaterialTheme.colorScheme.surface,
        border = if (selected) null else androidx.compose.foundation.BorderStroke(1.dp, TlTheme.colors.hairline),
    ) {
        Text(
            label,
            style = MaterialTheme.typography.labelMedium,
            color = if (selected) {
                MaterialTheme.colorScheme.onPrimaryContainer
            } else {
                MaterialTheme.colorScheme.onSurfaceVariant
            },
            modifier = Modifier.padding(horizontal = TlTheme.spacing.md, vertical = TlTheme.spacing.sm),
        )
    }
}
