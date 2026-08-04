package com.techlane.pos

import com.techlane.pos.data.remote.dto.SaleDto
import com.techlane.pos.data.session.PosPreferences
import com.techlane.pos.domain.model.ChargeRowDestination
import com.techlane.pos.domain.model.SaleDetail
import com.techlane.pos.domain.model.SaleLineItem
import com.techlane.pos.domain.model.SaleSummary
import com.techlane.pos.domain.model.SalesFilter
import com.techlane.pos.domain.model.chargeRowDestination
import com.techlane.pos.domain.model.paymentMethodLabel
import kotlinx.serialization.json.Json
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Sales module: Sale Details navigation, receipt reopen/reprint, Quick
 * Prompt persistence, and cost-visibility gating. See internal/sales for the
 * backend counterpart (GetSale/ListSales cost stripping, search/filters).
 */
class SalesModuleTest {

    private fun item(desc: String, qty: Int, price: Double, cost: Double? = null) = SaleLineItem(
        description = desc, quantity = qty, unitPrice = price, lineTotal = price * qty,
        unitCost = cost, margin = cost?.let { (price - it) * qty },
    )

    private fun detail(status: String, paid: Double, total: Double) = SaleDetail(
        id = "sale-1", reference = "SL-ABCD1234", status = status, channel = "pos",
        branchName = "Main", cashierName = "Jane", customerName = "Ayub Macharia",
        customerPhone = "254700000001", createdAt = 1_000L,
        items = listOf(item("Oraimo Charger", 1, 2800.0, 1800.0)),
        subtotal = total, taxTotal = 0.0, discountTotal = 0.0, total = total,
        paidTotal = paid, balanceDue = (total - paid).coerceAtLeast(0.0),
        paymentMethod = "mpesa_stk", paymentStatus = if (paid >= total) "confirmed" else "pending",
        paymentReference = "QHK7T9X001",
    )

    // --------------------------------------------------------- sale status

    @Test
    fun `a completed sale with full payment is settled`() {
        val d = detail(status = "completed", paid = 2800.0, total = 2800.0)
        assertTrue(d.isCompleted)
        assertTrue(d.paymentIsSettled)
        assertEquals(0.0, d.balanceDue, 0.0)
    }

    @Test
    fun `a draft sale (pending STK) must never read as completed`() {
        // This is the core "sale status vs payment status" separation: an STK
        // that only initiated, not yet confirmed, must not appear paid.
        val d = detail(status = "draft", paid = 0.0, total = 2800.0)
        assertFalse(d.isCompleted)
        assertFalse(d.paymentIsSettled)
        assertEquals(2800.0, d.balanceDue, 0.0)
    }

    @Test
    fun `a reversed sale is distinguishable from completed`() {
        val d = detail(status = "reversed", paid = 2800.0, total = 2800.0)
        assertFalse(d.isCompleted)
    }

    @Test
    fun `sale summary mirrors the same completed and reversed checks`() {
        val completed = SaleSummary(
            id = "s1", reference = "SL-1", customerName = null, total = 100.0,
            paymentMethod = "cash", status = "completed", createdAt = null, itemSummary = null, itemCount = 1,
        )
        val reversed = completed.copy(status = "reversed")
        val draft = completed.copy(status = "draft")
        assertTrue(completed.isCompleted)
        assertTrue(reversed.isReversed)
        assertFalse(draft.isCompleted)
        assertFalse(draft.isReversed)
    }

    // ------------------------------------------------------------ line items

    @Test
    fun `line item totals are quantity times unit price`() {
        val i = item("Screen protector", 3, 500.0)
        assertEquals(1500.0, i.lineTotal, 0.0)
    }

    @Test
    fun `cost info is only present when the server actually sent it`() {
        val withCost = item("Charger", 1, 2800.0, cost = 1800.0)
        val withoutCost = item("Charger", 1, 2800.0, cost = null)
        assertTrue(withCost.hasCostInfo)
        assertFalse(withoutCost.hasCostInfo)
        assertEquals(1000.0, withCost.margin)
        assertNull(withoutCost.margin)
    }

    // ---------------------------------------------------- charge row routing

    @Test
    fun `a charge row with a saleId routes to Sale Details`() {
        val dest = chargeRowDestination(saleId = "sale-42", repairId = null)
        assertTrue(dest is ChargeRowDestination.Sale)
        assertEquals("sale-42", (dest as ChargeRowDestination.Sale).saleId)
    }

    @Test
    fun `a charge row with a repairId routes to the Job, never a fabricated sale`() {
        val dest = chargeRowDestination(saleId = null, repairId = "job-9")
        assertTrue(dest is ChargeRowDestination.Job)
        assertEquals("job-9", (dest as ChargeRowDestination.Job).repairId)
    }

    @Test
    fun `a charge row with neither id is inert`() {
        assertEquals(ChargeRowDestination.None, chargeRowDestination(null, null))
    }

    @Test
    fun `a saleId always wins over a repairId if somehow both are set`() {
        val dest = chargeRowDestination(saleId = "sale-1", repairId = "job-1")
        assertTrue(dest is ChargeRowDestination.Sale)
    }

    // -------------------------------------------------------------- filters

    @Test
    fun `an empty filter is recognised as empty`() {
        assertTrue(SalesFilter().isEmpty)
        assertFalse(SalesFilter(query = "QHK").isEmpty)
        assertFalse(SalesFilter(method = "cash").isEmpty)
    }

    @Test
    fun `payment method labels are human readable`() {
        assertEquals("M-Pesa", paymentMethodLabel("mpesa_stk"))
        assertEquals("Paybill", paymentMethodLabel("mpesa_c2b"))
        assertEquals("Cash", paymentMethodLabel("cash"))
    }

    // --------------------------------------------------------- role gating

    @Test
    fun `owner and manager can see cost, cashier and technician cannot`() {
        assertTrue(PosPreferences(roles = listOf("owner")).canSeeCost)
        assertTrue(PosPreferences(roles = listOf("manager")).canSeeCost)
        assertFalse(PosPreferences(roles = listOf("cashier")).canSeeCost)
        assertFalse(PosPreferences(roles = listOf("technician")).canSeeCost)
        assertFalse(PosPreferences(roles = emptyList()).canSeeCost)
    }

    // ---------------------------------------------------- wire tolerance

    private val json = Json { ignoreUnknownKeys = true }

    @Test
    fun `a sale with no detail fields at all decodes with safe defaults`() {
        val dto = json.decodeFromString<SaleDto>("""{"id":"sale-1"}""")
        assertTrue(dto.items.isEmpty())
        assertEquals("", dto.reference)
        assertEquals(0.0, dto.paidTotal, 0.0)
    }

    @Test
    fun `an item with cost stripped by the server decodes with a null cost, not zero`() {
        val dto = json.decodeFromString<SaleDto>(
            """{"id":"sale-1","items":[{"description":"Charger","quantity":1,"unit_price":2800,"line_total":2800}]}""",
        )
        assertEquals(1, dto.items.size)
        assertNull(dto.items.first().unitCost)
        assertNull(dto.items.first().margin)
        assertEquals(2800.0, dto.items.first().unitPrice, 0.0)
    }
}
