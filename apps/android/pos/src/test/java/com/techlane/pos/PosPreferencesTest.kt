package com.techlane.pos

import com.techlane.pos.data.session.PosPreferences
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Role-derived gates. Roles are written only when `GET /me` succeeds, which is
 * best-effort, so what these do with an *empty* list matters as much as what
 * they do with a real one.
 */
class PosPreferencesTest {

    @Test
    fun `counter and bench roles are offered intake`() {
        listOf("owner", "manager", "cashier", "technician").forEach { role ->
            assertTrue(role, PosPreferences(roles = listOf(role)).canCreateIntake)
        }
    }

    @Test
    fun `an unknown role is not offered intake`() {
        assertFalse(PosPreferences(roles = listOf("auditor")).canCreateIntake)
    }

    @Test
    fun `unknown roles fail open so the primary action is never silently hidden`() {
        // Regression: gating this closed on an empty list meant a handset that
        // signed in on a bad connection showed no New Intake button at all,
        // with nothing on screen explaining why.
        assertTrue(PosPreferences(roles = emptyList()).canCreateIntake)
    }

    @Test
    fun `reconcile stays restricted to owners and managers`() {
        assertTrue(PosPreferences(roles = listOf("owner")).canForceReconcile)
        assertTrue(PosPreferences(roles = listOf("manager")).canForceReconcile)
        assertFalse(PosPreferences(roles = listOf("technician")).canForceReconcile)
        // Unlike intake, this one fails closed on purpose: it is a privileged
        // action, and offering it to someone who will be refused is worse than
        // not showing it.
        assertFalse(PosPreferences(roles = emptyList()).canForceReconcile)
    }

    @Test
    fun `the till needs both a branch and a stock location`() {
        assertFalse(PosPreferences(branchId = "b").tillReady)
        assertFalse(PosPreferences(locationId = "l").tillReady)
        assertTrue(PosPreferences(branchId = "b", locationId = "l").tillReady)
    }
}
