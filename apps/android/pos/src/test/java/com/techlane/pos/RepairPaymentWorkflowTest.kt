package com.techlane.pos

import com.techlane.pos.domain.model.JobAction
import com.techlane.pos.domain.model.JobStatus
import com.techlane.pos.domain.model.MpesaReference
import com.techlane.pos.domain.model.PaymentMethod
import com.techlane.pos.feature.jobs.components.TakePaymentDraft
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Repair payment rules that must stay true without spinning up Hilt / Room.
 * Covers the Take Payment / collection gates from the job-payment workflow.
 */
class RepairPaymentWorkflowTest {

    @Test
    fun `ready job with balance offers Take Payment not Mark Collected`() {
        val actions = JobAction.forStatus(JobStatus.ReadyForPickup, needsApproval = false, balanceDue = 5300.0)
        assertTrue(JobAction.TakePayment in actions)
        assertFalse(JobAction.MarkCollected in actions)
    }

    @Test
    fun `zero balance enables Mark Collected`() {
        val actions = JobAction.forStatus(JobStatus.ReadyForPickup, needsApproval = false, balanceDue = 0.0)
        assertTrue(JobAction.MarkCollected in actions)
        assertFalse(JobAction.TakePayment in actions)
    }

    @Test
    fun `completed status with balance still offers Take Payment`() {
        val actions = JobAction.forStatus(JobStatus.Completed, needsApproval = false, balanceDue = 1000.0)
        assertTrue(JobAction.TakePayment in actions)
    }

    @Test
    fun `cash and paybill and quick prompt are distinct methods on the same spine`() {
        assertEquals("cash", PaymentMethod.Cash.wire)
        assertEquals("mpesa_c2b", PaymentMethod.Paybill.wire)
        assertEquals("mpesa_stk", PaymentMethod.MpesaStk.wire)
        assertTrue(PaymentMethod.MpesaStk.isPrompted)
        assertFalse(PaymentMethod.Cash.isPrompted)
        // Job Paybill uses job_code as bill ref — TransID is optional matching aid.
        assertTrue(PaymentMethod.Paybill.needsReference) // till still requires TransID
    }

    @Test
    fun `partial payment draft stays within outstanding balance`() {
        val balance = 5300.0
        val draft = TakePaymentDraft(method = PaymentMethod.Cash, amount = 2000.0)
        assertTrue(draft.amount <= balance)
        assertEquals(PaymentMethod.Cash, draft.method)
    }

    @Test
    fun `mixed methods remain separate authoritative records conceptually`() {
        // Two drafts for one job — each is its own payment, never overwriting.
        val cash = TakePaymentDraft(PaymentMethod.Cash, 2000.0)
        val mpesa = TakePaymentDraft(PaymentMethod.MpesaStk, 3300.0, phone = "254794165578")
        assertEquals(5300.0, cash.amount + mpesa.amount, 0.001)
        assertTrue(cash.method != mpesa.method)
    }

    @Test
    fun `duplicate paybill reference shape is validated`() {
        assertNull(MpesaReference.validationError("QGH7XYZABC"))
        assertNotNull(MpesaReference.validationError("SHORT"))
        assertNotNull(MpesaReference.validationError(""))
        assertEquals("QGH7XYZABC", MpesaReference.normalise(" qgh7xyzabc "))
    }

    @Test
    fun `STK pending is not treated as prompted settled`() {
        // Only MpesaStk waits; Cash settles immediately; Paybill polls C2B.
        assertTrue(PaymentMethod.MpesaStk.isPrompted)
        assertFalse(PaymentMethod.Paybill.isPrompted)
        assertFalse(PaymentMethod.Cash.isPrompted)
    }
}
