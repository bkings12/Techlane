package com.techlane.pos.feature.jobs.components

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import com.techlane.pos.core.designsystem.component.TlBanner
import com.techlane.pos.core.designsystem.component.TlButton
import com.techlane.pos.core.designsystem.component.TlNeutralButton
import com.techlane.pos.core.designsystem.component.TlSecondaryButton
import com.techlane.pos.core.designsystem.component.TlTextField
import com.techlane.pos.core.designsystem.component.TlTone
import com.techlane.pos.core.designsystem.theme.TlTheme
import com.techlane.pos.core.util.formatKes

/**
 * Releases the device after the balance is cleared. Uses the server handover
 * endpoint — status→collected via changeStatus is rejected.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun CollectHandoverSheet(
    customerName: String?,
    balanceDue: Double,
    canVouch: Boolean,
    sendingCode: Boolean,
    onConfirm: (collectedByName: String, relationship: String, note: String?, pickupCode: String?, otp: String?) -> Unit,
    onSendCode: () -> Unit,
    onTakePayment: () -> Unit,
    onDismiss: () -> Unit,
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    var name by remember { mutableStateOf(customerName.orEmpty()) }
    var note by remember { mutableStateOf("") }
    var pickupCode by remember { mutableStateOf("") }
    var otp by remember { mutableStateOf("") }
    val blocked = balanceDue > 0.009

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        containerColor = MaterialTheme.colorScheme.surface,
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = TlTheme.spacing.gutter)
                .padding(bottom = TlTheme.spacing.xxl),
            verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.md),
        ) {
            Text("Mark collected", style = MaterialTheme.typography.titleLarge)
            if (blocked) {
                TlBanner(
                    message = "Outstanding balance ${formatKes(balanceDue)}. Collect payment before handing over the device.",
                    tone = TlTone.Warning,
                )
                TlButton(
                    text = "Take Payment",
                    onClick = {
                        onDismiss()
                        onTakePayment()
                    },
                    modifier = Modifier.fillMaxWidth(),
                    large = true,
                )
                TlNeutralButton(text = "Cancel", onClick = onDismiss, modifier = Modifier.fillMaxWidth())
                return@Column
            }

            Text(
                "Payment is settled. Record who is taking the device.",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            TlTextField(
                value = name,
                onValueChange = { name = it },
                label = "Collected by",
                placeholder = customerName ?: "Customer name",
            )
            TlTextField(
                value = pickupCode,
                onValueChange = { pickupCode = it.trim().uppercase() },
                label = "Pickup code",
                placeholder = "From intake slip / SMS",
                helper = if (canVouch) {
                    "Optional if you can vouch for the customer."
                } else {
                    "Required — or send an OTP below."
                },
                showClear = true,
            )
            if (!canVouch) {
                TlTextField(
                    value = otp,
                    onValueChange = { otp = it.filter(Char::isDigit).take(6) },
                    label = "OTP (if no pickup code)",
                    placeholder = "6-digit code",
                    showClear = true,
                )
                TlSecondaryButton(
                    text = if (sendingCode) "Sending…" else "Send handover OTP",
                    onClick = onSendCode,
                    enabled = !sendingCode,
                    modifier = Modifier.fillMaxWidth(),
                )
            }
            TlTextField(
                value = note,
                onValueChange = { note = it },
                label = "Note (optional)",
                placeholder = "ID checked, proxy, etc.",
                singleLine = false,
            )
            val hasProof = pickupCode.isNotBlank() || otp.isNotBlank() || canVouch
            TlButton(
                text = "Confirm collected",
                onClick = {
                    onConfirm(
                        name.trim().ifBlank { customerName ?: "Customer" },
                        "self",
                        note.trim().takeIf { it.isNotEmpty() },
                        pickupCode.takeIf { it.isNotBlank() },
                        otp.takeIf { it.isNotBlank() },
                    )
                },
                enabled = (name.isNotBlank() || !customerName.isNullOrBlank()) && hasProof,
                modifier = Modifier.fillMaxWidth(),
                large = true,
            )
            TlNeutralButton(text = "Cancel", onClick = onDismiss, modifier = Modifier.fillMaxWidth())
        }
    }
}
