package com.techlane.pos.feature.charge

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Add
import androidx.compose.material.icons.outlined.Build
import androidx.compose.material.icons.outlined.Inventory2
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.techlane.pos.core.designsystem.component.TlButton
import com.techlane.pos.core.designsystem.component.TlDivider
import com.techlane.pos.core.designsystem.component.TlEmptyState
import com.techlane.pos.core.designsystem.component.TlStatusPill
import com.techlane.pos.core.designsystem.component.TlTextField
import com.techlane.pos.core.designsystem.component.TlTone
import com.techlane.pos.core.designsystem.theme.PillShape
import com.techlane.pos.core.designsystem.theme.TlTheme
import com.techlane.pos.core.util.formatKes
import com.techlane.pos.data.local.CatalogItemEntity
import com.techlane.pos.data.local.ServiceItemEntity

private enum class TargetTab(val label: String) { Product("Product"), Service("Service") }

/**
 * "What's this for?" — deliberately optional. Nothing here is required to send a
 * prompt; picking something only labels the sale and fills in the amount.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ChargeTargetSheet(
    products: List<CatalogItemEntity>,
    services: List<ServiceItemEntity>,
    query: String,
    onQueryChange: (String) -> Unit,
    onSelectProduct: (CatalogItemEntity) -> Unit,
    onSelectService: (ServiceItemEntity) -> Unit,
    onAddService: (String, Double?) -> Unit,
    onDismiss: () -> Unit,
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    var tab by remember { mutableStateOf(TargetTab.Service) }
    var newServiceLabel by remember { mutableStateOf("") }
    var newServicePrice by remember { mutableStateOf("") }
    var addingService by remember { mutableStateOf(false) }

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
            verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.lg),
        ) {
            Text("What's this for?", style = MaterialTheme.typography.titleLarge)
            Text(
                "Optional — it labels the receipt and fills in the price.",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )

            SingleChoiceSegmentedButtonRow(modifier = Modifier.fillMaxWidth()) {
                TargetTab.entries.forEachIndexed { index, entry ->
                    SegmentedButton(
                        selected = tab == entry,
                        onClick = { tab = entry },
                        shape = SegmentedButtonDefaults.itemShape(index, TargetTab.entries.size),
                        label = { Text(entry.label) },
                    )
                }
            }

            when (tab) {
                TargetTab.Product -> {
                    TlTextField(
                        value = query,
                        onValueChange = onQueryChange,
                        label = "Search stock",
                        placeholder = "Name, brand or SKU",
                        leadingIcon = Icons.Outlined.Search,
                        showClear = true,
                    )
                    if (products.isEmpty()) {
                        TlEmptyState(
                            title = if (query.isBlank()) "No stock cached yet" else "Nothing matches \"$query\"",
                            subtitle = if (query.isBlank()) {
                                "Pull the catalog from Settings, or just charge an amount."
                            } else {
                                "Try a different word, or switch to Service."
                            },
                            icon = Icons.Outlined.Inventory2,
                        )
                    } else {
                        LazyColumn(
                            modifier = Modifier.heightIn(max = 380.dp),
                            verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.xs),
                        ) {
                            items(products, key = { it.variantId }) { item ->
                                ProductRow(item = item, onClick = { onSelectProduct(item) })
                                TlDivider()
                            }
                        }
                    }
                }

                TargetTab.Service -> {
                    if (addingService) {
                        TlTextField(
                            value = newServiceLabel,
                            onValueChange = { newServiceLabel = it },
                            label = "Service name",
                            placeholder = "e.g. Motherboard reball",
                        )
                        TlTextField(
                            value = newServicePrice,
                            onValueChange = { newServicePrice = it.filter(Char::isDigit) },
                            label = "Usual price (optional)",
                            placeholder = "0",
                            keyboardType = KeyboardType.Number,
                        )
                        Row(horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm)) {
                            TlButton(
                                text = "Save & use",
                                onClick = {
                                    onAddService(newServiceLabel.trim(), newServicePrice.toDoubleOrNull())
                                    newServiceLabel = ""
                                    newServicePrice = ""
                                    addingService = false
                                },
                                enabled = newServiceLabel.isNotBlank(),
                                modifier = Modifier.weight(1f),
                            )
                        }
                    } else {
                        LazyColumn(
                            modifier = Modifier.heightIn(max = 400.dp),
                            verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.xs),
                        ) {
                            items(services, key = { it.id }) { item ->
                                ServiceRow(item = item, onClick = { onSelectService(item) })
                                TlDivider()
                            }
                            item {
                                Row(
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .padding(vertical = TlTheme.spacing.md),
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.md),
                                ) {
                                    Surface(
                                        onClick = { addingService = true },
                                        shape = MaterialTheme.shapes.small,
                                        color = MaterialTheme.colorScheme.surfaceVariant,
                                        modifier = Modifier.fillMaxWidth(),
                                    ) {
                                        Row(
                                            modifier = Modifier.padding(TlTheme.spacing.lg),
                                            verticalAlignment = Alignment.CenterVertically,
                                            horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm),
                                        ) {
                                            Icon(Icons.Outlined.Add, contentDescription = null)
                                            Text("Add another service", style = MaterialTheme.typography.titleSmall)
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun ProductRow(item: CatalogItemEntity, onClick: () -> Unit) {
    val outOfStock = item.availableQty <= 0
    Surface(onClick = onClick, color = MaterialTheme.colorScheme.surface, modifier = Modifier.fillMaxWidth()) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(vertical = TlTheme.spacing.md),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.md),
        ) {
            Surface(shape = PillShape, color = MaterialTheme.colorScheme.surfaceVariant, modifier = Modifier.size(40.dp)) {
                Box(contentAlignment = Alignment.Center) {
                    Icon(
                        Icons.Outlined.Inventory2,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.size(TlTheme.sizes.iconSm),
                    )
                }
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    item.productName,
                    style = MaterialTheme.typography.titleSmall,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                Text(
                    listOfNotNull(item.brand, item.sku.takeIf { it.isNotBlank() }).joinToString(" · ")
                        .ifBlank { "In stock: ${item.availableQty}" },
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
            Column(horizontalAlignment = Alignment.End) {
                Text(formatKes(item.sellPrice), style = MaterialTheme.typography.titleSmall)
                if (outOfStock) {
                    TlStatusPill(text = "No stock", tone = TlTone.Warning, leadingDot = false)
                }
            }
        }
    }
}

@Composable
private fun ServiceRow(item: ServiceItemEntity, onClick: () -> Unit) {
    Surface(onClick = onClick, color = MaterialTheme.colorScheme.surface, modifier = Modifier.fillMaxWidth()) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(vertical = TlTheme.spacing.md),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.md),
        ) {
            Surface(
                shape = PillShape,
                color = MaterialTheme.colorScheme.primary.copy(alpha = 0.12f),
                modifier = Modifier.size(40.dp),
            ) {
                Box(contentAlignment = Alignment.Center) {
                    Icon(
                        Icons.Outlined.Build,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.primary,
                        modifier = Modifier.size(TlTheme.sizes.iconSm),
                    )
                }
            }
            Text(
                item.label,
                style = MaterialTheme.typography.titleSmall,
                modifier = Modifier.weight(1f),
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            if (item.lastPrice != null && item.lastPrice > 0) {
                Text(
                    formatKes(item.lastPrice),
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }
    }
}
