package com.techlane.supplier.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.automirrored.filled.Logout
import androidx.compose.material.icons.filled.AccountBalance
import androidx.compose.material.icons.filled.History
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.Inbox
import androidx.compose.material.icons.filled.Person
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Visibility
import androidx.compose.material.icons.filled.VisibilityOff
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.FilterChipDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.NavigationBarItemDefaults
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.techlane.core.PrintSupport
import com.techlane.core.qr.QrBitmap
import com.techlane.core.theme.Brand
import com.techlane.core.ui.BrandAuthHeader
import com.techlane.core.ui.BrandCard
import com.techlane.core.ui.BrandDetailHeader
import com.techlane.core.ui.BrandHero
import com.techlane.core.ui.BrandSectionTitle
import com.techlane.core.ui.GoldButton
import com.techlane.core.ui.PillBadge
import com.techlane.core.ui.SafeBottomBar
import com.techlane.core.ui.brandGradient
import com.techlane.supplier.SupplierApp
import com.techlane.supplier.network.SupplierApi
import androidx.compose.foundation.Image
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONObject

@Composable
fun SupplierNav(
    signedIn: Boolean,
    onSignedIn: () -> Unit,
    onSignedOut: () -> Unit,
    rootModifier: Modifier = Modifier,
) {
    var sessionExpired by remember { mutableStateOf(false) }
    DisposableEffect(onSignedOut) {
        SupplierApi.setSessionExpiredListener {
            android.os.Handler(android.os.Looper.getMainLooper()).post {
                sessionExpired = true
                onSignedOut()
            }
        }
        onDispose { SupplierApi.setSessionExpiredListener(null) }
    }
    if (!signedIn) {
        SupplierAuthScreen(
            onSignedIn = {
                sessionExpired = false
                onSignedIn()
            },
            sessionExpired = sessionExpired,
            rootModifier = rootModifier,
        )
    } else {
        SupplierShell(onSignedOut = onSignedOut, rootModifier = rootModifier)
    }
}

@Composable
fun SupplierAuthScreen(onSignedIn: () -> Unit, sessionExpired: Boolean = false, rootModifier: Modifier = Modifier) {
    var mode by remember { mutableStateOf("login") }
    var email by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var passwordVisible by remember { mutableStateOf(false) }
    var inviteToken by remember { mutableStateOf("") }
    var error by remember { mutableStateOf<String?>(null) }
    var notice by remember { mutableStateOf<String?>(if (sessionExpired) "Your session expired. Please sign in again." else null) }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()

    Box(
        rootModifier
            .fillMaxSize()
            .background(brandGradient()),
    ) {
        Column(
            Modifier
                .fillMaxSize()
                .statusBarsPadding()
                .navigationBarsPadding(),
        ) {
            BrandAuthHeader(
                appLabel = "Supplier",
                tagline = "Quote. Supply. Get paid.",
                modifier = Modifier
                    .padding(horizontal = 28.dp)
                    .padding(top = 36.dp, bottom = 24.dp),
            )
            Spacer(Modifier.weight(1f))
            Surface(
                modifier = Modifier.fillMaxWidth(),
                shape = RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp),
                color = Brand.Surface,
            ) {
                Column(
                    Modifier.padding(horizontal = 24.dp, vertical = 28.dp),
                    verticalArrangement = Arrangement.spacedBy(14.dp),
                ) {
                    notice?.let {
                        Text(it, color = Brand.Warning, style = MaterialTheme.typography.bodySmall)
                    }
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        FilterChip(
                            selected = mode == "login",
                            onClick = { mode = "login" },
                            label = { Text("Sign in") },
                            colors = FilterChipDefaults.filterChipColors(
                                selectedContainerColor = Brand.NavyTint,
                                selectedLabelColor = Brand.Navy,
                            ),
                        )
                        FilterChip(
                            selected = mode == "invite",
                            onClick = { mode = "invite" },
                            label = { Text("Accept invite") },
                            colors = FilterChipDefaults.filterChipColors(
                                selectedContainerColor = Brand.NavyTint,
                                selectedLabelColor = Brand.Navy,
                            ),
                        )
                    }
                    if (mode == "login") {
                        OutlinedTextField(
                            value = email,
                            onValueChange = { email = it },
                            label = { Text("Email") },
                            singleLine = true,
                            modifier = Modifier.fillMaxWidth(),
                        )
                    } else {
                        OutlinedTextField(
                            value = inviteToken,
                            onValueChange = { inviteToken = it },
                            label = { Text("Invite token") },
                            singleLine = true,
                            modifier = Modifier.fillMaxWidth(),
                        )
                    }
                    OutlinedTextField(
                        value = password,
                        onValueChange = { password = it },
                        label = { Text(if (mode == "invite") "Set password" else "Password") },
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
                    error?.let {
                        Text(it, color = Brand.Danger, style = MaterialTheme.typography.bodySmall)
                    }
                    GoldButton(
                        text = if (mode == "login") "Sign in" else "Activate account",
                        onClick = {
                            busy = true
                            error = null
                            notice = null
                            scope.launch {
                                try {
                                    val res = withContext(Dispatchers.IO) {
                                        if (mode == "login") {
                                            SupplierApi.login(email.trim(), password)
                                        } else {
                                            SupplierApi.acceptInvite(inviteToken.trim(), password)
                                        }
                                    }
                                    SupplierApp.instance.tokenStore.sessionToken = res.getString("token")
                                    res.optJSONObject("contact")?.optString("display_name")
                                        ?.takeIf { it.isNotBlank() }
                                        ?.let { SupplierApp.instance.tokenStore.displayName = it }
                                    onSignedIn()
                                } catch (e: Exception) {
                                    error = e.message
                                } finally {
                                    busy = false
                                }
                            }
                        },
                        enabled = !busy && password.length >= 6 &&
                            (mode == "login" && email.isNotBlank() || mode == "invite" && inviteToken.isNotBlank()),
                        loading = busy,
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SupplierShell(onSignedOut: () -> Unit, rootModifier: Modifier = Modifier) {
    var tab by remember { mutableStateOf("queue") }
    var selectedRequestId by remember { mutableStateOf<String?>(null) }
    var issuedQr by remember { mutableStateOf<JSONObject?>(null) }

    if (issuedQr != null) {
        IssueQrScreen(issue = issuedQr!!, onBack = { issuedQr = null }, rootModifier = rootModifier)
        return
    }
    if (selectedRequestId != null) {
        RequestDetailScreen(
            requestId = selectedRequestId!!,
            onBack = { selectedRequestId = null },
            onIssued = {
                issuedQr = it
                selectedRequestId = null
            },
            rootModifier = rootModifier,
        )
        return
    }

    Scaffold(
        modifier = rootModifier,
        contentWindowInsets = WindowInsets(0, 0, 0, 0),
        bottomBar = {
            SafeBottomBar(containerColor = Color.White) {
                NavigationBar(
                    containerColor = Color.White,
                    windowInsets = WindowInsets(0, 0, 0, 0),
                ) {
                    val itemColors = NavigationBarItemDefaults.colors(
                        selectedIconColor = Brand.Navy,
                        selectedTextColor = Brand.Navy,
                        indicatorColor = Brand.NavyTint,
                        unselectedIconColor = Brand.TextMuted,
                        unselectedTextColor = Brand.TextMuted,
                    )
                    NavigationBarItem(
                        selected = tab == "queue",
                        onClick = { tab = "queue" },
                        icon = { Icon(Icons.Default.Home, null) },
                        label = { Text("Queue") },
                        colors = itemColors,
                    )
                    NavigationBarItem(
                        selected = tab == "issues",
                        onClick = { tab = "issues" },
                        icon = { Icon(Icons.Default.History, null) },
                        label = { Text("Issued") },
                        colors = itemColors,
                    )
                    NavigationBarItem(
                        selected = tab == "credit",
                        onClick = { tab = "credit" },
                        icon = { Icon(Icons.Default.AccountBalance, null) },
                        label = { Text("Credit") },
                        colors = itemColors,
                    )
                    NavigationBarItem(
                        selected = tab == "profile",
                        onClick = { tab = "profile" },
                        icon = { Icon(Icons.Default.Person, null) },
                        label = { Text("Profile") },
                        colors = itemColors,
                    )
                }
            }
        },
    ) { padding ->
        when (tab) {
            "issues" -> IssuesScreen(onShowQr = { issuedQr = it }, rootModifier = Modifier.padding(padding))
            "credit" -> CreditScreen(rootModifier = Modifier.padding(padding))
            "profile" -> SupplierProfileScreen(
                onSignedOut = onSignedOut,
                rootModifier = Modifier.padding(padding),
            )
            else -> RequestQueueScreen(
                onOpen = { selectedRequestId = it },
                onSignedOut = onSignedOut,
                rootModifier = Modifier.padding(padding),
            )
        }
    }
}

@Composable
private fun HeroFilterChip(
    label: String,
    selected: Boolean,
    onClick: () -> Unit,
) {
    Surface(
        onClick = onClick,
        shape = RoundedCornerShape(999.dp),
        color = if (selected) Brand.Gold else Color.White.copy(alpha = 0.12f),
    ) {
        Text(
            label,
            style = MaterialTheme.typography.labelLarge,
            fontWeight = FontWeight.SemiBold,
            color = if (selected) Brand.NavyDark else Color.White,
            modifier = Modifier.padding(horizontal = 14.dp, vertical = 8.dp),
        )
    }
}

private fun requestStatusBadge(status: String): Pair<String, Color> {
    val s = status.lowercase()
    return when {
        s in listOf("awaiting", "assigned", "open", "pending", "invited") -> "New" to Brand.Info
        s in listOf("accepted", "quoted") -> "Quoted" to Brand.GoldDark
        s == "ready" -> "Ready" to Brand.Success
        s == "declined" -> "Declined" to Brand.Danger
        else -> status.replace('_', ' ').replaceFirstChar { it.uppercase() } to Brand.TextMuted
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun RequestQueueScreen(
    onOpen: (String) -> Unit,
    onSignedOut: () -> Unit = {},
    rootModifier: Modifier = Modifier,
) {
    var filter by remember { mutableStateOf<String?>(null) }
    var items by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var loading by remember { mutableStateOf(true) }
    var error by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()
    val chipScroll = rememberScrollState()
    val displayName = SupplierApp.instance.tokenStore.displayName

    fun refresh() {
        scope.launch {
            loading = true
            error = null
            try {
                val arr = withContext(Dispatchers.IO) { SupplierApi.listRequests(filter) }
                items = (0 until arr.length()).map { arr.getJSONObject(it) }
            } catch (e: Exception) {
                error = e.message
            } finally {
                loading = false
            }
        }
    }

    LaunchedEffect(filter) { refresh() }

    Column(rootModifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        BrandHero(
            title = "Part requests",
            subtitle = displayName?.takeIf { it.isNotBlank() },
            appLabel = "Supplier",
            trailing = {
                IconButton(onClick = { refresh() }) {
                    Icon(Icons.Default.Refresh, "Refresh", tint = Color.White)
                }
                IconButton(onClick = {
                    kotlinx.coroutines.MainScope().launch(Dispatchers.IO) { SupplierApi.logout() }
                    onSignedOut()
                }) {
                    Icon(Icons.AutoMirrored.Filled.Logout, "Sign out", tint = Color.White)
                }
            },
            bottomContent = {
                Row(
                    Modifier.horizontalScroll(chipScroll),
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    listOf(null to "All", "assigned" to "New", "quoted" to "Quoted", "ready" to "Ready")
                        .forEach { (value, label) ->
                            HeroFilterChip(
                                label = label,
                                selected = filter == value,
                                onClick = { filter = value },
                            )
                        }
                }
            },
        )
        when {
            loading -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                CircularProgressIndicator(color = Brand.Navy)
            }
            error != null -> ErrorState(error!!, onRetry = { refresh() })
            items.isEmpty() -> EmptyState("Queue is clear", "New part requests assigned to you will show up here.")
            else -> LazyColumn(
                contentPadding = PaddingValues(16.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                items(items, key = { it.optString("id") }) { req ->
                    val (badgeLabel, badgeColor) = requestStatusBadge(req.optString("status"))
                    BrandCard(onClick = { onOpen(req.getString("id")) }) {
                        Row(
                            Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.SpaceBetween,
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Text(
                                req.optString("part_name").ifBlank {
                                    req.optString("description").ifBlank { req.optString("sku") }
                                },
                                style = MaterialTheme.typography.titleMedium,
                                fontWeight = FontWeight.Bold,
                                color = Brand.TextPrimary,
                                modifier = Modifier.weight(1f),
                            )
                            PillBadge(text = badgeLabel, color = badgeColor)
                        }
                        Spacer(Modifier.height(8.dp))
                        Text(
                            "Qty ${req.optInt("quantity", 1)} · ${req.optString("branch_name").ifBlank { "Branch" }}",
                            color = Brand.TextMuted,
                            style = MaterialTheme.typography.bodyMedium,
                        )
                        req.optString("job_code").takeIf { it.isNotBlank() }?.let {
                            Spacer(Modifier.height(4.dp))
                            Text(
                                "Job $it",
                                style = MaterialTheme.typography.bodySmall,
                                color = Brand.TextSecondary,
                            )
                        }
                    }
                }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun RequestDetailScreen(
    requestId: String,
    onBack: () -> Unit,
    onIssued: (JSONObject) -> Unit,
    rootModifier: Modifier = Modifier,
) {
    var detail by remember { mutableStateOf<JSONObject?>(null) }
    var unitCost by remember { mutableStateOf("") }
    var notes by remember { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()

    fun refresh() {
        scope.launch {
            try {
                detail = withContext(Dispatchers.IO) { SupplierApi.requestDetail(requestId) }
                error = null
            } catch (e: Exception) {
                error = e.message
            }
        }
    }

    LaunchedEffect(requestId) { refresh() }

    val req = detail
    val headerTitle = req?.optString("part_name")?.takeIf { it.isNotBlank() } ?: "Request"

    Column(
        rootModifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background),
    ) {
        BrandDetailHeader(
            title = headerTitle,
            navigation = {
                IconButton(onClick = onBack) {
                    Icon(Icons.AutoMirrored.Filled.ArrowBack, "Back", tint = Color.White)
                }
            },
        )
        if (req == null) {
            Box(
                Modifier.fillMaxSize(),
                contentAlignment = Alignment.Center,
            ) {
                if (error != null) Text(error!!, color = Brand.Danger)
                else CircularProgressIndicator(color = Brand.Navy)
            }
            return
        }
        Column(
            Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .navigationBarsPadding()
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp),
        ) {
            BrandCard {
                Text(
                    req.optString("part_name").ifBlank { "Part request" },
                    style = MaterialTheme.typography.titleLarge,
                    fontWeight = FontWeight.Bold,
                    color = Brand.TextPrimary,
                )
                Spacer(Modifier.height(8.dp))
                val (badgeLabel, badgeColor) = requestStatusBadge(req.optString("status"))
                PillBadge(text = badgeLabel, color = badgeColor)
                Spacer(Modifier.height(8.dp))
                Text(
                    "Quantity: ${req.optInt("quantity", 1)}",
                    color = Brand.TextSecondary,
                )
                req.optString("notes").takeIf { it.isNotBlank() }?.let {
                    Spacer(Modifier.height(6.dp))
                    Text(it, color = Brand.TextMuted)
                }
            }

            if (req.optString("status") in listOf("assigned", "open", "pending", "invited")) {
                BrandCard {
                    BrandSectionTitle("Your quote")
                    Spacer(Modifier.height(8.dp))
                    OutlinedTextField(
                        value = unitCost,
                        onValueChange = { unitCost = it.filter { ch -> ch.isDigit() || ch == '.' } },
                        label = { Text("Unit cost (KES)") },
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth(),
                    )
                    Spacer(Modifier.height(10.dp))
                    OutlinedTextField(
                        value = notes,
                        onValueChange = { notes = it },
                        label = { Text("Notes (optional)") },
                        modifier = Modifier.fillMaxWidth(),
                    )
                    error?.let {
                        Spacer(Modifier.height(8.dp))
                        Text(it, color = Brand.Danger)
                    }
                    Spacer(Modifier.height(14.dp))
                    GoldButton(
                        text = "Submit quote",
                        onClick = {
                            busy = true
                            scope.launch {
                                try {
                                    withContext(Dispatchers.IO) {
                                        SupplierApi.quote(requestId, unitCost.toDouble(), notes)
                                    }
                                    refresh()
                                } catch (e: Exception) {
                                    error = e.message
                                } finally {
                                    busy = false
                                }
                            }
                        },
                        enabled = !busy && unitCost.toDoubleOrNull() != null,
                        loading = busy,
                        modifier = Modifier.fillMaxWidth(),
                    )
                    Spacer(Modifier.height(10.dp))
                    OutlinedButton(
                        onClick = {
                            busy = true
                            scope.launch {
                                try {
                                    withContext(Dispatchers.IO) { SupplierApi.decline(requestId, notes) }
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
                        border = BorderStroke(1.dp, Brand.Danger.copy(alpha = 0.45f)),
                    ) {
                        Text("Decline request", color = Brand.Danger)
                    }
                }
            }

            val issueObj = req.optJSONObject("issue")
            val quoteStatus = req.optString("quote_status")

            if (issueObj != null || quoteStatus in listOf("accepted", "quoted")) {
                if (quoteStatus != "ready") {
                    OutlinedButton(
                        onClick = {
                            busy = true
                            scope.launch {
                                try {
                                    withContext(Dispatchers.IO) { SupplierApi.markReady(requestId) }
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
                    ) { Text("Mark ready for collection") }
                } else {
                    Text("Marked ready for collection", color = Brand.Success, fontWeight = FontWeight.SemiBold)
                }
            }

            if (issueObj != null) {
                BrandCard {
                    if (issueObj.optString("status") == "collected") {
                        Text(
                            "Part collected",
                            color = Brand.Success,
                            fontWeight = FontWeight.SemiBold,
                        )
                    } else {
                        Text(
                            "AUTH CODE",
                            style = MaterialTheme.typography.labelMedium,
                            color = Brand.TextMuted,
                            fontWeight = FontWeight.SemiBold,
                        )
                        Spacer(Modifier.height(6.dp))
                        Text(
                            issueObj.optString("auth_code"),
                            style = MaterialTheme.typography.headlineMedium,
                            fontFamily = FontFamily.Monospace,
                            fontWeight = FontWeight.Bold,
                            letterSpacing = 3.sp,
                            color = Brand.Navy,
                        )
                    }
                    Spacer(Modifier.height(14.dp))
                    GoldButton(
                        text = "Show collection QR",
                        onClick = { onIssued(issueObj) },
                        enabled = !busy,
                        modifier = Modifier.fillMaxWidth(),
                    )
                    error?.let {
                        Spacer(Modifier.height(8.dp))
                        Text(it, color = Brand.Danger)
                    }
                }
            } else if (quoteStatus in listOf("accepted", "quoted", "ready")) {
                GoldButton(
                    text = "Issue part + show QR",
                    onClick = {
                        busy = true
                        scope.launch {
                            try {
                                val issue = withContext(Dispatchers.IO) { SupplierApi.issue(requestId) }
                                onIssued(issue)
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

@Composable
fun IssueQrScreen(issue: JSONObject, onBack: () -> Unit, rootModifier: Modifier = Modifier) {
    val nested = issue.optJSONObject("issue")
    val issueId = issue.optString("id")
        .ifBlank { issue.optString("issue_id") }
        .ifBlank { nested?.optString("id").orEmpty() }
    val authCode = issue.optString("auth_code")
        .ifBlank { nested?.optString("auth_code").orEmpty() }
    val payload = issue.optString("qr_payload")
        .ifBlank { "techlane://auth/$issueId/$authCode" }
    val qrBitmap = remember(payload) { QrBitmap.encode(payload, 640) }
    val scope = rememberCoroutineScope()
    val context = LocalContext.current
    var busy by remember { mutableStateOf(false) }
    var collectMessage by remember { mutableStateOf<String?>(null) }
    var collectError by remember { mutableStateOf<String?>(null) }

    Column(
        rootModifier
            .fillMaxSize()
            .background(brandGradient())
            .statusBarsPadding()
            .navigationBarsPadding()
            .padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        IconButton(onClick = onBack, modifier = Modifier.align(Alignment.Start)) {
            Icon(Icons.AutoMirrored.Filled.ArrowBack, "Back", tint = Color.White)
        }
        Text(
            "Collection QR",
            style = MaterialTheme.typography.headlineSmall,
            color = Color.White,
            fontWeight = FontWeight.Bold,
        )
        Text(
            "Show this to shop staff to collect the part.",
            textAlign = TextAlign.Center,
            color = Color.White.copy(alpha = 0.72f),
        )
        Surface(
            shape = RoundedCornerShape(20.dp),
            color = Color.White,
            shadowElevation = 4.dp,
            modifier = Modifier.fillMaxWidth(),
        ) {
            Box(
                Modifier
                    .fillMaxWidth()
                    .padding(22.dp),
                contentAlignment = Alignment.Center,
            ) {
                Image(
                    bitmap = qrBitmap.asImageBitmap(),
                    contentDescription = "Collection QR code",
                    modifier = Modifier.size(220.dp),
                )
            }
        }
        Text(
            "AUTH CODE",
            style = MaterialTheme.typography.labelMedium,
            color = Color.White.copy(alpha = 0.68f),
            fontWeight = FontWeight.SemiBold,
        )
        Text(
            authCode.ifBlank { "—" },
            style = MaterialTheme.typography.headlineMedium,
            fontFamily = FontFamily.Monospace,
            fontWeight = FontWeight.Bold,
            letterSpacing = 3.sp,
            color = Color.White,
        )
        collectMessage?.let { Text(it, color = Brand.Gold) }
        collectError?.let { Text(it, color = Color(0xFFFCA5A5)) }
        if (issueId.isNotBlank()) {
            GoldButton(
                text = "Confirm handover",
                onClick = {
                    collectError = null
                    collectMessage = null
                    busy = true
                    scope.launch {
                        try {
                            withContext(Dispatchers.IO) {
                                SupplierApi.collect(issueId, authCode)
                            }
                            collectMessage = "Marked as collected"
                        } catch (e: Exception) {
                            collectError = e.message
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
        if (issueId.isNotBlank()) {
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.fillMaxWidth()) {
                OutlinedButton(
                    onClick = {
                        busy = true
                        scope.launch {
                            try {
                                val html = withContext(Dispatchers.IO) {
                                    PrintSupport.fetchText(
                                        SupplierApi.voucherHtmlUrl(issueId),
                                        SupplierApp.instance.tokenStore.sessionToken,
                                    )
                                }
                                PrintSupport.printHtml(context, html, "Credit voucher")
                            } finally {
                                busy = false
                            }
                        }
                    },
                    enabled = !busy,
                    modifier = Modifier.weight(1f),
                    border = BorderStroke(1.dp, Color.White.copy(alpha = 0.4f)),
                    colors = androidx.compose.material3.ButtonDefaults.outlinedButtonColors(
                        contentColor = Color.White,
                    ),
                ) { Text("Print voucher") }
                OutlinedButton(
                    onClick = {
                        busy = true
                        scope.launch {
                            try {
                                val html = withContext(Dispatchers.IO) {
                                    PrintSupport.fetchText(
                                        SupplierApi.voucherHtmlUrl(issueId),
                                        SupplierApp.instance.tokenStore.sessionToken,
                                    )
                                }
                                PrintSupport.shareText(context, html, "Share voucher")
                            } finally {
                                busy = false
                            }
                        }
                    },
                    enabled = !busy,
                    modifier = Modifier.weight(1f),
                    border = BorderStroke(1.dp, Color.White.copy(alpha = 0.4f)),
                    colors = androidx.compose.material3.ButtonDefaults.outlinedButtonColors(
                        contentColor = Color.White,
                    ),
                ) { Text("Share") }
            }
        }
        OutlinedButton(
            onClick = onBack,
            modifier = Modifier.fillMaxWidth().height(52.dp),
            border = BorderStroke(1.dp, Color.White.copy(alpha = 0.4f)),
            colors = androidx.compose.material3.ButtonDefaults.outlinedButtonColors(
                contentColor = Color.White,
            ),
        ) { Text("Done") }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun IssuesScreen(onShowQr: (JSONObject) -> Unit, rootModifier: Modifier = Modifier) {
    var items by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var error by remember { mutableStateOf<String?>(null) }
    LaunchedEffect(Unit) {
        try {
            val arr = withContext(Dispatchers.IO) { SupplierApi.listIssues() }
            items = (0 until arr.length()).map { arr.getJSONObject(it) }
        } catch (e: Exception) {
            error = e.message
        }
    }
    Column(rootModifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        BrandHero(
            title = "Issued parts",
            appLabel = "Supplier",
        )
        when {
            error != null -> ErrorState(error!!) {}
            items.isEmpty() -> EmptyState("No issued parts yet", "After you issue a part, the collection code appears here.")
            else -> LazyColumn(
                contentPadding = PaddingValues(16.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                items(items, key = { it.optString("id") }) { issue ->
                    val (badgeLabel, badgeColor) = requestStatusBadge(issue.optString("status"))
                    BrandCard(onClick = { onShowQr(issue) }) {
                        Text(
                            issue.optString("part_name").ifBlank {
                                issue.optString("description").ifBlank { "Issue" }
                            },
                            style = MaterialTheme.typography.titleMedium,
                            fontWeight = FontWeight.Bold,
                            color = Brand.TextPrimary,
                        )
                        Spacer(Modifier.height(8.dp))
                        PillBadge(text = badgeLabel, color = badgeColor)
                        Spacer(Modifier.height(6.dp))
                        Text(
                            "KES ${"%.0f".format(issue.optDouble("unit_cost", 0.0))}",
                            color = Brand.TextSecondary,
                        )
                    }
                }
            }
        }
    }
}

@Composable
fun CreditScreen(rootModifier: Modifier = Modifier) {
    var credit by remember { mutableStateOf<JSONObject?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    LaunchedEffect(Unit) {
        try {
            credit = withContext(Dispatchers.IO) { SupplierApi.credit() }
        } catch (e: Exception) {
            error = e.message
        }
    }
    Column(
        rootModifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background),
    ) {
        BrandHero(
            title = "Credit",
            appLabel = "Supplier",
        )
        Column(
            Modifier
                .fillMaxSize()
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp),
        ) {
            Surface(
                modifier = Modifier.fillMaxWidth(),
                shape = RoundedCornerShape(18.dp),
                color = Brand.Navy,
                shadowElevation = 2.dp,
            ) {
                Column(Modifier.padding(20.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    Text(
                        "Available credit",
                        style = MaterialTheme.typography.labelLarge,
                        color = Color.White.copy(alpha = 0.72f),
                    )
                    Text(
                        "KES ${"%.0f".format(credit?.optDouble("outstanding", credit?.optDouble("balance", 0.0) ?: 0.0) ?: 0.0)}",
                        style = MaterialTheme.typography.displaySmall,
                        fontWeight = FontWeight.Bold,
                        color = Color.White,
                    )
                    Box(
                        Modifier
                            .size(width = 36.dp, height = 3.dp)
                            .background(Brand.Gold, CircleShape),
                    )
                }
            }
            error?.let { Text(it, color = Brand.Danger) }
            BrandSectionTitle("Ledger")
            val ledger = credit?.optJSONArray("entries") ?: credit?.optJSONArray("ledger")
            if (ledger == null || ledger.length() == 0) {
                Surface(
                    shape = RoundedCornerShape(14.dp),
                    color = Brand.Subtle,
                    modifier = Modifier.fillMaxWidth(),
                ) {
                    Column(Modifier.padding(20.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                        Text("No ledger entries", fontWeight = FontWeight.SemiBold, color = Brand.TextPrimary)
                        Spacer(Modifier.height(4.dp))
                        Text(
                            "Credit movements appear as parts are issued and reconciled.",
                            style = MaterialTheme.typography.bodyMedium,
                            color = Brand.TextMuted,
                            textAlign = TextAlign.Center,
                        )
                    }
                }
            } else {
                for (i in 0 until ledger.length()) {
                    val e = ledger.getJSONObject(i)
                    BrandCard(contentPadding = 14.dp) {
                        Row(
                            Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.SpaceBetween,
                        ) {
                            Column {
                                Text(
                                    e.optString("type").ifBlank { e.optString("entry_type") },
                                    fontWeight = FontWeight.SemiBold,
                                    color = Brand.TextPrimary,
                                )
                                Text(
                                    e.optString("created_at").take(10),
                                    style = MaterialTheme.typography.labelSmall,
                                    color = Brand.TextMuted,
                                )
                            }
                            Text(
                                "KES ${"%.0f".format(e.optDouble("amount", 0.0))}",
                                fontWeight = FontWeight.SemiBold,
                                color = Brand.Navy,
                            )
                        }
                    }
                }
            }
        }
    }
}

@Composable
fun SupplierProfileScreen(
    onSignedOut: () -> Unit = {},
    rootModifier: Modifier = Modifier,
) {
    var me by remember { mutableStateOf<JSONObject?>(null) }
    LaunchedEffect(Unit) {
        me = runCatching { withContext(Dispatchers.IO) { SupplierApi.me() } }.getOrNull()
    }
    val name = me?.optString("display_name")
        ?: SupplierApp.instance.tokenStore.displayName
        ?: "Supplier"
    val avatar = name.split(Regex("\\s+")).take(2).mapNotNull { it.firstOrNull()?.uppercaseChar() }.joinToString("")
        .ifBlank { "S" }
    Column(
        rootModifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background),
    ) {
        BrandHero(
            title = "Profile",
            appLabel = "Supplier",
            trailing = {
                IconButton(onClick = {
                    kotlinx.coroutines.MainScope().launch(Dispatchers.IO) { SupplierApi.logout() }
                    onSignedOut()
                }) {
                    Icon(Icons.AutoMirrored.Filled.Logout, "Sign out", tint = Color.White)
                }
            },
        )
        Column(
            Modifier.padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp),
        ) {
            BrandCard {
                Box(
                    Modifier
                        .size(56.dp)
                        .clip(CircleShape)
                        .background(Brand.Navy),
                    contentAlignment = Alignment.Center,
                ) {
                    Text(
                        avatar,
                        color = Color.White,
                        style = MaterialTheme.typography.titleLarge,
                        fontWeight = FontWeight.Bold,
                    )
                }
                Spacer(Modifier.height(4.dp))
                Text(name, style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.SemiBold, color = Brand.TextPrimary)
                Text(
                    me?.optString("email") ?: "—",
                    color = Brand.TextMuted,
                )
                Text(
                    me?.optJSONObject("supplier")?.optString("name") ?: me?.optString("supplier_name") ?: "",
                    color = Brand.TextSecondary,
                )
                Text(
                    "This account can only access assigned part requests for your supplier.",
                    style = MaterialTheme.typography.bodyMedium,
                    color = Brand.TextMuted,
                )
            }
        }
    }
}

@Composable
private fun ErrorState(message: String, onRetry: (() -> Unit)? = null) {
    Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
        Column(
            horizontalAlignment = Alignment.CenterHorizontally,
            modifier = Modifier.padding(24.dp),
        ) {
            Text(message, color = Brand.Danger, textAlign = TextAlign.Center)
            if (onRetry != null) {
                Spacer(Modifier.height(12.dp))
                GoldButton(text = "Retry", onClick = onRetry)
            }
        }
    }
}

@Composable
private fun EmptyState(title: String, body: String = "") {
    Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
        Surface(
            shape = RoundedCornerShape(16.dp),
            color = Brand.Subtle,
            modifier = Modifier.padding(24.dp).fillMaxWidth(),
        ) {
            Column(
                Modifier.padding(28.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Surface(
                    shape = RoundedCornerShape(14.dp),
                    color = Brand.NavyTint,
                    modifier = Modifier.size(56.dp),
                ) {
                    Box(contentAlignment = Alignment.Center) {
                        Icon(
                            Icons.Default.Inbox,
                            null,
                            modifier = Modifier.size(28.dp),
                            tint = Brand.Navy,
                        )
                    }
                }
                Spacer(Modifier.height(14.dp))
                Text(title, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold, color = Brand.TextPrimary)
                if (body.isNotBlank()) {
                    Spacer(Modifier.height(6.dp))
                    Text(
                        body,
                        style = MaterialTheme.typography.bodyMedium,
                        color = Brand.TextMuted,
                        textAlign = TextAlign.Center,
                    )
                }
            }
        }
    }
}
