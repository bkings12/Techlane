package com.techlane.pos.feature.sales

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ReceiptLong
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.techlane.pos.core.designsystem.component.TlBanner
import com.techlane.pos.core.designsystem.component.TlCard
import com.techlane.pos.core.designsystem.component.TlEmptyState
import com.techlane.pos.core.designsystem.component.TlScreen
import com.techlane.pos.core.designsystem.component.TlStatusPill
import com.techlane.pos.core.designsystem.component.TlTextField
import com.techlane.pos.core.designsystem.component.TlTone
import com.techlane.pos.core.designsystem.theme.TlTheme
import com.techlane.pos.core.util.formatKes
import com.techlane.pos.domain.model.SaleSummary
import com.techlane.pos.domain.model.paymentMethodLabel
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/**
 * Server-authoritative sales history (GET /sales) — distinct from the local,
 * per-device Activity ledger. Every row opens Sale Details; this is the
 * primary "tap a sale to see it" surface the Sales module was missing.
 */
@Composable
fun SalesHistoryScreen(
    onBack: () -> Unit,
    onOpenSale: (String) -> Unit,
    modifier: Modifier = Modifier,
    viewModel: SalesHistoryViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val sales by viewModel.sales.collectAsStateWithLifecycle()

    TlScreen(
        title = "Sales",
        subtitle = "${sales.size} ${if (sales.size == 1) "sale" else "sales"}",
        modifier = modifier,
        onBack = onBack,
        onRefresh = viewModel::refresh,
        refreshing = state.refreshing,
    ) {
        TlTextField(
            value = state.query,
            onValueChange = viewModel::setQuery,
            label = "Search",
            placeholder = "Receipt, customer, phone, M-Pesa ref, product",
            leadingIcon = Icons.Outlined.Search,
            showClear = true,
        )

        Row(
            modifier = Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()),
            horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm),
        ) {
            Chip("Today", state.dateFilter == DateQuickFilter.Today) { viewModel.setDateFilter(DateQuickFilter.Today) }
            Chip("This week", state.dateFilter == DateQuickFilter.ThisWeek) { viewModel.setDateFilter(DateQuickFilter.ThisWeek) }
            Chip("M-Pesa", state.method == "mpesa_stk") { viewModel.setMethod("mpesa_stk") }
            Chip("Cash", state.method == "cash") { viewModel.setMethod("cash") }
            Chip("Paybill", state.method == "mpesa_c2b") { viewModel.setMethod("mpesa_c2b") }
            Chip("Completed", state.status == "completed") { viewModel.setStatus("completed") }
            Chip("Refunded", state.status == "reversed") { viewModel.setStatus("reversed") }
        }

        TlBanner(message = state.error, tone = TlTone.Warning)

        when {
            state.loading && sales.isEmpty() -> Unit
            sales.isEmpty() -> TlEmptyState(
                title = "No sales found",
                subtitle = if (state.query.isBlank()) "Sales will show up here once you take a payment." else "Nothing matches \"${state.query}\".",
                icon = Icons.AutoMirrored.Outlined.ReceiptLong,
            )
            else -> sales.forEach { sale -> SaleRow(sale = sale, onClick = { onOpenSale(sale.id) }) }
        }
    }
}

@Composable
private fun Chip(label: String, selected: Boolean, onClick: () -> Unit) {
    Surface(
        onClick = onClick,
        shape = RoundedCornerShape(10.dp),
        color = if (selected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.surface,
        border = if (selected) null else BorderStroke(1.dp, TlTheme.colors.hairline),
    ) {
        Text(
            label,
            style = MaterialTheme.typography.labelMedium,
            color = if (selected) MaterialTheme.colorScheme.onPrimary else MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.padding(horizontal = TlTheme.spacing.md, vertical = 10.dp),
        )
    }
}

/** Compact row — amount, customer, method, item summary, time, status. No giant cards. */
@Composable
private fun SaleRow(sale: SaleSummary, onClick: () -> Unit) {
    TlCard(onClick = onClick) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.Top,
            horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.md),
        ) {
            Column(modifier = Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.xxs)) {
                Text(formatKes(sale.total), style = MaterialTheme.typography.titleSmall)
                Text(
                    sale.customerName ?: "Walk-in",
                    style = MaterialTheme.typography.bodyMedium,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                Text(
                    listOfNotNull(
                        paymentMethodLabel(sale.paymentMethod),
                        sale.itemSummary?.let { if (sale.itemCount > 1) "$it +${sale.itemCount - 1} more" else it },
                    ).joinToString(" · "),
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
            Column(horizontalAlignment = Alignment.End, verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.xs)) {
                Text(
                    sale.createdAt?.let { timeFormat.format(Date(it)) } ?: "—",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                TlStatusPill(
                    text = if (sale.isCompleted) "Completed" else if (sale.isReversed) "Refunded" else sale.status.replaceFirstChar(Char::uppercase),
                    tone = if (sale.isCompleted) TlTone.Success else if (sale.isReversed) TlTone.Danger else TlTone.Neutral,
                )
            }
        }
    }
}

private val timeFormat = SimpleDateFormat("d MMM, HH:mm", Locale.getDefault())
