package com.techlane.pos.domain.model

import java.util.Calendar

/**
 * What came in with the device. These are the disputes a shop actually has —
 * "I left my charger with you" — so they are one tap each and print on the slip.
 *
 * The wire values go into the backend's `condition_tags`, which is free-form
 * text, so they are written the way they should read on paper.
 */
enum class IntakeAccessory(val wire: String, val label: String) {
    Charger("Charger", "Charger"),
    Case("Bag / case", "Bag / case"),
    SimCard("SIM card", "SIM card"),
    MemoryCard("Memory card", "Memory card"),
    Battery("Spare battery", "Battery"),
    ;

    companion object {
        val ALL: List<IntakeAccessory> = entries
    }
}

/**
 * The faults a counter actually types, per device type. Tapping one appends to
 * the free-text field rather than replacing it — the chip is a shortcut, never
 * a constraint on what the technician can record.
 */
object CommonIssues {
    private val phone = listOf(
        "Screen broken", "Not charging", "Battery", "No power", "Water damage", "Network issue",
    )
    private val laptop = listOf(
        "No power", "Not charging", "Broken screen", "Keyboard", "Overheating", "Slow", "Liquid damage",
    )
    private val generic = listOf("No power", "Not charging", "Physical damage", "Not working")

    fun forKind(kind: DeviceKind): List<String> = when (kind) {
        DeviceKind.Phone, DeviceKind.Tablet -> phone
        DeviceKind.Laptop -> laptop
        DeviceKind.Other -> generic
    }
}

/**
 * When the shop told the customer to come back.
 *
 * Quick choices cover the overwhelming majority of counter promises; anything
 * else falls through to picking a date. Times land on a sensible closing-ish
 * hour rather than "now + 24h", because a promise of "tomorrow at 09:47" is
 * not a promise anyone made out loud.
 */
sealed interface PromiseOption {
    val label: String

    data object Today : PromiseOption {
        override val label = "Today"
    }

    data object Tomorrow : PromiseOption {
        override val label = "Tomorrow"
    }

    data object InTwoDays : PromiseOption {
        override val label = "+2 days"
    }

    /** An explicitly chosen instant, already resolved to a wall-clock time. */
    data class Exact(val epochMillis: Long, override val label: String) : PromiseOption

    /** Resolves to an epoch millisecond, or null when nothing was promised. */
    fun at(hourOfDay: Int = DEFAULT_HOUR, minute: Int = 0): Long? = when (this) {
        is Exact -> epochMillis
        Today -> dayAt(0, hourOfDay, minute)
        Tomorrow -> dayAt(1, hourOfDay, minute)
        InTwoDays -> dayAt(2, hourOfDay, minute)
    }

    companion object {
        /** Late afternoon: the shop's usual "come back by" hour. */
        const val DEFAULT_HOUR = 17

        val QUICK: List<PromiseOption> = listOf(Today, Tomorrow, InTwoDays)

        private fun dayAt(daysAhead: Int, hour: Int, minute: Int): Long =
            Calendar.getInstance().apply {
                add(Calendar.DAY_OF_YEAR, daysAhead)
                set(Calendar.HOUR_OF_DAY, hour)
                set(Calendar.MINUTE, minute)
                set(Calendar.SECOND, 0)
                set(Calendar.MILLISECOND, 0)
            }.timeInMillis
    }
}
