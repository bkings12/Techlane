package com.techlane.pos.data.local

import androidx.room.Dao
import androidx.room.Entity
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.PrimaryKey
import androidx.room.Query
import androidx.room.Transaction
import kotlinx.coroutines.flow.Flow

/**
 * The board row. Kept flat and small: a technician on a bad connection should
 * still get the whole board instantly from disk.
 */
@Entity(tableName = "jobs")
data class JobEntity(
    @PrimaryKey val id: String,
    val jobCode: String,
    val customerName: String?,
    val customerPhone: String?,
    val customerId: String?,
    val deviceKind: String,
    val deviceBrand: String?,
    val deviceModel: String?,
    val deviceImei: String?,
    val deviceSerial: String?,
    val status: String,
    val technicianId: String?,
    val problemSummary: String,
    val createdAt: Long,
    val promisedBy: Long?,
    val customerWaiting: Boolean,
    val authorizedAt: Long?,
    val authorizationSource: String?,
    val authorizedAmount: Double?,
    val pendingEstimateTotal: Double?,
    val amountDue: Double,
    val balanceDue: Double,
    val laborAmount: Double,
    /** When this row was last refreshed from the server, for staleness hints. */
    val syncedAt: Long,
)

@Entity(tableName = "job_notes")
data class JobNoteEntity(
    @PrimaryKey val id: String,
    val jobId: String,
    val note: String,
    val authorName: String?,
    val createdAt: Long,
    /** True while this note exists only on this handset. */
    val pending: Boolean,
)

@Entity(tableName = "job_status_events")
data class JobStatusEventEntity(
    @PrimaryKey val id: String,
    val jobId: String,
    val status: String,
    val note: String?,
    val at: Long,
)

@Entity(tableName = "job_estimates")
data class JobEstimateEntity(
    @PrimaryKey val id: String,
    val jobId: String,
    val total: Double,
    val status: String,
    val notes: String?,
    val createdAt: Long,
)

/**
 * A work-order line item: labour, a repair part, or a retail product — see
 * [lineType]. Parts/products predate the labour/product split and used to be
 * the only thing this table held (hence the name); [lineType] defaults to
 * "product" so old cached rows from before this field existed still render
 * somewhere sensible after a destructive migration re-syncs them.
 */
@Entity(tableName = "job_parts")
data class JobPartEntity(
    @PrimaryKey val id: String,
    val jobId: String,
    val lineId: String?,
    val variantId: String?,
    val name: String,
    val sku: String?,
    val quantity: Int,
    val unitPrice: Double,
    val pending: Boolean,
    /** "labour" | "part" | "product" — matches internal/repair's line_type. */
    val lineType: String = "product",
    val unitCost: Double? = null,
    /** "required" | "sourcing" | "ordered" | "received" | "installed" | "returned" | "cancelled" — parts only. */
    val partStatus: String? = null,
    /** "inventory" | "sourced" — parts only. */
    val partSource: String? = null,
    val supplierName: String? = null,
)

@Entity(tableName = "job_photos")
data class JobPhotoEntity(
    @PrimaryKey val id: String,
    val jobId: String,
    val remoteId: String?,
    val kind: String,
    val caption: String?,
    val localPath: String?,
    val createdAt: Long,
    val uploaded: Boolean,
    val uploadFailed: Boolean,
)

@Entity(tableName = "job_technicians")
data class TechnicianEntity(
    @PrimaryKey val id: String,
    val displayName: String,
    val email: String,
)

/**
 * A mutation waiting for connectivity.
 *
 * This is the piece that makes the promise "never lose technician-entered
 * diagnosis or notes" true: the write lands in Room first and is only removed
 * once the server has taken it. [attempts] and [lastError] exist so a
 * permanently rejected action (a transition the server refuses, say) surfaces
 * to the technician instead of retrying silently forever.
 */
@Entity(tableName = "job_outbox")
data class JobOutboxEntity(
    @PrimaryKey val id: String,
    val jobId: String,
    val type: String,
    /** JSON payload; shape is owned by JobOutbox.Action. */
    val payload: String,
    val createdAt: Long,
    val attempts: Int = 0,
    val lastError: String? = null,
)

@Dao
interface JobDao {

    @Query("SELECT * FROM jobs ORDER BY customerWaiting DESC, promisedBy IS NULL, promisedBy ASC, createdAt DESC")
    fun observeAll(): Flow<List<JobEntity>>

    @Query("SELECT * FROM jobs WHERE id = :id")
    fun observeJob(id: String): Flow<JobEntity?>

    @Query("SELECT * FROM jobs WHERE id = :id")
    suspend fun job(id: String): JobEntity?

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsertJobs(jobs: List<JobEntity>)

    @Query("UPDATE jobs SET status = :status WHERE id = :id")
    suspend fun setStatus(id: String, status: String)

    @Query("UPDATE jobs SET technicianId = :technicianId WHERE id = :id")
    suspend fun setTechnician(id: String, technicianId: String?)

    @Query("UPDATE jobs SET authorizedAt = :at, authorizationSource = :source, authorizedAmount = :amount WHERE id = :id")
    suspend fun setAuthorization(id: String, at: Long?, source: String?, amount: Double?)

    // ---- notes

    @Query("SELECT * FROM job_notes WHERE jobId = :jobId ORDER BY createdAt DESC")
    fun observeNotes(jobId: String): Flow<List<JobNoteEntity>>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsertNotes(notes: List<JobNoteEntity>)

    @Query("DELETE FROM job_notes WHERE jobId = :jobId AND pending = 0")
    suspend fun clearSyncedNotes(jobId: String)

    @Query("DELETE FROM job_notes WHERE id = :id")
    suspend fun deleteNote(id: String)

    /** Replaces server-sourced notes while leaving anything still queued alone. */
    @Transaction
    suspend fun replaceServerNotes(jobId: String, notes: List<JobNoteEntity>) {
        clearSyncedNotes(jobId)
        upsertNotes(notes)
    }

    // ---- status events

    @Query("SELECT * FROM job_status_events WHERE jobId = :jobId ORDER BY at DESC")
    fun observeStatusEvents(jobId: String): Flow<List<JobStatusEventEntity>>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsertStatusEvents(events: List<JobStatusEventEntity>)

    @Query("DELETE FROM job_status_events WHERE jobId = :jobId")
    suspend fun clearStatusEvents(jobId: String)

    @Transaction
    suspend fun replaceStatusEvents(jobId: String, events: List<JobStatusEventEntity>) {
        clearStatusEvents(jobId)
        upsertStatusEvents(events)
    }

    // ---- estimates

    @Query("SELECT * FROM job_estimates WHERE jobId = :jobId ORDER BY createdAt DESC")
    fun observeEstimates(jobId: String): Flow<List<JobEstimateEntity>>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsertEstimates(estimates: List<JobEstimateEntity>)

    // ---- parts

    @Query("SELECT * FROM job_parts WHERE jobId = :jobId")
    fun observeParts(jobId: String): Flow<List<JobPartEntity>>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsertParts(parts: List<JobPartEntity>)

    @Query("DELETE FROM job_parts WHERE jobId = :jobId AND pending = 0")
    suspend fun clearSyncedParts(jobId: String)

    @Query("DELETE FROM job_parts WHERE id = :id")
    suspend fun deletePart(id: String)

    @Transaction
    suspend fun replaceServerParts(jobId: String, parts: List<JobPartEntity>) {
        clearSyncedParts(jobId)
        upsertParts(parts)
    }

    // ---- photos

    @Query("SELECT * FROM job_photos WHERE jobId = :jobId ORDER BY createdAt DESC")
    fun observePhotos(jobId: String): Flow<List<JobPhotoEntity>>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsertPhotos(photos: List<JobPhotoEntity>)

    @Query("SELECT * FROM job_photos WHERE uploaded = 0 AND uploadFailed = 0")
    suspend fun pendingPhotos(): List<JobPhotoEntity>

    @Query("SELECT * FROM job_photos WHERE id = :id")
    suspend fun photo(id: String): JobPhotoEntity?

    @Query("DELETE FROM job_photos WHERE id = :id")
    suspend fun deletePhoto(id: String)

    @Query("DELETE FROM job_photos WHERE jobId = :jobId AND uploaded = 1")
    suspend fun clearUploadedPhotos(jobId: String)

    @Transaction
    suspend fun replaceServerPhotos(jobId: String, photos: List<JobPhotoEntity>) {
        clearUploadedPhotos(jobId)
        upsertPhotos(photos)
    }

    // ---- technicians

    @Query("SELECT * FROM job_technicians ORDER BY displayName")
    fun observeTechnicians(): Flow<List<TechnicianEntity>>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsertTechnicians(items: List<TechnicianEntity>)

    @Query("SELECT displayName FROM job_technicians WHERE id = :id")
    suspend fun technicianName(id: String): String?

    // ---- outbox

    @Query("SELECT * FROM job_outbox ORDER BY createdAt ASC")
    suspend fun outbox(): List<JobOutboxEntity>

    @Query("SELECT COUNT(*) FROM job_outbox")
    fun observeOutboxCount(): Flow<Int>

    @Query("SELECT COUNT(*) FROM job_outbox WHERE jobId = :jobId")
    fun observeOutboxCountForJob(jobId: String): Flow<Int>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun enqueue(action: JobOutboxEntity)

    @Query("DELETE FROM job_outbox WHERE id = :id")
    suspend fun dequeue(id: String)

    @Query("UPDATE job_outbox SET attempts = attempts + 1, lastError = :error WHERE id = :id")
    suspend fun markAttempt(id: String, error: String?)

    @Query("SELECT * FROM job_outbox WHERE attempts >= :threshold")
    fun observeStuck(threshold: Int): Flow<List<JobOutboxEntity>>
}
