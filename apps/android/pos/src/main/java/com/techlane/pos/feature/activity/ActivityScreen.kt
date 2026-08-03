package com.techlane.pos.feature.activity

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ReceiptLong
import androidx.compose.material.icons.outlined.Print
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.style.TextOverflow
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.techlane.pos.core.print.ReceiptPrinter
import com.techlane.pos.core.designsystem.component.TlBanner
import com.techlane.pos.core.designsystem.component.TlCard
import com.techlane.pos.core.designsystem.component.TlEmptyState
import com.techlane.pos.core.designsystem.component.TlScreen
import com.techlane.pos.core.designsystem.component.TlSecondaryButton
import com.techlane.pos.core.designsystem.component.TlStatusPill
import com.techlane.pos.core.designsystem.component.TlTone
import com.techlane.pos.core.designsystem.theme.TlTheme
import com.techlane.pos.core.util.Msisdn
import com.techlane.pos.core.util.formatKes
import com.techlane.pos.data.local.ChargeRecordEntity
import com.techlane.pos.data.repository.ChargeRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import javax.inject.Inject

@HiltViewModel
class ActivityViewModel @Inject constructor(
    private val charges: ChargeRepository,
) : ViewModel() {

    val records: StateFlow<List<ChargeRecordEntity>> = charges.observeRecent()
        .stateIn(viewModelScope, SharingStarted.WhileSubscribed(5_000), emptyList())

    private val _refreshing = MutableStateFlow(false)
    val refreshing: StateFlow<Boolean> = _refreshing.asStateFlow()

    private val _message = MutableStateFlow<String?>(null)
    val message: StateFlow<String?> = _message.asStateFlow()

    /**
     * Pulling down re-asks the server about every prompt still marked unconfirmed.
     * This is the manual counterpart to the poll that runs during a charge, and
     * it is how an end-of-day "did that one ever go through?" gets answered.
     */
    fun refresh() {
        if (_refreshing.value) return
        _refreshing.value = true
        viewModelScope.launch {
            val changed = runCatching { charges.refreshUnresolved() }.getOrDefault(0)
            _message.value = when (changed) {
                0 -> null
                1 -> "1 charge updated"
                else -> "$changed charges updated"
            }
            _refreshing.value = false
        }
    }

    fun clearMessage() { _message.value = null }

    /** Reprint for a customer who comes back for their receipt. */
    fun printReceipt(context: android.content.Context, saleId: String) {
        viewModelScope.launch {
            charges.receiptHtml(saleId)
                .onSuccess { html -> ReceiptPrinter.print(context, html, "TechLane receipt") }
                .onFailure { error -> _message.value = error.message ?: "Could not load that receipt" }
        }
    }
}

/**
 * Every prompt this phone has sent, including the ones that never resolved —
 * so an unconfirmed payment is never simply forgotten at close of day.
 */
@Composable
fun ActivityScreen(modifier: Modifier = Modifier, viewModel: ActivityViewModel = hiltViewModel()) {
    val records by viewModel.records.collectAsStateWithLifecycle()
    val refreshing by viewModel.refreshing.collectAsStateWithLifecycle()
    val message by viewModel.message.collectAsStateWithLifecycle()
    val context = LocalContext.current

    TlScreen(
        title = "Activity",
        subtitle = "Charges sent from this phone",
        modifier = modifier,
        onRefresh = viewModel::refresh,
        refreshing = refreshing,
    ) {
        TlBanner(message = message, tone = TlTone.Success)
        if (records.isEmpty()) {
            TlEmptyState(
                title = "Nothing charged yet",
                subtitle = "Prompts you send will show up here with their outcome.",
                icon = Icons.AutoMirrored.Outlined.ReceiptLong,
            )
        } else {
            records.forEach { record ->
                ChargeRow(
                    record = record,
                    onPrint = record.saleId
                        ?.takeIf { record.status == "paid" }
                        ?.let { saleId -> { viewModel.printReceipt(context, saleId) } },
                )
            }
        }
    }
}

@Composable
private fun ChargeRow(record: ChargeRecordEntity, onPrint: (() -> Unit)?) {
    TlCard {
        Row(
            modifier = Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.Top,
            horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.md),
        ) {
            Column(modifier = Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.xxs)) {
                Text(
                    record.label,
                    style = MaterialTheme.typography.titleSmall,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                Text(
                    buildString {
                        append(timeFormat.format(Date(record.createdAt)))
                        record.phone?.let { append(" · ").append(Msisdn.mask(it)) }
                    },
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                record.failureReason?.let {
                    Text(it, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.error)
                }
            }
            Column(horizontalAlignment = Alignment.End, verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.xs)) {
                Text(formatKes(record.amount), style = MaterialTheme.typography.titleSmall)
                TlStatusPill(text = record.status.display(), tone = record.status.tone())
            }
        }
        if (onPrint != null) {
            TlSecondaryButton(
                text = "Print receipt",
                onClick = onPrint,
                icon = Icons.Outlined.Print,
                modifier = Modifier.fillMaxWidth(),
            )
        }
    }
}

private fun String.display(): String = when (this) {
    "paid" -> "Paid"
    "failed" -> "Not paid"
    "pending" -> "Unconfirmed"
    "sent" -> "Sent"
    else -> replaceFirstChar { it.uppercase() }
}

private fun String.tone(): TlTone = when (this) {
    "paid" -> TlTone.Success
    "failed" -> TlTone.Danger
    "pending", "sent" -> TlTone.Warning
    else -> TlTone.Neutral
}

private val timeFormat = SimpleDateFormat("d MMM, HH:mm", Locale.getDefault())
