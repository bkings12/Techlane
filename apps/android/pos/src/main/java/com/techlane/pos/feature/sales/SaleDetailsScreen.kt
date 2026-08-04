package com.techlane.pos.feature.sales

import android.webkit.WebView
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Close
import androidx.compose.material.icons.outlined.Print
import androidx.compose.material.icons.outlined.Settings
import androidx.compose.material.icons.outlined.Share
import androidx.compose.material.icons.outlined.Visibility
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.viewinterop.AndroidView
import androidx.compose.ui.window.Dialog
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.techlane.pos.core.designsystem.component.TlBanner
import com.techlane.pos.core.designsystem.component.TlButton
import com.techlane.pos.core.designsystem.component.TlCard
import com.techlane.pos.core.designsystem.component.TlDivider
import com.techlane.pos.core.designsystem.component.TlKeyValue
import com.techlane.pos.core.designsystem.component.TlScreen
import com.techlane.pos.core.designsystem.component.TlSecondaryButton
import com.techlane.pos.core.designsystem.component.TlStatusPill
import com.techlane.pos.core.designsystem.component.TlTone
import com.techlane.pos.core.designsystem.theme.TlTheme
import com.techlane.pos.core.util.formatKes
import com.techlane.pos.domain.model.SaleDetail
import com.techlane.pos.domain.model.SaleLineItem
import com.techlane.pos.domain.model.paymentMethodLabel
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/**
 * A completed sale, reopened from anywhere (Sales history, Activity, or the
 * Quick Prompt success screen's "View Sale") — loaded fresh from GET
 * sales/{id} every time, so it works regardless of whether the payment
 * screen that created it is still open, per the "receipt availability must
 * not depend on a foreground session" requirement.
 */
@Composable
fun SaleDetailsScreen(
    onBack: () -> Unit,
    onOpenJob: (String) -> Unit,
    onConnectPrinter: () -> Unit = {},
    modifier: Modifier = Modifier,
    viewModel: SaleDetailsViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val context = LocalContext.current
    val sale = state.sale

    TlScreen(
        title = sale?.reference ?: "Sale",
        subtitle = sale?.let { statusLabel(it.status) },
        onBack = onBack,
        modifier = modifier,
        onRefresh = viewModel::load,
        refreshing = state.loading,
    ) {
        TlBanner(message = state.message, tone = if (state.printerDisconnected) TlTone.Warning else TlTone.Success)
        TlBanner(
            message = state.error?.let { if (sale == null) it else null },
            tone = TlTone.Danger,
        )

        when {
            sale == null && state.loading -> Unit
            sale == null -> {
                TlCard {
                    Text("Couldn't load this sale", style = MaterialTheme.typography.titleSmall)
                    state.error?.let {
                        Text(it, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                    TlSecondaryButton(text = "Retry", onClick = viewModel::load, modifier = Modifier.fillMaxWidth())
                }
            }
            else -> SaleDetailsBody(
                sale = sale,
                canSeeCost = state.canSeeCost,
                printing = state.printing,
                sharing = state.sharing,
                printerDisconnected = state.printerDisconnected,
                onOpenJob = onOpenJob,
                onConnectPrinter = onConnectPrinter,
                onViewReceipt = viewModel::viewReceipt,
                onPrintReceipt = viewModel::printReceipt,
                onShareReceipt = { viewModel.shareReceipt(context) },
            )
        }
    }

    state.viewingReceiptHtml?.let { html ->
        ReceiptViewDialog(
            html = html,
            onShare = { viewModel.shareReceiptHtml(context) },
            onDismiss = viewModel::dismissReceiptView,
        )
    }
}

@Composable
private fun SaleDetailsBody(
    sale: SaleDetail,
    canSeeCost: Boolean,
    printing: Boolean,
    sharing: Boolean,
    printerDisconnected: Boolean,
    onOpenJob: (String) -> Unit,
    onConnectPrinter: () -> Unit,
    onViewReceipt: () -> Unit,
    onPrintReceipt: () -> Unit,
    onShareReceipt: () -> Unit,
) {
    if (sale.relatedJobId != null) {
        TlCard {
            Text("Related Job", style = MaterialTheme.typography.titleSmall)
            Text(
                sale.relatedJobLabel ?: sale.relatedJobId,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            TlSecondaryButton(
                text = "Open Job",
                onClick = { onOpenJob(sale.relatedJobId) },
                modifier = Modifier.fillMaxWidth(),
            )
        }
    }

    TlCard {
        Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
            Text("Customer", style = MaterialTheme.typography.titleSmall)
        }
        TlKeyValue("Name", sale.customerName ?: "Walk-in")
        sale.customerPhone?.let { TlKeyValue("Phone", it) }
    }

    TlCard {
        Text("Transaction", style = MaterialTheme.typography.titleSmall)
        TlKeyValue("Sale", sale.reference)
        sale.createdAt?.let { TlKeyValue("Date/time", fullDateFormat.format(Date(it))) }
        sale.branchName?.let { TlKeyValue("Branch", it) }
        sale.cashierName?.let { TlKeyValue("Cashier", it) }
        TlKeyValue("Payment method", paymentMethodLabel(sale.paymentMethod))
        sale.paymentReference?.let { TlKeyValue("Reference", it) }
    }

    TlCard {
        Text("Items", style = MaterialTheme.typography.titleSmall)
        sale.items.forEachIndexed { index, item ->
            ItemRow(item = item, showCost = canSeeCost)
            if (index != sale.items.lastIndex) TlDivider()
        }
    }

    TlCard {
        Text("Totals", style = MaterialTheme.typography.titleSmall)
        TlKeyValue("Subtotal", formatKes(sale.subtotal))
        if (sale.discountTotal > 0) TlKeyValue("Discount", "-${formatKes(sale.discountTotal)}")
        if (sale.taxTotal > 0) TlKeyValue("Tax", formatKes(sale.taxTotal))
        TlKeyValue("Total", formatKes(sale.total), emphasise = true)
        TlKeyValue("Paid", formatKes(sale.paidTotal))
        if (sale.balanceDue > 0) {
            TlKeyValue("Balance", formatKes(sale.balanceDue), emphasise = true, valueColor = MaterialTheme.colorScheme.error)
        }
    }

    TlCard {
        Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
            Text("Payment", style = MaterialTheme.typography.titleSmall)
            TlStatusPill(
                text = if (sale.paymentIsSettled) "Confirmed" else sale.paymentStatus.ifBlank { "Unknown" }.replaceFirstChar(Char::uppercase),
                tone = if (sale.paymentIsSettled) TlTone.Success else TlTone.Warning,
            )
        }
        TlKeyValue("Method", paymentMethodLabel(sale.paymentMethod))
        sale.paymentReference?.let { TlKeyValue("Reference", it) }
    }

    TlCard {
        Text("Receipt", style = MaterialTheme.typography.titleSmall)
        if (sale.fromCache) {
            Text(
                "Showing a saved copy — reconnect to load the latest.",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        TlSecondaryButton(text = "View Receipt", onClick = onViewReceipt, icon = Icons.Outlined.Visibility, modifier = Modifier.fillMaxWidth())
        if (printerDisconnected) {
            TlSecondaryButton(text = "Connect Printer", onClick = onConnectPrinter, icon = Icons.Outlined.Settings, modifier = Modifier.fillMaxWidth())
        } else {
            TlButton(
                text = if (printing) "Printing…" else "Print Receipt",
                onClick = onPrintReceipt,
                icon = Icons.Outlined.Print,
                loading = printing,
                modifier = Modifier.fillMaxWidth(),
            )
        }
        TlSecondaryButton(
            text = if (sharing) "Preparing…" else "Share Receipt",
            onClick = onShareReceipt,
            icon = Icons.Outlined.Share,
            enabled = !sharing,
            modifier = Modifier.fillMaxWidth(),
        )
    }
}

@Composable
private fun ItemRow(item: SaleLineItem, showCost: Boolean) {
    Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
        androidx.compose.foundation.layout.Column(modifier = Modifier.weight(1f)) {
            Text(item.description, style = MaterialTheme.typography.bodyMedium)
            Text(
                "× ${item.quantity} @ ${formatKes(item.unitPrice)}",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            // Cost/margin are never shown to a cashier — the server already
            // strips them from the response for that role, this is belt-and-braces.
            if (showCost && item.hasCostInfo) {
                Text(
                    "Cost ${formatKes(item.unitCost ?: 0.0)} · Margin ${formatKes(item.margin ?: 0.0)}",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }
        Text(formatKes(item.lineTotal), style = MaterialTheme.typography.bodyMedium)
    }
}

@Composable
private fun ReceiptViewDialog(html: String, onShare: () -> Unit, onDismiss: () -> Unit) {
    Dialog(onDismissRequest = onDismiss) {
        androidx.compose.material3.Surface(modifier = Modifier.fillMaxSize()) {
            androidx.compose.foundation.layout.Column(Modifier.fillMaxSize()) {
                Row(
                    modifier = Modifier.fillMaxWidth().padding(TlTheme.spacing.md),
                    horizontalArrangement = Arrangement.SpaceBetween,
                ) {
                    IconButton(onClick = onDismiss) { Icon(Icons.Outlined.Close, contentDescription = "Close") }
                    IconButton(onClick = onShare) { Icon(Icons.Outlined.Share, contentDescription = "Share") }
                }
                AndroidView(
                    modifier = Modifier.fillMaxSize(),
                    factory = { ctx ->
                        WebView(ctx).apply {
                            settings.javaScriptEnabled = false
                            loadDataWithBaseURL("https://techlane.local/", html, "text/html", "UTF-8", null)
                        }
                    },
                )
            }
        }
    }
}

private fun statusLabel(status: String): String = when (status) {
    "completed" -> "Completed"
    "reversed" -> "Refunded"
    "draft" -> "Draft"
    else -> status.replaceFirstChar(Char::uppercase)
}

private val fullDateFormat = SimpleDateFormat("d MMM yyyy, h:mm a", Locale.getDefault())
