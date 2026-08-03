package com.techlane.pos.navigation

import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Bolt
import androidx.compose.material.icons.outlined.Build
import androidx.compose.material.icons.automirrored.outlined.ReceiptLong
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.unit.dp
import androidx.navigation.NavDestination.Companion.hierarchy
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import com.techlane.pos.core.designsystem.component.TlEmptyState
import com.techlane.pos.core.designsystem.component.TlScreen
import com.techlane.pos.core.designsystem.theme.PillShape
import com.techlane.pos.core.designsystem.theme.TlTheme
import com.techlane.pos.feature.activity.ActivityScreen
import com.techlane.pos.feature.auth.LoginScreen
import com.techlane.pos.feature.charge.QuickChargeScreen
import com.techlane.pos.feature.settings.SettingsScreen

object Routes {
    const val LOGIN = "login"
    const val SHELL = "shell"
    const val CHARGE = "charge"
    const val JOBS = "jobs"
    const val ACTIVITY = "activity"
    const val SETTINGS = "settings"
}

/** Tabs that exist today. Repairs/inventory modules slot in beside these. */
private enum class ShellTab(val route: String, val label: String, val icon: ImageVector) {
    Charge(Routes.CHARGE, "Charge", Icons.Outlined.Bolt),
    Jobs(Routes.JOBS, "Jobs", Icons.Outlined.Build),
    Activity(Routes.ACTIVITY, "Activity", Icons.AutoMirrored.Outlined.ReceiptLong),
}

@Composable
fun PosApp(signedIn: Boolean, modifier: Modifier = Modifier) {
    val navController = rememberNavController()

    NavHost(
        navController = navController,
        startDestination = if (signedIn) Routes.SHELL else Routes.LOGIN,
        modifier = modifier.fillMaxSize(),
        enterTransition = { fadeIn(tween(180)) + slideInHorizontally(tween(220)) { it / 12 } },
        exitTransition = { fadeOut(tween(140)) },
        popEnterTransition = { fadeIn(tween(180)) },
        popExitTransition = { fadeOut(tween(140)) + slideOutHorizontally(tween(200)) { it / 12 } },
    ) {
        composable(Routes.LOGIN) {
            LoginScreen(
                onSignedIn = {
                    navController.navigate(Routes.SHELL) {
                        popUpTo(Routes.LOGIN) { inclusive = true }
                    }
                },
            )
        }

        composable(Routes.SHELL) {
            AppShell(
                onOpenSettings = { navController.navigate(Routes.SETTINGS) },
            )
        }

        composable(Routes.SETTINGS) {
            SettingsScreen(
                onBack = { navController.popBackStack() },
                onSignedOut = {
                    navController.navigate(Routes.LOGIN) {
                        popUpTo(0) { inclusive = true }
                    }
                },
            )
        }
    }
}

@Composable
private fun AppShell(onOpenSettings: () -> Unit) {
    val tabController = rememberNavController()
    val backStack by tabController.currentBackStackEntryAsState()
    val currentRoute = backStack?.destination

    Column(Modifier.fillMaxSize()) {
        Box(Modifier.weight(1f)) {
            NavHost(
                navController = tabController,
                startDestination = Routes.CHARGE,
                enterTransition = { fadeIn(tween(140)) },
                exitTransition = { fadeOut(tween(100)) },
            ) {
                composable(Routes.CHARGE) { QuickChargeScreen(onOpenSettings = onOpenSettings) }
                composable(Routes.ACTIVITY) { ActivityScreen() }
                composable(Routes.JOBS) {
                    TlScreen(title = "Jobs", subtitle = "Repairs board") {
                        TlEmptyState(
                            title = "Repairs land here next",
                            subtitle = "Intake, diagnosis, parts and handover will use the same shell and " +
                                "components as the charge screen.",
                            icon = Icons.Outlined.Build,
                        )
                    }
                }
            }
        }
        BottomBar(
            current = currentRoute?.route,
            onSelect = { tab ->
                tabController.navigate(tab.route) {
                    popUpTo(tabController.graph.findStartDestination().id) { saveState = true }
                    launchSingleTop = true
                    restoreState = true
                }
            },
            isSelected = { tab -> currentRoute?.hierarchy?.any { it.route == tab.route } == true },
        )
    }
}

@Composable
private fun BottomBar(
    current: String?,
    onSelect: (ShellTab) -> Unit,
    isSelected: (ShellTab) -> Boolean,
) {
    Surface(color = MaterialTheme.colorScheme.surface, shadowElevation = 8.dp) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .navigationBarsPadding()
                .height(TlTheme.sizes.bottomBarHeight),
            horizontalArrangement = Arrangement.SpaceEvenly,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            ShellTab.entries.forEach { tab ->
                val selected = isSelected(tab) || current == tab.route
                Column(
                    modifier = Modifier
                        .weight(1f)
                        .padding(vertical = TlTheme.spacing.sm),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.xxs),
                ) {
                    Surface(
                        onClick = { onSelect(tab) },
                        shape = PillShape,
                        color = if (selected) {
                            MaterialTheme.colorScheme.primary.copy(alpha = 0.12f)
                        } else {
                            androidx.compose.ui.graphics.Color.Transparent
                        },
                    ) {
                        Box(
                            modifier = Modifier.padding(horizontal = TlTheme.spacing.xl, vertical = TlTheme.spacing.xs),
                            contentAlignment = Alignment.Center,
                        ) {
                            Icon(
                                tab.icon,
                                contentDescription = tab.label,
                                tint = if (selected) {
                                    MaterialTheme.colorScheme.primary
                                } else {
                                    MaterialTheme.colorScheme.onSurfaceVariant
                                },
                                modifier = Modifier.size(TlTheme.sizes.icon),
                            )
                        }
                    }
                    Text(
                        tab.label,
                        style = MaterialTheme.typography.labelSmall,
                        color = if (selected) {
                            MaterialTheme.colorScheme.primary
                        } else {
                            MaterialTheme.colorScheme.onSurfaceVariant
                        },
                    )
                }
            }
        }
    }
}
