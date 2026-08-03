package com.techlane.pos

import com.techlane.pos.data.update.AvailableUpdate
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

private fun update(notes: String?) = AvailableUpdate(
    versionCode = 7,
    versionName = "1.0.6",
    downloadUrl = "https://example.test/app.apk",
    notes = notes,
    mandatory = false,
)

/**
 * Release notes are free text typed by whoever cut the build, so the prompt has
 * to make bullets out of whatever shape they arrive in.
 */
class AvailableUpdateTest {

    @Test
    fun `newline-separated notes become one bullet each`() {
        assertEquals(
            listOf("Faster jobs board", "Bluetooth printing fixes"),
            update("Faster jobs board\nBluetooth printing fixes").noteLines,
        )
    }

    @Test
    fun `bullet and semicolon separated notes split too`() {
        assertEquals(listOf("One", "Two"), update("One • Two").noteLines)
        assertEquals(listOf("One", "Two"), update("One; Two").noteLines)
    }

    @Test
    fun `leading dashes and asterisks are stripped from hand-typed lists`() {
        assertEquals(listOf("Fixed printing", "Faster search"), update("- Fixed printing\n* Faster search").noteLines)
    }

    @Test
    fun `blank lines are dropped rather than becoming empty bullets`() {
        assertEquals(listOf("Only this"), update("\n\nOnly this\n\n").noteLines)
    }

    @Test
    fun `absent or blank notes produce no bullets`() {
        assertTrue(update(null).noteLines.isEmpty())
        assertTrue(update("   ").noteLines.isEmpty())
    }

    @Test
    fun `a single line with no separators stays one bullet`() {
        assertEquals(listOf("POS 1.0.6"), update("POS 1.0.6").noteLines)
    }
}
