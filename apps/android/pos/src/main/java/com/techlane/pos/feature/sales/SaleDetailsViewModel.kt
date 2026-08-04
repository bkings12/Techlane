package com.techlane.pos.feature.sales

import android.content.Context
import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.techlane.pos.core.print.PdfShare
import com.techlane.pos.core.print.ReceiptPrinter
import com.techlane.pos.data.printer.PrinterConnectionException
import com.techlane.pos.data.printer.PrinterRepository
import com.techlane.pos.data.repository.SalesRepository
import com.techlane.pos.data.session.PreferencesStore
import com.techlane.pos.domain.model.SaleDetail
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

data class SaleDetailsUiState(
    val loading: Boolean = true,
    val sale: SaleDetail? = null,
    val error: String? = null,
    val canSeeCost: Boolean = false,
    val printing: Boolean = false,
    val sharing: Boolean = false,
    val message: String? = null,
    /** Set specifically when a print failed because no printer is connected —
     *  distinct from [error] so the UI can show "Receipt ready — printer not
     *  connected" instead of implying the sale itself failed. */
    val printerDisconnected: Boolean = false,
    val viewingReceiptHtml: String? = null,
)

@HiltViewModel
class SaleDetailsViewModel @Inject constructor(
    savedStateHandle: SavedStateHandle,
    private val salesRepo: SalesRepository,
    private val printers: PrinterRepository,
    private val prefs: PreferencesStore,
) : ViewModel() {

    private val saleId: String = checkNotNull(savedStateHandle["saleId"])

    private val _state = MutableStateFlow(SaleDetailsUiState())
    val state: StateFlow<SaleDetailsUiState> = _state.asStateFlow()

    init {
        viewModelScope.launch {
            _state.update { it.copy(canSeeCost = prefs.preferences.first().canSeeCost) }
        }
        load()
    }

    fun load() {
        viewModelScope.launch {
            _state.update { it.copy(loading = true, error = null) }
            salesRepo.getSale(saleId)
                .onSuccess { sale -> _state.update { it.copy(sale = sale) } }
                .onFailure { e -> _state.update { it.copy(error = e.message ?: "Could not load this sale") } }
            _state.update { it.copy(loading = false) }
        }
    }

    fun clearFeedback() = _state.update { it.copy(message = null, error = null, printerDisconnected = false) }

    /** Print goes through PrinterRepository — the same Bluetooth ESC/POS
     *  service every other receipt in the app uses. No parallel printer path. */
    fun printReceipt() {
        if (_state.value.printing) return
        _state.update { it.copy(printing = true, printerDisconnected = false, error = null) }
        viewModelScope.launch {
            printers.printSaleReceipt(saleId)
                .onSuccess { _state.update { it.copy(message = "Receipt sent to the printer") } }
                .onFailure { e ->
                    if (e is PrinterConnectionException) {
                        // The sale itself is fine — only the printer isn't
                        // reachable. Never phrase this as a payment problem.
                        _state.update { it.copy(printerDisconnected = true, message = "Receipt ready — printer not connected") }
                    } else {
                        _state.update { it.copy(error = e.message ?: "Could not print the receipt") }
                    }
                }
            _state.update { it.copy(printing = false) }
        }
    }

    fun shareReceipt(context: Context) {
        if (_state.value.sharing) return
        _state.update { it.copy(sharing = true, error = null) }
        viewModelScope.launch {
            val sale = _state.value.sale
            salesRepo.receiptPdf(saleId)
                .onSuccess { bytes ->
                    val name = "${sale?.reference ?: saleId.take(8)}-receipt.pdf"
                    runCatching { PdfShare.share(context, bytes, name, "TechLane receipt") }
                        .onFailure { e -> _state.update { it.copy(error = e.message ?: "Could not share the receipt") } }
                }
                .onFailure { e -> _state.update { it.copy(error = e.message ?: "Could not load the receipt to share") } }
            _state.update { it.copy(sharing = false) }
        }
    }

    fun viewReceipt() {
        viewModelScope.launch {
            salesRepo.receiptHtml(saleId)
                .onSuccess { html -> _state.update { it.copy(viewingReceiptHtml = html) } }
                .onFailure { e -> _state.update { it.copy(error = e.message ?: "Could not load the receipt") } }
        }
    }

    fun dismissReceiptView() = _state.update { it.copy(viewingReceiptHtml = null) }

    /** "Share" fallback for the HTML view, matching the existing app-wide pattern. */
    fun shareReceiptHtml(context: Context) {
        _state.value.viewingReceiptHtml?.let { ReceiptPrinter.share(context, it, "TechLane receipt") }
    }
}
