package com.techlane.ops.sync

import com.techlane.ops.TechLaneApp
import com.techlane.ops.data.SyncCommandEntity
import org.json.JSONObject
import java.time.Instant
import java.util.UUID

object OutboxRepository {
    suspend fun enqueue(
        commandType: String,
        payload: JSONObject,
        branchId: String? = TechLaneApp.instance.tokenStore.selectedBranchId,
    ): String {
        val actionId = UUID.randomUUID().toString()
        val me = runCatching { com.techlane.ops.network.ApiClient.me() }.getOrNull()
        val userId = me?.optString("id").orEmpty().ifBlank { "offline-user" }
        val tenantId = me?.optString("tenant_id").orEmpty().ifBlank { "offline-tenant" }
        val cmd = SyncCommandEntity(
            actionId = actionId,
            tenantId = tenantId,
            branchId = branchId,
            deviceId = TechLaneApp.instance.tokenStore.deviceId,
            userId = userId,
            commandType = commandType,
            localTimestamp = Instant.now().toString(),
            payloadJson = payload.toString(),
            syncStatus = "pending",
        )
        TechLaneApp.instance.database.syncOutboxDao().insert(cmd)
        SyncScheduler.enqueueNow(TechLaneApp.instance)
        return actionId
    }
}
