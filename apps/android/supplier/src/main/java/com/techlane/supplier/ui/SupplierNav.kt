package com.techlane.supplier.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
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
import androidx.compose.material.icons.filled.Inventory2
import androidx.compose.material.icons.filled.Person
import androidx.compose.material.icons.filled.QrCode2
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.outlined.LocalShipping
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
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
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.techlane.supplier.SupplierApp
import com.techlane.supplier.network.SupplierApi
import com.techlane.core.PrintSupport
import com.techlane.core.qr.QrBitmap
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
    DisposableEffect(onSignedOut) {
        SupplierApi.setSessionExpiredListener {
            android.os.Handler(android.os.Looper.getMainLooper()).post { onSignedOut() }
        }
        onDispose { SupplierApi.setSessionExpiredListener(null) }
    }
    if (!signedIn) {
        SupplierAuthScreen(onSignedIn = onSignedIn, rootModifier = rootModifier)
    } else {
        SupplierShell(onSignedOut = onSignedOut, rootModifier = rootModifier)
    }
}

@Composable
fun SupplierAuthScreen(onSignedIn: () -> Unit, rootModifier: Modifier = Modifier) {
    var mode by remember { mutableStateOf("login") }
    var email by remember { mutableStateOf("supplier@techlane.local") }
    var password by remember { mutableStateOf("password") }
    var inviteToken by remember { mutableStateOf("") }
    var error by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()

    Box(
        rootModifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background),
    ) {
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .height(240.dp)
                .background(MaterialTheme.colorScheme.secondaryContainer.copy(alpha = 0.55f)),
        )
        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(28.dp),
            verticalArrangement = Arrangement.Center,
        ) {
            Icon(
                Icons.Outlined.LocalShipping,
                null,
                tint = MaterialTheme.colorScheme.primary,
                modifier = Modifier.size(40.dp),
            )
            Spacer(Modifier.height(12.dp))
            Text("Supplier portal", style = MaterialTheme.typography.displayMedium)
            Text(
                "Quote requests, issue parts, and track credit.",
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Spacer(Modifier.height(28.dp))
            Surface(
                shape = RoundedCornerShape(20.dp),
                tonalElevation = 3.dp,
                shadowElevation = 4.dp,
            ) {
                Column(
                    Modifier.padding(22.dp),
                    verticalArrangement = Arrangement.spacedBy(14.dp),
                ) {
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        FilterChip(
                            selected = mode == "login",
                            onClick = { mode = "login" },
                            label = { Text("Sign in") },
                        )
                        FilterChip(
                            selected = mode == "invite",
                            onClick = { mode = "invite" },
                            label = { Text("Accept invite") },
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
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(52.dp),
                    ) {
                        Text(if (busy) "Please wait…" else if (mode == "login") "Sign in" else "Activate account")
                    }
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
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        when (tab) {
                            "issues" -> "Issued parts"
                            "credit" -> "Credit"
                            "profile" -> "Profile"
                            else -> "Request queue"
                        },
                    )
                },
                actions = {
                    if (tab == "profile") {
                        IconButton(onClick = {
                            kotlinx.coroutines.MainScope().launch(Dispatchers.IO) { SupplierApi.logout() }
                            onSignedOut()
                        }) {
                            Icon(Icons.AutoMirrored.Filled.Logout, "Sign out")
                        }
                    }
                },
            )
        },
        bottomBar = {
            NavigationBar {
                NavigationBarItem(
                    selected = tab == "queue",
                    onClick = { tab = "queue" },
                    icon = { Icon(Icons.Default.Home, null) },
                    label = { Text("Queue") },
                )
                NavigationBarItem(
                    selected = tab == "issues",
                    onClick = { tab = "issues" },
                    icon = { Icon(Icons.Default.History, null) },
                    label = { Text("Issued") },
                )
                NavigationBarItem(
                    selected = tab == "credit",
                    onClick = { tab = "credit" },
                    icon = { Icon(Icons.Default.AccountBalance, null) },
                    label = { Text("Credit") },
                )
                NavigationBarItem(
                    selected = tab == "profile",
                    onClick = { tab = "profile" },
                    icon = { Icon(Icons.Default.Person, null) },
                    label = { Text("Profile") },
                )
            }
        },
    ) { padding ->
        when (tab) {
            "issues" -> IssuesScreen(onShowQr = { issuedQr = it }, rootModifier = Modifier.padding(padding))
            "credit" -> CreditScreen(rootModifier = Modifier.padding(padding))
            "profile" -> SupplierProfileScreen(rootModifier = Modifier.padding(padding))
            else -> RequestQueueScreen(
                onOpen = { selectedRequestId = it },
                rootModifier = Modifier.padding(padding),
            )
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun RequestQueueScreen(onOpen: (String) -> Unit, rootModifier: Modifier = Modifier) {
    var filter by remember { mutableStateOf<String?>(null) }
    var items by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var loading by remember { mutableStateOf(true) }
    var error by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()
    val chipScroll = rememberScrollState()

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

    Column(rootModifier.fillMaxSize()) {
        Row(
            Modifier
                .horizontalScroll(chipScroll)
                .padding(horizontal = 12.dp, vertical = 8.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            listOf(null to "All", "assigned" to "New", "quoted" to "Quoted", "ready" to "Ready").forEach { (value, label) ->
                FilterChip(
                    selected = filter == value,
                    onClick = { filter = value },
                    label = { Text(label) },
                )
            }
            IconButton(onClick = { refresh() }) {
                Icon(Icons.Default.Refresh, "Refresh")
            }
        }
        when {
            loading -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                CircularProgressIndicator()
            }
            error != null -> ErrorState(error!!, onRetry = { refresh() })
            items.isEmpty() -> EmptyState("Queue is clear", "New part requests assigned to you will show up here.")
            else -> LazyColumn(
                contentPadding = PaddingValues(16.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                items(items, key = { it.optString("id") }) { req ->
                    Card(
                        onClick = { onOpen(req.getString("id")) },
                        shape = RoundedCornerShape(16.dp),
                        elevation = CardDefaults.cardElevation(defaultElevation = 2.dp, pressedElevation = 4.dp),
                        modifier = Modifier.fillMaxWidth(),
                    ) {
                        Column(Modifier.padding(18.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
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
                                    fontWeight = FontWeight.SemiBold,
                                    modifier = Modifier.weight(1f),
                                )
                                Surface(
                                    color = MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.65f),
                                    shape = RoundedCornerShape(999.dp),
                                ) {
                                    Text(
                                        req.optString("status").replace('_', ' '),
                                        style = MaterialTheme.typography.labelMedium,
                                        fontWeight = FontWeight.SemiBold,
                                        color = MaterialTheme.colorScheme.primary,
                                        modifier = Modifier.padding(horizontal = 10.dp, vertical = 5.dp),
                                    )
                                }
                            }
                            Text(
                                "Qty ${req.optInt("quantity", 1)} · ${req.optString("branch_name").ifBlank { "Branch" }}",
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                            req.optString("job_code").takeIf { it.isNotBlank() }?.let {
                                Text("Job $it", style = MaterialTheme.typography.bodySmall)
                            }
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

    Scaffold(
        modifier = rootModifier,
        topBar = {
            TopAppBar(
                title = { Text("Request") },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, "Back")
                    }
                },
            )
        },
    ) { padding ->
        val req = detail
        if (req == null) {
            Box(
                Modifier
                    .fillMaxSize()
                    .padding(padding),
                contentAlignment = Alignment.Center,
            ) {
                if (error != null) Text(error!!, color = MaterialTheme.colorScheme.error)
                else CircularProgressIndicator()
            }
            return@Scaffold
        }
        Column(
            Modifier
                .fillMaxSize()
                .padding(padding)
                .verticalScroll(rememberScrollState())
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp),
        ) {
            Text(
                req.optString("part_name").ifBlank { "Part request" },
                style = MaterialTheme.typography.headlineSmall,
            )
            Text("Status: ${req.optString("status")}")
            Text("Quantity: ${req.optInt("quantity", 1)}")
            req.optString("notes").takeIf { it.isNotBlank() }?.let {
                Text(it, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }

            if (req.optString("status") in listOf("assigned", "open", "pending", "invited")) {
                OutlinedTextField(
                    value = unitCost,
                    onValueChange = { unitCost = it.filter { ch -> ch.isDigit() || ch == '.' } },
                    label = { Text("Unit cost (KES)") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                )
                OutlinedTextField(
                    value = notes,
                    onValueChange = { notes = it },
                    label = { Text("Notes (optional)") },
                    modifier = Modifier.fillMaxWidth(),
                )
                error?.let { Text(it, color = MaterialTheme.colorScheme.error) }
                Button(
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
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(48.dp),
                ) { Text("Submit quote") }
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
                ) { Text("Decline request") }
            }

            if (req.optString("status") in listOf("quote_accepted", "accepted", "quoted", "ready")) {
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
                Button(
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
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(52.dp),
                ) { Text("Issue part + show QR") }
            }
        }
    }
}

@Composable
fun IssueQrScreen(issue: JSONObject, onBack: () -> Unit, rootModifier: Modifier = Modifier) {
    val payload = issue.optString("qr_payload")
        .ifBlank {
            val id = issue.optString("id").ifBlank { issue.optString("issue_id") }
            val code = issue.optString("auth_code")
            "techlane://auth/$id/$code"
        }
    val issueId = issue.optString("id").ifBlank { issue.optString("issue_id") }
    val qrBitmap = remember(payload) { QrBitmap.encode(payload, 640) }
    val scope = rememberCoroutineScope()
    val context = LocalContext.current
    var busy by remember { mutableStateOf(false) }
    Column(
        rootModifier
            .fillMaxSize()
            .padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        IconButton(onClick = onBack, modifier = Modifier.align(Alignment.Start)) {
            Icon(Icons.AutoMirrored.Filled.ArrowBack, "Back")
        }
        Icon(Icons.Default.QrCode2, null, modifier = Modifier.size(48.dp), tint = MaterialTheme.colorScheme.primary)
        Text("Collection QR", style = MaterialTheme.typography.headlineSmall)
        Text(
            "Show this to shop staff to collect the part.",
            textAlign = TextAlign.Center,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Surface(
            shape = RoundedCornerShape(16.dp),
            tonalElevation = 1.dp,
            shadowElevation = 2.dp,
            modifier = Modifier.fillMaxWidth(),
        ) {
            Box(
                Modifier
                    .fillMaxWidth()
                    .border(
                        2.dp,
                        MaterialTheme.colorScheme.primary.copy(alpha = 0.45f),
                        RoundedCornerShape(16.dp),
                    )
                    .background(MaterialTheme.colorScheme.surface)
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
            payload,
            fontFamily = FontFamily.Monospace,
            fontSize = 11.sp,
            textAlign = TextAlign.Center,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Text(
            "AUTH CODE",
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            fontWeight = FontWeight.SemiBold,
        )
        Text(
            issue.optString("auth_code").ifBlank { "—" },
            style = MaterialTheme.typography.headlineSmall,
            fontFamily = FontFamily.Monospace,
            fontWeight = FontWeight.Bold,
            letterSpacing = 2.sp,
        )
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
                ) { Text("Share") }
            }
        }
        Button(onClick = onBack, modifier = Modifier.fillMaxWidth().height(52.dp)) { Text("Done") }
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
    when {
        error != null -> ErrorState(error!!) {}
        items.isEmpty() -> EmptyState("No issued parts yet", "After you issue a part, the collection code appears here.")
        else -> LazyColumn(
            modifier = rootModifier,
            contentPadding = PaddingValues(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            items(items, key = { it.optString("id") }) { issue ->
                Card(
                    onClick = { onShowQr(issue) },
                    shape = RoundedCornerShape(16.dp),
                    elevation = CardDefaults.cardElevation(defaultElevation = 2.dp),
                    modifier = Modifier.fillMaxWidth(),
                ) {
                    Column(Modifier.padding(18.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                        Text(
                            issue.optString("part_name").ifBlank {
                                issue.optString("description").ifBlank { "Issue" }
                            },
                            style = MaterialTheme.typography.titleMedium,
                            fontWeight = FontWeight.SemiBold,
                        )
                        Text(
                            issue.optString("status").replace('_', ' '),
                            style = MaterialTheme.typography.labelLarge,
                            color = MaterialTheme.colorScheme.primary,
                        )
                        Text("KES ${"%.0f".format(issue.optDouble("unit_cost", 0.0))}")
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
            .padding(20.dp),
        verticalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        Card(
            Modifier.fillMaxWidth(),
            shape = RoundedCornerShape(16.dp),
            elevation = CardDefaults.cardElevation(defaultElevation = 2.dp),
            colors = CardDefaults.cardColors(
                containerColor = MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.45f),
            ),
        ) {
            Column(Modifier.padding(20.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Text(
                    "Outstanding credit",
                    style = MaterialTheme.typography.labelLarge,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                Text(
                    "KES ${"%.0f".format(credit?.optDouble("outstanding", credit?.optDouble("balance", 0.0) ?: 0.0) ?: 0.0)}",
                    style = MaterialTheme.typography.displaySmall,
                    fontWeight = FontWeight.Bold,
                )
            }
        }
        error?.let { Text(it, color = MaterialTheme.colorScheme.error) }
        Text("Ledger", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold)
        val ledger = credit?.optJSONArray("entries") ?: credit?.optJSONArray("ledger")
        if (ledger == null || ledger.length() == 0) {
            Surface(
                shape = RoundedCornerShape(14.dp),
                color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.45f),
                modifier = Modifier.fillMaxWidth(),
            ) {
                Column(Modifier.padding(20.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                    Text("No ledger entries", fontWeight = FontWeight.SemiBold)
                    Spacer(Modifier.height(4.dp))
                    Text(
                        "Credit movements appear as parts are issued and reconciled.",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        textAlign = TextAlign.Center,
                    )
                }
            }
        } else {
            for (i in 0 until ledger.length()) {
                val e = ledger.getJSONObject(i)
                Card(
                    modifier = Modifier.fillMaxWidth(),
                    shape = RoundedCornerShape(14.dp),
                    elevation = CardDefaults.cardElevation(defaultElevation = 1.dp),
                ) {
                    Row(
                        Modifier
                            .fillMaxWidth()
                            .padding(14.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                    ) {
                        Column {
                            Text(
                                e.optString("type").ifBlank { e.optString("entry_type") },
                                fontWeight = FontWeight.SemiBold,
                            )
                            Text(
                                e.optString("created_at").take(10),
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                        }
                        Text(
                            "KES ${"%.0f".format(e.optDouble("amount", 0.0))}",
                            fontWeight = FontWeight.SemiBold,
                        )
                    }
                }
            }
        }
    }
}

@Composable
fun SupplierProfileScreen(rootModifier: Modifier = Modifier) {
    var me by remember { mutableStateOf<JSONObject?>(null) }
    LaunchedEffect(Unit) {
        me = runCatching { withContext(Dispatchers.IO) { SupplierApi.me() } }.getOrNull()
    }
    val name = me?.optString("display_name")
        ?: SupplierApp.instance.tokenStore.displayName
        ?: "Supplier"
    val avatar = name.split(Regex("\\s+")).take(2).mapNotNull { it.firstOrNull()?.uppercaseChar() }.joinToString("")
        .ifBlank { "S" }
    Column(rootModifier.padding(20.dp), verticalArrangement = Arrangement.spacedBy(14.dp)) {
        Card(
            Modifier.fillMaxWidth(),
            shape = RoundedCornerShape(16.dp),
            elevation = CardDefaults.cardElevation(defaultElevation = 2.dp),
        ) {
            Column(Modifier.padding(20.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                Box(
                    Modifier
                        .size(56.dp)
                        .clip(CircleShape)
                        .background(MaterialTheme.colorScheme.primary),
                    contentAlignment = Alignment.Center,
                ) {
                    Text(
                        avatar,
                        color = MaterialTheme.colorScheme.onPrimary,
                        style = MaterialTheme.typography.titleLarge,
                        fontWeight = FontWeight.Bold,
                    )
                }
                Text(name, style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.SemiBold)
                Text(
                    me?.optString("email") ?: "—",
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                Text(me?.optJSONObject("supplier")?.optString("name") ?: me?.optString("supplier_name") ?: "")
                Text(
                    "This account can only access assigned part requests for your supplier.",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
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
            Text(message, color = MaterialTheme.colorScheme.error, textAlign = TextAlign.Center)
            if (onRetry != null) {
                Spacer(Modifier.height(12.dp))
                Button(onClick = onRetry) { Text("Retry") }
            }
        }
    }
}

@Composable
private fun EmptyState(title: String, body: String = "") {
    Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
        Surface(
            shape = RoundedCornerShape(16.dp),
            color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.45f),
            modifier = Modifier.padding(24.dp).fillMaxWidth(),
        ) {
            Column(
                Modifier.padding(28.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Surface(
                    shape = RoundedCornerShape(14.dp),
                    color = MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.7f),
                    modifier = Modifier.size(56.dp),
                ) {
                    Box(contentAlignment = Alignment.Center) {
                        Icon(
                            Icons.Default.Inbox,
                            null,
                            modifier = Modifier.size(28.dp),
                            tint = MaterialTheme.colorScheme.primary,
                        )
                    }
                }
                Spacer(Modifier.height(14.dp))
                Text(title, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold)
                if (body.isNotBlank()) {
                    Spacer(Modifier.height(6.dp))
                    Text(
                        body,
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        textAlign = TextAlign.Center,
                    )
                }
            }
        }
    }
}
