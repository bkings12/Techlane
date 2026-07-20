package com.techlane.ops.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import android.provider.OpenableColumns
import android.util.Base64
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.Logout
import androidx.compose.material.icons.filled.AccountBalance
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Build
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.Inventory2
import androidx.compose.material.icons.filled.MoreHoriz
import androidx.compose.material.icons.filled.Payments
import androidx.compose.material.icons.filled.Place
import androidx.compose.material.icons.filled.PointOfSale
import androidx.compose.material.icons.filled.QrCodeScanner
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Sync
import androidx.compose.material.icons.outlined.Handyman
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.FilterChipDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import com.techlane.ops.TechLaneApp
import com.techlane.ops.data.SyncCommandEntity
import com.techlane.ops.network.ApiClient
import com.techlane.ops.sync.OutboxFlush
import com.techlane.ops.sync.OutboxRepository
import com.techlane.ops.sync.SyncCommandTypes
import com.techlane.core.PrintSupport
import com.techlane.core.theme.statusPalette
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import java.time.Instant
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

@Composable
fun AppNav(
    signedIn: Boolean,
    onSignedIn: () -> Unit,
    onSignedOut: () -> Unit,
    modifier: Modifier = Modifier,
) {
    DisposableEffect(onSignedOut) {
        ApiClient.setSessionExpiredListener {
            android.os.Handler(android.os.Looper.getMainLooper()).post {
                onSignedOut()
            }
        }
        onDispose { ApiClient.setSessionExpiredListener(null) }
    }
    if (!signedIn) {
        LoginScreen(onSignedIn = onSignedIn, modifier = modifier)
    } else {
        MainTabs(onSignedOut = onSignedOut, modifier = modifier)
    }
}

@Composable
fun LoginScreen(onSignedIn: () -> Unit, modifier: Modifier = Modifier) {
    var email by remember { mutableStateOf("tech@techlane.local") }
    var password by remember { mutableStateOf("password") }
    var error by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()

    Box(
        modifier = modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background),
    ) {
        // Soft teal wash behind the brand block — atmosphere without a flat fill.
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .height(280.dp)
                .background(MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.55f)),
        )
        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 28.dp, vertical = 48.dp),
            verticalArrangement = Arrangement.Center,
        ) {
            Icon(
                Icons.Outlined.Handyman,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.primary,
                modifier = Modifier.size(40.dp),
            )
            Spacer(Modifier.height(16.dp))
            Text("TechLane", style = MaterialTheme.typography.displayLarge)
            Spacer(Modifier.height(8.dp))
            Text(
                "Shop floor ops — jobs, parts, and cash without leakage.",
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Spacer(Modifier.height(36.dp))
            Surface(
                shape = RoundedCornerShape(20.dp),
                color = MaterialTheme.colorScheme.surface,
                tonalElevation = 3.dp,
                shadowElevation = 4.dp,
            ) {
                Column(
                    modifier = Modifier.padding(22.dp),
                    verticalArrangement = Arrangement.spacedBy(14.dp),
                ) {
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
                        visualTransformation = PasswordVisualTransformation(),
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth(),
                    )
                    error?.let {
                        Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
                    }
                    Button(
                        onClick = {
                            busy = true
                            error = null
                            scope.launch {
                                try {
                                    val tokens = withContext(Dispatchers.IO) { ApiClient.login(email, password) }
                                    TechLaneApp.instance.tokenStore.accessToken = tokens.getString("access_token")
                                    TechLaneApp.instance.tokenStore.refreshToken = tokens.getString("refresh_token")
                                    withContext(Dispatchers.IO) {
                                        runCatching { ApiClient.registerDevice() }
                                    }
                                    onSignedIn()
                                } catch (e: Exception) {
                                    error = e.message
                                } finally {
                                    busy = false
                                }
                            }
                        },
                        enabled = !busy,
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(52.dp),
                    ) {
                        Text(if (busy) "Signing in…" else "Sign in")
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
            val effectiveIds = if ("technician" in serverRoles) allowedIds.take(1) else allowedIds
            val serverBranches = runCatching { withContext(Dispatchers.IO) { ApiClient.listBranches() } }.getOrNull()
            val names = if (serverBranches != null) {
                (0 until serverBranches.length()).map { serverBranches.getJSONObject(it) }
                    .associate { it.optString("id") to it.optString("name") }
            } else emptyMap()
            branches = effectiveIds.map { BranchOption(it, names[it] ?: "Branch ${it.take(8)}") }
            if (selectedBranchId !in effectiveIds) {
                selectedBranchId = effectiveIds.firstOrNull()
                TechLaneApp.instance.tokenStore.selectedBranchId = selectedBranchId
            }
            profileError = null
        } catch (e: Exception) {
            profileError = e.message
        }
    }

    val owner = "owner" in roles || "*" in permissions
    val isTechnician = "technician" in roles && !owner
    val canRepair = owner || "technician" in roles || permissions.any { it.startsWith("repairs.") || it.startsWith("parts.") }
    val canSell = owner || "cashier" in roles || "sales.create" in permissions
    val canCash = owner || "cashier" in roles || permissions.any { it.startsWith("cash.") || it.startsWith("payments.") || it.startsWith("refunds.") }
    val canInventory = owner || "inventory" in roles || "inventory.read" in permissions
    val tabs = buildList {
        add("home")
        if (canRepair) add("jobs")
        if (canSell) add("pos")
        if (canInventory || canSell) add("inventory")
        if (canCash) add("cash")
        if (canSell) add("pickup")
        if (canCash) add("c2b")
        if (canRepair || canSell) add("scan")
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
        "pos" -> "POS"
        "inventory" -> "Stock"
        "cash" -> "Cash"
        "pickup" -> "Pickup"
        "c2b" -> "C2B"
        "scan" -> "Scan"
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
        "sync" -> Icons.Filled.Sync
        else -> Icons.Filled.Home
    }
    Scaffold(
        modifier = modifier,
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = {
                    Column {
                        Text(label(tab), style = MaterialTheme.typography.titleLarge)
                        branches.firstOrNull { it.id == selectedBranchId }?.let { branch ->
                            Text(
                                branch.name,
                                style = MaterialTheme.typography.labelMedium,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                        }
                    }
                },
                actions = {
                    IconButton(onClick = onSignedOut) {
                        Icon(Icons.AutoMirrored.Filled.Logout, contentDescription = "Sign out")
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.surface,
                ),
            )
        },
        bottomBar = {
            NavigationBar(containerColor = MaterialTheme.colorScheme.surface) {
                primaryTabs.forEach { key ->
                    NavigationBarItem(
                        selected = tab == key,
                        onClick = { tab = key },
                        icon = { Icon(tabIcon(key), contentDescription = label(key)) },
                        label = { Text(label(key)) },
                    )
                }
                if (moreTabs.isNotEmpty()) {
                    NavigationBarItem(
                        selected = tab in moreTabs,
                        onClick = { showMore = true },
                        icon = { Icon(Icons.Filled.MoreHoriz, contentDescription = "More") },
                        label = { Text("More") },
                    )
                }
            }
        },
    ) { padding ->
        Column(modifier = Modifier.padding(padding).fillMaxSize()) {
            ConnectivityBanner()
            if (branches.size > 1) {
                BranchPicker(
                    branches = branches,
                    selectedBranchId = selectedBranchId,
                    onSelected = {
                        selectedBranchId = it
                        TechLaneApp.instance.tokenStore.selectedBranchId = it
                    },
                )
            }
            profileError?.let { Text(it, color = MaterialTheme.colorScheme.error, modifier = Modifier.padding(8.dp)) }
            Box(Modifier.weight(1f)) {
                when (tab) {
                    "home" -> HomeScreen(
                        isTechnician = isTechnician,
                        onOpenJobs = { tab = if (canRepair) "jobs" else "pos" },
                        onOpenJob = { id ->
                            // Jump to jobs tab; JobsScreen will open detail via shared state is awkward.
                            // Use jobs tab with a simple approach: store pending job id.
                            pendingJobId = id
                            tab = "jobs"
                        },
                    )
                    "jobs" -> JobsScreen(
                        isTechnician = isTechnician,
                        defaultMine = isTechnician,
                        openJobId = pendingJobId,
                        onOpenJobConsumed = { pendingJobId = null },
                    )
                    "pos" -> PosScreen(selectedBranchId)
                    "inventory" -> InventoryLookupScreen(selectedBranchId)
                    "cash" -> CashScreen(selectedBranchId)
                    "pickup" -> PickupScreen()
                    "c2b" -> C2BExceptionsScreen()
                    "scan" -> ManualScanScreen()
                    else -> SyncCenterScreen()
                }
            }
        }
        if (showMore && moreTabs.isNotEmpty()) {
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
                    moreTabs.forEach { key ->
                        TextButton(
                            onClick = {
                                tab = key
                                showMore = false
                            },
                            modifier = Modifier.fillMaxWidth(),
                        ) {
                            Icon(tabIcon(key), contentDescription = null)
                            Spacer(Modifier.width(12.dp))
                            Text(label(key), modifier = Modifier.weight(1f))
                        }
                    }
                    Spacer(Modifier.height(16.dp))
                }
            }
        }
    }
}

@Composable
fun HomeScreen(
    isTechnician: Boolean = false,
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
    var summary by remember { mutableStateOf<JSONObject?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
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
                val openStatuses = setOf("intake", "diagnosed", "waiting_parts", "in_progress")
                myOpenJobs = all.filter {
                    it.optString("technician_id") == myId && it.optString("status") in openStatuses
                }
                myJobs = myOpenJobs.size
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
        }
    }

    LaunchedEffect(isTechnician) { refresh() }
    val statuses = statusPalette()

    Column(
        modifier = modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        ScreenHeader(
            title = if (displayName.isBlank()) "Welcome" else "Hi, $displayName",
            subtitle = if (isTechnician) {
                "Your bench — claim, diagnose, and move jobs forward"
            } else {
                "Floor pulse — what’s open right now"
            },
            action = {
                IconButton(onClick = { refresh() }) { Icon(Icons.Filled.Refresh, "Refresh") }
            },
        )
        error?.let { Text(it, color = MaterialTheme.colorScheme.error) }

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
                    "In progress",
                    myOpenJobs.count { it.optString("status") == "in_progress" }.toString(),
                    tileModifier = Modifier.weight(1f),
                    accent = statuses.inProgress,
                )
            }

            SectionLabel("My bench")
            if (myOpenJobs.isEmpty()) {
                EmptyHint(
                    message = "Claim an unassigned intake from the jobs board to get started.",
                    title = "No jobs on your bench",
                    icon = Icons.Outlined.Handyman,
                )
            } else {
                myOpenJobs.take(8).forEach { job ->
                    Card(
                        onClick = { onOpenJob(job.getString("id")) },
                        modifier = Modifier.fillMaxWidth(),
                        shape = RoundedCornerShape(16.dp),
                        elevation = CardDefaults.cardElevation(defaultElevation = 2.dp),
                        border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline.copy(alpha = 0.2f)),
                    ) {
                        Column(
                            Modifier.padding(16.dp),
                            verticalArrangement = Arrangement.spacedBy(6.dp),
                        ) {
                            Row(
                                Modifier.fillMaxWidth(),
                                horizontalArrangement = Arrangement.SpaceBetween,
                                verticalAlignment = Alignment.CenterVertically,
                            ) {
                                Text(
                                    job.optString("job_code").ifBlank { job.getString("id").take(8) },
                                    style = MaterialTheme.typography.labelLarge,
                                    fontWeight = FontWeight.SemiBold,
                                    color = MaterialTheme.colorScheme.primary,
                                )
                                StatusChip(job.optString("status"))
                            }
                            Text(
                                job.optString("problem_summary").ifBlank { "Repair job" },
                                style = MaterialTheme.typography.titleMedium,
                                fontWeight = FontWeight.SemiBold,
                            )
                            Text(
                                job.optString("customer_name").ifBlank { "Walk-in" },
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                        }
                    }
                }
            }

            Button(
                onClick = onOpenJobs,
                modifier = Modifier
                    .fillMaxWidth()
                    .height(52.dp),
            ) {
                Text("Open full jobs board")
            }
        } else {
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp), modifier = Modifier.fillMaxWidth()) {
                MetricTile("My open", myJobs.toString(), tileModifier = Modifier.weight(1f), accent = statuses.inProgress)
                MetricTile("In progress", (counts["in_progress"] ?: 0).toString(), tileModifier = Modifier.weight(1f), accent = statuses.inProgress)
            }
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp), modifier = Modifier.fillMaxWidth()) {
                MetricTile("Waiting parts", (counts["waiting_parts"] ?: 0).toString(), tileModifier = Modifier.weight(1f), accent = statuses.waitingParts)
                MetricTile("Ready", (counts["completed"] ?: 0).toString(), tileModifier = Modifier.weight(1f), accent = statuses.completed)
            }
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp), modifier = Modifier.fillMaxWidth()) {
                MetricTile("New intake", (counts["intake"] ?: 0).toString(), tileModifier = Modifier.weight(1f), accent = statuses.intake)
                MetricTile("Diagnosed", (counts["diagnosed"] ?: 0).toString(), tileModifier = Modifier.weight(1f), accent = statuses.diagnosed)
            }

            summary?.let { s ->
                SectionLabel("Money today")
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp), modifier = Modifier.fillMaxWidth()) {
                    MetricTile("Paid", "KES ${s.optDouble("payments_allocated_period", 0.0).toInt()}", tileModifier = Modifier.weight(1f))
                    MetricTile("Cash floor", "KES ${s.optDouble("payments_cash_provisional", 0.0).toInt()}", tileModifier = Modifier.weight(1f))
                }
            }

            Button(
                onClick = onOpenJobs,
                modifier = Modifier
                    .fillMaxWidth()
                    .height(48.dp),
            ) {
                Text("Open jobs board")
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun JobsScreen(
    isTechnician: Boolean = false,
    defaultMine: Boolean = false,
    openJobId: String? = null,
    onOpenJobConsumed: () -> Unit = {},
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
        JobDetailScreen(
            jobId = selectedJobId!!,
            isTechnician = isTechnician,
            onBack = { selectedJobId = null },
            modifier = modifier,
        )
        return
    }
    if (showIntake) {
        IntakeScreen(
            onBack = { showIntake = false },
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

    LaunchedEffect(filter, search) { refresh() }

    Column(modifier = modifier.fillMaxSize()) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 8.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            if (isTechnician) {
                Text(
                    "Your queue — claim intake, diagnose, request parts, move status",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            OutlinedTextField(
                value = search,
                onValueChange = { search = it },
                label = { Text("Search job, customer, problem") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
                trailingIcon = {
                    IconButton(onClick = { refresh() }) { Icon(Icons.Filled.Refresh, "Refresh") }
                },
            )
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
                    add("in_progress" to "In progress")
                    add("completed" to "Ready")
                }
                filters.forEach { (key, label) ->
                    FilterChip(
                        selected = filter == key,
                        onClick = { filter = key },
                        label = { Text(label) },
                        colors = FilterChipDefaults.filterChipColors(
                            selectedContainerColor = MaterialTheme.colorScheme.primaryContainer,
                            selectedLabelColor = MaterialTheme.colorScheme.onPrimaryContainer,
                        ),
                    )
                }
            }
            Button(
                onClick = { showIntake = true },
                modifier = Modifier
                    .fillMaxWidth()
                    .height(44.dp),
            ) {
                Icon(Icons.Filled.Add, null, modifier = Modifier.size(18.dp))
                Spacer(Modifier.width(8.dp))
                Text("New intake")
            }
        }
        error?.let { Text(it, color = MaterialTheme.colorScheme.error, modifier = Modifier.padding(16.dp)) }
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
        LazyColumn(
            contentPadding = PaddingValues(horizontal = 16.dp, vertical = 8.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
            modifier = Modifier.weight(1f),
        ) {
            items(jobs, key = { it.getString("id") }) { job ->
                Card(
                    onClick = { selectedJobId = job.getString("id") },
                    modifier = Modifier.fillMaxWidth(),
                    shape = RoundedCornerShape(16.dp),
                    colors = CardDefaults.cardColors(
                        containerColor = MaterialTheme.colorScheme.surface,
                    ),
                    elevation = CardDefaults.cardElevation(defaultElevation = 2.dp, pressedElevation = 4.dp),
                    border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline.copy(alpha = 0.2f)),
                ) {
                    Row(modifier = Modifier.fillMaxWidth()) {
                        Box(
                            modifier = Modifier
                                .width(4.dp)
                                .height(96.dp)
                                .background(statusColor(job.optString("status"))),
                        )
                        Column(
                            modifier = Modifier.padding(16.dp),
                            verticalArrangement = Arrangement.spacedBy(6.dp),
                        ) {
                            Row(
                                modifier = Modifier.fillMaxWidth(),
                                horizontalArrangement = Arrangement.SpaceBetween,
                                verticalAlignment = Alignment.CenterVertically,
                            ) {
                                Text(
                                    job.optString("job_code").ifBlank { job.getString("id").take(8) },
                                    style = MaterialTheme.typography.labelLarge,
                                    fontWeight = FontWeight.SemiBold,
                                    color = MaterialTheme.colorScheme.primary,
                                )
                                StatusChip(job.optString("status"))
                            }
                            Text(
                                job.optString("problem_summary"),
                                style = MaterialTheme.typography.titleMedium,
                                fontWeight = FontWeight.SemiBold,
                            )
                            Row(
                                modifier = Modifier.fillMaxWidth(),
                                horizontalArrangement = Arrangement.SpaceBetween,
                            ) {
                                Text(
                                    job.optString("customer_name").ifBlank { "Walk-in" },
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                    style = MaterialTheme.typography.bodySmall,
                                )
                                Text(
                                    timeAgo(job.optString("created_at")),
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                    style = MaterialTheme.typography.bodySmall,
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
fun IntakeScreen(onBack: () -> Unit, onCreated: (String) -> Unit, modifier: Modifier = Modifier) {
    var customerName by remember { mutableStateOf("") }
    var customerPhone by remember { mutableStateOf("") }
    var kind by remember { mutableStateOf("phone") }
    var brand by remember { mutableStateOf("") }
    var model by remember { mutableStateOf("") }
    var imei by remember { mutableStateOf("") }
    var problem by remember { mutableStateOf("") }
    var assignToMe by remember { mutableStateOf(true) }
    var imeiPhoto by remember { mutableStateOf<Pair<ByteArray, String>?>(null) }
    var devicePhoto by remember { mutableStateOf<Pair<ByteArray, String>?>(null) }
    var photoTarget by remember { mutableStateOf("imei") }
    var takePictureUri by remember { mutableStateOf<android.net.Uri?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    val context = LocalContext.current

    fun createCameraUri(): android.net.Uri {
        val dir = java.io.File(context.cacheDir, "camera").apply { mkdirs() }
        val file = java.io.File(dir, "intake-${System.currentTimeMillis()}.jpg")
        return androidx.core.content.FileProvider.getUriForFile(
            context,
            "${context.packageName}.fileprovider",
            file,
        )
    }

    fun readUri(uri: android.net.Uri): Pair<ByteArray, String> {
        val bytes = context.contentResolver.openInputStream(uri)?.use { it.readBytes() }
            ?: error("Could not read photo")
        if (bytes.size > 5 * 1024 * 1024) error("Photo must be 5 MB or smaller")
        val name = context.contentResolver.query(uri, arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)?.use { cursor ->
            if (cursor.moveToFirst()) cursor.getString(0) else null
        } ?: "intake-photo.jpg"
        return bytes to name
    }

    val picker = rememberLauncherForActivityResult(ActivityResultContracts.GetContent()) { uri ->
        if (uri == null) return@rememberLauncherForActivityResult
        try {
            val photo = readUri(uri)
            if (photoTarget == "device") devicePhoto = photo else imeiPhoto = photo
            error = null
        } catch (e: Exception) {
            error = e.message
        }
    }
    var launchCameraAfterPermission by remember { mutableStateOf(false) }
    val takePicture = rememberLauncherForActivityResult(ActivityResultContracts.TakePicture()) { ok ->
        val uri = takePictureUri
        takePictureUri = null
        if (!ok || uri == null) return@rememberLauncherForActivityResult
        try {
            val photo = readUri(uri)
            if (photoTarget == "device") devicePhoto = photo else imeiPhoto = photo
            error = null
        } catch (e: Exception) {
            error = e.message
        }
    }
    val cameraPermission = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        if (!granted) {
            error = "Camera permission is required to take photos"
            launchCameraAfterPermission = false
            return@rememberLauncherForActivityResult
        }
        launchCameraAfterPermission = true
    }
    LaunchedEffect(launchCameraAfterPermission) {
        if (!launchCameraAfterPermission) return@LaunchedEffect
        launchCameraAfterPermission = false
        val uri = createCameraUri()
        takePictureUri = uri
        takePicture.launch(uri)
    }

    fun startCamera(target: String) {
        photoTarget = target
        when {
            androidx.core.content.ContextCompat.checkSelfPermission(
                context,
                android.Manifest.permission.CAMERA,
            ) == android.content.pm.PackageManager.PERMISSION_GRANTED -> {
                launchCameraAfterPermission = true
            }
            else -> cameraPermission.launch(android.Manifest.permission.CAMERA)
        }
    }

    Column(
        modifier = modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        TextButton(onClick = onBack) { Text("← Jobs") }
        Text(
            "New repair intake",
            style = MaterialTheme.typography.headlineSmall,
            fontWeight = FontWeight.SemiBold,
        )
        Text(
            "Capture customer + device before the bench starts.",
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )

        FormSection("Customer") {
        OutlinedTextField(
            value = customerName,
            onValueChange = { customerName = it },
            label = { Text("Full name (blank = walk-in)") },
            modifier = Modifier.fillMaxWidth(),
        )
        OutlinedTextField(
            value = customerPhone,
            onValueChange = { customerPhone = it },
            label = { Text("Phone") },
            modifier = Modifier.fillMaxWidth(),
        )
        }

        FormSection("Device") {
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            listOf("phone", "laptop", "tablet", "other").forEach { k ->
                FilterChip(selected = kind == k, onClick = { kind = k }, label = { Text(k) })
            }
        }
        OutlinedTextField(
            value = brand,
            onValueChange = { brand = it },
            label = { Text("Brand") },
            modifier = Modifier.fillMaxWidth(),
        )
        OutlinedTextField(
            value = model,
            onValueChange = { model = it },
            label = { Text("Model") },
            modifier = Modifier.fillMaxWidth(),
        )
        OutlinedTextField(
            value = imei,
            onValueChange = { imei = it },
            label = { Text("IMEI / Serial (optional)") },
            modifier = Modifier.fillMaxWidth(),
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
        }

        FormSection("Problem") {
        OutlinedTextField(
            value = problem,
            onValueChange = { problem = it },
            label = { Text("What is wrong with it?") },
            minLines = 2,
            modifier = Modifier.fillMaxWidth(),
        )
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            FilterChip(selected = assignToMe, onClick = { assignToMe = !assignToMe }, label = { Text("Assign to me") })
        }
        }

        error?.let { Text(it, color = MaterialTheme.colorScheme.error) }
        Button(
            onClick = {
                if (problem.isBlank()) {
                    error = "Describe the problem"
                    return@Button
                }
                busy = true
                error = null
                scope.launch {
                    try {
                        val jobId = withContext(Dispatchers.IO) {
                            val me = ApiClient.me()
                            val branches = me.optJSONArray("branch_ids")
                            val allowed = if (branches != null) (0 until branches.length()).map { branches.getString(it) } else emptyList()
                            val branchId = TechLaneApp.instance.tokenStore.selectedBranchId
                                ?.takeIf { it in allowed }
                                ?: allowed.firstOrNull()
                                ?: error("No branch on your account")
                            TechLaneApp.instance.tokenStore.selectedBranchId = branchId
                            val customerId = if (customerName.isNotBlank()) {
                                ApiClient.createCustomer(customerName.trim(), customerPhone.trim().ifBlank { null }).getString("id")
                            } else null
                            val deviceId = ApiClient.createDevice(
                                customerId = customerId,
                                kind = kind,
                                brand = brand.trim().ifBlank { null },
                                model = model.trim().ifBlank { null },
                                imei = imei.trim().ifBlank { null },
                            ).getString("id")
                            val id = ApiClient.createRepair(
                                branchId = branchId,
                                deviceId = deviceId,
                                problemSummary = problem.trim(),
                                customerId = customerId,
                                technicianId = if (assignToMe) me.optString("id") else null,
                            ).getString("id")
                            imeiPhoto?.let { (bytes, name) ->
                                ApiClient.addRepairAttachment(
                                    id,
                                    name.ifBlank { "imei-photo.jpg" },
                                    "image/jpeg",
                                    Base64.encodeToString(bytes, Base64.NO_WRAP),
                                )
                            }
                            devicePhoto?.let { (bytes, name) ->
                                ApiClient.addRepairAttachment(
                                    id,
                                    name.ifBlank { "device-condition.jpg" },
                                    "image/jpeg",
                                    Base64.encodeToString(bytes, Base64.NO_WRAP),
                                )
                            }
                            id
                        }
                        onCreated(jobId)
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
            Text(if (busy) "Creating…" else "Create repair job")
        }
        Button(
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

@Composable
fun JobDetailScreen(
    jobId: String,
    onBack: () -> Unit,
    isTechnician: Boolean = false,
    modifier: Modifier = Modifier,
) {
    var job by remember { mutableStateOf<JSONObject?>(null) }
    var payments by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var parts by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var notes by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var estimates by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var myId by remember { mutableStateOf("") }
    var newNote by remember { mutableStateOf("") }
    var amount by remember { mutableStateOf("") }
    var phone by remember { mutableStateOf("") }
    var method by remember { mutableStateOf("cash") }
    var partDesc by remember { mutableStateOf("") }
    var unitCost by remember { mutableStateOf("0") }
    var collectCodes by remember { mutableStateOf<Map<String, String>>(emptyMap()) }
    var labor by remember { mutableStateOf("") }
    var estimateLabor by remember { mutableStateOf("") }
    var estimateParts by remember { mutableStateOf("") }
    var estimateNotes by remember { mutableStateOf("") }
    var error by remember { mutableStateOf<String?>(null) }
    var message by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    val context = LocalContext.current

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

    val j = job
    Column(
        modifier = modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        TextButton(onClick = onBack) { Text("← Jobs") }
        if (j == null) {
            Text(error ?: "Loading…")
        } else {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
        ) {
            Text(
                j.optString("job_code").ifBlank { j.getString("id").take(8) },
                style = MaterialTheme.typography.headlineSmall,
            )
            StatusChip(j.optString("status"))
        }
        Text(j.optString("problem_summary"), style = MaterialTheme.typography.titleMedium)
        Text(
            "Opened ${timeAgo(j.optString("created_at"))}",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
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
                        } catch (e: Exception) {
                            error = e.message
                        } finally {
                            busy = false
                        }
                    }
                },
                enabled = !busy,
            ) {
                Text("Print receipt")
            }
        }

        Card(
            modifier = Modifier.fillMaxWidth(),
            shape = RoundedCornerShape(16.dp),
            elevation = CardDefaults.cardElevation(defaultElevation = 2.dp),
            colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
            border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline.copy(alpha = 0.18f)),
        ) {
            Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
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
            Button(
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
                modifier = Modifier.fillMaxWidth(),
            ) {
                Text(if (assignedTo.isBlank() || assignedTo == "null") "Claim this job" else "Reassign to me")
            }
        }

        val next = j.optJSONArray("next_statuses")
        if (next != null && next.length() > 0) {
            Text("Status", style = MaterialTheme.typography.titleMedium)
            if (next.toString().contains("completed")) {
                OutlinedTextField(
                    value = labor,
                    onValueChange = { labor = it },
                    label = { Text("Labor amount (KES) if completing") },
                    modifier = Modifier.fillMaxWidth(),
                )
            }
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                for (i in 0 until next.length()) {
                    val status = next.getString(i)
                    Button(
                        onClick = {
                            busy = true
                            error = null
                            message = null
                            scope.launch {
                                try {
                                    withContext(Dispatchers.IO) {
                                        ApiClient.updateRepairStatus(
                                            jobId,
                                            status,
                                            if (status == "completed") labor.toDoubleOrNull() else null,
                                        )
                                    }
                                    message = "Status → $status"
                                    refresh()
                                } catch (e: Exception) {
                                    error = e.message
                                } finally {
                                    busy = false
                                }
                            }
                        },
                        enabled = !busy,
                    ) {
                        Text("→ ${status.replace('_', ' ')}")
                    }
                }
            }
        }

        Text("Notes", style = MaterialTheme.typography.titleMedium)
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
        notes.forEach { n ->
            Card(
            modifier = Modifier.fillMaxWidth(),
            shape = RoundedCornerShape(16.dp),
            elevation = CardDefaults.cardElevation(defaultElevation = 2.dp),
            colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
            border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline.copy(alpha = 0.18f)),
        ) {
                Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
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

        Text("Parts", style = MaterialTheme.typography.titleMedium)
        OutlinedTextField(
            value = partDesc,
            onValueChange = { partDesc = it },
            label = { Text("Part description") },
            modifier = Modifier.fillMaxWidth(),
        )
        Button(
            onClick = {
                if (partDesc.isBlank()) {
                    error = "Describe the part"
                    return@Button
                }
                busy = true
                error = null
                message = null
                scope.launch {
                    try {
                        withContext(Dispatchers.IO) {
                            try {
                                ApiClient.createPartRequest(
                                    repairJobId = jobId,
                                    branchId = j.optString("branch_id").ifBlank { null },
                                    description = partDesc,
                                )
                            } catch (_: Exception) {
                                OutboxRepository.enqueue(
                                    SyncCommandTypes.PARTS_REQUEST,
                                    JSONObject()
                                        .put("repair_job_id", jobId)
                                        .put("branch_id", j.optString("branch_id"))
                                        .put("description", partDesc.trim())
                                        .put("quantity", 1)
                                        .put("part_request_id", UUID.randomUUID().toString()),
                                )
                            }
                        }
                        partDesc = ""
                        message = "Part requested (synced or queued offline)"
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
            Text("Request part")
        }
        parts.forEach { p ->
            val issue = p.optJSONObject("issue")
            val status = issue?.optString("status")?.takeIf { it.isNotBlank() } ?: p.optString("status")
            Card(
            modifier = Modifier.fillMaxWidth(),
            shape = RoundedCornerShape(16.dp),
            elevation = CardDefaults.cardElevation(defaultElevation = 2.dp),
            colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
            border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline.copy(alpha = 0.18f)),
        ) {
                Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
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
                            Button(
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
                                modifier = Modifier.fillMaxWidth(),
                            ) {
                                Text("Mark collected")
                            }
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

        Text("Customer estimates", style = MaterialTheme.typography.titleMedium)
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.fillMaxWidth()) {
            OutlinedTextField(
                value = estimateLabor,
                onValueChange = { estimateLabor = it },
                label = { Text("Labor (KES)") },
                modifier = Modifier.weight(1f),
            )
            OutlinedTextField(
                value = estimateParts,
                onValueChange = { estimateParts = it },
                label = { Text("Parts (KES)") },
                modifier = Modifier.weight(1f),
            )
        }
        OutlinedTextField(
            value = estimateNotes,
            onValueChange = { estimateNotes = it },
            label = { Text("Estimate notes (optional)") },
            modifier = Modifier.fillMaxWidth(),
        )
        Button(
            onClick = {
                val laborAmount = estimateLabor.toDoubleOrNull()
                val partsAmount = estimateParts.toDoubleOrNull()
                if (laborAmount == null || partsAmount == null || laborAmount < 0 || partsAmount < 0) {
                    error = "Enter valid estimate amounts"
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
                                laborAmount,
                                partsAmount,
                                estimateNotes.trim().ifBlank { null },
                            )
                        }
                        estimateLabor = ""
                        estimateParts = ""
                        estimateNotes = ""
                        message = "Estimate sent to customer"
                        refresh()
                    } catch (e: Exception) {
                        error = e.message
                    } finally {
                        busy = false
                    }
                }
            },
            enabled = !busy && estimateLabor.isNotBlank() && estimateParts.isNotBlank(),
            modifier = Modifier.fillMaxWidth(),
        ) {
            Text("Create estimate")
        }
        if (estimates.isEmpty()) {
            Text(
                "No estimates yet",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        estimates.forEach { estimate ->
            Card(
            modifier = Modifier.fillMaxWidth(),
            shape = RoundedCornerShape(16.dp),
            elevation = CardDefaults.cardElevation(defaultElevation = 2.dp),
            colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
            border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline.copy(alpha = 0.18f)),
        ) {
                Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                    ) {
                        Text(
                            "KES ${(estimate.optDouble("labor_amount") + estimate.optDouble("parts_amount")).toInt()}",
                            style = MaterialTheme.typography.titleSmall,
                        )
                        StatusChip(estimate.optString("status"))
                    }
                    Text(
                        "Labor ${estimate.optDouble("labor_amount").toInt()} · Parts ${estimate.optDouble("parts_amount").toInt()}",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
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

        if (!isTechnician) {
        Text("Payment", style = MaterialTheme.typography.titleMedium)
        Text(
            "Cash is provisional until handover. STK pushes; C2B waits for paybill.",
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
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
        Button(
            onClick = {
                val value = amount.toDoubleOrNull()
                if (value == null || value <= 0) {
                    error = "Enter a positive amount"
                    return@Button
                }
                if (method != "cash" && method != "mpesa_stk" && method != "mpesa_c2b") {
                    error = "Use cash, STK, or paybill"
                    return@Button
                }
                if (method == "mpesa_stk" && phone.isBlank()) {
                    error = "Phone required for STK"
                    return@Button
                }
                busy = true
                error = null
                message = null
                scope.launch {
                    try {
                        withContext(Dispatchers.IO) {
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
                            } catch (_: Exception) {
                                if (method != "cash") throw IllegalStateException("Online required for $method")
                                OutboxRepository.enqueue(
                                    SyncCommandTypes.PAYMENTS_CASH_PROVISIONAL,
                                    JSONObject()
                                        .put("branch_id", j.optString("branch_id"))
                                        .put("payable_type", "repair")
                                        .put("payable_id", jobId)
                                        .put("amount", value)
                                        .put("currency", "KES"),
                                )
                            }
                        }
                        message = when (method) {
                            "mpesa_stk" -> "STK sent"
                            "mpesa_c2b" -> "Awaiting paybill · ref ${j.optString("job_code").ifBlank { jobId.take(8) }}"
                            else -> "Cash recorded (synced or queued offline)"
                        }
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
            modifier = Modifier.fillMaxWidth(),
        ) {
            Text(
                when {
                    busy -> "Working…"
                    method == "mpesa_stk" -> "Send STK push"
                    method == "mpesa_c2b" -> "Await paybill"
                    else -> "Record cash"
                },
            )
        }

        
        Text("Payments on this job", style = MaterialTheme.typography.titleSmall)
        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            payments.forEach { p ->
                Card(
            modifier = Modifier.fillMaxWidth(),
            shape = RoundedCornerShape(16.dp),
            elevation = CardDefaults.cardElevation(defaultElevation = 2.dp),
            colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
            border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline.copy(alpha = 0.18f)),
        ) {
                    Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                        Text(
                            "${p.optString("method")} · KES ${p.optDouble("amount", 0.0).toInt()}",
                            style = MaterialTheme.typography.titleSmall,
                        )
                        Text(p.optString("status"))
                        val st = p.optString("status")
                        if (p.optString("method") == "mpesa_stk" && (st == "initiated" || st == "pending")) {
                            Button(
                                onClick = {
                                    busy = true
                                    scope.launch {
                                        try {
                                            withContext(Dispatchers.IO) {
                                                ApiClient.confirmMpesaPayment(p.getString("id"))
                                            }
                                            message = "STK reconciled via Query API"
                                            refresh()
                                        } catch (e: Exception) {
                                            error = e.message
                                        } finally {
                                            busy = false
                                        }
                                    }
                                },
                                enabled = !busy,
                            ) {
                                Text("Reconcile")
                            }
                        }
                        if (st == "allocated" || st == "confirmed") {
                            Button(
                                onClick = {
                                    busy = true
                                    error = null
                                    message = null
                                    scope.launch {
                                        try {
                                            withContext(Dispatchers.IO) {
                                                ApiClient.createRefund(
                                                    paymentId = p.getString("id"),
                                                    amount = p.optDouble("amount", 0.0),
                                                    reason = "Job ${j.optString("job_code").ifBlank { jobId.take(8) }}",
                                                )
                                            }
                                            message = "Refund requested — needs another approver"
                                            refresh()
                                        } catch (e: Exception) {
                                            error = e.message
                                        } finally {
                                            busy = false
                                        }
                                    }
                                },
                                enabled = !busy,
                            ) {
                                Text("Request full refund")
                            }
                        }
                    }
                }
            }
        }

        }
        error?.let { Text(it, color = MaterialTheme.colorScheme.error) }
        message?.let { Text(it, color = MaterialTheme.colorScheme.primary) }
        val timeline = j.optJSONArray("timeline")
        if (timeline != null && timeline.length() > 0) {
            Text("History", style = MaterialTheme.typography.titleSmall)
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

@Composable
fun SyncCenterScreen(modifier: Modifier = Modifier) {
    var items by remember { mutableStateOf<List<SyncCommandEntity>>(emptyList()) }
    var pending by remember { mutableStateOf(0) }
    var error by remember { mutableStateOf<String?>(null) }
    var message by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    val dao = TechLaneApp.instance.database.syncOutboxDao()

    fun refresh() {
        scope.launch {
            items = dao.recent()
            pending = dao.pendingCount()
        }
    }

    LaunchedEffect(Unit) { refresh() }

    Column(modifier = modifier.fillMaxSize()) {
        Column(modifier = Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            ScreenHeader(
                title = "Outbox",
                subtitle = "$pending command(s) waiting to sync",
            )
            Text(
                "Drafts sync over WorkManager. Tap Sync now to flush immediately.",
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                style = MaterialTheme.typography.bodyMedium,
            )
            Button(
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
                modifier = Modifier.fillMaxWidth(),
            ) {
                Text(if (busy) "Syncing…" else "Sync now")
            }
            Button(
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
            error?.let { Text(it, color = MaterialTheme.colorScheme.error) }
            message?.let { Text(it, color = MaterialTheme.colorScheme.primary) }
        }
        LazyColumn(
            contentPadding = PaddingValues(16.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            items(items, key = { it.actionId }) { cmd ->
                Card(
            modifier = Modifier.fillMaxWidth(),
            shape = RoundedCornerShape(16.dp),
            elevation = CardDefaults.cardElevation(defaultElevation = 2.dp),
            colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
            border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline.copy(alpha = 0.18f)),
        ) {
                    Column(modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                        Text(cmd.commandType, style = MaterialTheme.typography.titleSmall)
                        Text(cmd.syncStatus, style = MaterialTheme.typography.labelLarge)
                        Text(cmd.actionId.take(8) + "… · retries ${cmd.retryCount}")
                        cmd.lastError?.let {
                            Text(it, color = MaterialTheme.colorScheme.error)
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
                                    onClick = {
                                        scope.launch {
                                            dao.delete(cmd.actionId)
                                            refresh()
                                        }
                                    },
                                ) { Text("Discard") }
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
fun CashScreen(selectedBranchId: String? = null, modifier: Modifier = Modifier) {
    var userId by remember { mutableStateOf("") }
    var pendingCash by remember { mutableStateOf(0.0) }
    var handovers by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var refunds by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var amount by remember { mutableStateOf("") }
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

    Column(modifier = modifier.fillMaxSize()) {
        Column(modifier = Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            ScreenHeader(
                title = "Provisional cash",
                subtitle = "Cash stays provisional until another staff member confirms the count.",
            )
            Text(
                "KES ${pendingCash.toInt()}",
                style = MaterialTheme.typography.displaySmall,
                fontWeight = FontWeight.Bold,
            )
            OutlinedTextField(
                value = amount,
                onValueChange = { amount = it },
                label = { Text("Handover amount (KES)") },
                modifier = Modifier.fillMaxWidth(),
            )
            Button(
                onClick = {
                    val value = amount.toDoubleOrNull()
                    if (value == null || value <= 0) {
                        error = "Enter a positive amount"
                        return@Button
                    }
                    busy = true
                    message = null
                    error = null
                    scope.launch {
                        try {
                            withContext(Dispatchers.IO) {
                                ApiClient.requestCashHandover(value, selectedBranchId)
                            }
                            message = "Handover requested"
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
                modifier = Modifier.fillMaxWidth(),
            ) {
                Text(if (busy) "Working…" else "Request handover")
            }
            error?.let { Text(it, color = MaterialTheme.colorScheme.error) }
            message?.let { Text(it, color = MaterialTheme.colorScheme.primary) }
        }

        Text(
            "Queue",
            style = MaterialTheme.typography.titleMedium,
            modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
        )
        LazyColumn(
            contentPadding = PaddingValues(16.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            items(handovers, key = { it.getString("id") }) { h ->
                val id = h.getString("id")
                val declared = h.optDouble("amount", 0.0)
                val status = h.optString("status")
                val fromMe = h.optString("from_user_id") == userId
                val shortage = h.optDouble("shortage_amount", 0.0)
                Card(
            modifier = Modifier.fillMaxWidth(),
            shape = RoundedCornerShape(16.dp),
            elevation = CardDefaults.cardElevation(defaultElevation = 2.dp),
            colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
            border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline.copy(alpha = 0.18f)),
        ) {
                    Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        Text("KES ${declared.toInt()}", style = MaterialTheme.typography.titleMedium)
                        Text(status, style = MaterialTheme.typography.labelLarge)
                        if (status == "confirmed" && shortage > 0) {
                            Text(
                                "Shortage KES ${shortage.toInt()}",
                                color = MaterialTheme.colorScheme.error,
                            )
                        }
                        if (status == "requested" && fromMe) {
                            Text(
                                "Waiting for another staff member to confirm.",
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
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
                            Button(
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
                                modifier = Modifier.fillMaxWidth(),
                            ) {
                                Text("Confirm received")
                            }
                        }
                    }
                }
            }

            item {
                Text("Refunds", style = MaterialTheme.typography.titleMedium)
                Text(
                    "Approve requests from another staff member. You cannot approve your own.",
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            if (refunds.isEmpty()) {
                item {
                    Text(
                        "No refund requests",
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
            items(refunds, key = { it.getString("id") }) { r ->
                val rid = r.getString("id")
                val status = r.optString("status")
                val mine = r.optString("created_by") == userId
                Card(
            modifier = Modifier.fillMaxWidth(),
            shape = RoundedCornerShape(16.dp),
            elevation = CardDefaults.cardElevation(defaultElevation = 2.dp),
            colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
            border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline.copy(alpha = 0.18f)),
        ) {
                    Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        Text(
                            "KES ${r.optDouble("amount", 0.0).toInt()}",
                            style = MaterialTheme.typography.titleMedium,
                        )
                        Text(status, style = MaterialTheme.typography.labelLarge)
                        r.optString("reason").takeIf { it.isNotBlank() }?.let { Text(it) }
                        Text(
                            "Payment ${r.optString("payment_id").take(8)}…",
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                        if (status == "pending" && mine) {
                            Text(
                                "Waiting for another approver.",
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                        }
                        if (status == "pending" && !mine) {
                            Button(
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
                                modifier = Modifier.fillMaxWidth(),
                            ) {
                                Text("Approve refund")
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
fun PickupScreen(modifier: Modifier = Modifier) {
    var code by remember { mutableStateOf("") }
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

    Column(modifier = modifier.fillMaxSize()) {
        Column(modifier = Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            ScreenHeader(
                title = "Branch pickup",
                subtitle = "Enter the customer collection code after online payment. Marks the order delivered.",
            )
            OutlinedTextField(
                value = code,
                onValueChange = { code = it.uppercase() },
                label = { Text("Collection code") },
                modifier = Modifier.fillMaxWidth(),
            )
            Button(
                onClick = {
                    if (code.trim().length < 4) {
                        error = "Enter a valid code"
                        return@Button
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
                modifier = Modifier.fillMaxWidth(),
            ) {
                Text(if (busy) "Working…" else "Mark collected")
            }
            error?.let { Text(it, color = MaterialTheme.colorScheme.error) }
            message?.let { Text(it, color = MaterialTheme.colorScheme.primary) }
        }

        Text(
            "Ready for pickup (${orders.size})",
            style = MaterialTheme.typography.titleMedium,
            modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
        )
        LazyColumn(
            contentPadding = PaddingValues(16.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            items(orders, key = { it.getString("id") }) { o ->
                Card(
            modifier = Modifier.fillMaxWidth(),
            shape = RoundedCornerShape(16.dp),
            elevation = CardDefaults.cardElevation(defaultElevation = 2.dp),
            colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
            border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline.copy(alpha = 0.18f)),
        ) {
                    Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                        Text(
                            "KES ${o.optDouble("total", 0.0).toInt()}",
                            style = MaterialTheme.typography.titleMedium,
                        )
                        Text(o.optString("status"), style = MaterialTheme.typography.labelLarge)
                        val c = o.optString("collection_code")
                        if (c.isNotBlank()) {
                            Text("Code $c", style = MaterialTheme.typography.titleSmall)
                        }
                        Text(
                            o.optString("id").take(8) + "…",
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                        Button(
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
