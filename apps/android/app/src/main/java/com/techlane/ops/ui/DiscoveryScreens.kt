package com.techlane.ops.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Notifications
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.techlane.core.theme.Brand
import com.techlane.core.ui.BrandCard
import com.techlane.core.ui.BrandHero
import com.techlane.core.ui.PillBadge
import com.techlane.ops.network.ApiClient
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONObject

@Composable
fun UniversalSearchScreen(
    onOpenJob: (String) -> Unit,
    onNavigate: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    var query by remember { mutableStateOf("") }
    var results by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var loading by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }

    LaunchedEffect(query) {
        val term = query.trim()
        if (term.length < 2) {
            results = emptyList()
            loading = false
            return@LaunchedEffect
        }
        delay(250)
        loading = true
        error = null
        try {
            val items = withContext(Dispatchers.IO) { ApiClient.universalSearch(term) }
            results = (0 until items.length()).map { items.getJSONObject(it) }
        } catch (e: Exception) {
            error = e.message
        } finally {
            loading = false
        }
    }

    Column(modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        BrandHero(
            title = "Search everything",
            subtitle = "Jobs, customers, phones and collection orders",
            appLabel = "Ops",
        )
        Column(
            Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            OutlinedTextField(
                value = query,
                onValueChange = { query = it },
                label = { Text("Name, phone, job code, device or collection code") },
                leadingIcon = { Icon(Icons.Filled.Search, null) },
                supportingText = { Text(if (query.trim().length < 2) "Type at least 2 characters" else if (loading) "Searching…" else "${results.size} results") },
                modifier = Modifier.fillMaxWidth(),
                singleLine = true,
            )
            error?.let { Text(it, color = Brand.Danger) }
            if (query.trim().length >= 2 && !loading && results.isEmpty() && error == null) {
                EmptyHint("Try a phone number, job code, customer or device.", title = "No matches")
            }
            results.forEach { item ->
                BrandCard(
                    onClick = {
                        when (item.optString("type")) {
                            "repair" -> onOpenJob(item.optString("id"))
                            "order" -> onNavigate("pickup")
                            "stock" -> onNavigate("inventory")
                            "customer" -> {
                                query = item.optString("title")
                                onNavigate("jobs")
                            }
                        }
                    },
                ) {
                    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                        Text(item.optString("title"), fontWeight = FontWeight.SemiBold, modifier = Modifier.weight(1f))
                        PillBadge(item.optString("type").replaceFirstChar { it.uppercase() }, Brand.Navy)
                    }
                    Text(item.optString("subtitle"), color = Brand.TextSecondary, style = MaterialTheme.typography.bodySmall)
                }
            }
        }
    }
}

@OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)
@Composable
fun NotificationCenterScreen(
    onOpenJob: (String) -> Unit,
    modifier: Modifier = Modifier,
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
                val response = withContext(Dispatchers.IO) { ApiClient.listNotifications() }
                items = (0 until response.length()).map { response.getJSONObject(it) }
            } catch (e: Exception) {
                error = e.message
            } finally {
                loading = false
            }
        }
    }
    LaunchedEffect(Unit) { refresh() }

    Column(modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        BrandHero(
            title = "Attention centre",
            subtitle = "Messages and operational exceptions in one place",
            appLabel = "Ops",
            trailing = {
                IconButton(onClick = ::refresh) {
                    Icon(Icons.Filled.Notifications, "Refresh inbox", tint = Color.White)
                }
            },
        )
        androidx.compose.material3.pulltorefresh.PullToRefreshBox(
            isRefreshing = loading,
            onRefresh = { refresh() },
            modifier = Modifier.weight(1f),
        ) {
        Column(
            Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            if (loading && items.isEmpty()) SkeletonList()
            FeedbackBanner(message = null, error = error)
            if (!loading && items.isEmpty() && error == null) {
                EmptyHint("New customer, payment and workshop alerts will appear here.", title = "You’re all caught up")
            }
            items.forEach { item ->
                val acked = item.optString("acked_at").let { it.isNotBlank() && it != "null" }
                val payload = item.optJSONObject("payload")
                val repairID = payload?.optString("repair_job_id").orEmpty()
                BrandCard(onClick = { if (repairID.isNotBlank()) onOpenJob(repairID) }) {
                    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                        Text(item.optString("title"), fontWeight = FontWeight.SemiBold, modifier = Modifier.weight(1f))
                        PillBadge(if (acked) "Seen" else "New", if (acked) Brand.TextMuted else Brand.Warning)
                    }
                    Text(item.optString("body"), color = Brand.TextSecondary)
                    Text(timeAgo(item.optString("created_at")), style = MaterialTheme.typography.bodySmall, color = Brand.TextMuted)
                    if (!acked) {
                        TextButton(
                            onClick = {
                                scope.launch {
                                    runCatching { withContext(Dispatchers.IO) { ApiClient.ackNotification(item.getString("id")) } }
                                    refresh()
                                }
                            },
                        ) { Text("Mark handled") }
                    }
                }
            }
        }
        }
    }
}
