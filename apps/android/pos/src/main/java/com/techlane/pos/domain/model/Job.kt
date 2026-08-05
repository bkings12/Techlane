package com.techlane.pos.domain.model

/**
 * The repair statuses the backend actually stores (internal/repair/status.go).
 *
 * The technician-facing wording differs from the wire value on purpose — "on
 * bench" is what staff say, `in_progress` is what the API and the web console
 * have always persisted. Inventing new statuses here would fork the two clients,
 * so the vocabulary is a label, never a second source of truth.
 */
enum class JobStatus(val wire: String, val label: String, val shortLabel: String) {
    Intake("intake", "Received", "Received"),
    Diagnosed("diagnosed", "Diagnosed", "Diagnosed"),
    WaitingParts("waiting_parts", "Waiting parts", "Parts"),
    InProgress("in_progress", "On bench", "Bench"),
    ReadyForPickup("ready_for_pickup", "Ready", "Ready"),
    Completed("completed", "Complete", "Complete"),
    Collected("collected", "Collected", "Collected"),
    Cancelled("cancelled", "Cancelled", "Cancelled"),
    Unrepairable("unrepairable", "Unrepairable", "Unrepairable"),
    ;

    val isClosure: Boolean get() = this == Cancelled || this == Unrepairable

    /** Still live work: not handed back, not written off. */
    val isOpen: Boolean get() = this != Collected && !isClosure

    /** Statuses whose consequences are hard to walk back. */
    val needsConfirmation: Boolean
        get() = this == ReadyForPickup || this == Completed || this == Collected || isClosure

    /** Mirrors allowedTransitions in internal/repair/status.go. */
    val allowedNext: List<JobStatus>
        get() = when (this) {
            Intake -> listOf(Diagnosed, WaitingParts, InProgress, Cancelled, Unrepairable)
            Diagnosed -> listOf(WaitingParts, InProgress, Cancelled, Unrepairable)
            WaitingParts -> listOf(InProgress, Cancelled, Unrepairable)
            InProgress -> listOf(ReadyForPickup, WaitingParts, Cancelled, Unrepairable)
            ReadyForPickup -> listOf(Completed, InProgress, Cancelled, Unrepairable)
            Completed -> listOf(Collected)
            Cancelled, Unrepairable -> listOf(Collected)
            Collected -> emptyList()
        }

    companion object {
        fun fromWire(value: String?): JobStatus =
            entries.firstOrNull { it.wire == value?.trim()?.lowercase() } ?: Intake
    }
}

/** Structured closure codes the server accepts; free text goes in the note. */
enum class ClosureReason(val wire: String, val label: String, val status: JobStatus) {
    CustomerDeclined("customer_declined_quote", "Customer declined the quote", JobStatus.Cancelled),
    CustomerWithdrew("customer_withdrew", "Customer withdrew the job", JobStatus.Cancelled),
    NoResponse("no_response", "No response from customer", JobStatus.Cancelled),
    Duplicate("duplicate_job", "Duplicate job", JobStatus.Cancelled),
    CancelledOther("other", "Other", JobStatus.Cancelled),
    BeyondRepair("beyond_economical_repair", "Beyond economical repair", JobStatus.Unrepairable),
    PartsUnavailable("parts_unavailable", "Parts unavailable", JobStatus.Unrepairable),
    LiquidDamage("severe_liquid_damage", "Severe liquid damage", JobStatus.Unrepairable),
    FurtherDamage("further_damage_found", "Further damage found", JobStatus.Unrepairable),
    UnrepairableOther("other", "Other", JobStatus.Unrepairable),
    ;

    companion object {
        fun forStatus(status: JobStatus): List<ClosureReason> = entries.filter { it.status == status }
    }
}

/**
 * The board filters. Two of these are *derived* rather than stored:
 *
 *  - [AwaitingApproval] has no backing status. A job is awaiting approval when
 *    it is open, has no work authorization, and nobody has approved a price —
 *    which is exactly the condition the server blocks bench work on.
 *  - [Mine] is a technician filter, not a state.
 *
 * Modelling them as statuses would have meant inventing values the web console
 * and the database do not know about.
 */
enum class JobFilter(val label: String) {
    Mine("My jobs"),
    All("All"),
    AwaitingDiagnosis("Awaiting diagnosis"),
    AwaitingApproval("Awaiting approval"),
    WaitingParts("Waiting parts"),
    OnBench("On bench"),
    Ready("Ready"),
    Completed("Completed"),
    ;

    /** Server-side status filter, when this maps cleanly to one. */
    val statusFilter: JobStatus?
        get() = when (this) {
            AwaitingDiagnosis -> JobStatus.Intake
            WaitingParts -> JobStatus.WaitingParts
            OnBench -> JobStatus.InProgress
            Ready -> JobStatus.ReadyForPickup
            Completed -> JobStatus.Completed
            Mine, All, AwaitingApproval -> null
        }
}

enum class ApprovalMethod(val wire: String, val label: String) {
    CustomerEstimate("customer_estimate", "Approved estimate"),
    IntakeAgreed("intake_agreed", "Agreed at counter"),
    ManagerOverride("manager_override", "Manager go-ahead"),
    ReturnRework("return_rework", "Return / rework"),
    Legacy("legacy", "Recorded earlier"),
    ;

    companion object {
        fun fromWire(value: String?): ApprovalMethod? =
            entries.firstOrNull { it.wire == value?.trim()?.lowercase() }
    }
}

/** How a verbal go-ahead was obtained. Recorded in the authorization note. */
enum class VerbalApprovalChannel(val label: String) {
    AtCounter("Verbal at counter"),
    PhoneCall("Phone call"),
    WhatsApp("WhatsApp"),
    Sms("SMS / link"),
    Other("Other"),
}

/** A job as the board needs it — deliberately small so lists stay cheap. */
data class JobSummary(
    val id: String,
    val jobCode: String,
    val customerName: String?,
    val customerPhone: String?,
    val deviceLabel: String,
    val imei: String?,
    val serialNumber: String?,
    val status: JobStatus,
    val technicianId: String?,
    val technicianName: String?,
    val promisedBy: Long?,
    val createdAt: Long,
    val customerWaiting: Boolean,
    val awaitingApproval: Boolean,
    val partsPending: Boolean,
    val amountDue: Double,
    val pendingSync: Boolean = false,
) {
    /** Overdue against its promise, and still something we owe the customer. */
    val isOverdue: Boolean
        get() = promisedBy != null && status.isOpen && promisedBy < System.currentTimeMillis()

    /** Walk-in waiting at the counter outranks everything else on the board. */
    val isUrgent: Boolean get() = customerWaiting && status.isOpen
}

/**
 * The counts along the top of the board. Derived from the whole shop's jobs
 * rather than the filtered view, because the point of the strip is to show
 * what is waiting *outside* whatever queue is currently on screen.
 */
data class JobBoardSummary(
    val open: Int = 0,
    val onBench: Int = 0,
    val waitingParts: Int = 0,
    val ready: Int = 0,
    val overdue: Int = 0,
) {
    val isEmpty: Boolean get() = open == 0 && onBench == 0 && waitingParts == 0 && ready == 0 && overdue == 0

    companion object {
        fun from(jobs: List<JobSummary>): JobBoardSummary = JobBoardSummary(
            open = jobs.count { it.status.isOpen },
            onBench = jobs.count { it.status == JobStatus.InProgress },
            waitingParts = jobs.count { it.status == JobStatus.WaitingParts },
            ready = jobs.count { it.status == JobStatus.ReadyForPickup },
            overdue = jobs.count { it.isOverdue },
        )
    }
}

/** How the board is ordered. Default keeps the most recently touched work on top. */
enum class JobSort(val label: String) {
    RecentlyUpdated("Recently updated"),
    Newest("Newest"),
    Oldest("Oldest"),
    PromiseDate("Promise date"),
    ;

    /**
     * Overdue and waiting walk-ins always float regardless of sort — an
     * operator scanning the board needs those first no matter how they ordered it.
     */
    fun sort(jobs: List<JobSummary>): List<JobSummary> {
        val comparator: Comparator<JobSummary> = when (this) {
            // The board has no separate "updated at", so newest-first is the
            // closest honest proxy rather than inventing a timestamp.
            RecentlyUpdated, Newest -> compareByDescending { it.createdAt }
            Oldest -> compareBy { it.createdAt }
            // Jobs with no promise sink below those that have one.
            PromiseDate -> compareBy { it.promisedBy ?: Long.MAX_VALUE }
        }
        return jobs.sortedWith(
            compareByDescending<JobSummary> { it.isUrgent }
                .thenByDescending { it.isOverdue }
                .then(comparator),
        )
    }
}

data class JobCustomer(
    val id: String?,
    val name: String?,
    val phone: String?,
)

/** The device kinds `POST /repairs/intake` accepts — mirrors internal/repair/intake.go. */
enum class DeviceKind(val wire: String, val label: String) {
    Phone("phone", "Phone"),
    Laptop("laptop", "Laptop"),
    Tablet("tablet", "Tablet"),
    Other("other", "Other"),
    ;

    /** Phones and tablets are identified by IMEI; everything else by serial. */
    val identifierIsImei: Boolean get() = this == Phone || this == Tablet

    val identifierLabel: String get() = if (identifierIsImei) "IMEI" else "Serial number"
}

data class JobDevice(
    val id: String?,
    val kind: String,
    val brand: String?,
    val model: String?,
    val imei: String?,
    val serialNumber: String?,
) {
    val label: String
        get() = listOfNotNull(brand, model).joinToString(" ").ifBlank { kind.replaceFirstChar(Char::uppercase) }

    /** IMEI for phones, serial for laptops — show whichever the device carries. */
    val identifier: Pair<String, String>?
        get() = when {
            !imei.isNullOrBlank() -> "IMEI" to imei
            !serialNumber.isNullOrBlank() -> "Serial" to serialNumber
            else -> null
        }
}

data class WorkApproval(
    val approvedAt: Long?,
    val approvedByName: String?,
    val method: ApprovalMethod?,
    val amount: Double?,
) {
    val isApproved: Boolean get() = approvedAt != null
}

data class JobNote(
    val id: String,
    val text: String,
    val authorName: String?,
    val createdAt: Long,
    val pendingSync: Boolean = false,
)

data class JobEstimate(
    val id: String,
    val total: Double,
    val status: String,
    val notes: String?,
    val createdAt: Long,
) {
    val isPending: Boolean get() = status == "pending" || status == "sent"
    val isApproved: Boolean get() = status == "approved"
}

data class JobPart(
    /** Room row id — the only handle a not-yet-synced part has. */
    val rowId: String,
    val lineId: String?,
    val variantId: String?,
    val name: String,
    val sku: String?,
    val quantity: Int,
    val unitPrice: Double,
    val availableQty: Int?,
    val pendingSync: Boolean = false,
    /** "labour" | "part" | "product" — see internal/repair's line_type. */
    val lineType: String = "product",
    val unitCost: Double? = null,
    val partStatus: String? = null,
    val partSource: String? = null,
    val supplierName: String? = null,
) {
    val lineTotal: Double get() = unitPrice * quantity
    val isLabour: Boolean get() = lineType == "labour"
    val isProduct: Boolean get() = lineType == "product"
}

/** Where in the job a photo was taken. Drives grouping in the gallery. */
enum class PhotoKind(val wire: String, val label: String) {
    Intake("intake", "Intake"),
    Diagnosis("diagnosis", "Diagnosis"),
    Progress("progress", "Progress"),
    Completion("completion", "Completion"),
    ;

    companion object {
        fun fromWire(value: String?): PhotoKind =
            entries.firstOrNull { it.wire == value?.trim()?.lowercase() } ?: Progress
    }
}

data class JobPhoto(
    val id: String,
    val remoteId: String?,
    val kind: PhotoKind,
    val caption: String?,
    /** Local file for a photo not yet uploaded; null once it lives on the server. */
    val localPath: String?,
    val createdAt: Long,
    val uploaded: Boolean,
    val uploadFailed: Boolean = false,
) {
    /** Only an unsynced photo can be discarded outright — see JobRepository. */
    val canDelete: Boolean get() = !uploaded
}

/** One line on the repair timeline, whatever produced it. */
data class TimelineEvent(
    val id: String,
    val at: Long,
    val kind: TimelineKind,
    val title: String,
    val detail: String?,
    val actorName: String?,
)

enum class TimelineKind {
    Received,
    StatusChange,
    Assigned,
    Diagnosis,
    Note,
    Photo,
    Estimate,
    Approval,
    Part,
    CustomerNotified,
    Collected,
}

/** Everything the detail screen renders. Assembled offline-first from Room. */
data class JobDetail(
    val id: String,
    val jobCode: String,
    val status: JobStatus,
    val customer: JobCustomer,
    val device: JobDevice,
    val problemSummary: String,
    val technicianId: String?,
    val technicianName: String?,
    val createdAt: Long,
    val promisedBy: Long?,
    val customerWaiting: Boolean,
    val approval: WorkApproval,
    val amountDue: Double,
    val balanceDue: Double,
    val laborAmount: Double,
    val notes: List<JobNote>,
    val estimates: List<JobEstimate>,
    val parts: List<JobPart>,
    val photos: List<JobPhoto>,
    val statusEvents: List<TimelineEvent>,
    val pendingSyncCount: Int = 0,
    val stale: Boolean = false,
) {
    /** Settled so far — derived so Room does not need a second money column. */
    val paidTotal: Double get() = (amountDue - balanceDue).coerceAtLeast(0.0)

    /** Ready for physical handover: balance clear and status collectable. */
    val canCollect: Boolean
        get() = balanceDue <= 0.009 &&
            (status == JobStatus.ReadyForPickup || status == JobStatus.Completed)

    /** The most recent technician note doubles as the current diagnosis. */
    val latestDiagnosis: JobNote? get() = notes.maxByOrNull { it.createdAt }

    val pendingEstimate: JobEstimate? get() = estimates.firstOrNull { it.isPending }

    /**
     * The server refuses intake/diagnosed → in_progress without an authorization
     * (ErrWorkNotAuthorized). Mirroring the condition lets the UI explain it
     * before the technician hits a wall, without ever being the thing that decides.
     */
    val needsApprovalBeforeBench: Boolean
        get() = !approval.isApproved && (status == JobStatus.Intake || status == JobStatus.Diagnosed)

    /**
     * Composes the full timeline the technician sees. The API stores status
     * events, notes, photos and estimates in separate places — there is no
     * single feed endpoint — so the merge happens here rather than inventing one.
     */
    fun timeline(): List<TimelineEvent> {
        val events = buildList {
            addAll(statusEvents)
            notes.forEach {
                add(
                    TimelineEvent(
                        id = "note-${it.id}",
                        at = it.createdAt,
                        kind = TimelineKind.Diagnosis,
                        title = "Diagnosis updated",
                        detail = it.text,
                        actorName = it.authorName,
                    ),
                )
            }
            photos.groupBy { it.kind to dayBucket(it.createdAt) }.forEach { (key, group) ->
                val newest = group.maxByOrNull { it.createdAt } ?: return@forEach
                add(
                    TimelineEvent(
                        id = "photos-${key.first.wire}-${key.second}",
                        at = newest.createdAt,
                        kind = TimelineKind.Photo,
                        title = if (group.size == 1) {
                            "${key.first.label} photo added"
                        } else {
                            "${group.size} ${key.first.label.lowercase()} photos added"
                        },
                        detail = group.mapNotNull { it.caption }.firstOrNull(),
                        actorName = null,
                    ),
                )
            }
            estimates.forEach {
                add(
                    TimelineEvent(
                        id = "estimate-${it.id}",
                        at = it.createdAt,
                        kind = if (it.isApproved) TimelineKind.Approval else TimelineKind.Estimate,
                        title = if (it.isApproved) "Customer approved the estimate" else "Estimate sent",
                        detail = it.notes,
                        actorName = null,
                    ),
                )
            }
            parts.filter { it.lineId != null }.forEach {
                add(
                    TimelineEvent(
                        id = "part-${it.lineId}",
                        at = createdAt,
                        kind = TimelineKind.Part,
                        title = "Part added",
                        detail = "${it.name} × ${it.quantity}",
                        actorName = null,
                    ),
                )
            }
        }
        return events.sortedByDescending { it.at }
    }

    private fun dayBucket(at: Long): Long = at / (60 * 60 * 1000)
}

/**
 * Contextual actions. Showing every possible action at once is how a technician
 * ends up reading a menu instead of working, so each status offers only the two
 * or three moves that make sense from where the job actually is.
 */
enum class JobAction(val label: String) {
    AddDiagnosis("Add diagnosis"),
    AddPhoto("Add photo"),
    AddNote("Add note"),
    SendEstimate("Send estimate"),
    RecordApproval("Record approval"),
    AddPart("Add part"),
    ResumeRepair("Resume repair"),
    MarkReady("Mark ready"),
    NotifyCustomer("Notify customer"),
    /** Opens handover — never a bare status flip past an unpaid balance. */
    MarkCollected("Mark collected"),
    TakePayment("Take payment"),
    UpdateStatus("Update status"),
    ;

    companion object {
        fun forStatus(
            status: JobStatus,
            needsApproval: Boolean,
            balanceDue: Double = 0.0,
        ): List<JobAction> = when (status) {
            JobStatus.Intake ->
                if (needsApproval) listOf(AddDiagnosis, AddPhoto, SendEstimate) else listOf(AddDiagnosis, AddPhoto)
            JobStatus.Diagnosed ->
                if (needsApproval) listOf(SendEstimate, RecordApproval, AddPhoto) else listOf(AddPart, AddNote, AddPhoto)
            JobStatus.WaitingParts -> listOf(AddPart, ResumeRepair, AddNote)
            JobStatus.InProgress -> listOf(AddNote, AddPart, MarkReady)
            JobStatus.ReadyForPickup, JobStatus.Completed -> buildList {
                if (balanceDue > 0.009) add(TakePayment)
                else add(MarkCollected)
                add(NotifyCustomer)
                add(AddPhoto)
            }
            JobStatus.Collected, JobStatus.Cancelled, JobStatus.Unrepairable -> listOf(AddNote)
        }
    }
}
