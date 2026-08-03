package com.techlane.pos.core.designsystem.component

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Clear
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import com.techlane.pos.core.designsystem.theme.TlTheme

/**
 * The single text input in the system. Label sits above the box rather than
 * floating inside it — at arm's length on a counter, a static label is faster
 * to scan than one that animates away.
 */
@Composable
fun TlTextField(
    value: String,
    onValueChange: (String) -> Unit,
    label: String,
    modifier: Modifier = Modifier,
    placeholder: String? = null,
    helper: String? = null,
    error: String? = null,
    enabled: Boolean = true,
    readOnly: Boolean = false,
    singleLine: Boolean = true,
    leadingIcon: ImageVector? = null,
    trailingIcon: (@Composable () -> Unit)? = null,
    keyboardType: KeyboardType = KeyboardType.Text,
    imeAction: ImeAction = ImeAction.Next,
    visualTransformation: VisualTransformation = VisualTransformation.None,
    showClear: Boolean = false,
) {
    Column(modifier = modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.xs)) {
        Text(
            text = label,
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        OutlinedTextField(
            value = value,
            onValueChange = onValueChange,
            modifier = Modifier
                .fillMaxWidth()
                .heightIn(min = TlTheme.sizes.controlHeight),
            enabled = enabled,
            readOnly = readOnly,
            singleLine = singleLine,
            isError = error != null,
            textStyle = MaterialTheme.typography.bodyLarge,
            placeholder = placeholder?.let {
                {
                    Text(
                        it,
                        style = MaterialTheme.typography.bodyLarge,
                        color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.7f),
                    )
                }
            },
            leadingIcon = leadingIcon?.let {
                { Icon(it, contentDescription = null, modifier = Modifier.size(TlTheme.sizes.icon)) }
            },
            trailingIcon = when {
                trailingIcon != null -> trailingIcon
                showClear && value.isNotEmpty() -> {
                    {
                        IconButton(onClick = { onValueChange("") }) {
                            Icon(Icons.Outlined.Clear, contentDescription = "Clear")
                        }
                    }
                }
                else -> null
            },
            keyboardOptions = KeyboardOptions(keyboardType = keyboardType, imeAction = imeAction),
            visualTransformation = visualTransformation,
            shape = MaterialTheme.shapes.small,
            colors = OutlinedTextFieldDefaults.colors(
                focusedBorderColor = MaterialTheme.colorScheme.primary,
                unfocusedBorderColor = TlTheme.colors.hairline,
                focusedContainerColor = MaterialTheme.colorScheme.surface,
                unfocusedContainerColor = MaterialTheme.colorScheme.surface,
                disabledContainerColor = MaterialTheme.colorScheme.surfaceVariant,
            ),
        )
        val message = error ?: helper
        if (message != null) {
            Text(
                text = message,
                style = MaterialTheme.typography.bodySmall,
                color = if (error != null) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

/**
 * Kenyan mobile number entry. Keeps the raw digits the technician typed and
 * shows the normalised MSISDN as helper text, so a mistyped prefix is visible
 * *before* the STK prompt goes to the wrong phone.
 */
@Composable
fun TlPhoneField(
    value: String,
    onValueChange: (String) -> Unit,
    modifier: Modifier = Modifier,
    label: String = "Customer phone",
    error: String? = null,
    helper: String? = null,
    enabled: Boolean = true,
    imeAction: ImeAction = ImeAction.Done,
) = TlTextField(
    value = value,
    onValueChange = { input -> onValueChange(input.filter { it.isDigit() || it == '+' }.take(15)) },
    label = label,
    modifier = modifier,
    placeholder = "07XX XXX XXX",
    helper = helper,
    error = error,
    enabled = enabled,
    keyboardType = KeyboardType.Phone,
    imeAction = imeAction,
    showClear = true,
)

/** Fixed 1dp divider tuned to the theme hairline. */
@Composable
fun TlDivider(modifier: Modifier = Modifier) {
    androidx.compose.material3.HorizontalDivider(
        modifier = modifier,
        thickness = 1.dp,
        color = TlTheme.colors.hairline,
    )
}
