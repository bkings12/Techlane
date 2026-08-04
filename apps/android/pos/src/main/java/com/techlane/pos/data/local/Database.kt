package com.techlane.pos.data.local

import androidx.room.Dao
import androidx.room.Database
import androidx.room.Entity
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.PrimaryKey
import androidx.room.Query
import androidx.room.RoomDatabase
import androidx.room.Transaction
import kotlinx.coroutines.flow.Flow

/**
 * Catalog snapshot for the current stock location. The counter must still be
 * able to pick "what we're selling" when the shop's link drops; only the charge
 * itself needs the network.
 */
@Entity(tableName = "catalog_items")
data class CatalogItemEntity(
    @PrimaryKey val variantId: String,
    val locationId: String,
    val productName: String,
    val brand: String?,
    val category: String?,
    val sku: String,
    val sellPrice: Double,
    val availableQty: Int,
    val imageUrl: String?,
    val updatedAt: Long,
)

/** Named services (screen replacement, diagnostics…) with the price last charged. */
@Entity(tableName = "service_items")
data class ServiceItemEntity(
    @PrimaryKey val id: String,
    val label: String,
    val lastPrice: Double?,
    val useCount: Int,
    val lastUsedAt: Long,
    val builtIn: Boolean,
)

/** Local ledger of prompts sent from this device, for the Activity tab. */
@Entity(tableName = "charge_records")
data class ChargeRecordEntity(
    @PrimaryKey val id: String,
    val amount: Double,
    val method: String,
    val phone: String?,
    val label: String,
    val status: String,
    val saleId: String?,
    val paymentId: String?,
    val receipt: String?,
    val failureReason: String?,
    val createdAt: Long,
    val updatedAt: Long,
)

@Dao
interface CatalogDao {
    @Query("SELECT * FROM catalog_items WHERE locationId = :locationId ORDER BY productName")
    fun observe(locationId: String): Flow<List<CatalogItemEntity>>

    @Query(
        """
        SELECT * FROM catalog_items
        WHERE locationId = :locationId
          AND (productName LIKE '%' || :query || '%' OR sku LIKE '%' || :query || '%' OR brand LIKE '%' || :query || '%')
        ORDER BY productName LIMIT 60
        """,
    )
    fun search(locationId: String, query: String): Flow<List<CatalogItemEntity>>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsert(items: List<CatalogItemEntity>)

    @Query("DELETE FROM catalog_items WHERE locationId = :locationId")
    suspend fun clear(locationId: String)

    @Transaction
    suspend fun replaceAll(locationId: String, items: List<CatalogItemEntity>) {
        clear(locationId)
        upsert(items)
    }
}

@Dao
interface ServiceDao {
    @Query("SELECT * FROM service_items ORDER BY useCount DESC, label ASC")
    fun observe(): Flow<List<ServiceItemEntity>>

    @Query("SELECT * FROM service_items WHERE id = :id")
    suspend fun byId(id: String): ServiceItemEntity?

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsert(items: List<ServiceItemEntity>)

    @Query("SELECT COUNT(*) FROM service_items")
    suspend fun count(): Int

    /** Learns from use so a shop's real services rise to the top of the picker. */
    @Query(
        """
        UPDATE service_items
        SET useCount = useCount + 1, lastUsedAt = :now, lastPrice = :price
        WHERE id = :id
        """,
    )
    suspend fun recordUse(id: String, price: Double?, now: Long)
}

@Dao
interface ChargeDao {
    @Query("SELECT * FROM charge_records ORDER BY createdAt DESC LIMIT 100")
    fun observeRecent(): Flow<List<ChargeRecordEntity>>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsert(record: ChargeRecordEntity)

    @Query("SELECT * FROM charge_records WHERE status IN ('pending', 'sent') ORDER BY createdAt DESC")
    suspend fun unresolved(): List<ChargeRecordEntity>

    @Query("SELECT * FROM charge_records WHERE id = :id")
    suspend fun byId(id: String): ChargeRecordEntity?

    @Query("UPDATE charge_records SET status = :status, receipt = :receipt, failureReason = :reason, updatedAt = :now WHERE id = :id")
    suspend fun updateStatus(id: String, status: String, receipt: String?, reason: String?, now: Long)
}

@Database(
    entities = [
        CatalogItemEntity::class,
        ServiceItemEntity::class,
        ChargeRecordEntity::class,
        JobEntity::class,
        JobNoteEntity::class,
        JobStatusEventEntity::class,
        JobEstimateEntity::class,
        JobPartEntity::class,
        JobPhotoEntity::class,
        TechnicianEntity::class,
        JobOutboxEntity::class,
    ],
    version = 3,
    exportSchema = false,
)
abstract class PosDatabase : RoomDatabase() {
    abstract fun catalogDao(): CatalogDao
    abstract fun serviceDao(): ServiceDao
    abstract fun chargeDao(): ChargeDao
    abstract fun jobDao(): JobDao
}
