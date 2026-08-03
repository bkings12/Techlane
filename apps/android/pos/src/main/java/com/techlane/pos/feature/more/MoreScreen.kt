package com.techlane.pos.feature.more

import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ReceiptLong
import androidx.compose.material.icons.outlined.Build
import androidx.compose.material.icons.outlined.CloudQueue
import androidx.compose.material.icons.outlined.Settings
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewModelScope
import com.techlane.pos.core.designsystem.component.TlCard
import com.techlane.pos.core.designsystem.component.TlListRow
import com.techlane.pos.core.designsystem.component.TlScreen
import com.techlane.pos.core.designsystem.component.TlSectionHeader
import com.techlane.pos.data.repository.JobRepository
import com.techlane.pos.data.session.PosPreferences
import com.techlane.pos.data.session.PreferencesStore
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.stateIn
import javax.inject.Inject

@HiltViewModel
class MoreViewModel @Inject constructor(
    jobs: JobRepository,
    prefs: PreferencesStore,
) : ViewModel() {

    val pendingSync: StateFlow<Int> = jobs.observePendingSyncCount()
        .stateIn(viewModelScope, SharingStarted.WhileSubscribed(5_000), 0)

    val preferences: StateFlow<PosPreferences> = prefs.preferences
        .stateIn(viewModelScope, SharingStarted.WhileSubscribed(5_000), PosPreferences())
}

/**
 * Overflow for everything that does not deserve a tab. Kept short on purpose —
 * this is a drawer, not a second home screen.
 */
@Composable
fun MoreScreen(
    onOpenSettings: () -> Unit,
    modifier: Modifier = Modifier,
    viewModel: MoreViewModel = hiltViewModel(),
) {
    val pending by viewModel.pendingSync.collectAsStateWithLifecycle()
    val prefs by viewModel.preferences.collectAsStateWithLifecycle()

    TlScreen(
        title = "More",
        subtitle = prefs.displayName,
        modifier = modifier,
    ) {
        TlSectionHeader(title = "This device")
        TlCard(contentPadding = PaddingValues(0.dp)) {
            TlListRow(
                title = "Sync",
                subtitle = if (pending == 0) {
                    "Everything is up to date"
                } else {
                    "$pending change${if (pending == 1) "" else "s"} waiting for a connection"
                },
                leadingIcon = Icons.Outlined.CloudQueue,
            )
            TlListRow(
                title = "Till",
                subtitle = listOfNotNull(prefs.branchName, prefs.locationName)
                    .joinToString(" · ")
                    .ifBlank { "No branch selected" },
                leadingIcon = Icons.AutoMirrored.Outlined.ReceiptLong,
            )
            TlListRow(
                title = "Settings",
                subtitle = "Branch, biometrics, theme, sign out",
                leadingIcon = Icons.Outlined.Settings,
                onClick = onOpenSettings,
            )
        }

        TlSectionHeader(title = "Coming next")
        TlCard(contentPadding = PaddingValues(0.dp)) {
            TlListRow(
                title = "Inventory & procurement",
                subtitle = "Supplier part requests are handled on the web console for now",
                leadingIcon = Icons.Outlined.Build,
            )
        }

        Text(
            "Repairs, charging and receipts work offline. Anything you change is saved on " +
                "this phone first and syncs when you're back on a connection.",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}
