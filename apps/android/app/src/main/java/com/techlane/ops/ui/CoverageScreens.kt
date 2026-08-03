package com.techlane.ops.ui

import android.content.Context
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.util.Base64
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Button
import androidx.compose.material3.FilterChip
import androidx.compose.material3.FilterChipDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.unit.dp
import com.techlane.core.PrintSupport
import com.techlane.ops.TechLaneApp
import com.techlane.ops.network.ApiClient
import com.techlane.core.media.PhotoCapture
import com.techlane.core.scan.ScanCameraPanel
import com.techlane.core.scan.parseScanPayload
import com.techlane.core.theme.Brand
import com.techlane.core.ui.BrandCard
import com.techlane.core.ui.BrandHero
import com.techlane.core.ui.BrandSectionTitle
import com.techlane.core.ui.GoldButton
import com.techlane.core.ui.PillBadge
import com.techlane.core.ui.PleaseWaitOverlay
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
                val (bytes, name) = withContext(Dispatchers.IO) {
                    PhotoCapture.readImageBytes(context, uri, fallbackName)
                }
                withContext(Dispatchers.IO) {
                    ApiClient.addRepairAttachment(
                        jobId,
                        name,
                        "image/jpeg",
                        Base64.encodeToString(bytes, Base64.NO_WRAP),
                    )
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

    val picker = rememberLauncherForActivityResult(ActivityResultContracts.GetContent()) { uri ->
        if (uri != null) uploadUri(uri, "repair-photo.jpg")
    }
    val takePicture = rememberLauncherForActivityResult(ActivityResultContracts.TakePicture()) { ok ->
        val uri = takePictureUri
        takePictureUri = null
        if (uri == null) return@rememberLauncherForActivityResult
        if (!ok && !PhotoCapture.hasContent(context, uri)) return@rememberLauncherForActivityResult
        uploadUri(uri, "repair-camera.jpg")
    }
    fun launchCameraNow() {
        val uri = PhotoCapture.createCameraOutputUri(context, "repair")
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

    LaunchedEffect(jobId, refreshKey) {
        try {
            val result = withContext(Dispatchers.IO) { ApiClient.listRepairAttachments(jobId) }
            attachments = (0 until result.length()).map { result.getJSONObject(it) }
        } catch (e: Exception) {
            error = e.message
        }
    }

    fun startCameraCapture() {
        val granted = androidx.core.content.ContextCompat.checkSelfPermission(
            context,
            android.Manifest.permission.CAMERA,
        ) == android.content.pm.PackageManager.PERMISSION_GRANTED
        if (granted) {
            launchCameraNow()
        } else {
            cameraPermission.launch(android.Manifest.permission.CAMERA)
        }
    }

    Column(modifier = modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        BrandSectionTitle("Photos and attachments")
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.fillMaxWidth()) {
            OutlinedButton(
                onClick = { startCameraCapture() },
                enabled = !busy,
                modifier = Modifier.weight(1f),
            ) {
                Text(if (busy) "Uploading…" else "Take photo")
            }
            OutlinedButton(
                onClick = { picker.launch("*/*") },
                enabled = !busy,
                modifier = Modifier.weight(1f),
            ) {
                Text("Choose file")
            }
        }
        attachments.forEach {
            BrandCard {
                Text(it.optString("file_name"), style = MaterialTheme.typography.titleSmall)
                Text(
                    "${it.optString("content_type")} · ${it.optLong("size_bytes") / 1024} KB",
                    color = Brand.TextMuted,
                    style = MaterialTheme.typography.bodySmall,
                )
            }
        }
        FeedbackBanner(message = message, error = error)
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
                    selectedContainerColor = Brand.NavyTint,
                    selectedLabelColor = Brand.Navy,
                ),
            )
        }
    }
}

@Composable
fun PosScreen(
    branchId: String?,
    modifier: Modifier = Modifier,
    canOverridePrice: Boolean = false,
) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    val snackbarHostState = LocalSnackbarHost.current
    var locations by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var locationId by rememberSaveable { mutableStateOf<String?>(null) }
    var catalog by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    // Catalog cart lines (qty + optional bargained price) survive rotation / process death.
    var cart by rememberSaveable(stateSaver = PosCartLineListSaver) { mutableStateOf<List<PosCartLine>>(emptyList()) }
    var sales by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var query by rememberSaveable { mutableStateOf("") }
    var method by rememberSaveable { mutableStateOf("cash") }
    var phone by rememberSaveable { mutableStateOf("") }
    var tenderInitialized by remember { mutableStateOf(false) }
    var mpesaConfigured by remember { mutableStateOf(false) }
    var bankConfigured by remember { mutableStateOf(false) }
    var mpesaShortcode by remember { mutableStateOf("") }
    var bankPaybill by remember { mutableStateOf("") }
    var bankAccount by remember { mutableStateOf("") }
    var error by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    var waitMessage by remember { mutableStateOf("Please wait") }
    var waitDetail by remember { mutableStateOf<String?>(null) }
    var stkPolling by remember { mutableStateOf(false) }
    var stkFailed by remember { mutableStateOf(false) }
    var lastSale by rememberSaveable(stateSaver = JsonObjectSaver) { mutableStateOf<JSONObject?>(null) }
    var lastPayment by rememberSaveable(stateSaver = JsonObjectSaver) { mutableStateOf<JSONObject?>(null) }
    var lastCompleted by rememberSaveable { mutableStateOf(false) }
    var checkoutMode by rememberSaveable { mutableStateOf(false) }
    var cashReceived by rememberSaveable { mutableStateOf("") }
    var loadingCatalog by remember { mutableStateOf(true) }
    var showBarcodeScanner by remember { mutableStateOf(false) }

    // Catalog line price edit (bargain) — mirrors web POS edit-price panel.
    var editingVariantId by rememberSaveable { mutableStateOf<String?>(null) }
    var editPrice by rememberSaveable { mutableStateOf("") }
    var editReason by rememberSaveable { mutableStateOf("") }
    var editError by remember { mutableStateOf<String?>(null) }

    // Quick sale: an item not in stock, sourced from a supplier on the spot. The
    // customer only ever sees description + sell price on the receipt — unitCost/
    // supplierId are internal-only (supplier credit ledger + margin).
    var suppliers by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var quickSaleLines by rememberSaveable(stateSaver = JsonListSaver) { mutableStateOf<List<JSONObject>>(emptyList()) }
    var showQuickSale by rememberSaveable { mutableStateOf(false) }
    var qsDescription by rememberSaveable { mutableStateOf("") }
    var qsPrice by rememberSaveable { mutableStateOf("") }
    var qsQty by rememberSaveable { mutableStateOf("1") }
    var qsSourced by rememberSaveable { mutableStateOf(false) }
    var qsSupplierId by rememberSaveable { mutableStateOf("") }
    var qsCost by rememberSaveable { mutableStateOf("") }
    var qsError by rememberSaveable { mutableStateOf<String?>(null) }

    // Quick STK: skip the cart entirely — just an amount and a phone number for a
    // customer in a hurry. Rides the same checkout path as a one-line quick sale.
    var showQuickStk by rememberSaveable { mutableStateOf(false) }
    var qStkAmount by rememberSaveable { mutableStateOf("") }
    var qStkPhone by rememberSaveable { mutableStateOf("") }
    var qStkError by rememberSaveable { mutableStateOf<String?>(null) }

    fun catalogById(id: String): JSONObject? = catalog.firstOrNull { it.optString("variant_id") == id }

    fun addQuickSaleLine() {
        val description = qsDescription.trim()
        val price = qsPrice.toDoubleOrNull()
        val qty = qsQty.toIntOrNull()
        if (description.isBlank()) { qsError = "Describe the item"; return }
        if (price == null || price <= 0) { qsError = "Enter a positive sell price"; return }
        if (qty == null || qty <= 0) { qsError = "Enter a positive quantity"; return }
        val line = JSONObject()
            .put("local_id", java.util.UUID.randomUUID().toString())
            .put("description", description)
            .put("unit_price", price)
            .put("quantity", qty)
        if (qsSourced) {
            val cost = qsCost.toDoubleOrNull()
            if (qsSupplierId.isBlank()) { qsError = "Choose which supplier this came from"; return }
            if (cost == null || cost < 0) { qsError = "Enter what we paid the supplier"; return }
            line.put("unit_cost", cost)
            line.put("supplier_id", qsSupplierId)
            line.put("supplier_name", suppliers.firstOrNull { it.optString("id") == qsSupplierId }?.optString("name").orEmpty())
        }
        quickSaleLines = quickSaleLines + line
        qsError = null
        qsDescription = ""
        qsPrice = ""
        qsQty = "1"
        qsSourced = false
        qsSupplierId = ""
        qsCost = ""
        showQuickSale = false
    }

    fun refreshSales() {
        val branch = branchId ?: return
        scope.launch {
            runCatching {
                val items = withContext(Dispatchers.IO) { ApiClient.listSales(branch, 20) }
                sales = (0 until items.length()).map { items.getJSONObject(it) }
            }
        }
    }

    fun reloadCatalog() {
        scope.launch {
            loadingCatalog = true
            try {
                val products = withContext(Dispatchers.IO) { ApiClient.listCatalog(locationId) }
                catalog = (0 until products.length()).map { products.getJSONObject(it) }
            } catch (e: Exception) {
                error = e.message
            } finally {
                loadingCatalog = false
            }
        }
    }

    fun load() {
        val branch = branchId ?: return
        scope.launch {
            try {
                val locs = withContext(Dispatchers.IO) { ApiClient.listStockLocations(branch) }
                locations = (0 until locs.length()).map { locs.getJSONObject(it) }
                val counter = locations.firstOrNull {
                    it.optString("location_type") == "counter"
                }
                val valid = locations.any { it.optString("id") == locationId }
                if (!valid) {
                    locationId = counter?.optString("id") ?: locations.firstOrNull()?.optString("id")
                }
                val settings = withContext(Dispatchers.IO) {
                    runCatching { ApiClient.getPaymentSettings() }.getOrNull()
                }
                mpesaConfigured = settings?.optBoolean("configured") == true
                bankConfigured = settings?.optBoolean("bank_configured") == true
                mpesaShortcode = settings?.optString("mpesa_shortcode").orEmpty()
                bankPaybill = settings?.optString("bank_paybill").orEmpty()
                bankAccount = settings?.optString("bank_account").orEmpty()
                if (!mpesaConfigured && method.startsWith("mpesa")) method = "cash"
                if (!bankConfigured && method == "bank_paybill") method = "cash"
                if (!tenderInitialized) {
                    method = if (mpesaConfigured) "mpesa_stk" else "cash"
                    tenderInitialized = true
                }
                val products = withContext(Dispatchers.IO) { ApiClient.listCatalog(locationId) }
                catalog = (0 until products.length()).map { products.getJSONObject(it) }
                val supplierItems = withContext(Dispatchers.IO) {
                    runCatching { ApiClient.listSuppliers() }.getOrNull()
                }
                if (supplierItems != null) {
                    suppliers = (0 until supplierItems.length()).map { supplierItems.getJSONObject(it) }
                }
                error = null
            } catch (e: Exception) {
                error = e.message
            }
        }
        refreshSales()
    }

    suspend fun printSaleReceipt(saleId: String) {
        val html = withContext(Dispatchers.IO) {
            PrintSupport.fetchText(
                ApiClient.saleReceiptHtmlUrl(saleId),
                TechLaneApp.instance.tokenStore.accessToken,
            )
        }
        PrintSupport.printHtml(context, html, "Sale receipt")
    }

    suspend fun runCheckout(branch: String, location: String, lines: List<JSONObject>, chargeMethod: String, chargePhone: String?) {
        busy = true
        waitMessage = if (chargeMethod == "mpesa_stk") "Sending STK…" else "Processing sale…"
        waitDetail = if (chargeMethod == "mpesa_stk") "Prompting ${chargePhone?.trim()} on M-Pesa" else null
        error = null
        stkFailed = false
        try {
            val result = withContext(Dispatchers.IO) {
                ApiClient.posCheckout(branch, location, lines, chargeMethod, chargePhone, null)
            }
            lastSale = result.optJSONObject("sale")
            lastPayment = result.optJSONObject("payment")
            lastCompleted = result.optBoolean("completed")
            if (lastCompleted) {
                cart = emptyList()
                quickSaleLines = emptyList()
                editingVariantId = null
                reloadCatalog()
                lastSale?.optString("id")?.takeIf { it.isNotBlank() }?.let { id ->
                    runCatching { printSaleReceipt(id) }
                }
            }
            refreshSales()
        } catch (e: Exception) {
            error = e.message
            lastSale = null
            lastPayment = null
            lastCompleted = false
        } finally {
            busy = false
            waitDetail = null
        }
    }

    fun addToCart(item: JSONObject) {
        val id = item.optString("variant_id")
        val max = item.optInt("available_qty")
        if (max <= 0) return
        if (cart.isEmpty()) cashReceived = ""
        val listPrice = item.optDouble("sell_price", 0.0)
        val idx = cart.indexOfFirst { it.variantId == id }
        cart = if (idx >= 0) {
            val line = cart[idx]
            cart.toMutableList().also {
                it[idx] = line.copy(qty = (line.qty + 1).coerceAtMost(max))
            }
        } else {
            cart + PosCartLine(variantId = id, qty = 1, listPrice = listPrice)
        }
        lastSale = null
        lastPayment = null
        lastCompleted = false
        stkFailed = false
        error = null
    }

    fun setQty(variantId: String, qty: Int) {
        cart = if (qty <= 0) {
            if (editingVariantId == variantId) editingVariantId = null
            cart.filterNot { it.variantId == variantId }
        } else {
            cart.map { if (it.variantId == variantId) it.copy(qty = qty) else it }
        }
    }

    fun beginEditPrice(line: PosCartLine) {
        editingVariantId = line.variantId
        editPrice = (line.overridePrice ?: line.listPrice).let { p ->
            if (p == p.toLong().toDouble()) p.toLong().toString() else p.toString()
        }
        editReason = line.overrideReason
        editError = null
    }

    fun applyEditPrice(variantId: String) {
        val price = editPrice.toDoubleOrNull()
        val reason = editReason.trim()
        if (price == null || price <= 0) {
            editError = "Enter a positive price"
            return
        }
        if (reason.isBlank()) {
            editError = "Reason is required when changing price"
            return
        }
        cart = cart.map { line ->
            if (line.variantId != variantId) line
            else if (kotlin.math.abs(price - line.listPrice) <= 0.009) {
                line.copy(overridePrice = null, overrideReason = "")
            } else {
                line.copy(overridePrice = price, overrideReason = reason)
            }
        }
        editingVariantId = null
        editError = null
    }

    fun isTerminalStkError(message: String): Boolean {
        val m = message.lowercase()
        return listOf("1032", "1037", "cancelled", "canceled", "timeout", "ds timeout").any { it in m }
    }

    LaunchedEffect(branchId) { load() }
    LaunchedEffect(locationId) {
        if (locationId != null) reloadCatalog()
    }

    val filtered = catalog.filter {
        query.isBlank() || listOf(
            it.optString("product_name"),
            it.optString("sku"),
            it.optString("brand"),
            it.optString("barcode"),
        ).any { value -> value.contains(query, ignoreCase = true) }
    }
    val cartLines = cart.filter { it.qty > 0 }.mapNotNull { line ->
        catalogById(line.variantId)?.let { item -> item to line }
    }
    val quickSaleCount = quickSaleLines.sumOf { it.optInt("quantity") }
    val quickSaleTotal = quickSaleLines.sumOf { it.optDouble("unit_price") * it.optInt("quantity") }
    val cartItemCount = cartLines.sumOf { it.second.qty } + quickSaleCount
    val total = cartLines.sumOf { (_, line) -> line.unitPrice() * line.qty } + quickSaleTotal
    val cashAmount = cashReceived.toDoubleOrNull() ?: 0.0
    val changeDue = (cashAmount - total).coerceAtLeast(0.0)

    fun paymentStatusLabel(payment: JSONObject?, completed: Boolean): String {
        val status = payment?.optString("status").orEmpty()
        val payMethod = payment?.optString("method").orEmpty()
        return when {
            stkFailed -> "STK failed or cancelled"
            completed && payMethod == "cash" && status in setOf("confirmed", "allocated") ->
                "Paid · cash recorded"
            completed -> "Sale complete"
            payMethod == "mpesa_stk" && stkPolling -> "Waiting for customer PIN…"
            payMethod == "mpesa_stk" -> "STK sent — waiting for payment"
            payMethod == "mpesa_c2b" -> "Waiting for paybill"
            else -> "Awaiting payment"
        }
    }

    suspend fun finishStkSale(saleId: String, paymentId: String, location: String): Boolean {
        val reconcile = withContext(Dispatchers.IO) {
            runCatching { ApiClient.reconcileMpesa(paymentId) }
        }
        if (reconcile.isFailure) {
            val msg = reconcile.exceptionOrNull()?.message.orEmpty()
            val pay = withContext(Dispatchers.IO) {
                runCatching { ApiClient.getPayment(paymentId) }.getOrNull()
            }
            if (pay != null) lastPayment = pay
            val status = pay?.optString("status").orEmpty()
            if (status in listOf("failed", "cancelled") || isTerminalStkError(msg)) {
                stkFailed = true
                error = msg.ifBlank { "STK failed or cancelled" }
                return false
            }
            return false
        }
        val pay = reconcile.getOrThrow()
        lastPayment = pay
        val status = pay.optString("status")
        if (status in listOf("failed", "cancelled")) {
            stkFailed = true
            error = "STK $status"
            return false
        }
        if (status != "allocated" && status != "confirmed") return false
        val completed = withContext(Dispatchers.IO) {
            ApiClient.completeSale(saleId, location)
        }
        lastSale = completed
        lastCompleted = true
        snackbarHostState?.let { host -> scope.launch { host.showSnackbar("Sale completed") } }
        stkFailed = false
        cart = emptyList()
        quickSaleLines = emptyList()
        editingVariantId = null
        reloadCatalog()
        refreshSales()
        runCatching { printSaleReceipt(saleId) }
        return true
    }

    LaunchedEffect(lastPayment?.optString("id"), lastCompleted, locationId, stkFailed) {
        val payment = lastPayment ?: return@LaunchedEffect
        val sale = lastSale ?: return@LaunchedEffect
        val location = locationId ?: return@LaunchedEffect
        if (lastCompleted || stkFailed || payment.optString("method") != "mpesa_stk") return@LaunchedEffect
        val paymentId = payment.optString("id")
        val saleId = sale.optString("id")
        if (paymentId.isBlank() || saleId.isBlank()) return@LaunchedEffect
        stkPolling = true
        waitMessage = "Waiting for M-Pesa"
        waitDetail = "Ask the customer to enter their PIN on the phone.\nReceipt prints only after payment succeeds."
        try {
            repeat(48) {
                delay(2500)
                try {
                    if (finishStkSale(saleId, paymentId, location)) return@LaunchedEffect
                    if (stkFailed) return@LaunchedEffect
                } catch (e: Exception) {
                    if (isTerminalStkError(e.message.orEmpty())) {
                        stkFailed = true
                        error = e.message
                        return@LaunchedEffect
                    }
                }
            }
            if (!lastCompleted && !stkFailed) {
                error = "STK timed out — ask customer to retry or confirm manually"
            }
        } finally {
            stkPolling = false
            waitDetail = null
        }
    }

    val showWait = busy || stkPolling

    Box(
        modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background),
    ) {
        Column(Modifier.fillMaxSize()) {
            BrandHero(
                title = "Sell",
                subtitle = if (bankConfigured) {
                    "Stock · quick sale · STK · paybill"
                } else {
                    "Tap to add · cart · charge"
                },
                appLabel = "Ops",
                bottomContent = {
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        com.techlane.core.ui.HeroStat(
                            "Total",
                            "KES ${total.toInt()}",
                            Modifier.weight(1f),
                        )
                        com.techlane.core.ui.HeroStat(
                            "Items",
                            cartItemCount.toString(),
                            Modifier.weight(1f),
                        )
                    }
                },
            )
            OpsShellChrome()
            Column(
                modifier = Modifier
                    .weight(1f)
                    .verticalScroll(rememberScrollState())
                    .padding(horizontal = 16.dp, vertical = 12.dp),
                verticalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                if (branchId == null) {
                    Text("Select a branch before selling.", color = Brand.Danger)
                }

                if (!showQuickStk) {
                    OutlinedButton(
                        onClick = { showQuickStk = true },
                        modifier = Modifier.fillMaxWidth(),
                    ) { Text("Quick STK — amount only") }
                } else {
                    FormSection("Quick STK") {
                        Text(
                            "Skip the cart — just send an STK prompt for an amount and phone number.",
                            style = MaterialTheme.typography.bodySmall,
                            color = Brand.TextSecondary,
                        )
                        OutlinedTextField(
                            value = qStkAmount,
                            onValueChange = { qStkAmount = it },
                            label = { Text("Amount (KES)") },
                            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
                            singleLine = true,
                            modifier = Modifier.fillMaxWidth(),
                        )
                        OutlinedTextField(
                            value = qStkPhone,
                            onValueChange = { qStkPhone = it },
                            label = { Text("Customer phone") },
                            placeholder = { Text("07XXXXXXXX") },
                            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Phone),
                            singleLine = true,
                            modifier = Modifier.fillMaxWidth(),
                        )
                        qStkError?.let { Text(it, color = Brand.Danger) }
                        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                            GoldButton(
                                text = if (busy) "Sending…" else "Send STK",
                                onClick = {
                                    val branch = branchId
                                    val location = locationId
                                    val amount = qStkAmount.toDoubleOrNull()
                                    val stkPhone = qStkPhone.trim()
                                    when {
                                        branch == null -> qStkError = "Select a branch"
                                        location == null -> qStkError = "Select a stock location"
                                        amount == null || amount <= 0 -> qStkError = "Enter a positive amount"
                                        stkPhone.isBlank() -> qStkError = "Enter the customer's phone number"
                                        else -> {
                                            qStkError = null
                                            val line = JSONObject()
                                                .put("description", "Quick payment")
                                                .put("quantity", 1)
                                                .put("unit_price", amount)
                                            scope.launch {
                                                runCheckout(branch, location, listOf(line), "mpesa_stk", stkPhone)
                                                showQuickStk = false
                                                qStkAmount = ""
                                                qStkPhone = ""
                                            }
                                        }
                                    }
                                },
                                enabled = !showWait,
                                loading = busy && !stkPolling,
                                modifier = Modifier.weight(1f),
                            )
                            OutlinedButton(
                                onClick = { showQuickStk = false; qStkError = null },
                                modifier = Modifier.weight(1f),
                            ) { Text("Cancel") }
                        }
                    }
                }

                Row(horizontalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.fillMaxWidth()) {
                    FilterChip(
                        selected = !checkoutMode,
                        onClick = { checkoutMode = false },
                        label = { Text("Products") },
                        modifier = Modifier.weight(1f),
                    )
                    FilterChip(
                        selected = checkoutMode,
                        onClick = { if (cartItemCount > 0) checkoutMode = true },
                        enabled = cartItemCount > 0,
                        label = { Text("Cart ($cartItemCount)") },
                        modifier = Modifier.weight(1f),
                    )
                }
                if (!checkoutMode && cartItemCount > 0) {
                    Surface(
                        shape = androidx.compose.foundation.shape.RoundedCornerShape(16.dp),
                        color = Brand.Navy,
                        modifier = Modifier.fillMaxWidth(),
                    ) {
                        Row(Modifier.padding(14.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                            Column(Modifier.weight(1f)) {
                                Text("$cartItemCount items", color = Color.White.copy(alpha = 0.75f))
                                Text("KES " + total.toInt(), color = Color.White, fontWeight = FontWeight.Bold)
                            }
                            Button(onClick = { checkoutMode = true }) { Text("View cart") }
                        }
                    }
                }

                if (!checkoutMode) {
                if (locations.size > 1) {
                    Row(
                        Modifier.horizontalScroll(rememberScrollState()),
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                    ) {
                        locations.forEach { location ->
                            val id = location.optString("id")
                            FilterChip(
                                selected = id == locationId,
                                onClick = {
                                    if (cart.isNotEmpty()) {
                                        error = "Finish or clear the current cart before changing stock location"
                                    } else {
                                        locationId = id
                                    }
                                },
                                label = { Text(location.optString("name", "Stock")) },
                                colors = FilterChipDefaults.filterChipColors(
                                    selectedContainerColor = Brand.NavyTint,
                                    selectedLabelColor = Brand.Navy,
                                ),
                            )
                        }
                    }
                }

                OutlinedTextField(
                    value = query,
                    onValueChange = { query = it },
                    label = { Text("Search products") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                )
                OutlinedButton(
                    onClick = { showBarcodeScanner = !showBarcodeScanner },
                    modifier = Modifier.fillMaxWidth(),
                ) { Text(if (showBarcodeScanner) "Close scanner" else "Scan barcode") }
                if (showBarcodeScanner) {
                    ScanCameraPanel(
                        enabled = !showWait,
                        onCode = { code ->
                            val item = catalog.firstOrNull {
                                it.optString("barcode") == code || it.optString("sku").equals(code, ignoreCase = true)
                            }
                            if (item == null) {
                                error = "No product matches barcode $code"
                            } else {
                                addToCart(item)
                                showBarcodeScanner = false
                                context.showAppToast("Added ${item.optString("product_name")}")
                            }
                        },
                    )
                }

                if (!showQuickSale) {
                    OutlinedButton(
                        onClick = { showQuickSale = true },
                        modifier = Modifier.fillMaxWidth(),
                    ) { Text("+ Quick sale (item not in stock)") }
                } else {
                    FormSection("Quick sale") {
                        Text(
                            "For something you don't stock but sourced on the spot. The customer only ever sees the " +
                                "description and sell price — supplier and cost stay internal.",
                            style = MaterialTheme.typography.bodySmall,
                            color = Brand.TextSecondary,
                        )
                        OutlinedTextField(
                            value = qsDescription,
                            onValueChange = { qsDescription = it },
                            label = { Text("What are you selling?") },
                            modifier = Modifier.fillMaxWidth(),
                        )
                        OutlinedTextField(
                            value = qsPrice,
                            onValueChange = { qsPrice = it },
                            label = { Text("Sell price (KES)") },
                            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
                            singleLine = true,
                            modifier = Modifier.fillMaxWidth(),
                        )
                        OutlinedTextField(
                            value = qsQty,
                            onValueChange = { qsQty = it },
                            label = { Text("Quantity") },
                            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                            singleLine = true,
                            modifier = Modifier.fillMaxWidth(),
                        )
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            androidx.compose.material3.Checkbox(checked = qsSourced, onCheckedChange = { qsSourced = it })
                            Text("Sourced from a supplier (track what we owe them)")
                        }
                        if (qsSourced) {
                            if (suppliers.isEmpty()) {
                                Text("No suppliers yet — add one under Suppliers first.", color = Brand.TextSecondary)
                            } else {
                                Text("Supplier", style = MaterialTheme.typography.labelLarge)
                                Row(
                                    Modifier.horizontalScroll(rememberScrollState()),
                                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                                ) {
                                    suppliers.forEach { s ->
                                        val sid = s.optString("id")
                                        FilterChip(
                                            selected = qsSupplierId == sid,
                                            onClick = { qsSupplierId = sid },
                                            label = { Text(s.optString("name")) },
                                        )
                                    }
                                }
                            }
                            OutlinedTextField(
                                value = qsCost,
                                onValueChange = { qsCost = it },
                                label = { Text("What we paid them (KES, per item)") },
                                supportingText = { Text("Internal only — supplier credit ledger and margin, never on the receipt.") },
                                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
                                singleLine = true,
                                modifier = Modifier.fillMaxWidth(),
                            )
                        }
                        qsError?.let { Text(it, color = Brand.Danger) }
                        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                            GoldButton(text = "Add to cart", onClick = { addQuickSaleLine() }, modifier = Modifier.weight(1f))
                            OutlinedButton(
                                onClick = { showQuickSale = false; qsError = null },
                                modifier = Modifier.weight(1f),
                            ) { Text("Cancel") }
                        }
                    }
                }

                if (loadingCatalog) {
                    Text("Loading products…", color = Brand.TextSecondary)
                } else if (filtered.isEmpty()) {
                    Text("No products at this location.", color = Brand.TextSecondary)
                } else {
                    filtered.forEach { item ->
                        val id = item.optString("variant_id")
                        val inCart = cart.firstOrNull { it.variantId == id }?.qty ?: 0
                        val available = item.optInt("available_qty")
                        Surface(
                            onClick = { if (available > 0 && !showWait) addToCart(item) },
                            enabled = available > 0 && !showWait,
                            shape = androidx.compose.foundation.shape.RoundedCornerShape(14.dp),
                            color = if (inCart > 0) Brand.NavyTint else Brand.Surface,
                            border = androidx.compose.foundation.BorderStroke(
                                1.dp,
                                if (inCart > 0) Brand.Navy.copy(alpha = 0.35f) else Brand.Border,
                            ),
                            modifier = Modifier.fillMaxWidth(),
                        ) {
                            Row(
                                Modifier.padding(horizontal = 14.dp, vertical = 12.dp),
                                verticalAlignment = Alignment.CenterVertically,
                            ) {
                                Column(Modifier.weight(1f)) {
                                    Text(
                                        item.optString("product_name"),
                                        fontWeight = FontWeight.SemiBold,
                                        color = Brand.TextPrimary,
                                    )
                                    Text(
                                        "${item.optString("sku")} · $available left",
                                        color = Brand.TextSecondary,
                                        style = MaterialTheme.typography.bodySmall,
                                    )
                                }
                                Text(
                                    "KES ${item.optDouble("sell_price").toInt()}",
                                    fontWeight = FontWeight.Bold,
                                    color = Brand.Navy,
                                )
                                if (inCart > 0) {
                                    Spacer(modifier.padding(start = 8.dp))
                                    PillBadge("×$inCart", Brand.Navy)
                                } else {
                                    Text("  ADD", color = Brand.Navy, fontWeight = FontWeight.Bold, style = MaterialTheme.typography.labelMedium)
                                }
                            }
                        }
                    }
                }
                }

                if (checkoutMode) {
                if (cartLines.isNotEmpty()) {
                    BrandSectionTitle("Cart")
                    cartLines.forEach { (item, line) ->
                        val id = line.variantId
                        val qty = line.qty
                        val max = item.optInt("available_qty")
                        val bargained = line.isBargained()
                        val unit = line.unitPrice()
                        Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                            Row(
                                Modifier.fillMaxWidth(),
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(8.dp),
                            ) {
                                Column(Modifier.weight(1f)) {
                                    Text(item.optString("product_name"), fontWeight = FontWeight.Medium)
                                    Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                                        if (bargained) {
                                            Text(
                                                "KES ${(line.listPrice * qty).toInt()}",
                                                color = Brand.TextMuted,
                                                style = MaterialTheme.typography.bodySmall.copy(
                                                    textDecoration = TextDecoration.LineThrough,
                                                ),
                                            )
                                        }
                                        Text(
                                            "KES ${(unit * qty).toInt()}",
                                            color = Brand.TextSecondary,
                                            style = MaterialTheme.typography.bodySmall,
                                        )
                                    }
                                    if (bargained) {
                                        Text(
                                            "Was KES ${line.listPrice.toInt()} · now KES ${unit.toInt()} each",
                                            color = Brand.TextMuted,
                                            style = MaterialTheme.typography.labelSmall,
                                        )
                                    }
                                }
                                OutlinedButton(
                                    onClick = { setQty(id, qty - 1) },
                                    enabled = !showWait,
                                ) { Text("−") }
                                Text("$qty", fontWeight = FontWeight.Bold)
                                OutlinedButton(
                                    onClick = { setQty(id, (qty + 1).coerceAtMost(max)) },
                                    enabled = !showWait && qty < max,
                                ) { Text("+") }
                            }
                            Row(
                                Modifier.fillMaxWidth(),
                                horizontalArrangement = Arrangement.spacedBy(8.dp),
                                verticalAlignment = Alignment.CenterVertically,
                            ) {
                                if (canOverridePrice) {
                                    TextButton(
                                        onClick = { beginEditPrice(line) },
                                        enabled = !showWait,
                                    ) { Text("Edit price") }
                                }
                                if (bargained) {
                                    TextButton(
                                        onClick = {
                                            cart = cart.map {
                                                if (it.variantId == id) it.copy(overridePrice = null, overrideReason = "")
                                                else it
                                            }
                                            if (editingVariantId == id) editingVariantId = null
                                        },
                                        enabled = !showWait,
                                    ) { Text("Reset price") }
                                }
                            }
                            if (editingVariantId == id) {
                                OutlinedTextField(
                                    value = editPrice,
                                    onValueChange = { editPrice = it.filter { ch -> ch.isDigit() || ch == '.' } },
                                    label = { Text("New price (KES)") },
                                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
                                    singleLine = true,
                                    enabled = !showWait,
                                    modifier = Modifier.fillMaxWidth(),
                                )
                                OutlinedTextField(
                                    value = editReason,
                                    onValueChange = { editReason = it },
                                    label = { Text("Reason") },
                                    placeholder = { Text("e.g. Regular customer discount") },
                                    singleLine = true,
                                    enabled = !showWait,
                                    modifier = Modifier.fillMaxWidth(),
                                )
                                editError?.let { Text(it, color = Brand.Danger, style = MaterialTheme.typography.bodySmall) }
                                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                    GoldButton(
                                        text = "Apply",
                                        onClick = { applyEditPrice(id) },
                                        enabled = !showWait,
                                    )
                                    OutlinedButton(
                                        onClick = { editingVariantId = null; editError = null },
                                        enabled = !showWait,
                                    ) { Text("Cancel") }
                                }
                            }
                        }
                    }
                }

                if (quickSaleLines.isNotEmpty()) {
                    if (cartLines.isEmpty()) BrandSectionTitle("Cart")
                    quickSaleLines.forEach { line ->
                        val localId = line.optString("local_id")
                        val qty = line.optInt("quantity")
                        Row(
                            Modifier.fillMaxWidth(),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(8.dp),
                        ) {
                            Column(Modifier.weight(1f)) {
                                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                                    Text(line.optString("description"), fontWeight = FontWeight.Medium)
                                    PillBadge("Quick sale", Brand.GoldDark)
                                }
                                Text(
                                    "KES ${(line.optDouble("unit_price") * qty).toInt()} · qty $qty",
                                    color = Brand.TextSecondary,
                                    style = MaterialTheme.typography.bodySmall,
                                )
                                if (line.has("supplier_id")) {
                                    val margin = (line.optDouble("unit_price") - line.optDouble("unit_cost")) * qty
                                    Text(
                                        "Internal — from ${line.optString("supplier_name").ifBlank { "supplier" }} @ KES ${line.optDouble("unit_cost").toInt()} each · margin KES ${margin.toInt()}",
                                        color = Brand.TextMuted,
                                        style = MaterialTheme.typography.labelSmall,
                                    )
                                }
                            }
                            OutlinedButton(
                                onClick = { quickSaleLines = quickSaleLines.filterNot { it.optString("local_id") == localId } },
                                enabled = !showWait,
                            ) { Text("Remove") }
                        }
                    }
                }

                BrandSectionTitle("Payment")
                Row(
                    Modifier.horizontalScroll(rememberScrollState()),
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    listOf(
                        "cash" to "Cash",
                        "mpesa_stk" to if (bankConfigured) "STK → bank" else "M-Pesa STK",
                        "mpesa_c2b" to "Paybill",
                        "bank_paybill" to "Bank manual",
                    ).forEach { (key, label) ->
                        val enabled = when (key) {
                            "mpesa_stk", "mpesa_c2b" -> mpesaConfigured
                            "bank_paybill" -> bankConfigured
                            else -> true
                        }
                        FilterChip(
                            selected = method == key,
                            onClick = { if (enabled && !showWait) method = key },
                            enabled = enabled && !showWait,
                            label = { Text(label) },
                            colors = FilterChipDefaults.filterChipColors(
                                selectedContainerColor = Brand.NavyTint,
                                selectedLabelColor = Brand.Navy,
                            ),
                        )
                    }
                }

                if (method == "cash") {
                    OutlinedTextField(
                        value = cashReceived,
                        onValueChange = { cashReceived = it.filter { ch -> ch.isDigit() || ch.code == 46 } },
                        label = { Text("Cash received (KES)") },
                        supportingText = {
                            if (cashAmount >= total && total > 0) Text("Change · KES ${changeDue.toInt()}")
                            else Text("Enter the amount handed over")
                        },
                        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
                        singleLine = true,
                        enabled = !showWait,
                        modifier = Modifier.fillMaxWidth(),
                    )
                }

                if (method == "mpesa_stk") {
                    OutlinedTextField(
                        value = phone,
                        onValueChange = { phone = it.filter { ch -> ch.isDigit() }.take(12) },
                        label = { Text("Customer phone") },
                        placeholder = { Text("07XXXXXXXX") },
                        singleLine = true,
                        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Phone),
                        enabled = !showWait,
                        modifier = Modifier.fillMaxWidth(),
                    )
                    if (bankConfigured) {
                        Text(
                            "Uses M-Pesa credentials · routes to bank $bankPaybill / $bankAccount",
                            color = Brand.TextSecondary,
                            style = MaterialTheme.typography.bodySmall,
                        )
                    }
                }
                if (method == "mpesa_c2b" && mpesaShortcode.isNotBlank()) {
                    Text(
                        "Customer pays $mpesaShortcode using the sale account ref.",
                        color = Brand.TextSecondary,
                        style = MaterialTheme.typography.bodySmall,
                    )
                }

                GoldButton(
                    text = "Charge KES ${total.toInt()}",
                    onClick = {
                        val branch = branchId
                        val location = locationId
                        val catalogJson = cart.filter { it.qty > 0 }.map { line ->
                            val obj = JSONObject()
                                .put("variant_id", line.variantId)
                                .put("quantity", line.qty)
                            if (line.isBargained()) {
                                obj.put("override_price", line.overridePrice)
                                obj.put(
                                    "override_reason",
                                    line.overrideReason.ifBlank { "Price override" },
                                )
                            }
                            obj
                        }
                        val quickSaleJson = quickSaleLines.map { line ->
                            val obj = JSONObject()
                                .put("description", line.optString("description"))
                                .put("quantity", line.optInt("quantity"))
                                .put("unit_price", line.optDouble("unit_price"))
                            if (line.has("supplier_id")) {
                                obj.put("unit_cost", line.optDouble("unit_cost"))
                                obj.put("supplier_id", line.optString("supplier_id"))
                            }
                            obj
                        }
                        val lines = catalogJson + quickSaleJson
                        when {
                            branch == null -> error = "Select a branch"
                            location == null -> error = "Select a stock location"
                            lines.isEmpty() -> error = "Add an item"
                            method == "mpesa_stk" && phone.isBlank() -> error = "Phone required for STK"
                            method == "cash" && cashAmount < total -> error = "Cash received is less than the sale total"
                            else -> scope.launch { runCheckout(branch, location, lines, method, phone.ifBlank { null }) }
                        }
                    },
                    enabled = !showWait && cartItemCount > 0 && total > 0,
                    loading = busy && !stkPolling,
                    modifier = Modifier.fillMaxWidth(),
                )

                val sale = lastSale
                val payment = lastPayment
                if (sale != null && payment != null) {
                    val tone = when {
                        stkFailed -> Brand.Danger
                        lastCompleted -> Brand.Success
                        else -> Brand.Warning
                    }
                    Surface(
                        shape = androidx.compose.foundation.shape.RoundedCornerShape(16.dp),
                        color = tone.copy(alpha = 0.08f),
                        border = androidx.compose.foundation.BorderStroke(1.dp, tone.copy(alpha = 0.28f)),
                        modifier = Modifier.fillMaxWidth(),
                    ) {
                        Column(
                            Modifier.padding(16.dp),
                            verticalArrangement = Arrangement.spacedBy(8.dp),
                        ) {
                            Text(
                                paymentStatusLabel(payment, lastCompleted),
                                fontWeight = FontWeight.Bold,
                                color = tone,
                            )
                            Text(
                                "KES ${sale.optDouble("total").toInt()} · ${payment.optString("method").replace('_', ' ')}",
                                color = Brand.TextSecondary,
                                style = MaterialTheme.typography.bodySmall,
                            )
                            if (lastCompleted && payment.optString("method") == "cash") {
                                Text(
                                    "Cash has been added to this cashier’s till balance.",
                                    color = Brand.TextSecondary,
                                    style = MaterialTheme.typography.bodySmall,
                                )
                            }
                            if (!lastCompleted && !stkFailed && payment.optString("method") == "mpesa_stk") {
                                OutlinedButton(
                                    onClick = {
                                        val location = locationId ?: return@OutlinedButton
                                        busy = true
                                        waitMessage = "Checking STK…"
                                        waitDetail = null
                                        error = null
                                        scope.launch {
                                            try {
                                                if (!finishStkSale(
                                                        sale.getString("id"),
                                                        payment.getString("id"),
                                                        location,
                                                    )
                                                ) {
                                                    if (!stkFailed) {
                                                        error = error
                                                            ?: "Not paid yet — customer must finish on phone"
                                                    }
                                                }
                                            } catch (e: Exception) {
                                                error = e.message
                                                if (isTerminalStkError(e.message.orEmpty())) stkFailed = true
                                            } finally {
                                                busy = false
                                            }
                                        }
                                    },
                                    enabled = !showWait,
                                    modifier = Modifier.fillMaxWidth(),
                                ) { Text("Check payment now") }
                            }
                            if (!lastCompleted && !stkFailed && payment.optString("method") == "mpesa_c2b") {
                                OutlinedButton(
                                    onClick = {
                                        val location = locationId ?: return@OutlinedButton
                                        busy = true
                                        waitMessage = "Checking paybill…"
                                        error = null
                                        scope.launch {
                                            try {
                                                val pay = withContext(Dispatchers.IO) {
                                                    ApiClient.getPayment(payment.getString("id"))
                                                }
                                                val status = pay.optString("status")
                                                if (status != "allocated" && status != "confirmed") {
                                                    error("Waiting for paybill confirmation")
                                                }
                                                val completed = withContext(Dispatchers.IO) {
                                                    ApiClient.completeSale(sale.getString("id"), location)
                                                }
                                                lastSale = completed
                                                lastPayment = pay
                                                lastCompleted = true
                                                snackbarHostState?.let { host -> scope.launch { host.showSnackbar("Sale completed") } }
                                                cart = emptyList()
                                                quickSaleLines = emptyList()
                                                editingVariantId = null
                                                reloadCatalog()
                                                refreshSales()
                                                runCatching { printSaleReceipt(sale.getString("id")) }
                                            } catch (e: Exception) {
                                                error = e.message
                                            } finally {
                                                busy = false
                                            }
                                        }
                                    },
                                    enabled = !showWait,
                                    modifier = Modifier.fillMaxWidth(),
                                ) { Text("Check paybill") }
                            }
                            if (lastCompleted) {
                                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                    GoldButton(
                                        text = "Print receipt",
                                        onClick = {
                                            scope.launch {
                                                busy = true
                                                waitMessage = "Preparing receipt…"
                                                try {
                                                    printSaleReceipt(sale.getString("id"))
                                                } catch (e: Exception) {
                                                    error = e.message
                                                } finally {
                                                    busy = false
                                                }
                                            }
                                        },
                                        enabled = !showWait,
                                        modifier = Modifier.weight(1f),
                                    )
                                    OutlinedButton(
                                        onClick = {
                                            scope.launch {
                                                try {
                                                    val html = withContext(Dispatchers.IO) {
                                                        PrintSupport.fetchText(
                                                            ApiClient.saleReceiptHtmlUrl(sale.getString("id")),
                                                            TechLaneApp.instance.tokenStore.accessToken,
                                                        )
                                                    }
                                                    PrintSupport.shareHtml(context, html, "Sale receipt")
                                                } catch (e: Exception) {
                                                    error = e.message
                                                }
                                            }
                                        },
                                        enabled = !showWait,
                                    ) { Text("Share") }
                                }
                                OutlinedButton(
                                    onClick = {
                                        lastSale = null
                                        lastPayment = null
                                        lastCompleted = false
                                        stkFailed = false
                                        cashReceived = ""
                                        phone = ""
                                        checkoutMode = false
                                        query = ""
                                    },
                                    modifier = Modifier.fillMaxWidth(),
                                ) { Text("New sale") }
                            }
                            if (stkFailed) {
                                Text(
                                    "No receipt — payment did not complete. Start a new sale when ready.",
                                    color = Brand.Danger,
                                    style = MaterialTheme.typography.bodySmall,
                                )
                            }
                        }
                    }
                }

                FeedbackBanner(message = null, error = error)
                }

                val completedSales = sales.filter { it.optString("status") == "completed" }
                if (completedSales.isNotEmpty()) {
                    BrandSectionTitle("Recent completed")
                    completedSales.take(8).forEach { s ->
                        val saleId = s.optString("id")
                        Row(
                            Modifier.fillMaxWidth(),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Column(Modifier.weight(1f)) {
                                Text(
                                    "${saleId.take(8)} · KES ${s.optDouble("total").toInt()}",
                                    fontWeight = FontWeight.Medium,
                                )
                                Text(
                                    s.optString("created_at").take(16).replace('T', ' '),
                                    color = Brand.TextSecondary,
                                    style = MaterialTheme.typography.bodySmall,
                                )
                            }
                            TextButton(
                                onClick = {
                                    scope.launch {
                                        try {
                                            printSaleReceipt(saleId)
                                        } catch (e: Exception) {
                                            error = e.message
                                        }
                                    }
                                },
                                enabled = !showWait,
                            ) { Text("Receipt") }
                        }
                    }
                }
            }
        }

        PleaseWaitOverlay(
            visible = showWait,
            message = waitMessage,
            detail = waitDetail,
        )
    }
}

@Composable
fun InventoryLookupScreen(branchId: String?, modifier: Modifier = Modifier) {
    var query by remember { mutableStateOf("") }
    var locationId by remember { mutableStateOf<String?>(null) }
    var locations by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var balances by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var error by remember { mutableStateOf<String?>(null) }
    var loading by remember { mutableStateOf(true) }

    LaunchedEffect(branchId, locationId) {
        if (branchId == null) return@LaunchedEffect
        loading = true
        try {
            val locs = withContext(Dispatchers.IO) { ApiClient.listStockLocations(branchId) }
            locations = (0 until locs.length()).map { locs.getJSONObject(it) }
            if (locationId != null && locations.none { it.optString("id") == locationId }) locationId = null
            val items = withContext(Dispatchers.IO) { ApiClient.listInventoryBalances(locationId) }
            balances = (0 until items.length()).map { items.getJSONObject(it) }
            error = null
        } catch (e: Exception) {
            error = e.message
        } finally {
            loading = false
        }
    }
    val filtered = balances.filter {
        query.isBlank() || it.optString("product_name").contains(query, true) || it.optString("sku").contains(query, true)
    }
    Column(
        modifier = modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background),
    ) {
        BrandHero(
            title = "Inventory",
            subtitle = "Check available, physical, and reserved quantities.",
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
        Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            FilterChip(
                selected = locationId == null,
                onClick = { locationId = null },
                label = { Text("All") },
                colors = FilterChipDefaults.filterChipColors(
                    selectedContainerColor = Brand.NavyTint,
                    selectedLabelColor = Brand.Navy,
                ),
            )
            locations.forEach {
                val id = it.optString("id")
                FilterChip(
                    selected = locationId == id,
                    onClick = { locationId = id },
                    label = { Text(it.optString("name")) },
                    colors = FilterChipDefaults.filterChipColors(
                        selectedContainerColor = Brand.NavyTint,
                        selectedLabelColor = Brand.Navy,
                    ),
                )
            }
        }
         OutlinedTextField(query, { query = it }, label = { Text("Search product or SKU") }, modifier = Modifier.fillMaxWidth())
        if (loading) Text("Loading stock…", color = Brand.TextSecondary)
        val lowStockCount = filtered.count { it.optInt("available_qty") <= 3 }
        if (lowStockCount > 0) {
            Text(
                "$lowStockCount item${if (lowStockCount == 1) "" else "s"} running low (3 or fewer available).",
                color = Brand.Warning,
                style = MaterialTheme.typography.bodySmall,
                fontWeight = FontWeight.SemiBold,
            )
        }
        filtered.forEach { item ->
            val available = item.optInt("available_qty")
            val low = available <= 3
            BrandCard {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                ) {
                    Text(item.optString("product_name"), style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold)
                    if (low) PillBadge("Low stock", Brand.Warning)
                }
                Text("${item.optString("sku")} · ${item.optString("location_name")}", color = Brand.TextSecondary)
                Text(
                    "Available $available · Physical ${item.optInt("physical_qty")} · Reserved ${item.optInt("reserved_qty")}",
                    color = if (low) Brand.Warning else Brand.TextMuted,
                    style = MaterialTheme.typography.bodySmall,
                )
            }
        }
        if (filtered.isEmpty() && error == null) {
            EmptyHint(
                message = "Try another location or search term.",
                title = "No stock matches",
            )
        }
        FeedbackBanner(message = null, error = error)
        }
    }
}

@Composable
fun C2BExceptionsScreen(modifier: Modifier = Modifier) {
    var items by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var payments by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var selectedPayment by remember { mutableStateOf<Map<String, String>>(emptyMap()) }
    var loading by remember { mutableStateOf(true) }
    var busyId by remember { mutableStateOf<String?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var message by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()

    fun refresh() {
        scope.launch {
            try {
                val result = withContext(Dispatchers.IO) { ApiClient.listC2B("unmatched") }
                items = (0 until result.length()).map { result.getJSONObject(it) }
                val payResult = withContext(Dispatchers.IO) { ApiClient.listPayments() }
                payments = (0 until payResult.length()).map { payResult.getJSONObject(it) }
                    .filter { it.optString("status") in setOf("pending", "initiated", "provisional") }
                error = null
            } catch (e: Exception) {
                error = e.message
            } finally {
                loading = false
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
            title = "Unmatched C2B",
            subtitle = "Match received paybill money to an existing pending payment.",
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
        items.forEach { item ->
            val id = item.optString("id")
            val amount = item.optDouble("amount")
            val exact = payments.filter { kotlin.math.abs(it.optDouble("amount") - amount) < 0.01 }
            val candidates = (exact + payments).distinctBy { it.optString("id") }.take(6)
            BrandCard {
                Text(
                    "KES ${item.optDouble("amount").toInt()} · ${item.optString("trans_id")}",
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.Bold,
                )
                Text("Ref ${item.optString("bill_ref_number")} · ${item.optString("msisdn")}", color = Brand.TextSecondary)
                Text("Choose the pending payment", style = MaterialTheme.typography.labelMedium, color = Brand.TextSecondary)
                if (candidates.isEmpty()) {
                    Text("No pending payments are available. Create or locate the payment first.", color = Brand.Warning)
                } else {
                    candidates.forEach { payment ->
                        val pid = payment.optString("id")
                        val exactAmount = kotlin.math.abs(payment.optDouble("amount") - amount) < 0.01
                        FilterChip(
                            selected = selectedPayment[id] == pid,
                            onClick = { selectedPayment = selectedPayment + (id to pid) },
                            label = { Text("KES " + payment.optDouble("amount").toInt() + " · " + payment.optString("method").replace(95.toChar(), 32.toChar()) + if (exactAmount) " · exact" else "") },
                            modifier = Modifier.fillMaxWidth(),
                        )
                    }
                }
                GoldButton(
                    text = if (busyId == id) "Matching…" else "Confirm match",
                    onClick = {
                        val target = selectedPayment[id]
                        if (target.isNullOrBlank()) {
                            error = "Choose the payment this money belongs to"
                        } else {
                            busyId = id
                            scope.launch {
                                try {
                                    withContext(Dispatchers.IO) { ApiClient.matchC2B(id, target) }
                                    message = "C2B transaction matched"
                                    selectedPayment = selectedPayment - id
                                    refresh()
                                } catch (e: Exception) {
                                    error = e.message
                                } finally {
                                    busyId = null
                                }
                            }
                        }
                    },
                    enabled = busyId == null && selectedPayment[id] != null,
                    loading = busyId == id,
                    modifier = Modifier.fillMaxWidth(),
                )
            }
        }
        if (loading) { Text("Loading unmatched payments…", color = Brand.TextSecondary) }
        if (items.isEmpty() && error == null) {
            EmptyHint(
                message = "Paybill exceptions that need a payment match will appear here.",
                title = "No unmatched C2B",
            )
        }
        FeedbackBanner(message = message, error = error)
        }
    }
}

@Composable
fun ManualScanScreen(
    onOpenJob: (String) -> Unit = {},
    modifier: Modifier = Modifier,
) {
    var mode by remember { mutableStateOf("auto") }
    var showModes by remember { mutableStateOf(false) }
    var code by remember { mutableStateOf("") }
    var relatedId by remember { mutableStateOf("") }
    var results by remember { mutableStateOf<List<JSONObject>>(emptyList()) }
    var lookedUpJob by remember { mutableStateOf<JSONObject?>(null) }
    var message by remember { mutableStateOf<String?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var scanning by remember { mutableStateOf(true) }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    val context = LocalContext.current

    fun runLookup(
        lookupCode: String = code,
        lookupMode: String = mode,
        lookupRelatedId: String = relatedId,
    ) {
        val value = lookupCode.trim()
        if (value.isBlank()) {
            error = "Scan or enter a code"
            context.showAppToast("Scan or enter a code")
            return
        }
        if (busy) return
        scope.launch {
            busy = true
            scanning = false
            try {
                error = null
                message = null
                results = emptyList()
                lookedUpJob = null
                val resolvedMode = when {
                    lookupMode != "auto" -> lookupMode
                    else -> parseScanPayload(value).mode ?: "imei"
                }
                when (resolvedMode) {
                    "imei" -> {
                        val array = withContext(Dispatchers.IO) { ApiClient.listRepairs(q = value) }
                        results = (0 until array.length()).map { array.getJSONObject(it) }
                        message = if (results.isEmpty()) "No repairs match that code" else "${results.size} repair(s) found"
                        context.showAppToast(message!!)
                    }
                    "barcode" -> {
                        val array = withContext(Dispatchers.IO) { ApiClient.listCatalog() }
                        results = (0 until array.length()).map { array.getJSONObject(it) }
                            .filter {
                                it.optString("sku").contains(value, true) ||
                                    it.optString("barcode").contains(value, true) ||
                                    it.optString("product_name").contains(value, true)
                            }
                        message = if (results.isEmpty()) "No catalog match" else "${results.size} catalog item(s)"
                        context.showAppToast(message!!)
                    }
                    "auth" -> {
                        if (lookupRelatedId.isBlank()) {
                            error = "Auth QR must include the supplier issue id — scan the full auth QR"
                            context.showAppToast(error!!)
                        } else {
                            withContext(Dispatchers.IO) { ApiClient.collectSupplierIssue(lookupRelatedId, value) }
                            message = "Supplier issue collected"
                            context.showAppToast(message!!, long = true)
                        }
                    }
                    "collection", "repair_pickup" -> {
                        val isRepairPickup = resolvedMode == "repair_pickup" ||
                            value.startsWith("PK-", ignoreCase = true) ||
                            value.contains("repair-pickup", ignoreCase = true)
                        if (isRepairPickup) {
                            val job = withContext(Dispatchers.IO) {
                                ApiClient.lookupRepairByPickupCode(value)
                            }
                            lookedUpJob = job
                            results = listOf(job)
                            val status = job.optString("status").replace('_', ' ')
                            val canRelease = job.optBoolean("can_release", false)
                            val balance = job.optDouble("balance_due", 0.0)
                            message = when {
                                canRelease ->
                                    "Job ${job.optString("job_code")} · $status — ready to release"
                                balance > 0.01 ->
                                    "Job ${job.optString("job_code")} · $status — balance KES ${balance.toInt()} still due"
                                else ->
                                    "Job ${job.optString("job_code")} · $status — not ready for release yet"
                            }
                            context.showAppToast(message!!, long = true)
                        } else {
                            val order = withContext(Dispatchers.IO) { ApiClient.collectOnlineOrder(value) }
                            results = listOf(order)
                            message = "Order ${order.optString("id").take(8)} collected"
                            context.showAppToast(message!!, long = true)
                        }
                    }
                    else -> {
                        val array = withContext(Dispatchers.IO) { ApiClient.listRepairs(q = value) }
                        results = (0 until array.length()).map { array.getJSONObject(it) }
                        message = if (results.isEmpty()) "No match for $value" else "${results.size} result(s)"
                        context.showAppToast(message!!)
                    }
                }
            } catch (e: Exception) {
                error = e.message ?: "Lookup failed"
                context.showAppToast(error!!, long = true)
            } finally {
                busy = false
                delay(700)
                scanning = true
            }
        }
    }

    fun onScanned(raw: String) {
        val parsed = parseScanPayload(raw)
        if (parsed.code.isBlank()) return
        if (parsed.mode != null) {
            mode = parsed.mode!!
        }
        code = parsed.code
        parsed.relatedId?.let { relatedId = it }
        error = null
        results = emptyList()
        lookedUpJob = null
        val nextMode = parsed.mode ?: mode
        val nextRelated = parsed.relatedId ?: relatedId
        if (nextMode == "auth" && nextRelated.isBlank()) {
            error = "Auth code loaded — scan the full supplier auth QR"
            context.showAppToast(error!!)
            return
        }
        context.showAppToast("Scanned ${parsed.code}")
        runLookup(
            lookupCode = parsed.code,
            lookupMode = parsed.mode ?: mode,
            lookupRelatedId = nextRelated,
        )
    }

    Column(
        modifier = modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background),
    ) {
        BrandHero(
            title = "Scan",
            subtitle = "Point at an intake QR, IMEI, auth code, or barcode.",
            appLabel = "Ops",
        )
        OpsShellChrome()
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp)
                .padding(top = 8.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            OutlinedButton(onClick = { showModes = !showModes }, modifier = Modifier.fillMaxWidth()) {
                Text(if (showModes) "Hide scan types" else "Scan type · " + mode.replace(95.toChar(), 32.toChar()))
            }
            if (showModes) {
            Row(
                modifier = Modifier.horizontalScroll(rememberScrollState()),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                listOf(
                    "auto" to "Auto",
                    "repair_pickup" to "Repair pickup",
                    "imei" to "IMEI / job",
                    "barcode" to "Barcode",
                    "auth" to "Auth",
                    "collection" to "Order pickup",
                ).forEach { (key, label) ->
                    FilterChip(
                        selected = mode == key,
                        onClick = {
                            mode = key
                            results = emptyList()
                            lookedUpJob = null
                            message = null
                            error = null
                        },
                        label = { Text(label) },
                        colors = FilterChipDefaults.filterChipColors(
                            selectedContainerColor = Brand.NavyTint,
                            selectedLabelColor = Brand.Navy,
                        ),
                    )
                }
            }
            }
            ScanCameraPanel(
                enabled = scanning && !busy,
                onCode = { value -> onScanned(value) },
            )
        }
        Column(
            modifier = Modifier
                .weight(1f)
                .verticalScroll(rememberScrollState())
                .padding(16.dp)
                .padding(bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            if (busy) {
                Text("Looking up…", color = Brand.Navy, fontWeight = FontWeight.SemiBold)
            }
            OutlinedTextField(
                value = code,
                onValueChange = { code = it.trim() },
                label = { Text("Code (auto-filled from scan)") },
                modifier = Modifier.fillMaxWidth(),
            )
            if (mode == "auth") {
                OutlinedTextField(
                    value = relatedId,
                    onValueChange = { relatedId = it.trim() },
                    label = { Text("Supplier issue UUID") },
                    modifier = Modifier.fillMaxWidth(),
                )
            }
            GoldButton(
                text = if (busy) "Looking up…" else "Look up code",
                onClick = { runLookup() },
                enabled = !busy && code.isNotBlank(),
                loading = busy,
                modifier = Modifier.fillMaxWidth(),
            )
            FeedbackBanner(message = message, error = error)

            lookedUpJob?.let { job ->
                BrandCard {
                    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        Text(
                            job.optString("job_code").ifBlank { job.optString("id").take(8) },
                            style = MaterialTheme.typography.titleMedium,
                            fontWeight = FontWeight.Bold,
                        )
                        Text(
                            "${job.optString("status").replace('_', ' ')} · ${job.optString("problem_summary")}",
                            color = Brand.TextSecondary,
                        )
                        job.optJSONObject("customer")?.optString("full_name")?.takeIf { it.isNotBlank() }?.let {
                            Text(it, style = MaterialTheme.typography.bodyMedium)
                        }
                        val canRelease = job.optBoolean("can_release", false)
                        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.fillMaxWidth()) {
                            OutlinedButton(
                                onClick = { onOpenJob(job.optString("id")) },
                                modifier = Modifier.weight(1f),
                                enabled = !busy && job.optString("id").isNotBlank(),
                            ) {
                                Text("Open job")
                            }
                            if (canRelease) {
                                Button(
                                    onClick = {
                                        busy = true
                                        scope.launch {
                                            try {
                                                withContext(Dispatchers.IO) {
                                                    ApiClient.collectRepairByPickupCode(
                                                        job.optString("pickup_code").ifBlank { code },
                                                    )
                                                }
                                                message = "Device released"
                                                context.showAppToast("Device released", long = true)
                                                lookedUpJob = null
                                            } catch (e: Exception) {
                                                error = e.message
                                                context.showAppToast(e.message ?: "Release failed", long = true)
                                            } finally {
                                                busy = false
                                                scanning = true
                                            }
                                        }
                                    },
                                    enabled = !busy,
                                    modifier = Modifier.weight(1f),
                                ) {
                                    Text("Release")
                                }
                            }
                        }
                    }
                }
            }

            results.filter { lookedUpJob == null || it.optString("id") != lookedUpJob?.optString("id") }.forEach { result ->
                BrandCard {
                    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                        Text(
                            result.optString("job_code").ifBlank {
                                result.optString("product_name").ifBlank { result.optString("id").take(8) }
                            },
                            style = MaterialTheme.typography.titleMedium,
                            fontWeight = FontWeight.Bold,
                        )
                        Text(
                            result.optString("problem_summary").ifBlank {
                                result.optString("sku").ifBlank { result.optString("status") }
                            },
                            color = Brand.TextSecondary,
                        )
                        val jobId = result.optString("id")
                        if (jobId.isNotBlank() && result.has("problem_summary")) {
                            TextButton(onClick = { onOpenJob(jobId) }) {
                                Text("Open job")
                            }
                        }
                    }
                }
            }
        }
    }
}
