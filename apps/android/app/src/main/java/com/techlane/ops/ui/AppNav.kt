package com.techlane.ops.ui

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.relocation.BringIntoViewRequester
import androidx.compose.foundation.relocation.bringIntoViewRequester
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.compose.BackHandler
import androidx.activity.result.contract.ActivityResultContracts
import android.util.Base64
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.automirrored.filled.Logout
import androidx.compose.material.icons.filled.AccountBalance
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Bolt
import androidx.compose.material.icons.filled.Build
import androidx.compose.material.icons.filled.DateRange
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.Inventory2
import androidx.compose.material.icons.filled.MoreHoriz
import androidx.compose.material.icons.filled.Payments
import androidx.compose.material.icons.filled.Place
import androidx.compose.material.icons.filled.PointOfSale
import androidx.compose.material.icons.filled.QrCodeScanner
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Search
import androidx.compose.material.icons.filled.Notifications
import androidx.compose.material.icons.filled.Sync
import androidx.compose.material.icons.filled.Visibility
import androidx.compose.material.icons.filled.VisibilityOff
import androidx.compose.material.icons.outlined.Handyman
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.DatePicker
import androidx.compose.material3.DatePickerDialog
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.FilterChipDefaults
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.NavigationBarItemDefaults
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TimePicker
import androidx.compose.material3.rememberDatePickerState
import androidx.compose.material3.rememberTimePickerState
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import com.techlane.ops.BuildConfig
import com.techlane.ops.TechLaneApp
import com.techlane.ops.data.SyncCommandEntity
import com.techlane.ops.network.ApiClient
import com.techlane.ops.sync.OutboxFlush
import com.techlane.ops.sync.OutboxRepository
import com.techlane.ops.sync.SyncCommandTypes
import com.techlane.core.PrintSupport
import com.techlane.core.scan.CameraPermissionGate
import com.techlane.core.scan.ScanCameraPanel
import com.techlane.core.scan.parseScanPayload
import com.techlane.core.theme.Brand
import com.techlane.core.theme.statusPalette
import com.techlane.core.ui.BrandAuthHeader
import com.techlane.core.ui.BrandCard
import com.techlane.core.ui.BrandDetailHeader
import com.techlane.core.ui.BrandHero
import com.techlane.core.ui.BrandSectionTitle
import com.techlane.core.ui.GoldButton
import com.techlane.core.ui.HeroStat
import com.techlane.core.ui.LocalWindowLayout
import com.techlane.core.ui.PleaseWaitOverlay
import com.techlane.core.ui.SafeBottomBar
import com.techlane.core.ui.PillBadge
import com.techlane.core.ui.brandGradient
import com.techlane.core.media.PhotoCapture
import com.techlane.core.update.AppUpdateSettingsPanel
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import java.time.Instant
import java.time.LocalDate
import java.time.LocalTime
import java.time.ZoneOffset
import java.time.format.DateTimeFormatter
import java.util.UUID

fun timeAgo(iso: String): String {
    val instant = runCatching { java.time.OffsetDateTime.parse(iso).toInstant() }
        .recoverCatching { Instant.parse(iso) }
        .getOrNull() ?: return ""
    val mins = java.time.Duration.between(instant, Instant.now()).toMinutes()
    return when {
        mins < 1 -> "just now"
        mins < 60 -> "${mins}m ago"
        mins < 60 * 24 -> "${mins / 60}h ago"
        else -> "${mins / (60 * 24)}d ago"
    }
}

private data class OpsShellState(
    val branches: List<BranchOption>,
    val selectedBranchId: String?,
    val onBranchSelected: (String) -> Unit,
    val profileError: String?,
)

private val LocalOpsShell = staticCompositionLocalOf<OpsShellState?> { null }

/** Shared snackbar host for transient confirmations (e.g. "Cash recorded") — provided
 * once at the shell level so any screen can surface a message without its own Scaffold. */
internal val LocalSnackbarHost = staticCompositionLocalOf<SnackbarHostState?> { null }

@Composable
internal fun OpsShellChrome(modifier: Modifier = Modifier) {
    val shell = LocalOpsShell.current
    Column(modifier.fillMaxWidth()) {
        ConnectivityBanner()
        if (shell != null && shell.branches.size > 1) {
            BranchPicker(
                branches = shell.branches,
                selectedBranchId = shell.selectedBranchId,
                onSelected = shell.onBranchSelected,
            )
        }
        shell?.profileError?.let {
            Text(it, color = Brand.Danger, modifier = Modifier.padding(horizontal = 16.dp, vertical = 6.dp))
        }
    }
}

@Composable
fun AppNav(
    signedIn: Boolean,
    onSignedIn: () -> Unit,
    onSignedOut: () -> Unit,
    modifier: Modifier = Modifier,
) {
    var sessionExpired by remember { mutableStateOf(false) }
    DisposableEffect(onSignedOut) {
        ApiClient.setSessionExpiredListener {
            android.os.Handler(android.os.Looper.getMainLooper()).post {
                sessionExpired = true
                onSignedOut()
            }
        }
        onDispose { ApiClient.setSessionExpiredListener(null) }
    }
    if (!signedIn) {
        LoginScreen(
            onSignedIn = {
                sessionExpired = false
                onSignedIn()
            },
            sessionExpired = sessionExpired,
            modifier = modifier,
        )
    } else {
        MainTabs(onSignedOut = onSignedOut, modifier = modifier)
    }
}

@Composable
fun LoginScreen(onSignedIn: () -> Unit, sessionExpired: Boolean = false, modifier: Modifier = Modifier) {
    var email by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var passwordVisible by remember { mutableStateOf(false) }
    var mfaCode by remember { mutableStateOf("") }
    var mfaChallenge by remember { mutableStateOf<String?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var notice by remember { mutableStateOf<String?>(if (sessionExpired) "Your session expired. Please sign in again." else null) }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()

    fun persistAndEnter(result: ApiClient.LoginResult.Ok) {
        TechLaneApp.instance.tokenStore.accessToken = result.accessToken
        TechLaneApp.instance.tokenStore.refreshToken = result.refreshToken
    }

    Box(
        modifier = modifier
            .fillMaxSize()
            .background(brandGradient())
            .imePadding(),
    ) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .statusBarsPadding(),
        ) {
            BrandAuthHeader(
                appLabel = "Ops",
                tagline = "Run your repair shop from your pocket",
                modifier = Modifier
                    .then(
                        if (LocalWindowLayout.current.compactChrome) Modifier
                        else Modifier.weight(1f),
                    )
                    .padding(horizontal = 28.dp)
                    .padding(top = 24.dp, bottom = if (LocalWindowLayout.current.compactChrome) 12.dp else 0.dp),
            )
            Surface(
                shape = RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp),
                color = Brand.Surface,
                modifier = Modifier.fillMaxWidth(),
            ) {
                Column(
                    modifier = Modifier
                        .verticalScroll(rememberScrollState())
                        .padding(24.dp),
                    verticalArrangement = Arrangement.spacedBy(14.dp),
                ) {
                    notice?.let {
                        Text(it, color = Brand.Warning, style = MaterialTheme.typography.bodySmall)
                    }
                    if (mfaChallenge == null) {
                        OutlinedTextField(
                            value = email,
                            onValueChange = { email = it },
                            label = { Text("Email") },
                            singleLine = true,
                            modifier = Modifier.fillMaxWidth(),
                        )
                        OutlinedTextField(
                            value = password,
                            onValueChange = { password = it },
                            label = { Text("Password") },
                            visualTransformation = if (passwordVisible) VisualTransformation.None else PasswordVisualTransformation(),
                            trailingIcon = {
                                IconButton(onClick = { passwordVisible = !passwordVisible }) {
                                    Icon(
                                        imageVector = if (passwordVisible) Icons.Filled.VisibilityOff else Icons.Filled.Visibility,
                                        contentDescription = if (passwordVisible) "Hide password" else "Show password",
                                    )
                                }
                            },
                            singleLine = true,
                            modifier = Modifier.fillMaxWidth(),
                        )
                    } else {
                        Text(
                            "Enter the 6-digit code from your authenticator app.",
                            style = MaterialTheme.typography.bodyMedium,
                            color = Brand.TextPrimary,
                        )
                        OutlinedTextField(
                            value = mfaCode,
                            onValueChange = { mfaCode = it.filter { ch -> ch.isDigit() }.take(8) },
                            label = { Text("Authentication code") },
                            singleLine = true,
                            modifier = Modifier.fillMaxWidth(),
                        )
                    }
                    FeedbackBanner(message = null, error = error)
                    GoldButton(
                        text = if (mfaChallenge == null) "Sign in" else "Verify",
                        onClick = {
                            busy = true
                            error = null
                            notice = null
                            scope.launch {
                                try {
                                    val challenge = mfaChallenge
                                    val result = withContext(Dispatchers.IO) {
                                        if (challenge == null) {
                                            ApiClient.login(email.trim(), password)
                                        } else {
                                            ApiClient.verifyMfa(challenge, mfaCode.trim())
                                        }
                                    }
                                    when (result) {
                                        is ApiClient.LoginResult.MfaRequired -> {
                                            mfaChallenge = result.challenge
                                            mfaCode = ""
                                            notice = "Two-factor authentication required."
                                        }
                                        is ApiClient.LoginResult.Ok -> {
                                            persistAndEnter(result)
                                            withContext(Dispatchers.IO) {
                                                runCatching { ApiClient.registerDevice() }
                                            }
                                            onSignedIn()
                                        }
                                    }
                                } catch (e: Exception) {
                                    error = e.message
                                } finally {
                                    busy = false
                                }
                            }
                        },
                        enabled = !busy && (
                            if (mfaChallenge == null) email.isNotBlank() && password.isNotBlank()
                            else mfaCode.length >= 6
                            ),
                        loading = busy,
                        modifier = Modifier.fillMaxWidth(),
                    )
                    if (mfaChallenge != null) {
                        TextButton(
                            onClick = {
                                mfaChallenge = null
                                mfaCode = ""
                                error = null
                                notice = null
                            },
                            enabled = !busy,
                        ) {
                            Text("Back to sign in", color = Brand.Navy)
                        }
                    }
                }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun MainTabs(onSignedOut: () -> Unit, modifier: Modifier = Modifier) {
    var tab by remember { mutableStateOf("home") }
    var pendingJobId by remember { mutableStateOf<String?>(null) }
    var roles by remember { mutableStateOf<Set<String>>(emptySet()) }
    var permissions by remember { mutableStateOf<Set<String>>(emptySet()) }
    var branches by remember { mutableStateOf<List<BranchOption>>(emptyList()) }
    var selectedBranchId by remember { mutableStateOf(TechLaneApp.instance.tokenStore.selectedBranchId) }
    var profileError by remember { mutableStateOf<String?>(null) }
    var tabBootstrapped by remember { mutableStateOf(false) }
    val snackbarHostState = remember { SnackbarHostState() }

    LaunchedEffect(Unit) {
        try {
            val me = withContext(Dispatchers.IO) { ApiClient.me() }
            val serverRoles = me.optJSONArray("roles")?.let { a ->
                (0 until a.length()).map { a.getString(it) }.toSet()
            } ?: emptySet()
            roles = serverRoles
            permissions = me.optJSONArray("permissions")?.let { a ->
                (0 until a.length()).map { a.getString(it) }.toSet()
            } ?: emptySet()
            val allowedIds = me.optJSONArray("branch_ids")?.let { a ->
                (0 until a.length()).map { a.getString(it) }
            } ?: emptyList()
            val serverBranches = runCatching { withContext(Dispatchers.IO) { ApiClient.listBranches() } }.getOrNull()
            val names = if (serverBranches != null) {
                (0 until serverBranches.length()).map { serverBranches.getJSONObject(it) }
                    .associate { it.optString("id") to it.optString("name") }
            } else emptyMap()
            // Technicians may work across every branch they are assigned to.
            branches = allowedIds.map { BranchOption(it, names[it] ?: "Branch ${it.take(8)}") }
            if (selectedBranchId !in allowedIds) {
                selectedBranchId = allowedIds.firstOrNull()
                TechLaneApp.instance.tokenStore.selectedBranchId = selectedBranchId
            }
            profileError = null
        } catch (e: Exception) {
            profileError = e.message
        }
    }

    val owner = "owner" in roles || "*" in permissions
    val isTechnician = "technician" in roles && !owner
    val isCashier = "cashier" in roles && !owner
    val canCollectRepair = owner || "repairs.collect" in permissions
    val canCloseRepair = owner || "repairs.close" in permissions
    val canAuthorizeWork = owner || "repairs.authorize_work" in permissions
    val canReleaseUnverifiedRepair = owner || "repairs.release_unverified" in permissions
    val canTakePayment = owner || "cashier" in roles || "payments.initiate" in permissions ||
        permissions.any { it.startsWith("cash.") || it.startsWith("payments.") }
    val canRepair = owner || "technician" in roles || permissions.any { it.startsWith("repairs.") || it.startsWith("parts.") }
    val canSell = owner || "cashier" in roles || "sales.create" in permissions
    val canCash = canTakePayment
    val canInventory = owner || "inventory" in roles || "inventory.read" in permissions
    val tabs = buildList {
        add("home")
        if (canRepair) add("jobs")
        if (canSell) add("pos")
        add("notifications")
        if (canInventory || canSell) add("inventory")
        if (canCash) add("cash")
        if (canSell) add("pickup")
        if (canCash) add("c2b")
        if (canRepair || canSell) add("scan")
        if (canRepair && canTakePayment) add("quick_repair")
        add("search")
        add("sync")
    }
    // Technicians land on the jobs board — their primary workspace.
    LaunchedEffect(isTechnician, canRepair, roles) {
        if (!tabBootstrapped && roles.isNotEmpty()) {
            if (isTechnician && canRepair) tab = "jobs"
            tabBootstrapped = true
        }
    }
    val primaryTabs = tabs.take(4)
    val moreTabs = tabs.drop(4)
    var showMore by remember { mutableStateOf(false) }
    val moreSheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    LaunchedEffect(tabs) {
        if (tab !in tabs) tab = tabs.first()
    }
    fun label(key: String) = when (key) {
        "home" -> "Home"
        "jobs" -> "Jobs"
        "pos" -> "Sell"
        "inventory" -> "Stock"
        "cash" -> "Cash"
        "pickup" -> "Pickup"
        "c2b" -> "C2B"
        "scan" -> "Scan"
        "quick_repair" -> "Quick fix"
        "search" -> "Search"
        "notifications" -> "Inbox"
        else -> "Sync"
    }
    fun tabIcon(key: String) = when (key) {
        "jobs" -> Icons.Outlined.Handyman
        "pos" -> Icons.Filled.PointOfSale
        "inventory" -> Icons.Filled.Inventory2
        "cash" -> Icons.Filled.Payments
        "pickup" -> Icons.Filled.Place
        "c2b" -> Icons.Filled.AccountBalance
        "scan" -> Icons.Filled.QrCodeScanner
        "quick_repair" -> Icons.Filled.Bolt
        "search" -> Icons.Filled.Search
        "notifications" -> Icons.Filled.Notifications
        "sync" -> Icons.Filled.Sync
        else -> Icons.Filled.Home
    }
    val navItemColors = NavigationBarItemDefaults.colors(
        selectedIconColor = Brand.Navy,
        selectedTextColor = Brand.Navy,
        indicatorColor = Brand.NavyTint,
        unselectedIconColor = Brand.TextMuted,
        unselectedTextColor = Brand.TextMuted,
    )
    val windowLayout = LocalWindowLayout.current
    val useSideNav = windowLayout.useSideNav
    val compactChrome = windowLayout.compactChrome

    @Composable
    fun TabBody(contentModifier: Modifier) {
        CompositionLocalProvider(
            LocalSnackbarHost provides snackbarHostState,
            LocalOpsShell provides OpsShellState(
                branches = branches,
                selectedBranchId = selectedBranchId,
                onBranchSelected = {
                    selectedBranchId = it
                    TechLaneApp.instance.tokenStore.selectedBranchId = it
                },
                profileError = profileError,
            ),
        ) {
            Box(modifier = contentModifier.fillMaxSize()) {
                when (tab) {
                    "home" -> HomeScreen(
                        isTechnician = isTechnician,
                        isCashier = isCashier,
                        isOwner = owner,
                        canSell = canSell,
                        canCash = canCash,
                        canRepair = canRepair,
                        onNavigate = { destination -> if (destination in tabs) tab = destination },
                        onOpenJobs = { tab = if (canRepair) "jobs" else "pos" },
                        onOpenJob = { id ->
                            pendingJobId = id
                            tab = "jobs"
                        },
                    )
                    "jobs" -> JobsScreen(
                        isTechnician = isTechnician,
                        canCollect = canCollectRepair,
                        canClose = canCloseRepair,
                        canAuthorize = canAuthorizeWork,
                        canReleaseUnverified = canReleaseUnverifiedRepair,
                        canTakePayment = canTakePayment,
                        defaultMine = isTechnician,
                        openJobId = pendingJobId,
                        onOpenJobConsumed = { pendingJobId = null },
                        onOpenPos = { if (canSell) tab = "pos" },
                    )
                    "pos" -> PosScreen(selectedBranchId)
                    "inventory" -> InventoryLookupScreen(selectedBranchId)
                    "cash" -> CashScreen(selectedBranchId)
                    "pickup" -> PickupScreen()
                    "c2b" -> C2BExceptionsScreen()
                    "search" -> UniversalSearchScreen(
                        onOpenJob = { id -> pendingJobId = id; tab = "jobs" },
                        onNavigate = { destination -> if (destination in tabs) tab = destination },
                    )
                    "notifications" -> NotificationCenterScreen(
                        onOpenJob = { id -> pendingJobId = id; tab = "jobs" },
                    )
                    "scan" -> ManualScanScreen(
                        onOpenJob = { id ->
                            pendingJobId = id
                            tab = "jobs"
                        },
                    )
                    "quick_repair" -> QuickRepairScreen(selectedBranchId)
                    else -> SyncCenterScreen()
                }
            }
        }
    }

    Box(Modifier.fillMaxSize()) {
    if (useSideNav) {
        Row(
            modifier
                .fillMaxSize()
                .background(MaterialTheme.colorScheme.background),
        ) {
            // Slim icon rail — not Material NavigationRail (looks like stacked buttons).
            Column(
                modifier = Modifier
                    .fillMaxHeight()
                    .width(64.dp)
                    .background(Brand.Navy)
                    .statusBarsPadding()
                    .navigationBarsPadding()
                    .padding(vertical = 10.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(4.dp),
            ) {
                primaryTabs.forEach { key ->
                    val selected = tab == key
                    IconButton(
                        onClick = { tab = key },
                        modifier = Modifier
                            .size(48.dp)
                            .background(
                                if (selected) Brand.Gold.copy(alpha = 0.22f) else Color.Transparent,
                                RoundedCornerShape(12.dp),
                            ),
                    ) {
                        Icon(
                            tabIcon(key),
                            contentDescription = label(key),
                            tint = if (selected) Brand.Gold else Color.White.copy(alpha = 0.72f),
                        )
                    }
                }
                Spacer(modifier = Modifier.weight(1f))
                IconButton(
                    onClick = { showMore = true },
                    modifier = Modifier
                        .size(48.dp)
                        .background(
                            if (tab in moreTabs) Brand.Gold.copy(alpha = 0.22f) else Color.Transparent,
                            RoundedCornerShape(12.dp),
                        ),
                ) {
                    Icon(
                        Icons.Filled.MoreHoriz,
                        contentDescription = "More",
                        tint = if (tab in moreTabs) Brand.Gold else Color.White.copy(alpha = 0.72f),
                    )
                }
            }
            TabBody(
                Modifier
                    .weight(1f)
                    .fillMaxHeight(),
            )
        }
    } else {
        Scaffold(
            modifier = modifier,
            containerColor = MaterialTheme.colorScheme.background,
            // Scaffold does not consume system insets; SafeBottomBar pads above
            // the 3-button/gesture bar (with a fallback if WindowInsets is 0).
            contentWindowInsets = WindowInsets(0, 0, 0, 0),
            bottomBar = {
                SafeBottomBar(containerColor = Color.White) {
                    NavigationBar(
                        containerColor = Color.White,
                        tonalElevation = 0.dp,
                        // Insets handled by SafeBottomBar — avoid double padding.
                        windowInsets = WindowInsets(0, 0, 0, 0),
                    ) {
                        primaryTabs.forEach { key ->
                            NavigationBarItem(
                                selected = tab == key,
                                onClick = { tab = key },
                                icon = { Icon(tabIcon(key), contentDescription = label(key)) },
                                label = { Text(label(key), style = MaterialTheme.typography.labelSmall) },
                                colors = navItemColors,
                                alwaysShowLabel = !compactChrome,
                            )
                        }
                        NavigationBarItem(
                            selected = tab in moreTabs,
                            onClick = { showMore = true },
                            icon = { Icon(Icons.Filled.MoreHoriz, contentDescription = "More") },
                            label = { Text("More", style = MaterialTheme.typography.labelSmall) },
                            colors = navItemColors,
                            alwaysShowLabel = !compactChrome,
                        )
                    }
                }
            },
        ) { padding ->
            // Fresh Modifier — do not reuse the outer fillMaxSize modifier or padding
            // can be applied in the wrong order and leave actions under the tab bar.
            TabBody(
                Modifier
                    .fillMaxSize()
                    .padding(bottom = padding.calculateBottomPadding()),
            )
        }
    }
        SnackbarHost(snackbarHostState, modifier = Modifier.align(Alignment.BottomCenter))
    }
    if (showMore) {
            ModalBottomSheet(
                onDismissRequest = { showMore = false },
                sheetState = moreSheetState,
            ) {
                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = 16.dp, vertical = 8.dp),
                    verticalArrangement = Arrangement.spacedBy(4.dp),
                ) {
                    Text("More tools", style = MaterialTheme.typography.titleMedium)
                    Text(
                        (if (roles.isEmpty()) "Staff account" else roles.joinToString(", ").replaceFirstChar { it.uppercase() }) +
                            (branches.firstOrNull { it.id == selectedBranchId }?.let { " · " + it.name } ?: ""),
                        color = Brand.TextSecondary,
                        style = MaterialTheme.typography.bodySmall,
                    )
                    moreTabs.forEach { key ->
                        TextButton(
                            onClick = {
                                tab = key
                                showMore = false
                            },
                            modifier = Modifier.fillMaxWidth(),
                        ) {
                            Icon(tabIcon(key), contentDescription = null, tint = Brand.Navy)
                            Spacer(Modifier.width(12.dp))
                            Text(label(key), modifier = Modifier.weight(1f), color = Brand.Navy)
                        }
                    }
                    HorizontalDivider(
                        modifier = Modifier.padding(vertical = 8.dp),
                        color = Brand.Border,
                    )
                    AppUpdateSettingsPanel(
                        apiBase = BuildConfig.API_BASE,
                        appKey = "ops",
                        currentVersionCode = BuildConfig.VERSION_CODE,
                        currentVersionName = BuildConfig.VERSION_NAME,
                    )
                    HorizontalDivider(
                        modifier = Modifier.padding(vertical = 8.dp),
                        color = Brand.Border,
                    )
                    TextButton(
                        onClick = {
                            showMore = false
                            onSignedOut()
                        },
                        modifier = Modifier.fillMaxWidth(),
                    ) {
                        Icon(
                            Icons.AutoMirrored.Filled.Logout,
                            contentDescription = null,
                            tint = Brand.Danger,
                        )
                        Spacer(Modifier.width(12.dp))
                        Text("Sign out", modifier = Modifier.weight(1f), color = Brand.Danger)
                    }
                    Spacer(Modifier.height(16.dp))
                }
            }
    }
}

@Composable
fun HomeScreen(
    isTechnician: Boolean = false,
    isCashier: Boolean = false,
    isOwner: Boolean = false,
    canSell: Boolean = false,
    canCash: Boolean = false,
    canRepair: Boolean = false,
    onNavigate: (String) -> Unit = {},
    onOpenJobs: () -> Unit,
    onOpenJob: (String) -> Unit = {},
    modifier: Modifier = Modifier,
) {
    var displayName by remember { mutableStateOf("") }
    var myId by remember { mutableStateOf("") }
    var myJobs by remember { mutableStateOf(0) }
    var myOpenJobs by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var unassigned by remember { mutableStateOf(0) }
    var counts by remember { mutableStateOf<Map<String, Int>>(emptyMap()) }
    var collectedCount by remember { mutableStateOf(0) }
    var collectedCreditJobs by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var summary by remember { mutableStateOf<JSONObject?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var showMoreMetrics by remember { mutableStateOf(false) }
    var unmatchedPayments by remember { mutableStateOf(0) }
    var pendingCash by remember { mutableStateOf(0.0) }
    var readyOrders by remember { mutableStateOf(0) }
    var quickOpenCount by remember { mutableStateOf(0) }
    var unreadNotifications by remember { mutableStateOf(0) }
    val scope = rememberCoroutineScope()

    fun refresh() {
        scope.launch {
            try {
                val me = withContext(Dispatchers.IO) { ApiClient.me() }
                displayName = me.optString("display_name")
                myId = me.optString("id")
                val items = withContext(Dispatchers.IO) { ApiClient.listRepairs() }
                val all = (0 until items.length()).map { items.getJSONObject(it) }
                counts = all.groupingBy { it.optString("status") }.eachCount()
                val collectedItems = withContext(Dispatchers.IO) { ApiClient.listRepairs(status = "collected") }
                collectedCount = collectedItems.length()
                collectedCreditJobs = (0 until collectedItems.length()).map { collectedItems.getJSONObject(it) }.filter {
                    it.optBoolean("customer_credit") && it.optDouble("balance_due", 0.0) > 0.009
                }
                val openStatuses = setOf("intake", "diagnosed", "waiting_parts", "in_progress", "ready_for_pickup")
                myOpenJobs = all.filter {
                    it.optString("technician_id") == myId && it.optString("status") in openStatuses
                }
                myJobs = myOpenJobs.size
                quickOpenCount = all.count { it.optString("service_type") == "quick_replacement" && it.optString("status") in openStatuses }
                unassigned = all.count {
                    it.optString("technician_id").isBlank() && it.optString("status") in openStatuses
                }
                error = null
            } catch (e: Exception) {
                error = e.message
            }
            // Money summary needs reports.read; hide quietly for techs without it.
            summary = if (isTechnician) {
                null
            } else {
                runCatching { withContext(Dispatchers.IO) { ApiClient.reportSummary(1) } }.getOrNull()
            }
            if (canCash) {
                unmatchedPayments = runCatching { withContext(Dispatchers.IO) { ApiClient.listC2B("unmatched").length() } }.getOrDefault(0)
                pendingCash = runCatching { withContext(Dispatchers.IO) { ApiClient.pendingCashTotal() } }.getOrDefault(0.0)
            }
            unreadNotifications = runCatching { withContext(Dispatchers.IO) { ApiClient.listNotifications(true).length() } }.getOrDefault(0)
            if (canSell) {
                readyOrders = runCatching { withContext(Dispatchers.IO) { ApiClient.listOnlineOrders("ready_for_pickup").length() } }.getOrDefault(0)
            }
        }
    }

    LaunchedEffect(isTechnician, isCashier, isOwner, canSell, canCash) { refresh() }
    val statuses = statusPalette()
    val openCount = listOf("intake", "diagnosed", "waiting_parts", "in_progress", "ready_for_pickup")
        .sumOf { counts[it] ?: 0 }
    val readyCount = (counts["completed"] ?: 0) + (counts["ready_for_pickup"] ?: 0)
    val waitingCount = counts["waiting_parts"] ?: 0

    Column(
        modifier = modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background),
    ) {
        BrandHero(
            title = if (displayName.isBlank()) "Welcome" else "Hi, $displayName",
            subtitle = when {
                isTechnician -> "Your bench and next workshop actions"
                isCashier -> "Sales, payments and customer pickups"
                isOwner -> "Today’s shop priorities and exceptions"
                else -> "Today’s operational priorities"
            },
            appLabel = "Ops",
            trailing = {
                IconButton(onClick = { refresh() }) {
                    Icon(Icons.Filled.Refresh, "Refresh", tint = Color.White)
                }
            },
            bottomContent = {
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    if (isTechnician) {
                        HeroStat("My open", myJobs.toString(), Modifier.weight(1f))
                        HeroStat(
                            "QC",
                            myOpenJobs.count { it.optString("status") == "ready_for_pickup" }.toString(),
                            Modifier.weight(1f),
                        )
                        HeroStat(
                            "Parts",
                            myOpenJobs.count { it.optString("status") == "waiting_parts" }.toString(),
                            Modifier.weight(1f),
                        )
                    } else {
                        HeroStat("Open", openCount.toString(), Modifier.weight(1f))
                        HeroStat("Ready", readyCount.toString(), Modifier.weight(1f))
                        HeroStat("Collected", collectedCount.toString(), Modifier.weight(1f))
                    }
                }
            },
        )
        OpsShellChrome()
        Column(
            modifier = Modifier
                .weight(1f)
                .verticalScroll(rememberScrollState())
                .padding(16.dp)
                .padding(bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
        FeedbackBanner(message = null, error = error)

        if (isTechnician) {
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp), modifier = Modifier.fillMaxWidth()) {
                MetricTile("My open", myJobs.toString(), tileModifier = Modifier.weight(1f), accent = statuses.inProgress)
                MetricTile("Unassigned", unassigned.toString(), tileModifier = Modifier.weight(1f), accent = statuses.intake)
            }
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp), modifier = Modifier.fillMaxWidth()) {
                MetricTile(
                    "Waiting parts",
                    myOpenJobs.count { it.optString("status") == "waiting_parts" }.toString(),
                    tileModifier = Modifier.weight(1f),
                    accent = statuses.waitingParts,
                )
                MetricTile(
                    "In QC",
                    myOpenJobs.count { it.optString("status") == "ready_for_pickup" }.toString(),
                    tileModifier = Modifier.weight(1f),
                    accent = statuses.completed,
                )
            }
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp), modifier = Modifier.fillMaxWidth()) {
                MetricTile(
                    "In progress",
                    myOpenJobs.count { it.optString("status") == "in_progress" }.toString(),
                    tileModifier = Modifier.weight(1f),
                    accent = statuses.inProgress,
                )
            }

            BrandSectionTitle("Quick actions")
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp), modifier = Modifier.fillMaxWidth()) {
                GoldButton(text = "Open jobs", onClick = onOpenJobs, modifier = Modifier.weight(1f))
                OutlinedButton(onClick = { onNavigate("scan") }, modifier = Modifier.weight(1f)) { Text("Scan job") }
            }

            BrandSectionTitle("My bench")
            if (myOpenJobs.isEmpty()) {
                EmptyHint(
                    message = "Claim an unassigned intake from the jobs board to get started.",
                    title = "No jobs on your bench",
                    icon = Icons.Outlined.Handyman,
                )
            } else {
                myOpenJobs.take(8).forEach { job ->
                    BrandCard(onClick = { onOpenJob(job.getString("id")) }) {
                        Row(
                            Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.SpaceBetween,
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Text(
                                job.optString("job_code").ifBlank { job.getString("id").take(8) },
                                style = MaterialTheme.typography.titleMedium,
                                fontWeight = FontWeight.Bold,
                            )
                            PillBadge(
                                statusLabel(job.optString("status")).replaceFirstChar { it.uppercase() },
                                statusColor(job.optString("status")),
                            )
                        }
                        Text(
                            job.optString("problem_summary").ifBlank { "Repair job" },
                            style = MaterialTheme.typography.bodyMedium,
                            color = Brand.TextSecondary,
                        )
                        Text(
                            job.optString("customer_name").ifBlank { "Walk-in" },
                            style = MaterialTheme.typography.bodySmall,
                            color = Brand.TextMuted,
                        )
                    }
                }
            }

            GoldButton(
                text = "Open full jobs board",
                onClick = onOpenJobs,
                modifier = Modifier.fillMaxWidth(),
            )
        } else {
            BrandSectionTitle("Quick actions")
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp), modifier = Modifier.fillMaxWidth()) {
                if (canSell) {
                    GoldButton(text = "New sale", onClick = { onNavigate("pos") }, modifier = Modifier.weight(1f))
                }
                OutlinedButton(onClick = onOpenJobs, modifier = Modifier.weight(1f)) {
                    Text(if (isCashier) "Repair payment" else "New intake")
                }
            }
            if (canRepair && canCash) {
                GoldButton(
                    text = "Same-day counter fix — fix + pay",
                    onClick = { onNavigate("quick_repair") },
                    modifier = Modifier.fillMaxWidth(),
                )
            }
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp), modifier = Modifier.fillMaxWidth()) {
                OutlinedButton(onClick = { onNavigate("scan") }, modifier = Modifier.weight(1f)) { Text("Scan code") }
                OutlinedButton(
                    onClick = { onNavigate(if (isCashier) "pickup" else "cash") },
                    modifier = Modifier.weight(1f),
                ) { Text(if (isCashier) "Pickup" else "Cash") }
            }
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp), modifier = Modifier.fillMaxWidth()) {
                OutlinedButton(onClick = { onNavigate("search") }, modifier = Modifier.weight(1f)) { Text("Search all") }
                OutlinedButton(onClick = { onNavigate("notifications") }, modifier = Modifier.weight(1f)) { Text(if (unreadNotifications > 0) "Inbox ($unreadNotifications)" else "Inbox") }
            }

            BrandSectionTitle("Needs attention")
            if (unmatchedPayments == 0 && readyOrders == 0 && unassigned == 0 && readyCount == 0 && (!isOwner || collectedCreditJobs.isEmpty()) && quickOpenCount == 0 && unreadNotifications == 0) {
                EmptyHint(message = "No urgent exceptions right now.", title = "Everything is on track")
            } else {
                if (unreadNotifications > 0) {
                    BrandCard(onClick = { onNavigate("notifications") }) {
                        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                            Text("New notifications", fontWeight = FontWeight.SemiBold)
                            PillBadge(unreadNotifications.toString(), Brand.Warning)
                        }
                        Text("Open the attention centre and handle them", color = Brand.TextSecondary)
                    }
                }
                if (quickOpenCount > 0) {
                    BrandCard(onClick = onOpenJobs) {
                        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                            Text("Quick replacements", fontWeight = FontWeight.SemiBold)
                            PillBadge(quickOpenCount.toString(), Brand.Navy)
                        }
                        Text("Fitting, testing or waiting for a replacement part", color = Brand.TextSecondary)
                    }
                }
                if (isOwner && collectedCreditJobs.isNotEmpty()) {
                    val today = LocalDate.now()
                    val overdue = collectedCreditJobs.count { job ->
                        runCatching { LocalDate.parse(job.optString("credit_due_date").take(10)) }.getOrNull()?.isBefore(today) == true
                    }
                    val dueSoon = collectedCreditJobs.count { job ->
                        val due = runCatching { LocalDate.parse(job.optString("credit_due_date").take(10)) }.getOrNull()
                        due != null && !due.isBefore(today) && !due.isAfter(today.plusDays(5))
                    }
                    val outstanding = collectedCreditJobs.sumOf { it.optDouble("balance_due", 0.0) }
                    BrandCard(onClick = onOpenJobs) {
                        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                            Text("Collected · payment outstanding", fontWeight = FontWeight.SemiBold)
                            PillBadge(collectedCreditJobs.size.toString(), if (overdue > 0) Brand.Danger else Brand.Warning)
                        }
                        Text("KES ${outstanding.toInt()} unpaid", color = Brand.TextSecondary)
                        Text(
                            when { overdue > 0 -> "$overdue overdue · action required"; dueSoon > 0 -> "$dueSoon due within 5 days · remind customers"; else -> "Credit is within agreed terms" },
                            color = if (overdue > 0) Brand.Danger else Brand.Warning,
                            fontWeight = FontWeight.Medium,
                        )
                        collectedCreditJobs.sortedBy { it.optString("credit_due_date") }.take(3).forEach { job ->
                            Text("${job.optString("job_code")} · KES ${job.optDouble("balance_due").toInt()} · due ${job.optString("credit_due_date").take(10)}", style = MaterialTheme.typography.bodySmall)
                        }
                    }
                }
                if (unmatchedPayments > 0) {
                    BrandCard(onClick = { onNavigate("c2b") }) {
                        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                            Text("Unmatched payments", fontWeight = FontWeight.SemiBold)
                            PillBadge(unmatchedPayments.toString(), Brand.Danger)
                        }
                        Text("Match received M-Pesa money", color = Brand.TextSecondary)
                    }
                }
                if (readyOrders > 0) {
                    BrandCard(onClick = { onNavigate("pickup") }) {
                        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                            Text("Orders ready for pickup", fontWeight = FontWeight.SemiBold)
                            PillBadge(readyOrders.toString(), Brand.Warning)
                        }
                        Text("Verify and hand orders to customers", color = Brand.TextSecondary)
                    }
                }
                if (unassigned > 0) {
                    BrandCard(onClick = onOpenJobs) {
                        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                            Text("Unassigned repairs", fontWeight = FontWeight.SemiBold)
                            PillBadge(unassigned.toString(), Brand.Warning)
                        }
                        Text("Assign these jobs to the workshop", color = Brand.TextSecondary)
                    }
                }
                if (readyCount > 0) {
                    BrandCard(onClick = onOpenJobs) {
                        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                            Text("Devices ready", fontWeight = FontWeight.SemiBold)
                            PillBadge(readyCount.toString(), Brand.Success)
                        }
                        Text("Notify or hand over to customers", color = Brand.TextSecondary)
                    }
                }
            }

            BrandSectionTitle("Today")
            BrandCard {
                summary?.let { s ->
                    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                        Text("Payments received", color = Brand.TextSecondary)
                        Text("KES " + s.optDouble("payments_allocated_period", 0.0).toInt(), fontWeight = FontWeight.Bold)
                    }
                }
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                    Text("Open repairs", color = Brand.TextSecondary)
                    Text(openCount.toString(), fontWeight = FontWeight.Bold)
                }
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                    Text("Ready for customers", color = Brand.TextSecondary)
                    Text(readyCount.toString(), fontWeight = FontWeight.Bold)
                }
                if (canCash) {
                    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                        Text("Cash in till", color = Brand.TextSecondary)
                        Text("KES " + pendingCash.toInt(), fontWeight = FontWeight.Bold, color = if (pendingCash > 0) Brand.Warning else Brand.TextPrimary)
                    }
                }
            }

            OutlinedButton(onClick = { showMoreMetrics = !showMoreMetrics }, modifier = Modifier.fillMaxWidth()) {
                Text(if (showMoreMetrics) "Hide workload details" else "View workload details")
            }
            if (showMoreMetrics) {
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp), modifier = Modifier.fillMaxWidth()) {
                    MetricTile("In progress", (counts["in_progress"] ?: 0).toString(), tileModifier = Modifier.weight(1f), accent = statuses.inProgress)
                    MetricTile("Waiting parts", waitingCount.toString(), tileModifier = Modifier.weight(1f), accent = statuses.waitingParts)
                }
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp), modifier = Modifier.fillMaxWidth()) {
                    MetricTile("Collected", collectedCount.toString(), tileModifier = Modifier.weight(1f), accent = statuses.collected)
                    MetricTile("Intake", (counts["intake"] ?: 0).toString(), tileModifier = Modifier.weight(1f), accent = statuses.intake)
                }
            }
        }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun JobsScreen(
    isTechnician: Boolean = false,
    canCollect: Boolean = false,
    canClose: Boolean = false,
    canAuthorize: Boolean = false,
    canReleaseUnverified: Boolean = false,
    canTakePayment: Boolean = false,
    defaultMine: Boolean = false,
    openJobId: String? = null,
    onOpenJobConsumed: () -> Unit = {},
    onOpenPos: () -> Unit = {},
    modifier: Modifier = Modifier,
) {
    var selectedJobId by remember { mutableStateOf<String?>(null) }
    var showIntake by remember { mutableStateOf(false) }

    LaunchedEffect(openJobId) {
        if (openJobId != null) {
            selectedJobId = openJobId
            onOpenJobConsumed()
        }
    }

    if (selectedJobId != null) {
        BackHandler { selectedJobId = null }
        JobDetailScreen(
            jobId = selectedJobId!!,
            isTechnician = isTechnician,
            canCollect = canCollect,
            canClose = canClose,
            canAuthorize = canAuthorize,
            canReleaseUnverified = canReleaseUnverified,
            canTakePayment = canTakePayment,
            onBack = { selectedJobId = null },
            onOpenRelatedJob = { id -> selectedJobId = id },
            modifier = modifier,
        )
        return
    }
    if (showIntake) {
        IntakeScreen(
            onBack = { showIntake = false },
            onOpenPos = onOpenPos,
            onCreated = { jobId ->
                showIntake = false
                selectedJobId = jobId
            },
            modifier = modifier,
        )
        return
    }

    var jobs by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var error by remember { mutableStateOf<String?>(null) }
    var filter by remember { mutableStateOf(if (defaultMine) "mine" else "all") }
    var search by remember { mutableStateOf("") }
    var loading by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()

    LaunchedEffect(defaultMine) {
        if (defaultMine && filter == "all") filter = "mine"
    }

    fun refresh() {
        scope.launch {
            loading = true
            try {
                val items = withContext(Dispatchers.IO) {
                    ApiClient.listRepairs(
                        status = filter.takeIf { it != "all" && it != "mine" },
                        technicianId = if (filter == "mine") "me" else null,
                        q = search.takeIf { it.isNotBlank() },
                    )
                }
                jobs = (0 until items.length()).map { items.getJSONObject(it) }
                error = null
            } catch (e: Exception) {
                error = e.message
            } finally {
                loading = false
            }
        }
    }

    LaunchedEffect(filter, search) {
        if (search.isNotBlank()) kotlinx.coroutines.delay(300)
        refresh()
    }

    Column(
        modifier = modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background),
    ) {
        BrandHero(
            title = "Jobs",
            subtitle = if (isTechnician) {
                "Your queue — claim intake, diagnose, request parts"
            } else {
                "Repair board across the floor"
            },
            appLabel = "Ops",
            trailing = {
                IconButton(onClick = { refresh() }) {
                    Icon(Icons.Filled.Refresh, "Refresh", tint = Color.White)
                }
            },
            bottomContent = {
                OutlinedTextField(
                    value = search,
                    onValueChange = { search = it },
                    label = { Text("Search job, customer, problem") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                    colors = androidx.compose.material3.OutlinedTextFieldDefaults.colors(
                        focusedContainerColor = Color.White.copy(alpha = 0.12f),
                        unfocusedContainerColor = Color.White.copy(alpha = 0.08f),
                        focusedTextColor = Color.White,
                        unfocusedTextColor = Color.White,
                        focusedLabelColor = Color.White.copy(alpha = 0.8f),
                        unfocusedLabelColor = Color.White.copy(alpha = 0.65f),
                        cursorColor = Brand.Gold,
                        focusedBorderColor = Color.White.copy(alpha = 0.45f),
                        unfocusedBorderColor = Color.White.copy(alpha = 0.22f),
                    ),
                )
            },
        )
        OpsShellChrome()
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 10.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .horizontalScroll(rememberScrollState()),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                val filters = buildList {
                    if (isTechnician || defaultMine) {
                        add("mine" to "Mine")
                        add("all" to "All")
                    } else {
                        add("all" to "All")
                        add("mine" to "Mine")
                    }
                    add("intake" to "Intake")
                    add("diagnosed" to "Diagnosed")
                    add("waiting_parts" to "Waiting parts")
                    add("in_progress" to "Repairing")
                    add("ready_for_pickup" to "QC")
                    add("completed" to "Ready")
                    add("collected" to "Collected")
                    add("closed" to "Closed")
                }
                filters.forEach { (key, label) ->
                    FilterChip(
                        selected = filter == key,
                        onClick = { filter = key },
                        label = { Text(label) },
                        colors = FilterChipDefaults.filterChipColors(
                            selectedContainerColor = Brand.NavyTint,
                            selectedLabelColor = Brand.Navy,
                        ),
                    )
                }
            }
            GoldButton(
                text = "New intake",
                onClick = { showIntake = true },
                modifier = Modifier.fillMaxWidth(),
            )
        }
        FeedbackBanner(message = null, error = error, modifier = Modifier.padding(horizontal = 16.dp))
        if (loading && jobs.isEmpty()) {
            SkeletonList(modifier = Modifier.padding(16.dp))
        }
        if (!loading && jobs.isEmpty() && error == null) {
            EmptyHint(
                message = when {
                    filter == "mine" -> "Claim an unassigned intake from All or Intake."
                    else -> "Try another status filter or clear search."
                },
                title = if (filter == "mine") "Nothing on your bench" else "No jobs match",
                icon = Icons.Outlined.Handyman,
                hintModifier = Modifier.padding(16.dp),
            )
        }
        val displayedJobs = jobs.sortedBy { job ->
            val promised = runCatching { Instant.parse(job.optString("promised_by")) }.getOrNull()
            when {
                promised != null && promised.isBefore(Instant.now()) && job.optString("status") !in setOf("completed", "collected") -> 0
                job.optBoolean("customer_waiting") -> 1
                job.optString("status") in setOf("ready_for_pickup", "completed") -> 2
                else -> 3
            }
        }
        PullToRefreshBox(
            isRefreshing = loading,
            onRefresh = { refresh() },
            modifier = Modifier.weight(1f),
        ) {
        LazyColumn(
            contentPadding = PaddingValues(horizontal = 16.dp, vertical = 8.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
            modifier = Modifier.fillMaxSize(),
        ) {
            items(displayedJobs, key = { it.getString("id") }) { job ->
                val device = job.optJSONObject("device")
                val deviceLabel = listOf(
                    device?.optString("brand").orEmpty(),
                    device?.optString("model").orEmpty(),
                ).filter { it.isNotBlank() }.joinToString(" ").ifBlank {
                    job.optString("problem_summary").ifBlank { "Repair job" }
                }
                BrandCard(onClick = { selectedJobId = job.getString("id") }) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text(
                            job.optString("job_code").ifBlank { job.getString("id").take(8) },
                            style = MaterialTheme.typography.titleMedium,
                            fontWeight = FontWeight.Bold,
                        )
                        PillBadge(
                            statusLabel(job.optString("status")).replaceFirstChar { it.uppercase() },
                            statusColor(job.optString("status")),
                        )
                    }
                    if (job.optString("service_type") == "quick_replacement") {
                        Text("Quick replacement", style = MaterialTheme.typography.labelMedium, color = Brand.GoldDark, fontWeight = FontWeight.SemiBold)
                    }
                    Text(
                        deviceLabel,
                        style = MaterialTheme.typography.bodyMedium,
                        color = Brand.TextSecondary,
                    )
                    val promisedInstant = runCatching { Instant.parse(job.optString("promised_by")) }.getOrNull()
                    if (promisedInstant != null) {
                        val overdue = promisedInstant.isBefore(Instant.now()) && job.optString("status") !in setOf("completed", "collected")
                        Text(if (overdue) "Overdue promise · action required" else "Promised " + timeAgo(job.optString("promised_by")).removeSuffix(" ago"), color = if (overdue) Brand.Danger else Brand.TextSecondary, style = MaterialTheme.typography.bodySmall, fontWeight = if (overdue) FontWeight.SemiBold else FontWeight.Normal)
                    }
                    Text(
                        when (job.optString("status")) {
                            "intake" -> if (job.optString("service_type") == "quick_replacement") "Next · start fitting or wait for stock" else "Next · diagnose"
                            "diagnosed" -> "Next · agree price and start work"
                            "waiting_parts" -> "Next · receive part and resume"
                            "in_progress" -> if (job.optString("service_type") == "quick_replacement") "Next · fit and test" else "Next · finish repair and test"
                            "ready_for_pickup" -> "Next · final check"
                            "completed" -> "Next · payment and collection"
                            else -> ""
                        },
                        color = Brand.Navy, style = MaterialTheme.typography.bodySmall, fontWeight = FontWeight.Medium,
                    )
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                    ) {
                        Text(
                            buildString {
                                append(job.optString("customer_name").ifBlank { "Walk-in" })
                                if (job.optBoolean("customer_waiting")) {
                                    append(" · wait bench")
                                    val mins = job.optInt("estimated_wait_minutes")
                                    if (mins > 0) append(" (~${mins}m)")
                                }
                                val phone = job.optString("customer_phone")
                                if (phone.isNotBlank() && phone != "null") append(" · $phone")
                            },
                            color = Brand.TextMuted,
                            style = MaterialTheme.typography.bodySmall,
                        )
                        Text(
                            timeAgo(job.optString("created_at")),
                            color = Brand.TextMuted,
                            style = MaterialTheme.typography.bodySmall,
                        )
                    }
                }
            }
        }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class, ExperimentalFoundationApi::class)
@Composable
fun IntakeScreen(onBack: () -> Unit, onOpenPos: () -> Unit = {}, onCreated: (String) -> Unit, modifier: Modifier = Modifier) {
    // Typed/selected form state survives rotation and process death; caches (search
    // results, stock lookups) and in-flight busy flags intentionally do not — they
    // re-derive from the network rather than risk showing stale or stuck state.
    var customerName by rememberSaveable { mutableStateOf("") }
    var customerPhone by rememberSaveable { mutableStateOf("") }
    var selectedCustomerId by rememberSaveable { mutableStateOf<String?>(null) }
    var customerMatches by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var searchingCustomers by remember { mutableStateOf(false) }
    var anonymous by rememberSaveable { mutableStateOf(false) }
    var kind by rememberSaveable { mutableStateOf("phone") }
    var brand by rememberSaveable { mutableStateOf("") }
    var model by rememberSaveable { mutableStateOf("") }
    var imei by rememberSaveable { mutableStateOf("") }
    var problem by rememberSaveable { mutableStateOf("") }
    var chargeAmount by rememberSaveable { mutableStateOf("") }
    var priceAgreed by rememberSaveable { mutableStateOf(false) }
    var promisedDate by rememberSaveable { mutableStateOf("") }
    var promisedTime by rememberSaveable { mutableStateOf("") }
    var showPromisedPicker by remember { mutableStateOf(false) }
    var showPromisedTimePicker by remember { mutableStateOf(false) }
    var customerCredit by rememberSaveable { mutableStateOf(false) }
    var creditDueDate by rememberSaveable { mutableStateOf("") }
    var showCreditDuePicker by remember { mutableStateOf(false) }
    var customerWaiting by rememberSaveable { mutableStateOf(false) }
    var waitMinutes by rememberSaveable { mutableStateOf("45") }
    var accessoriesText by rememberSaveable { mutableStateOf("") }
    var conditionNotes by rememberSaveable { mutableStateOf("") }
    var devicePasscode by rememberSaveable { mutableStateOf("") }
    var devicePasscodeVisible by remember { mutableStateOf(false) }
    var assignToMe by rememberSaveable { mutableStateOf(true) }
    // Photo bytes are deliberately kept out of saved-instance state (Bundle size limits) —
    // losing an in-progress photo pick on process death is an acceptable trade-off.
    var imeiPhoto by remember { mutableStateOf<Pair<ByteArray, String>?>(null) }
    var devicePhoto by remember { mutableStateOf<Pair<ByteArray, String>?>(null) }
    var photoTarget by rememberSaveable { mutableStateOf("imei") }
    var takePictureUri by rememberSaveable { mutableStateOf<android.net.Uri?>(null) }
    var error by rememberSaveable { mutableStateOf<String?>(null) }
    var photoHint by rememberSaveable { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    var currentStep by rememberSaveable { mutableStateOf(0) }
    var serviceType by rememberSaveable { mutableStateOf("repair") }
    var quickStock by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var quickStockQuery by rememberSaveable { mutableStateOf("") }
    var quickStockKey by rememberSaveable { mutableStateOf("") }
    var quickWaitingPart by rememberSaveable { mutableStateOf(false) }
    var quickStockLoading by remember { mutableStateOf(false) }
    var showDiscardDialog by remember { mutableStateOf(false) }
    var showEvidence by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    val context = LocalContext.current
    val intakeScroll = rememberScrollState()
    val matchesBringIntoView = remember { BringIntoViewRequester() }
    val stepLabels = if (serviceType == "quick_replacement") listOf("Customer", "Device", "Replacement") else listOf("Customer", "Device", "Service", "Confirm")
    val hasDraft = customerName.isNotBlank() || customerPhone.isNotBlank() || brand.isNotBlank() ||
        model.isNotBlank() || imei.isNotBlank() || problem.isNotBlank() || chargeAmount.isNotBlank() ||
        promisedDate.isNotBlank() || accessoriesText.isNotBlank() || conditionNotes.isNotBlank() ||
        devicePasscode.isNotBlank() || imeiPhoto != null || devicePhoto != null || anonymous

    fun navigateBack() {
        when {
            busy -> Unit
            currentStep > 0 -> {
                error = null
                currentStep -= 1
            }
            hasDraft -> showDiscardDialog = true
            else -> onBack()
        }
    }

    BackHandler(enabled = currentStep > 0 || hasDraft) { navigateBack() }

    val searchQuery = remember(customerPhone, customerName) {
        when {
            customerPhone.trim().length >= 3 -> customerPhone.trim()
            customerName.trim().length >= 2 -> customerName.trim()
            else -> ""
        }
    }

    LaunchedEffect(searchQuery, selectedCustomerId) {
        if (selectedCustomerId != null || searchQuery.length < 2) {
            customerMatches = emptyList()
            searchingCustomers = false
            return@LaunchedEffect
        }
        searchingCustomers = true
        kotlinx.coroutines.delay(280)
        try {
            val items = withContext(Dispatchers.IO) { ApiClient.listCustomers(searchQuery) }
            customerMatches = (0 until items.length()).map { items.getJSONObject(it) }
        } catch (_: Exception) {
            customerMatches = emptyList()
        } finally {
            searchingCustomers = false
        }
    }

    LaunchedEffect(serviceType) {
        if (serviceType == "quick_replacement" && quickStock.isEmpty()) {
            quickStockLoading = true
            quickStock = runCatching { withContext(Dispatchers.IO) { ApiClient.listInventoryBalances() } }
                .getOrNull()?.let { items -> (0 until items.length()).map { items.getJSONObject(it) }.filter { it.optInt("available_qty", 0) > 0 } } ?: emptyList()
            quickStockLoading = false
        }
    }

    LaunchedEffect(customerMatches) {
        if (customerMatches.isNotEmpty()) {
            matchesBringIntoView.bringIntoView()
        }
    }

    val promisedPickerState = rememberDatePickerState(
        initialSelectedDateMillis = promisedDate.takeIf { it.length == 10 }?.let {
            runCatching {
                LocalDate.parse(it).atStartOfDay(ZoneOffset.UTC).toInstant().toEpochMilli()
            }.getOrNull()
        },
    )
    // Shops promise "by end of day" most often — default the clock to 5 PM.
    val promisedTimeState = rememberTimePickerState(initialHour = 17, initialMinute = 0)
    val creditDuePickerState = rememberDatePickerState(
        initialSelectedDateMillis = creditDueDate.takeIf { it.length == 10 }?.let {
            runCatching { LocalDate.parse(it).atStartOfDay(ZoneOffset.UTC).toInstant().toEpochMilli() }.getOrNull()
        },
    )

    fun applyPhoto(uri: android.net.Uri) {
        val photo = PhotoCapture.readImageBytes(context, uri, "intake-photo.jpg")
        if (photoTarget == "device") devicePhoto = photo else imeiPhoto = photo
        error = null
        photoHint = "Photo attached — scroll down to create the job when ready."
    }

    val picker = rememberLauncherForActivityResult(ActivityResultContracts.GetContent()) { uri ->
        if (uri == null) return@rememberLauncherForActivityResult
        try {
            applyPhoto(uri)
        } catch (e: Exception) {
            error = e.message
            photoHint = null
        }
    }
    val takePicture = rememberLauncherForActivityResult(ActivityResultContracts.TakePicture()) { ok ->
        val uri = takePictureUri
        takePictureUri = null
        if (uri == null) return@rememberLauncherForActivityResult
        // Many OEM cameras write the file then return RESULT_CANCELED.
        if (!ok && !PhotoCapture.hasContent(context, uri)) {
            return@rememberLauncherForActivityResult
        }
        try {
            applyPhoto(uri)
        } catch (e: Exception) {
            error = e.message
            photoHint = null
        }
    }
    fun launchCameraNow() {
        val uri = PhotoCapture.createCameraOutputUri(context, "intake")
        takePictureUri = uri
        takePicture.launch(uri)
    }
    val cameraPermission = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        if (!granted) {
            error = "Camera permission is required to take photos"
            return@rememberLauncherForActivityResult
        }
        launchCameraNow()
    }

    fun startCamera(target: String) {
        photoTarget = target
        error = null
        photoHint = null
        when {
            androidx.core.content.ContextCompat.checkSelfPermission(
                context,
                android.Manifest.permission.CAMERA,
            ) == android.content.pm.PackageManager.PERMISSION_GRANTED -> {
                launchCameraNow()
            }
            else -> cameraPermission.launch(android.Manifest.permission.CAMERA)
        }
    }

    Column(
        modifier = modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background),
    ) {
        BrandDetailHeader(
            title = "New intake",
            subtitle = "Capture customer + device before the bench starts.",
            navigation = {
                IconButton(onClick = ::navigateBack) {
                    Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back", tint = Color.White)
                }
            },
        )
        Column(
            modifier = Modifier
                .weight(1f)
                .imePadding()
                .verticalScroll(intakeScroll)
                .padding(16.dp)
                .padding(bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {

        Column(verticalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.fillMaxWidth()) {
            Text("Step ${currentStep + 1} of ${stepLabels.size} · ${stepLabels[currentStep]}", fontWeight = FontWeight.SemiBold, color = Brand.Navy)
            Row(horizontalArrangement = Arrangement.spacedBy(6.dp), modifier = Modifier.fillMaxWidth()) {
                stepLabels.indices.forEach { index ->
                    Surface(
                        modifier = Modifier.weight(1f).height(5.dp),
                        shape = RoundedCornerShape(99.dp),
                        color = if (index <= currentStep) Brand.Gold else Brand.Border,
                    ) {}
                }
            }
        }

        if (currentStep == 0) {
        FormSection("What does the customer need?") {
            Row(Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                listOf("repair" to "Repair", "quick_replacement" to "Quick replacement", "parts_only" to "Parts only").forEach { (key, label) ->
                    FilterChip(
                        selected = serviceType == key,
                        onClick = {
                            serviceType = key
                            if (key == "quick_replacement") priceAgreed = true
                            if (key != "quick_replacement") { quickStockKey = ""; quickWaitingPart = false }
                        },
                        label = { Text(label) },
                    )
                }
            }
            Text(
                when (serviceType) {
                    "quick_replacement" -> "Known replacement: check stock, fit, test and finish."
                    "parts_only" -> "No device intake needed. Continue in POS for stock and payment."
                    else -> "Diagnosis or full workshop repair."
                },
                style = MaterialTheme.typography.bodySmall, color = Brand.TextSecondary,
            )
            if (serviceType == "parts_only") GoldButton("Open POS", onClick = onOpenPos, modifier = Modifier.fillMaxWidth())
        }
        if (serviceType != "parts_only") FormSection("Customer") {
        Text(
            "Returning customer? Type their phone or name and tap a match.",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        FilterChip(
            selected = anonymous,
            onClick = {
                anonymous = !anonymous
                if (anonymous) {
                    selectedCustomerId = null
                    customerName = ""
                    customerPhone = ""
                    customerMatches = emptyList()
                }
                error = null
            },
            label = { Text("Walk-in customer — no phone") },
        )
        if (selectedCustomerId != null) {
            Surface(
                shape = RoundedCornerShape(12.dp),
                color = MaterialTheme.colorScheme.primaryContainer,
                border = BorderStroke(1.dp, MaterialTheme.colorScheme.primary.copy(alpha = 0.35f)),
                modifier = Modifier.fillMaxWidth(),
            ) {
                Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    Text("Returning customer", fontWeight = FontWeight.SemiBold)
                    Text(
                        buildString {
                            append(customerName.ifBlank { "Customer" })
                            if (customerPhone.isNotBlank()) append(" · $customerPhone")
                        },
                    )
                    TextButton(
                        onClick = {
                            selectedCustomerId = null
                            customerMatches = emptyList()
                        },
                    ) { Text("Change customer") }
                }
            }
        } else {
            OutlinedTextField(
                value = customerPhone,
                onValueChange = { customerPhone = it },
                label = { Text("Phone (searches existing)") },
                modifier = Modifier.fillMaxWidth(),
                singleLine = true,
                enabled = !anonymous,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Phone),
            )
            if (searchingCustomers) {
                Text("Searching…", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            if (customerMatches.isNotEmpty()) {
                Surface(
                    shape = RoundedCornerShape(12.dp),
                    color = Brand.NavyTint,
                    border = BorderStroke(1.dp, Brand.Navy.copy(alpha = 0.2f)),
                    modifier = Modifier
                        .fillMaxWidth()
                        .bringIntoViewRequester(matchesBringIntoView),
                ) {
                    Column(modifier = Modifier.padding(vertical = 4.dp)) {
                        Text(
                            "Matches — tap to use",
                            style = MaterialTheme.typography.labelMedium,
                            color = Brand.Navy,
                            modifier = Modifier.padding(horizontal = 12.dp, vertical = 6.dp),
                            fontWeight = FontWeight.SemiBold,
                        )
                        customerMatches.take(8).forEach { c ->
                            val cid = c.optString("id")
                            val label = buildString {
                                append(c.optString("full_name"))
                                val ph = c.optString("phone")
                                if (ph.isNotBlank() && ph != "null") append(" · $ph")
                            }
                            TextButton(
                                onClick = {
                                    selectedCustomerId = cid
                                    customerName = c.optString("full_name")
                                    customerPhone = c.optString("phone").takeIf { it.isNotBlank() && it != "null" }.orEmpty()
                                    customerMatches = emptyList()
                                    context.showAppToast("Using existing customer")
                                },
                                modifier = Modifier.fillMaxWidth(),
                            ) {
                                Text(label, modifier = Modifier.fillMaxWidth(), color = Brand.Navy)
                            }
                        }
                    }
                }
            }
            OutlinedTextField(
                value = customerName,
                onValueChange = { customerName = it },
                label = { Text("Full name (for new customers)") },
                modifier = Modifier.fillMaxWidth(),
                singleLine = true,
                enabled = !anonymous,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Text),
            )
        }
        }
        }

        if (currentStep == 1) {
        FormSection("Device") {
        val kindOptions = listOf(
            "phone" to "Phone",
            "laptop" to "Laptop",
            "tablet" to "Tablet",
            "other" to "Other",
        )
        Text("Device type", style = MaterialTheme.typography.labelMedium, color = Brand.TextSecondary)
        Row(
            modifier = Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            kindOptions.forEach { (key, label) ->
                FilterChip(
                    selected = kind == key,
                    onClick = { kind = key },
                    label = { Text(label) },
                    colors = FilterChipDefaults.filterChipColors(
                        selectedContainerColor = Brand.Navy,
                        selectedLabelColor = Color.White,
                    ),
                )
            }
        }
        Row(
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            modifier = Modifier.fillMaxWidth(),
        ) {
            OutlinedTextField(
                value = brand,
                onValueChange = { brand = it },
                label = { Text("Brand") },
                modifier = Modifier.weight(1f),
                singleLine = true,
            )
            OutlinedTextField(
                value = model,
                onValueChange = { model = it },
                label = { Text("Model") },
                modifier = Modifier.weight(1f),
                singleLine = true,
            )
        }
        OutlinedTextField(
            value = imei,
            onValueChange = { imei = it },
            label = { Text("IMEI / Serial (optional)") },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true,
        )
        Text(
            "Or photograph the IMEI sticker instead of typing",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.fillMaxWidth()) {
            OutlinedButton(
                onClick = { startCamera("imei") },
                modifier = Modifier.weight(1f),
            ) { Text(if (imeiPhoto != null) "IMEI photo ✓" else "Take IMEI photo") }
            OutlinedButton(
                onClick = {
                    photoTarget = "imei"
                    picker.launch("image/*")
                },
                modifier = Modifier.weight(1f),
            ) { Text("Upload IMEI") }
        }
        if (imeiPhoto != null) {
            TextButton(onClick = { imeiPhoto = null; photoHint = "IMEI photo removed" }) { Text("Remove IMEI photo") }
        }
        Text(
            "Device condition photo (cracks, water marks)",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.fillMaxWidth()) {
            OutlinedButton(
                onClick = { startCamera("device") },
                modifier = Modifier.weight(1f),
            ) { Text(if (devicePhoto != null) "Condition ✓" else "Take photo") }
            OutlinedButton(
                onClick = {
                    photoTarget = "device"
                    picker.launch("image/*")
                },
                modifier = Modifier.weight(1f),
            ) { Text("Upload") }
        }
        if (devicePhoto != null) {
            TextButton(onClick = { devicePhoto = null; photoHint = "Condition photo removed" }) { Text("Remove condition photo") }
        }
        photoHint?.let {
            Text(it, style = MaterialTheme.typography.bodySmall, color = Brand.Navy)
        }
        error?.takeIf { it.contains("photo", ignoreCase = true) || it.contains("camera", ignoreCase = true) || it.contains("empty", ignoreCase = true) }?.let {
            Text(it, style = MaterialTheme.typography.bodySmall, color = Brand.Danger)
        }
        }
        }

        if (currentStep == 2) {
        FormSection("Problem") {
        OutlinedTextField(
            value = problem,
            onValueChange = { problem = it },
            label = { Text(if (serviceType == "quick_replacement") "What needs replacing?" else "What is wrong with it?") },
            minLines = 2,
            modifier = Modifier.fillMaxWidth(),
        )
        Text("Common issues", style = MaterialTheme.typography.labelMedium, color = Brand.TextSecondary)
        Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            (if (serviceType == "quick_replacement") listOf("Battery replacement", "Screen replacement", "Charging port", "Camera", "Speaker", "Back cover") else listOf("Broken screen", "Not charging", "No power", "Battery", "Water damage", "Software")).forEach { issue ->
                FilterChip(selected = problem == issue, onClick = { problem = issue }, label = { Text(issue) })
            }
        }
        // Whether the price is known at intake changes the whole shape of the job:
        // an agreed price starts work immediately, no price waits for an approved
        // estimate. Make it an explicit choice rather than a blank amount box.
        SectionLabel("Pricing")
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            FilterChip(
                selected = priceAgreed,
                onClick = { priceAgreed = true },
                label = { Text("Price agreed now") },
                modifier = Modifier.weight(1f),
            )
            FilterChip(
                selected = !priceAgreed,
                onClick = { if (serviceType != "quick_replacement") priceAgreed = false },
                label = { Text(if (serviceType == "quick_replacement") "Price required" else "Diagnose first") },
                modifier = Modifier.weight(1f),
            )
        }
        if (priceAgreed) {
            OutlinedTextField(
                value = chargeAmount,
                onValueChange = { chargeAmount = it.filter { ch -> ch.isDigit() || ch == '.' } },
                label = { Text("Agreed price (KES)") },
                modifier = Modifier.fillMaxWidth(),
                singleLine = true,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
            )
        } else {
            Text(
                "No price yet. The technician diagnoses, sends an estimate, and work starts once the customer " +
                    "approves it.",
                style = MaterialTheme.typography.bodySmall,
                color = Brand.TextSecondary,
            )
        }
        if (serviceType == "quick_replacement") {
            SectionLabel("Replacement stock")
            OutlinedTextField(
                value = quickStockQuery,
                onValueChange = { quickStockQuery = it; quickStockKey = ""; quickWaitingPart = false },
                label = { Text("Search battery, screen, SKU or barcode") },
                supportingText = { Text(if (quickStockQuery.trim().length < 2) "Type at least 2 characters" else "Select an available part, or mark it unavailable") },
                modifier = Modifier.fillMaxWidth(), singleLine = true,
            )
            val quickMatches = if (quickStockQuery.trim().length < 2) emptyList() else quickStock.filter { item ->
                listOf(item.optString("product_name"), item.optString("variant_name"), item.optString("sku"), item.optString("barcode"))
                    .any { it.contains(quickStockQuery.trim(), ignoreCase = true) }
            }.take(6)
            if (quickStockLoading) Text("Checking stock…", color = Brand.TextSecondary)
            quickMatches.forEach { item ->
                val key = "${item.optString("variant_id")}:${item.optString("location_id")}"
                FilterChip(
                    selected = quickStockKey == key,
                    onClick = { quickStockKey = key; quickWaitingPart = false },
                    label = { Text("${item.optString("product_name")} · KES ${item.optDouble("sell_price").toInt()} · ${item.optInt("available_qty")} left", maxLines = 1) },
                    modifier = Modifier.fillMaxWidth(),
                )
            }
            FilterChip(
                selected = quickWaitingPart,
                onClick = { quickWaitingPart = !quickWaitingPart; if (quickWaitingPart) quickStockKey = "" },
                label = { Text("Part not in stock · wait for part") },
            )
        }
        SectionLabel("Payment terms")
        FilterChip(
            selected = customerCredit,
            onClick = { customerCredit = !customerCredit; if (customerCredit) { priceAgreed = true; customerWaiting = false } },
            label = { Text(if (customerCredit) "Credit approved" else "Allow customer to pay later") },
        )
        if (customerCredit) {
            OutlinedTextField(
                value = creditDueDate.takeIf { it.isNotBlank() }?.let {
                    runCatching { LocalDate.parse(it).format(DateTimeFormatter.ofPattern("d MMM yyyy")) }.getOrDefault(it)
                } ?: "",
                onValueChange = {},
                readOnly = true,
                label = { Text("Payment due date") },
                supportingText = { Text("Owner warned 5 days before and when overdue") },
                trailingIcon = { IconButton(onClick = { showCreditDuePicker = true }) { Icon(Icons.Filled.DateRange, "Pick payment due date") } },
                modifier = Modifier.fillMaxWidth().clickable { showCreditDuePicker = true },
            )
            if (showCreditDuePicker) {
                DatePickerDialog(
                    onDismissRequest = { showCreditDuePicker = false },
                    confirmButton = { TextButton(onClick = {
                        creditDuePickerState.selectedDateMillis?.let { creditDueDate = Instant.ofEpochMilli(it).atZone(ZoneOffset.UTC).toLocalDate().toString() }
                        showCreditDuePicker = false
                    }) { Text("Set due date") } },
                    dismissButton = { TextButton(onClick = { showCreditDuePicker = false }) { Text("Cancel") } },
                ) { DatePicker(state = creditDuePickerState, title = { Text("Payment due by") }) }
            }
        }
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            FilterChip(
                selected = customerWaiting,
                onClick = { customerWaiting = true },
                label = { Text("Waiting at bench") },
                modifier = Modifier.weight(1f),
            )
            FilterChip(
                selected = !customerWaiting,
                onClick = { customerWaiting = false },
                label = { Text("Leaving device") },
                modifier = Modifier.weight(1f),
            )
        }
        if (customerWaiting) {
            OutlinedTextField(
                value = waitMinutes,
                onValueChange = { waitMinutes = it.filter { ch -> ch.isDigit() } },
                label = { Text("Estimated wait (minutes)") },
                supportingText = { Text("Customer stays; wait-bench SMS is sent") },
                modifier = Modifier.fillMaxWidth(),
                singleLine = true,
            )
        } else {
        OutlinedTextField(
            value = when {
                promisedDate.isBlank() -> ""
                else -> buildString {
                    append(
                        runCatching {
                            LocalDate.parse(promisedDate).format(DateTimeFormatter.ofPattern("d MMM yyyy"))
                        }.getOrDefault(promisedDate),
                    )
                    promisedTime.takeIf { it.isNotBlank() }?.let { t ->
                        append(" · ")
                        append(
                            runCatching {
                                LocalTime.parse(t).format(DateTimeFormatter.ofPattern("h:mm a"))
                            }.getOrDefault(t),
                        )
                    }
                }
            },
            onValueChange = {},
            readOnly = true,
            label = { Text("Promised ready (date & time)") },
            placeholder = { Text("Optional") },
            supportingText = { Text("Tap the calendar — leave blank if none was promised") },
            trailingIcon = {
                IconButton(onClick = { showPromisedPicker = true }) {
                    Icon(Icons.Filled.DateRange, contentDescription = "Pick date and time")
                }
            },
            modifier = Modifier
                .fillMaxWidth()
                .clickable { showPromisedPicker = true },
            singleLine = true,
        )
        if (promisedDate.isNotBlank()) {
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                TextButton(onClick = { showPromisedTimePicker = true }) {
                    Text("Change time")
                }
                TextButton(
                    onClick = {
                        promisedDate = ""
                        promisedTime = ""
                    },
                ) {
                    Text("Clear")
                }
            }
        }
        }
        if (showPromisedPicker) {
            DatePickerDialog(
                onDismissRequest = { showPromisedPicker = false },
                confirmButton = {
                    TextButton(
                        onClick = {
                            promisedPickerState.selectedDateMillis?.let { millis ->
                                promisedDate = Instant.ofEpochMilli(millis)
                                    .atZone(ZoneOffset.UTC)
                                    .toLocalDate()
                                    .toString()
                            }
                            showPromisedPicker = false
                            // Date chosen — flow straight into picking the time.
                            if (promisedDate.isNotBlank()) showPromisedTimePicker = true
                        },
                    ) { Text("Next: time") }
                },
                dismissButton = {
                    TextButton(onClick = { showPromisedPicker = false }) { Text("Cancel") }
                },
            ) {
                DatePicker(state = promisedPickerState, title = { Text("Promised ready by") })
            }
        }
        if (showPromisedTimePicker) {
            AlertDialog(
                onDismissRequest = { showPromisedTimePicker = false },
                title = { Text("What time is it promised?") },
                text = {
                    TimePicker(state = promisedTimeState)
                },
                confirmButton = {
                    TextButton(
                        onClick = {
                            promisedTime = "%02d:%02d".format(promisedTimeState.hour, promisedTimeState.minute)
                            showPromisedTimePicker = false
                        },
                    ) { Text("OK") }
                },
                dismissButton = {
                    TextButton(
                        onClick = {
                            // Keep the date usable even without an explicit time.
                            if (promisedTime.isBlank()) promisedTime = "17:00"
                            showPromisedTimePicker = false
                        },
                    ) { Text("Skip") }
                },
            )
        }
        OutlinedButton(onClick = { showEvidence = !showEvidence }, modifier = Modifier.fillMaxWidth()) {
            Text(if (showEvidence) "Hide optional evidence" else "Add photos, condition, accessories or passcode")
        }
        if (showEvidence) {
        SectionLabel("Intake evidence")
        OutlinedTextField(
            value = accessoriesText,
            onValueChange = { accessoriesText = it },
            label = { Text("Accessories (comma separated)") },
            supportingText = { Text("Example: charger, case, SIM card") },
            modifier = Modifier.fillMaxWidth(),
        )
        OutlinedTextField(
            value = conditionNotes,
            onValueChange = { conditionNotes = it },
            label = { Text("Existing condition") },
            modifier = Modifier.fillMaxWidth(),
        )
        OutlinedTextField(
            value = devicePasscode,
            onValueChange = { devicePasscode = it },
            label = { Text("Device passcode") },
            supportingText = { Text("Encrypted; every reveal is audited") },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true,
            visualTransformation = if (devicePasscodeVisible) VisualTransformation.None else PasswordVisualTransformation(),
            trailingIcon = {
                IconButton(onClick = { devicePasscodeVisible = !devicePasscodeVisible }) {
                    Icon(
                        imageVector = if (devicePasscodeVisible) Icons.Filled.VisibilityOff else Icons.Filled.Visibility,
                        contentDescription = if (devicePasscodeVisible) "Hide passcode" else "Show passcode",
                    )
                }
            },
        )
        }
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            FilterChip(selected = assignToMe, onClick = { assignToMe = !assignToMe }, label = { Text("Assign to me") })
        }
        }
        }

        if (currentStep == 3) {
            FormSection("Review intake") {
                Text(if (anonymous) "Walk-in customer" else customerName.ifBlank { customerPhone })
                Text(listOf(kind.replaceFirstChar { it.uppercase() }, brand, model).filter { it.isNotBlank() }.joinToString(" · "))
                Text(if (serviceType == "quick_replacement") "Quick replacement · $problem" else problem, fontWeight = FontWeight.SemiBold)
                if (serviceType == "quick_replacement") Text(if (quickWaitingPart) "Part unavailable · waiting for stock" else "Part selected from stock")
                Text(if (priceAgreed) "Agreed price · KES $chargeAmount" else "Diagnose first · estimate required")
                if (customerCredit) Text("Credit · pay by $creditDueDate", color = Brand.Warning, fontWeight = FontWeight.SemiBold)
                Text(if (customerWaiting) "Customer waiting · $waitMinutes minutes" else promisedDate.takeIf { it.isNotBlank() }?.let { "Promised · $it $promisedTime" } ?: "No promised time")
                if (imeiPhoto != null || devicePhoto != null) {
                    val photoCount = (if (imeiPhoto != null) 1 else 0) + (if (devicePhoto != null) 1 else 0)
                    Text("Photos attached · $photoCount")
                }
                Text("Tap Create repair job to confirm these details.", style = MaterialTheme.typography.bodySmall, color = Brand.TextSecondary)
            }
        }

        FeedbackBanner(message = null, error = error)
        if (currentStep > 0) {
            OutlinedButton(onClick = ::navigateBack, enabled = !busy, modifier = Modifier.fillMaxWidth()) {
                Text("Back")
            }
        }
        GoldButton(
            text = when { busy -> "Creating…"; serviceType == "parts_only" -> "Open POS"; serviceType == "quick_replacement" && currentStep == 2 -> "Create quick job"; currentStep < 3 -> "Continue"; serviceType == "quick_replacement" -> "Create quick job"; else -> "Create repair job" },
            onClick = {
                if (serviceType == "parts_only") { onOpenPos(); return@GoldButton }
                if (currentStep == 0) {
                    if (!anonymous && selectedCustomerId.isNullOrBlank() && customerPhone.trim().isBlank()) {
                        error = "Enter a phone number or choose Walk-in customer"
                        return@GoldButton
                    }
                    error = null
                    currentStep = 1
                    return@GoldButton
                }
                if (currentStep == 1) {
                    error = null
                    currentStep = 2
                    return@GoldButton
                }
                if (currentStep == 2) {
                    if (problem.isBlank()) {
                        error = "Describe the problem before continuing"
                        return@GoldButton
                    }
                    val stepCharge = if (priceAgreed) chargeAmount.toDoubleOrNull() else null
                    if (priceAgreed && (stepCharge == null || stepCharge <= 0)) {
                        error = "Enter the agreed price, or choose Diagnose first"
                        return@GoldButton
                    }
                    if (serviceType == "quick_replacement" && quickStockKey.isBlank() && !quickWaitingPart) { error = "Select an available part or mark it as waiting for stock"; return@GoldButton }
                    if (customerCredit && (anonymous || customerPhone.isBlank() && selectedCustomerId.isNullOrBlank())) { error = "Credit requires a customer with a phone number"; return@GoldButton }
                    if (customerCredit && creditDueDate.isBlank()) { error = "Choose when the credit must be paid"; return@GoldButton }
                    if (customerWaiting && (waitMinutes.toIntOrNull() ?: 0) <= 0) {
                        error = "Enter the estimated wait time"
                        return@GoldButton
                    }
                    error = null
                    if (serviceType != "quick_replacement") {
                        currentStep = 3
                        return@GoldButton
                    }
                }
                if (problem.isBlank()) {
                    error = "Describe the problem"
                    return@GoldButton
                }
                if (!anonymous && selectedCustomerId.isNullOrBlank() && customerPhone.trim().isBlank()) {
                    error = "Phone number is required for non-anonymous jobs"
                    return@GoldButton
                }
                if (customerCredit && creditDueDate.isBlank()) {
                    error = "Choose when the credit must be paid"
                    return@GoldButton
                }
                if (customerWaiting && (waitMinutes.toIntOrNull() ?: 0) <= 0) {
                    error = "Enter the estimated wait time for the wait bench"
                    return@GoldButton
                }
                val charge = if (priceAgreed) chargeAmount.toDoubleOrNull() else null
                if (priceAgreed && (charge == null || charge <= 0)) {
                    error = "Enter the agreed price, or switch to Diagnose first"
                    return@GoldButton
                }
                busy = true
                error = null
                scope.launch {
                    try {
                        val job = withContext(Dispatchers.IO) {
                            val me = ApiClient.me()
                            val branches = me.optJSONArray("branch_ids")
                            val allowed = if (branches != null) (0 until branches.length()).map { branches.getString(it) } else emptyList()
                            val branchId = TechLaneApp.instance.tokenStore.selectedBranchId
                                ?.takeIf { it in allowed }
                                ?: allowed.firstOrNull()
                                ?: error("No branch on your account")
                            TechLaneApp.instance.tokenStore.selectedBranchId = branchId
                            val customerId = when {
                                !selectedCustomerId.isNullOrBlank() -> selectedCustomerId
                                customerName.isNotBlank() || customerPhone.isNotBlank() -> {
                                    val created = ApiClient.createCustomer(
                                        fullName = customerName.trim().ifBlank { "Customer" },
                                        phone = customerPhone.trim().ifBlank { null },
                                    )
                                    // API reuses existing record when phone already exists.
                                    created.getString("id")
                                }
                                else -> null
                            }
                            val deviceId = ApiClient.createDevice(
                                customerId = customerId,
                                kind = kind,
                                brand = brand.trim().ifBlank { null },
                                model = model.trim().ifBlank { null },
                                imei = imei.trim().ifBlank { null },
                            ).getString("id")
                            val created = ApiClient.createRepair(
                                branchId = branchId,
                                deviceId = deviceId,
                                problemSummary = problem.trim(),
                                serviceType = if (serviceType == "quick_replacement") "quick_replacement" else "repair",
                                customerId = customerId,
                                technicianId = if (assignToMe) me.optString("id") else null,
                                laborAmount = charge,
                                promisedBy = if (!customerWaiting) {
                                    promisedDate.trim().takeIf { it.isNotBlank() }?.let { d ->
                                        val time = runCatching { LocalTime.parse(promisedTime) }
                                            .getOrDefault(LocalTime.of(17, 0))
                                        // Picked in shop-local time; API stores an instant.
                                        LocalDate.parse(d).atTime(time)
                                            .atZone(java.time.ZoneId.systemDefault())
                                            .toInstant()
                                            .toString()
                                    }
                                } else {
                                    null
                                },
                                customerWaiting = customerWaiting,
                                estimatedWaitMinutes = if (customerWaiting) waitMinutes.toIntOrNull() else null,
                                customerCredit = customerCredit,
                                creditDueDate = creditDueDate.takeIf { customerCredit }?.let { "${it}T00:00:00Z" },
                                intakeAccessories = accessoriesText.split(",").map { it.trim() }.filter { it.isNotBlank() },
                                intakeCondition = conditionNotes.trim().ifBlank { null },
                                devicePasscode = devicePasscode.trim().ifBlank { null },
                            )
                            val id = created.getString("id")
                            if (serviceType == "quick_replacement") {
                                if (quickWaitingPart) {
                                    ApiClient.updateRepairStatus(id, "waiting_parts", note = "Quick replacement awaiting stock")
                                } else {
                                    val stockParts = quickStockKey.split(":")
                                    if (stockParts.size == 2) ApiClient.addRepairSaleLine(id, stockParts[0], stockParts[1], 1)
                                    ApiClient.updateRepairStatus(id, "in_progress", note = "Quick replacement ready for fitting")
                                }
                            }
                            val attachErrors = mutableListOf<String>()
                            imeiPhoto?.let { (bytes, name) ->
                                runCatching {
                                    ApiClient.addRepairAttachment(
                                        id,
                                        name.ifBlank { "imei-photo.jpg" },
                                        "image/jpeg",
                                        Base64.encodeToString(bytes, Base64.NO_WRAP),
                                    )
                                }.onFailure { attachErrors += "IMEI photo: ${it.message}" }
                            }
                            devicePhoto?.let { (bytes, name) ->
                                runCatching {
                                    ApiClient.addRepairAttachment(
                                        id,
                                        name.ifBlank { "device-condition.jpg" },
                                        "image/jpeg",
                                        Base64.encodeToString(bytes, Base64.NO_WRAP),
                                    )
                                }.onFailure { attachErrors += "Condition photo: ${it.message}" }
                            }
                            if (attachErrors.isNotEmpty()) {
                                created.put("_attach_warning", attachErrors.joinToString("; "))
                            }
                            created
                        }
                        // Print the intake slip so the customer has a QR even without SMS.
                        try {
                            val html = withContext(Dispatchers.IO) {
                                PrintSupport.fetchText(
                                    "${com.techlane.ops.BuildConfig.API_BASE}/repairs/${job.getString("id")}/intake-slip.html",
                                    TechLaneApp.instance.tokenStore.accessToken,
                                )
                            }
                            PrintSupport.printHtml(
                                context,
                                html,
                                "Intake ${job.optString("job_code").ifBlank { job.getString("id").take(8) }}",
                            )
                        } catch (_: Exception) {
                            // Job is created — printing is best-effort; they can reprint from job detail.
                        }
                        context.showAppToast(
                            buildString {
                                append("Job created · collection code texted")
                                job.optString("pickup_code").takeIf { it.isNotBlank() }?.let { append(" ($it)") }
                                job.optString("_attach_warning").takeIf { it.isNotBlank() }?.let {
                                    append(" · photo upload failed — add from job detail")
                                }
                            },
                            long = true,
                        )
                        onCreated(job.getString("id"))
                    } catch (e: Exception) {
                        error = e.message
                    } finally {
                        busy = false
                    }
                }
            },
            enabled = !busy,
            loading = busy,
            modifier = Modifier.fillMaxWidth(),
        )
        OutlinedButton(
            onClick = {
                busy = true
                error = null
                scope.launch {
                    try {
                        val me = withContext(Dispatchers.IO) { ApiClient.me() }
                        val branches = me.optJSONArray("branch_ids")
                        val allowed = if (branches != null) (0 until branches.length()).map { branches.getString(it) } else emptyList()
                        val branchId = TechLaneApp.instance.tokenStore.selectedBranchId
                            ?.takeIf { it in allowed }
                            ?: allowed.firstOrNull()
                        val payload = JSONObject()
                            .put("problem_summary", problem.ifBlank { "Offline draft repair" })
                            .put("customer_name", customerName.trim())
                            .put("customer_phone", customerPhone.trim())
                            .put("kind", kind)
                            .put("brand", brand.trim())
                            .put("model", model.trim())
                            .put("imei", imei.trim())
                            .put("anonymous", anonymous)
                            .put("price_agreed", priceAgreed)
                            .put("labor_amount", if (priceAgreed) chargeAmount.toDoubleOrNull() else null)
                            .put("customer_waiting", customerWaiting)
                            .put("estimated_wait_minutes", if (customerWaiting) waitMinutes.toIntOrNull() else null)
                            .put("promised_date", promisedDate)
                            .put("promised_time", promisedTime)
                            .put("intake_accessories", accessoriesText)
                            .put("intake_condition", conditionNotes)
                            .put("device_passcode", devicePasscode)
                        if (assignToMe) payload.put("technician_id", me.optString("id"))
                        if (branchId != null) payload.put("branch_id", branchId)
                        val entity = SyncCommandEntity(
                            actionId = UUID.randomUUID().toString(),
                            tenantId = me.optString("tenant_id", "local"),
                            branchId = branchId,
                            deviceId = TechLaneApp.instance.tokenStore.deviceId,
                            userId = me.optString("id", "local"),
                            commandType = SyncCommandTypes.REPAIR_CREATE_DRAFT,
                            localTimestamp = Instant.now().toString(),
                            payloadJson = payload.toString(),
                            syncStatus = "pending",
                        )
                        TechLaneApp.instance.database.syncOutboxDao().insert(entity)
                        onBack()
                    } catch (e: Exception) {
                        error = e.message
                    } finally {
                        busy = false
                    }
                }
            },
            enabled = !busy,
            modifier = Modifier.fillMaxWidth(),
        ) {
            Text("Save as offline draft")
        }
        }
    }
    if (showDiscardDialog) {
        AlertDialog(
            onDismissRequest = { showDiscardDialog = false },
            title = { Text("Discard this intake?") },
            text = { Text("Your entered customer and device details will be lost.") },
            dismissButton = { TextButton(onClick = { showDiscardDialog = false }) { Text("Keep editing") } },
            confirmButton = { TextButton(onClick = onBack) { Text("Discard") } },
        )
    }
}

@Composable
private fun RepairStageRail(status: String) {
    val stages = listOf(
        "intake" to "Intake",
        "diagnosed" to "Diagnosed",
        "waiting_parts" to "Parts",
        "in_progress" to "Repair",
        "ready_for_pickup" to "QC",
        "completed" to "Ready",
        "collected" to "Collected",
    )
    val current = stages.indexOfFirst { it.first == status }.coerceAtLeast(0)
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .horizontalScroll(rememberScrollState()),
        horizontalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        stages.forEachIndexed { index, (_, label) ->
            val active = index == current
            val done = index < current
            Surface(
                color = when {
                    active -> Brand.GoldTint
                    done -> Brand.NavyTint
                    else -> Brand.Subtle
                },
                shape = RoundedCornerShape(8.dp),
            ) {
                Text(
                    text = "${if (done) "✓" else index + 1}  $label",
                    modifier = Modifier.padding(horizontal = 10.dp, vertical = 7.dp),
                    style = MaterialTheme.typography.labelSmall,
                    fontWeight = if (active) FontWeight.Bold else FontWeight.Medium,
                    color = if (active || done) Brand.NavyDark else Brand.TextMuted,
                )
            }
        }
    }
}

/** Primary forward step for the bench — everything else is a side door. */
private fun primaryForwardStatus(current: String): String? = when (current) {
    "intake" -> "diagnosed"
    "diagnosed" -> "in_progress"
    "waiting_parts" -> "in_progress"
    "in_progress" -> "ready_for_pickup"
    "ready_for_pickup" -> "completed"
    else -> null
}

private fun statusActionLabel(status: String): String = when (status) {
    "diagnosed" -> "Mark diagnosed"
    "waiting_parts" -> "Waiting for parts"
    "in_progress" -> "Start / continue repair"
    "ready_for_pickup" -> "Send to QC"
    "completed" -> "Mark ready for customer"
    "cancelled" -> "Cancel job"
    "unrepairable" -> "Write off (BER)"
    else -> status.replace('_', ' ').replaceFirstChar { it.uppercase() }
}

private fun statusHint(current: String): String = when (current) {
    "intake" -> "Next: diagnose the fault, then get a price agreed before repair starts."
    "diagnosed" -> "Next: start repair once the customer (or a manager) has agreed the price."
    "waiting_parts" -> "Parts are outstanding. When they arrive, continue the repair."
    "in_progress" -> "When the repair is done, send it to QC — not straight to the customer."
    "ready_for_pickup" -> "QC passed? Mark ready for customer. Failed? Send back to repair."
    "completed" -> "Device is ready — the counter will collect payment and hand it over."
    else -> ""
}

@OptIn(ExperimentalMaterial3Api::class, ExperimentalFoundationApi::class)
@Composable
fun JobDetailScreen(
    jobId: String,
    onBack: () -> Unit,
    onOpenRelatedJob: (String) -> Unit = {},
    isTechnician: Boolean = false,
    canCollect: Boolean = false,
    canClose: Boolean = false,
    canAuthorize: Boolean = false,
    canReleaseUnverified: Boolean = false,
    canTakePayment: Boolean = false,
    modifier: Modifier = Modifier,
) {
    // Server-fetched caches re-derive from jobId on recreation — kept as `remember`.
    // Amounts/notes/references the staff typed mid-transaction survive rotation and
    // process death instead (e.g. a half-entered payment during a live counter sale).
    var job by remember { mutableStateOf<JSONObject?>(null) }
    var payments by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var parts by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var notes by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var estimates by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var myId by remember { mutableStateOf("") }
    var newNote by rememberSaveable { mutableStateOf("") }
    var amount by rememberSaveable { mutableStateOf("") }
    var phone by rememberSaveable { mutableStateOf("") }
    var method by rememberSaveable { mutableStateOf("cash") }
    var partDesc by rememberSaveable { mutableStateOf("") }
    var partSupplierId by rememberSaveable { mutableStateOf("") }
    var suppliers by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var unitCost by rememberSaveable { mutableStateOf("0") }
    var collectCodes by remember { mutableStateOf<Map<String, String>>(emptyMap()) }
    var labor by rememberSaveable { mutableStateOf("") }
    var overrideCharge by rememberSaveable { mutableStateOf(false) }
    var estimateTotal by rememberSaveable { mutableStateOf("") }
    var estimateNotes by rememberSaveable { mutableStateOf("") }
    var showEstimateDialog by remember { mutableStateOf(false) }
    var showEditAgreedAmount by remember { mutableStateOf(false) }
    var showCloseDialog by remember { mutableStateOf(false) }
    var authNote by rememberSaveable { mutableStateOf("") }
    var authAmount by rememberSaveable { mutableStateOf("") }
    var returnReason by rememberSaveable { mutableStateOf("") }
    var varianceReason by rememberSaveable { mutableStateOf("") }
    // Not saved on purpose — a revealed device passcode shouldn't linger in a
    // saved-instance Bundle any longer than the screen that revealed it.
    var revealedPasscode by remember { mutableStateOf("") }
    var error by remember { mutableStateOf<String?>(null) }
    var message by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    var stkPolling by remember { mutableStateOf(false) }
    var waitMessage by remember { mutableStateOf("Waiting for M-Pesa") }
    var waitDetail by remember { mutableStateOf<String?>("Approve the STK prompt on the phone.") }
    var promptHandover by remember { mutableStateOf(false) }
    var stockBalances by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var saleStockQuery by rememberSaveable { mutableStateOf("") }
    var saleStockKey by rememberSaveable { mutableStateOf("") }
    var saleQty by rememberSaveable { mutableStateOf("1") }
    var showWorkDetails by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    val snackbarHostState = LocalSnackbarHost.current
    val context = LocalContext.current
    val jobScroll = rememberScrollState()
    val handoverBringIntoView = remember { BringIntoViewRequester() }
    val overviewBringIntoView = remember { BringIntoViewRequester() }
    val workBringIntoView = remember { BringIntoViewRequester() }
    val partsBringIntoView = remember { BringIntoViewRequester() }
    val paymentBringIntoView = remember { BringIntoViewRequester() }
    val historyBringIntoView = remember { BringIntoViewRequester() }

    fun refresh() {
        scope.launch {
            try {
                job = withContext(Dispatchers.IO) { ApiClient.getRepair(jobId) }
                val payItems = withContext(Dispatchers.IO) { ApiClient.listRepairPayments(jobId) }
                payments = (0 until payItems.length()).map { payItems.getJSONObject(it) }
                val partItems = withContext(Dispatchers.IO) { ApiClient.listPartRequests(jobId) }
                parts = (0 until partItems.length()).map { partItems.getJSONObject(it) }
                val noteItems = withContext(Dispatchers.IO) { ApiClient.listRepairNotes(jobId) }
                notes = (0 until noteItems.length()).map { noteItems.getJSONObject(it) }
                val estimateItems = withContext(Dispatchers.IO) { ApiClient.listRepairEstimates(jobId) }
                estimates = (0 until estimateItems.length()).map { estimateItems.getJSONObject(it) }
                val supplierItems = withContext(Dispatchers.IO) { ApiClient.listSuppliers() }
                suppliers = (0 until supplierItems.length()).map { supplierItems.getJSONObject(it) }
                val balItems = withContext(Dispatchers.IO) { ApiClient.listInventoryBalances() }
                stockBalances = (0 until balItems.length()).map { balItems.getJSONObject(it) }
                    .filter { it.optInt("available_qty", 0) > 0 && it.optDouble("sell_price", 0.0) > 0 }
                if (partSupplierId.isBlank() && suppliers.isNotEmpty()) {
                    partSupplierId = suppliers.first().optString("id")
                }
                if (myId.isBlank()) {
                    myId = withContext(Dispatchers.IO) { ApiClient.me() }.optString("id")
                }
                error = null
            } catch (e: Exception) {
                error = e.message
            }
        }
    }

    LaunchedEffect(jobId) { refresh() }

    fun paymentIdFromCreate(res: JSONObject): String? {
        val items = res.optJSONArray("items")
        if (items != null && items.length() > 0) {
            for (i in 0 until items.length()) {
                val p = items.getJSONObject(i)
                if (p.optString("method") == "mpesa_stk") {
                    return p.optString("id").takeIf { it.isNotBlank() }
                }
            }
            return items.getJSONObject(0).optString("id").takeIf { it.isNotBlank() }
        }
        return res.optString("id").takeIf { it.isNotBlank() }
    }

    fun isTerminalStkError(msg: String): Boolean {
        val m = msg.lowercase()
        return listOf("1032", "1037", "cancelled", "canceled", "ds timeout", "request cancelled").any { m.contains(it) }
    }

    fun pollStkPayment(paymentId: String) {
        if (stkPolling || paymentId.isBlank()) return
        stkPolling = true
        error = null
        message = null
        waitMessage = "Waiting for M-Pesa"
        waitDetail = "Approve the STK prompt on the phone. We'll confirm payment automatically."
        scope.launch {
            try {
                repeat(48) {
                    delay(2500)
                    try {
                        val p = withContext(Dispatchers.IO) { ApiClient.confirmMpesaPayment(paymentId) }
                        val status = p.optString("status")
                        if (status == "allocated" || status == "confirmed") {
                            message = "Payment successful"
                            snackbarHostState?.let { host ->
                                launch { host.showSnackbar("Payment successful") }
                            }
                            refresh()
                            return@launch
                        }
                        if (status == "failed" || status == "cancelled") {
                            error = "STK $status"
                            return@launch
                        }
                    } catch (e: Exception) {
                        if (isTerminalStkError(e.message.orEmpty())) {
                            error = e.message
                            return@launch
                        }
                    }
                }
                error = "STK timed out — tap Check payment to try again"
            } finally {
                stkPolling = false
            }
        }
    }

    LaunchedEffect(job, payments) {
        val j = job ?: return@LaunchedEffect
        val paid = payments
            .filter { it.optString("status") in setOf("allocated", "confirmed", "pending_handover", "provisional") }
            .sumOf { it.optDouble("amount", 0.0) }
        val due = when {
            j.has("balance_due") && !j.isNull("balance_due") -> j.optDouble("balance_due", 0.0)
            else -> {
                val approvedEst = j.optDouble("approved_estimate_total", Double.NaN)
                val charge = if (!approvedEst.isNaN() && approvedEst > 0) approvedEst else j.optDouble("labor_amount", 0.0)
                (charge + j.optDouble("sale_lines_total", 0.0) - paid).coerceAtLeast(0.0)
            }
        }
        if (due > 0.009) {
            amount = due.toInt().toString()
        }
        val custPhone = j.optJSONObject("customer")?.optString("phone").orEmpty()
        if (custPhone.isNotBlank() && custPhone != "null" && phone.isBlank()) {
            phone = custPhone
        }
    }

    val j = job
    val detailTitle = j?.optString("job_code")?.ifBlank { null }
        ?: j?.getString("id")?.take(8)
        ?: "Job"
    val detailSubtitle = j?.optString("problem_summary")?.takeIf { it.isNotBlank() }
        ?: (error ?: if (j == null) "Loading…" else null)
    Box(
        modifier = modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background),
    ) {
    Column(
        modifier = Modifier.fillMaxSize(),
    ) {
        BrandDetailHeader(
            title = detailTitle,
            subtitle = detailSubtitle,
            navigation = {
                IconButton(onClick = onBack) {
                    Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back", tint = Color.White)
                }
            },
            trailing = {
                j?.let { StatusChip(it.optString("status")) }
            },
        )
        Column(
            modifier = Modifier
                .weight(1f)
                .verticalScroll(jobScroll)
                .padding(16.dp)
                .padding(bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
        if (j == null) {
            Text(error ?: "Loading…", color = Brand.TextSecondary)
        } else {
        if (j.optString("created_at").isNotBlank()) {
            Text(
                "Opened ${timeAgo(j.optString("created_at"))}",
                style = MaterialTheme.typography.bodySmall,
                color = Brand.TextMuted,
            )
        }
        FeedbackBanner(message = message, error = error)
        Spacer(Modifier.height(1.dp).bringIntoViewRequester(overviewBringIntoView))
        Row(Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            listOf(
                "Overview" to overviewBringIntoView, "Work" to workBringIntoView, "Parts" to partsBringIntoView,
                "Payment" to paymentBringIntoView, "History" to historyBringIntoView,
            ).forEach { (label, target) ->
                FilterChip(selected = false, onClick = { scope.launch { target.bringIntoView() } }, label = { Text(label) })
            }
        }
        RepairStageRail(j.optString("status"))
        if (j.optBoolean("has_device_passcode")) {
            BrandCard {
                BrandSectionTitle("Device access")
                Text(
                    if (revealedPasscode.isBlank()) "Passcode captured and encrypted" else "Passcode: $revealedPasscode",
                    color = Brand.TextPrimary,
                )
                TextButton(
                    onClick = {
                        busy = true
                        scope.launch {
                            try {
                                revealedPasscode = withContext(Dispatchers.IO) { ApiClient.revealRepairPasscode(jobId) }
                                message = "Passcode reveal recorded on the timeline"
                            } catch (e: Exception) {
                                error = e.message
                            } finally {
                                busy = false
                            }
                        }
                    },
                    enabled = !busy,
                ) {
                    Text(if (revealedPasscode.isBlank()) "Reveal passcode (audited)" else "Reveal again")
                }
            }
        }
        val parentJobId = j.optString("parent_job_id").takeIf { it.isNotBlank() && it != "null" }
        val parentJobCode = j.optString("parent_job_code").takeIf { it.isNotBlank() && it != "null" }
        val reworkReason = j.optString("rework_reason").takeIf { it.isNotBlank() && it != "null" }
        if (parentJobId != null) {
            Surface(color = Brand.GoldTint, shape = RoundedCornerShape(8.dp)) {
                Text(
                    buildString {
                        append("Customer return of ")
                        append(parentJobCode ?: parentJobId.take(8))
                        if (reworkReason != null) append(" · $reworkReason")
                    },
                    modifier = Modifier.padding(12.dp),
                    color = Brand.NavyDark,
                    style = MaterialTheme.typography.bodySmall,
                    fontWeight = FontWeight.SemiBold,
                )
            }
        }
        val canOpenReturn = j.optString("status") in setOf("completed", "collected") && parentJobId == null
        if (canOpenReturn) {
            BrandCard {
                Text("Customer returned this device?", fontWeight = FontWeight.SemiBold, color = Brand.NavyDark)
                Text(
                    "Opens a linked follow-up job on the same customer and device.",
                    style = MaterialTheme.typography.bodySmall,
                    color = Brand.TextSecondary,
                )
                OutlinedTextField(
                    value = returnReason,
                    onValueChange = { returnReason = it },
                    label = { Text("What came back / what failed?") },
                    modifier = Modifier.fillMaxWidth(),
                    singleLine = true,
                )
                GoldButton(
                    text = if (busy) "Opening…" else "Open return job",
                    onClick = {
                        val reason = returnReason.trim()
                        if (reason.isBlank()) {
                            error = "Enter why the device came back"
                            return@GoldButton
                        }
                        busy = true
                        error = null
                        scope.launch {
                            try {
                                val created = withContext(Dispatchers.IO) {
                                    ApiClient.createRepairRework(jobId, reason)
                                }
                                returnReason = ""
                                message = "Return job opened"
                                onOpenRelatedJob(created.getString("id"))
                            } catch (e: Exception) {
                                error = e.message ?: "Could not open return job"
                            } finally {
                                busy = false
                            }
                        }
                    },
                    enabled = !busy && returnReason.isNotBlank(),
                    loading = busy,
                    modifier = Modifier.fillMaxWidth(),
                )
            }
        }
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.fillMaxWidth()) {
            GoldButton(
                text = "Print intake slip",
                onClick = {
                    busy = true
                    scope.launch {
                        try {
                            val html = withContext(Dispatchers.IO) {
                                PrintSupport.fetchText(
                                    "${com.techlane.ops.BuildConfig.API_BASE}/repairs/$jobId/intake-slip.html",
                                    TechLaneApp.instance.tokenStore.accessToken,
                                )
                            }
                            PrintSupport.printHtml(context, html, "Intake slip")
                            message = "Printer sheet opened — tap your printer"
                        } catch (e: Exception) {
                            error = e.message ?: "Could not print intake slip"
                        } finally {
                            busy = false
                        }
                    }
                },
                enabled = !busy,
                loading = busy,
                modifier = Modifier.weight(1f),
            )
            OutlinedButton(
                onClick = {
                    busy = true
                    scope.launch {
                        try {
                            val html = withContext(Dispatchers.IO) {
                                PrintSupport.fetchText(
                                    "${com.techlane.ops.BuildConfig.API_BASE}/repairs/$jobId/receipt.html",
                                    TechLaneApp.instance.tokenStore.accessToken,
                                )
                            }
                            PrintSupport.printHtml(context, html, "Repair receipt")
                            message = "Printer sheet opened — tap your printer"
                        } catch (e: Exception) {
                            error = e.message ?: "Could not print receipt"
                        } finally {
                            busy = false
                        }
                    }
                },
                enabled = !busy,
                modifier = Modifier.weight(1f),
            ) {
                Text("Print receipt")
            }
        }
        j.optString("pickup_code").takeIf { it.isNotBlank() && it != "null" }?.let { pickup ->
            BrandCard {
                BrandSectionTitle("Pickup code")
                Text(pickup, style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold, color = Brand.Navy)
                Text(
                    "Printed as QR on the intake slip; the code itself is sent by intake SMS. Scanning or typing this releases the device only after payment.",
                    style = MaterialTheme.typography.bodySmall,
                    color = Brand.TextSecondary,
                )
            }
        }

        BrandCard {
                    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                val customer = j.optJSONObject("customer")
                val device = j.optJSONObject("device")
                Text(customer?.optString("full_name")?.ifBlank { null } ?: "Walk-in customer", style = MaterialTheme.typography.titleSmall)
                customer?.optString("phone")?.takeIf { it.isNotBlank() }?.let {
                    Text(it, style = MaterialTheme.typography.bodySmall)
                }
                if (device != null) {
                    val desc = listOf(device.optString("kind"), device.optString("brand"), device.optString("model"))
                        .filter { it.isNotBlank() }.joinToString(" ")
                    Text(desc.ifBlank { "Unknown device" }, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    device.optString("imei").takeIf { it.isNotBlank() }?.let {
                        Text("IMEI $it", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                }
            }
        }
        RepairAttachmentsPanel(jobId)
        if (isTechnician) {
            Text(
                "Bench workflow — claim, update status, note diagnosis, request parts",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }

        val assignedTo = j.optString("technician_id")
        if (myId.isNotBlank() && assignedTo != myId) {
            GoldButton(
                text = if (assignedTo.isBlank() || assignedTo == "null") "Claim this job" else "Reassign to me",
                onClick = {
                    busy = true
                    error = null
                    scope.launch {
                        try {
                            withContext(Dispatchers.IO) { ApiClient.assignRepair(jobId, myId) }
                            message = "Job assigned to you"
                            refresh()
                        } catch (e: Exception) {
                            error = e.message
                        } finally {
                            busy = false
                        }
                    }
                },
                enabled = !busy,
                loading = busy,
                modifier = Modifier.fillMaxWidth(),
            )
        }

        val nextArr = j.optJSONArray("next_statuses")
        val allNext = nextArr?.let { a -> (0 until a.length()).map { a.getString(it) } } ?: emptyList()
        val closureOptions = allNext.filter { it == "cancelled" || it == "unrepairable" }
        val nextList = allNext - closureOptions.toSet()
        val paidTotal = payments
            .filter { it.optString("status") in setOf("allocated", "confirmed", "pending_handover", "provisional") }
            .sumOf { it.optDouble("amount", 0.0) }
        // Prefer server amount_due / balance_due so estimate + sale-lines match web / handover.
        val amountDue = when {
            j.has("amount_due") && !j.isNull("amount_due") -> j.optDouble("amount_due", 0.0)
            else -> {
                val approvedEst = j.optDouble("approved_estimate_total", Double.NaN)
                val charge = if (!approvedEst.isNaN() && approvedEst > 0) approvedEst else j.optDouble("labor_amount", 0.0)
                charge + j.optDouble("sale_lines_total", 0.0)
            }
        }
        val balanceDue = when {
            j.has("balance_due") && !j.isNull("balance_due") -> j.optDouble("balance_due", 0.0)
            else -> (amountDue - paidTotal).coerceAtLeast(0.0)
        }
        val paymentLocked = amountDue > 0.009 && balanceDue <= 0.009
        val saleLines = j.optJSONArray("sale_lines")?.let { arr ->
            (0 until arr.length()).map { arr.getJSONObject(it) }
        } ?: emptyList()

        Spacer(Modifier.height(1.dp).bringIntoViewRequester(workBringIntoView))
        val authObj = j.optJSONObject("authorization")
        val authorizedAt = authObj?.optString("authorized_at").orEmpty()
        val hasAuthorization = authorizedAt.isNotBlank() && authorizedAt != "null"
        val authorizedAmount = if (authObj?.has("authorized_amount") == true) {
            authObj.optDouble("authorized_amount", 0.0)
        } else {
            null
        }
        val jobStatus = j.optString("status")
        val needsAuthorization = !hasAuthorization && (jobStatus == "intake" || jobStatus == "diagnosed")
        val canRequestParts = jobStatus in setOf("intake", "diagnosed", "waiting_parts", "in_progress")
        val notesEditable = canRequestParts

        if (hasAuthorization) {
            Text(
                "Price agreed: KES ${(authorizedAmount ?: 0.0).toInt()}" +
                    when (authObj?.optString("source")) {
                        "customer_estimate" -> " (customer approved the estimate)"
                        "manager_override" -> " (go-ahead recorded by staff)"
                        else -> ""
                    },
                style = MaterialTheme.typography.bodySmall,
                color = Brand.TextSecondary,
            )
        }

        if (hasAuthorization && canAuthorize && jobStatus != "collected") {
            if (paidTotal <= 0.009) {
                TextButton(onClick = {
                    showEditAgreedAmount = !showEditAgreedAmount
                    if (showEditAgreedAmount) authAmount = (authorizedAmount ?: j.optDouble("labor_amount", 0.0)).toInt().toString()
                }) { Text(if (showEditAgreedAmount) "Cancel amount correction" else "Edit agreed amount") }
                if (showEditAgreedAmount) {
                    Surface(color = Brand.GoldTint, shape = RoundedCornerShape(12.dp), modifier = Modifier.fillMaxWidth()) {
                        Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                            Text("Correct agreed amount", fontWeight = FontWeight.SemiBold, color = Brand.NavyDark)
                            Text("The old amount remains visible in the audit timeline.", style = MaterialTheme.typography.bodySmall, color = Brand.TextSecondary)
                            OutlinedTextField(
                                value = authAmount,
                                onValueChange = { authAmount = it.filter { ch -> ch.isDigit() || ch == '.' } },
                                label = { Text("Correct amount (KES)") },
                                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
                                modifier = Modifier.fillMaxWidth(), singleLine = true,
                            )
                            OutlinedTextField(
                                value = authNote, onValueChange = { authNote = it },
                                label = { Text("Reason for correction") },
                                supportingText = { Text("Required for audit") },
                                modifier = Modifier.fillMaxWidth(),
                            )
                            GoldButton(
                                text = if (busy) "Saving…" else "Save corrected amount",
                                onClick = {
                                    val corrected = authAmount.toDoubleOrNull()
                                    if (corrected == null || corrected <= 0) { error = "Enter a valid corrected amount"; return@GoldButton }
                                    if (authNote.isBlank()) { error = "Enter why the amount is being corrected"; return@GoldButton }
                                    busy = true; error = null
                                    scope.launch {
                                        try {
                                            withContext(Dispatchers.IO) { ApiClient.authorizeRepairWork(jobId, corrected, authNote.trim()) }
                                            message = "Agreed amount corrected"
                                            authAmount = ""; authNote = ""; showEditAgreedAmount = false; refresh()
                                        } catch (e: Exception) { error = e.message } finally { busy = false }
                                    }
                                },
                                enabled = !busy, modifier = Modifier.fillMaxWidth(),
                            )
                        }
                    }
                }
            } else {
                Text("Agreed amount is locked because payment has started.", style = MaterialTheme.typography.bodySmall, color = Brand.TextMuted)
            }
        }

        if (needsAuthorization) {
            BrandSectionTitle("Authorize work")
            Text(
                "Nobody has agreed to a price yet, so this job cannot go on the bench. Send an estimate for the " +
                    "customer to approve, or record their verbal go-ahead.",
                style = MaterialTheme.typography.bodySmall,
                color = Brand.Danger,
            )
            GoldButton(
                text = "Send customer estimate",
                onClick = { showEstimateDialog = true },
                enabled = !busy && canRequestParts,
                modifier = Modifier.fillMaxWidth(),
            )
            if (canAuthorize) {
                OutlinedTextField(
                    value = authAmount,
                    onValueChange = { authAmount = it },
                    label = { Text("Agreed price (KES)") },
                    modifier = Modifier.fillMaxWidth(),
                )
                OutlinedTextField(
                    value = authNote,
                    onValueChange = { authNote = it },
                    label = { Text("How did they agree?") },
                    modifier = Modifier.fillMaxWidth(),
                )
                Button(
                    onClick = {
                        if (authNote.isBlank()) {
                            error = "Record how the customer agreed — this is the audit trail for the price"
                            return@Button
                        }
                        busy = true
                        error = null
                        scope.launch {
                            try {
                                withContext(Dispatchers.IO) {
                                    ApiClient.authorizeRepairWork(jobId, authAmount.toDoubleOrNull(), authNote.trim())
                                }
                                authNote = ""
                                authAmount = ""
                                message = "Work authorized"
                                refresh()
                            } catch (e: Exception) {
                                error = e.message
                            } finally {
                                busy = false
                            }
                        }
                    },
                    enabled = !busy && authNote.isNotBlank(),
                    modifier = Modifier.fillMaxWidth(),
                ) {
                    Text("Authorize work")
                }
            } else {
                Text(
                    "Only a manager or owner can record a go-ahead without a written estimate.",
                    style = MaterialTheme.typography.bodySmall,
                    color = Brand.TextSecondary,
                )
            }
        }

        val overrun = if (authorizedAmount != null) {
            ((labor.toDoubleOrNull() ?: 0.0) - authorizedAmount).coerceAtLeast(0.0)
        } else {
            0.0
        }
        val outstandingParts = parts.filter {
            it.optString("status") in setOf("pending", "approved")
        }
        val pendingEstimates = estimates.filter { it.optString("status") == "pending" }

        val forwardStatuses = nextList.filter { it != "collected" }
        if (outstandingParts.isNotEmpty()) {
            Surface(color = Brand.GoldTint, shape = RoundedCornerShape(8.dp)) {
                Text(
                    "Waiting on ${outstandingParts.size} part(s) from supplier — finish or cancel them before marking ready/complete.",
                    modifier = Modifier.padding(12.dp),
                    color = Brand.NavyDark,
                    style = MaterialTheme.typography.bodySmall,
                )
            }
        }
        if (pendingEstimates.isNotEmpty()) {
            Surface(color = Brand.GoldTint, shape = RoundedCornerShape(8.dp)) {
                Text(
                    "Customer estimate still pending — wait for approval or cancel it before finishing.",
                    modifier = Modifier.padding(12.dp),
                    color = Brand.NavyDark,
                    style = MaterialTheme.typography.bodySmall,
                )
            }
        }
        if (forwardStatuses.isNotEmpty()) {
            BrandSectionTitle("Next step")
            val isQuickJob = j.optString("service_type") == "quick_replacement"
            (if (isQuickJob) when (jobStatus) {
                "in_progress" -> "Fit the replacement, test the device, then send it to final check."
                "waiting_parts" -> "Part unavailable. Resume fitting when stock arrives."
                "ready_for_pickup" -> "Final test passed? Complete the quick job."
                else -> statusHint(jobStatus)
            } else statusHint(jobStatus)).takeIf { it.isNotBlank() }?.let {
                Text(it, style = MaterialTheme.typography.bodySmall, color = Brand.TextSecondary)
            }
            if (nextList.contains("completed")) {
                val agreed = authorizedAmount
                    ?: j.optDouble("labor_amount", 0.0).takeIf { it > 0 }
                if (agreed != null && agreed > 0 && !overrideCharge) {
                    Text(
                        "Customer will be charged the agreed price: KES ${agreed.toInt()}",
                        style = MaterialTheme.typography.bodyMedium,
                        color = Brand.TextPrimary,
                    )
                    TextButton(onClick = { overrideCharge = true }) {
                        Text("Charge a different amount")
                    }
                } else {
                    OutlinedTextField(
                        value = labor,
                        onValueChange = { labor = it },
                        label = { Text("Final charge (KES)") },
                        modifier = Modifier.fillMaxWidth(),
                    )
                    if (overrun > 1.0) {
                        Text(
                            "This is KES ${overrun.toInt()} more than the KES ${authorizedAmount!!.toInt()} the customer " +
                                "agreed to — explain why before completing.",
                            style = MaterialTheme.typography.bodySmall,
                            color = Brand.Danger,
                        )
                        OutlinedTextField(
                            value = varianceReason,
                            onValueChange = { varianceReason = it },
                            label = { Text("Reason for the higher charge") },
                            modifier = Modifier.fillMaxWidth(),
                        )
                    }
                    if (agreed != null && agreed > 0) {
                        TextButton(onClick = {
                            overrideCharge = false
                            labor = ""
                            varianceReason = ""
                        }) {
                            Text("Use agreed price instead")
                        }
                    }
                }
            }
            val primary = primaryForwardStatus(jobStatus)?.takeIf { it in forwardStatuses }
            val secondary = forwardStatuses.filter { it != primary }
            fun blockedFinish(status: String): Boolean {
                val needsClear = status == "ready_for_pickup" || status == "completed" ||
                    (status == "in_progress" && jobStatus == "waiting_parts")
                if (!needsClear) return false
                if (outstandingParts.isNotEmpty()) return true
                if ((status == "ready_for_pickup" || status == "completed") && pendingEstimates.isNotEmpty()) {
                    return true
                }
                return false
            }
            fun advanceTo(status: String) {
                busy = true
                error = null
                message = null
                scope.launch {
                    try {
                        val agreed = authorizedAmount
                            ?: j.optDouble("labor_amount", 0.0).takeIf { it > 0 }
                        val finalLabor = when {
                            status != "completed" -> null
                            // Agreed price already on the job — don't re-ask for "labor".
                            !overrideCharge && agreed != null && agreed > 0 -> null
                            else -> labor.toDoubleOrNull()
                        }
                        if (status == "completed" && (overrideCharge || agreed == null || agreed <= 0) && finalLabor == null) {
                            error = "Enter the final charge amount"
                            busy = false
                            return@launch
                        }
                        withContext(Dispatchers.IO) {
                            ApiClient.updateRepairStatus(
                                jobId,
                                status,
                                finalLabor,
                                varianceReason = if (status == "completed" && overrideCharge) {
                                    varianceReason.trim().ifBlank { null }
                                } else {
                                    null
                                },
                            )
                        }
                        message = statusActionLabel(status)
                        overrideCharge = false
                        refresh()
                    } catch (e: Exception) {
                        error = e.message
                    } finally {
                        busy = false
                    }
                }
            }
            if (primary != null) {
                val blockedByAuth = primary == "in_progress" && needsAuthorization
                val blocked = blockedByAuth || blockedFinish(primary)
                GoldButton(
                    text = if (isQuickJob) when (primary) { "ready_for_pickup" -> "Replacement fitted · test"; "completed" -> "Complete quick job"; "in_progress" -> "Start fitting"; else -> statusActionLabel(primary) } else statusActionLabel(primary),
                    onClick = { advanceTo(primary) },
                    enabled = !busy && !blocked,
                    loading = busy,
                    modifier = Modifier.fillMaxWidth(),
                )
                if (blockedByAuth) {
                    Text(
                        "Blocked until a price is agreed (estimate approved or manager go-ahead).",
                        style = MaterialTheme.typography.bodySmall,
                        color = Brand.Danger,
                    )
                    GoldButton(
                        text = "Send customer estimate",
                        onClick = { showEstimateDialog = true },
                        enabled = !busy && canRequestParts,
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
            }
            if (secondary.isNotEmpty()) {
                Text(
                    "Other moves",
                    style = MaterialTheme.typography.labelMedium,
                    color = Brand.TextMuted,
                )
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    for (status in secondary) {
                        val blockedByAuth = status == "in_progress" && needsAuthorization
                        val blocked = blockedByAuth || blockedFinish(status)
                        OutlinedButton(
                            onClick = { advanceTo(status) },
                            enabled = !busy && !blocked,
                            modifier = Modifier.fillMaxWidth(),
                        ) {
                            Text(statusActionLabel(status))
                        }
                    }
                }
            }
        }

        val closureReason = j.optString("closure_reason")
        if (closureReason.isNotBlank() && closureReason != "null") {
            BrandCard {
                Text(
                    "Closed: ${closureReasonLabel(closureReason)}",
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.SemiBold,
                    color = Brand.Danger,
                )
                if (j.optString("status") != "collected") {
                    Text(
                        "Device still needs to be returned to the customer.",
                        style = MaterialTheme.typography.bodySmall,
                        color = Brand.TextSecondary,
                    )
                }
            }
        }

        // Accessories sold onto the job join the same balance as the repair.
        if (canTakePayment && jobStatus != "collected") {
            Spacer(Modifier.height(1.dp).bringIntoViewRequester(partsBringIntoView))
            BrandSectionTitle("Accessories & extras")
            Text(
                "Add cases, chargers, glass — they join this job’s bill for one cash/STK payment.",
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                style = MaterialTheme.typography.bodySmall,
            )
            saleLines.forEach { line ->
                BrandCard {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                    ) {
                        Column(modifier = Modifier.weight(1f)) {
                            Text(line.optString("description"), fontWeight = FontWeight.SemiBold)
                            Text(
                                "Reserved to this job · ×${line.optInt("quantity", 1)} · KES ${line.optDouble("line_total", 0.0).toInt()}",
                                style = MaterialTheme.typography.bodySmall,
                                color = Brand.TextSecondary,
                            )
                        }
                        if (!paymentLocked) {
                            TextButton(
                                onClick = {
                                    busy = true
                                    scope.launch {
                                        try {
                                            withContext(Dispatchers.IO) {
                                                ApiClient.removeRepairSaleLine(jobId, line.getString("id"))
                                            }
                                            message = "Accessory removed"
                                            refresh()
                                        } catch (e: Exception) {
                                            error = e.message
                                        } finally {
                                            busy = false
                                        }
                                    }
                                },
                                enabled = !busy,
                            ) { Text("Remove") }
                        }
                    }
                }
            }
            if (!paymentLocked && stockBalances.isNotEmpty()) {
                val normalizedQuery = saleStockQuery.trim()
                val matchingStock = if (normalizedQuery.length < 2) emptyList() else stockBalances.filter { item ->
                    listOf(item.optString("product_name"), item.optString("variant_name"), item.optString("sku"), item.optString("barcode"))
                        .any { it.contains(normalizedQuery, ignoreCase = true) }
                }.take(8)
                val labels = matchingStock.map {
                    val key = "${it.optString("variant_id")}:${it.optString("location_id")}"
                    key to "${it.optString("product_name")} · KES ${it.optDouble("sell_price", 0.0).toInt()} · ${it.optInt("available_qty")} left"
                }
                SectionLabel("Add from stock")
                OutlinedTextField(
                    value = saleStockQuery,
                    onValueChange = { saleStockQuery = it; saleStockKey = "" },
                    label = { Text("Search stock") },
                    placeholder = { Text("Product, SKU or barcode") },
                    supportingText = { Text(if (normalizedQuery.length < 2) "Type at least 2 characters" else "${labels.size} result${if (labels.size == 1) "" else "s"}") },
                    modifier = Modifier.fillMaxWidth(),
                    singleLine = true,
                )
                if (normalizedQuery.length >= 2 && labels.isEmpty()) {
                    Text("No available stock matches ‘$normalizedQuery’.", style = MaterialTheme.typography.bodySmall, color = Brand.TextSecondary)
                }
                labels.forEach { (key, label) ->
                    FilterChip(
                        selected = saleStockKey == key,
                        onClick = { saleStockKey = key },
                        label = { Text(label, maxLines = 1) },
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
                OutlinedTextField(
                    value = saleQty,
                    onValueChange = { saleQty = it },
                    label = { Text("Qty") },
                    modifier = Modifier.fillMaxWidth(),
                    singleLine = true,
                )
                GoldButton(
                    text = if (busy) "Adding…" else "Add to bill",
                    onClick = {
                        val parts = saleStockKey.split(":")
                        val qty = saleQty.toIntOrNull() ?: 1
                        if (parts.size != 2) {
                            error = "Select a stock item"
                            return@GoldButton
                        }
                        busy = true
                        error = null
                        scope.launch {
                            try {
                                withContext(Dispatchers.IO) {
                                    ApiClient.addRepairSaleLine(jobId, parts[0], parts[1], qty.coerceAtLeast(1))
                                }
                                message = "Stock reserved and added to this job"
                                saleQty = "1"
                                saleStockQuery = ""
                                saleStockKey = ""
                                refresh()
                            } catch (e: Exception) {
                                error = e.message
                            } finally {
                                busy = false
                            }
                        }
                    },
                    enabled = !busy && saleStockKey.isNotBlank(),
                    loading = busy,
                    modifier = Modifier.fillMaxWidth(),
                )
            }
            if (j.optDouble("sale_lines_total", 0.0) > 0.009) {
                Text(
                    "Accessories subtotal: KES ${j.optDouble("sale_lines_total", 0.0).toInt()}",
                    fontWeight = FontWeight.SemiBold,
                )
            }
        }

        Spacer(Modifier.height(1.dp).bringIntoViewRequester(paymentBringIntoView))
        // Payment must sit above handover — otherwise staff only see "take payment first" with nowhere to pay.
        if (canTakePayment && (balanceDue > 0.009 || jobStatus in setOf("completed", "ready_for_pickup", "in_progress", "diagnosed"))) {
            BrandSectionTitle(if (balanceDue > 0.009) "Take payment" else "Payment")
            if (balanceDue > 0.009) {
                Text(
                    "KES ${balanceDue.toInt()} still owed before the device can be released.",
                    style = MaterialTheme.typography.bodyMedium,
                    color = Brand.Danger,
                    fontWeight = FontWeight.SemiBold,
                )
            } else if (paymentLocked) {
                Text(
                    "Fully paid. Recording is locked to prevent duplicates — use a refund to reverse cash.",
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            } else {
                Text(
                    "Cash completes immediately and is tracked in the till. STK and paybill wait for provider confirmation.",
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            if (!paymentLocked) {
            OutlinedTextField(
                value = amount,
                onValueChange = { amount = it },
                label = { Text("Amount (KES)") },
                modifier = Modifier.fillMaxWidth(),
            )
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                listOf(
                    "cash" to "Cash",
                    "mpesa_stk" to "STK",
                    "mpesa_c2b" to "Paybill",
                ).forEach { (key, label) ->
                    FilterChip(
                        selected = method == key,
                        onClick = { method = key },
                        label = { Text(label) },
                        colors = FilterChipDefaults.filterChipColors(
                            selectedContainerColor = MaterialTheme.colorScheme.primaryContainer,
                            selectedLabelColor = MaterialTheme.colorScheme.onPrimaryContainer,
                        ),
                    )
                }
            }
            if (method == "mpesa_stk") {
                OutlinedTextField(
                    value = phone,
                    onValueChange = { phone = it },
                    label = { Text("Customer phone") },
                    modifier = Modifier.fillMaxWidth(),
                )
            }
            if (method == "mpesa_c2b") {
                Text(
                    "Customer pays your Till/Paybill using account ref ${j.optString("job_code").ifBlank { jobId.take(8) }}.",
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            GoldButton(
                text = when {
                    busy -> "Working…"
                    method == "mpesa_stk" -> "Send STK push"
                    method == "mpesa_c2b" -> "Await paybill"
                    else -> "Record cash"
                },
                onClick = {
                    val value = amount.toDoubleOrNull()
                    if (value == null || value <= 0) {
                        error = "Enter a positive amount"
                        return@GoldButton
                    }
                    if (method != "cash" && method != "mpesa_stk" && method != "mpesa_c2b") {
                        error = "Use cash, STK, or paybill"
                        return@GoldButton
                    }
                    if (method == "mpesa_stk" && phone.isBlank()) {
                        error = "Phone required for STK"
                        return@GoldButton
                    }
                    busy = true
                    error = null
                    message = null
                    scope.launch {
                        try {
                            val created = withContext(Dispatchers.IO) {
                                try {
                                    ApiClient.createPayment(
                                        method = method,
                                        amount = value,
                                        payableType = "repair",
                                        payableId = jobId,
                                        branchId = j.optString("branch_id").ifBlank { null },
                                        phone = phone.ifBlank { null },
                                        accountRef = j.optString("job_code").ifBlank { jobId.take(8) },
                                    )
                                } catch (e: Exception) {
                                    // Cash can queue offline; STK/C2B need the network — but never
                                    // hide a real API error (credentials, phone, Daraja) behind
                                    // "Online required".
                                    if (method != "cash") throw e
                                    if (e is java.io.IOException) {
                                        OutboxRepository.enqueue(
                                            SyncCommandTypes.PAYMENTS_CASH_PROVISIONAL,
                                            JSONObject()
                                                .put("branch_id", j.optString("branch_id"))
                                                .put("payable_type", "repair")
                                                .put("payable_id", jobId)
                                                .put("amount", value)
                                                .put("currency", "KES"),
                                        )
                                        null
                                    } else {
                                        throw e
                                    }
                                }
                            }
                            message = when (method) {
                                "mpesa_stk" -> "STK sent"
                                "mpesa_c2b" -> "Awaiting paybill · ref ${j.optString("job_code").ifBlank { jobId.take(8) }}"
                                else -> "Cash recorded"
                            }
                            snackbarHostState?.let { host -> scope.launch { host.showSnackbar(message ?: "Payment recorded") } }
                            amount = ""
                            val willClear = amountDue > 0.009 && value + 0.009 >= balanceDue
                            if (willClear && method == "cash") {
                                promptHandover = true
                            }
                            refresh()
                            if (willClear && method == "cash") {
                                handoverBringIntoView.bringIntoView()
                            }
                            if (method == "mpesa_stk" && created != null) {
                                paymentIdFromCreate(created)?.let { pollStkPayment(it) }
                            }
                        } catch (e: Exception) {
                            error = e.message
                        } finally {
                            busy = false
                        }
                    }
                },
                enabled = !busy && !stkPolling,
                loading = busy,
                modifier = Modifier.fillMaxWidth(),
            )
            }
            if (payments.isNotEmpty()) {
                Text("Payments on this job", style = MaterialTheme.typography.titleSmall)
                payments.forEach { p ->
                    BrandCard {
                        Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                            Text(
                                "${p.optString("method")} · KES ${p.optDouble("amount", 0.0).toInt()}",
                                style = MaterialTheme.typography.titleSmall,
                            )
                            Text(p.optString("status"))
                            val st = p.optString("status")
                            if (p.optString("method") == "mpesa_stk" && (st == "initiated" || st == "pending")) {
                                Button(
                                    onClick = {
                                        pollStkPayment(p.getString("id"))
                                    },
                                    enabled = !busy && !stkPolling,
                                ) {
                                    Text("Check payment")
                                }
                            }
                        }
                    }
                }
            }
        }

        val handover = j.optJSONObject("handover")
        val collectable = jobStatus == "ready_for_pickup" || jobStatus == "completed" ||
            jobStatus == "cancelled" || jobStatus == "unrepairable"
        if (handover != null) {
            BrandSectionTitle("Handed over")
            BrandCard {
                val who = handover.optString("collected_by_name")
                val rel = handover.optString("relationship")
                Text(
                    if (rel.isNotBlank() && rel != "self") "$who ($rel) collected this device" else "$who collected this device",
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.SemiBold,
                )
                val method = handover.optString("verification_method")
                Text(
                    when (method) {
                        "otp" -> "Confirmed by a code on the owner's phone"
                        "pickup_code" -> "Confirmed by intake slip / QR"
                        else -> "Released by staff without a code"
                    },
                    style = MaterialTheme.typography.bodySmall,
                    color = if (method == "otp" || method == "pickup_code") Brand.TextSecondary else Brand.Danger,
                )
                handover.optString("id_number").takeIf { it.isNotBlank() && it != "null" }?.let {
                    Text("ID recorded: $it", style = MaterialTheme.typography.bodySmall, color = Brand.TextSecondary)
                }
            }
        } else if (collectable && canCollect) {
            if (!canTakePayment && balanceDue > 0.009) {
                Text(
                    "KES ${balanceDue.toInt()} is still owed. A cashier or owner needs to take Cash / STK payment before release.",
                    style = MaterialTheme.typography.bodySmall,
                    color = Brand.Danger,
                )
            }
            HandoverSection(
                jobId = jobId,
                defaultName = j.optJSONObject("customer")?.optString("full_name") ?: "",
                balanceDue = balanceDue,
                canVouch = canReleaseUnverified,
                busy = busy,
                setBusy = { busy = it },
                onError = { error = it },
                onMessage = { message = it },
                refresh = { refresh() },
                scope = scope,
                startRelease = promptHandover && balanceDue <= 0.009,
                bringIntoViewRequester = handoverBringIntoView,
            )
        }

        if (closureOptions.isNotEmpty() && canClose) {
            TextButton(onClick = { showCloseDialog = true }, enabled = !busy) {
                Text("Close without repairing", color = Brand.Danger)
            }
        }

        if (showCloseDialog) {
            CloseJobDialog(
                options = closureOptions,
                reasonsByStatus = j.optJSONObject("closure_reasons"),
                busy = busy,
                onDismiss = { showCloseDialog = false },
                onConfirm = { status, reason, note ->
                    busy = true
                    error = null
                    message = null
                    scope.launch {
                        try {
                            withContext(Dispatchers.IO) {
                                ApiClient.updateRepairStatus(jobId, status, closureReason = reason, note = note)
                            }
                            showCloseDialog = false
                            message = "Job closed as ${status.replace('_', ' ')}"
                            refresh()
                        } catch (e: Exception) {
                            error = e.message
                        } finally {
                            busy = false
                        }
                    }
                },
            )
        }

        if (showEstimateDialog) {
            AlertDialog(
                onDismissRequest = { if (!busy) showEstimateDialog = false },
                title = { Text("Customer estimate") },
                text = {
                    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                        Text(
                            "Customer only sees this total — not labor or parts breakdown.",
                            style = MaterialTheme.typography.bodySmall,
                            color = Brand.TextSecondary,
                        )
                        OutlinedTextField(
                            value = estimateTotal,
                            onValueChange = { estimateTotal = it },
                            label = { Text("Total amount (KES)") },
                            singleLine = true,
                            modifier = Modifier.fillMaxWidth(),
                        )
                        OutlinedTextField(
                            value = estimateNotes,
                            onValueChange = { estimateNotes = it },
                            label = { Text("Notes (optional)") },
                            modifier = Modifier.fillMaxWidth(),
                        )
                    }
                },
                confirmButton = {
                    Button(
                        onClick = {
                            val total = estimateTotal.toDoubleOrNull()
                            if (total == null || total <= 0) {
                                error = "Enter a total greater than zero"
                                return@Button
                            }
                            busy = true
                            error = null
                            message = null
                            scope.launch {
                                try {
                                    withContext(Dispatchers.IO) {
                                        ApiClient.createRepairEstimate(
                                            jobId,
                                            total,
                                            estimateNotes.trim().ifBlank { null },
                                        )
                                    }
                                    estimateTotal = ""
                                    estimateNotes = ""
                                    showEstimateDialog = false
                                    message = "Estimate sent to customer"
                                    refresh()
                                } catch (e: Exception) {
                                    error = e.message
                                } finally {
                                    busy = false
                                }
                            }
                        },
                        enabled = !busy && estimateTotal.isNotBlank(),
                    ) {
                        Text(if (busy) "Sending…" else "Send estimate")
                    }
                },
                dismissButton = {
                    TextButton(onClick = { showEstimateDialog = false }, enabled = !busy) {
                        Text("Cancel")
                    }
                },
            )
        }

        Spacer(Modifier.height(1.dp).bringIntoViewRequester(historyBringIntoView))
        OutlinedButton(
            onClick = { showWorkDetails = !showWorkDetails },
            modifier = Modifier.fillMaxWidth(),
        ) { Text(if (showWorkDetails) "Hide notes, parts & estimates" else "Show notes, parts & estimates") }
        if (showWorkDetails) {
        BrandSectionTitle("Notes")
        if (notesEditable) {
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.fillMaxWidth()) {
                OutlinedTextField(
                    value = newNote,
                    onValueChange = { newNote = it },
                    label = { Text("Diagnosis / work done") },
                    modifier = Modifier.weight(1f),
                )
                Button(
                    onClick = {
                        if (newNote.isBlank()) return@Button
                        busy = true
                        scope.launch {
                            try {
                                withContext(Dispatchers.IO) {
                                    try {
                                        ApiClient.addRepairNote(jobId, newNote.trim())
                                    } catch (_: Exception) {
                                        OutboxRepository.enqueue(
                                            SyncCommandTypes.REPAIR_ADD_NOTE,
                                            org.json.JSONObject()
                                                .put("repair_job_id", jobId)
                                                .put("note", newNote.trim())
                                                .put("note_id", java.util.UUID.randomUUID().toString()),
                                        )
                                    }
                                }
                                newNote = ""
                                message = "Note saved (synced or queued offline)"
                                refresh()
                            } catch (e: Exception) {
                                error = e.message
                            } finally {
                                busy = false
                            }
                        }
                    },
                    enabled = !busy && newNote.isNotBlank(),
                ) {
                    Text("Add")
                }
            }
        } else if (notes.isEmpty()) {
            Text(
                "No notes on this job.",
                style = MaterialTheme.typography.bodySmall,
                color = Brand.TextMuted,
            )
        }
        notes.forEach { n ->
            BrandCard {
                    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Text(n.optString("note"))
                    Text(
                        listOf(n.optString("author_name"), timeAgo(n.optString("created_at")))
                            .filter { it.isNotBlank() && it != "null" }.joinToString(" · "),
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
        }

        BrandSectionTitle("Parts")
        if (canRequestParts) {
        OutlinedTextField(
            value = partDesc,
            onValueChange = { if (!busy) partDesc = it },
            label = { Text("Part description") },
            enabled = !busy,
            modifier = Modifier.fillMaxWidth(),
        )
        if (suppliers.isNotEmpty()) {
            Text("Supplier", style = MaterialTheme.typography.labelLarge)
            suppliers.forEach { s ->
                val sid = s.optString("id")
                FilterChip(
                    selected = partSupplierId == sid,
                    onClick = { if (!busy) partSupplierId = sid },
                    enabled = !busy,
                    label = {
                        Text(
                            buildString {
                                append(s.optString("name"))
                                val phone = s.optString("phone")
                                if (phone.isNotBlank() && phone != "null") append(" · $phone")
                            },
                        )
                    },
                )
            }
        } else {
            Text(
                "Add a supplier in web-ops (with phone) to assign part requests.",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        val canRequestPart = canRequestParts && !busy && partDesc.isNotBlank() && partSupplierId.isNotBlank()
        Button(
            onClick = {
                if (busy) return@Button
                if (partDesc.isBlank()) {
                    error = "Describe the part"
                    context.showAppToast("Describe the part")
                    return@Button
                }
                if (partSupplierId.isBlank()) {
                    error = "Select a supplier"
                    context.showAppToast("Select a supplier")
                    return@Button
                }
                busy = true
                error = null
                message = null
                val descSnapshot = partDesc.trim()
                val supplierSnapshot = partSupplierId
                val branchSnapshot = j.optString("branch_id").ifBlank { null }
                scope.launch {
                    try {
                        val offline = withContext(Dispatchers.IO) {
                            try {
                                ApiClient.createPartRequest(
                                    repairJobId = jobId,
                                    branchId = branchSnapshot,
                                    description = descSnapshot,
                                    supplierId = supplierSnapshot,
                                )
                                false
                            } catch (e: Exception) {
                                if (!e.isLikelyNetworkFailure()) throw e
                                OutboxRepository.enqueue(
                                    SyncCommandTypes.PARTS_REQUEST,
                                    JSONObject()
                                        .put("repair_job_id", jobId)
                                        .put("branch_id", branchSnapshot ?: "")
                                        .put("description", descSnapshot)
                                        .put("quantity", 1)
                                        .put("supplier_id", supplierSnapshot)
                                        .put("part_request_id", UUID.randomUUID().toString()),
                                )
                                true
                            }
                        }
                        partDesc = ""
                        val okMsg = if (offline) {
                            "Part request saved offline — will sync when online"
                        } else {
                            "Part requested — job is waiting on parts"
                        }
                        message = okMsg
                        context.showAppToast(okMsg, long = true)
                        refresh()
                    } catch (e: Exception) {
                        val errMsg = e.message?.takeIf { it.isNotBlank() } ?: "Could not request part"
                        error = errMsg
                        context.showAppToast(errMsg, long = true)
                    } finally {
                        busy = false
                    }
                }
            },
            enabled = canRequestPart,
            modifier = Modifier.fillMaxWidth(),
        ) {
            Text(if (busy) "Requesting…" else "Request part")
        }
        } else {
            Text(
                "Parts can only be requested while the job is on the bench (not ready/completed).",
                style = MaterialTheme.typography.bodySmall,
                color = Brand.TextMuted,
            )
        }
        parts.forEach { p ->
            val issue = p.optJSONObject("issue")
            val status = issue?.optString("status")?.takeIf { it.isNotBlank() } ?: p.optString("status")
            BrandCard {
                    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    Text(p.optString("description"), style = MaterialTheme.typography.titleSmall)
                    Text(status)
                    if (p.optString("status") == "pending") {
                        if (isTechnician) {
                            Text(
                                "Waiting for supplier — once they set a price the pickup code appears here automatically.",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                        } else {
                            OutlinedTextField(
                                value = unitCost,
                                onValueChange = { unitCost = it },
                                label = { Text("Unit cost (KES)") },
                                modifier = Modifier.fillMaxWidth(),
                            )
                            Button(
                                onClick = {
                                    busy = true
                                    error = null
                                    message = null
                                    scope.launch {
                                        try {
                                            val issueRes = withContext(Dispatchers.IO) {
                                                ApiClient.approvePartRequest(p.getString("id"), unitCost.toDoubleOrNull() ?: 0.0)
                                            }
                                            message = "Auth ${issueRes.optString("auth_code")}"
                                            refresh()
                                        } catch (e: Exception) {
                                            error = e.message
                                        } finally {
                                            busy = false
                                        }
                                    }
                                },
                                enabled = !busy,
                                modifier = Modifier.fillMaxWidth(),
                            ) {
                                Text("Approve & issue auth code")
                            }
                        }
                    }
                    if (issue != null) {
                        val auth = issue.optString("auth_code")
                        if (auth.isNotBlank()) {
                            Text("Auth: $auth", style = MaterialTheme.typography.labelLarge)
                        }
                        if (issue.optString("status") == "approved") {
                            val issueId = issue.getString("id")
                            OutlinedTextField(
                                value = collectCodes[issueId] ?: auth,
                                onValueChange = { v -> collectCodes = collectCodes + (issueId to v) },
                                label = { Text("Confirm auth code") },
                                modifier = Modifier.fillMaxWidth(),
                            )
                            GoldButton(
                                text = "Mark collected",
                                onClick = {
                                    busy = true
                                    error = null
                                    message = null
                                    scope.launch {
                                        try {
                                            withContext(Dispatchers.IO) {
                                                ApiClient.collectSupplierIssue(
                                                    issueId,
                                                    collectCodes[issueId] ?: auth,
                                                )
                                            }
                                            message = "Part collected"
                                            refresh()
                                        } catch (e: Exception) {
                                            error = e.message
                                        } finally {
                                            busy = false
                                        }
                                    }
                                },
                                enabled = !busy,
                                loading = busy,
                                modifier = Modifier.fillMaxWidth(),
                            )
                        }
                        if (issue.optString("status") == "collected") {
                            Text(
                                "Collected · credit ${issue.optString("reconciliation_status")}",
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                        }
                    }
                }
            }
        }

        BrandSectionTitle("Customer estimates")
        if (canRequestParts) {
            OutlinedButton(
                onClick = { showEstimateDialog = true },
                enabled = !busy,
                modifier = Modifier.fillMaxWidth(),
            ) {
                Text("Send customer estimate")
            }
        }
        if (estimates.isEmpty()) {
            Text(
                "No estimates yet",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        estimates.forEach { estimate ->
            BrandCard {
                    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                    ) {
                        val total = if (estimate.has("total_amount")) {
                            estimate.optDouble("total_amount")
                        } else {
                            estimate.optDouble("labor_amount") + estimate.optDouble("parts_amount")
                        }
                        Text(
                            "KES ${total.toInt()}",
                            style = MaterialTheme.typography.titleSmall,
                        )
                        StatusChip(estimate.optString("status"))
                    }
                    estimate.optString("notes").takeIf { it.isNotBlank() && it != "null" }?.let { Text(it) }
                    estimate.optString("expires_at")
                        .takeIf { estimate.optString("status") == "pending" && it.isNotBlank() && it != "null" }
                        ?.let {
                            Text(
                                "Expires ${it.take(10)}",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                        }
                }
            }
        }

        }
        FeedbackBanner(message = null, error = error)
        message?.let { Text(it, color = Brand.Navy) }
        val timeline = j.optJSONArray("timeline")
        if (timeline != null && timeline.length() > 0) {
            BrandSectionTitle("History")
            Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                for (i in timeline.length() - 1 downTo 0) {
                    val ev = timeline.getJSONObject(i)
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                    ) {
                        Text(statusLabel(ev.optString("status")), style = MaterialTheme.typography.bodySmall)
                        Text(
                            timeAgo(ev.optString("at")),
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                    ev.optString("note").takeIf { it.isNotBlank() && it != "null" }?.let {
                        Text(
                            it,
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                }
            }
        }
        }
        }
    }
    PleaseWaitOverlay(
        visible = stkPolling,
        message = waitMessage,
        detail = waitDetail,
    )
    }
}

@Composable
fun CashScreen(selectedBranchId: String? = null, modifier: Modifier = Modifier) {
    var userId by remember { mutableStateOf("") }
    var pendingCash by remember { mutableStateOf(0.0) }
    var handovers by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var refunds by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var amount by rememberSaveable { mutableStateOf("") }
    var countById by remember { mutableStateOf<Map<String, String>>(emptyMap()) }
    var error by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    var message by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()

    fun refresh() {
        scope.launch {
            try {
                val me = withContext(Dispatchers.IO) { ApiClient.me() }
                userId = me.optString("id")
                pendingCash = withContext(Dispatchers.IO) { ApiClient.pendingCashTotal() }
                val items = withContext(Dispatchers.IO) { ApiClient.listCashHandovers() }
                val list = (0 until items.length()).map { items.getJSONObject(it) }
                handovers = list
                countById = list.associate { h ->
                    val id = h.getString("id")
                    id to (countById[id] ?: h.optDouble("amount", 0.0).toInt().toString())
                }
                val refItems = withContext(Dispatchers.IO) {
                    runCatching { ApiClient.listRefunds() }.getOrElse { org.json.JSONArray() }
                }
                refunds = (0 until refItems.length()).map { refItems.getJSONObject(it) }
                if (amount.isBlank() && pendingCash > 0) {
                    amount = pendingCash.toInt().toString()
                }
                error = null
            } catch (e: Exception) {
                error = e.message
            }
        }
    }

    LaunchedEffect(selectedBranchId) { refresh() }

    Column(
        modifier = modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background),
    ) {
        BrandHero(
            title = "Till",
            subtitle = "Track cash received, transfer custody, and close the shift.",
            appLabel = "Ops",
            bottomContent = {
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    HeroStat("Expected", "KES ${pendingCash.toInt()}", Modifier.weight(1f))
                }
            },
        )
        OpsShellChrome()
        Column(
            modifier = Modifier
                .weight(1f)
                .verticalScroll(rememberScrollState())
                .padding(16.dp)
                .padding(bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            OutlinedTextField(
                value = amount,
                onValueChange = { amount = it },
                label = { Text("Amount to transfer (KES)") },
                modifier = Modifier.fillMaxWidth(),
            )
            Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                listOf(500, 1000, 2000, 5000).forEach { value ->
                    FilterChip(
                        selected = amount.toIntOrNull() == value,
                        onClick = { amount = value.toString() },
                        label = { Text("KES " + value) },
                    )
                }
            }
            GoldButton(
                text = if (busy) "Working…" else "Transfer cash",
                onClick = {
                    val value = amount.toDoubleOrNull()
                    if (value == null || value <= 0) {
                        error = "Enter a positive amount"
                        return@GoldButton
                    }
                    busy = true
                    message = null
                    error = null
                    scope.launch {
                        try {
                            withContext(Dispatchers.IO) {
                                ApiClient.requestCashHandover(value, selectedBranchId)
                            }
                            message = "Cash transfer requested"
                            amount = ""
                            refresh()
                        } catch (e: Exception) {
                            error = e.message
                        } finally {
                            busy = false
                        }
                    }
                },
                enabled = !busy,
                loading = busy,
                modifier = Modifier.fillMaxWidth(),
            )
            FeedbackBanner(message = null, error = error)
            message?.let { Text(it, color = Brand.Navy) }
        }

        BrandSectionTitle("Cash transfers", modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp))
        LazyColumn(
            contentPadding = PaddingValues(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
            modifier = Modifier.weight(1f),
        ) {
            items(handovers, key = { it.getString("id") }) { h ->
                val id = h.getString("id")
                val declared = h.optDouble("amount", 0.0)
                val status = h.optString("status")
                val fromMe = h.optString("from_user_id") == userId
                val shortage = h.optDouble("shortage_amount", 0.0)
                BrandCard {
                    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        Text("KES ${declared.toInt()}", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold)
                        PillBadge(status.replaceFirstChar { it.uppercase() }, if (status == "confirmed") Brand.Success else Brand.Warning)
                        if (status == "confirmed" && shortage > 0) {
                            Text(
                                "Shortage KES ${shortage.toInt()}",
                                color = Brand.Danger,
                            )
                        }
                        if (status == "requested" && fromMe) {
                            Text(
                                "Waiting for the receiving staff member to confirm the count.",
                                color = Brand.TextSecondary,
                            )
                        }
                        if (status == "requested" && !fromMe) {
                            OutlinedTextField(
                                value = countById[id].orEmpty(),
                                onValueChange = { v -> countById = countById + (id to v) },
                                label = { Text("Counted cash (KES)") },
                                modifier = Modifier.fillMaxWidth(),
                            )
                            val counted = countById[id]?.toDoubleOrNull() ?: declared
                            val shortPreview = (declared - counted).coerceAtLeast(0.0)
                            if (shortPreview > 0) {
                                Text("Will record shortage KES ${shortPreview.toInt()}")
                            }
                            GoldButton(
                                text = "Confirm received",
                                onClick = {
                                    busy = true
                                    error = null
                                    scope.launch {
                                        try {
                                            withContext(Dispatchers.IO) {
                                                ApiClient.confirmCashHandover(id, countById[id]?.toDoubleOrNull())
                                            }
                                            message = "Handover confirmed"
                                            refresh()
                                        } catch (e: Exception) {
                                            error = e.message
                                        } finally {
                                            busy = false
                                        }
                                    }
                                },
                                enabled = !busy,
                                loading = busy,
                                modifier = Modifier.fillMaxWidth(),
                            )
                        }
                    }
                }
            }

            item {
                BrandSectionTitle("Refunds")
                Text(
                    "Approve requests from another staff member. You cannot approve your own.",
                    color = Brand.TextSecondary,
                )
            }
            if (refunds.isEmpty()) {
                item {
                    Text(
                        "No refund requests",
                        color = Brand.TextMuted,
                    )
                }
            }
            items(refunds, key = { it.getString("id") }) { r ->
                val rid = r.getString("id")
                val status = r.optString("status")
                val mine = r.optString("created_by") == userId
                BrandCard {
                    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        Text(
                            "KES ${r.optDouble("amount", 0.0).toInt()}",
                            style = MaterialTheme.typography.titleMedium,
                            fontWeight = FontWeight.Bold,
                        )
                        PillBadge(status.replaceFirstChar { it.uppercase() }, Brand.Info)
                        r.optString("reason").takeIf { it.isNotBlank() }?.let { Text(it) }
                        Text(
                            "Payment ${r.optString("payment_id").take(8)}…",
                            color = Brand.TextMuted,
                        )
                        if (status == "pending" && mine) {
                            Text(
                                "Waiting for another approver.",
                                color = Brand.TextSecondary,
                            )
                        }
                        if (status == "pending" && !mine) {
                            GoldButton(
                                text = "Approve refund",
                                onClick = {
                                    busy = true
                                    error = null
                                    message = null
                                    scope.launch {
                                        try {
                                            withContext(Dispatchers.IO) { ApiClient.approveRefund(rid) }
                                            message = "Refund approved"
                                            refresh()
                                        } catch (e: Exception) {
                                            error = e.message
                                        } finally {
                                            busy = false
                                        }
                                    }
                                },
                                enabled = !busy,
                                loading = busy,
                                modifier = Modifier.fillMaxWidth(),
                            )
                        }
                    }
                }
            }
        }
    }
}

private fun syncCommandLabel(command: String): String = when (command) {
    "repair.create_draft" -> "New repair intake"
    "repair.add_note" -> "Repair note"
    "repair.add_attachment" -> "Repair photo"
    "parts.request" -> "Parts request"
    "payments.cash_provisional" -> "Cash payment"
    else -> command.replace(95.toChar(), 32.toChar()).replace(46.toChar(), 32.toChar()).replaceFirstChar { it.uppercase() }
}



@Composable
fun SyncCenterScreen(modifier: Modifier = Modifier) {
    var items by remember { mutableStateOf<List<SyncCommandEntity>>(emptyList()) }
    var pending by remember { mutableStateOf(0) }
    var error by remember { mutableStateOf<String?>(null) }
    var message by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    var discardId by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()
    val dao = TechLaneApp.instance.database.syncOutboxDao()

    fun refresh() {
        scope.launch {
            items = dao.recent()
            pending = dao.pendingCount()
        }
    }

    LaunchedEffect(Unit) { refresh() }

    Column(
        modifier = modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background),
    ) {
        BrandHero(
            title = "Outbox",
            subtitle = "$pending command(s) waiting to sync",
            appLabel = "Ops",
        )
        OpsShellChrome()
        Column(
            modifier = Modifier
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Text(
                "Drafts sync over WorkManager. Tap Sync now to flush immediately.",
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                style = MaterialTheme.typography.bodyMedium,
            )
            GoldButton(
                text = if (busy) "Syncing…" else "Sync now",
                onClick = {
                    busy = true
                    error = null
                    message = null
                    scope.launch {
                        try {
                            val n = withContext(Dispatchers.IO) { OutboxFlush.flushPending() }
                            message = "Synced $n command(s)"
                            refresh()
                        } catch (e: Exception) {
                            error = e.message
                            refresh()
                        } finally {
                            busy = false
                        }
                    }
                },
                enabled = !busy,
                loading = busy,
                modifier = Modifier.fillMaxWidth(),
            )
            OutlinedButton(
                onClick = {
                    scope.launch {
                        dao.clearSettled()
                        refresh()
                        message = "Cleared synced / discarded"
                    }
                },
                modifier = Modifier.fillMaxWidth(),
            ) {
                Text("Clear settled")
            }
            FeedbackBanner(message = null, error = error)
            message?.let { Text(it, color = Brand.Navy) }
        }
        LazyColumn(
            contentPadding = PaddingValues(16.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
            modifier = Modifier.weight(1f),
        ) {
            items(items, key = { it.actionId }) { cmd ->
                BrandCard {
                    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                        Text(syncCommandLabel(cmd.commandType), style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.SemiBold)
                        PillBadge(cmd.syncStatus.replaceFirstChar { it.uppercase() }, if (cmd.syncStatus == "failed" || cmd.syncStatus == "conflict") Brand.Danger else if (cmd.syncStatus == "applied") Brand.Success else Brand.Warning)
                        Text(cmd.actionId.take(8) + "… · retries ${cmd.retryCount}")
                        cmd.lastError?.let {
                            Text(it, color = Brand.Danger)
                        }
                        val actionable = cmd.syncStatus in setOf("pending", "failed", "conflict")
                        if (actionable) {
                            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                Button(
                                    onClick = {
                                        scope.launch {
                                            dao.requeue(cmd.actionId)
                                            refresh()
                                        }
                                    },
                                ) { Text("Retry") }
                                Button(
                                    onClick = { discardId = cmd.actionId },
                                ) { Text("Discard") }
                            }
                        }
                    }
                }
            }
        }
    }
    discardId?.let { id ->
        AlertDialog(
            onDismissRequest = { discardId = null },
            title = { Text("Discard offline change?") },
            text = { Text("This change has not reached the server. Discarding it cannot be undone.") },
            dismissButton = { TextButton(onClick = { discardId = null }) { Text("Keep") } },
            confirmButton = { TextButton(onClick = { scope.launch { dao.delete(id); discardId = null; refresh() } }) { Text("Discard") } },
        )
    }
}

@Composable
fun PickupScreen(modifier: Modifier = Modifier) {
    var code by remember { mutableStateOf("") }
    var showScanner by remember { mutableStateOf(false) }
    var orders by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var error by remember { mutableStateOf<String?>(null) }
    var message by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()

    fun refresh() {
        scope.launch {
            try {
                val items = withContext(Dispatchers.IO) { ApiClient.listOnlineOrders("ready_for_pickup") }
                orders = (0 until items.length()).map { items.getJSONObject(it) }
                error = null
            } catch (e: Exception) {
                error = e.message
            }
        }
    }

    LaunchedEffect(Unit) { refresh() }

    Column(
        modifier = modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background),
    ) {
        BrandHero(
            title = "Pickup",
            subtitle = "Enter the customer collection code after online payment.",
            appLabel = "Ops",
        )
        OpsShellChrome()
        Column(
            modifier = Modifier
                .weight(1f)
                .verticalScroll(rememberScrollState())
                .padding(16.dp)
                .padding(bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            OutlinedTextField(
                value = code,
                onValueChange = { code = it.uppercase() },
                label = { Text("Collection code") },
                modifier = Modifier.fillMaxWidth(),
            )
            OutlinedButton(
                onClick = { showScanner = !showScanner },
                modifier = Modifier.fillMaxWidth(),
            ) { Text(if (showScanner) "Close scanner" else "Scan collection code") }
            if (showScanner) {
                ScanCameraPanel(
                    enabled = !busy,
                    onCode = { scanned ->
                        code = scanned.uppercase()
                        showScanner = false
                    },
                )
            }
            GoldButton(
                text = if (busy) "Working…" else "Mark collected",
                onClick = {
                    if (code.trim().length < 4) {
                        error = "Enter a valid code"
                        return@GoldButton
                    }
                    busy = true
                    error = null
                    message = null
                    scope.launch {
                        try {
                            val order = withContext(Dispatchers.IO) { ApiClient.collectOnlineOrder(code) }
                            message = "Collected · ${order.optString("status")} · ${order.optString("id").take(8)}…"
                            code = ""
                            refresh()
                        } catch (e: Exception) {
                            error = e.message
                        } finally {
                            busy = false
                        }
                    }
                },
                enabled = !busy,
                loading = busy,
                modifier = Modifier.fillMaxWidth(),
            )
            FeedbackBanner(message = null, error = error)
            message?.let { Text(it, color = Brand.Navy) }
        }

        BrandSectionTitle(
            "Ready for pickup (${orders.size})",
            modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
        )
        LazyColumn(
            contentPadding = PaddingValues(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
            modifier = Modifier.weight(1f),
        ) {
            items(orders, key = { it.getString("id") }) { o ->
                BrandCard {
                    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                        Text(
                            "KES ${o.optDouble("total", 0.0).toInt()}",
                            style = MaterialTheme.typography.titleMedium,
                            fontWeight = FontWeight.Bold,
                        )
                        o.optString("customer_name").takeIf { it.isNotBlank() }?.let { Text(it, fontWeight = FontWeight.SemiBold) }
                        o.optString("customer_phone").takeIf { it.isNotBlank() }?.let { Text(it, color = Brand.TextSecondary) }
                        Text(o.optString("status"), style = MaterialTheme.typography.labelLarge, color = Brand.TextSecondary)
                        val c = o.optString("collection_code")
                        if (c.isNotBlank()) {
                            Text("Code $c", style = MaterialTheme.typography.titleSmall)
                        }
                        Text(
                            o.optString("id").take(8) + "…",
                            color = Brand.TextMuted,
                        )
                        OutlinedButton(
                            onClick = {
                                code = o.optString("collection_code")
                            },
                            enabled = c.isNotBlank(),
                        ) {
                            Text("Use code")
                        }
                    }
                }
            }
        }
    }
}

fun closureReasonLabel(code: String): String = when (code) {
    "customer_declined_quote" -> "Customer declined the quote"
    "customer_withdrew" -> "Customer took the device back"
    "no_response" -> "Customer never responded"
    "duplicate_job" -> "Duplicate job card"
    "beyond_economical_repair" -> "Beyond economical repair"
    "parts_unavailable" -> "Parts not available"
    "severe_liquid_damage" -> "Severe liquid damage"
    "further_damage_found" -> "Further damage found on opening"
    "other" -> "Other (see note)"
    else -> code.replace('_', ' ')
}

private fun closureStatusLabel(status: String): String = when (status) {
    "cancelled" -> "Cancelled — job will not go ahead"
    "unrepairable" -> "Unrepairable — cannot be fixed"
    else -> status.replace('_', ' ')
}

/**
 * Closing a job writes off pipeline value, so the reason is mandatory and the
 * server rejects the call without one. The list of codes comes from the API so
 * the app never drifts from the backend's catalogue.
 */
/**
 * Releasing a device is the one step with no undo, so it asks who is taking it and
 * how we know they are entitled to it before letting go.
 */
@OptIn(ExperimentalFoundationApi::class)
@Composable
fun HandoverSection(
    jobId: String,
    defaultName: String,
    balanceDue: Double,
    canVouch: Boolean,
    busy: Boolean,
    setBusy: (Boolean) -> Unit,
    onError: (String?) -> Unit,
    onMessage: (String?) -> Unit,
    refresh: () -> Unit,
    scope: CoroutineScope,
    startRelease: Boolean = false,
    bringIntoViewRequester: BringIntoViewRequester = remember { BringIntoViewRequester() },
) {
    var name by remember(defaultName) { mutableStateOf(defaultName) }
    var relationship by remember { mutableStateOf("self") }
    var idNumber by remember { mutableStateOf("") }
    var code by remember { mutableStateOf("") }
    var note by remember { mutableStateOf("") }
    var codeSent by remember { mutableStateOf(false) }
    var showScanner by remember { mutableStateOf(false) }
    val codeFocus = remember { FocusRequester() }

    val blocked = balanceDue > 0.009
    val vouching = code.isBlank()

    LaunchedEffect(startRelease, blocked) {
        if (!startRelease || blocked) return@LaunchedEffect
        bringIntoViewRequester.bringIntoView()
        try {
            withContext(Dispatchers.IO) { ApiClient.sendHandoverCode(jobId) }
            codeSent = true
            onMessage("Payment received — code sent. Enter it or scan the intake QR.")
        } catch (_: Exception) {
            onMessage("Payment received — enter the SMS code or scan the intake QR.")
        }
        codeFocus.requestFocus()
    }

    BrandSectionTitle("Hand the device over")
    if (blocked) {
        Text(
            "KES ${balanceDue.toInt()} is still owed. Take the payment first — handing the device over now turns " +
                "the repair into an unsecured loan.",
            style = MaterialTheme.typography.bodySmall,
            color = Brand.Danger,
        )
        return
    }

    Column(
        modifier = Modifier.bringIntoViewRequester(bringIntoViewRequester),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
    if (startRelease) {
        Text(
            "Cash is on the counter. Enter the SMS code from the owner's phone, or scan the intake slip QR.",
            style = MaterialTheme.typography.bodyMedium,
            color = Brand.Navy,
            fontWeight = FontWeight.SemiBold,
        )
    }

    OutlinedTextField(
        value = name,
        onValueChange = { name = it },
        label = { Text("Who is collecting it?") },
        modifier = Modifier.fillMaxWidth(),
        singleLine = true,
    )
    SectionLabel("Relationship to the owner")
    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        FilterChip(
            selected = relationship == "self",
            onClick = { relationship = "self" },
            label = { Text("The owner") },
            modifier = Modifier.weight(1f),
        )
        FilterChip(
            selected = relationship != "self",
            onClick = { if (relationship == "self") relationship = "other" },
            label = { Text("Someone else") },
            modifier = Modifier.weight(1f),
        )
    }
    if (relationship != "self") {
        OutlinedTextField(
            value = idNumber,
            onValueChange = { idNumber = it },
            label = { Text("ID number shown") },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true,
        )
    }
    OutlinedTextField(
        value = code,
        onValueChange = { code = it.trim().uppercase() },
        label = { Text("SMS code or intake slip code (PK-…)") },
        supportingText = { Text("SMS digits from the owner, or the printed PK- code / QR from intake") },
        modifier = Modifier
            .fillMaxWidth()
            .focusRequester(codeFocus),
        singleLine = true,
    )
    Row(horizontalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.fillMaxWidth()) {
        TextButton(
            onClick = {
                setBusy(true)
                onError(null)
                scope.launch {
                    try {
                        withContext(Dispatchers.IO) { ApiClient.sendHandoverCode(jobId) }
                        codeSent = true
                        onMessage("Code sent to the owner's phone")
                    } catch (e: Exception) {
                        onError(e.message)
                    } finally {
                        setBusy(false)
                    }
                }
            },
            enabled = !busy,
            modifier = Modifier.weight(1f),
        ) {
            Text(if (codeSent) "Resend code" else "Text a code")
        }
        OutlinedButton(
            onClick = { showScanner = !showScanner },
            enabled = !busy,
            modifier = Modifier.weight(1f),
        ) {
            Icon(Icons.Filled.QrCodeScanner, contentDescription = null)
            Text(
                if (showScanner) "Hide scanner" else "Scan QR",
                modifier = Modifier.padding(start = 6.dp),
            )
        }
    }
    if (showScanner) {
        ScanCameraPanel(
            onCode = { raw ->
                val parsed = parseScanPayload(raw)
                val next = parsed.code.ifBlank { raw }.trim()
                if (next.isNotBlank()) {
                    code = next.uppercase()
                    showScanner = false
                    onMessage("QR captured — review and release")
                }
            },
            modifier = Modifier
                .fillMaxWidth()
                .height(220.dp),
            enabled = true,
        )
    }
    if (vouching) {
        if (canVouch) {
            Text(
                "Without a code you are personally vouching that this is the right person. That is recorded " +
                    "against your name.",
                style = MaterialTheme.typography.bodySmall,
                color = Brand.Danger,
            )
            OutlinedTextField(
                value = note,
                onValueChange = { note = it },
                label = { Text("Why release it without a code?") },
                modifier = Modifier.fillMaxWidth(),
            )
        } else {
            Text(
                "Enter the SMS code or scan the intake QR. Only a manager or owner can release without one.",
                style = MaterialTheme.typography.bodySmall,
                color = Brand.TextSecondary,
            )
        }
    }
    Button(
        onClick = {
            setBusy(true)
            onError(null)
            onMessage(null)
            scope.launch {
                try {
                    withContext(Dispatchers.IO) {
                        val entered = code.trim()
                        val pickupMatch = Regex("""PK-[A-Z0-9]+""", RegexOption.IGNORE_CASE).find(entered)
                        val isPickup = pickupMatch != null ||
                            entered.contains("REPAIR-PICKUP", ignoreCase = true)
                        val pickupCode = pickupMatch?.value?.uppercase()
                            ?: entered.takeIf { it.startsWith("PK-", ignoreCase = true) }?.uppercase()
                        ApiClient.recordHandover(
                            jobId,
                            name.trim(),
                            relationship,
                            idNumber.trim().ifBlank { null },
                            note.trim().ifBlank { null },
                            otpCode = if (isPickup) null else entered.ifBlank { null },
                            pickupCode = if (isPickup) pickupCode else null,
                        )
                    }
                    onMessage("Device released to ${name.trim()}")
                    refresh()
                } catch (e: Exception) {
                    onError(e.message)
                } finally {
                    setBusy(false)
                }
            }
        },
        enabled = !busy && name.isNotBlank() && (!vouching || canVouch),
        modifier = Modifier.fillMaxWidth(),
    ) {
        Text("Release device")
    }
    }
}

@Composable
fun CloseJobDialog(
    options: List<String>,
    reasonsByStatus: JSONObject?,
    busy: Boolean,
    onDismiss: () -> Unit,
    onConfirm: (status: String, reason: String, note: String) -> Unit,
) {
    var status by remember { mutableStateOf(options.firstOrNull() ?: "cancelled") }
    var reason by remember { mutableStateOf("") }
    var note by remember { mutableStateOf("") }

    val reasons = remember(status, reasonsByStatus) {
        reasonsByStatus?.optJSONArray(status)?.let { arr ->
            (0 until arr.length()).map { arr.getString(it) }
        } ?: emptyList()
    }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("Close without repair") },
        text = {
            Column(
                verticalArrangement = Arrangement.spacedBy(8.dp),
                modifier = Modifier.verticalScroll(rememberScrollState()),
            ) {
                Text(
                    "No commission, warranty or loyalty points are issued. Any price quoted at intake is cleared so " +
                        "the device can be handed back.",
                    style = MaterialTheme.typography.bodySmall,
                    color = Brand.TextSecondary,
                )
                for (option in options) {
                    FilterChip(
                        selected = status == option,
                        onClick = {
                            status = option
                            reason = ""
                        },
                        label = { Text(closureStatusLabel(option)) },
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
                SectionLabel("Reason")
                for (code in reasons) {
                    FilterChip(
                        selected = reason == code,
                        onClick = { reason = code },
                        label = { Text(closureReasonLabel(code)) },
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
                OutlinedTextField(
                    value = note,
                    onValueChange = { note = it },
                    label = { Text("Note (optional)") },
                    modifier = Modifier.fillMaxWidth(),
                )
            }
        },
        confirmButton = {
            Button(
                onClick = { onConfirm(status, reason, note.trim()) },
                enabled = !busy && reason.isNotBlank(),
                colors = ButtonDefaults.buttonColors(containerColor = Brand.Danger),
            ) {
                Text("Close job")
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss, enabled = !busy) { Text("Cancel") }
        },
    )
}
