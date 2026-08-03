package com.techlane.pos.feature.charge

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutVertically
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Close
import androidx.compose.material.icons.outlined.DarkMode
import androidx.compose.material.icons.outlined.LightMode
import androidx.compose.material.icons.outlined.LocalAtm
import androidx.compose.material.icons.outlined.PhoneAndroid
import androidx.compose.material.icons.outlined.Settings
import androidx.compose.material.icons.outlined.ShoppingBag
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.techlane.pos.core.designsystem.component.TlAccentButton
import com.techlane.pos.core.designsystem.component.TlAmountField
import com.techlane.pos.core.designsystem.component.TlBanner
import com.techlane.pos.core.designsystem.component.TlCard
import com.techlane.pos.core.designsystem.component.TlDivider
import com.techlane.pos.core.designsystem.component.TlPhoneField
import com.techlane.pos.core.designsystem.component.TlScreen
import com.techlane.pos.core.designsystem.component.TlStepper
import com.techlane.pos.core.designsystem.component.TlTone
import com.techlane.pos.core.designsystem.theme.PillShape
import com.techlane.pos.core.designsystem.theme.ThemeMode
import com.techlane.pos.core.designsystem.theme.TlTheme
import com.techlane.pos.core.util.Msisdn
import com.techlane.pos.core.util.formatKes
import com.techlane.pos.domain.model.ChargeTarget
import com.techlane.pos.domain.model.PaymentMethod

/**
 * The screen the shop lives on: type an amount, optionally say what it's for,
 * send the prompt. Everything below the amount is one thumb-reach away, and the
 * charge button never scrolls off.
 */
@Composable
fun QuickChargeScreen(
    onOpenSettings: () -> Unit,
    modifier: Modifier = Modifier,
    viewModel: QuickChargeViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val products by viewModel.products.collectAsStateWithLifecycle()
    val services by viewModel.services.collectAsStateWithLifecycle()
    var showTargetSheet by remember { mutableStateOf(false) }
    var searchQuery by remember { mutableStateOf("") }
    val amountFocus = remember { FocusRequester() }
    val context = androidx.compose.ui.platform.LocalContext.current

    val systemDark = isSystemInDarkTheme()
    val showingDark = when (state.prefs.themeMode) {
        ThemeMode.SYSTEM -> systemDark
        ThemeMode.LIGHT -> false
        ThemeMode.DARK -> true
    }

    // The till opens ready to take a number — that is the whole job of this screen.
    LaunchedEffect(Unit) { runCatching { amountFocus.requestFocus() } }

    // Keeps the product list current without a manual "sync now" tap: a
    // technician who steps away and comes back should see anything added on
    // the web console meanwhile, not just what was cached at cold start.
    val lifecycleOwner = LocalLifecycleOwner.current
    DisposableEffect(lifecycleOwner) {
        val observer = LifecycleEventObserver { _, event ->
            if (event == Lifecycle.Event.ON_RESUME) viewModel.syncCatalogIfStale()
        }
        lifecycleOwner.lifecycle.addObserver(observer)
        onDispose { lifecycleOwner.lifecycle.removeObserver(observer) }
    }

    TlScreen(
        title = "Charge",
        subtitle = listOfNotNull(state.prefs.branchName, state.prefs.locationName)
            .joinToString(" · ")
            .ifBlank { "No till selected" },
        modifier = modifier,
        onRefresh = viewModel::refresh,
        refreshing = state.refreshing,
        actions = {
            IconButton(
                onClick = { viewModel.setThemeMode(if (showingDark) ThemeMode.LIGHT else ThemeMode.DARK) },
            ) {
                Icon(
                    if (showingDark) Icons.Outlined.LightMode else Icons.Outlined.DarkMode,
                    contentDescription = if (showingDark) "Switch to light mode" else "Switch to dark mode",
                )
            }
            IconButton(onClick = onOpenSettings) {
                Icon(Icons.Outlined.Settings, contentDescription = "Settings")
            }
        },
        footer = {
            TlBanner(message = state.validationError ?: state.blockedReason, tone = TlTone.Warning)
            TlAccentButton(
                text = when (state.method) {
                    PaymentMethod.MpesaStk -> "Send M-Pesa prompt · ${formatKes(state.amount)}"
                    PaymentMethod.Cash -> "Record cash · ${formatKes(state.amount)}"
                },
                onClick = viewModel::charge,
                enabled = state.canCharge,
                modifier = Modifier.fillMaxWidth(),
            )
        },
    ) {
        AmountCard(
            digits = state.amountDigits,
            onDigitsChange = viewModel::onAmountChange,
            method = state.method,
            onMethodChange = viewModel::onMethodChange,
            focusRequester = amountFocus,
            onImeDone = viewModel::charge,
        )

        TargetRow(
            target = state.target,
            onOpen = { showTargetSheet = true },
            onClear = viewModel::clearTarget,
            onQuantityChange = viewModel::setProductQuantity,
        )

        AnimatedContent(
            targetState = state.method,
            transitionSpec = {
                (fadeIn(tween(160)) + slideInVertically { it / 6 })
                    .togetherWith(fadeOut(tween(120)) + slideOutVertically { it / 6 })
            },
            label = "methodFields",
        ) { method ->
            if (method == PaymentMethod.MpesaStk) {
                TlPhoneField(
                    value = state.phone,
                    onValueChange = viewModel::onPhoneChange,
                    error = if (state.phone.isNotBlank() && !state.phoneValid) {
                        "That doesn't look like a Kenyan mobile number."
                    } else {
                        null
                    },
                    helper = state.normalisedPhone?.let { "Prompt goes to ${Msisdn.formatLocal(it)}" },
                )
            } else {
                Spacer(Modifier.height(0.dp))
            }
        }

        Spacer(Modifier.height(TlTheme.spacing.sm))
    }

    if (showTargetSheet) {
        ChargeTargetSheet(
            products = products,
            services = services,
            query = searchQuery,
            onQueryChange = {
                searchQuery = it
                viewModel.onSearchQueryChange(it)
            },
            onSelectProduct = {
                viewModel.selectProduct(it)
                showTargetSheet = false
            },
            onSelectService = {
                viewModel.selectService(it)
                showTargetSheet = false
            },
            onAddService = { label, price ->
                viewModel.addAndSelectService(label, price)
                showTargetSheet = false
            },
            onDismiss = { showTargetSheet = false },
        )
    }

    // Blocks the counter until the prompt resolves — the whole point of the flow.
    state.stage?.let { stage ->
        StkStatusScreen(
            stage = stage,
            amount = state.amount,
            phone = state.normalisedPhone,
            label = state.target.label,
            method = state.method,
            canForceReconcile = state.prefs.canForceReconcile,
            receiptBusy = state.receiptBusy,
            receiptError = state.receiptError,
            onPrintReceipt = { viewModel.printReceipt(context) },
            onShareReceipt = { viewModel.shareReceipt(context) },
            onRetry = viewModel::retryFailed,
            onTakeCash = viewModel::switchToCashAndCharge,
            onCheckAgain = viewModel::checkAgain,
            onStopWaiting = viewModel::stopWaiting,
            onDone = { viewModel.finishAndReset() },
            onDismiss = viewModel::dismissResult,
        )
    }
}

@Composable
private fun AmountCard(
    digits: String,
    onDigitsChange: (String) -> Unit,
    method: PaymentMethod,
    onMethodChange: (PaymentMethod) -> Unit,
    focusRequester: FocusRequester,
    onImeDone: () -> Unit,
) {
    TlCard(contentPadding = androidx.compose.foundation.layout.PaddingValues(TlTheme.spacing.xl)) {
        Text(
            "Amount",
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        TlAmountField(
            digits = digits,
            onDigitsChange = onDigitsChange,
            onImeAction = onImeDone,
            modifier = Modifier.focusRequester(focusRequester),
        )

        Row(horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm), modifier = Modifier.fillMaxWidth()) {
            MethodChip(
                label = "M-Pesa prompt",
                icon = Icons.Outlined.PhoneAndroid,
                selected = method == PaymentMethod.MpesaStk,
                onClick = { onMethodChange(PaymentMethod.MpesaStk) },
                modifier = Modifier.weight(1f),
            )
            MethodChip(
                label = "Cash",
                icon = Icons.Outlined.LocalAtm,
                selected = method == PaymentMethod.Cash,
                onClick = { onMethodChange(PaymentMethod.Cash) },
                modifier = Modifier.weight(1f),
            )
        }
    }
}

@Composable
private fun MethodChip(
    label: String,
    icon: ImageVector,
    selected: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Surface(
        onClick = onClick,
        modifier = modifier,
        shape = MaterialTheme.shapes.small,
        color = if (selected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.surfaceVariant,
        border = if (selected) null else androidx.compose.foundation.BorderStroke(1.dp, TlTheme.colors.hairline),
    ) {
        Row(
            modifier = Modifier.padding(horizontal = TlTheme.spacing.md, vertical = TlTheme.spacing.md),
            horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm, Alignment.CenterHorizontally),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(
                icon,
                contentDescription = null,
                modifier = Modifier.size(TlTheme.sizes.iconSm),
                tint = if (selected) MaterialTheme.colorScheme.onPrimary else MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Text(
                label,
                style = MaterialTheme.typography.labelLarge,
                color = if (selected) MaterialTheme.colorScheme.onPrimary else MaterialTheme.colorScheme.onSurface,
                maxLines = 1,
            )
        }
    }
}

@Composable
private fun TargetRow(
    target: ChargeTarget,
    onOpen: () -> Unit,
    onClear: () -> Unit,
    onQuantityChange: (Int) -> Unit,
) {
    val chosen = target !is ChargeTarget.None
    val product = target as? ChargeTarget.Product

    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.small,
        color = if (chosen) MaterialTheme.colorScheme.primaryContainer else MaterialTheme.colorScheme.surface,
        border = androidx.compose.foundation.BorderStroke(
            1.dp,
            if (chosen) MaterialTheme.colorScheme.primary.copy(alpha = 0.35f) else TlTheme.colors.hairline,
        ),
    ) {
        Column {
            Surface(onClick = onOpen, color = androidx.compose.ui.graphics.Color.Transparent) {
                Row(
                    modifier = Modifier.fillMaxWidth().padding(TlTheme.spacing.lg),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.md),
                ) {
                    Surface(
                        shape = PillShape,
                        color = MaterialTheme.colorScheme.primary.copy(alpha = 0.14f),
                        modifier = Modifier.size(36.dp),
                    ) {
                        Box(contentAlignment = Alignment.Center) {
                            Icon(
                                Icons.Outlined.ShoppingBag,
                                contentDescription = null,
                                tint = MaterialTheme.colorScheme.primary,
                                modifier = Modifier.size(TlTheme.sizes.iconSm),
                            )
                        }
                    }
                    Column(modifier = Modifier.weight(1f)) {
                        Text(
                            if (chosen) target.label else "What's this for?",
                            style = MaterialTheme.typography.titleSmall,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                        Text(
                            when {
                                product != null -> "${formatKes(product.unitPrice)} each · tap to change"
                                chosen -> "Tap to change"
                                else -> "Optional — product or service"
                            },
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                    if (chosen) {
                        IconButton(onClick = onClear) {
                            Icon(Icons.Outlined.Close, contentDescription = "Clear")
                        }
                    }
                }
            }

            // Quantity only makes sense for a catalog line: a service is one job,
            // priced by whoever is at the counter.
            if (product != null) {
                TlDivider()
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = TlTheme.spacing.lg, vertical = TlTheme.spacing.md),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.SpaceBetween,
                ) {
                    Column {
                        Text("Quantity", style = MaterialTheme.typography.titleSmall)
                        val stockNote = when {
                            product.availableQty <= 0 -> "Not in stock — the sale will be refused"
                            product.quantity > product.availableQty ->
                                "Only ${product.availableQty} in stock"
                            else -> "${product.availableQty} in stock"
                        }
                        Text(
                            stockNote,
                            style = MaterialTheme.typography.bodySmall,
                            color = if (product.quantity > product.availableQty) {
                                MaterialTheme.colorScheme.error
                            } else {
                                MaterialTheme.colorScheme.onSurfaceVariant
                            },
                        )
                    }
                    TlStepper(
                        value = product.quantity,
                        onValueChange = onQuantityChange,
                        min = 1,
                    )
                }
            }
        }
    }
}
