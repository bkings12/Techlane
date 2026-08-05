package com.techlane.pos.feature.jobs.components

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ReceiptLong
import androidx.compose.material.icons.outlined.LocalAtm
import androidx.compose.material.icons.outlined.PhoneAndroid
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.RadioButton
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
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.dp
import com.techlane.pos.core.designsystem.component.TlAmountField
import com.techlane.pos.core.designsystem.component.TlButton
import com.techlane.pos.core.designsystem.component.TlKeyValue
import com.techlane.pos.core.designsystem.component.TlNeutralButton
import com.techlane.pos.core.designsystem.component.TlPhoneField
import com.techlane.pos.core.designsystem.component.TlTextField
import com.techlane.pos.core.designsystem.theme.TlTheme
import com.techlane.pos.core.util.Msisdn
import com.techlane.pos.core.util.formatKes
import com.techlane.pos.domain.model.MpesaReference
import com.techlane.pos.domain.model.PaymentMethod

/**
 * Single entry for settling a repair balance: pick Cash / Paybill / Quick Prompt,
 * then fill the few fields that method needs. Confirms into the existing
 * [com.techlane.pos.data.repository.ChargeRepository.chargeRepair] path.
 */
data class TakePaymentDraft(
    val method: PaymentMethod,
    val amount: Double,
    val phone: String? = null,
    /** Paybill: optional M-Pesa TransID for display/audit; bill ref is the job code. */
    val reference: String? = null,
    val cashReceived: Double? = null,
)

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun TakePaymentSheet(
    balanceDue: Double,
    customerPhone: String?,
    jobCode: String,
    onConfirm: (TakePaymentDraft) -> Unit,
    onDismiss: () -> Unit,
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    var method by remember { mutableStateOf(PaymentMethod.MpesaStk) }
    var amountDigits by remember {
        mutableStateOf(balanceDigits(balanceDue))
    }
    var phone by remember { mutableStateOf(customerPhone.orEmpty()) }
    var paybillCode by remember { mutableStateOf("") }
    var cashReceivedDigits by remember { mutableStateOf("") }

    val amount = parseDigits(amountDigits)
    val cashReceived = parseDigits(cashReceivedDigits)
    val change = (cashReceived - amount).coerceAtLeast(0.0)

    val phoneOk = method != PaymentMethod.MpesaStk || Msisdn.normalise(phone) != null
    val amountOk = amount > 0.0 && amount <= balanceDue + 0.009
    val paybillOk = method != PaymentMethod.Paybill ||
        paybillCode.isBlank() || MpesaReference.validationError(paybillCode) == null
    val canContinue = amountOk && phoneOk && paybillOk

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        containerColor = MaterialTheme.colorScheme.surface,
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = TlTheme.spacing.gutter)
                .padding(bottom = TlTheme.spacing.xxl),
            verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.md),
        ) {
            Text("Take payment", style = MaterialTheme.typography.titleLarge)
            Text(
                "${formatKes(balanceDue)} due · $jobCode",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )

            MethodRow(
                method = PaymentMethod.Cash,
                selected = method,
                icon = Icons.Outlined.LocalAtm,
                onSelect = { method = it },
            )
            MethodRow(
                method = PaymentMethod.Paybill,
                selected = method,
                icon = Icons.AutoMirrored.Outlined.ReceiptLong,
                onSelect = { method = it },
            )
            MethodRow(
                method = PaymentMethod.MpesaStk,
                labelOverride = "Quick Prompt",
                selected = method,
                icon = Icons.Outlined.PhoneAndroid,
                onSelect = { method = it },
            )

            Text("Amount", style = MaterialTheme.typography.labelMedium)
            TlAmountField(
                digits = amountDigits,
                onDigitsChange = { amountDigits = it },
            )
            if (amount > balanceDue + 0.009) {
                Text(
                    "Cannot take more than the outstanding balance.",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.error,
                )
            }

            when (method) {
                PaymentMethod.Cash -> {
                    Text("Cash received", style = MaterialTheme.typography.labelMedium)
                    TlAmountField(
                        digits = cashReceivedDigits,
                        onDigitsChange = { cashReceivedDigits = it },
                    )
                    if (cashReceivedDigits.isNotBlank()) {
                        TlKeyValue("Change", formatKes(change), emphasise = change > 0)
                    }
                }
                PaymentMethod.MpesaStk -> {
                    TlPhoneField(
                        value = phone,
                        onValueChange = { phone = it },
                        label = "Customer M-Pesa number",
                        helper = "Pre-filled from the job — edit if they use a different line.",
                    )
                }
                PaymentMethod.Paybill -> {
                    Text(
                        "Customer pays Till/Paybill with account reference $jobCode. " +
                            "Optional: paste the M-Pesa code once they confirm.",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                    TlTextField(
                        value = paybillCode,
                        onValueChange = { paybillCode = MpesaReference.normalise(it).take(10) },
                        label = "M-Pesa code (optional)",
                        placeholder = "e.g. QHK7T9XXXX",
                        error = MpesaReference.validationError(paybillCode)
                            .takeIf { paybillCode.isNotBlank() },
                        showClear = true,
                    )
                }
            }

            TlButton(
                text = when (method) {
                    PaymentMethod.MpesaStk -> "Send Prompt · ${formatKes(amount)}"
                    PaymentMethod.Cash -> "Record Cash · ${formatKes(amount)}"
                    PaymentMethod.Paybill -> "Record Paybill · ${formatKes(amount)}"
                },
                onClick = {
                    onConfirm(
                        TakePaymentDraft(
                            method = method,
                            amount = amount,
                            phone = phone.takeIf { it.isNotBlank() },
                            reference = paybillCode.takeIf { it.isNotBlank() },
                            cashReceived = cashReceived.takeIf { cashReceivedDigits.isNotBlank() },
                        ),
                    )
                },
                enabled = canContinue,
                modifier = Modifier.fillMaxWidth(),
                large = true,
            )
            TlNeutralButton(
                text = "Cancel",
                onClick = onDismiss,
                modifier = Modifier.fillMaxWidth(),
            )
        }
    }
}

@Composable
private fun MethodRow(
    method: PaymentMethod,
    selected: PaymentMethod,
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    onSelect: (PaymentMethod) -> Unit,
    labelOverride: String? = null,
) {
    val isSelected = method == selected
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .selectable(
                selected = isSelected,
                onClick = { onSelect(method) },
                role = Role.RadioButton,
            ),
        shape = MaterialTheme.shapes.small,
        color = if (isSelected) {
            MaterialTheme.colorScheme.primaryContainer
        } else {
            MaterialTheme.colorScheme.surfaceVariant
        },
        border = if (isSelected) {
            androidx.compose.foundation.BorderStroke(2.dp, MaterialTheme.colorScheme.primary)
        } else {
            null
        },
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .heightIn(min = 64.dp)
                .padding(horizontal = TlTheme.spacing.lg, vertical = TlTheme.spacing.md),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.md),
        ) {
            RadioButton(selected = isSelected, onClick = null)
            Icon(
                icon,
                contentDescription = null,
                modifier = Modifier.size(TlTheme.sizes.icon),
                tint = if (isSelected) {
                    MaterialTheme.colorScheme.primary
                } else {
                    MaterialTheme.colorScheme.onSurfaceVariant
                },
            )
            Text(
                labelOverride ?: method.display,
                style = MaterialTheme.typography.titleMedium,
                modifier = Modifier.weight(1f),
            )
        }
    }
}

/** Whole shillings as digits — matches [TlAmountField] / Quick Charge. */
private fun balanceDigits(balance: Double): String {
    if (balance <= 0) return ""
    return kotlin.math.round(balance).toLong().coerceAtLeast(0).toString()
}

private fun parseDigits(digits: String): Double {
    val cleaned = digits.filter { it.isDigit() }
    if (cleaned.isEmpty()) return 0.0
    return cleaned.toLongOrNull()?.toDouble() ?: 0.0
}
