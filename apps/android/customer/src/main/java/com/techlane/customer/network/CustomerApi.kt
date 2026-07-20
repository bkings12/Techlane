package com.techlane.customer.network

import com.techlane.core.network.ApiHttp
import com.techlane.customer.BuildConfig
import com.techlane.customer.CustomerApp
import org.json.JSONArray
import org.json.JSONObject

object CustomerApi {
    private var onSessionExpired: (() -> Unit)? = null

    private val http = ApiHttp(
        apiBase = BuildConfig.API_BASE,
        tokenProvider = { CustomerApp.instance.tokenStore.sessionToken },
        onUnauthorized = { onSessionExpired?.invoke() },
    )

    fun setSessionExpiredListener(listener: (() -> Unit)?) {
        onSessionExpired = listener
    }

    fun requestOtp(phone: String): JSONObject =
        http.post("/customer/auth/otp/request", JSONObject().put("phone", phone))

    fun verifyOtp(phone: String, code: String): JSONObject =
        http.post(
            "/customer/auth/otp/verify",
            JSONObject().put("phone", phone).put("code", code),
        )

    fun logout() {
        runCatching { http.post("/customer/auth/logout") }
    }

    fun me(): JSONObject = http.get("/customer/me")

    fun listRepairs(): JSONArray = http.getArray("/customer/repairs")

    fun repairDetail(id: String): JSONObject = http.get("/customer/repairs/$id")

    fun approveEstimate(repairId: String, estimateId: String): JSONObject =
        http.post("/customer/repairs/$repairId/estimates/$estimateId/approve")

    fun rejectEstimate(repairId: String, estimateId: String): JSONObject =
        http.post("/customer/repairs/$repairId/estimates/$estimateId/reject")

    fun payRepair(repairId: String, phone: String? = null): JSONObject {
        val body = JSONObject().put("method", "mpesa_stk")
        if (!phone.isNullOrBlank()) body.put("phone", phone)
        return http.post("/customer/repairs/$repairId/pay", body)
    }

    fun paymentStatus(repairId: String, paymentId: String): JSONObject =
        http.get("/customer/repairs/$repairId/payments/$paymentId")

    fun receipts(repairId: String): JSONArray =
        http.getArray("/customer/repairs/$repairId/receipts")

    fun warranty(repairId: String): JSONObject = http.get("/customer/repairs/$repairId/warranty")

    fun claimWarranty(repairId: String, note: String): JSONObject =
        http.post("/customer/repairs/$repairId/warranty/claim", JSONObject().put("note", note))

    fun receiptHtmlUrl(repairId: String): String =
        "${BuildConfig.API_BASE}/customer/repairs/$repairId/receipt.html"
}
