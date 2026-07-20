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

    fun login(email: String, password: String): JSONObject {
        val body = JSONObject(mapOf("email" to email, "password" to password)).toString()
            .toRequestBody(json)
        val req = Request.Builder()
            .url("${BuildConfig.API_BASE}/auth/login")
            .post(body)
            .build()
        return execute(req, authed = false)
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

    fun posCheckout(
        branchId: String,
        locationId: String,
        items: List<Pair<String, Int>>,
        method: String,
        phone: String? = null,
        accountReference: String? = null,
    ): JSONObject {
        val lines = org.json.JSONArray()
        items.forEach { (variantId, quantity) ->
            lines.put(JSONObject().put("variant_id", variantId).put("quantity", quantity))
        }
        val payload = JSONObject()
            .put("branch_id", branchId)
            .put("location_id", locationId)
            .put("items", lines)
            .put("method", method)
        if (!phone.isNullOrBlank()) payload.put("phone", phone)
        if (!accountReference.isNullOrBlank()) payload.put("account_reference", accountReference)
        return post("/pos/checkout", payload, java.util.UUID.randomUUID().toString())
    }

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
        customerId: String? = null,
        technicianId: String? = null,
    ): JSONObject {
        val payload = JSONObject()
            .put("branch_id", branchId)
            .put("device_id", deviceId)
            .put("problem_summary", problemSummary)
        if (!customerId.isNullOrBlank()) payload.put("customer_id", customerId)
        if (!technicianId.isNullOrBlank()) payload.put("technician_id", technicianId)
        return post("/repairs", payload)
    }

    fun assignRepair(id: String, technicianId: String): JSONObject {
        return post("/repairs/$id/assign", JSONObject().put("technician_id", technicianId))
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
        laborAmount: Double,
        partsAmount: Double,
        notes: String? = null,
    ): JSONObject {
        val payload = JSONObject()
            .put("labor_amount", laborAmount)
            .put("parts_amount", partsAmount)
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

    fun updateRepairStatus(id: String, status: String, laborAmount: Double? = null): JSONObject {
        val payload = JSONObject().put("status", status)
        if (laborAmount != null) payload.put("labor_amount", laborAmount)
        return post("/repairs/$id/status", payload)
    }

    fun listPartRequests(repairId: String): org.json.JSONArray {
        return get("/part-requests?repair_job_id=$repairId").optJSONArray("items")
            ?: org.json.JSONArray()
    }

    fun createPartRequest(repairJobId: String, branchId: String?, description: String, quantity: Int = 1): JSONObject {
        val payload = JSONObject()
            .put("repair_job_id", repairJobId)
            .put("description", description)
            .put("quantity", quantity)
        if (!branchId.isNullOrBlank()) payload.put("branch_id", branchId)
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
