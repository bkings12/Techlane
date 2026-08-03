package com.techlane.pos.data.printer

import android.content.Context
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.intPreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map
import javax.inject.Inject
import javax.inject.Singleton

private val Context.printerDataStore by preferencesDataStore(name = "techlane_pos_printer")

/**
 * The saved printer, kept in its own DataStore file rather than folded into
 * the general [com.techlane.pos.data.session.PreferencesStore] — a peripheral
 * a till is wired to is a different lifetime than the signed-in session or
 * branch it's parked at, and shouldn't get cleared by the same code path that
 * clears those on sign-out.
 */
@Singleton
class PrinterPreferencesStore @Inject constructor(
    @ApplicationContext context: Context,
) {
    private val store = context.printerDataStore

    val preferences: Flow<PrinterPreferences> = store.data.map { it.toPrinterPreferences() }

    suspend fun savePrinter(device: PrinterDevice) = store.edit {
        it[KEY_NAME] = device.name
        it[KEY_ADDRESS] = device.address
    }

    suspend fun clearPrinter() = store.edit {
        it.remove(KEY_NAME)
        it.remove(KEY_ADDRESS)
    }

    suspend fun setPaperWidth(width: PaperWidth) = store.edit { it[KEY_PAPER_MM] = width.millimetres }

    suspend fun setAutoPrintEnabled(enabled: Boolean) = store.edit { it[KEY_AUTO_PRINT] = enabled }

    private fun Preferences.toPrinterPreferences() = PrinterPreferences(
        name = this[KEY_NAME],
        address = this[KEY_ADDRESS],
        paperWidth = this[KEY_PAPER_MM]?.let(PaperWidth::fromMillimetres) ?: PaperWidth.DEFAULT,
        autoPrintEnabled = this[KEY_AUTO_PRINT] ?: false,
    )

    private companion object {
        val KEY_NAME = stringPreferencesKey("printer_name")
        val KEY_ADDRESS = stringPreferencesKey("printer_address")
        val KEY_PAPER_MM = intPreferencesKey("printer_paper_mm")
        val KEY_AUTO_PRINT = booleanPreferencesKey("printer_auto_print")
    }
}

data class PrinterPreferences(
    val name: String? = null,
    val address: String? = null,
    val paperWidth: PaperWidth = PaperWidth.DEFAULT,
    /** Not acted on anywhere yet — reserved for the receipt/payment integration. */
    val autoPrintEnabled: Boolean = false,
) {
    val isConfigured: Boolean get() = !address.isNullOrBlank()

    val device: PrinterDevice?
        get() = address?.takeIf { it.isNotBlank() }?.let { PrinterDevice(name = name.orEmpty(), address = it) }
}
