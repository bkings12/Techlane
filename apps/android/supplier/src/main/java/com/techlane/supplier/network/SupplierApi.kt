package com.techlane.supplier.network

import com.techlane.core.network.ApiHttp
import com.techlane.supplier.BuildConfig
import com.techlane.supplier.SupplierApp
import org.json.JSONArray
import org.json.JSONObject

object SupplierApi {
    private var onSessionExpired: (() -> Unit)? = null

    private val http = ApiHttp(
        apiBase = BuildConfig.API_BASE,
        tokenProvider = { SupplierApp.instance.tokenStore.sessionToken },
        onUnauthorized = { onSessionExpired?.invoke() },
    )

    fun setSessionExpiredListener(listener: (() -> Unit)?) {
        onSessionExpired = listener
    }

    fun acceptInvite(token: String, password: String): JSONObject =
        http.post(
            "/supplier/auth/accept-invite",
            JSONObject().put("token", token).put("password", password),
        )

    fun login(email: String, password: String): JSONObject =
        http.post(
            "/supplier/auth/login",
            JSONObject().put("email", email).put("password", password),
        )

    fun logout() {
        runCatching { http.post("/supplier/auth/logout") }
    }

    fun me(): JSONObject = http.get("/supplier/me")

    fun listRequests(status: String? = null): JSONArray {
        val q = if (status.isNullOrBlank()) "" else "?status=$status"
        return http.getArray("/supplier/requests$q")
    }

    fun requestDetail(id: String): JSONObject = http.get("/supplier/requests/$id")

    fun quote(id: String, unitCost: Double, notes: String? = null): JSONObject {
        val body = JSONObject().put("unit_cost", unitCost)
        if (!notes.isNullOrBlank()) body.put("notes", notes)
        return http.post("/supplier/requests/$id/quote", body)
    }

    fun decline(id: String, notes: String? = null): JSONObject {
        val body = JSONObject()
        if (!notes.isNullOrBlank()) body.put("notes", notes)
        return http.post("/supplier/requests/$id/decline", body)
    }

    fun markReady(id: String): JSONObject = http.post("/supplier/requests/$id/ready")

    fun issue(id: String): JSONObject = http.post("/supplier/requests/$id/issue")

    fun listIssues(): JSONArray = http.getArray("/supplier/issues")

    fun credit(): JSONObject = http.get("/supplier/credit")

    fun voucherHtmlUrl(issueId: String): String =
        "${BuildConfig.API_BASE}/supplier/issues/$issueId/voucher.html"
}
