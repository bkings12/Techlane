package com.techlane.customer.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
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
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
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
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.Person
import androidx.compose.material.icons.filled.PhoneAndroid
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
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
import com.techlane.core.PrintSupport
import com.techlane.core.theme.Brand
import com.techlane.core.theme.statusPalette
import com.techlane.core.ui.BrandAuthHeader
import com.techlane.core.ui.BrandCard
import com.techlane.core.ui.BrandDetailHeader
import com.techlane.core.ui.BrandHero
import com.techlane.core.ui.BrandSectionTitle
import com.techlane.core.ui.GoldButton
import com.techlane.core.ui.HeroStat
import com.techlane.core.ui.PillBadge
import com.techlane.core.ui.SafeBottomBar
import com.techlane.core.ui.brandGradient
import com.techlane.customer.CustomerApp
import com.techlane.customer.network.CustomerApi
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONObject

@Composable
private fun statusColor(status: String): Color {
    val palette = statusPalette()
    return when (status.lowercase()) {
        "intake" -> palette.intake
        "diagnosed" -> palette.diagnosed
        "waiting_parts" -> palette.waitingParts
        "in_progress" -> palette.inProgress
        "ready_for_pickup", "completed", "ready" -> palette.completed
        "collected" -> palette.collected
        else -> Brand.Navy
    }
}

@Composable
fun CustomerNav(
    signedIn: Boolean,
    onSignedIn: () -> Unit,
    onSignedOut: () -> Unit,
    rootModifier: Modifier = Modifier,
) {
    var sessionExpired by remember { mutableStateOf(false) }
    DisposableEffect(onSignedOut) {
        CustomerApi.setSessionExpiredListener {
            android.os.Handler(android.os.Looper.getMainLooper()).post {
                sessionExpired = true
                onSignedOut()
            }
        }
        onDispose { CustomerApi.setSessionExpiredListener(null) }
    }
    if (!signedIn) {
        OtpAuthScreen(
            onSignedIn = {
                sessionExpired = false
                onSignedIn()
            },
            sessionExpired = sessionExpired,
            rootModifier = rootModifier,
        )
    } else {
        CustomerShell(onSignedOut = onSignedOut, rootModifier = rootModifier)
    }
}

@Composable
fun OtpAuthScreen(onSignedIn: () -> Unit, sessionExpired: Boolean = false, rootModifier: Modifier = Modifier) {
    var phone by remember { mutableStateOf(CustomerApp.instance.tokenStore.phone.orEmpty()) }
    var code by remember { mutableStateOf("") }
    var step by remember { mutableStateOf("phone") }
    var error by remember { mutableStateOf<String?>(null) }
    var hint by remember { mutableStateOf<String?>(if (sessionExpired) "Your session expired. Please sign in again." else null) }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()

    Box(
        rootModifier
            .fillMaxSize()
            .background(brandGradient()),
    ) {
        BrandAuthHeader(
            appLabel = "Customer",
            tagline = "Track your repairs. Pay. Collect.",
            modifier = Modifier
                .align(Alignment.TopCenter)
                .statusBarsPadding()
                .padding(horizontal = 28.dp, vertical = 36.dp),
        )
        Surface(
            modifier = Modifier
                .align(Alignment.BottomCenter)
                .fillMaxWidth(),
            shape = RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp),
            color = Brand.Surface,
            shadowElevation = 8.dp,
        ) {
            Column(
                Modifier
                    .verticalScroll(rememberScrollState())
                    .padding(horizontal = 24.dp, vertical = 28.dp),
                verticalArrangement = Arrangement.spacedBy(14.dp),
            ) {
                Text(
                    if (step == "phone") "Sign in" else "Enter code",
                    style = MaterialTheme.typography.titleLarge,
                    fontWeight = FontWeight.Bold,
                    color = Brand.TextPrimary,
                )
                Text(
                    if (step == "phone") {
                        "Use the phone number you left at the shop."
                    } else {
                        "We sent a 6-digit code to $phone"
                    },
                    style = MaterialTheme.typography.bodyMedium,
                    color = Brand.TextSecondary,
                )
                if (step == "phone") {
                    hint?.let {
                        Text(it, style = MaterialTheme.typography.bodySmall, color = Brand.Warning)
                    }
                }
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
                            color = Brand.Navy,
                        )
                    }
                }
                error?.let {
                    Text(it, color = Brand.Danger, style = MaterialTheme.typography.bodySmall)
                }
                GoldButton(
                    text = if (step == "phone") "Send code" else "Verify",
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
                    loading = busy,
                    modifier = Modifier.fillMaxWidth(),
                )
                if (step == "code") {
                    Text(
                        "Use a different number",
                        color = Brand.Navy,
                        style = MaterialTheme.typography.labelLarge,
                        fontWeight = FontWeight.SemiBold,
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

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun CustomerShell(onSignedOut: () -> Unit, rootModifier: Modifier = Modifier) {
    var tab by remember { mutableStateOf("home") }
    var selectedRepairId by remember { mutableStateOf<String?>(null) }

    fun signOut() {
        kotlinx.coroutines.MainScope().launch(Dispatchers.IO) {
            CustomerApi.logout()
        }
        onSignedOut()
    }

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
        containerColor = MaterialTheme.colorScheme.background,
        contentWindowInsets = WindowInsets(0, 0, 0, 0),
        bottomBar = {
            SafeBottomBar(containerColor = Brand.Surface) {
                NavigationBar(
                    containerColor = Brand.Surface,
                    windowInsets = WindowInsets(0, 0, 0, 0),
                ) {
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
            }
        },
    ) { padding ->
        val contentMod = Modifier.padding(bottom = padding.calculateBottomPadding())
        if (tab == "profile") {
            Column(contentMod.fillMaxSize()) {
                BrandHero(
                    title = "Profile",
                    subtitle = "Your account details",
                    appLabel = "Customer",
                    trailing = {
                        IconButton(onClick = { signOut() }) {
                            Icon(
                                Icons.AutoMirrored.Filled.Logout,
                                contentDescription = "Sign out",
                                tint = Color.White,
                            )
                        }
                    },
                )
                ProfileScreen(Modifier.weight(1f))
            }
        } else {
            RepairListScreen(
                onOpen = { selectedRepairId = it },
                onSignedOut = { signOut() },
                rootModifier = contentMod,
            )
        }
    }
}

@Composable
fun RepairListScreen(
    onOpen: (String) -> Unit,
    rootModifier: Modifier = Modifier,
    onSignedOut: () -> Unit = {},
) {
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

    val inProgressCount = items.count {
        it.optString("status").lowercase() in setOf(
            "intake", "diagnosed", "waiting_parts", "in_progress", "open",
        )
    }
    val readyCount = items.count {
        it.optString("status").lowercase() in setOf("ready_for_pickup", "completed", "ready")
    }
    val welcome = CustomerApp.instance.tokenStore.displayName
        ?.takeIf { it.isNotBlank() }
        ?.let { "Welcome, $it" }
        ?: "Approve estimates and pay when ready."

    Column(rootModifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        BrandHero(
            title = "My repairs",
            subtitle = welcome,
            appLabel = "Customer",
            trailing = {
                Row {
                    IconButton(onClick = { refresh() }) {
                        Icon(Icons.Default.Refresh, contentDescription = "Refresh", tint = Color.White)
                    }
                    IconButton(onClick = onSignedOut) {
                        Icon(
                            Icons.AutoMirrored.Filled.Logout,
                            contentDescription = "Sign out",
                            tint = Color.White,
                        )
                    }
                }
            },
            bottomContent = if (!loading && items.isNotEmpty()) {
                {
                    Row(
                        Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(10.dp),
                    ) {
                        HeroStat("In progress", "$inProgressCount", Modifier.weight(1f))
                        HeroStat("Ready", "$readyCount", Modifier.weight(1f))
                        HeroStat("Total", "${items.size}", Modifier.weight(1f))
                    }
                }
            } else {
                null
            },
        )
        when {
            loading -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                CircularProgressIndicator(color = Brand.Navy)
            }
            error != null -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                Column(
                    horizontalAlignment = Alignment.CenterHorizontally,
                    modifier = Modifier.padding(24.dp),
                ) {
                    Text(error!!, color = Brand.Danger, textAlign = TextAlign.Center)
                    Spacer(Modifier.height(12.dp))
                    GoldButton(text = "Retry", onClick = { refresh() })
                }
            }
            items.isEmpty() -> Box(
                Modifier
                    .fillMaxSize()
                    .verticalScroll(rememberScrollState())
                    .padding(16.dp),
                contentAlignment = Alignment.TopCenter,
            ) {
                BrandCard {
                    Column(
                        horizontalAlignment = Alignment.CenterHorizontally,
                        verticalArrangement = Arrangement.spacedBy(12.dp),
                        modifier = Modifier.fillMaxWidth(),
                    ) {
                        Surface(
                            shape = RoundedCornerShape(14.dp),
                            color = Brand.NavyTint,
                            modifier = Modifier.size(56.dp),
                        ) {
                            Box(contentAlignment = Alignment.Center) {
                                Icon(
                                    Icons.Default.Build,
                                    null,
                                    modifier = Modifier.size(28.dp),
                                    tint = Brand.Navy,
                                )
                            }
                        }
                        Text(
                            "No repairs yet",
                            style = MaterialTheme.typography.titleMedium,
                            fontWeight = FontWeight.SemiBold,
                            color = Brand.TextPrimary,
                        )
                        Text(
                            "When you drop a device at the shop, it will show here.",
                            style = MaterialTheme.typography.bodyMedium,
                            color = Brand.TextSecondary,
                            textAlign = TextAlign.Center,
                        )
                        HorizontalDivider(color = Brand.Border)
                        BrandSectionTitle("Already dropped a device?")
                        var claimJob by remember { mutableStateOf("") }
                        var claimBusy by remember { mutableStateOf(false) }
                        var claimError by remember { mutableStateOf<String?>(null) }
                        OutlinedTextField(
                            value = claimJob,
                            onValueChange = { claimJob = it.uppercase() },
                            label = { Text("Job code") },
                            modifier = Modifier.fillMaxWidth(),
                            singleLine = true,
                        )
                        claimError?.let { Text(it, color = Brand.Danger) }
                        GoldButton(
                            text = "Claim my repair",
                            onClick = {
                                if (claimJob.isBlank()) {
                                    claimError = "Job code required"
                                    return@GoldButton
                                }
                                claimBusy = true
                                claimError = null
                                scope.launch {
                                    try {
                                        withContext(Dispatchers.IO) { CustomerApi.claimRepair(claimJob.trim()) }
                                        refresh()
                                        claimJob = ""
                                    } catch (e: Exception) {
                                        claimError = e.message
                                    } finally {
                                        claimBusy = false
                                    }
                                }
                            },
                            enabled = !claimBusy,
                            loading = claimBusy,
                            modifier = Modifier.fillMaxWidth(),
                        )
                    }
                }
            }
            else -> LazyColumn(
                contentPadding = PaddingValues(16.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                items(items, key = { it.optString("id") }) { job ->
                    RepairCard(job) { onOpen(job.getString("id")) }
                }
            }
        }
    }
}

@Composable
private fun RepairCard(job: JSONObject, onClick: () -> Unit) {
    val status = job.optString("status").ifBlank { "open" }
    val color = statusColor(status)
    val balance = when {
        job.has("balance_due") && !job.isNull("balance_due") -> {
            val raw = job.opt("balance_due")
            when (raw) {
                is Number -> raw.toDouble()
                is Boolean -> if (raw) job.optDouble("amount_due", 0.0) else 0.0
                else -> job.optDouble("balance_due", job.optDouble("amount_due", 0.0))
            }
        }
        else -> job.optDouble("amount_due", 0.0)
    }
    BrandCard(onClick = onClick) {
        Row(
            Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                job.optString("job_code").ifBlank { "Repair" },
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.Bold,
                color = Brand.TextPrimary,
            )
            PillBadge(status.replace('_', ' '), color)
        }
        Text(
            listOfNotNull(
                job.optString("device_brand").takeIf { it.isNotBlank() },
                job.optString("device_model").takeIf { it.isNotBlank() },
            ).joinToString(" ").ifBlank { "Device under repair" },
            style = MaterialTheme.typography.bodyLarge,
            color = Brand.TextSecondary,
        )
        job.optJSONObject("estimate")?.let { est ->
            if (est.optString("status") == "pending") {
                PillBadge("Estimate awaiting approval", Brand.GoldDark)
            }
        }
        if (balance > 0) {
            Text(
                "KES ${"%.0f".format(balance)} due",
                color = Brand.Warning,
                style = MaterialTheme.typography.labelLarge,
                fontWeight = FontWeight.SemiBold,
            )
        }
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

    val deviceLabel = detail?.let { job ->
        listOfNotNull(
            job.optString("device_brand").takeIf { it.isNotBlank() },
            job.optString("device_model").takeIf { it.isNotBlank() },
        ).joinToString(" ").ifBlank { null }
    }

    Column(
        rootModifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background),
    ) {
        BrandDetailHeader(
            title = detail?.optString("job_code") ?: "Repair",
            subtitle = deviceLabel,
            navigation = {
                IconButton(onClick = onBack) {
                    Icon(
                        Icons.AutoMirrored.Filled.ArrowBack,
                        contentDescription = "Back",
                        tint = Color.White,
                    )
                }
            },
            trailing = {
                IconButton(onClick = { refresh() }) {
                    Icon(Icons.Default.Refresh, contentDescription = "Refresh", tint = Color.White)
                }
            },
        )
        when {
            loading && detail == null -> Box(
                Modifier.fillMaxSize(),
                contentAlignment = Alignment.Center,
            ) { CircularProgressIndicator(color = Brand.Navy) }
            error != null && detail == null -> Box(
                Modifier.fillMaxSize(),
                contentAlignment = Alignment.Center,
            ) {
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    Text(error!!, color = Brand.Danger)
                    Spacer(Modifier.height(12.dp))
                    GoldButton(text = "Retry", onClick = { refresh() })
                }
            }
            else -> {
                val job = detail!!
                val estimate = job.optJSONObject("estimate") ?: job.optJSONObject("pending_estimate")
                val timeline = job.optJSONArray("timeline") ?: job.optJSONArray("events")
                val statusFg = statusColor(job.optString("status"))
                Column(
                    Modifier
                        .fillMaxSize()
                        .verticalScroll(rememberScrollState())
                        .padding(16.dp),
                    verticalArrangement = Arrangement.spacedBy(12.dp),
                ) {
                    BrandCard {
                        BrandSectionTitle("Device")
                        Spacer(Modifier.height(8.dp))
                        Text(
                            listOfNotNull(
                                job.optString("device_brand").takeIf { it.isNotBlank() },
                                job.optString("device_model").takeIf { it.isNotBlank() },
                            ).joinToString(" ").ifBlank { "Device" },
                            style = MaterialTheme.typography.headlineSmall,
                            color = Brand.TextPrimary,
                        )
                        Spacer(Modifier.height(8.dp))
                        PillBadge(job.optString("status").replace('_', ' '), statusFg)
                        job.optString("problem_summary").takeIf { it.isNotBlank() }?.let {
                            Spacer(Modifier.height(8.dp))
                            Text(it, style = MaterialTheme.typography.bodyMedium, color = Brand.TextSecondary)
                        }
                        job.optString("customer_name").takeIf { it.isNotBlank() }?.let {
                            Spacer(Modifier.height(4.dp))
                            Text(it, style = MaterialTheme.typography.bodyMedium, color = Brand.TextPrimary)
                        }
                    }

                    if (estimate != null && estimate.optString("status") == "pending") {
                        BrandCard {
                            BrandSectionTitle("Estimate for approval")
                            Spacer(Modifier.height(10.dp))
                            val total = if (estimate.has("total_amount")) {
                                estimate.optDouble("total_amount", 0.0)
                            } else {
                                estimate.optDouble("labor_amount", 0.0) + estimate.optDouble("parts_amount", 0.0)
                            }
                            val currency = estimate.optString("currency", "KES")
                            Text(
                                "$currency ${"%.0f".format(total)}",
                                style = MaterialTheme.typography.headlineSmall,
                                fontWeight = FontWeight.Bold,
                                color = Brand.TextPrimary,
                            )
                            estimate.optString("notes").takeIf { it.isNotBlank() }?.let {
                                Spacer(Modifier.height(6.dp))
                                Text(it, style = MaterialTheme.typography.bodyMedium, color = Brand.TextSecondary)
                            }
                            error?.let {
                                Spacer(Modifier.height(6.dp))
                                Text(it, color = Brand.Danger)
                            }
                            Spacer(Modifier.height(10.dp))
                            GoldButton(
                                text = "Approve estimate",
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
                                loading = busy,
                                modifier = Modifier.fillMaxWidth(),
                            )
                            Spacer(Modifier.height(8.dp))
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
                                modifier = Modifier.fillMaxWidth().height(48.dp),
                            ) { Text("Reject") }
                        }
                    }

                    val balance = job.optDouble("balance_due", job.optDouble("amount_due", 0.0))
                    if (balance > 0) {
                        BrandCard {
                            BrandSectionTitle("Pay balance")
                            Spacer(Modifier.height(8.dp))
                            Text(
                                "KES ${"%.0f".format(balance)}",
                                style = MaterialTheme.typography.headlineMedium,
                                fontWeight = FontWeight.Bold,
                                color = Brand.TextPrimary,
                            )
                            payMessage?.let {
                                Spacer(Modifier.height(6.dp))
                                Text(it, color = Brand.Navy)
                            }
                            Spacer(Modifier.height(10.dp))
                            GoldButton(
                                text = if (busy) "Sending STK…" else "Pay now",
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
                                loading = busy,
                                modifier = Modifier.fillMaxWidth(),
                            )
                        }
                    }

                    BrandCard {
                        BrandSectionTitle("Timeline")
                        Spacer(Modifier.height(12.dp))
                        if (timeline == null || timeline.length() == 0) {
                            Text("No updates yet.", color = Brand.TextSecondary)
                        } else {
                            for (i in 0 until timeline.length()) {
                                val ev = timeline.getJSONObject(i)
                                val evStatus = ev.optString("status").ifBlank { ev.optString("event_type") }
                                val dotColor = statusColor(evStatus)
                                Row(
                                    Modifier.fillMaxWidth(),
                                    horizontalArrangement = Arrangement.spacedBy(12.dp),
                                ) {
                                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                                        Box(
                                            Modifier
                                                .size(10.dp)
                                                .clip(CircleShape)
                                                .background(dotColor),
                                        )
                                        if (i < timeline.length() - 1) {
                                            Box(
                                                Modifier
                                                    .width(2.dp)
                                                    .height(36.dp)
                                                    .background(Brand.Border),
                                            )
                                        }
                                    }
                                    Column(Modifier.padding(bottom = 12.dp)) {
                                        Text(
                                            evStatus.replace('_', ' '),
                                            style = MaterialTheme.typography.titleSmall,
                                            color = Brand.TextPrimary,
                                            fontWeight = FontWeight.SemiBold,
                                        )
                                        ev.optString("note").takeIf { it.isNotBlank() }?.let {
                                            Text(it, style = MaterialTheme.typography.bodySmall, color = Brand.TextSecondary)
                                        }
                                        Text(
                                            ev.optString("created_at").take(16),
                                            style = MaterialTheme.typography.labelSmall,
                                            color = Brand.TextMuted,
                                        )
                                    }
                                }
                            }
                        }
                    }

                    BrandCard {
                        BrandSectionTitle("Receipts")
                        Spacer(Modifier.height(10.dp))
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
                        val receipts = job.optJSONArray("receipts")
                        Spacer(Modifier.height(10.dp))
                        if (receipts != null && receipts.length() > 0) {
                            for (i in 0 until receipts.length()) {
                                val r = receipts.getJSONObject(i)
                                Surface(
                                    Modifier.fillMaxWidth().padding(vertical = 4.dp),
                                    shape = RoundedCornerShape(12.dp),
                                    color = Brand.Subtle,
                                ) {
                                    Column(Modifier.padding(12.dp)) {
                                        Text(
                                            "KES ${"%.2f".format(r.optDouble("amount", 0.0))}",
                                            style = MaterialTheme.typography.titleMedium,
                                            color = Brand.TextPrimary,
                                        )
                                        Text(
                                            "${r.optString("method")} · ${r.optString("status")}",
                                            style = MaterialTheme.typography.bodySmall,
                                            color = Brand.TextSecondary,
                                        )
                                    }
                                }
                            }
                        } else {
                            Text(
                                "Receipts appear after payment.",
                                style = MaterialTheme.typography.bodySmall,
                                color = Brand.TextMuted,
                            )
                        }
                    }

                    BrandCard {
                        BrandSectionTitle("Warranty")
                        Spacer(Modifier.height(10.dp))
                        if (warranty != null) {
                            val w = warranty!!
                            Text(
                                "${w.optString("status")} · ${w.optInt("duration_days")} days",
                                color = Brand.TextPrimary,
                            )
                            Text(
                                "Ends ${w.optString("ends_at").take(10)}",
                                color = Brand.TextSecondary,
                                style = MaterialTheme.typography.bodyMedium,
                            )
                            if (w.optString("status") == "active") {
                                Spacer(Modifier.height(8.dp))
                                OutlinedTextField(
                                    value = claimNote,
                                    onValueChange = { claimNote = it },
                                    label = { Text("Claim note") },
                                    modifier = Modifier.fillMaxWidth(),
                                )
                                Spacer(Modifier.height(8.dp))
                                GoldButton(
                                    text = "Claim warranty",
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
                                    loading = busy,
                                    modifier = Modifier.fillMaxWidth(),
                                )
                            }
                        } else {
                            Text(
                                "Warranty is issued when the repair is completed.",
                                style = MaterialTheme.typography.bodySmall,
                                color = Brand.TextMuted,
                            )
                        }
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
        rootModifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        BrandCard {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(14.dp),
            ) {
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
                Column {
                    Text(
                        name,
                        style = MaterialTheme.typography.headlineSmall,
                        fontWeight = FontWeight.SemiBold,
                        color = Brand.TextPrimary,
                    )
                    Text(
                        phone,
                        style = MaterialTheme.typography.titleMedium,
                        color = Brand.TextSecondary,
                    )
                }
            }
            error?.let {
                Spacer(Modifier.height(8.dp))
                Text(it, color = Brand.Danger)
            }
            Spacer(Modifier.height(12.dp))
            Text(
                "Sessions expire for your security. You can request a new code anytime.",
                style = MaterialTheme.typography.bodyMedium,
                color = Brand.TextSecondary,
            )
        }
    }
}
