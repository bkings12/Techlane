package com.techlane.pos.navigation

import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.consumeWindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.isImeVisible
import androidx.compose.foundation.layout.navigationBars
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ReceiptLong
import androidx.compose.material.icons.outlined.Build
import androidx.compose.material.icons.outlined.Home
import androidx.compose.material.icons.outlined.MoreHoriz
import androidx.compose.material.icons.outlined.QrCodeScanner
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavDestination.Companion.hierarchy
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import androidx.navigation.navArgument
import androidx.navigation.NavType
import com.techlane.pos.core.designsystem.theme.PillShape
import com.techlane.pos.core.designsystem.theme.TlTheme
import com.techlane.pos.domain.model.PhotoKind
import com.techlane.pos.feature.activity.ActivityScreen
import com.techlane.pos.feature.auth.LoginScreen
import com.techlane.pos.feature.camera.JobCameraScreen
import com.techlane.pos.feature.charge.QuickChargeScreen
import com.techlane.pos.feature.dashboard.DashboardScreen
import com.techlane.pos.feature.intake.IntakeScreen
import com.techlane.pos.feature.jobs.JobDetailsScreen
import com.techlane.pos.feature.jobs.JobDetailsViewModel
import com.techlane.pos.feature.jobs.JobsScreen
import com.techlane.pos.feature.more.MoreScreen
import com.techlane.pos.feature.scan.ScanScreen
import com.techlane.pos.feature.settings.SettingsScreen
import com.techlane.pos.feature.settings.printer.PrinterSettingsScreen
import com.techlane.pos.feature.update.AppUpdateViewModel
import com.techlane.pos.feature.update.UpdatePrompt

object Routes {
    const val LOGIN = "login"
    const val SHELL = "shell"
    const val HOME = "home"
    const val JOBS = "jobs?filter={filter}"
    const val SCAN = "scan"
    const val SALES = "sales"
    const val MORE = "more"
    const val HISTORY = "history"
    const val SETTINGS = "settings"
    const val PRINTER_SETTINGS = "settings/printer"
    const val INTAKE = "intake"
    const val JOB_DETAILS = "job/{jobId}"
    const val JOB_CAMERA = "job/{jobId}/camera/{kind}"

    fun jobs(filter: com.techlane.pos.domain.model.JobFilter? = null) =
        if (filter == null) "jobs?filter=" else "jobs?filter=${filter.name}"

    fun jobDetails(jobId: String) = "job/$jobId"
    fun jobCamera(jobId: String, kind: PhotoKind) = "job/$jobId/camera/${kind.wire}"
}

/**
 * Five destinations, no more. Scan sits in the middle because it is the fastest
 * route into a job from a physical slip, which is how most bench work starts.
 */
private enum class ShellTab(val route: String, val label: String, val icon: ImageVector) {
    Home(Routes.HOME, "Home", Icons.Outlined.Home),
    Jobs("jobs", "Jobs", Icons.Outlined.Build),
    Scan(Routes.SCAN, "Scan", Icons.Outlined.QrCodeScanner),
    Sales(Routes.SALES, "Sales", Icons.AutoMirrored.Outlined.ReceiptLong),
    More(Routes.MORE, "More", Icons.Outlined.MoreHoriz),
}

@Composable
fun PosApp(signedIn: Boolean, modifier: Modifier = Modifier) {
    val navController = rememberNavController()

    // Sits above the whole graph so a mandatory update cannot be walked around
    // by being on some other screen when it lands. Only checked while signed
    // in — an update nag over the login form helps nobody.
    if (signedIn) AppUpdateHost()

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
                onOpenJob = { navController.navigate(Routes.jobDetails(it)) },
                onNewIntake = { navController.navigate(Routes.INTAKE) },
            )
        }

        composable(Routes.INTAKE) {
            IntakeScreen(
                onBack = { navController.popBackStack() },
                onOpenJob = { jobId ->
                    // The booked job replaces the form in the stack: Back from
                    // Job Details returns to the board, never to a filled-in
                    // intake form that would book a duplicate if re-submitted.
                    navController.navigate(Routes.jobDetails(jobId)) {
                        popUpTo(Routes.INTAKE) { inclusive = true }
                        launchSingleTop = true
                    }
                },
            )
        }

        composable(
            route = Routes.JOB_DETAILS,
            arguments = listOf(navArgument("jobId") { type = NavType.StringType }),
        ) { entry ->
            val jobId = entry.arguments?.getString("jobId").orEmpty()
            JobDetailsScreen(
                onBack = { navController.popBackStack() },
                onTakePhoto = { kind -> navController.navigate(Routes.jobCamera(jobId, kind)) },
            )
        }

        composable(
            route = Routes.JOB_CAMERA,
            arguments = listOf(
                navArgument("jobId") { type = NavType.StringType },
                navArgument("kind") { type = NavType.StringType },
            ),
        ) { entry ->
            val kind = PhotoKind.fromWire(entry.arguments?.getString("kind"))
            // Scoped to the details entry so the captured photo lands on the same
            // ViewModel that is about to redisplay the gallery.
            val parentEntry = remember(entry) {
                navController.getBackStackEntry(Routes.jobDetails(entry.arguments?.getString("jobId").orEmpty()))
            }
            val detailsViewModel: JobDetailsViewModel =
                androidx.hilt.navigation.compose.hiltViewModel(parentEntry)
            JobCameraScreen(
                kind = kind,
                onCaptured = { file, caption ->
                    detailsViewModel.addPhoto(file, kind, caption)
                    navController.popBackStack()
                },
                onCancel = { navController.popBackStack() },
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
                onOpenPrinterSettings = { navController.navigate(Routes.PRINTER_SETTINGS) },
            )
        }

        composable(Routes.PRINTER_SETTINGS) {
            PrinterSettingsScreen(onBack = { navController.popBackStack() })
        }
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun AppShell(
    onOpenSettings: () -> Unit,
    onOpenJob: (String) -> Unit,
    onNewIntake: () -> Unit,
) {
    val tabController = rememberNavController()
    val backStack by tabController.currentBackStackEntryAsState()
    val currentRoute = backStack?.destination

    // The app draws edge to edge, so nothing lifts off the keyboard on its own.
    // Without this the charge button sits underneath the IME — which is exactly
    // where a technician cannot reach it.
    val imeVisible = WindowInsets.isImeVisible

    Column(
        Modifier
            .fillMaxSize()
            .imePadding(),
    ) {
        Box(
            Modifier
                .weight(1f)
                // The tab bar already clears the navigation bar; consuming the
                // inset here stops the screen's own footer padding for it twice.
                .then(
                    if (imeVisible) Modifier else Modifier.consumeWindowInsets(WindowInsets.navigationBars),
                ),
        ) {
            NavHost(
                navController = tabController,
                startDestination = Routes.HOME,
                enterTransition = { fadeIn(tween(140)) },
                exitTransition = { fadeOut(tween(100)) },
            ) {
                composable(Routes.HOME) {
                    DashboardScreen(
                        onOpenJobs = { filter -> tabController.navigateTab(Routes.jobs(filter)) },
                        onOpenJob = onOpenJob,
                        onScan = { tabController.navigateTab(Routes.SCAN) },
                        onNewSale = { tabController.navigateTab(Routes.SALES) },
                        onNewIntake = onNewIntake,
                        onOpenSettings = onOpenSettings,
                    )
                }
                composable(
                    route = Routes.JOBS,
                    arguments = listOf(
                        navArgument("filter") { type = NavType.StringType; defaultValue = "" },
                    ),
                ) {
                    JobsScreen(
                        onOpenJob = onOpenJob,
                        onScan = { tabController.navigateTab(Routes.SCAN) },
                        onNewIntake = onNewIntake,
                    )
                }
                composable(Routes.SCAN) { ScanScreen(onJobResolved = onOpenJob) }
                composable(Routes.SALES) { QuickChargeScreen(onOpenSettings = onOpenSettings) }
                composable(Routes.MORE) {
                    MoreScreen(
                        onOpenSettings = onOpenSettings,
                        onOpenChargeHistory = { tabController.navigateTab(Routes.HISTORY) },
                    )
                }
                composable(Routes.HISTORY) { ActivityScreen() }
            }
        }
        if (!imeVisible) {
            BottomBar(
                current = currentRoute?.route,
                onSelect = { tab -> tabController.navigateTab(tab.route) },
                isSelected = { tab ->
                    currentRoute?.hierarchy?.any { it.route?.substringBefore('?') == tab.route } == true
                },
            )
        }
    }
}

/**
 * Checks for a newer APK on launch and whenever the app is brought back to the
 * foreground; the repository throttles the actual request, so resuming all day
 * costs one call.
 */
@Composable
private fun AppUpdateHost(viewModel: AppUpdateViewModel = hiltViewModel()) {
    val update by viewModel.prompt.collectAsStateWithLifecycle()
    val lifecycleOwner = LocalLifecycleOwner.current

    DisposableEffect(lifecycleOwner) {
        val observer = LifecycleEventObserver { _, event ->
            if (event == Lifecycle.Event.ON_RESUME) viewModel.checkOnResume()
        }
        lifecycleOwner.lifecycle.addObserver(observer)
        onDispose { lifecycleOwner.lifecycle.removeObserver(observer) }
    }

    update?.let { available ->
        UpdatePrompt(
            update = available,
            installedVersionName = viewModel.installedVersionName,
            onDismiss = { viewModel.dismiss(available.versionCode) },
        )
    }
}

private fun NavHostController.navigateTab(route: String) {
    navigate(route) {
        popUpTo(graph.findStartDestination().id) { saveState = true }
        launchSingleTop = true
        restoreState = true
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
                        // A solid brand-tinted pill rather than a 12% wash of
                        // it — the selected tab has to be obvious at a glance
                        // on a bright shop floor.
                        color = if (selected) {
                            MaterialTheme.colorScheme.primaryContainer
                        } else {
                            androidx.compose.ui.graphics.Color.Transparent
                        },
                    ) {
                        Box(
                            modifier = Modifier.padding(horizontal = TlTheme.spacing.lg, vertical = TlTheme.spacing.xs),
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
                        // Weight as well as colour: on a bright counter the
                        // selected tab has to survive glare and a glance.
                        fontWeight = if (selected) FontWeight.Bold else FontWeight.Medium,
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
