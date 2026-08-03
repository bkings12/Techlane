package com.techlane.pos.data.repository

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * The set of job mutations that can be made offline and replayed later.
 *
 * Each variant is a value the technician already committed to. Replay order is
 * insertion order, which matters: "add diagnosis" then "move to bench" must
 * reach the server the same way round, or the server's own transition rules
 * would reject the pair.
 */
@Serializable
sealed interface JobAction {

    @Serializable
    @SerialName("status")
    data class ChangeStatus(
        val status: String,
        val note: String? = null,
        val closureReason: String? = null,
        val varianceReason: String? = null,
    ) : JobAction

    @Serializable
    @SerialName("assign")
    data class Assign(val technicianId: String) : JobAction

    @Serializable
    @SerialName("note")
    data class AddNote(val localId: String, val text: String) : JobAction

    @Serializable
    @SerialName("estimate")
    data class SendEstimate(val total: Double, val notes: String? = null) : JobAction

    @Serializable
    @SerialName("authorize")
    data class AuthorizeWork(val note: String, val amount: Double? = null) : JobAction

    @Serializable
    @SerialName("part_add")
    data class AddPart(
        val localId: String,
        val variantId: String,
        val locationId: String,
        val quantity: Int,
    ) : JobAction

    @Serializable
    @SerialName("part_remove")
    data class RemovePart(val lineId: String) : JobAction

    @Serializable
    @SerialName("photo")
    data class UploadPhoto(val photoId: String) : JobAction

    @Serializable
    @SerialName("sms")
    data class SendCustomerUpdate(val phone: String, val body: String, val customerId: String? = null) : JobAction
}

/**
 * How many times a queued action is retried before it is treated as stuck and
 * shown to the technician. The server rejecting a transition is not something
 * more retries will fix, and silently dropping it would be worse.
 */
const val OUTBOX_STUCK_THRESHOLD = 4
