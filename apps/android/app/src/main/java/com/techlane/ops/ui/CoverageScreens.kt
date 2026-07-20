package com.techlane.ops.ui

import android.content.Context
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.provider.OpenableColumns
import android.util.Base64
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.FilterChip
import androidx.compose.material3.FilterChipDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.techlane.ops.network.ApiClient
import com.techlane.core.scan.ScanCameraPanel
import com.techlane.core.scan.parseScanPayload
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONObject

data class BranchOption(val id: String, val name: String)

@Composable
fun RepairAttachmentsPanel(jobId: String, modifier: Modifier = Modifier) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    var attachments by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var refreshKey by remember { mutableStateOf(0) }
    var busy by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    var message by remember { mutableStateOf<String?>(null) }
    var takePictureUri by remember { mutableStateOf<android.net.Uri?>(null) }

    fun uploadUri(uri: android.net.Uri, fallbackName: String) {
        scope.launch {
            busy = true
            error = null
            message = null
            try {
                val bytes = withContext(Dispatchers.IO) {
                    context.contentResolver.openInputStream(uri)?.use { it.readBytes() }
                        ?: error("Could not read selected file")
                }
                if (bytes.size > 5 * 1024 * 1024) error("Attachment must be 5 MB or smaller")
                val name = context.contentResolver.query(uri, arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)?.use { cursor ->
                    if (cursor.moveToFirst()) cursor.getString(0) else null
                } ?: fallbackName
                val type = context.contentResolver.getType(uri) ?: "image/jpeg"
                withContext(Dispatchers.IO) {
                    ApiClient.addRepairAttachment(jobId, name, type, Base64.encodeToString(bytes, Base64.NO_WRAP))
                }
                message = "Attachment uploaded"
                refreshKey++
            } catch (e: Exception) {
                error = e.message
            } finally {
                busy = false
            }
        }
    }

    fun createCameraUri(): android.net.Uri {
        val dir = java.io.File(context.cacheDir, "camera").apply { mkdirs() }
        val file = java.io.File(dir, "repair-${System.currentTimeMillis()}.jpg")
        return androidx.core.content.FileProvider.getUriForFile(
            context,
            "${context.packageName}.fileprovider",
            file,
        )
    }

    LaunchedEffect(jobId, refreshKey) {
        try {
            val result = withContext(Dispatchers.IO) { ApiClient.listRepairAttachments(jobId) }
            attachments = (0 until result.length()).map { result.getJSONObject(it) }
        } catch (e: Exception) {
            error = e.message
        }
    }
    val picker = rememberLauncherForActivityResult(ActivityResultContracts.GetContent()) { uri ->
        if (uri != null) uploadUri(uri, "repair-photo")
    }
    var launchCameraAfterPermission by remember { mutableStateOf(false) }
    val takePicture = rememberLauncherForActivityResult(ActivityResultContracts.TakePicture()) { ok ->
        val uri = takePictureUri
        takePictureUri = null
        if (ok && uri != null) uploadUri(uri, "repair-camera.jpg")
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

    fun startCameraCapture() {
        val granted = androidx.core.content.ContextCompat.checkSelfPermission(
            context,
            android.Manifest.permission.CAMERA,
        ) == android.content.pm.PackageManager.PERMISSION_GRANTED
        if (granted) {
            val uri = createCameraUri()
            takePictureUri = uri
            takePicture.launch(uri)
        } else {
            cameraPermission.launch(android.Manifest.permission.CAMERA)
        }
    }

    Column(modifier = modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text("Photos and attachments", style = MaterialTheme.typography.titleMedium)
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.fillMaxWidth()) {
            Button(
                onClick = { startCameraCapture() },
                enabled = !busy,
                modifier = Modifier.weight(1f),
            ) {
                Text(if (busy) "Uploading…" else "Take photo")
            }
            Button(
                onClick = { picker.launch("*/*") },
                enabled = !busy,
                modifier = Modifier.weight(1f),
            ) {
                Text("Choose file")
            }
        }
        attachments.forEach {
            Card(
                modifier.fillMaxWidth(),
                shape = RoundedCornerShape(14.dp),
                elevation = CardDefaults.cardElevation(defaultElevation = 1.dp),
                border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline.copy(alpha = 0.18f)),
            ) {
                Column(Modifier.padding(10.dp)) {
                    Text(it.optString("file_name"), style = MaterialTheme.typography.titleSmall)
                    Text("${it.optString("content_type")} · ${it.optLong("size_bytes") / 1024} KB")
                }
            }
        }
        message?.let { Text(it, color = MaterialTheme.colorScheme.primary) }
        error?.let { Text(it, color = MaterialTheme.colorScheme.error) }
    }
}

@Composable
fun ConnectivityBanner(modifier: Modifier = Modifier) {
    val context = LocalContext.current
    var online by remember { mutableStateOf(isOnline(context)) }
    LaunchedEffect(Unit) {
        while (true) {
            online = isOnline(context)
            delay(5_000)
        }
    }
    if (!online) {
        Text(
            "Offline — network actions will fail; intake drafts remain in the outbox.",
            color = MaterialTheme.colorScheme.onErrorContainer,
            modifier = modifier
                .fillMaxWidth()
                .background(MaterialTheme.colorScheme.errorContainer)
                .padding(8.dp),
        )
    }
}

private fun isOnline(context: Context): Boolean {
    val manager = context.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
    val network = manager.activeNetwork ?: return false
    val capabilities = manager.getNetworkCapabilities(network) ?: return false
    return capabilities.hasCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET) ||
        capabilities.hasCapability(NetworkCapabilities.NET_CAPABILITY_VALIDATED) ||
        capabilities.hasTransport(NetworkCapabilities.TRANSPORT_WIFI) ||
        capabilities.hasTransport(NetworkCapabilities.TRANSPORT_CELLULAR) ||
        capabilities.hasTransport(NetworkCapabilities.TRANSPORT_ETHERNET)
}

@Composable
fun BranchPicker(
    branches: List<BranchOption>,
    selectedBranchId: String?,
    onSelected: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    if (branches.isEmpty()) return
    Row(
        modifier = modifier
            .fillMaxWidth()
            .horizontalScroll(rememberScrollState())
            .padding(horizontal = 12.dp, vertical = 4.dp),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        branches.forEach { branch ->
            FilterChip(
                selected = selectedBranchId == branch.id,
                onClick = { onSelected(branch.id) },
                label = { Text(branch.name) },
                colors = FilterChipDefaults.filterChipColors(
                    selectedContainerColor = MaterialTheme.colorScheme.primaryContainer,
                    selectedLabelColor = MaterialTheme.colorScheme.onPrimaryContainer,
                ),
            )
        }
    }
}

@Composable
fun PosScreen(branchId: String?, modifier: Modifier = Modifier) {
    var locations by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var locationId by remember { mutableStateOf<String?>(null) }
    var catalog by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var cart by remember { mutableStateOf<Map<String, Int>>(emptyMap()) }
    var query by remember { mutableStateOf("") }
    var method by remember { mutableStateOf("cash") }
    var phone by remember { mutableStateOf("") }
    var accountRef by remember { mutableStateOf("") }
    var error by remember { mutableStateOf<String?>(null) }
    var completion by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()

    fun load() {
        val branch = branchId ?: return
        scope.launch {
            try {
                val locs = withContext(Dispatchers.IO) { ApiClient.listStockLocations(branch) }
                locations = (0 until locs.length()).map { locs.getJSONObject(it) }
                val valid = locations.any { it.optString("id") == locationId }
                if (!valid) locationId = locations.firstOrNull()?.optString("id")
                val products = withContext(Dispatchers.IO) { ApiClient.listCatalog(locationId) }
                catalog = (0 until products.length()).map { products.getJSONObject(it) }
                error = null
            } catch (e: Exception) {
                error = e.message
            }
        }
    }

    LaunchedEffect(branchId, locationId) { load() }
    val filtered = catalog.filter {
        query.isBlank() || listOf(it.optString("product_name"), it.optString("sku"), it.optString("brand"))
            .any { value -> value.contains(query, ignoreCase = true) }
    }
    val total = catalog.sumOf { item ->
        item.optDouble("sell_price", 0.0) * (cart[item.optString("variant_id")] ?: 0)
    }

    Column(
        modifier = modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Text(
            "Point of sale",
            style = MaterialTheme.typography.headlineSmall,
            fontWeight = FontWeight.SemiBold,
        )
        Text(
            "Sell from branch stock with cash or M-Pesa.",
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        if (branchId == null) Text("Select a branch before selling.", color = MaterialTheme.colorScheme.error)
        if (locations.isNotEmpty()) {
            Text("Stock location", style = MaterialTheme.typography.titleSmall)
            Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                locations.forEach { location ->
                    val id = location.optString("id")
                    FilterChip(
                        selected = id == locationId,
                        onClick = { locationId = id; cart = emptyMap() },
                        label = { Text(location.optString("name", "Stock")) },
                    )
                }
            }
        }
        OutlinedTextField(
            value = query,
            onValueChange = { query = it },
            label = { Text("Search name, SKU, or barcode") },
            modifier = Modifier.fillMaxWidth(),
        )
        filtered.forEach { item ->
            val id = item.optString("variant_id")
            val qty = cart[id] ?: 0
            Card(
                modifier.fillMaxWidth(),
                shape = RoundedCornerShape(14.dp),
                elevation = CardDefaults.cardElevation(defaultElevation = 1.dp),
                border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline.copy(alpha = 0.18f)),
            ) {
                Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    Text(item.optString("product_name"), style = MaterialTheme.typography.titleMedium)
                    Text("${item.optString("sku")} · KES ${item.optDouble("sell_price").toInt()} · ${item.optInt("available_qty")} available")
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        Button(onClick = { cart = cart + (id to (qty - 1).coerceAtLeast(0)) }, enabled = qty > 0) { Text("−") }
                        Text(qty.toString(), modifier = Modifier.padding(vertical = 12.dp))
                        Button(
                            onClick = { cart = cart + (id to (qty + 1)) },
                            enabled = qty < item.optInt("available_qty"),
                        ) { Text("+") }
                    }
                }
            }
        }
        Text("Cart total: KES ${total.toInt()}", style = MaterialTheme.typography.titleMedium)
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            listOf("cash" to "Cash", "mpesa_stk" to "STK", "mpesa_c2b" to "C2B").forEach { (key, label) ->
                FilterChip(selected = method == key, onClick = { method = key }, label = { Text(label) })
            }
        }
        if (method == "mpesa_stk") {
            OutlinedTextField(phone, { phone = it }, label = { Text("Customer phone") }, modifier = Modifier.fillMaxWidth())
        }
        if (method == "mpesa_c2b") {
            OutlinedTextField(accountRef, { accountRef = it.uppercase() }, label = { Text("Account reference (optional)") }, modifier = Modifier.fillMaxWidth())
        }
        Button(
            onClick = {
                val branch = branchId
                val location = locationId
                val lines = cart.filterValues { it > 0 }.map { it.key to it.value }
                when {
                    branch == null -> error = "Select a branch"
                    location == null -> error = "Select a stock location"
                    lines.isEmpty() -> error = "Add an item"
                    method == "mpesa_stk" && phone.isBlank() -> error = "Phone required for STK"
                    else -> {
                        busy = true
                        error = null
                        completion = null
                        scope.launch {
                            try {
                                val result = withContext(Dispatchers.IO) {
                                    ApiClient.posCheckout(branch, location, lines, method, phone.ifBlank { null }, accountRef.ifBlank { null })
                                }
                                val sale = result.optJSONObject("sale")
                                val payment = result.optJSONObject("payment")
                                completion = if (result.optBoolean("completed")) {
                                    "Sale complete · ${sale?.optString("id")?.take(8)} · ${payment?.optString("status")}"
                                } else {
                                    "Sale awaiting payment · ${payment?.optString("account_reference")} · ${payment?.optString("status")}"
                                }
                                cart = emptyMap()
                                load()
                            } catch (e: Exception) {
                                error = e.message
                            } finally {
                                busy = false
                            }
                        }
                    }
                }
            },
            enabled = !busy,
            modifier = Modifier.fillMaxWidth(),
        ) { Text(if (busy) "Taking payment…" else "Create sale and take payment") }
        completion?.let { Text(it, color = MaterialTheme.colorScheme.primary) }
        error?.let { Text(it, color = MaterialTheme.colorScheme.error) }
    }
}

@Composable
fun InventoryLookupScreen(branchId: String?, modifier: Modifier = Modifier) {
    var query by remember { mutableStateOf("") }
    var locationId by remember { mutableStateOf<String?>(null) }
    var locations by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var balances by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var error by remember { mutableStateOf<String?>(null) }

    LaunchedEffect(branchId, locationId) {
        if (branchId == null) return@LaunchedEffect
        try {
            val locs = withContext(Dispatchers.IO) { ApiClient.listStockLocations(branchId) }
            locations = (0 until locs.length()).map { locs.getJSONObject(it) }
            if (locationId != null && locations.none { it.optString("id") == locationId }) locationId = null
            val items = withContext(Dispatchers.IO) { ApiClient.listInventoryBalances(locationId) }
            balances = (0 until items.length()).map { items.getJSONObject(it) }
            error = null
        } catch (e: Exception) {
            error = e.message
        }
    }
    val filtered = balances.filter {
        query.isBlank() || it.optString("product_name").contains(query, true) || it.optString("sku").contains(query, true)
    }
    Column(
        modifier = modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Text(
            "Inventory lookup",
            style = MaterialTheme.typography.headlineSmall,
            fontWeight = FontWeight.SemiBold,
        )
        Text(
            "Check available, physical, and reserved quantities.",
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            FilterChip(selected = locationId == null, onClick = { locationId = null }, label = { Text("All") })
            locations.forEach {
                val id = it.optString("id")
                FilterChip(selected = locationId == id, onClick = { locationId = id }, label = { Text(it.optString("name")) })
            }
        }
        OutlinedTextField(query, { query = it }, label = { Text("Search product or SKU") }, modifier = Modifier.fillMaxWidth())
        filtered.forEach { item ->
            Card(
                modifier.fillMaxWidth(),
                shape = RoundedCornerShape(14.dp),
                elevation = CardDefaults.cardElevation(defaultElevation = 1.dp),
                border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline.copy(alpha = 0.18f)),
            ) {
                Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(3.dp)) {
                    Text(item.optString("product_name"), style = MaterialTheme.typography.titleMedium)
                    Text("${item.optString("sku")} · ${item.optString("location_name")}")
                    Text("Available ${item.optInt("available_qty")} · Physical ${item.optInt("physical_qty")} · Reserved ${item.optInt("reserved_qty")}")
                }
            }
        }
        if (filtered.isEmpty() && error == null) {
            EmptyHint(
                message = "Try another location or search term.",
                title = "No stock matches",
            )
        }
        error?.let { Text(it, color = MaterialTheme.colorScheme.error) }
    }
}

@Composable
fun C2BExceptionsScreen(modifier: Modifier = Modifier) {
    var items by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var paymentId by remember { mutableStateOf<Map<String, String>>(emptyMap()) }
    var error by remember { mutableStateOf<String?>(null) }
    var message by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()

    fun refresh() {
        scope.launch {
            try {
                val result = withContext(Dispatchers.IO) { ApiClient.listC2B("unmatched") }
                items = (0 until result.length()).map { result.getJSONObject(it) }
                error = null
            } catch (e: Exception) {
                error = e.message
            }
        }
    }
    LaunchedEffect(Unit) { refresh() }
    Column(
        modifier = modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Text(
            "Unmatched C2B",
            style = MaterialTheme.typography.headlineSmall,
            fontWeight = FontWeight.SemiBold,
        )
        Text(
            "Match received paybill money to an existing pending payment ID.",
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        items.forEach { item ->
            val id = item.optString("id")
            Card(
                modifier.fillMaxWidth(),
                shape = RoundedCornerShape(14.dp),
                elevation = CardDefaults.cardElevation(defaultElevation = 1.dp),
                border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline.copy(alpha = 0.18f)),
            ) {
                Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    Text("KES ${item.optDouble("amount").toInt()} · ${item.optString("trans_id")}", style = MaterialTheme.typography.titleMedium)
                    Text("Ref ${item.optString("bill_ref_number")} · ${item.optString("msisdn")}")
                    OutlinedTextField(
                        paymentId[id].orEmpty(),
                        { paymentId = paymentId + (id to it.trim()) },
                        label = { Text("Payment UUID") },
                        modifier = Modifier.fillMaxWidth(),
                    )
                    Button(onClick = {
                        val target = paymentId[id].orEmpty()
                        if (target.isBlank()) {
                            error = "Enter a payment ID"
                        } else {
                            scope.launch {
                                try {
                                    withContext(Dispatchers.IO) { ApiClient.matchC2B(id, target) }
                                    message = "C2B transaction matched"
                                    refresh()
                                } catch (e: Exception) {
                                    error = e.message
                                }
                            }
                        }
                    }) { Text("Match payment") }
                }
            }
        }
        if (items.isEmpty() && error == null) {
            EmptyHint(
                message = "Paybill exceptions that need a payment match will appear here.",
                title = "No unmatched C2B",
            )
        }
        message?.let { Text(it, color = MaterialTheme.colorScheme.primary) }
        error?.let { Text(it, color = MaterialTheme.colorScheme.error) }
    }
}

@Composable
fun ManualScanScreen(modifier: Modifier = Modifier) {
    var mode by remember { mutableStateOf("imei") }
    var code by remember { mutableStateOf("") }
    var relatedId by remember { mutableStateOf("") }
    var results by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var message by remember { mutableStateOf<String?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var scanning by remember { mutableStateOf(true) }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()

    fun runLookup(
        lookupCode: String = code,
        lookupMode: String = mode,
        lookupRelatedId: String = relatedId,
    ) {
        val value = lookupCode.trim()
        if (value.isBlank()) {
            error = "Scan or enter a code"
            return
        }
        if (busy) return
        scope.launch {
            busy = true
            scanning = false
            try {
                error = null
                when (lookupMode) {
                    "imei" -> {
                        val array = withContext(Dispatchers.IO) { ApiClient.listRepairs(q = value) }
                        results = (0 until array.length()).map { array.getJSONObject(it) }
                        message = if (results.isEmpty()) "No repairs for $value" else "${results.size} repair(s) found"
                    }
                    "barcode" -> {
                        val array = withContext(Dispatchers.IO) { ApiClient.listCatalog() }
                        results = (0 until array.length()).map { array.getJSONObject(it) }
                            .filter {
                                it.optString("sku").contains(value, true) ||
                                    it.optString("barcode").contains(value, true) ||
                                    it.optString("product_name").contains(value, true)
                            }
                        message = if (results.isEmpty()) "No catalog match for $value" else "${results.size} catalog item(s) found"
                    }
                    "auth" -> {
                        if (lookupRelatedId.isBlank()) {
                            error = "Auth QR must include the supplier issue id"
                        } else {
                            withContext(Dispatchers.IO) { ApiClient.collectSupplierIssue(lookupRelatedId, value) }
                            results = emptyList()
                            message = "Supplier issue collected"
                        }
                    }
                    "collection" -> {
                        val order = withContext(Dispatchers.IO) { ApiClient.collectOnlineOrder(value) }
                        results = listOf(order)
                        message = "Order ${order.optString("id").take(8)} collected"
                    }
                }
            } catch (e: Exception) {
                error = e.message
            } finally {
                busy = false
                // Resume camera so the next code can be scanned without another tap.
                delay(900)
                scanning = true
            }
        }
    }

    fun onScanned(raw: String) {
        val parsed = parseScanPayload(raw)
        if (parsed.code.isBlank()) return
        parsed.mode?.let { mode = it }
        code = parsed.code
        parsed.relatedId?.let { relatedId = it }
        message = "Scanned ${parsed.code}"
        error = null
        results = emptyList()
        val nextMode = parsed.mode ?: mode
        val nextRelated = parsed.relatedId ?: relatedId
        if (nextMode == "auth" && nextRelated.isBlank()) {
            error = "Auth code loaded — scan/enter the supplier issue QR or UUID"
            return
        }
        runLookup(
            lookupCode = parsed.code,
            lookupMode = nextMode,
            lookupRelatedId = nextRelated,
        )
    }

    Column(
        modifier = modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Text(
            "Scan",
            style = MaterialTheme.typography.headlineSmall,
            fontWeight = FontWeight.SemiBold,
        )
        Text(
            "Point the camera at a QR / barcode — details fill in and look up automatically.",
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Row(modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            listOf("imei" to "IMEI", "barcode" to "Barcode", "auth" to "Auth", "collection" to "Collection").forEach { (key, label) ->
                FilterChip(
                    selected = mode == key,
                    onClick = {
                        mode = key
                        results = emptyList()
                        message = null
                        error = null
                    },
                    label = { Text(label) },
                    colors = FilterChipDefaults.filterChipColors(
                        selectedContainerColor = MaterialTheme.colorScheme.primaryContainer,
                        selectedLabelColor = MaterialTheme.colorScheme.onPrimaryContainer,
                    ),
                )
            }
        }
        ScanCameraPanel(
            enabled = scanning && !busy,
            onCode = { value -> onScanned(value) },
        )
        if (busy) {
            Text("Looking up…", color = MaterialTheme.colorScheme.primary)
        }
        OutlinedTextField(
            value = code,
            onValueChange = { code = it.trim() },
            label = { Text("Code (auto-filled from scan)") },
            modifier = Modifier.fillMaxWidth(),
            readOnly = false,
        )
        if (mode == "auth") {
            OutlinedTextField(
                value = relatedId,
                onValueChange = { relatedId = it.trim() },
                label = { Text("Supplier issue UUID (auto-filled from auth QR)") },
                modifier = Modifier.fillMaxWidth(),
            )
        }
        message?.let { Text(it, color = MaterialTheme.colorScheme.primary) }
        error?.let { Text(it, color = MaterialTheme.colorScheme.error) }
        results.forEach { result ->
            Card(
                modifier.fillMaxWidth(),
                shape = RoundedCornerShape(14.dp),
                elevation = CardDefaults.cardElevation(defaultElevation = 1.dp),
                border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline.copy(alpha = 0.18f)),
            ) {
                Column(Modifier.padding(12.dp)) {
                    Text(
                        result.optString("job_code").ifBlank {
                            result.optString("product_name").ifBlank { result.optString("id").take(8) }
                        },
                        style = MaterialTheme.typography.titleMedium,
                    )
                    Text(
                        result.optString("problem_summary").ifBlank {
                            result.optString("sku").ifBlank { result.optString("status") }
                        },
                    )
                }
            }
        }
    }
}
