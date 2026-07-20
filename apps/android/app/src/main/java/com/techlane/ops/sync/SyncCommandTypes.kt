package com.techlane.ops.sync

/** Server-supported offline sync command types (POST /sync/commands). */
object SyncCommandTypes {
    const val REPAIR_CREATE_DRAFT = "repair.create_draft"
    const val REPAIR_ADD_NOTE = "repair.add_note"
    const val REPAIR_ADD_ATTACHMENT = "repair.add_attachment"
    const val PARTS_REQUEST = "parts.request"
    const val PAYMENTS_CASH_PROVISIONAL = "payments.cash_provisional"
}
