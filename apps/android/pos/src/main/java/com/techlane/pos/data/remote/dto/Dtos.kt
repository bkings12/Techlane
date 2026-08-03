package com.techlane.pos.data.remote.dto

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

// ---------------------------------------------------------------- auth

@Serializable
data class LoginRequest(val email: String, val password: String)

@Serializable
data class MfaVerifyRequest(
    @SerialName("mfa_challenge") val mfaChallenge: String,
    val code: String,
)

@Serializable
data class RefreshRequest(@SerialName("refresh_token") val refreshToken: String)

/**
 * /auth/login answers either with tokens nested under "tokens", a flat token
 * pair (older builds), or an MFA challenge. All three shapes decode here.
 */
@Serializable
data class LoginResponse(
    @SerialName("mfa_required") val mfaRequired: Boolean = false,
    @SerialName("mfa_challenge") val mfaChallenge: String? = null,
    val tokens: TokenPairDto? = null,
    @SerialName("access_token") val accessToken: String? = null,
    @SerialName("refresh_token") val refreshToken: String? = null,
) {
    fun tokenPair(): TokenPairDto? {
        tokens?.let { return it }
        val access = accessToken ?: return null
        val refresh = refreshToken ?: return null
        return TokenPairDto(access, refresh)
    }
}

@Serializable
data class TokenPairDto(
    @SerialName("access_token") val accessToken: String,
    @SerialName("refresh_token") val refreshToken: String,
)

@Serializable
data class MeDto(
    val id: String,
    val email: String = "",
    @SerialName("display_name") val displayName: String = "",
    @SerialName("tenant_id") val tenantId: String = "",
    val roles: List<String> = emptyList(),
    val permissions: List<String> = emptyList(),
)

// ---------------------------------------------------------------- shop context

@Serializable
data class BranchDto(
    val id: String,
    val name: String,
    val code: String = "",
    val location: String = "",
)

@Serializable
data class StockLocationDto(
    val id: String,
    val name: String,
    @SerialName("branch_id") val branchId: String? = null,
    @SerialName("location_type") val locationType: String = "",
)

@Serializable
data class CatalogItemDto(
    @SerialName("variant_id") val variantId: String,
    @SerialName("product_id") val productId: String = "",
    @SerialName("product_name") val productName: String = "",
    val brand: String? = null,
    val category: String? = null,
    @SerialName("image_url") val imageUrl: String? = null,
    val sku: String = "",
    @SerialName("sell_price") val sellPrice: Double = 0.0,
    @SerialName("available_qty") val availableQty: Int = 0,
    @SerialName("location_id") val locationId: String = "",
)

@Serializable
data class IntakePresetDto(
    val id: String,
    val kind: String = "",
    val label: String = "",
    @SerialName("sort_order") val sortOrder: Int = 0,
)

@Serializable
data class ItemsEnvelope<T>(val items: List<T> = emptyList())

// ---------------------------------------------------------------- checkout

@Serializable
data class SaleItemInputDto(
    /** Catalog line. Omit for a quick-sale line and send [description]+[unitPrice]. */
    @SerialName("variant_id") val variantId: String? = null,
    val quantity: Int = 1,
    val description: String? = null,
    @SerialName("unit_price") val unitPrice: Double? = null,
)

@Serializable
data class CheckoutRequest(
    @SerialName("branch_id") val branchId: String,
    @SerialName("location_id") val locationId: String,
    val items: List<SaleItemInputDto>,
    val method: String,
    val phone: String? = null,
    @SerialName("account_reference") val accountReference: String? = null,
)

@Serializable
data class SaleDto(
    val id: String,
    @SerialName("branch_id") val branchId: String = "",
    val status: String = "",
    val subtotal: Double = 0.0,
    val total: Double = 0.0,
    @SerialName("payment_method") val paymentMethod: String = "",
    @SerialName("created_at") val createdAt: String? = null,
)

@Serializable
data class PaymentDto(
    val id: String,
    val method: String = "",
    val amount: Double = 0.0,
    val status: String = "",
    @SerialName("checkout_request_id") val checkoutRequestId: String = "",
    val phone: String = "",
    @SerialName("account_reference") val accountReference: String = "",
    @SerialName("payable_type") val payableType: String = "",
    @SerialName("payable_id") val payableId: String? = null,
    @SerialName("created_at") val createdAt: String? = null,
) {
    val isSettled: Boolean get() = status == "allocated" || status == "confirmed"
    val isFailed: Boolean get() = status == "failed" || status == "cancelled"
}

@Serializable
data class CheckoutResponse(
    val sale: SaleDto? = null,
    val payment: PaymentDto? = null,
    val completed: Boolean = false,
)

@Serializable
data class CompleteSaleRequest(@SerialName("location_id") val locationId: String)

@Serializable
data class PaymentSettingsDto(
    val configured: Boolean = false,
    @SerialName("mpesa_enabled") val mpesaEnabled: Boolean = false,
    @SerialName("mpesa_shortcode") val mpesaShortcode: String = "",
    val environment: String = "",
)

// ---------------------------------------------------------------- errors

@Serializable
data class ApiErrorEnvelope(val error: ApiErrorBody? = null)

@Serializable
data class ApiErrorBody(
    val code: String = "",
    val message: String = "",
)
