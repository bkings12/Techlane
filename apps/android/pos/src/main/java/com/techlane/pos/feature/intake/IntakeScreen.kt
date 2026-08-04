package com.techlane.pos.feature.intake

import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Check
import androidx.compose.material.icons.outlined.PersonSearch
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
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
import com.techlane.pos.core.designsystem.component.TlDivider
import com.techlane.pos.core.designsystem.component.TlKeyValue
import com.techlane.pos.core.designsystem.component.TlPhoneField
import com.techlane.pos.core.designsystem.component.TlScreen
import com.techlane.pos.core.designsystem.component.TlSectionHeader
import com.techlane.pos.core.designsystem.component.TlTextButton
import com.techlane.pos.core.designsystem.component.TlTextField
import com.techlane.pos.core.designsystem.component.TlTone
import com.techlane.pos.core.designsystem.theme.PillShape
import com.techlane.pos.core.designsystem.theme.TlTheme
import com.techlane.pos.domain.model.CommonIssues
import com.techlane.pos.domain.model.DeviceKind
import com.techlane.pos.domain.model.IntakeAccessory
import com.techlane.pos.domain.model.PromiseOption
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/**
 * Booking a walk-in, in one screen.
 *
 * Ordered the way the counter actually works: the phone number first (which is
 * how a returning customer is recognised without re-typing anything), then the
 * device, then what is wrong with it. Everything below that is optional and can
 * be filled in from Job Details later — a queue at the counter is the enemy.
 */
@Composable
fun IntakeScreen(
    onBack: () -> Unit,
    onOpenJob: (String) -> Unit,
    modifier: Modifier = Modifier,
    viewModel: IntakeViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val technicians by viewModel.technicians.collectAsStateWithLifecycle()

    state.success?.let { success ->
        IntakeSuccessSheet(
            success = success,
            onPrint = viewModel::printReceipt,
            onOpenJob = { onOpenJob(success.jobId) },
            onNewIntake = viewModel::startAnother,
            onDismiss = { onOpenJob(success.jobId) },
        )
        return
    }

    TlScreen(
        title = "New intake",
        subtitle = "Book a device in",
        modifier = modifier,
        onBack = onBack,
        // Sticky: the action stays reachable no matter how long the form gets
        // or whether the keyboard is up.
        footer = {
            TlBanner(message = state.error, tone = TlTone.Danger)
            TlBanner(
                message = state.validationHint.takeIf { state.error == null },
                tone = TlTone.Warning,
            )
            // A compact summary instead of a separate review page.
            if (state.canSave) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm),
                ) {
                    Text(
                        "${state.summaryCustomer} · ${state.summaryDevice}" +
                            (state.promise?.let { " · ${it.label}" } ?: ""),
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
            TlButton(
                text = if (state.saving) "Booking…" else "Create job",
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
                TlPhoneField(
                    value = state.customerPhone,
                    onValueChange = viewModel::setCustomerPhone,
                    label = "Phone number",
                    imeAction = ImeAction.Next,
                    helper = "Finds a returning customer as you type",
                )

                state.matchedCustomer?.let { matched ->
                    MatchedCustomerRow(name = matched.fullName, onClear = viewModel::clearMatchedCustomer)
                }

                if (state.matchedCustomer == null) {
                    if (state.searching) {
                        Row(
                            horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            CircularProgressIndicator(modifier = Modifier.size(14.dp), strokeWidth = 2.dp)
                            Text(
                                "Looking up…",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                        }
                    }
                    state.suggestions.take(3).forEach { suggestion ->
                        SuggestionRow(
                            name = suggestion.fullName,
                            phone = suggestion.phone,
                            onClick = { viewModel.selectCustomer(suggestion) },
                        )
                    }
                    // Name only appears once we know there is no existing match
                    // to pick — a returning customer never re-types it.
                    if (state.suggestions.isEmpty() && state.customerPhone.isNotBlank()) {
                        TlTextField(
                            value = state.customerName,
                            onValueChange = viewModel::setCustomerName,
                            label = "Name",
                            placeholder = "Who is dropping it off?",
                        )
                        TlTextField(
                            value = state.customerEmail,
                            onValueChange = viewModel::setCustomerEmail,
                            label = "Email (optional)",
                            keyboardType = KeyboardType.Email,
                        )
                    }
                }
            }
        }

        TlSectionHeader(title = "Device")
        TlCard {
            ChipRow(
                items = DeviceKind.entries.map { it.label },
                selected = state.deviceKind.label,
                onSelect = { label ->
                    DeviceKind.entries.firstOrNull { it.label == label }?.let(viewModel::setDeviceKind)
                },
            )
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
                label = "${state.deviceKind.identifierLabel} (optional)",
                placeholder = "Makes the device unmistakable",
            )
        }

        TlSectionHeader(title = "Fault")
        TlCard {
            ChipRow(
                items = CommonIssues.forKind(state.deviceKind),
                selected = null,
                onSelect = viewModel::toggleIssue,
            )
            TlTextField(
                value = state.problem,
                onValueChange = viewModel::setProblem,
                label = "What's wrong with the device?",
                placeholder = "Tap a chip above, or describe it",
                singleLine = false,
                modifier = Modifier.heightIn(min = 88.dp),
            )
        }

        TlSectionHeader(title = "Received with it", subtitle = "Optional")
        TlCard {
            ChipRow(
                items = IntakeAccessory.ALL.map { it.label },
                selectedSet = state.accessories.map { it.label }.toSet(),
                onSelect = { label ->
                    IntakeAccessory.ALL.firstOrNull { it.label == label }?.let(viewModel::toggleAccessory)
                },
            )
            TlTextField(
                value = state.conditionNote,
                onValueChange = viewModel::setConditionNote,
                label = "Condition note (optional)",
                placeholder = "Scratched back, cracked corner…",
            )
        }

        TlSectionHeader(title = "Promise", subtitle = "Optional")
        TlCard {
            ChipRow(
                items = PromiseOption.QUICK.map { it.label },
                selected = state.promise?.label,
                onSelect = { label ->
                    val option = PromiseOption.QUICK.firstOrNull { it.label == label }
                    // Tapping the active choice clears it — nothing was promised.
                    viewModel.setPromise(if (state.promise?.label == label) null else option)
                },
            )
            state.promise?.at()?.let { at ->
                TlKeyValue("Ready by", promiseFormat.format(Date(at)), emphasise = true)
            }
        }

        TlSectionHeader(title = "Optional", subtitle = "All of this can be set later from the job")
        TlCard {
            TlTextField(
                value = state.estimateLabour,
                onValueChange = viewModel::setEstimateLabour,
                label = "Rough estimate (KES)",
                placeholder = "Leave blank until diagnosed",
                keyboardType = KeyboardType.Number,
                helper = "Entering a price authorises the work",
            )
            if (technicians.isNotEmpty()) {
                TlDivider()
                Text(
                    "Assign a technician",
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                ChipRow(
                    items = listOf(UNASSIGNED) + technicians.map { it.displayName },
                    selected = technicians.firstOrNull { it.id == state.technicianId }?.displayName
                        ?: UNASSIGNED,
                    onSelect = { name ->
                        viewModel.setTechnician(technicians.firstOrNull { it.displayName == name }?.id)
                    },
                )
            }
        }
    }
}

private const val UNASSIGNED = "Unassigned"

private val promiseFormat = SimpleDateFormat("EEE d MMM · h:mm a", Locale.getDefault())

@Composable
private fun MatchedCustomerRow(name: String, onClear: () -> Unit) {
    Surface(
        shape = MaterialTheme.shapes.small,
        color = MaterialTheme.colorScheme.primaryContainer,
        modifier = Modifier.fillMaxWidth(),
    ) {
        Row(
            modifier = Modifier.padding(TlTheme.spacing.md),
            horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(
                Icons.Outlined.Check,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onPrimaryContainer,
                modifier = Modifier.size(TlTheme.sizes.iconSm),
            )
            Text(
                name,
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.onPrimaryContainer,
                modifier = Modifier.weight(1f),
            )
            TlTextButton(text = "Change", onClick = onClear)
        }
    }
}

@Composable
private fun SuggestionRow(name: String, phone: String?, onClick: () -> Unit) {
    Surface(
        onClick = onClick,
        shape = MaterialTheme.shapes.small,
        color = MaterialTheme.colorScheme.surfaceVariant,
        modifier = Modifier.fillMaxWidth(),
    ) {
        Row(
            modifier = Modifier.padding(TlTheme.spacing.md),
            horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(
                Icons.Outlined.PersonSearch,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.primary,
                modifier = Modifier.size(TlTheme.sizes.iconSm),
            )
            Column(modifier = Modifier.weight(1f)) {
                Text(name, style = MaterialTheme.typography.titleSmall)
                phone?.let {
                    Text(
                        it,
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
            Text("Use", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.primary)
        }
    }
}

/** Single- or multi-select chip row. Horizontal scroll keeps it to one line. */
@Composable
private fun ChipRow(
    items: List<String>,
    onSelect: (String) -> Unit,
    selected: String? = null,
    selectedSet: Set<String> = emptySet(),
) {
    Row(
        modifier = Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()),
        horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm),
    ) {
        items.forEach { item ->
            val isOn = item == selected || item in selectedSet
            Surface(
                onClick = { onSelect(item) },
                shape = PillShape,
                color = if (isOn) {
                    MaterialTheme.colorScheme.primaryContainer
                } else {
                    MaterialTheme.colorScheme.surface
                },
                border = if (isOn) null else androidx.compose.foundation.BorderStroke(1.dp, TlTheme.colors.hairline),
            ) {
                Text(
                    item,
                    style = MaterialTheme.typography.labelMedium,
                    color = if (isOn) {
                        MaterialTheme.colorScheme.onPrimaryContainer
                    } else {
                        MaterialTheme.colorScheme.onSurfaceVariant
                    },
                    modifier = Modifier.padding(horizontal = TlTheme.spacing.md, vertical = TlTheme.spacing.sm),
                )
            }
        }
    }
}

