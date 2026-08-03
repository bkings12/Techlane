package com.techlane.pos.feature.jobs.components

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.techlane.pos.core.designsystem.component.TlBanner
import com.techlane.pos.core.designsystem.component.TlButton
import com.techlane.pos.core.designsystem.component.TlDivider
import com.techlane.pos.core.designsystem.component.TlSecondaryButton
import com.techlane.pos.core.designsystem.component.TlStepper
import com.techlane.pos.core.designsystem.component.TlTextField
import com.techlane.pos.core.designsystem.component.TlTone
import com.techlane.pos.core.designsystem.theme.TlTheme
import com.techlane.pos.core.util.formatKes
import com.techlane.pos.data.local.CatalogItemEntity
import com.techlane.pos.domain.model.CustomerUpdateContext
import com.techlane.pos.domain.model.CustomerUpdateTemplate
import com.techlane.pos.domain.model.PhotoKind
import com.techlane.pos.domain.model.VerbalApprovalChannel

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun JobSheetScaffold(
    title: String,
    subtitle: String? = null,
    onDismiss: () -> Unit,
    content: @Composable androidx.compose.foundation.layout.ColumnScope.() -> Unit,
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
            Text(title, style = MaterialTheme.typography.titleLarge)
            if (subtitle != null) {
                Text(
                    subtitle,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            content()
        }
    }
}

/** Multiline diagnosis entry. Saves locally first, so it survives no signal. */
@Composable
fun DiagnosisSheet(
    existing: String?,
    onSave: (String) -> Unit,
    onMarkInconclusive: () -> Unit,
    onDismiss: () -> Unit,
) {
    var text by remember { mutableStateOf(existing.orEmpty()) }
    JobSheetScaffold(
        title = if (existing.isNullOrBlank()) "Add diagnosis" else "Update diagnosis",
        subtitle = "Saved on this phone straight away, then synced.",
        onDismiss = onDismiss,
    ) {
        TlTextField(
            value = text,
            onValueChange = { text = it },
            label = "Findings",
            placeholder = "What did you find? What needs doing?",
            singleLine = false,
            modifier = Modifier.heightIn(min = 120.dp),
        )
        TlButton(
            text = "Save diagnosis",
            onClick = { onSave(text) },
            enabled = text.isNotBlank(),
            modifier = Modifier.fillMaxWidth(),
        )
        TlSecondaryButton(
            text = "Mark inconclusive",
            onClick = onMarkInconclusive,
            modifier = Modifier.fillMaxWidth(),
        )
    }
}

@Composable
fun EstimateSheet(suggested: Double?, onSend: (Double, String?) -> Unit, onDismiss: () -> Unit) {
    var amount by remember { mutableStateOf(suggested?.takeIf { it > 0 }?.toLong()?.toString().orEmpty()) }
    var notes by remember { mutableStateOf("") }
    JobSheetScaffold(
        title = "Send estimate",
        subtitle = "The customer approves this before repair work can begin.",
        onDismiss = onDismiss,
    ) {
        TlTextField(
            value = amount,
            onValueChange = { amount = it.filter(Char::isDigit) },
            label = "Total (KES)",
            placeholder = "0",
            keyboardType = KeyboardType.Number,
        )
        TlTextField(
            value = notes,
            onValueChange = { notes = it },
            label = "What's included (optional)",
            singleLine = false,
        )
        TlButton(
            text = "Send estimate",
            onClick = { amount.toDoubleOrNull()?.let { onSend(it, notes.takeIf(String::isNotBlank)) } },
            enabled = (amount.toDoubleOrNull() ?: 0.0) > 0,
            modifier = Modifier.fillMaxWidth(),
        )
    }
}

/**
 * Records an approval taken outside the estimate flow. The channel is required
 * because "the customer said yes" with no record of how is exactly the gap this
 * whole authorization rule exists to close.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun RecordApprovalSheet(
    suggestedAmount: Double?,
    onRecord: (VerbalApprovalChannel, Double?, String?) -> Unit,
    onDismiss: () -> Unit,
) {
    var channel by remember { mutableStateOf(VerbalApprovalChannel.AtCounter) }
    var amount by remember { mutableStateOf(suggestedAmount?.takeIf { it > 0 }?.toLong()?.toString().orEmpty()) }
    var detail by remember { mutableStateOf("") }

    JobSheetScaffold(
        title = "Record customer approval",
        subtitle = "This unlocks bench work, so it is written to the job's audit trail.",
        onDismiss = onDismiss,
    ) {
        Text(
            "How was it given?",
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        FlowRow(horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm)) {
            VerbalApprovalChannel.entries.forEach { option ->
                FilterChip(
                    selected = channel == option,
                    onClick = { channel = option },
                    label = { Text(option.label) },
                )
            }
        }
        TlTextField(
            value = amount,
            onValueChange = { amount = it.filter(Char::isDigit) },
            label = "Agreed price (KES)",
            placeholder = "0",
            keyboardType = KeyboardType.Number,
            helper = "Leave blank if the price was not fixed.",
        )
        TlTextField(
            value = detail,
            onValueChange = { detail = it },
            label = "Who agreed, and anything said (optional)",
            singleLine = false,
        )
        TlButton(
            text = "Record approval",
            onClick = { onRecord(channel, amount.toDoubleOrNull(), detail.takeIf(String::isNotBlank)) },
            modifier = Modifier.fillMaxWidth(),
        )
    }
}

/** Inventory search for parts. Reads the same cached catalog the till uses. */
@Composable
fun PartsPickerSheet(
    results: List<CatalogItemEntity>,
    query: String,
    onQueryChange: (String) -> Unit,
    onAdd: (CatalogItemEntity, Int) -> Unit,
    onDismiss: () -> Unit,
) {
    var selected by remember { mutableStateOf<CatalogItemEntity?>(null) }
    var quantity by remember { mutableIntStateOf(1) }

    JobSheetScaffold(title = "Add a part", onDismiss = onDismiss) {
        val chosen = selected
        if (chosen == null) {
            TlTextField(
                value = query,
                onValueChange = onQueryChange,
                label = "Search inventory",
                placeholder = "Name, brand or SKU",
                leadingIcon = Icons.Outlined.Search,
                showClear = true,
            )
            LazyColumn(
                modifier = Modifier.heightIn(max = 360.dp),
                verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.xs),
            ) {
                items(results, key = { it.variantId }) { item ->
                    Surface(
                        onClick = { selected = item },
                        modifier = Modifier.fillMaxWidth(),
                        color = MaterialTheme.colorScheme.surface,
                    ) {
                        Row(
                            modifier = Modifier.fillMaxWidth().padding(vertical = TlTheme.spacing.md),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.md),
                        ) {
                            Column(modifier = Modifier.weight(1f)) {
                                Text(
                                    item.productName,
                                    style = MaterialTheme.typography.titleSmall,
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis,
                                )
                                Text(
                                    listOfNotNull(item.sku.takeIf(String::isNotBlank), "${item.availableQty} in stock")
                                        .joinToString(" · "),
                                    style = MaterialTheme.typography.bodySmall,
                                    color = if (item.availableQty <= 0) {
                                        MaterialTheme.colorScheme.error
                                    } else {
                                        MaterialTheme.colorScheme.onSurfaceVariant
                                    },
                                )
                            }
                            Text(formatKes(item.sellPrice), style = MaterialTheme.typography.titleSmall)
                        }
                    }
                    TlDivider()
                }
                if (results.isEmpty()) {
                    item {
                        Text(
                            if (query.isBlank()) {
                                "No stock cached. Sync the catalog from Settings."
                            } else {
                                "Nothing matches \"$query\". If it isn't in stock, use Part required."
                            },
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(vertical = TlTheme.spacing.lg),
                        )
                    }
                }
            }
        } else {
            Text(chosen.productName, style = MaterialTheme.typography.titleMedium)
            Text(
                "${formatKes(chosen.sellPrice)} each · ${chosen.availableQty} in stock",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            if (chosen.availableQty <= 0) {
                TlBanner(
                    message = "This part is not in stock. Adding it will be refused when it syncs.",
                    tone = TlTone.Warning,
                )
            }
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text("Quantity", style = MaterialTheme.typography.titleSmall)
                TlStepper(value = quantity, onValueChange = { quantity = it })
            }
            TlButton(
                text = "Add ${formatKes(chosen.sellPrice * quantity)}",
                onClick = { onAdd(chosen, quantity) },
                modifier = Modifier.fillMaxWidth(),
            )
            TlSecondaryButton(
                text = "Pick a different part",
                onClick = { selected = null },
                modifier = Modifier.fillMaxWidth(),
            )
        }
    }
}

/**
 * Customer update. Templates are a starting point the technician edits — the
 * message goes out through the backend so it lands in the shop's own log.
 */
@Composable
fun CustomerUpdateSheet(
    templates: List<CustomerUpdateTemplate>,
    context: CustomerUpdateContext,
    customerName: String?,
    hasPhone: Boolean,
    onSend: (String) -> Unit,
    onDismiss: () -> Unit,
) {
    var body by remember { mutableStateOf(templates.firstOrNull()?.build?.invoke(context).orEmpty()) }

    JobSheetScaffold(
        title = "Send customer update",
        subtitle = customerName?.let { "To $it" },
        onDismiss = onDismiss,
    ) {
        if (!hasPhone) {
            TlBanner(message = "This customer has no phone number on file.", tone = TlTone.Warning)
        }
        if (templates.size > 1) {
            FlowRowTemplates(templates = templates, onPick = { body = it.build(context) })
        }
        TlTextField(
            value = body,
            onValueChange = { body = it },
            label = "Message",
            singleLine = false,
            modifier = Modifier.heightIn(min = 140.dp),
            helper = "${body.length} characters",
        )
        TlButton(
            text = "Send update",
            onClick = { onSend(body) },
            enabled = hasPhone && body.isNotBlank(),
            modifier = Modifier.fillMaxWidth(),
        )
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun FlowRowTemplates(templates: List<CustomerUpdateTemplate>, onPick: (CustomerUpdateTemplate) -> Unit) {
    var selected by remember { mutableStateOf(templates.firstOrNull()?.id) }
    FlowRow(horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm)) {
        templates.forEach { template ->
            FilterChip(
                selected = selected == template.id,
                onClick = {
                    selected = template.id
                    onPick(template)
                },
                label = { Text(template.label) },
            )
        }
    }
}

/** Asks what the photo is of before opening the camera. One tap, then shoot. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun PhotoKindSheet(
    onPick: (PhotoKind) -> Unit,
    onDismiss: () -> Unit,
) {
    JobSheetScaffold(
        title = "What is this photo of?",
        subtitle = "Photos are grouped by stage so the job history stays readable.",
        onDismiss = onDismiss,
    ) {
        PhotoKind.entries.forEach { kind ->
            TlSecondaryButton(
                text = kind.label,
                onClick = { onPick(kind) },
                modifier = Modifier.fillMaxWidth(),
            )
        }
    }
}
