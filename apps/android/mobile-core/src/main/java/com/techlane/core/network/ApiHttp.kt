package com.techlane.core.network

import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONArray
import org.json.JSONObject
import java.util.concurrent.TimeUnit

class ApiHttp(
    private val apiBase: String,
    private val tokenProvider: () -> String?,
    private val onUnauthorized: (() -> Unit)? = null,
) {
    private val client = OkHttpClient.Builder()
        .connectTimeout(20, TimeUnit.SECONDS)
        .readTimeout(30, TimeUnit.SECONDS)
        .build()

    fun get(path: String): JSONObject = execute(Request.Builder().url("$apiBase$path").get().build())

    fun getArray(path: String, itemsKey: String = "items"): JSONArray {
        val obj = get(path)
        return obj.optJSONArray(itemsKey) ?: JSONArray()
    }

    fun post(path: String, body: JSONObject = JSONObject()): JSONObject {
        val req = Request.Builder()
            .url("$apiBase$path")
            .post(body.toString().toRequestBody(JSON))
            .build()
        return execute(req)
    }

    fun patch(path: String, body: JSONObject = JSONObject()): JSONObject {
        val req = Request.Builder()
            .url("$apiBase$path")
            .patch(body.toString().toRequestBody(JSON))
            .build()
        return execute(req)
    }

    private fun execute(request: Request): JSONObject {
        val authed = request.newBuilder().apply {
            tokenProvider()?.let { header("Authorization", "Bearer $it") }
            header("Accept", "application/json")
        }.build()
        client.newCall(authed).execute().use { res ->
            val text = res.body?.string().orEmpty()
            if (res.code == 401) {
                onUnauthorized?.invoke()
                throw IllegalStateException(errorMessage(text, "Session expired"))
            }
            if (!res.isSuccessful) {
                throw IllegalStateException(errorMessage(text, "HTTP ${res.code}"))
            }
            if (text.isBlank()) return JSONObject()
            return runCatching { JSONObject(text) }.getOrElse { JSONObject().put("raw", text) }
        }
    }

    private fun errorMessage(text: String, fallback: String): String {
        return runCatching {
            val obj = JSONObject(text)
            obj.optJSONObject("error")?.optString("message")
                ?.takeIf { it.isNotBlank() }
                ?: obj.optString("message").takeIf { it.isNotBlank() }
                ?: fallback
        }.getOrDefault(fallback)
    }

    companion object {
        private val JSON = "application/json; charset=utf-8".toMediaType()
    }
}
