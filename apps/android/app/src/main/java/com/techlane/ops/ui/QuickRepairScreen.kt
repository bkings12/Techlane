package com.techlane.ops.ui

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.background
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.FilterChip
import androidx.compose.material3.FilterChipDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.techlane.core.PrintSupport
import com.techlane.core.theme.Brand
import com.techlane.core.ui.BrandHero
import com.techlane.core.ui.GoldButton
import com.techlane.core.ui.PleaseWaitOverlay
import com.techlane.ops.BuildConfig
import com.techlane.ops.TechLaneApp
import com.techlane.ops.network.ApiClient
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONObject

private val QUICK_REPAIR_STEPS = listOf("Customer", "Device", "Fix & amount", "Payment")

/**
 * Fast counter flow for a walk-in device that's fixed on the spot — no multi-day
 * tracking. A short wizard creates the customer/device/job, walks the job through
 * to completed, takes payment, hands the device back (collected), and prints the
 * receipt. Uses the same endpoints as full intake / job detail / POS.
 */
@Composable
fun QuickRepairScreen(branchId: String?, modifier: Modifier = Modifier) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    val snackbarHostState = LocalSnackbarHost.current

    var currentStep by rememberSaveable { mutableStateOf(0) }

    // Form inputs and the created job/payment survive rotation and process death —
    // losing a half-filled quick repair (or, worse, re-creating a job because the
    // screen forgot it already made one) is exactly the failure mode this guards.
    var customerName by rememberSaveable { mutableStateOf("") }
    var customerPhone by rememberSaveable { mutableStateOf("") }
    var kind by rememberSaveable { mutableStateOf("phone") }
    var brand by rememberSaveable { mutableStateOf("") }
    var model by rememberSaveable { mutableStateOf("") }
    var problem by rememberSaveable { mutableStateOf("") }
    var amount by rememberSaveable { mutableStateOf("") }

    var creating by remember { mutableStateOf(false) }
    var createError by rememberSaveable { mutableStateOf<String?>(null) }

    var job by rememberSaveable(stateSaver = JsonObjectSaver) { mutableStateOf<JSONObject?>(null) }
    var payment by rememberSaveable(stateSaver = JsonObjectSaver) { mutableStateOf<JSONObject?>(null) }
    var method by rememberSaveable { mutableStateOf("cash") }
    var payBusy by remember { mutableStateOf(false) }
    var payError by rememberSaveable { mutableStateOf<String?>(null) }
    var payMessage by rememberSaveable { mutableStateOf<String?>(null) }
    var printBusy by remember { mutableStateOf(false) }
    var stkPolling by remember { mutableStateOf(false) }

    val paid = payment?.let { it.optString("status") in setOf("confirmed", "allocated") || it.optString("method") == "cash" } == true

    fun reset() {
        currentStep = 0
        customerName = ""
        customerPhone = ""
        kind = "phone"
        brand = ""
        model = ""
        problem = ""
        amount = ""
        job = null
        payment = null
        method = "cash"
        createError = null
        payError = null
        payMessage = null
        stkPolling = false
    }

    // Steps 1-2 are plain local edits — step back freely. Once the job exists (step 3)
    // there's nothing to "undo" server-side, so Back gives way to New quick repair instead.
    BackHandler(enabled = currentStep in 1..2) { currentStep -= 1 }

    suspend fun printReceipt(jobId: String) {
        val html = withContext(Dispatchers.IO) {
            PrintSupport.fetchText(
                "${BuildConfig.API_BASE}/repairs/$jobId/receipt.html",
                TechLaneApp.instance.tokenStore.accessToken,
            )
        }
        PrintSupport.printHtml(context, html, "Repair receipt")
    }

    fun refreshPayment(jobId: String) {
        scope.launch {
            runCatching {
                val items = withContext(Dispatchers.IO) { ApiClient.listRepairPayments(jobId) }
                (0 until items.length()).map { items.getJSONObject(it) }
            }.getOrNull()?.lastOrNull()?.let { payment = it }
        }
    }

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

    suspend fun finishPaidHandover(jobId: String, current: JSONObject) {
        val pickup = current.optString("pickup_code").takeIf { it.isNotBlank() && it != "null" }
        if (!pickup.isNullOrBlank()) {
            runCatching {
                withContext(Dispatchers.IO) {
                    ApiClient.recordHandover(
                        id = jobId,
                        collectedByName = customerName.trim().ifBlank { "Customer" },
                        relationship = "self",
                        idNumber = null,
                        note = "Same-day counter fix — handed back at the till",
                        otpCode = null,
                        pickupCode = pickup,
                    )
                }
                job = withContext(Dispatchers.IO) { ApiClient.getRepair(jobId) }
            }.onFailure { e ->
                payMessage = "Paid, but handover failed: ${e.message}. Finish collection from the job."
            }.onSuccess {
                payMessage = "Paid and handed over"
                printBusy = true
                try { printReceipt(jobId) } finally { printBusy = false }
            }
        } else {
            payMessage = "Paid — finish handover from the job screen"
        }
        snackbarHostState?.let { host -> scope.launch { host.showSnackbar(payMessage ?: "Payment successful") } }
    }

    fun pollStkPayment(paymentId: String, jobId: String, current: JSONObject) {
        if (stkPolling || paymentId.isBlank()) return
        stkPolling = true
        payError = null
        scope.launch {
            try {
                repeat(48) {
                    delay(2500)
                    try {
                        val p = withContext(Dispatchers.IO) { ApiClient.confirmMpesaPayment(paymentId) }
                        payment = p
                        val status = p.optString("status")
                        if (status == "allocated" || status == "confirmed") {
                            finishPaidHandover(jobId, current)
                            return@launch
                        }
                        if (status == "failed" || status == "cancelled") {
                            payError = "STK $status"
                            return@launch
                        }
                    } catch (e: Exception) {
                        if (isTerminalStkError(e.message.orEmpty())) {
                            payError = e.message
                            return@launch
                        }
                    }
                }
                payError = "STK timed out — tap Check payment to try again"
                refreshPayment(jobId)
            } finally {
                stkPolling = false
            }
        }
    }

    fun createQuickJob() {
        val name = customerName.trim()
        val problemText = problem.trim()
        val value = amount.toDoubleOrNull()
        if (name.isBlank()) { createError = "Enter the customer's name"; return }
        if (problemText.isBlank()) { createError = "What did you fix?"; return }
        if (value == null || value <= 0) { createError = "Enter a positive amount"; return }
        if (branchId.isNullOrBlank()) { createError = "Select a branch first"; return }

        creating = true
        createError = null
        scope.launch {
            try {
                val jobResult = withContext(Dispatchers.IO) {
                    val customer = ApiClient.createCustomer(name, customerPhone.trim().ifBlank { null })
                    val customerId = customer.getString("id")
                    val device = ApiClient.createDevice(
                        customerId = customerId,
                        kind = kind,
                        brand = brand.trim().ifBlank { null },
                        model = model.trim().ifBlank { null },
                    )
                    val deviceId = device.getString("id")
                    val created = ApiClient.createRepair(
                        branchId = branchId,
                        deviceId = deviceId,
                        problemSummary = problemText,
                        serviceType = "repair",
                        customerId = customerId,
                        laborAmount = value,
                    )
                    val id = created.getString("id")
                    // Bench flow is linear — walk it straight through since the fix is already done.
                    ApiClient.updateRepairStatus(id, "in_progress")
                    ApiClient.updateRepairStatus(id, "ready_for_pickup")
                    ApiClient.updateRepairStatus(id, "completed")
                }
                job = jobResult
                currentStep = 3
            } catch (e: Exception) {
                createError = e.message ?: "Could not create the job"
            } finally {
                creating = false
            }
        }
    }

    fun takePayment() {
        val current = job ?: return
        val jobId = current.getString("id")
        val value = amount.toDoubleOrNull()
        if (value == null || value <= 0) { payError = "Enter a positive amount"; return }
        if (method == "mpesa_stk" && customerPhone.isBlank()) { payError = "Phone required for STK"; return }

        payBusy = true
        payError = null
        payMessage = null
        scope.launch {
            try {
                val created = withContext(Dispatchers.IO) {
                    ApiClient.createPayment(
                        method = method,
                        amount = value,
                        payableType = "repair",
                        payableId = jobId,
                        branchId = branchId,
                        phone = customerPhone.trim().ifBlank { null },
                        accountRef = current.optString("job_code").ifBlank { jobId.take(8) },
                    )
                }
                if (method == "cash") {
                    payMessage = "Cash recorded"
                    snackbarHostState?.let { host -> scope.launch { host.showSnackbar("Cash recorded") } }
                    refreshPayment(jobId)
                    // Same-visit counter fix: release with the intake pickup code so the
                    // job reaches collected (and does not sit forever as completed).
                    val pickup = current.optString("pickup_code").takeIf { it.isNotBlank() && it != "null" }
                    if (!pickup.isNullOrBlank()) {
                        runCatching {
                            withContext(Dispatchers.IO) {
                                ApiClient.recordHandover(
                                    id = jobId,
                                    collectedByName = customerName.trim().ifBlank { "Customer" },
                                    relationship = "self",
                                    idNumber = null,
                                    note = "Same-day counter fix — handed back at the till",
                                    otpCode = null,
                                    pickupCode = pickup,
                                )
                            }
                        }.onFailure { e ->
                            payMessage = "Paid, but handover failed: ${e.message}. Finish collection from the job."
                        }.onSuccess {
                            payMessage = "Paid and handed over"
                            // Refresh job so status shows collected.
                            runCatching {
                                job = withContext(Dispatchers.IO) { ApiClient.getRepair(jobId) }
                            }
                        }
                    }
                    printBusy = true
                    try {
                        printReceipt(jobId)
                    } finally {
                        printBusy = false
                    }
                } else {
                    payment = created
                    payMessage = "STK sent — waiting for PIN"
                    snackbarHostState?.let { host -> scope.launch { host.showSnackbar("STK sent to customer") } }
                    paymentIdFromCreate(created)?.let { pollStkPayment(it, jobId, current) }
                        ?: refreshPayment(jobId)
                }
            } catch (e: Exception) {
                payError = e.message ?: "Payment failed"
            } finally {
                payBusy = false
            }
        }
    }

    fun reconcile() {
        val p = payment ?: return
        val current = job ?: return
        val jobId = current.optString("id")
        pollStkPayment(p.getString("id"), jobId, current)
    }

    Box(modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        Column(Modifier.fillMaxSize()) {
            BrandHero(
                title = "Same-day counter fix",
                subtitle = "Fix now · take payment · hand over · print",
                appLabel = "Ops",
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
                    Text("Select a branch before starting.", color = Brand.Danger)
                }

                Column(verticalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.fillMaxWidth()) {
                    Text(
                        "Step ${currentStep + 1} of ${QUICK_REPAIR_STEPS.size} · ${QUICK_REPAIR_STEPS[currentStep]}",
                        fontWeight = FontWeight.SemiBold,
                        color = Brand.Navy,
                    )
                    Row(horizontalArrangement = Arrangement.spacedBy(6.dp), modifier = Modifier.fillMaxWidth()) {
                        QUICK_REPAIR_STEPS.indices.forEach { index ->
                            Surface(
                                modifier = Modifier.weight(1f).height(5.dp),
                                shape = RoundedCornerShape(99.dp),
                                color = if (index <= currentStep) Brand.Gold else Brand.Border,
                            ) {}
                        }
                    }
                }

                if (currentStep == 0) {
                    FormSection("Customer") {
                        OutlinedTextField(
                            value = customerName,
                            onValueChange = { customerName = it },
                            label = { Text("Full name") },
                            modifier = Modifier.fillMaxWidth(),
                        )
                        OutlinedTextField(
                            value = customerPhone,
                            onValueChange = { customerPhone = it },
                            label = { Text("Phone (required for STK)") },
                            modifier = Modifier.fillMaxWidth(),
                        )
                    }
                }

                if (currentStep == 1) {
                    FormSection("Device") {
                        Text("Device type", style = MaterialTheme.typography.labelMedium, color = Brand.TextSecondary)
                        Row(
                            modifier = Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()),
                            horizontalArrangement = Arrangement.spacedBy(8.dp),
                        ) {
                            listOf("phone" to "Phone", "laptop" to "Laptop", "tablet" to "Tablet", "other" to "Other")
                                .forEach { (key, label) ->
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
                        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.fillMaxWidth()) {
                            OutlinedTextField(
                                value = brand,
                                onValueChange = { brand = it },
                                label = { Text("Brand") },
                                modifier = Modifier.weight(1f),
                            )
                            OutlinedTextField(
                                value = model,
                                onValueChange = { model = it },
                                label = { Text("Model") },
                                modifier = Modifier.weight(1f),
                            )
                        }
                    }
                }

                if (currentStep == 2) {
                    FormSection("Fix + amount") {
                        Text(
                            "${customerName.ifBlank { "Walk-in" }} · ${listOf(brand, model).filter { it.isNotBlank() }.joinToString(" ")}".trim(),
                            color = Brand.TextSecondary,
                            style = MaterialTheme.typography.bodySmall,
                        )
                        OutlinedTextField(
                            value = problem,
                            onValueChange = { problem = it },
                            label = { Text("What did you fix?") },
                            modifier = Modifier.fillMaxWidth(),
                        )
                        OutlinedTextField(
                            value = amount,
                            onValueChange = { amount = it },
                            label = { Text("Amount (KES)") },
                            modifier = Modifier.fillMaxWidth(),
                        )
                    }
                    FeedbackBanner(message = null, error = createError)
                }

                if (currentStep < 3) {
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp), modifier = Modifier.fillMaxWidth()) {
                        if (currentStep > 0) {
                            OutlinedButton(
                                onClick = { currentStep -= 1 },
                                enabled = !creating,
                                modifier = Modifier.weight(1f),
                            ) { Text("Back") }
                        }
                        GoldButton(
                            text = when {
                                creating -> "Creating…"
                                currentStep == 2 -> "Create job"
                                else -> "Continue"
                            },
                            onClick = {
                                when (currentStep) {
                                    0 -> {
                                        if (customerName.trim().isBlank()) {
                                            createError = "Enter the customer's name"
                                            return@GoldButton
                                        }
                                        createError = null
                                        currentStep = 1
                                    }
                                    1 -> currentStep = 2
                                    2 -> createQuickJob()
                                }
                            },
                            enabled = !creating,
                            loading = creating,
                            modifier = Modifier.weight(1f),
                        )
                    }
                }

                if (currentStep == 3 && job != null) {
                    val j = job!!
                    FormSection("Job created") {
                        Text(
                            j.optString("job_code").ifBlank { j.optString("id").take(8) },
                            style = MaterialTheme.typography.titleLarge,
                        )
                        Text("${customerName.ifBlank { "Walk-in" }} · $brand $model".trim())
                        Text(problem)
                        Text("KES ${amount.toDoubleOrNull()?.toInt() ?: 0}", style = MaterialTheme.typography.titleMedium)
                    }

                    if (!paid) {
                        FormSection("Take payment") {
                            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                listOf("cash" to "Cash", "mpesa_stk" to "STK").forEach { (key, label) ->
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
                                    value = customerPhone,
                                    onValueChange = { customerPhone = it },
                                    label = { Text("Customer phone") },
                                    modifier = Modifier.fillMaxWidth(),
                                )
                            }
                            FeedbackBanner(message = payMessage, error = payError)
                            GoldButton(
                                text = when {
                                    payBusy || stkPolling -> "Working…"
                                    method == "mpesa_stk" -> "Send STK push"
                                    else -> "Mark paid"
                                },
                                onClick = { takePayment() },
                                enabled = !payBusy && !stkPolling,
                                loading = payBusy || stkPolling,
                                modifier = Modifier.fillMaxWidth(),
                            )
                            if (payment?.optString("method") == "mpesa_stk" &&
                                payment?.optString("status") in setOf("initiated", "pending")
                            ) {
                                OutlinedButton(
                                    onClick = { reconcile() },
                                    enabled = !payBusy && !stkPolling,
                                    modifier = Modifier.fillMaxWidth(),
                                ) { Text("Check payment") }
                            }
                        }
                    } else {
                        FormSection("Paid") {
                            Text("Payment recorded ✓", color = Brand.Success, style = MaterialTheme.typography.titleMedium)
                            GoldButton(
                                text = if (printBusy) "Opening printer…" else "Print receipt",
                                onClick = {
                                    printBusy = true
                                    scope.launch {
                                        try {
                                            printReceipt(j.getString("id"))
                                        } catch (e: Exception) {
                                            payError = e.message ?: "Could not print receipt"
                                        } finally {
                                            printBusy = false
                                        }
                                    }
                                },
                                enabled = !printBusy,
                                loading = printBusy,
                                modifier = Modifier.fillMaxWidth(),
                            )
                        }
                    }

                    OutlinedButton(onClick = { reset() }, modifier = Modifier.fillMaxWidth()) {
                        Text("New quick repair")
                    }
                }
            }
        }
        PleaseWaitOverlay(
            visible = stkPolling,
            message = "Waiting for M-Pesa",
            detail = "Ask the customer to enter their PIN.\nWe'll confirm and hand over when payment succeeds.",
        )
    }
}
