package com.techlane.pos.data.remote

import com.techlane.pos.data.remote.dto.ApiErrorEnvelope
import kotlinx.serialization.json.Json
import retrofit2.HttpException
import java.io.IOException

/** Server-reported failure carrying the message staff should actually read. */
class ApiException(
    val statusCode: Int,
    val code: String,
    override val message: String,
) : Exception(message)

/** The refresh token is gone or rejected — the session is over. */
class SessionExpiredException(
    override val message: String = "Session expired. Sign in again.",
) : Exception(message)

/** No usable connection. Distinct from a server error so the UI can offer a retry. */
class OfflineException(
    override val message: String = "No connection. Check this phone's internet and try again.",
) : Exception(message)

private val errorJson = Json { ignoreUnknownKeys = true }

/**
 * Turns whatever a call threw into one of our three exception types, so screens
 * never have to reason about Retrofit or OkHttp internals.
 */
fun Throwable.toAppException(): Exception = when (this) {
    is ApiException, is SessionExpiredException, is OfflineException -> this as Exception
    is HttpException -> {
        val body = runCatching { response()?.errorBody()?.string() }.getOrNull()
        val parsed = body?.let {
            runCatching { errorJson.decodeFromString<ApiErrorEnvelope>(it).error }.getOrNull()
        }
        if (code() == 401) {
            SessionExpiredException(parsed?.message?.takeIf { it.isNotBlank() } ?: "Session expired. Sign in again.")
        } else {
            ApiException(
                statusCode = code(),
                code = parsed?.code.orEmpty(),
                message = parsed?.message?.takeIf { it.isNotBlank() }
                    ?: body?.takeIf { it.isNotBlank() && it.length < 300 }
                    ?: message().ifBlank { "Request failed (${code()})" },
            )
        }
    }
    is IOException -> OfflineException()
    else -> ApiException(0, "UNKNOWN", message ?: "Something went wrong")
}

/** Reads {"error":{"code","message"}} out of a raw body, for non-throwing paths. */
fun parseApiErrorMessage(body: String?, fallback: String): String {
    if (body.isNullOrBlank()) return fallback
    val parsed = runCatching { errorJson.decodeFromString<ApiErrorEnvelope>(body).error }.getOrNull()
    return parsed?.message?.takeIf { it.isNotBlank() } ?: fallback
}
