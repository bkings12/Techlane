package com.techlane.ops.network

import com.techlane.ops.BuildConfig
import com.techlane.ops.TechLaneApp
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject

object ApiClient {
    private val json = "application/json; charset=utf-8".toMediaType()
    private val client = OkHttpClient()
    private val refreshLock = Any()
    @Volatile private var sessionExpiredListener: (() -> Unit)? = null

    class SessionExpiredException(message: String = "Session expired. Please sign in again.") : Exception(message)

    fun setSessionExpiredListener(listener: (() -> Unit)?) {
        sessionExpiredListener = listener
    }

    /** Result of password login — either tokens, or an MFA challenge to complete. */
    sealed class LoginResult {
        data class Ok(val accessToken: String, val refreshToken: String) : LoginResult()
        data class MfaRequired(val challenge: String) : LoginResult()
    }

    fun login(email: String, password: String): LoginResult {
        val body = JSONObject(mapOf("email" to email, "password" to password)).toString()
            .toRequestBody(json)
        val req = Request.Builder()
            .url("${BuildConfig.API_BASE}/auth/login")
            .post(body)
            .build()
        return parseLoginOutcome(execute(req, authed = false))
    }

    fun verifyMfa(challenge: String, code: String): LoginResult.Ok {
        val body = JSONObject()
            .put("mfa_challenge", challenge)
            .put("code", code)
            .toString()
            .toRequestBody(json)
        val req = Request.Builder()
            .url("${BuildConfig.API_BASE}/auth/mfa/verify")
            .post(body)
            .build()
        val res = execute(req, authed = false)
        // MFA verify returns a flat TokenPair (not nested under "tokens").
        val access = res.optString("access_token")
        val refresh = res.optString("refresh_token")
        if (access.isBlank() || refresh.isBlank()) {
            error("Login failed: token not available")
        }
        return LoginResult.Ok(access, refresh)
    }

    private fun parseLoginOutcome(outcome: JSONObject): LoginResult {
        if (outcome.optBoolean("mfa_required")) {
            val challenge = outcome.optString("mfa_challenge")
            if (challenge.isBlank()) error("Login failed: MFA challenge missing")
            return LoginResult.MfaRequired(challenge)
        }
        // Prefer nested LoginOutcome.tokens; fall back to flat TokenPair for older APIs.
        val nested = outcome.optJSONObject("tokens")
        val access = nested?.optString("access_token")?.takeIf { it.isNotBlank() }
            ?: outcome.optString("access_token").takeIf { it.isNotBlank() }
        val refresh = nested?.optString("refresh_token")?.takeIf { it.isNotBlank() }
            ?: outcome.optString("refresh_token").takeIf { it.isNotBlank() }
        if (access.isNullOrBlank() || refresh.isNullOrBlank()) {
            error("Login failed: token not available")
        }
        return LoginResult.Ok(access, refresh)
    }

    /** Register/bind this staff phone for offline sync; persists server device id. */
    fun registerDevice(deviceName: String = android.os.Build.MODEL ?: "Android"): JSONObject {
        val localId = TechLaneApp.instance.tokenStore.deviceId
        val payload = JSONObject()
            .put("id", localId)
            .put("device_name", deviceName)
            .put("platform", "android")
            .put("fingerprint", localId)
        val res = post("/devices/register", payload)
        val serverId = res.optString("id").ifBlank { localId }
        TechLaneApp.instance.tokenStore.deviceId = serverId
        return res
    }

    fun reconcileMpesa(paymentId: String): JSONObject =
        post("/payments/$paymentId/mpesa/reconcile", JSONObject())

    fun get(path: String): JSONObject {
        val token = TechLaneApp.instance.tokenStore.accessToken ?: error("Not signed in")
        val req = Request.Builder()
            .url("${BuildConfig.API_BASE}$path")
            .header("Authorization", "Bearer $token")
            .get()
            .build()
        return execute(req, authed = true)
    }

    fun post(path: String, payload: JSONObject = JSONObject(), idempotencyKey: String? = null): JSONObject {
        val token = TechLaneApp.instance.tokenStore.accessToken ?: error("Not signed in")
        val builder = Request.Builder()
            .url("${BuildConfig.API_BASE}$path")
            .header("Authorization", "Bearer $token")
            .post(payload.toString().toRequestBody(json))
        if (idempotencyKey != null) builder.header("Idempotency-Key", idempotencyKey)
        return execute(builder.build(), authed = true)
    }

    fun delete(path: String): JSONObject {
        val token = TechLaneApp.instance.tokenStore.accessToken ?: error("Not signed in")
        val req = Request.Builder()
            .url("${BuildConfig.API_BASE}$path")
            .header("Authorization", "Bearer $token")
            .delete()
            .build()
        return execute(req, authed = true)
    }

    fun me(): JSONObject = get("/me")

    fun listBranches(): org.json.JSONArray =
        get("/branches").optJSONArray("items") ?: org.json.JSONArray()

    fun listStockLocations(branchId: String): org.json.JSONArray =
        get("/stock-locations?branch_id=$branchId").optJSONArray("items") ?: org.json.JSONArray()

    fun listCatalog(locationId: String? = null): org.json.JSONArray {
        val query = locationId?.let { "?location_id=$it" }.orEmpty()
        return get("/catalog$query").optJSONArray("items") ?: org.json.JSONArray()
    }

    fun listInventoryBalances(locationId: String? = null): org.json.JSONArray {
        val query = locationId?.let { "?location_id=$it" }.orEmpty()
        return get("/inventory/balances$query").optJSONArray("items") ?: org.json.JSONArray()
    }

    /** A catalog line ({variant_id, quantity}) or a quick-sale line — item not in
     * stock, sourced on the spot ({description, quantity, unit_price} plus optional
     * internal-only unit_cost/supplier_id that never reach the customer receipt). */
    fun posCheckout(
        branchId: String,
        locationId: String,
        items: List<JSONObject>,
        method: String,
        phone: String? = null,
        accountReference: String? = null,
    ): JSONObject {
        val lines = org.json.JSONArray()
        items.forEach { lines.put(it) }
        val payload = JSONObject()
            .put("branch_id", branchId)
            .put("location_id", locationId)
            .put("items", lines)
            .put("method", method)
        if (!phone.isNullOrBlank()) payload.put("phone", phone)
        if (!accountReference.isNullOrBlank()) payload.put("account_reference", accountReference)
        return post("/pos/checkout", payload, java.util.UUID.randomUUID().toString())
    }

    fun getPaymentSettings(): JSONObject = get("/payments/settings")

    fun getPayment(id: String): JSONObject = get("/payments/$id")

    fun listSales(branchId: String? = null, limit: Int = 25): org.json.JSONArray {
        val params = mutableListOf<String>()
        if (!branchId.isNullOrBlank()) params.add("branch_id=$branchId")
        if (limit > 0) params.add("limit=$limit")
        val qs = if (params.isEmpty()) "" else "?" + params.joinToString("&")
        return get("/sales$qs").optJSONArray("items") ?: org.json.JSONArray()
    }

    fun completeSale(saleId: String, locationId: String): JSONObject =
        post("/sales/$saleId/complete", JSONObject().put("location_id", locationId))

    fun reverseSale(saleId: String, locationId: String): JSONObject =
        post("/sales/$saleId/reverse", JSONObject().put("location_id", locationId))

    fun saleReceiptHtmlUrl(saleId: String): String =
        "${BuildConfig.API_BASE}/sales/$saleId/receipt.html"

    fun listC2B(status: String? = null): org.json.JSONArray {
        val query = status?.let { "?status=$it" }.orEmpty()
        return get("/payments/c2b$query").optJSONArray("items") ?: org.json.JSONArray()
    }

    fun matchC2B(id: String, paymentId: String): JSONObject =
        post("/payments/c2b/$id/match", JSONObject().put("payment_id", paymentId))

    fun pendingCashTotal(): Double = get("/cash/pending-total").optDouble("amount", 0.0)

    fun getRepair(id: String): JSONObject = get("/repairs/$id")

    fun listRepairs(status: String? = null, technicianId: String? = null, q: String? = null): org.json.JSONArray {
        val params = mutableListOf<String>()
        if (!status.isNullOrBlank()) params.add("status=$status")
        if (!technicianId.isNullOrBlank()) params.add("technician_id=$technicianId")
        if (!q.isNullOrBlank()) params.add("q=" + java.net.URLEncoder.encode(q, "UTF-8"))
        val qs = if (params.isEmpty()) "" else "?" + params.joinToString("&")
        return get("/repairs$qs").optJSONArray("items") ?: org.json.JSONArray()
    }

    fun createCustomer(fullName: String, phone: String? = null): JSONObject {
        val payload = JSONObject().put("full_name", fullName)
        if (!phone.isNullOrBlank()) payload.put("phone", phone)
        return post("/customers", payload)
    }

    fun universalSearch(q: String): org.json.JSONArray =
        get("/search?q=" + java.net.URLEncoder.encode(q, "UTF-8")).optJSONArray("results") ?: org.json.JSONArray()

    fun listNotifications(unackedOnly: Boolean = false): org.json.JSONArray =
        get("/notifications?unacked=" + unackedOnly).optJSONArray("items") ?: org.json.JSONArray()

    fun ackNotification(id: String) { post("/notifications/$id/ack", JSONObject()) }

    fun reserveInventory(variantId: String, locationId: String, quantity: Int, referenceType: String, referenceId: String, ttlSeconds: Int = 86400): JSONObject =
        post("/inventory/reserve", JSONObject().put("variant_id", variantId).put("location_id", locationId).put("quantity", quantity).put("reference_type", referenceType).put("reference_id", referenceId).put("ttl_seconds", ttlSeconds))

    fun listCustomers(q: String? = null): org.json.JSONArray {
        val qs = if (q.isNullOrBlank()) "" else "?q=" + java.net.URLEncoder.encode(q, "UTF-8")
        return get("/customers$qs").optJSONArray("items") ?: org.json.JSONArray()
    }

    fun createDevice(
        customerId: String?,
        kind: String,
        brand: String? = null,
        model: String? = null,
        imei: String? = null,
    ): JSONObject {
        val payload = JSONObject().put("kind", kind)
        if (!customerId.isNullOrBlank()) payload.put("customer_id", customerId) else payload.put("anonymous", true)
        if (!brand.isNullOrBlank()) payload.put("brand", brand)
        if (!model.isNullOrBlank()) payload.put("model", model)
        if (!imei.isNullOrBlank()) payload.put("imei", imei)
        return post("/devices", payload)
    }

    fun createRepair(
        branchId: String,
        deviceId: String,
        problemSummary: String,
        serviceType: String = "repair",
        customerId: String? = null,
        technicianId: String? = null,
        laborAmount: Double? = null,
        promisedBy: String? = null,
        customerWaiting: Boolean = false,
        estimatedWaitMinutes: Int? = null,
        customerCredit: Boolean = false,
        creditDueDate: String? = null,
        intakeAccessories: List<String> = emptyList(),
        intakeCondition: String? = null,
        conditionTags: List<String> = emptyList(),
        devicePasscode: String? = null,
    ): JSONObject {
        val payload = JSONObject()
            .put("branch_id", branchId)
            .put("device_id", deviceId)
            .put("problem_summary", problemSummary)
            .put("service_type", serviceType)
        if (!customerId.isNullOrBlank()) payload.put("customer_id", customerId)
        if (!technicianId.isNullOrBlank()) payload.put("technician_id", technicianId)
        if (laborAmount != null && laborAmount > 0) payload.put("labor_amount", laborAmount)
        if (!promisedBy.isNullOrBlank()) payload.put("promised_by", promisedBy)
        if (customerWaiting) {
            payload.put("customer_waiting", true)
            if (estimatedWaitMinutes != null && estimatedWaitMinutes > 0) {
                payload.put("estimated_wait_minutes", estimatedWaitMinutes)
            }
        }
        if (customerCredit) {
            payload.put("customer_credit", true)
            if (!creditDueDate.isNullOrBlank()) payload.put("credit_due_date", creditDueDate)
        }
        if (intakeAccessories.isNotEmpty()) {
            payload.put("intake_accessories", org.json.JSONArray(intakeAccessories))
        }
        if (!intakeCondition.isNullOrBlank()) payload.put("intake_condition", intakeCondition)
        if (conditionTags.isNotEmpty()) {
            payload.put("condition_tags", org.json.JSONArray(conditionTags))
        }
        if (!devicePasscode.isNullOrBlank()) payload.put("device_passcode", devicePasscode)
        return post("/repairs", payload)
    }

    fun assignRepair(id: String, technicianId: String): JSONObject {
        return post("/repairs/$id/assign", JSONObject().put("technician_id", technicianId))
    }

    fun revealRepairPasscode(id: String): String {
        return post("/repairs/$id/passcode/reveal", JSONObject()).getString("passcode")
    }

    fun listIntakePresets(kind: String): org.json.JSONArray {
        val q = java.net.URLEncoder.encode(kind, Charsets.UTF_8.name())
        return get("/intake-presets?kind=$q").optJSONArray("items") ?: org.json.JSONArray()
    }

    fun listRepairNotes(id: String): org.json.JSONArray {
        return get("/repairs/$id/notes").optJSONArray("items") ?: org.json.JSONArray()
    }

    fun addRepairNote(id: String, note: String): JSONObject {
        return post("/repairs/$id/notes", JSONObject().put("note", note))
    }

    fun listRepairEstimates(id: String): org.json.JSONArray {
        return get("/repairs/$id/estimates").optJSONArray("items") ?: org.json.JSONArray()
    }

    fun createRepairEstimate(
        id: String,
        totalAmount: Double,
        notes: String? = null,
    ): JSONObject {
        val payload = JSONObject().put("total_amount", totalAmount)
        if (!notes.isNullOrBlank()) payload.put("notes", notes)
        return post("/repairs/$id/estimates", payload)
    }

    fun listRepairAttachments(id: String): org.json.JSONArray =
        get("/repairs/$id/attachments").optJSONArray("items") ?: org.json.JSONArray()

    fun addRepairAttachment(id: String, fileName: String, contentType: String, dataBase64: String): JSONObject =
        post(
            "/repairs/$id/attachments",
            JSONObject()
                .put("file_name", fileName)
                .put("content_type", contentType)
                .put("data_base64", dataBase64),
        )

    fun reportSummary(days: Int = 1): JSONObject = get("/reports/summary?days=$days")

    fun updateRepairStatus(
        id: String,
        status: String,
        laborAmount: Double? = null,
        closureReason: String? = null,
        note: String? = null,
        varianceReason: String? = null,
    ): JSONObject {
        val payload = JSONObject().put("status", status)
        if (laborAmount != null) payload.put("labor_amount", laborAmount)
        if (!closureReason.isNullOrBlank()) payload.put("closure_reason", closureReason)
        if (!note.isNullOrBlank()) payload.put("note", note)
        if (!varianceReason.isNullOrBlank()) payload.put("variance_reason", varianceReason)
        return post("/repairs/$id/status", payload)
    }

    fun authorizeRepairWork(id: String, amount: Double?, note: String): JSONObject {
        val payload = JSONObject().put("note", note)
        if (amount != null) payload.put("amount", amount)
        return post("/repairs/$id/authorize-work", payload)
    }

    fun sendHandoverCode(id: String): JSONObject = post("/repairs/$id/handover/send-code", JSONObject())

    fun recordHandover(
        id: String,
        collectedByName: String,
        relationship: String,
        idNumber: String?,
        note: String?,
        otpCode: String?,
        pickupCode: String? = null,
    ): JSONObject {
        val payload = JSONObject()
            .put("collected_by_name", collectedByName)
            .put("relationship", relationship)
        if (!idNumber.isNullOrBlank()) payload.put("id_number", idNumber)
        if (!note.isNullOrBlank()) payload.put("note", note)
        if (!otpCode.isNullOrBlank()) payload.put("otp_code", otpCode)
        if (!pickupCode.isNullOrBlank()) payload.put("pickup_code", pickupCode)
        return post("/repairs/$id/handover", payload)
    }

    fun collectRepairByPickupCode(
        pickupCode: String,
        collectedByName: String = "Customer",
        relationship: String = "self",
    ): JSONObject {
        return post(
            "/repairs/collect",
            JSONObject()
                .put("pickup_code", pickupCode.trim())
                .put("collected_by_name", collectedByName)
                .put("relationship", relationship),
        )
    }

    fun createRepairRework(id: String, reason: String): JSONObject {
        return post("/repairs/$id/rework", JSONObject().put("reason", reason.trim()))
    }

    fun lookupRepairByPickupCode(pickupCode: String): JSONObject {
        val encoded = java.net.URLEncoder.encode(pickupCode.trim(), Charsets.UTF_8.name())
        return get("/repairs/by-pickup-code?code=$encoded")
    }

    fun listPartRequests(repairId: String): org.json.JSONArray {
        return get("/part-requests?repair_job_id=$repairId").optJSONArray("items")
            ?: org.json.JSONArray()
    }

    fun listSuppliers(): org.json.JSONArray {
        return get("/suppliers").optJSONArray("items") ?: org.json.JSONArray()
    }

    fun createPartRequest(
        repairJobId: String,
        branchId: String?,
        description: String,
        quantity: Int = 1,
        supplierId: String? = null,
    ): JSONObject {
        val payload = JSONObject()
            .put("repair_job_id", repairJobId)
            .put("description", description)
            .put("quantity", quantity)
        if (!branchId.isNullOrBlank()) payload.put("branch_id", branchId)
        if (!supplierId.isNullOrBlank()) payload.put("supplier_id", supplierId)
        return post("/part-requests", payload)
    }

    fun approvePartRequest(id: String, unitCost: Double): JSONObject {
        return post("/part-requests/$id/approve", JSONObject().put("unit_cost", unitCost))
    }

    fun collectSupplierIssue(id: String, authCode: String): JSONObject {
        return post("/supplier-issues/$id/collect", JSONObject().put("auth_code", authCode))
    }

    fun listRepairPayments(repairId: String): org.json.JSONArray {
        return get("/payments?payable_type=repair&payable_id=$repairId").optJSONArray("items")
            ?: org.json.JSONArray()
    }

    fun listPayments(): org.json.JSONArray =
        get("/payments").optJSONArray("items") ?: org.json.JSONArray()

    fun createPayment(
        method: String,
        amount: Double,
        payableType: String,
        payableId: String,
        branchId: String?,
        phone: String? = null,
        accountRef: String? = null,
    ): JSONObject {
        val payload = JSONObject()
            .put("method", method)
            .put("amount", amount)
            .put("payable_type", payableType)
            .put("payable_id", payableId)
            .put("currency", "KES")
        if (!branchId.isNullOrBlank()) payload.put("branch_id", branchId)
        if (!phone.isNullOrBlank()) payload.put("phone", phone)
        if (!accountRef.isNullOrBlank()) payload.put("account_reference", accountRef)
        return post("/payments", payload)
    }

    fun addRepairSaleLine(repairId: String, variantId: String, locationId: String, quantity: Int): JSONObject {
        return post(
            "/repairs/$repairId/sale-lines",
            JSONObject()
                .put("variant_id", variantId)
                .put("location_id", locationId)
                .put("quantity", quantity),
        )
    }

    fun removeRepairSaleLine(repairId: String, lineId: String) {
        delete("/repairs/$repairId/sale-lines/$lineId")
    }

    fun confirmMpesaPayment(id: String, providerRef: String = ""): JSONObject {
        // Typed provider_ref is ignored server-side; STK Query is the source of truth.
        return reconcileMpesa(id)
    }

    fun listCashHandovers(status: String? = null): org.json.JSONArray {
        val q = if (status.isNullOrBlank()) "" else "?status=$status"
        return get("/cash/handovers$q").optJSONArray("items") ?: org.json.JSONArray()
    }

    fun requestCashHandover(amount: Double, branchId: String?, toUserId: String? = null): JSONObject {
        val payload = JSONObject().put("amount", amount)
        if (!branchId.isNullOrBlank()) payload.put("branch_id", branchId)
        if (!toUserId.isNullOrBlank()) payload.put("to_user_id", toUserId)
        return post("/cash/handovers", payload)
    }

    fun confirmCashHandover(id: String, countedAmount: Double?): JSONObject {
        val payload = JSONObject()
        if (countedAmount != null) payload.put("counted_amount", countedAmount)
        return post("/cash/handovers/$id/confirm", payload)
    }

    fun listRefunds(status: String? = null): org.json.JSONArray {
        val q = if (status.isNullOrBlank()) "" else "?status=$status"
        return get("/refunds$q").optJSONArray("items") ?: org.json.JSONArray()
    }

    fun createRefund(paymentId: String, amount: Double, reason: String? = null): JSONObject {
        val payload = JSONObject()
            .put("payment_id", paymentId)
            .put("amount", amount)
        if (!reason.isNullOrBlank()) payload.put("reason", reason)
        return post("/refunds", payload)
    }

    fun approveRefund(id: String): JSONObject {
        return post("/refunds/$id/approve", JSONObject())
    }

    fun listOnlineOrders(status: String? = null): org.json.JSONArray {
        val q = if (status.isNullOrBlank()) "" else "?status=$status"
        return get("/commerce/orders$q").optJSONArray("items") ?: org.json.JSONArray()
    }

    fun collectOnlineOrder(collectionCode: String): JSONObject {
        return post("/commerce/collect", JSONObject().put("collection_code", collectionCode.trim().uppercase()))
    }

    private fun execute(req: Request, authed: Boolean, retried: Boolean = false): JSONObject {
        client.newCall(req).execute().use { res ->
            val text = res.body?.string().orEmpty()
            if (res.code == 401 && authed) {
                if (!retried && refreshAccessToken()) {
                    val token = TechLaneApp.instance.tokenStore.accessToken
                        ?: throw SessionExpiredException()
                    val retry = req.newBuilder()
                        .header("Authorization", "Bearer $token")
                        .build()
                    return execute(retry, authed = true, retried = true)
                }
                forceLogout()
                throw SessionExpiredException(errorMessage(text, res.message))
            }
            if (!res.isSuccessful) {
                error(errorMessage(text, res.message))
            }
            return if (text.isBlank()) JSONObject() else JSONObject(text)
        }
    }

    private fun errorMessage(text: String, fallback: String): String {
        return runCatching {
            JSONObject(text).optJSONObject("error")?.optString("message")
        }.getOrNull()?.takeIf { it.isNotBlank() } ?: text.ifBlank { fallback }
    }

    private fun refreshAccessToken(): Boolean {
        synchronized(refreshLock) {
            val refresh = TechLaneApp.instance.tokenStore.refreshToken ?: return false
            val body = JSONObject().put("refresh_token", refresh).toString().toRequestBody(json)
            val req = Request.Builder()
                .url("${BuildConfig.API_BASE}/auth/refresh")
                .post(body)
                .build()
            return try {
                client.newCall(req).execute().use { res ->
                    val text = res.body?.string().orEmpty()
                    if (!res.isSuccessful) return false
                    val tokens = JSONObject(text)
                    val access = tokens.optString("access_token")
                    val nextRefresh = tokens.optString("refresh_token")
                    if (access.isBlank() || nextRefresh.isBlank()) return false
                    TechLaneApp.instance.tokenStore.accessToken = access
                    TechLaneApp.instance.tokenStore.refreshToken = nextRefresh
                    true
                }
            } catch (_: Exception) {
                false
            }
        }
    }

    private fun forceLogout() {
        TechLaneApp.instance.tokenStore.clear()
        sessionExpiredListener?.invoke()
    }
}
