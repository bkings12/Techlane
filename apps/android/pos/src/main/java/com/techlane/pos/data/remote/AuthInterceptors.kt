package com.techlane.pos.data.remote

import com.techlane.pos.data.session.SecureTokenStore
import kotlinx.serialization.json.Json
import okhttp3.Interceptor
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import javax.inject.Inject
import javax.inject.Named
import javax.inject.Singleton

/** Attaches the bearer token to every call except the auth endpoints. */
@Singleton
class AuthHeaderInterceptor @Inject constructor(
    private val tokens: SecureTokenStore,
) : Interceptor {
    override fun intercept(chain: Interceptor.Chain): Response {
        val request = chain.request()
        if (request.url.encodedPath.contains("/auth/")) return chain.proceed(request)
        val token = tokens.accessToken ?: return chain.proceed(request)
        return chain.proceed(
            request.newBuilder().header("Authorization", "Bearer $token").build(),
        )
    }
}

/**
 * Single-flight refresh on 401. Retrofit's own Authenticator would work, but it
 * cannot see the response body, and we want a hard sign-out (rather than an
 * endless retry) the moment the refresh token itself is rejected.
 */
@Singleton
class TokenRefreshInterceptor @Inject constructor(
    private val tokens: SecureTokenStore,
    @Named("apiBase") private val baseUrl: String,
    private val onSessionExpired: SessionExpiryNotifier,
) : Interceptor {

    private val json = Json { ignoreUnknownKeys = true }
    private val jsonMedia = "application/json; charset=utf-8".toMediaType()
    private val refreshLock = Any()

    override fun intercept(chain: Interceptor.Chain): Response {
        val response = chain.proceed(chain.request())
        if (response.code != 401 || chain.request().url.encodedPath.contains("/auth/")) return response

        val staleToken = chain.request().header("Authorization")
        response.close()

        val refreshed = synchronized(refreshLock) {
            // Another thread may already have rotated the token while we queued.
            val current = tokens.accessToken
            if (current != null && "Bearer $current" != staleToken) true else refresh()
        }

        if (!refreshed) {
            tokens.clear()
            onSessionExpired.notifyExpired()
            return chain.proceed(chain.request())
        }

        val token = tokens.accessToken ?: return chain.proceed(chain.request())
        return chain.proceed(
            chain.request().newBuilder().header("Authorization", "Bearer $token").build(),
        )
    }

    private fun refresh(): Boolean {
        val refreshToken = tokens.refreshToken ?: return false
        val body = """{"refresh_token":"$refreshToken"}""".toRequestBody(jsonMedia)
        val request = Request.Builder()
            .url(baseUrl.trimEnd('/') + "/auth/refresh")
            .post(body)
            .build()
        // A bare client: reusing the app client here would recurse through this
        // very interceptor on another 401.
        return runCatching {
            OkHttpClient().newCall(request).execute().use { res ->
                if (!res.isSuccessful) return@use false
                val text = res.body?.string().orEmpty()
                val pair = json.decodeFromString<com.techlane.pos.data.remote.dto.TokenPairDto>(text)
                tokens.save(pair.accessToken, pair.refreshToken)
                true
            }
        }.getOrDefault(false)
    }
}

/** Lets the data layer tell the UI to drop back to the login screen. */
@Singleton
class SessionExpiryNotifier @Inject constructor() {
    private val _expired = kotlinx.coroutines.flow.MutableSharedFlow<Unit>(extraBufferCapacity = 1)
    val expired: kotlinx.coroutines.flow.SharedFlow<Unit> = _expired

    fun notifyExpired() {
        _expired.tryEmit(Unit)
    }
}
