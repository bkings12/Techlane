package com.techlane.core.scan

import org.json.JSONObject

data class ParsedScan(
    val mode: String? = null,
    val code: String,
    val relatedId: String? = null,
)

/**
 * Accepts plain codes or structured QR payloads used on the shop floor.
 *
 * Supported shapes:
 * - plain text / IMEI / barcode / collection code
 * - JSON: {"type":"imei|barcode|auth|collection|repair_pickup","code":"...","issue_id":"..."}
 * - techlane://auth/<issueId>/<authCode>
 * - techlane://collect/<code>  (commerce order)
 * - techlane://repair-pickup/<code>  (repair intake slip)
 * - auth:<issueId>:<authCode>
 * - <issueUuid>:<authCode>
 * - PK-XXXXXX  (repair pickup code printed at intake)
 */
fun parseScanPayload(raw: String): ParsedScan {
    val value = raw.trim()
    if (value.isEmpty()) return ParsedScan(code = "")

    if (value.startsWith("{")) {
        runCatching {
            val json = JSONObject(value)
            val type = json.optString("type").ifBlank { json.optString("mode") }.lowercase()
            val code = json.optString("code")
                .ifBlank { json.optString("auth_code") }
                .ifBlank { json.optString("collection_code") }
                .ifBlank { json.optString("pickup_code") }
                .ifBlank { json.optString("imei") }
                .ifBlank { json.optString("barcode") }
            val related = json.optString("issue_id")
                .ifBlank { json.optString("supplier_issue_id") }
                .ifBlank { json.optString("related_id") }
                .takeIf { it.isNotBlank() }
            if (code.isNotBlank()) {
                val mode = when (type) {
                    "imei", "barcode", "auth", "collection", "repair_pickup" -> type
                    else -> null
                }
                return ParsedScan(mode = mode, code = code.trim(), relatedId = related)
            }
        }
    }

    val lower = value.lowercase()
    when {
        lower.startsWith("techlane://auth/") || lower.startsWith("techlane:auth:") -> {
            val parts = value.substringAfter("auth").trim(':', '/', ' ').split('/', ':').filter { it.isNotBlank() }
            if (parts.size >= 2) {
                return ParsedScan(mode = "auth", code = parts[1], relatedId = parts[0])
            }
        }
        lower.startsWith("techlane://repair-pickup/") || lower.startsWith("techlane:repair-pickup:") -> {
            val code = value.substringAfter("repair-pickup").trim(':', '/', ' ')
            if (code.isNotBlank()) return ParsedScan(mode = "repair_pickup", code = code.uppercase())
        }
        lower.startsWith("techlane://collect/") || lower.startsWith("techlane:collect:") -> {
            val code = value.substringAfter("collect").trim(':', '/', ' ')
            if (code.isNotBlank()) return ParsedScan(mode = "collection", code = code)
        }
        lower.startsWith("auth:") -> {
            val parts = value.removePrefix("auth:").split(':', limit = 2)
            if (parts.size == 2 && parts[0].isNotBlank() && parts[1].isNotBlank()) {
                return ParsedScan(mode = "auth", code = parts[1].trim(), relatedId = parts[0].trim())
            }
        }
    }

    // UUID:authCode (common printed auth QR)
    val colon = value.indexOf(':')
    if (colon in 8..64) {
        val left = value.take(colon).trim()
        val right = value.substring(colon + 1).trim()
        val uuidLike = left.matches(Regex("(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"))
        if (uuidLike && right.isNotBlank()) {
            return ParsedScan(mode = "auth", code = right, relatedId = left)
        }
    }

    // Repair intake pickup codes: PK-XXXXXX
    if (value.matches(Regex("(?i)^PK-[A-Z0-9]{4,12}$"))) {
        return ParsedScan(mode = "repair_pickup", code = value.uppercase())
    }

    // Collection codes are usually short alphanumeric with a prefix.
    if (value.matches(Regex("(?i)^[A-Z]{2,6}-?[A-Z0-9]{4,12}$")) && value.any { it.isLetter() }) {
        return ParsedScan(mode = "collection", code = value.uppercase())
    }

    // 15-digit IMEI
    if (value.matches(Regex("^\\d{15}$"))) {
        return ParsedScan(mode = "imei", code = value)
    }

    return ParsedScan(code = value)
}
