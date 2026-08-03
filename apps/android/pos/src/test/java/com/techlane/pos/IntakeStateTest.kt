package com.techlane.pos

import com.techlane.pos.domain.model.DeviceKind
import com.techlane.pos.feature.intake.IntakeUiState
import com.techlane.pos.feature.intake.toRequest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Intake is the one screen that can create a customer record, so what it does
 * and doesn't send matters — a stray blank string becomes a customer named "".
 */
class IntakeStateTest {

    private val valid = IntakeUiState(
        customerName = "Ayub Macharia",
        customerPhone = "0794165578",
        problem = "Cracked screen",
        branchId = "branch-1",
    )

    @Test
    fun `a named customer with a phone and a fault can be saved`() {
        assertTrue(valid.canSave)
        assertNull(valid.validationHint)
    }

    @Test
    fun `a name without a phone is refused`() {
        val state = valid.copy(customerPhone = "")

        assertFalse(state.canSave)
        assertEquals("Add the customer's name and phone, or mark it a walk-in.", state.validationHint)
    }

    @Test
    fun `a walk-in needs no customer details at all`() {
        val state = IntakeUiState(anonymous = true, problem = "No power", branchId = "branch-1")

        assertTrue(state.canSave)
    }

    @Test
    fun `a fault description shorter than three characters is refused`() {
        assertFalse(valid.copy(problem = "no").canSave)
    }

    @Test
    fun `no branch blocks saving and says why`() {
        val state = valid.copy(branchId = null)

        assertFalse(state.canSave)
        assertEquals("Pick a branch in Settings before booking a job.", state.validationHint)
    }

    @Test
    fun `saving is blocked while a save is already in flight`() {
        assertFalse(valid.copy(saving = true).canSave)
    }

    @Test
    fun `a phone's identifier is sent as IMEI and never as a serial`() {
        val request = valid.copy(deviceKind = DeviceKind.Phone, identifier = "356938035643809")
            .toRequest("branch-1")

        assertEquals("356938035643809", request.imei)
        assertNull(request.serialNumber)
    }

    @Test
    fun `a laptop's identifier is sent as a serial and never as IMEI`() {
        val request = valid.copy(deviceKind = DeviceKind.Laptop, identifier = "C02X1234JGH5")
            .toRequest("branch-1")

        assertEquals("C02X1234JGH5", request.serialNumber)
        assertNull(request.imei)
    }

    @Test
    fun `a walk-in sends no customer name or phone even if they were typed first`() {
        // Toggling "walk-in" after typing must not smuggle the details through.
        val request = valid.copy(anonymous = true).toRequest("branch-1")

        assertTrue(request.anonymous)
        assertNull(request.customerName)
        assertNull(request.customerPhone)
    }

    @Test
    fun `blank optional fields are omitted rather than sent as empty strings`() {
        val request = valid.copy(brand = "  ", model = "", identifier = "").toRequest("branch-1")

        assertNull(request.brand)
        assertNull(request.model)
        assertNull(request.imei)
        assertNull(request.serialNumber)
    }

    @Test
    fun `text fields are trimmed before they are sent`() {
        val request = valid.copy(customerName = "  Ayub  ", problem = "  Cracked screen  ")
            .toRequest("branch-1")

        assertEquals("Ayub", request.customerName)
        assertEquals("Cracked screen", request.problemSummary)
    }

    @Test
    fun `a zero or unparseable labour estimate is omitted, not sent as zero`() {
        assertNull(valid.copy(estimateLabour = "0").toRequest("branch-1").estimateLaborAmount)
        assertNull(valid.copy(estimateLabour = "").toRequest("branch-1").estimateLaborAmount)
        assertEquals(2500.0, valid.copy(estimateLabour = "2500").toRequest("branch-1").estimateLaborAmount)
    }
}
