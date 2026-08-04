package com.techlane.pos

import com.techlane.pos.data.remote.dto.RepairJobDto
import com.techlane.pos.domain.model.JobPart
import kotlinx.serialization.json.Json
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Work-order line items (labour/part/product) — the Android-side data shapes
 * and the filtering the job screen does to split one JobPart list into three
 * cards. See internal/repair/line_items.go for the backend counterpart.
 */
class JobLineItemsTest {

    private fun part(
        name: String,
        lineType: String,
        quantity: Int = 1,
        unitPrice: Double = 0.0,
        unitCost: Double? = null,
        partStatus: String? = null,
        partSource: String? = null,
    ) = JobPart(
        rowId = name, lineId = name, variantId = null, name = name, sku = null,
        quantity = quantity, unitPrice = unitPrice, availableQty = null,
        lineType = lineType, unitCost = unitCost, partStatus = partStatus, partSource = partSource,
    )

    @Test
    fun `line total is quantity times unit price regardless of type`() {
        val labour = part("Board repair", "labour", quantity = 1, unitPrice = 3000.0)
        val p = part("Charging IC", "part", quantity = 2, unitPrice = 2500.0)
        assertEquals(3000.0, labour.lineTotal, 0.0)
        assertEquals(5000.0, p.lineTotal, 0.0)
    }

    @Test
    fun `isLabour and isProduct match the wire line_type`() {
        assertTrue(part("Diagnosis", "labour").isLabour)
        assertTrue(part("Charger", "product").isProduct)
        val p = part("Screen", "part")
        assertEquals(false, p.isLabour)
        assertEquals(false, p.isProduct)
    }

    @Test
    fun `filtering one parts list into labour, parts, and products never drops or duplicates a line`() {
        // Mirrors exactly what JobDetailsScreen does with detail.parts before
        // handing each group to its own LineItemsCard.
        val all = listOf(
            part("Diagnosis", "labour", unitPrice = 500.0),
            part("Board repair", "labour", unitPrice = 2500.0),
            part("Charging IC", "part", unitPrice = 2500.0, unitCost = 1500.0, partSource = "inventory"),
            part("MacBook Display", "part", unitPrice = 18000.0, unitCost = 14000.0, partSource = "sourced"),
            part("Oraimo Charger", "product", unitPrice = 2800.0, unitCost = 1800.0),
        )
        val labour = all.filter { it.lineType == "labour" }
        val parts = all.filter { it.lineType == "part" }
        val products = all.filter { it.lineType == "product" }

        assertEquals(2, labour.size)
        assertEquals(2, parts.size)
        assertEquals(1, products.size)
        assertEquals(all.size, labour.size + parts.size + products.size)

        assertEquals(3000.0, labour.sumOf { it.lineTotal }, 0.0)
        assertEquals(20500.0, parts.sumOf { it.lineTotal }, 0.0)
        assertEquals(2800.0, products.sumOf { it.lineTotal }, 0.0)
    }

    @Test
    fun `a sourced part is distinguishable from an inventory part`() {
        val sourced = part("MacBook Display", "part", partSource = "sourced")
        val fromStock = part("Charging IC", "part", partSource = "inventory")
        assertEquals("sourced", sourced.partSource)
        assertEquals("inventory", fromStock.partSource)
    }

    // ------------------------------------------------- wire-format tolerance
    //
    // A handset on an older build must not crash decoding a job payload that
    // predates work-order line items (or that a technician-role response has
    // had unit_cost stripped from) — every new field needs a safe default.

    private val json = Json { ignoreUnknownKeys = true }

    @Test
    fun `a repair job with no line-item fields at all decodes with empty defaults`() {
        val dto = json.decodeFromString<RepairJobDto>(
            """{"id":"job-1","status":"in_progress","problem_summary":"No display"}""",
        )
        assertTrue(dto.labourLines.isEmpty())
        assertTrue(dto.partLines.isEmpty())
        assertTrue(dto.productLines.isEmpty())
        assertEquals(0.0, dto.labourTotal, 0.0)
    }

    @Test
    fun `a line item with unit_cost stripped (no reports_read) decodes with a null cost`() {
        val dto = json.decodeFromString<RepairJobDto>(
            """{"id":"job-1","status":"in_progress","problem_summary":"No display",
                "part_lines":[{"id":"li-1","line_type":"part","description":"Charging IC",
                "quantity":1,"unit_price":2500,"line_total":2500}]}""",
        )
        assertEquals(1, dto.partLines.size)
        assertNull(dto.partLines.first().unitCost)
        assertEquals(2500.0, dto.partLines.first().unitPrice, 0.0)
    }
}
