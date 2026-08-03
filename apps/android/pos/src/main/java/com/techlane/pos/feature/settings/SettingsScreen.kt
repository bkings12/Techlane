package com.techlane.pos.feature.settings

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.DarkMode
import androidx.compose.material.icons.outlined.Fingerprint
import androidx.compose.material.icons.outlined.Inventory2
import androidx.compose.material.icons.outlined.Print
import androidx.compose.material.icons.automirrored.outlined.Logout
import androidx.compose.material.icons.outlined.Storefront
import androidx.compose.material3.FilterChip
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.techlane.pos.core.designsystem.component.TlBanner
import com.techlane.pos.core.designsystem.component.TlCard
import com.techlane.pos.core.designsystem.component.TlDangerButton
import com.techlane.pos.core.designsystem.component.TlListRow
import com.techlane.pos.core.designsystem.component.TlScreen
import com.techlane.pos.core.designsystem.component.TlSecondaryButton
import com.techlane.pos.core.designsystem.component.TlSectionHeader
import com.techlane.pos.core.designsystem.component.TlTone
import com.techlane.pos.core.designsystem.theme.ThemeMode
import com.techlane.pos.core.designsystem.theme.TlTheme
import com.techlane.pos.feature.auth.findFragmentActivity

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun SettingsScreen(
    onBack: () -> Unit,
    onSignedOut: () -> Unit,
    onOpenPrinterSettings: () -> Unit,
    modifier: Modifier = Modifier,
    viewModel: SettingsViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val activity = LocalContext.current.findFragmentActivity()

    TlScreen(
        title = "Settings",
        onBack = onBack,
        modifier = modifier,
        onRefresh = viewModel::loadBranches,
        refreshing = state.loading,
    ) {
        TlBanner(message = state.error, tone = TlTone.Danger)
        TlBanner(message = state.message, tone = TlTone.Success)

        TlSectionHeader(
            title = "Till",
            subtitle = "Charges post against this branch and stock location.",
        )
        TlCard {
            Text("Branch", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            FlowRow(horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm)) {
                state.branches.forEach { branch ->
                    FilterChip(
                        selected = state.prefs.branchId == branch.id,
                        onClick = { viewModel.selectBranch(branch) },
                        label = { Text(branch.name) },
                    )
                }
            }
            if (state.branches.isEmpty()) {
                Text(
                    if (state.loading) "Loading branches…" else "No branches found for your account.",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }

            Text(
                "Stock location",
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            FlowRow(horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm)) {
                state.locations.forEach { location ->
                    FilterChip(
                        selected = state.prefs.locationId == location.id,
                        onClick = { viewModel.selectLocation(location) },
                        label = { Text(location.name) },
                    )
                }
            }
            if (state.locations.isEmpty() && state.prefs.branchId != null) {
                Text(
                    "No stock locations on this branch yet.",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }

        TlSectionHeader(title = "Offline")
        TlCard(contentPadding = PaddingValues(0.dp)) {
            TlListRow(
                title = "Cached stock",
                subtitle = state.catalogCount?.let { "$it items ready to sell without a connection" }
                    ?: "Pull the catalog so the product picker works offline",
                leadingIcon = Icons.Outlined.Inventory2,
            )
            TlSecondaryButton(
                text = if (state.syncing) "Syncing…" else "Sync catalog now",
                onClick = viewModel::syncCatalog,
                loading = state.syncing,
                enabled = state.prefs.locationId != null,
                modifier = Modifier.fillMaxWidth().padding(TlTheme.spacing.lg),
            )
        }

        TlSectionHeader(title = "Printer")
        TlCard(contentPadding = PaddingValues(0.dp)) {
            TlListRow(
                title = "Bluetooth thermal printer",
                subtitle = "GOOJPRT MTP-II · 58mm",
                leadingIcon = Icons.Outlined.Print,
                onClick = onOpenPrinterSettings,
            )
        }

        TlSectionHeader(title = "Security")
        TlCard(contentPadding = PaddingValues(0.dp)) {
            TlListRow(
                title = "Fingerprint sign-in",
                subtitle = when {
                    !state.biometricAvailable -> "Not available on this phone"
                    state.prefs.biometricEnabled -> "On — one touch reopens the till"
                    else -> "Off — you'll type your password each time"
                },
                leadingIcon = Icons.Outlined.Fingerprint,
                trailing = {
                    Switch(
                        checked = state.prefs.biometricEnabled,
                        onCheckedChange = { enabled ->
                            activity?.let { viewModel.setBiometricEnabled(it, enabled) }
                        },
                        enabled = state.biometricAvailable && activity != null,
                    )
                },
            )
        }

        TlSectionHeader(title = "Appearance")
        TlCard {
            Text(
                "Theme",
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            FlowRow(horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm)) {
                ThemeMode.entries.forEach { mode ->
                    FilterChip(
                        selected = state.prefs.themeMode == mode,
                        onClick = { viewModel.setThemeMode(mode) },
                        label = {
                            Text(
                                when (mode) {
                                    ThemeMode.SYSTEM -> "Match phone"
                                    ThemeMode.LIGHT -> "Light"
                                    ThemeMode.DARK -> "Dark"
                                },
                            )
                        },
                    )
                }
            }
        }

        TlSectionHeader(title = "Account")
        TlCard(contentPadding = PaddingValues(0.dp)) {
            TlListRow(
                title = state.prefs.displayName ?: "Signed in",
                subtitle = state.prefs.roles.joinToString(", ").ifBlank { "Staff" },
                leadingIcon = Icons.Outlined.Storefront,
            )
            TlDangerButton(
                text = "Sign out",
                onClick = { viewModel.signOut(onSignedOut) },
                icon = Icons.AutoMirrored.Outlined.Logout,
                modifier = Modifier.fillMaxWidth().padding(TlTheme.spacing.lg),
            )
        }

        Column(modifier = Modifier.fillMaxWidth()) {
            Text(
                "Signing out clears cached stock and turns off fingerprint sign-in on this phone.",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}
