package com.techlane.pos

import com.techlane.pos.data.remote.dto.CustomerDto
import com.techlane.pos.domain.model.CommonIssues
import com.techlane.pos.domain.model.DeviceKind
import com.techlane.pos.domain.model.IntakeAccessory
import com.techlane.pos.domain.model.PromiseOption
import com.techlane.pos.feature.intake.IntakeUiState
import com.techlane.pos.feature.intake.toRequest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Intake can create a customer record and a job in one shot, so what it does
 * and doesn't send matters more than most forms.
 */
class IntakeFlowTest {

    private val typed = IntakeUiState(
        customerName = "Ayub Macharia",
        customerPhone = "0794165578",
        problem = "Cracked screen",
        branchId = "branch-1",
    )

    private val existing = CustomerDto(id = "cust-1", fullName = "Ayub Macharia", phone = "+254794165578")

    @Test
    fun `a matched customer is sent by id, never re-created by name`() {
        // Re-sending name/phone for someone we matched would risk a duplicate
        // customer record for the same person.
        val request = typed.copy(matchedCustomer = existing).toRequest("branch-1")

        assertEquals("cust-1", request.customerId)
        assertNull(request.customerName)
        assertNull(request.customerPhone)
    }

    @Test
    fun `a brand new customer is sent by name and phone`() {
        val request = typed.toRequest("branch-1")

        assertNull(request.customerId)
        assertEquals("Ayub Macharia", request.customerName)
        assertNotNull(request.customerPhone)
    }

    @Test
    fun `a matched customer satisfies the customer requirement without typing a name`() {
        val state = IntakeUiState(
            matchedCustomer = existing,
            problem = "No power",
            branchId = "branch-1",
        )

        assertTrue(state.hasCustomer)
        assertTrue(state.canSave)
    }

    @Test
    fun `a name without a phone is refused`() {
        assertFalse(typed.copy(customerPhone = "").canSave)
    }

    @Test
    fun `a walk-in needs no customer details and sends none`() {
        val state = typed.copy(anonymous = true)
        val request = state.toRequest("branch-1")

        assertTrue(state.canSave)
        assertTrue(request.anonymous)
        assertNull(request.customerName)
        assertNull(request.customerId)
    }

    @Test
    fun `accessories and a condition note both travel as condition tags`() {
        val request = typed.copy(
            accessories = setOf(IntakeAccessory.Charger, IntakeAccessory.SimCard),
            conditionNote = "Scratched back",
        ).toRequest("branch-1")

        assertTrue(request.conditionTags.contains("Charger"))
        assertTrue(request.conditionTags.contains("SIM card"))
        assertTrue(request.conditionTags.contains("Scratched back"))
    }

    @Test
    fun `no accessories and no note means no tags rather than blank ones`() {
        assertTrue(typed.toRequest("branch-1").conditionTags.isEmpty())
    }

    @Test
    fun `a phone's identifier is sent as IMEI and a laptop's as a serial`() {
        val phone = typed.copy(deviceKind = DeviceKind.Phone, identifier = "356938035643809")
            .toRequest("branch-1")
        assertEquals("356938035643809", phone.imei)
        assertNull(phone.serialNumber)

        val laptop = typed.copy(deviceKind = DeviceKind.Laptop, identifier = "C02X1234JGH5")
            .toRequest("branch-1")
        assertEquals("C02X1234JGH5", laptop.serialNumber)
        assertNull(laptop.imei)
    }

    @Test
    fun `no promise sends no promise rather than a default one`() {
        // A job with no promise is simply never overdue; inventing one would
        // make the board lie.
        assertNull(typed.toRequest("branch-1").promisedBy)
    }

    @Test
    fun `a chosen promise is sent as an instant`() {
        val request = typed.copy(promise = PromiseOption.Tomorrow).toRequest("branch-1")

        assertNotNull(request.promisedBy)
    }

    @Test
    fun `a zero estimate is omitted so intake does not authorise work`() {
        // Entering a price authorises the repair, so "0" must not count as one.
        assertNull(typed.copy(estimateLabour = "0").toRequest("branch-1").estimateLaborAmount)
        assertNull(typed.toRequest("branch-1").estimateLaborAmount)
        assertEquals(2500.0, typed.copy(estimateLabour = "2500").toRequest("branch-1").estimateLaborAmount)
    }

    @Test
    fun `the summary reflects a matched customer over typed text`() {
        assertEquals("Ayub Macharia", typed.copy(matchedCustomer = existing).summaryCustomer)
        assertEquals("Walk-in", typed.copy(anonymous = true).summaryCustomer)
    }

    @Test
    fun `device summary falls back to the device type when unnamed`() {
        assertEquals("Phone", typed.summaryDevice)
        assertEquals("Apple A2337", typed.copy(brand = "Apple", model = "A2337").summaryDevice)
    }
}

class IntakeHelpersTest {

    @Test
    fun `issue chips are scoped to the device type`() {
        assertTrue(CommonIssues.forKind(DeviceKind.Laptop).contains("Keyboard"))
        assertFalse(CommonIssues.forKind(DeviceKind.Phone).contains("Keyboard"))
        assertTrue(CommonIssues.forKind(DeviceKind.Phone).contains("Network issue"))
    }

    @Test
    fun `every device type offers some issues`() {
        DeviceKind.entries.forEach { kind ->
            assertTrue("$kind has no chips", CommonIssues.forKind(kind).isNotEmpty())
        }
    }

    @Test
    fun `quick promises resolve to increasing future instants`() {
        val today = PromiseOption.Today.at()!!
        val tomorrow = PromiseOption.Tomorrow.at()!!
        val twoDays = PromiseOption.InTwoDays.at()!!

        assertTrue(tomorrow > today)
        assertTrue(twoDays > tomorrow)
    }

    @Test
    fun `an exact promise returns exactly what was chosen`() {
        assertEquals(1_234_567L, PromiseOption.Exact(1_234_567L, "Fri").at())
    }
}
