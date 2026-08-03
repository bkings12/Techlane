package com.techlane.ops.data

import androidx.room.Dao
import androidx.room.Database
import androidx.room.Entity
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.PrimaryKey
import androidx.room.Query
import androidx.room.RoomDatabase
import androidx.room.Update

@Entity(tableName = "sync_outbox")
data class SyncCommandEntity(
    @PrimaryKey val actionId: String,
    val tenantId: String,
    val branchId: String?,
    val deviceId: String?,
    val userId: String,
    val commandType: String,
    val localTimestamp: String,
    val payloadJson: String,
    val syncStatus: String,
    val retryCount: Int = 0,
    val lastError: String? = null,
)

@Dao
interface SyncOutboxDao {
    @Insert(onConflict = OnConflictStrategy.ABORT)
    suspend fun insert(cmd: SyncCommandEntity)

    @Update
    suspend fun update(cmd: SyncCommandEntity)

    @Query("SELECT * FROM sync_outbox WHERE syncStatus IN ('pending','failed','conflict') ORDER BY localTimestamp ASC LIMIT :limit")
    suspend fun pending(limit: Int = 50): List<SyncCommandEntity>

    @Query("SELECT * FROM sync_outbox ORDER BY localTimestamp DESC LIMIT :limit")
    suspend fun recent(limit: Int = 40): List<SyncCommandEntity>

    @Query("SELECT COUNT(*) FROM sync_outbox WHERE syncStatus IN ('pending','failed','syncing','conflict')")
    suspend fun pendingCount(): Int

    @Query("UPDATE sync_outbox SET syncStatus = 'pending', lastError = NULL WHERE actionId = :actionId")
    suspend fun requeue(actionId: String)

    @Query("DELETE FROM sync_outbox WHERE actionId = :actionId")
    suspend fun delete(actionId: String)

    @Query("DELETE FROM sync_outbox WHERE syncStatus IN ('synced','discarded')")
    suspend fun clearSettled()
}

@Database(entities = [SyncCommandEntity::class], version = 1)
abstract class AppDatabase : RoomDatabase() {
    abstract fun syncOutboxDao(): SyncOutboxDao
}
