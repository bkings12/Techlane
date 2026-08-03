package com.techlane.pos.core.designsystem.component

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.LocalTextStyle
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.OffsetMapping
import androidx.compose.ui.text.input.TransformedText
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import com.techlane.pos.core.designsystem.theme.MoneyDisplay
import com.techlane.pos.core.designsystem.theme.TlTheme
import com.techlane.pos.core.util.groupDigits

/**
 * Large money entry. Holds raw digits and renders them grouped, so the caller
 * never has to strip separators back out and the technician still reads
 * "12,500" rather than "12500" at a glance.
 */
@Composable
fun TlAmountField(
    digits: String,
    onDigitsChange: (String) -> Unit,
    modifier: Modifier = Modifier,
    currency: String = "KES",
    maxDigits: Int = 7,
    enabled: Boolean = true,
    imeAction: ImeAction = ImeAction.Done,
    onImeAction: () -> Unit = {},
    textStyle: TextStyle = MoneyDisplay,
) {
    Row(
        modifier = modifier.fillMaxWidth(),
        verticalAlignment = Alignment.Bottom,
        horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm),
    ) {
        Text(
            currency,
            style = MaterialTheme.typography.titleMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.padding(bottom = 8.dp),
        )
        Box(modifier = Modifier.weight(1f), contentAlignment = Alignment.BottomStart) {
            if (digits.isEmpty()) {
                Text(
                    "0",
                    style = textStyle,
                    color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.4f),
                )
            }
            BasicTextField(
                value = digits,
                onValueChange = { input -> onDigitsChange(input.filter(Char::isDigit).take(maxDigits)) },
                modifier = Modifier.fillMaxWidth(),
                enabled = enabled,
                singleLine = true,
                textStyle = LocalTextStyle.current.merge(textStyle)
                    .copy(color = MaterialTheme.colorScheme.onSurface),
                cursorBrush = androidx.compose.ui.graphics.SolidColor(MaterialTheme.colorScheme.primary),
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number, imeAction = imeAction),
                keyboardActions = androidx.compose.foundation.text.KeyboardActions(
                    onDone = { onImeAction() },
                    onGo = { onImeAction() },
                ),
                visualTransformation = ThousandsSeparatorTransformation,
            )
        }
    }
}

/**
 * Inserts thousands separators for display while keeping the cursor on the digit
 * the user is actually editing. The index maps are built from the rendered
 * string rather than computed arithmetically — shorter, and obviously right.
 */
private object ThousandsSeparatorTransformation : VisualTransformation {

    override fun filter(text: AnnotatedString): TransformedText {
        val digits = text.text
        if (digits.isEmpty()) return TransformedText(text, OffsetMapping.Identity)
        val grouped = groupDigits(digits)

        val originalToTransformed = IntArray(digits.length + 1)
        var digitIndex = 0
        grouped.forEachIndexed { index, char ->
            if (char.isDigit()) {
                originalToTransformed[digitIndex] = index
                digitIndex++
            }
        }
        originalToTransformed[digits.length] = grouped.length

        val transformedToOriginal = IntArray(grouped.length + 1)
        var seen = 0
        grouped.forEachIndexed { index, char ->
            transformedToOriginal[index] = seen
            if (char.isDigit()) seen++
        }
        transformedToOriginal[grouped.length] = seen

        return TransformedText(
            AnnotatedString(grouped),
            object : OffsetMapping {
                override fun originalToTransformed(offset: Int): Int =
                    originalToTransformed[offset.coerceIn(0, digits.length)]

                override fun transformedToOriginal(offset: Int): Int =
                    transformedToOriginal[offset.coerceIn(0, grouped.length)]
            },
        )
    }
}
