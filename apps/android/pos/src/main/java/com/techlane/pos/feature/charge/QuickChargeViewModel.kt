package com.techlane.pos.feature.charge

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.techlane.pos.core.designsystem.theme.ThemeMode
import com.techlane.pos.core.print.ReceiptPrinter
import com.techlane.pos.core.util.Msisdn
import com.techlane.pos.data.local.CatalogItemEntity
import com.techlane.pos.data.local.ServiceItemEntity
import com.techlane.pos.data.repository.ChargeRepository
import com.techlane.pos.data.repository.ShopRepository
import com.techlane.pos.data.session.PosPreferences
import com.techlane.pos.data.session.PreferencesStore
import com.techlane.pos.domain.model.ChargeRequest
import com.techlane.pos.domain.model.ChargeTarget
import com.techlane.pos.domain.model.PaymentMethod
import com.techlane.pos.domain.model.StkFailure
import com.techlane.pos.domain.model.StkStage
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.flatMapLatest
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.util.UUID
import javax.inject.Inject

data class ChargeUiState(
    val amountDigits: String = "",
    val phone: String = "",
    val target: ChargeTarget = ChargeTarget.None,
    val method: PaymentMethod = PaymentMethod.MpesaStk,
    val prefs: PosPreferences = PosPreferences(),
    val mpesaConfigured: Boolean = true,
    val validationError: String? = null,
    /** Non-null while a charge is in flight or resolved — the UI blocks on it. */
    val stage: StkStage? = null,
    val refreshing: Boolean = false,
    val receiptBusy: Boolean = false,
    val receiptError: String? = null,
) {
    val amount: Double get() = amountDigits.toDoubleOrNull() ?: 0.0
    val hasAmount: Boolean get() = amount > 0
    val normalisedPhone: String? get() = Msisdn.normalise(phone)
    val phoneValid: Boolean get() = normalisedPhone != null

    val canCharge: Boolean
        get() = hasAmount &&
            prefs.tillReady &&
            stage == null &&
            (method == PaymentMethod.Cash || (phoneValid && mpesaConfigured))

    /** Why the charge button is off, phrased as the next thing to do. */
    val blockedReason: String?
        get() = when {
            !prefs.tillReady -> "Pick a branch and stock location in Settings before charging."
            !hasAmount -> null
            method == PaymentMethod.MpesaStk && !mpesaConfigured ->
                "M-Pesa isn't set up for this shop yet. Take cash, or ask an owner to finish setup."
            method == PaymentMethod.MpesaStk && !phoneValid -> "Enter the customer's M-Pesa number."
            else -> null
        }

    /** True once the stage can no longer change on its own. */
    val stageIsTerminal: Boolean
        get() = stage is StkStage.Paid || stage is StkStage.Failed || stage is StkStage.TimedOut
}

@HiltViewModel
class QuickChargeViewModel @Inject constructor(
    private val charges: ChargeRepository,
    private val shop: ShopRepository,
    private val prefs: PreferencesStore,
) : ViewModel() {

    private val _state = MutableStateFlow(ChargeUiState())
    val state: StateFlow<ChargeUiState> = _state.asStateFlow()

    private val searchQuery = MutableStateFlow("")

    /**
     * Last attempt, kept so "Try again" and "Check again" have something to act
     * on. Its idempotency key doubles as the local record id, which is where
     * the payment and sale ids actually live.
     */
    private var lastRequest: ChargeRequest? = null
    private var chargeJob: Job? = null

    @OptIn(kotlinx.coroutines.ExperimentalCoroutinesApi::class)
    val products: StateFlow<List<CatalogItemEntity>> =
        combine(prefs.preferences.map { it.locationId }, searchQuery) { locationId, query ->
            locationId to query.trim()
        }.flatMapLatest { (locationId, query) ->
            when {
                locationId.isNullOrBlank() -> flowOf(emptyList())
                query.isBlank() -> shop.observeCatalog(locationId)
                else -> shop.searchCatalog(locationId, query)
            }
        }.stateIn(viewModelScope, SharingStarted.WhileSubscribed(5_000), emptyList())

    val services: StateFlow<List<ServiceItemEntity>> = shop.observeServices()
        .stateIn(viewModelScope, SharingStarted.WhileSubscribed(5_000), emptyList())

    init {
        viewModelScope.launch {
            shop.ensureServiceSeed()
            prefs.preferences.collect { preferences ->
                val locationChanged = preferences.locationId != _state.value.prefs.locationId
                _state.update {
                    it.copy(
                        prefs = preferences,
                        phone = if (it.phone.isBlank()) preferences.lastPhone.orEmpty() else it.phone,
                    )
                }
                if (locationChanged && !preferences.locationId.isNullOrBlank()) {
                    shop.syncCatalog(preferences.locationId)
                }
            }
        }
        refreshPaymentSettings()
    }

    fun refreshPaymentSettings() {
        viewModelScope.launch {
            shop.paymentSettings().onSuccess { settings ->
                _state.update { it.copy(mpesaConfigured = settings.configured && settings.mpesaEnabled) }
            }
        }
    }

    // -------------------------------------------------------------- entry edits

    fun onAmountChange(digits: String) =
        _state.update { it.copy(amountDigits = digits.filter(Char::isDigit), validationError = null) }

    fun onPhoneChange(value: String) = _state.update { it.copy(phone = value, validationError = null) }

    fun onMethodChange(method: PaymentMethod) = _state.update { it.copy(method = method, validationError = null) }

    fun onSearchQueryChange(value: String) { searchQuery.value = value }

    /**
     * Light/dark from the top bar. Shop lighting changes through the day and a
     * technician should not have to dig through Settings to keep the screen
     * readable, so this writes an explicit mode rather than toggling "match phone".
     */
    fun setThemeMode(mode: ThemeMode) {
        viewModelScope.launch { prefs.setThemeMode(mode) }
    }

    /**
     * Choosing what's being sold is optional, but when it is chosen it should
     * also answer "how much" — a technician who picked a 2,500/- screen should
     * not have to type 2500 as well.
     */
    fun selectProduct(item: CatalogItemEntity, quantity: Int = 1) {
        val target = ChargeTarget.Product(
            variantId = item.variantId,
            name = item.productName,
            unitPrice = item.sellPrice,
            quantity = quantity,
            availableQty = item.availableQty,
        )
        _state.update {
            it.copy(
                target = target,
                amountDigits = target.suggestedAmount.toDigits() ?: it.amountDigits,
                validationError = null,
            )
        }
    }

    fun selectService(item: ServiceItemEntity) {
        val target = ChargeTarget.Service(item.id, item.label, item.lastPrice)
        _state.update {
            it.copy(
                target = target,
                // Services are priced per job; a remembered price is a hint, not a rule.
                amountDigits = target.suggestedAmount?.toDigits() ?: it.amountDigits,
                validationError = null,
            )
        }
    }

    /**
     * Quantity for a picked product. The amount follows it, because for a
     * catalog line the server prices from the variant and quantity — whatever
     * is typed in the amount box is ignored, so the two must agree or the
     * technician is shown a total the customer will never be charged.
     */
    fun setProductQuantity(quantity: Int) {
        val product = _state.value.target as? ChargeTarget.Product ?: return
        val capped = quantity.coerceAtLeast(1)
        val target = product.copy(quantity = capped)
        _state.update {
            it.copy(
                target = target,
                amountDigits = target.suggestedAmount.toDigits() ?: it.amountDigits,
                validationError = null,
            )
        }
    }

    fun addAndSelectService(label: String, price: Double?) {
        if (label.isBlank()) return
        viewModelScope.launch {
            val created = shop.addService(label, price)
            selectService(created)
        }
    }

    fun clearTarget() = _state.update { it.copy(target = ChargeTarget.None) }

    /** Pull-to-refresh: re-read stock and whether M-Pesa is still switched on. */
    fun refresh() {
        if (_state.value.refreshing) return
        _state.update { it.copy(refreshing = true) }
        viewModelScope.launch {
            val locationId = prefs.preferences.first().locationId
            if (!locationId.isNullOrBlank()) shop.syncCatalog(locationId)
            shop.paymentSettings().onSuccess { settings ->
                _state.update { it.copy(mpesaConfigured = settings.configured && settings.mpesaEnabled) }
            }
            _state.update { it.copy(refreshing = false) }
        }
    }

    // -------------------------------------------------------------- the charge

    fun charge() {
        val current = _state.value
        if (!current.canCharge) {
            _state.update { it.copy(validationError = it.blockedReason ?: "Enter an amount first.") }
            return
        }
        val branchId = current.prefs.branchId ?: return
        val locationId = current.prefs.locationId ?: return

        val request = ChargeRequest(
            branchId = branchId,
            locationId = locationId,
            amount = current.amount,
            method = current.method,
            phone = current.normalisedPhone,
            target = current.target,
            idempotencyKey = UUID.randomUUID().toString(),
        )
        lastRequest = request

        viewModelScope.launch { prefs.setLastPhone(current.phone.takeIf { it.isNotBlank() }) }
        runCharge { charges.charge(request) }
    }

    /** Re-checks an unconfirmed prompt without sending a new one. */
    fun checkAgain() {
        val request = lastRequest ?: return
        runCharge { charges.verify(request.idempotencyKey, request.locationId) }
    }

    private fun runCharge(source: () -> kotlinx.coroutines.flow.Flow<StkStage>) {
        chargeJob?.cancel()
        chargeJob = viewModelScope.launch {
            _state.update { it.copy(stage = StkStage.Sending, validationError = null) }
            source().collect { stage -> _state.update { it.copy(stage = stage) } }
        }
    }

    /**
     * Stops watching without pretending the prompt is dead — an M-Pesa request
     * we stopped looking at may still be paid a minute later, so this parks it
     * as unconfirmed rather than failed.
     */
    fun stopWaiting() {
        chargeJob?.cancel()
        chargeJob = null
        // The payment id it would need lives in the local charge record, which
        // is what "Check again" reads — nothing is lost by parking it here.
        _state.update { it.copy(stage = StkStage.TimedOut(null)) }
    }

    /**
     * Fetches the rendered receipt for the sale just paid and hands it to the
     * print spooler. The sale id comes off the Paid stage — a receipt only
     * exists once the server has actually completed the sale.
     */
    fun printReceipt(context: android.content.Context) {
        val saleId = (_state.value.stage as? StkStage.Paid)?.saleId
        if (saleId == null) {
            _state.update { it.copy(receiptError = "This charge has no completed sale to print yet.") }
            return
        }
        printSale(context, saleId)
    }

    fun printSale(context: android.content.Context, saleId: String) {
        if (_state.value.receiptBusy) return
        _state.update { it.copy(receiptBusy = true, receiptError = null) }
        viewModelScope.launch {
            charges.receiptHtml(saleId)
                .onSuccess { html -> ReceiptPrinter.print(context, html, "TechLane receipt") }
                .onFailure { error ->
                    _state.update { it.copy(receiptError = error.message ?: "Could not load the receipt") }
                }
            _state.update { it.copy(receiptBusy = false) }
        }
    }

    /** Sends the receipt out over WhatsApp/email/anything the phone has. */
    fun shareReceipt(context: android.content.Context) {
        val saleId = (_state.value.stage as? StkStage.Paid)?.saleId ?: return
        if (_state.value.receiptBusy) return
        _state.update { it.copy(receiptBusy = true, receiptError = null) }
        viewModelScope.launch {
            charges.receiptHtml(saleId)
                .onSuccess { html -> ReceiptPrinter.share(context, html, "TechLane receipt") }
                .onFailure { error ->
                    _state.update { it.copy(receiptError = error.message ?: "Could not load the receipt") }
                }
            _state.update { it.copy(receiptBusy = false) }
        }
    }

    /** Clears the result and readies the till for the next customer. */
    fun finishAndReset(keepPhone: Boolean = false) {
        chargeJob?.cancel()
        chargeJob = null
        lastRequest = null
        _state.update {
            it.copy(
                stage = null,
                receiptError = null,
                amountDigits = "",
                target = ChargeTarget.None,
                phone = if (keepPhone) it.phone else "",
                validationError = null,
            )
        }
    }

    /** Dismisses a resolved result but keeps the entry, so staff can retry fast. */
    fun dismissResult() {
        chargeJob?.cancel()
        chargeJob = null
        _state.update { it.copy(stage = null) }
    }

    fun switchToCashAndCharge() {
        _state.update { it.copy(stage = null, method = PaymentMethod.Cash) }
        charge()
    }

    fun retryFailed() {
        val failure = (_state.value.stage as? StkStage.Failed)?.reason
        _state.update { it.copy(stage = null) }
        // A bad number is the one failure where re-sending unchanged is pointless.
        if (failure == StkFailure.InvalidNumber) return
        charge()
    }
}

private fun Double?.toDigits(): String? =
    this?.takeIf { it > 0 }?.let { java.math.BigDecimal(it).setScale(0, java.math.RoundingMode.HALF_UP).toPlainString() }
