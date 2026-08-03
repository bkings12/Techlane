package com.techlane.pos

import com.techlane.pos.core.util.Msisdn
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

/**
 * A prompt sent to the wrong number is money gone to a stranger, so the shapes
 * staff actually type are pinned here.
 */
class MsisdnTest {

    @Test
    fun `accepts the forms customers read out`() {
        val expected = "254712345678"
        listOf(
            "0712345678",
            "0712 345 678",
            "+254712345678",
            "254712345678",
            "712345678",
            "00254712345678",
        ).forEach { input ->
            assertEquals("failed on $input", expected, Msisdn.normalise(input))
        }
    }

    @Test
    fun `accepts the 01x Safaricom range`() {
        assertEquals("254110123456", Msisdn.normalise("0110123456"))
    }

    @Test
    fun `rejects landlines, short numbers and non-mobile prefixes`() {
        listOf("0202345678", "07123", "0812345678", "", "abcd", "25471234567890").forEach { input ->
            assertNull("should have rejected $input", Msisdn.normalise(input))
        }
    }

    @Test
    fun `formats back to the local form staff recognise`() {
        assertEquals("0712 345 678", Msisdn.formatLocal("254712345678"))
        assertEquals("0712 *** 678", Msisdn.mask("254712345678"))
    }
}
