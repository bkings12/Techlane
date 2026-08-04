package com.techlane.pos

import com.techlane.pos.domain.model.MpesaReference
import com.techlane.pos.domain.model.PaymentMethod
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Wire values are the contract with `internal/payments` — a typo here is a
 * 400 at the counter, so they are pinned rather than trusted.
 */
class PaymentMethodTest {

    @Test
    fun `wire values match the backend's accepted methods`() {
        assertEquals("mpesa_stk", PaymentMethod.MpesaStk.wire)
        assertEquals("cash", PaymentMethod.Cash.wire)
        // The backend has no "paybill" — customer-initiated M-Pesa is mpesa_c2b.
        assertEquals("mpesa_c2b", PaymentMethod.Paybill.wire)
    }

    @Test
    fun `only STK waits on a prompt`() {
        assertTrue(PaymentMethod.MpesaStk.isPrompted)
        assertFalse(PaymentMethod.Cash.isPrompted)
        assertFalse(PaymentMethod.Paybill.isPrompted)
    }

    @Test
    fun `only Paybill requires a transaction reference`() {
        assertTrue(PaymentMethod.Paybill.needsReference)
        assertFalse(PaymentMethod.Cash.needsReference)
        assertFalse(PaymentMethod.MpesaStk.needsReference)
    }

    @Test
    fun `every method has receipt wording`() {
        assertEquals("M-PESA STK", PaymentMethod.MpesaStk.receiptLabel)
        assertEquals("CASH", PaymentMethod.Cash.receiptLabel)
        assertEquals("M-PESA PAYBILL", PaymentMethod.Paybill.receiptLabel)
    }
}

class MpesaReferenceTest {

    @Test
    fun `a well-formed code is accepted`() {
        assertTrue(MpesaReference.isValid("QHK7T9XXXX"))
        assertNull(MpesaReference.validationError("QHK7T9XXXX"))
    }

    @Test
    fun `codes are upper-cased and trimmed as typed`() {
        assertEquals("QHK7T9XXXX", MpesaReference.normalise("  qhk7t9xxxx  "))
        // A technician copying off a phone shouldn't fight the field.
        assertTrue(MpesaReference.isValid("  qhk7t9xxxx  "))
    }

    @Test
    fun `a blank code asks for one rather than complaining about format`() {
        assertEquals(
            "Enter the M-Pesa code from the customer's message.",
            MpesaReference.validationError(""),
        )
    }

    @Test
    fun `wrong-length codes are rejected`() {
        assertFalse(MpesaReference.isValid("QHK7T9"))
        assertFalse(MpesaReference.isValid("QHK7T9XXXXX"))
        assertNotNull(MpesaReference.validationError("QHK7T9"))
    }

    @Test
    fun `codes with punctuation or spaces inside are rejected`() {
        assertFalse(MpesaReference.isValid("QHK7-T9XXX"))
        assertFalse(MpesaReference.isValid("QHK7 T9XXX"))
    }
}
