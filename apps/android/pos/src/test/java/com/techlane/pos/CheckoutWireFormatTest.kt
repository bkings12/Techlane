package com.techlane.pos

import com.techlane.pos.data.remote.dto.CheckoutRequest
import com.techlane.pos.data.remote.dto.SaleItemInputDto
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * The Go handler reads `quantity` into a plain int and rejects anything <= 0,
 * so a field that kotlinx quietly drops becomes "invalid quantity" at the till.
 * These assertions pin the exact body we put on the wire — this must stay in
 * step with [com.techlane.pos.di.AppModule.json].
 */
class CheckoutWireFormatTest {

    private val json = Json {
        ignoreUnknownKeys = true
        coerceInputValues = true
        explicitNulls = false
        isLenient = true
        encodeDefaults = true
    }

    @Test
    fun `quick-sale line carries quantity even when it equals the default`() {
        val body = json.encodeToString(
            CheckoutRequest.serializer(),
            CheckoutRequest(
                branchId = "b1",
                locationId = "l1",
                items = listOf(SaleItemInputDto(description = "Screen replacement", quantity = 1, unitPrice = 2500.0)),
                method = "mpesa_stk",
                phone = "254712345678",
            ),
        )
        val line = json.parseToJsonElement(body).jsonObject["items"]!!.jsonArray.first().jsonObject

        assertEquals(1, line["quantity"]?.jsonPrimitive?.int())
        assertEquals("Screen replacement", line["description"]?.jsonPrimitive?.content)
        assertEquals(2500.0, line["unit_price"]?.jsonPrimitive?.content?.toDouble())
        // A quick-sale line must not carry a variant, or the server prices it from the catalog.
        assertFalse("variant_id must be absent on a quick-sale line", line.containsKey("variant_id"))
    }

    @Test
    fun `catalog line carries variant and quantity`() {
        val body = json.encodeToString(
            CheckoutRequest.serializer(),
            CheckoutRequest(
                branchId = "b1",
                locationId = "l1",
                items = listOf(SaleItemInputDto(variantId = "v-123", quantity = 3)),
                method = "cash",
            ),
        )
        val line = json.parseToJsonElement(body).jsonObject["items"]!!.jsonArray.first().jsonObject

        assertEquals("v-123", line["variant_id"]?.jsonPrimitive?.content)
        assertEquals(3, line["quantity"]?.jsonPrimitive?.int())
    }

    @Test
    fun `null optionals stay off the wire`() {
        val body = json.encodeToString(
            CheckoutRequest.serializer(),
            CheckoutRequest(
                branchId = "b1",
                locationId = "l1",
                items = listOf(SaleItemInputDto(variantId = "v-1", quantity = 1)),
                method = "cash",
                phone = null,
                accountReference = null,
            ),
        )
        val root = json.parseToJsonElement(body).jsonObject

        assertFalse(root.containsKey("phone"))
        assertFalse(root.containsKey("account_reference"))
        assertTrue(root.containsKey("method"))
    }

    private fun kotlinx.serialization.json.JsonPrimitive.int() = content.toInt()
}
