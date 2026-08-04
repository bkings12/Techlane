package com.techlane.pos.core.designsystem.component

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ArrowBack
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.techlane.pos.core.designsystem.theme.TlTheme

/**
 * Standard screen frame: flat top bar, optional sticky footer, page gutters, and
 * a width cap so the layout stays readable on tablets and foldables instead of
 * stretching a single column across 10 inches.
 */
@OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)
@Composable
fun TlScreen(
    title: String,
    modifier: Modifier = Modifier,
    subtitle: String? = null,
    onBack: (() -> Unit)? = null,
    actions: @Composable RowScope.() -> Unit = {},
    snackbarHostState: SnackbarHostState? = null,
    scrollable: Boolean = true,
    contentPadding: PaddingValues = PaddingValues(vertical = 16.dp),
    /** Supplying this turns on pull-to-refresh for the screen body. */
    onRefresh: (() -> Unit)? = null,
    refreshing: Boolean = false,
    footer: (@Composable ColumnScope.() -> Unit)? = null,
    content: @Composable ColumnScope.() -> Unit,
) {
    Scaffold(
        modifier = modifier.fillMaxSize(),
        containerColor = MaterialTheme.colorScheme.background,
        topBar = { TlTopBar(title = title, subtitle = subtitle, onBack = onBack, actions = actions) },
        bottomBar = { if (footer != null) TlFooterBar(content = footer) },
        snackbarHost = { if (snackbarHostState != null) SnackbarHost(snackbarHostState) },
    ) { inner ->
        val body: @Composable BoxScope.() -> Unit = {
            val column = Modifier
                .fillMaxWidth()
                .widthIn(max = 640.dp)
                .align(Alignment.TopCenter)
                .padding(horizontal = TlTheme.spacing.gutter)
                .padding(contentPadding)
            if (scrollable) {
                Column(
                    // Pull-to-refresh drives off nested scroll, so the body has to
                    // stay scrollable even when its content is shorter than the screen.
                    modifier = column.verticalScroll(rememberScrollState()),
                    verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.lg),
                    content = content,
                )
            } else {
                Column(
                    modifier = column,
                    verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.lg),
                    content = content,
                )
            }
        }

        if (onRefresh != null) {
            PullToRefreshBox(
                isRefreshing = refreshing,
                onRefresh = onRefresh,
                modifier = Modifier.padding(inner).fillMaxSize(),
                content = body,
            )
        } else {
            Box(modifier = Modifier.padding(inner).fillMaxSize(), content = body)
        }
    }
}

@OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)
@Composable
fun TlTopBar(
    title: String,
    modifier: Modifier = Modifier,
    subtitle: String? = null,
    onBack: (() -> Unit)? = null,
    actions: @Composable RowScope.() -> Unit = {},
) {
    TopAppBar(
        modifier = modifier,
        title = {
            Column {
                Text(
                    title,
                    style = MaterialTheme.typography.titleLarge,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                if (subtitle != null) {
                    Text(
                        subtitle,
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
            }
        },
        navigationIcon = {
            if (onBack != null) {
                IconButton(onClick = onBack) {
                    Icon(Icons.AutoMirrored.Outlined.ArrowBack, contentDescription = "Back")
                }
            }
        },
        actions = actions,
        colors = TopAppBarDefaults.topAppBarColors(
            containerColor = MaterialTheme.colorScheme.background,
            titleContentColor = TlTheme.colors.brand,
            // Bar actions are real controls, so they carry the action colour
            // rather than the grey Material hands out by default.
            actionIconContentColor = MaterialTheme.colorScheme.primary,
            navigationIconContentColor = MaterialTheme.colorScheme.primary,
        ),
    )
}

/** Sticky action area — the primary button lives here, never scrolled off-screen. */
@Composable
fun TlFooterBar(modifier: Modifier = Modifier, content: @Composable ColumnScope.() -> Unit) {
    Surface(
        modifier = modifier.fillMaxWidth(),
        color = MaterialTheme.colorScheme.surface,
        shadowElevation = 12.dp,
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .imePadding()
                .navigationBarsPadding()
                .padding(horizontal = TlTheme.spacing.gutter, vertical = TlTheme.spacing.md),
            verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm),
            content = content,
        )
    }
}

/** Circular icon affordance used in headers and hero blocks. */
@Composable
fun TlIconBubble(
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    modifier: Modifier = Modifier,
    tint: androidx.compose.ui.graphics.Color = MaterialTheme.colorScheme.primary,
    size: androidx.compose.ui.unit.Dp = 44.dp,
) {
    Surface(
        shape = com.techlane.pos.core.designsystem.theme.PillShape,
        color = MaterialTheme.colorScheme.primaryContainer,
        modifier = modifier.size(size),
    ) {
        Row(horizontalArrangement = Arrangement.Center, verticalAlignment = Alignment.CenterVertically) {
            Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                Icon(icon, contentDescription = null, tint = tint, modifier = Modifier.size(size * 0.5f))
            }
        }
    }
}
