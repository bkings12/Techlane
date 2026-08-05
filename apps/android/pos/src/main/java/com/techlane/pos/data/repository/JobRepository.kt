package com.techlane.pos.data.repository

import android.util.Base64
import com.techlane.pos.data.local.JobDao
import com.techlane.pos.data.local.JobEntity
import com.techlane.pos.data.local.JobEstimateEntity
import com.techlane.pos.data.local.JobNoteEntity
import com.techlane.pos.data.local.JobOutboxEntity
import com.techlane.pos.data.local.JobPartEntity
import com.techlane.pos.data.local.JobPhotoEntity
import com.techlane.pos.data.local.JobStatusEventEntity
import com.techlane.pos.data.local.TechnicianEntity
import com.techlane.pos.data.remote.ApiException
import com.techlane.pos.data.remote.OfflineException
import com.techlane.pos.data.remote.TechLaneApi
import com.techlane.pos.data.remote.dto.AddAttachmentRequest
import com.techlane.pos.data.remote.dto.AddLineItemRequest
import com.techlane.pos.data.remote.dto.AddNoteRequest
import com.techlane.pos.data.remote.dto.AddSaleLineRequest
import com.techlane.pos.data.remote.dto.AssignRequest
import com.techlane.pos.data.remote.dto.AuthorizeWorkRequest
import com.techlane.pos.data.remote.dto.ChangeStatusRequest
import com.techlane.pos.data.remote.dto.CreateEstimateRequest
import com.techlane.pos.data.remote.dto.CustomerDto
import com.techlane.pos.data.remote.dto.IntakeRequest
import com.techlane.pos.data.remote.dto.RepairJobDto
import com.techlane.pos.data.remote.dto.SendSmsRequest
import com.techlane.pos.data.remote.toAppException
import com.techlane.pos.domain.model.ApprovalMethod
import com.techlane.pos.domain.model.JobCustomer
import com.techlane.pos.domain.model.JobDetail
import com.techlane.pos.domain.model.JobDevice
import com.techlane.pos.domain.model.JobEstimate
import com.techlane.pos.domain.model.JobNote
import com.techlane.pos.domain.model.JobPart
import com.techlane.pos.domain.model.JobPhoto
import com.techlane.pos.domain.model.JobStatus
import com.techlane.pos.domain.model.JobSummary
import com.techlane.pos.domain.model.PhotoKind
import com.techlane.pos.domain.model.PhotoMetadata
import com.techlane.pos.domain.model.TimelineEvent
import com.techlane.pos.domain.model.TimelineKind
import com.techlane.pos.domain.model.WorkApproval
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.map
import kotlinx.serialization.json.Json
import java.io.File
import java.time.Instant
import java.time.format.DateTimeParseException
import java.util.UUID
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Jobs, offline-first.
 *
 * Room is the single source of truth the UI reads; the network only ever *fills*
 * it. Every mutation is applied locally and queued, so a technician working in a
 * back room with no signal sees their own change immediately and never has to
 * remember to redo it. [syncOutbox] is what eventually reconciles that with the
 * server, and it is the only place that decides an action is beyond saving.
 */
@Singleton
class JobRepository @Inject constructor(
    private val api: TechLaneApi,
    private val dao: JobDao,
) {

    private val json = Json { ignoreUnknownKeys = true; encodeDefaults = true }

    // ------------------------------------------------------------------ reads

    fun observeJobs(): Flow<List<JobSummary>> =
        combine(dao.observeAll(), dao.observeOutboxCount()) { jobs, _ -> jobs }
            .map { jobs -> jobs.map { it.toSummary() } }

    fun observeJob(id: String): Flow<JobDetail?> = combine(
        dao.observeJob(id),
        dao.observeNotes(id),
        dao.observeStatusEvents(id),
        dao.observeParts(id),
        dao.observePhotos(id),
    ) { job, notes, events, parts, photos ->
        Triple(job, notes to events, parts to photos)
    }.combine(dao.observeEstimates(id)) { (job, notesEvents, partsPhotos), estimates ->
        job ?: return@combine null
        val (notes, events) = notesEvents
        val (parts, photos) = partsPhotos
        job.toDetail(
            notes = notes.map { it.toDomain() },
            statusEvents = events.map { it.toTimelineEvent() },
            estimates = estimates.map { it.toDomain() },
            parts = parts.map { it.toDomain() },
            photos = photos.map { it.toDomain() },
        )
    }.combine(dao.observeOutboxCountForJob(id)) { detail, pending ->
        detail?.copy(pendingSyncCount = pending)
    }

    fun observeTechnicians(): Flow<List<TechnicianEntity>> = dao.observeTechnicians()

    fun observePendingSyncCount(): Flow<Int> = dao.observeOutboxCount()

    fun observeStuckActions(): Flow<List<JobOutboxEntity>> = dao.observeStuck(OUTBOX_STUCK_THRESHOLD)

    // --------------------------------------------------------------- refresh

    /** Pulls the board. Failure is non-fatal — the cached board stays usable. */
    suspend fun refreshJobs(technicianId: String? = null, branchId: String? = null): Result<Int> = runCatching {
        val items = api.repairs(technicianId = technicianId, branchId = branchId).items
        dao.upsertJobs(items.map { it.toEntity() })
        items.size
    }.recoverCatching { throw it.toAppException() }

    suspend fun refreshJob(id: String): Result<Unit> = runCatching {
        val dto = api.repair(id)
        dao.upsertJobs(listOf(dto.toEntity()))
        dao.replaceStatusEvents(
            id,
            dto.timeline.mapIndexed { index, event ->
                JobStatusEventEntity(
                    id = "$id-$index-${event.at.orEmpty()}",
                    jobId = id,
                    status = event.status,
                    note = event.note,
                    at = event.at.toEpochMillis() ?: 0L,
                )
            },
        )
        // Each of these is independently useful; one failing must not blank the
        // others out of the cache the technician is about to rely on.
        runCatching {
            val notes = api.repairNotes(id).items
            dao.replaceServerNotes(
                id,
                notes.map {
                    JobNoteEntity(
                        id = it.id,
                        jobId = id,
                        note = it.note,
                        authorName = it.authorName,
                        createdAt = it.createdAt.toEpochMillis() ?: 0L,
                        pending = false,
                    )
                },
            )
        }
        runCatching {
            val estimates = api.repairEstimates(id).items
            dao.upsertEstimates(
                estimates.map {
                    JobEstimateEntity(
                        id = it.id,
                        jobId = id,
                        total = it.totalAmount,
                        status = it.status,
                        notes = it.notes,
                        createdAt = it.createdAt.toEpochMillis() ?: 0L,
                    )
                },
            )
        }
        runCatching {
            // Legacy accessory sale-lines and the newer typed work-order line
            // items share one local table (job_parts) so the UI reads a single
            // list; replaceServerParts wipes and reinserts, so both sources
            // must be merged into one call rather than two.
            val legacyLines = runCatching { api.repairSaleLines(id).items }.getOrDefault(emptyList())
            val lineItems = runCatching { api.jobLineItems(id).items }.getOrDefault(emptyList())
            val rows = legacyLines.map {
                JobPartEntity(
                    id = it.id ?: UUID.randomUUID().toString(),
                    jobId = id,
                    lineId = it.id,
                    variantId = it.variantId,
                    name = it.description,
                    sku = null,
                    quantity = it.quantity,
                    unitPrice = it.unitPrice,
                    pending = false,
                    lineType = "product",
                )
            } + lineItems.map {
                JobPartEntity(
                    id = it.id ?: UUID.randomUUID().toString(),
                    jobId = id,
                    lineId = it.id,
                    variantId = it.variantId,
                    name = it.description,
                    sku = null,
                    quantity = it.quantity.toInt(),
                    unitPrice = it.unitPrice,
                    pending = false,
                    lineType = it.lineType,
                    unitCost = it.unitCost,
                    partStatus = it.partStatus,
                    partSource = it.partSource,
                    supplierName = it.supplierName,
                )
            }
            dao.replaceServerParts(id, rows)
        }
        runCatching {
            val attachments = api.repairAttachments(id).items
            dao.replaceServerPhotos(
                id,
                attachments.map {
                    val meta = PhotoMetadata.decode(it.fileName)
                    JobPhotoEntity(
                        id = it.id,
                        jobId = id,
                        remoteId = it.id,
                        kind = meta.kind.wire,
                        caption = meta.caption,
                        localPath = null,
                        createdAt = it.createdAt.toEpochMillis() ?: 0L,
                        uploaded = true,
                        uploadFailed = false,
                    )
                },
            )
        }
        Unit
    }.recoverCatching { throw it.toAppException() }

    suspend fun refreshTechnicians(): Result<Unit> = runCatching {
        val items = api.technicians().items
        dao.upsertTechnicians(items.map { TechnicianEntity(it.id, it.displayName, it.email) })
    }.recoverCatching { throw it.toAppException() }

    /** Server-side search — reaches jobs this handset has never cached. */
    suspend fun searchJobs(query: String): Result<List<JobSummary>> = runCatching {
        val items = api.repairs(query = query).items
        dao.upsertJobs(items.map { it.toEntity() })
        items.map { it.toEntity().toSummary() }
    }.recoverCatching { throw it.toAppException() }

    /**
     * Books a walk-in. Deliberately online-only rather than queued through the
     * outbox: intake mints a job number, a pickup code and a customer record
     * server-side, and a phone that invented those offline would hand the
     * customer a slip whose code the shop cannot look up. The job lands in the
     * cache on success so Job Details opens instantly on the returned id.
     */
    suspend fun createIntake(request: IntakeRequest, idempotencyKey: String): Result<String> = runCatching {
        val repair = api.intake(request, correlationId = idempotencyKey).repair
            ?: error("The job was created but the server did not return it. Pull to refresh the board.")
        dao.upsertJobs(listOf(repair.toEntity()))
        repair.id
    }.recoverCatching { throw it.toAppException() }

    /**
     * Finds an existing customer by phone (or name) so a returning walk-in is
     * one tap rather than a re-typed record. Failures return an empty list:
     * lookup is an accelerator, and a flaky connection must not block booking
     * the job by hand.
     */
    suspend fun searchCustomers(query: String): List<CustomerDto> =
        runCatching { api.searchCustomers(query).items }.getOrDefault(emptyList())

    /** Resolves the intake-slip QR to a job id. */
    suspend fun findByPickupCode(code: String): Result<String> = runCatching {
        val dto = api.repairByPickupCode(code)
        dao.upsertJobs(listOf(dto.toEntity()))
        dto.id
    }.recoverCatching { throw it.toAppException() }

    // ------------------------------------------------------------- mutations

    /**
     * Applies the change to the local cache first, then queues it. The optimistic
     * write is what lets the UI stay responsive on a dead connection; the queue
     * is what makes it true later.
     */
    suspend fun changeStatus(
        jobId: String,
        status: JobStatus,
        note: String?,
        closureReason: String?,
    ) {
        dao.setStatus(jobId, status.wire)
        dao.upsertStatusEvents(
            listOf(
                JobStatusEventEntity(
                    id = "local-${UUID.randomUUID()}",
                    jobId = jobId,
                    status = status.wire,
                    note = note,
                    at = System.currentTimeMillis(),
                ),
            ),
        )
        enqueue(jobId, JobAction.ChangeStatus(status.wire, note, closureReason))
    }

    suspend fun assign(jobId: String, technicianId: String) {
        dao.setTechnician(jobId, technicianId)
        enqueue(jobId, JobAction.Assign(technicianId))
    }

    /**
     * Online-only device release. Refuses to invent a collected status offline —
     * handover is the money + custody gate the server owns.
     */
    suspend fun recordHandover(
        jobId: String,
        collectedByName: String,
        relationship: String,
        note: String?,
        pickupCode: String?,
        otpCode: String?,
    ): Result<Unit> = runCatching {
        api.recordHandover(
            jobId,
            com.techlane.pos.data.remote.dto.HandoverRequest(
                collectedByName = collectedByName,
                relationship = relationship,
                note = note,
                pickupCode = pickupCode,
                otpCode = otpCode,
            ),
        )
        refreshJob(jobId)
        Unit
    }.recoverCatching { throw it.toAppException() }

    suspend fun sendHandoverCode(jobId: String): Result<Unit> = runCatching {
        api.sendHandoverCode(jobId)
        Unit
    }.recoverCatching { throw it.toAppException() }

    suspend fun listRepairPayments(jobId: String): Result<List<com.techlane.pos.data.remote.dto.PaymentDto>> =
        runCatching {
            api.payments(payableType = "repair", payableId = jobId).items
        }.recoverCatching { throw it.toAppException() }

    suspend fun listUnmatchedC2b(): Result<List<com.techlane.pos.data.remote.dto.C2bTransactionDto>> =
        runCatching {
            api.c2bTransactions(status = "unmatched").items
        }.recoverCatching { throw it.toAppException() }

    suspend fun matchC2bToPayment(c2bId: String, paymentId: String): Result<com.techlane.pos.data.remote.dto.PaymentDto> =
        runCatching {
            api.matchC2b(c2bId, com.techlane.pos.data.remote.dto.MatchC2bRequest(paymentId))
        }.recoverCatching { throw it.toAppException() }

    suspend fun addNote(jobId: String, text: String, authorName: String?) {
        val localId = "local-${UUID.randomUUID()}"
        dao.upsertNotes(
            listOf(
                JobNoteEntity(
                    id = localId,
                    jobId = jobId,
                    note = text,
                    authorName = authorName,
                    createdAt = System.currentTimeMillis(),
                    pending = true,
                ),
            ),
        )
        enqueue(jobId, JobAction.AddNote(localId, text))
    }

    suspend fun sendEstimate(jobId: String, total: Double, notes: String?) {
        enqueue(jobId, JobAction.SendEstimate(total, notes))
    }

    /**
     * Records a manager/counter go-ahead. Locally we mark the job authorised so
     * the bench action unlocks immediately, but the server still has the final
     * word — a rejected authorisation surfaces as a stuck outbox action rather
     * than silently leaving the job unblocked.
     */
    suspend fun authorizeWork(jobId: String, note: String, amount: Double?) {
        dao.setAuthorization(jobId, System.currentTimeMillis(), ApprovalMethod.ManagerOverride.wire, amount)
        enqueue(jobId, JobAction.AuthorizeWork(note, amount))
    }

    suspend fun addPart(jobId: String, variantId: String, locationId: String, name: String, sku: String?, unitPrice: Double, quantity: Int) {
        val localId = "local-${UUID.randomUUID()}"
        dao.upsertParts(
            listOf(
                JobPartEntity(
                    id = localId,
                    jobId = jobId,
                    lineId = null,
                    variantId = variantId,
                    name = name,
                    sku = sku,
                    quantity = quantity,
                    unitPrice = unitPrice,
                    pending = true,
                ),
            ),
        )
        enqueue(jobId, JobAction.AddPart(localId, variantId, locationId, quantity))
    }

    suspend fun removePart(jobId: String, part: JobPart) {
        dao.deletePart(part.rowId)
        // A part that never reached the server needs no server-side undo; dropping
        // the local row is the whole operation. Cancelling the queued add would be
        // better still, but the outbox is append-only by design.
        val lineId = part.lineId ?: return
        enqueue(jobId, JobAction.RemovePart(lineId))
    }

    // ------------------------------------------------------- work-order lines
    //
    // Labour, sourced parts, and products are deliberately online-only, the
    // same call as createIntake makes: the server resolves catalog price/cost,
    // deducts stock, and books the line atomically, none of which a phone can
    // safely invent offline. Each call refreshes the job on success so the new
    // line shows up immediately rather than waiting for the next board sync.

    suspend fun addLabourLine(jobId: String, description: String, unitPrice: Double, quantity: Double = 1.0): Result<Unit> =
        runCatching {
            api.addLineItem(jobId, AddLineItemRequest(type = "labour", description = description, unitPrice = unitPrice, quantity = quantity))
            refreshJob(jobId).getOrThrow()
        }.recoverCatching { throw it.toAppException() }

    suspend fun addInventoryPartLine(jobId: String, variantId: String, locationId: String, quantity: Double = 1.0): Result<Unit> =
        runCatching {
            api.addLineItem(
                jobId,
                AddLineItemRequest(type = "part", variantId = variantId, locationId = locationId, quantity = quantity),
            )
            refreshJob(jobId).getOrThrow()
        }.recoverCatching { throw it.toAppException() }

    suspend fun addSourcedPartLine(
        jobId: String,
        description: String,
        unitCost: Double,
        unitPrice: Double,
        quantity: Double = 1.0,
        supplierName: String? = null,
        supplierRef: String? = null,
        expectedArrival: String? = null,
    ): Result<Unit> = runCatching {
        api.addLineItem(
            jobId,
            AddLineItemRequest(
                type = "part", description = description, unitCost = unitCost, unitPrice = unitPrice,
                quantity = quantity, supplierName = supplierName, supplierRef = supplierRef, expectedArrival = expectedArrival,
            ),
        )
        refreshJob(jobId).getOrThrow()
    }.recoverCatching { throw it.toAppException() }

    suspend fun addProductLine(jobId: String, variantId: String, locationId: String, quantity: Double = 1.0): Result<Unit> =
        runCatching {
            api.addLineItem(
                jobId,
                AddLineItemRequest(type = "product", variantId = variantId, locationId = locationId, quantity = quantity),
            )
            refreshJob(jobId).getOrThrow()
        }.recoverCatching { throw it.toAppException() }

    /** Removes any work-order line item (labour, part, or product) — online-only, same reasoning as above. */
    suspend fun removeLineItem(jobId: String, part: JobPart): Result<Unit> = runCatching {
        val lineId = part.lineId ?: run {
            dao.deletePart(part.rowId)
            return@runCatching
        }
        api.removeLineItem(jobId, lineId)
        refreshJob(jobId).getOrThrow()
    }.recoverCatching { throw it.toAppException() }

    suspend fun addPhoto(jobId: String, file: File, kind: PhotoKind, caption: String?) {
        val id = "local-${UUID.randomUUID()}"
        dao.upsertPhotos(
            listOf(
                JobPhotoEntity(
                    id = id,
                    jobId = jobId,
                    remoteId = null,
                    kind = kind.wire,
                    caption = caption,
                    localPath = file.absolutePath,
                    createdAt = System.currentTimeMillis(),
                    uploaded = false,
                    uploadFailed = false,
                ),
            ),
        )
        enqueue(jobId, JobAction.UploadPhoto(id))
    }

    /** Only an un-uploaded photo can be discarded; the server owns the rest. */
    suspend fun deleteLocalPhoto(photoId: String) {
        val photo = dao.photo(photoId) ?: return
        if (photo.uploaded) return
        photo.localPath?.let { runCatching { File(it).delete() } }
        dao.deletePhoto(photoId)
    }

    suspend fun sendCustomerUpdate(jobId: String, phone: String, body: String, customerId: String?) {
        enqueue(jobId, JobAction.SendCustomerUpdate(phone, body, customerId))
    }

    private suspend fun enqueue(jobId: String, action: JobAction) {
        dao.enqueue(
            JobOutboxEntity(
                id = UUID.randomUUID().toString(),
                jobId = jobId,
                type = action::class.simpleName.orEmpty(),
                payload = json.encodeToString(JobAction.serializer(), action),
                createdAt = System.currentTimeMillis(),
            ),
        )
    }

    // ------------------------------------------------------------------ sync

    /**
     * Drains the queue oldest-first. Returns the number of actions still waiting.
     *
     * Stopping on the first network failure is deliberate: order matters between
     * actions on the same job, so skipping ahead could apply a status change the
     * server would reject without the note that justified it.
     */
    suspend fun syncOutbox(): Int {
        val pending = dao.outbox()
        for (entry in pending) {
            val action = runCatching {
                json.decodeFromString(JobAction.serializer(), entry.payload)
            }.getOrNull()
            if (action == null) {
                // Unreadable payload can never succeed; drop it rather than jam the queue.
                dao.dequeue(entry.id)
                continue
            }
            try {
                apply(entry.jobId, action)
                dao.dequeue(entry.id)
            } catch (offline: OfflineException) {
                dao.markAttempt(entry.id, offline.message)
                break
            } catch (e: Exception) {
                val mapped = e.toAppException()
                dao.markAttempt(entry.id, mapped.message)
                // A 4xx will never succeed on retry; leave it queued and visible
                // so the technician sees why, but keep draining the rest.
                if (mapped is ApiException && mapped.statusCode in 400..499) continue else break
            }
        }
        return dao.outbox().size
    }

    private suspend fun apply(jobId: String, action: JobAction) {
        when (action) {
            is JobAction.ChangeStatus -> api.changeRepairStatus(
                jobId,
                ChangeStatusRequest(
                    status = action.status,
                    note = action.note,
                    closureReason = action.closureReason,
                    varianceReason = action.varianceReason,
                ),
            ).also { dao.upsertJobs(listOf(it.toEntity())) }

            is JobAction.Assign -> api.assignRepair(jobId, AssignRequest(action.technicianId))
                .also { dao.upsertJobs(listOf(it.toEntity())) }

            is JobAction.AddNote -> {
                val saved = api.addRepairNote(jobId, AddNoteRequest(action.text))
                // Swap the optimistic row for the server's, keeping the timeline honest.
                dao.deleteNote(action.localId)
                dao.upsertNotes(
                    listOf(
                        JobNoteEntity(
                            id = saved.id,
                            jobId = jobId,
                            note = saved.note,
                            authorName = saved.authorName,
                            createdAt = saved.createdAt.toEpochMillis() ?: System.currentTimeMillis(),
                            pending = false,
                        ),
                    ),
                )
            }

            is JobAction.SendEstimate -> api.createRepairEstimate(
                jobId,
                CreateEstimateRequest(action.total, action.notes),
            )

            is JobAction.AuthorizeWork -> api.authorizeRepairWork(
                jobId,
                AuthorizeWorkRequest(action.note, action.amount),
            ).also { dao.upsertJobs(listOf(it.toEntity())) }

            is JobAction.AddPart -> {
                val line = api.addRepairSaleLine(
                    jobId,
                    AddSaleLineRequest(action.variantId, action.locationId, action.quantity),
                )
                dao.deletePart(action.localId)
                dao.upsertParts(
                    listOf(
                        JobPartEntity(
                            id = line.id ?: UUID.randomUUID().toString(),
                            jobId = jobId,
                            lineId = line.id,
                            variantId = line.variantId,
                            name = line.description,
                            sku = null,
                            quantity = line.quantity,
                            unitPrice = line.unitPrice,
                            pending = false,
                        ),
                    ),
                )
            }

            is JobAction.RemovePart -> api.removeRepairSaleLine(jobId, action.lineId)

            is JobAction.UploadPhoto -> {
                val photo = dao.photo(action.photoId) ?: return
                val file = photo.localPath?.let(::File)
                if (file == null || !file.exists()) {
                    dao.deletePhoto(photo.id)
                    return
                }
                val saved = api.addRepairAttachment(
                    jobId,
                    AddAttachmentRequest(
                        fileName = PhotoMetadata.encode(
                            PhotoKind.fromWire(photo.kind),
                            photo.caption,
                            photo.createdAt,
                        ),
                        contentType = "image/jpeg",
                        dataBase64 = Base64.encodeToString(file.readBytes(), Base64.NO_WRAP),
                    ),
                )
                dao.deletePhoto(photo.id)
                dao.upsertPhotos(
                    listOf(photo.copy(id = saved.id, remoteId = saved.id, uploaded = true, localPath = null)),
                )
                runCatching { file.delete() }
            }

            is JobAction.SendCustomerUpdate -> api.sendSms(
                SendSmsRequest(
                    phone = action.phone,
                    body = action.body,
                    repairJobId = jobId,
                    customerId = action.customerId,
                ),
            )
        }
    }

    /** Lets the technician clear an action the server will never accept. */
    suspend fun discardQueuedAction(id: String) = dao.dequeue(id)

    // ------------------------------------------------------------- mapping

    private fun RepairJobDto.toEntity() = JobEntity(
        id = id,
        jobCode = jobCode.ifBlank { "#$jobNumber" },
        customerName = customer?.fullName ?: customerName,
        customerPhone = customer?.phone,
        customerId = customer?.id ?: customerId,
        deviceKind = device?.kind ?: "",
        deviceBrand = device?.brand,
        deviceModel = device?.model,
        deviceImei = device?.imei,
        deviceSerial = device?.serialNumber,
        status = status,
        technicianId = technicianId,
        problemSummary = problemSummary,
        createdAt = createdAt.toEpochMillis() ?: System.currentTimeMillis(),
        promisedBy = promisedBy.toEpochMillis(),
        customerWaiting = customerWaiting,
        authorizedAt = authorization?.authorizedAt.toEpochMillis(),
        authorizationSource = authorization?.source?.takeIf { it.isNotBlank() },
        authorizedAmount = authorization?.authorizedAmount ?: authorizedAmount,
        pendingEstimateTotal = pendingEstimateTotal,
        amountDue = amountDue,
        balanceDue = balanceDue,
        laborAmount = laborAmount,
        syncedAt = System.currentTimeMillis(),
    )

    private fun JobEntity.toSummary() = JobSummary(
        id = id,
        jobCode = jobCode,
        customerName = customerName,
        customerPhone = customerPhone,
        deviceLabel = listOfNotNull(deviceBrand, deviceModel).joinToString(" ")
            .ifBlank { deviceKind.replaceFirstChar(Char::uppercase) },
        imei = deviceImei,
        serialNumber = deviceSerial,
        status = JobStatus.fromWire(status),
        technicianId = technicianId,
        technicianName = null,
        promisedBy = promisedBy,
        createdAt = createdAt,
        customerWaiting = customerWaiting,
        awaitingApproval = authorizedAt == null && JobStatus.fromWire(status).isOpen,
        partsPending = JobStatus.fromWire(status) == JobStatus.WaitingParts,
        amountDue = amountDue,
    )

    private fun JobEntity.toDetail(
        notes: List<JobNote>,
        statusEvents: List<TimelineEvent>,
        estimates: List<JobEstimate>,
        parts: List<JobPart>,
        photos: List<JobPhoto>,
    ) = JobDetail(
        id = id,
        jobCode = jobCode,
        status = JobStatus.fromWire(status),
        customer = JobCustomer(customerId, customerName, customerPhone),
        device = JobDevice(null, deviceKind, deviceBrand, deviceModel, deviceImei, deviceSerial),
        problemSummary = problemSummary,
        technicianId = technicianId,
        technicianName = null,
        createdAt = createdAt,
        promisedBy = promisedBy,
        customerWaiting = customerWaiting,
        approval = WorkApproval(
            approvedAt = authorizedAt,
            approvedByName = null,
            method = ApprovalMethod.fromWire(authorizationSource),
            amount = authorizedAmount,
        ),
        amountDue = amountDue,
        balanceDue = balanceDue,
        laborAmount = laborAmount,
        notes = notes,
        estimates = estimates,
        parts = parts,
        photos = photos,
        statusEvents = statusEvents,
        // An hour-old board is fine; a day-old one should say so.
        stale = System.currentTimeMillis() - syncedAt > STALE_AFTER_MS,
    )

    private fun JobNoteEntity.toDomain() = JobNote(id, note, authorName, createdAt, pending)

    private fun JobEstimateEntity.toDomain() = JobEstimate(id, total, status, notes, createdAt)

    private fun JobPartEntity.toDomain() =
        JobPart(
            rowId = id, lineId = lineId, variantId = variantId, name = name, sku = sku,
            quantity = quantity, unitPrice = unitPrice, availableQty = null, pendingSync = pending,
            lineType = lineType, unitCost = unitCost, partStatus = partStatus, partSource = partSource,
            supplierName = supplierName,
        )

    private fun JobPhotoEntity.toDomain() = JobPhoto(
        id = id,
        remoteId = remoteId,
        kind = PhotoKind.fromWire(kind),
        caption = caption,
        localPath = localPath,
        createdAt = createdAt,
        uploaded = uploaded,
        uploadFailed = uploadFailed,
    )

    private fun JobStatusEventEntity.toTimelineEvent(): TimelineEvent {
        val jobStatus = JobStatus.fromWire(status)
        return TimelineEvent(
            id = id,
            at = at,
            kind = when (jobStatus) {
                JobStatus.Intake -> TimelineKind.Received
                JobStatus.Collected -> TimelineKind.Collected
                else -> TimelineKind.StatusChange
            },
            title = when (jobStatus) {
                JobStatus.Intake -> "Device received"
                JobStatus.Collected -> "Device collected"
                else -> "Status → ${jobStatus.label}"
            },
            detail = note,
            actorName = null,
        )
    }

    private companion object {
        const val STALE_AFTER_MS = 6 * 60 * 60 * 1000L
    }
}

/** RFC3339 from the Go API; a bad value must never take the screen down. */
internal fun String?.toEpochMillis(): Long? {
    if (this.isNullOrBlank()) return null
    return try {
        Instant.parse(this).toEpochMilli()
    } catch (_: DateTimeParseException) {
        null
    } catch (_: Exception) {
        null
    }
}
