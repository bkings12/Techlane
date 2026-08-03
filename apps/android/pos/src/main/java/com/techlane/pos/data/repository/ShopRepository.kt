package com.techlane.pos.data.repository

import com.techlane.pos.data.local.CatalogDao
import com.techlane.pos.data.local.CatalogItemEntity
import com.techlane.pos.data.local.ServiceDao
import com.techlane.pos.data.local.ServiceItemEntity
import com.techlane.pos.data.remote.TechLaneApi
import com.techlane.pos.data.remote.dto.BranchDto
import com.techlane.pos.data.remote.dto.PaymentSettingsDto
import com.techlane.pos.data.remote.dto.StockLocationDto
import com.techlane.pos.data.remote.toAppException
import kotlinx.coroutines.flow.Flow
import java.util.UUID
import javax.inject.Inject
import javax.inject.Singleton

/** Branches, stock locations, sellable stock, and the shop's service menu. */
@Singleton
class ShopRepository @Inject constructor(
    private val api: TechLaneApi,
    private val catalogDao: CatalogDao,
    private val serviceDao: ServiceDao,
) {

    suspend fun branches(): Result<List<BranchDto>> = runCatching {
        api.branches().items
    }.recoverCatching { throw it.toAppException() }

    suspend fun locations(branchId: String): Result<List<StockLocationDto>> = runCatching {
        api.stockLocations(branchId).items
    }.recoverCatching { throw it.toAppException() }

    suspend fun paymentSettings(): Result<PaymentSettingsDto> = runCatching {
        api.paymentSettings()
    }.recoverCatching { throw it.toAppException() }

    fun observeCatalog(locationId: String): Flow<List<CatalogItemEntity>> = catalogDao.observe(locationId)

    fun searchCatalog(locationId: String, query: String): Flow<List<CatalogItemEntity>> =
        catalogDao.search(locationId, query)

    /** Pulls the location's sellable stock into Room. Safe to call on every open. */
    suspend fun syncCatalog(locationId: String): Result<Int> = runCatching {
        val items = api.catalog(locationId).items
        val now = System.currentTimeMillis()
        catalogDao.replaceAll(
            locationId,
            items.map {
                CatalogItemEntity(
                    variantId = it.variantId,
                    locationId = locationId,
                    productName = it.productName,
                    brand = it.brand,
                    category = it.category,
                    sku = it.sku,
                    sellPrice = it.sellPrice,
                    availableQty = it.availableQty,
                    imageUrl = it.imageUrl,
                    updatedAt = now,
                )
            },
        )
        items.size
    }.recoverCatching { throw it.toAppException() }

    fun observeServices(): Flow<List<ServiceItemEntity>> = serviceDao.observe()

    /**
     * Seeds the service menu the first time the app runs. These are the jobs a
     * phone/laptop shop bills for daily; the list re-sorts itself by use, and
     * staff can add their own, so it converges on what this shop actually does.
     */
    suspend fun ensureServiceSeed() {
        if (serviceDao.count() > 0) return
        val now = System.currentTimeMillis()
        serviceDao.upsert(
            DEFAULT_SERVICES.mapIndexed { index, label ->
                ServiceItemEntity(
                    id = "seed-${label.lowercase().replace(Regex("[^a-z0-9]+"), "-")}",
                    label = label,
                    lastPrice = null,
                    // Seed order is a starting hint only — real use overtakes it fast.
                    useCount = DEFAULT_SERVICES.size - index,
                    lastUsedAt = now,
                    builtIn = true,
                )
            },
        )
    }

    suspend fun addService(label: String, price: Double?): ServiceItemEntity {
        val entity = ServiceItemEntity(
            id = UUID.randomUUID().toString(),
            label = label.trim(),
            lastPrice = price,
            useCount = 1,
            lastUsedAt = System.currentTimeMillis(),
            builtIn = false,
        )
        serviceDao.upsert(listOf(entity))
        return entity
    }

    suspend fun recordServiceUse(id: String, price: Double?) {
        serviceDao.recordUse(id, price, System.currentTimeMillis())
    }

    private companion object {
        val DEFAULT_SERVICES = listOf(
            "Screen replacement",
            "Battery replacement",
            "Charging port repair",
            "Software / flashing",
            "Water damage treatment",
            "Laptop keyboard replacement",
            "Laptop hinge repair",
            "Data recovery",
            "Diagnostics",
            "Cleaning & servicing",
            "Speaker / mic repair",
            "Camera replacement",
        )
    }
}
