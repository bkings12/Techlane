package com.techlane.customer.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
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
import androidx.compose.material.icons.filled.Build
import androidx.compose.material.icons.filled.CheckCircle
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.Person
import androidx.compose.material.icons.filled.PhoneAndroid
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.outlined.Handyman
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
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
import androidx.compose.material3.TopAppBarDefaults
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
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.techlane.core.theme.statusPalette
import com.techlane.customer.CustomerApp
import com.techlane.customer.network.CustomerApi
import com.techlane.core.PrintSupport
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONObject

@Composable
private fun statusColors(status: String): Pair<Color, Color> {
    val palette = statusPalette()
    val fg = when (status.lowercase()) {
        "intake" -> palette.intake
        "diagnosed" -> palette.diagnosed
        "waiting_parts" -> palette.waitingParts
        "in_progress" -> palette.inProgress
        "completed", "ready" -> palette.completed
        "collected" -> palette.collected
        else -> MaterialTheme.colorScheme.primary
    }
    return fg.copy(alpha = 0.16f) to fg
}

@Composable
fun CustomerNav(
    signedIn: Boolean,
    onSignedIn: () -> Unit,
    onSignedOut: () -> Unit,
    rootModifier: Modifier = Modifier,
) {
    DisposableEffect(onSignedOut) {
        CustomerApi.setSessionExpiredListener {
            android.os.Handler(android.os.Looper.getMainLooper()).post { onSignedOut() }
        }
        onDispose { CustomerApi.setSessionExpiredListener(null) }
    }
    if (!signedIn) {
        OtpAuthScreen(onSignedIn = onSignedIn, rootModifier = rootModifier)
    } else {
        CustomerShell(onSignedOut = onSignedOut, rootModifier = rootModifier)
    }
}

@Composable
fun OtpAuthScreen(onSignedIn: () -> Unit, rootModifier: Modifier = Modifier) {
    var phone by remember { mutableStateOf(CustomerApp.instance.tokenStore.phone.orEmpty()) }
    var code by remember { mutableStateOf("") }
    var step by remember { mutableStateOf("phone") }
    var error by remember { mutableStateOf<String?>(null) }
    var hint by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()

    Box(
        rootModifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background),
    ) {
        Box(
            Modifier
                .fillMaxWidth()
                .height(260.dp)
                .background(MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.6f)),
        )
        Column(
            Modifier
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
            Text("Track your repair", style = MaterialTheme.typography.displayMedium)
            Spacer(Modifier.height(8.dp))
            Text(
                "Sign in with the phone number you left at the shop.",
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Spacer(Modifier.height(32.dp))
            Surface(
                shape = RoundedCornerShape(20.dp),
                color = MaterialTheme.colorScheme.surface,
                tonalElevation = 3.dp,
                shadowElevation = 4.dp,
            ) {
                Column(
                    Modifier.padding(22.dp),
                    verticalArrangement = Arrangement.spacedBy(14.dp),
                ) {
                    OutlinedTextField(
                        value = phone,
                        onValueChange = { phone = it },
                        label = { Text("Phone number") },
                        leadingIcon = { Icon(Icons.Default.PhoneAndroid, null) },
                        singleLine = true,
                        enabled = step == "phone",
                        modifier = Modifier.fillMaxWidth(),
                    )
                    if (step == "code") {
                        OutlinedTextField(
                            value = code,
                            onValueChange = { code = it.filter { ch -> ch.isDigit() }.take(6) },
                            label = { Text("6-digit code") },
                            singleLine = true,
                            modifier = Modifier.fillMaxWidth(),
                        )
                        hint?.let {
                            Text(
                                it,
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.primary,
                            )
                        }
                    }
                    error?.let {
                        Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
                    }
                    Button(
                        onClick = {
                            busy = true
                            error = null
                            scope.launch {
                                try {
                                    if (step == "phone") {
                                        val res = withContext(Dispatchers.IO) { CustomerApi.requestOtp(phone.trim()) }
                                        hint = res.optString("dev_hint").takeIf { it.isNotBlank() }
                                            ?: "We sent a code by SMS."
                                        CustomerApp.instance.tokenStore.phone = phone.trim()
                                        step = "code"
                                    } else {
                                        val res = withContext(Dispatchers.IO) {
                                            CustomerApi.verifyOtp(phone.trim(), code.trim())
                                        }
                                        CustomerApp.instance.tokenStore.sessionToken = res.getString("token")
                                        CustomerApp.instance.tokenStore.phone = phone.trim()
                                        res.optJSONObject("customer")?.optString("name")
                                            ?.takeIf { it.isNotBlank() }
                                            ?.let { CustomerApp.instance.tokenStore.displayName = it }
                                        onSignedIn()
                                    }
                                } catch (e: Exception) {
                                    error = e.message
                                } finally {
                                    busy = false
                                }
                            }
                        },
                        enabled = !busy && phone.isNotBlank() && (step == "phone" || code.length >= 4),
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(52.dp),
                    ) {
                        Text(
                            when {
                                busy && step == "phone" -> "Sending…"
                                busy -> "Verifying…"
                                step == "phone" -> "Send code"
                                else -> "Continue"
                            },
                        )
                    }
                    if (step == "code") {
                        Text(
                            "Use a different number",
                            color = MaterialTheme.colorScheme.primary,
                            style = MaterialTheme.typography.labelLarge,
                            modifier = Modifier
                                .fillMaxWidth()
                                .clickable {
                                    step = "phone"
                                    code = ""
                                    hint = null
                                }
                                .padding(vertical = 8.dp),
                            textAlign = TextAlign.Center,
                        )
                    }
                }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun CustomerShell(onSignedOut: () -> Unit, rootModifier: Modifier = Modifier) {
    var tab by remember { mutableStateOf("home") }
    var selectedRepairId by remember { mutableStateOf<String?>(null) }

    if (selectedRepairId != null) {
        RepairDetailScreen(
            repairId = selectedRepairId!!,
            onBack = { selectedRepairId = null },
            rootModifier = rootModifier,
        )
        return
    }

    Scaffold(
        modifier = rootModifier,
        topBar = {
            TopAppBar(
                title = {
                    Text(if (tab == "profile") "Profile" else "My repairs")
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.surface,
                ),
                actions = {
                    if (tab == "profile") {
                        IconButton(onClick = {
                            kotlinx.coroutines.MainScope().launch(Dispatchers.IO) {
                                CustomerApi.logout()
                            }
                            onSignedOut()
                        }) {
                            Icon(Icons.AutoMirrored.Filled.Logout, contentDescription = "Sign out")
                        }
                    }
                },
            )
        },
        bottomBar = {
            NavigationBar {
                NavigationBarItem(
                    selected = tab == "home",
                    onClick = { tab = "home" },
                    icon = { Icon(Icons.Default.Home, null) },
                    label = { Text("Repairs") },
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
        val contentMod = Modifier.padding(padding)
        if (tab == "profile") {
            ProfileScreen(contentMod)
        } else {
            RepairListScreen(onOpen = { selectedRepairId = it }, rootModifier = contentMod)
        }
    }
}

@Composable
fun RepairListScreen(onOpen: (String) -> Unit, rootModifier: Modifier = Modifier) {
    var items by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var loading by remember { mutableStateOf(true) }
    var error by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()

    fun refresh() {
        scope.launch {
            loading = true
            error = null
            try {
                val arr = withContext(Dispatchers.IO) { CustomerApi.listRepairs() }
                items = (0 until arr.length()).map { arr.getJSONObject(it) }
            } catch (e: Exception) {
                error = e.message
            } finally {
                loading = false
            }
        }
    }

    LaunchedEffect(Unit) { refresh() }

    Column(rootModifier.fillMaxSize()) {
        Row(
            Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 8.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                "Approve estimates and pay when ready.",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.weight(1f),
            )
            IconButton(onClick = { refresh() }) {
                Icon(Icons.Default.Refresh, contentDescription = "Refresh")
            }
        }
        when {
            loading -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                CircularProgressIndicator()
            }
            error != null -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    Text(error!!, color = MaterialTheme.colorScheme.error, textAlign = TextAlign.Center)
                    Spacer(Modifier.height(12.dp))
                    Button(onClick = { refresh() }) { Text("Retry") }
                }
            }
            items.isEmpty() -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
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
                                    Icons.Default.Build,
                                    null,
                                    modifier = Modifier.size(28.dp),
                                    tint = MaterialTheme.colorScheme.primary,
                                )
                            }
                        }
                        Spacer(Modifier.height(14.dp))
                        Text("No repairs yet", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold)
                        Spacer(Modifier.height(6.dp))
                        Text(
                            "When you drop a device at the shop, it will show here.",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            textAlign = TextAlign.Center,
                        )
                    }
                }
            }
            else -> LazyColumn(
                contentPadding = PaddingValues(16.dp),
                verticalArrangement = Arrangement.spacedBy(14.dp),
            ) {
                items(items, key = { it.optString("id") }) { job ->
                    RepairCard(job) { onOpen(job.getString("id")) }
                }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun RepairCard(job: JSONObject, onClick: () -> Unit) {
    val status = job.optString("status").ifBlank { "open" }
    val colors = statusColors(status)
    Card(
        onClick = onClick,
        shape = RoundedCornerShape(16.dp),
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
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
                    job.optString("job_code").ifBlank { "Repair" },
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.SemiBold,
                )
                StatusPill(status, colors.first, colors.second)
            }
            Text(
                listOfNotNull(
                    job.optString("device_brand").takeIf { it.isNotBlank() },
                    job.optString("device_model").takeIf { it.isNotBlank() },
                ).joinToString(" ").ifBlank { "Device under repair" },
                style = MaterialTheme.typography.bodyLarge,
            )
            job.optJSONObject("estimate")?.let { est ->
                if (est.optString("status") == "pending") {
                    Text(
                        "Estimate awaiting your approval",
                        color = MaterialTheme.colorScheme.primary,
                        style = MaterialTheme.typography.labelLarge,
                    )
                }
            }
            if (job.optBoolean("balance_due") || job.optDouble("balance_due", 0.0) > 0) {
                Text(
                    "Payment due",
                    color = MaterialTheme.colorScheme.tertiary,
                    style = MaterialTheme.typography.labelLarge,
                )
            }
        }
    }
}

@Composable
private fun StatusPill(label: String, bg: Color, fg: Color) {
    Surface(color = bg, shape = RoundedCornerShape(999.dp)) {
        Text(
            label.replace('_', ' '),
            color = fg,
            style = MaterialTheme.typography.labelMedium,
            fontWeight = FontWeight.SemiBold,
            modifier = Modifier.padding(horizontal = 10.dp, vertical = 5.dp),
        )
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun RepairDetailScreen(repairId: String, onBack: () -> Unit, rootModifier: Modifier = Modifier) {
    var detail by remember { mutableStateOf<JSONObject?>(null) }
    var loading by remember { mutableStateOf(true) }
    var error by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    var payMessage by remember { mutableStateOf<String?>(null) }
    var paymentId by remember { mutableStateOf<String?>(null) }
    var warranty by remember { mutableStateOf<JSONObject?>(null) }
    var claimNote by remember { mutableStateOf("") }
    val scope = rememberCoroutineScope()
    val context = LocalContext.current

    fun refresh() {
        scope.launch {
            loading = true
            error = null
            try {
                detail = withContext(Dispatchers.IO) { CustomerApi.repairDetail(repairId) }
                warranty = withContext(Dispatchers.IO) {
                    runCatching { CustomerApi.warranty(repairId) }.getOrNull()
                }
            } catch (e: Exception) {
                error = e.message
            } finally {
                loading = false
            }
        }
    }

    LaunchedEffect(repairId) { refresh() }

    LaunchedEffect(paymentId) {
        val id = paymentId ?: return@LaunchedEffect
        repeat(12) {
            delay(2500)
            try {
                val st = withContext(Dispatchers.IO) { CustomerApi.paymentStatus(repairId, id) }
                val status = st.optString("status")
                payMessage = "Payment: $status"
                if (status in listOf("confirmed", "failed", "cancelled")) {
                    refresh()
                    return@LaunchedEffect
                }
            } catch (_: Exception) {
            }
        }
    }

    Scaffold(
        modifier = rootModifier,
        topBar = {
            TopAppBar(
                title = { Text(detail?.optString("job_code") ?: "Repair") },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                },
                actions = {
                    IconButton(onClick = { refresh() }) {
                        Icon(Icons.Default.Refresh, contentDescription = "Refresh")
                    }
                },
            )
        },
    ) { padding ->
        when {
            loading && detail == null -> Box(
                Modifier.fillMaxSize().padding(padding),
                contentAlignment = Alignment.Center,
            ) { CircularProgressIndicator() }
            error != null && detail == null -> Box(
                Modifier.fillMaxSize().padding(padding),
                contentAlignment = Alignment.Center,
            ) {
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    Text(error!!, color = MaterialTheme.colorScheme.error)
                    Button(onClick = { refresh() }) { Text("Retry") }
                }
            }
            else -> {
                val job = detail!!
                val estimate = job.optJSONObject("estimate") ?: job.optJSONObject("pending_estimate")
                val timeline = job.optJSONArray("timeline") ?: job.optJSONArray("events")
                val statusPair = statusColors(job.optString("status"))
                Column(
                    Modifier
                        .fillMaxSize()
                        .padding(padding)
                        .verticalScroll(rememberScrollState())
                        .padding(16.dp),
                    verticalArrangement = Arrangement.spacedBy(16.dp),
                ) {
                    Card(
                        Modifier.fillMaxWidth(),
                        shape = RoundedCornerShape(16.dp),
                        elevation = CardDefaults.cardElevation(defaultElevation = 2.dp),
                    ) {
                        Column(
                            Modifier.padding(18.dp),
                            verticalArrangement = Arrangement.spacedBy(8.dp),
                        ) {
                            Text(
                                listOfNotNull(
                                    job.optString("device_brand").takeIf { it.isNotBlank() },
                                    job.optString("device_model").takeIf { it.isNotBlank() },
                                ).joinToString(" ").ifBlank { "Device" },
                                style = MaterialTheme.typography.headlineSmall,
                            )
                            StatusPill(job.optString("status"), statusPair.first, statusPair.second)
                            job.optString("problem_summary").takeIf { it.isNotBlank() }?.let {
                                Text(
                                    it,
                                    style = MaterialTheme.typography.bodyMedium,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                )
                            }
                            job.optString("customer_name").takeIf { it.isNotBlank() }?.let {
                                Text(it, style = MaterialTheme.typography.bodyMedium)
                            }
                        }
                    }

                    if (estimate != null && estimate.optString("status") == "pending") {
                        Card(
                            Modifier.fillMaxWidth(),
                            shape = RoundedCornerShape(16.dp),
                            elevation = CardDefaults.cardElevation(defaultElevation = 2.dp),
                            colors = CardDefaults.cardColors(
                                containerColor = MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.55f),
                            ),
                        ) {
                            Column(
                                Modifier.padding(18.dp),
                                verticalArrangement = Arrangement.spacedBy(10.dp),
                            ) {
                                Text(
                                    "Estimate for approval",
                                    style = MaterialTheme.typography.titleMedium,
                                    fontWeight = FontWeight.SemiBold,
                                )
                                val labor = estimate.optDouble("labor_amount", 0.0)
                                val partsAmt = estimate.optDouble("parts_amount", 0.0)
                                val currency = estimate.optString("currency", "KES")
                                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                                    Text("Labor", color = MaterialTheme.colorScheme.onSurfaceVariant)
                                    Text("$currency ${"%.0f".format(labor)}")
                                }
                                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                                    Text("Parts", color = MaterialTheme.colorScheme.onSurfaceVariant)
                                    Text("$currency ${"%.0f".format(partsAmt)}")
                                }
                                Text(
                                    "$currency ${"%.0f".format(labor + partsAmt)}",
                                    style = MaterialTheme.typography.headlineSmall,
                                    fontWeight = FontWeight.Bold,
                                )
                                estimate.optString("notes").takeIf { it.isNotBlank() }?.let {
                                    Text(it, style = MaterialTheme.typography.bodyMedium)
                                }
                                error?.let { Text(it, color = MaterialTheme.colorScheme.error) }
                                Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                                    Button(
                                        onClick = {
                                            busy = true
                                            scope.launch {
                                                try {
                                                    withContext(Dispatchers.IO) {
                                                        CustomerApi.approveEstimate(repairId, estimate.getString("id"))
                                                    }
                                                    refresh()
                                                } catch (e: Exception) {
                                                    error = e.message
                                                } finally {
                                                    busy = false
                                                }
                                            }
                                        },
                                        enabled = !busy,
                                        modifier = Modifier.weight(1f).height(48.dp),
                                    ) { Text("Approve") }
                                    OutlinedButton(
                                        onClick = {
                                            busy = true
                                            scope.launch {
                                                try {
                                                    withContext(Dispatchers.IO) {
                                                        CustomerApi.rejectEstimate(repairId, estimate.getString("id"))
                                                    }
                                                    refresh()
                                                } catch (e: Exception) {
                                                    error = e.message
                                                } finally {
                                                    busy = false
                                                }
                                            }
                                        },
                                        enabled = !busy,
                                        modifier = Modifier.weight(1f).height(48.dp),
                                    ) { Text("Reject") }
                                }
                            }
                        }
                    }

                    val balance = job.optDouble("balance_due", job.optDouble("amount_due", 0.0))
                    if (balance > 0) {
                        Card(
                            Modifier.fillMaxWidth(),
                            shape = RoundedCornerShape(16.dp),
                            elevation = CardDefaults.cardElevation(defaultElevation = 2.dp),
                        ) {
                            Column(
                                Modifier.padding(18.dp),
                                verticalArrangement = Arrangement.spacedBy(10.dp),
                            ) {
                                Text(
                                    "Pay balance",
                                    style = MaterialTheme.typography.titleMedium,
                                    fontWeight = FontWeight.SemiBold,
                                )
                                Text(
                                    "KES ${"%.0f".format(balance)}",
                                    style = MaterialTheme.typography.headlineMedium,
                                    fontWeight = FontWeight.Bold,
                                )
                                payMessage?.let { Text(it, color = MaterialTheme.colorScheme.primary) }
                                Button(
                                    onClick = {
                                        busy = true
                                        payMessage = null
                                        scope.launch {
                                            try {
                                                val res = withContext(Dispatchers.IO) {
                                                    CustomerApi.payRepair(
                                                        repairId,
                                                        CustomerApp.instance.tokenStore.phone,
                                                    )
                                                }
                                                paymentId = res.optString("payment_id").takeIf { it.isNotBlank() }
                                                    ?: res.optString("id").takeIf { it.isNotBlank() }
                                                payMessage = res.optString("message")
                                                    .ifBlank { "STK push sent — check your phone." }
                                            } catch (e: Exception) {
                                                payMessage = e.message
                                            } finally {
                                                busy = false
                                            }
                                        }
                                    },
                                    enabled = !busy,
                                    modifier = Modifier.fillMaxWidth().height(52.dp),
                                ) {
                                    Text(if (busy) "Sending STK…" else "Pay with M-Pesa")
                                }
                            }
                        }
                    }

                    Text("Timeline", style = MaterialTheme.typography.titleMedium)
                    if (timeline == null || timeline.length() == 0) {
                        Text("No updates yet.", color = MaterialTheme.colorScheme.onSurfaceVariant)
                    } else {
                        for (i in 0 until timeline.length()) {
                            val ev = timeline.getJSONObject(i)
                            Row(
                                Modifier.fillMaxWidth().padding(vertical = 6.dp),
                                horizontalArrangement = Arrangement.spacedBy(12.dp),
                            ) {
                                Icon(
                                    Icons.Default.CheckCircle,
                                    null,
                                    tint = MaterialTheme.colorScheme.primary,
                                    modifier = Modifier.size(20.dp),
                                )
                                Column {
                                    Text(
                                        ev.optString("status").ifBlank { ev.optString("event_type") },
                                        style = MaterialTheme.typography.titleSmall,
                                    )
                                    ev.optString("note").takeIf { it.isNotBlank() }?.let {
                                        Text(it, style = MaterialTheme.typography.bodySmall)
                                    }
                                    Text(
                                        ev.optString("created_at").take(16),
                                        style = MaterialTheme.typography.labelSmall,
                                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                                    )
                                }
                            }
                        }
                    }

                    val receipts = job.optJSONArray("receipts")
                    Text("Receipts", style = MaterialTheme.typography.titleMedium)
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        OutlinedButton(
                            onClick = {
                                busy = true
                                scope.launch {
                                    try {
                                        val html = withContext(Dispatchers.IO) {
                                            PrintSupport.fetchText(
                                                CustomerApi.receiptHtmlUrl(repairId),
                                                CustomerApp.instance.tokenStore.sessionToken,
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
                        ) { Text("Print") }
                        OutlinedButton(
                            onClick = {
                                busy = true
                                scope.launch {
                                    try {
                                        val html = withContext(Dispatchers.IO) {
                                            PrintSupport.fetchText(
                                                CustomerApi.receiptHtmlUrl(repairId),
                                                CustomerApp.instance.tokenStore.sessionToken,
                                            )
                                        }
                                        PrintSupport.shareText(context, html, "Share receipt")
                                    } catch (e: Exception) {
                                        error = e.message
                                    } finally {
                                        busy = false
                                    }
                                }
                            },
                            enabled = !busy,
                        ) { Text("Share") }
                    }
                    if (receipts != null && receipts.length() > 0) {
                        for (i in 0 until receipts.length()) {
                            val r = receipts.getJSONObject(i)
                            Card(Modifier.fillMaxWidth()) {
                                Column(Modifier.padding(12.dp)) {
                                    Text(
                                        "KES ${"%.2f".format(r.optDouble("amount", 0.0))}",
                                        style = MaterialTheme.typography.titleMedium,
                                    )
                                    Text(
                                        "${r.optString("method")} · ${r.optString("status")}",
                                        style = MaterialTheme.typography.bodySmall,
                                    )
                                }
                            }
                        }
                    } else {
                        Text(
                            "Receipts appear after payment.",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }

                    Text("Warranty", style = MaterialTheme.typography.titleMedium)
                    if (warranty != null) {
                        val w = warranty!!
                        Card(Modifier.fillMaxWidth()) {
                            Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                                Text("${w.optString("status")} · ${w.optInt("duration_days")} days")
                                Text("Ends ${w.optString("ends_at").take(10)}")
                                if (w.optString("status") == "active") {
                                    OutlinedTextField(
                                        value = claimNote,
                                        onValueChange = { claimNote = it },
                                        label = { Text("Claim note") },
                                        modifier = Modifier.fillMaxWidth(),
                                    )
                                    Button(
                                        onClick = {
                                            busy = true
                                            scope.launch {
                                                try {
                                                    warranty = withContext(Dispatchers.IO) {
                                                        CustomerApi.claimWarranty(repairId, claimNote)
                                                    }
                                                    claimNote = ""
                                                } catch (e: Exception) {
                                                    error = e.message
                                                } finally {
                                                    busy = false
                                                }
                                            }
                                        },
                                        enabled = !busy && claimNote.isNotBlank(),
                                    ) { Text("Claim warranty") }
                                }
                            }
                        }
                    } else {
                        Text(
                            "Warranty is issued when the repair is completed.",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                }
            }
        }
    }
}

@Composable
fun ProfileScreen(rootModifier: Modifier = Modifier) {
    var me by remember { mutableStateOf<JSONObject?>(null) }
    var error by remember { mutableStateOf<String?>(null) }

    LaunchedEffect(Unit) {
        try {
            me = withContext(Dispatchers.IO) { CustomerApi.me() }
        } catch (e: Exception) {
            error = e.message
        }
    }

    val name = me?.optString("name")?.takeIf { it.isNotBlank() }
        ?: CustomerApp.instance.tokenStore.displayName
        ?: "Customer"
    val phone = me?.optString("phone")
        ?: CustomerApp.instance.tokenStore.phone
        ?: "—"
    val avatar = name.split(Regex("\\s+")).take(2).mapNotNull { it.firstOrNull()?.uppercaseChar() }.joinToString("")
        .ifBlank { phone.takeLast(2) }

    Column(
        rootModifier.fillMaxSize().padding(20.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Card(
            Modifier.fillMaxWidth(),
            shape = RoundedCornerShape(16.dp),
            elevation = CardDefaults.cardElevation(defaultElevation = 2.dp),
        ) {
            Column(
                Modifier.padding(20.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp),
            ) {
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
                Text(phone, style = MaterialTheme.typography.titleMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                error?.let { Text(it, color = MaterialTheme.colorScheme.error) }
                Text(
                    "Sessions expire for your security. You can request a new code anytime.",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }
    }
}
