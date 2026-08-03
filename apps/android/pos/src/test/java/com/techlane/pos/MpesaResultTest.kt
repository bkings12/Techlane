package com.techlane.pos

import com.techlane.pos.domain.model.MpesaResult
import com.techlane.pos.domain.model.StkFailure
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * The polling loop decides "stop" or "keep waiting" from these, so a
 * misclassification either strands the counter or declares a live prompt dead.
 */
class MpesaResultTest {

    @Test
    fun `maps the result codes staff hit daily`() {
        assertEquals(StkFailure.CancelledByCustomer, MpesaResult.classify("1032", "").first)
        assertEquals(StkFailure.NoResponse, MpesaResult.classify("1037", "").first)
        assertEquals(StkFailure.InsufficientFunds, MpesaResult.classify("1", "").first)
        assertEquals(StkFailure.SubscriberLocked, MpesaResult.classify("1001", "").first)
        assertEquals(StkFailure.WrongPin, MpesaResult.classify("2001", "").first)
    }

    @Test
    fun `falls back to the description when the code is unknown`() {
        assertEquals(
            StkFailure.InsufficientFunds,
            MpesaResult.classify("4771", "The balance is insufficient for the transaction").first,
        )
        assertEquals(
            StkFailure.CancelledByCustomer,
            MpesaResult.classify(null, "Request cancelled by user").first,
        )
    }

    @Test
    fun `pending responses are never treated as terminal`() {
        listOf(
            "STK pending: 4999 The transaction is being processed",
            "Request under processing",
            "still processing",
        ).forEach { message ->
            assertTrue("$message should be pending", MpesaResult.isStillProcessing(message))
            assertFalse("$message must not be terminal", MpesaResult.isTerminal(message))
        }
    }

    @Test
    fun `hard outcomes are terminal`() {
        listOf(
            "STK not paid: 1032 Request cancelled by user",
            "DS timeout user cannot be reached",
            "Insufficient funds",
        ).forEach { message ->
            assertTrue("$message should be terminal", MpesaResult.isTerminal(message))
        }
    }

    @Test
    fun `every failure carries advice on what to do next`() {
        StkFailure.entries.forEach { failure ->
            assertTrue(MpesaResult.recovery(failure).isNotBlank())
        }
    }
}
