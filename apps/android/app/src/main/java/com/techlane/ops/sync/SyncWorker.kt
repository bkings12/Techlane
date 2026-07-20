package com.techlane.ops.sync

import android.content.Context
import androidx.work.CoroutineWorker
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.ExistingWorkPolicy
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import com.techlane.ops.TechLaneApp
import com.techlane.ops.network.ApiClient
import org.json.JSONObject
import java.util.concurrent.TimeUnit

class SyncWorker(appContext: Context, params: WorkerParameters) : CoroutineWorker(appContext, params) {
    override suspend fun doWork(): Result {
        OutboxFlush.flushPending()
        return Result.success()
    }
}

object OutboxFlush {
    suspend fun flushPending(): Int {
        val dao = TechLaneApp.instance.database.syncOutboxDao()
        val pending = dao.pending()
        var synced = 0
        for (cmd in pending) {
            var persistedPayload = cmd.payloadJson
            try {
                dao.update(cmd.copy(syncStatus = "syncing"))
                val payload = JSONObject(cmd.payloadJson)
                ensureRepairDraftIds(payload)
                persistedPayload = payload.toString()
                dao.update(cmd.copy(syncStatus = "syncing", payloadJson = persistedPayload))
                val body = JSONObject()
                    .put("action_id", cmd.actionId)
                    .put("command_type", cmd.commandType)
                    .put("local_timestamp", cmd.localTimestamp)
                    .put("payload", payload)
                cmd.branchId?.let { body.put("branch_id", it) }
                cmd.deviceId?.let { body.put("device_id", it) }
                body.put("payload", payload)

                val res = ApiClient.post("/sync/commands", body, cmd.actionId)
                val serverStatus = res.optString("sync_status")
                when (serverStatus) {
                    "applied" -> {
                        dao.update(
                            cmd.copy(
                                syncStatus = "synced",
                                lastError = null,
                                payloadJson = payload.toString(),
                            ),
                        )
                        synced++
                    }
                    "failed" -> dao.update(
                        cmd.copy(
                            syncStatus = "failed",
                            retryCount = cmd.retryCount + 1,
                            lastError = res.optString("error").ifBlank { "failed" },
                            payloadJson = payload.toString(),
                        ),
                    )
                    "conflict" -> dao.update(
                        cmd.copy(
                            syncStatus = "conflict",
                            retryCount = cmd.retryCount + 1,
                            lastError = res.optString("error").ifBlank { "conflict" },
                            payloadJson = payload.toString(),
                        ),
                    )
                    else -> dao.update(
                        cmd.copy(
                            syncStatus = "synced",
                            lastError = null,
                            payloadJson = payload.toString(),
                        ),
                    )
                }
            } catch (e: Exception) {
                val msg = e.message.orEmpty()
                val status = if (msg.contains("payload mismatch", ignoreCase = true) || msg.contains("CONFLICT")) {
                    "conflict"
                } else {
                    "failed"
                }
                dao.update(
                    cmd.copy(
                        syncStatus = status,
                        retryCount = cmd.retryCount + 1,
                        lastError = e.message,
                        payloadJson = persistedPayload,
                    ),
                )
            }
        }
        return synced
    }

    private fun ensureRepairDraftIds(payload: JSONObject) {
        if (payload.optString("branch_id").isNotBlank() && payload.optString("device_id").isNotBlank()) {
            return
        }
        val me = ApiClient.me()
        val branches = me.optJSONArray("branch_ids")
        val allowed = if (branches != null) (0 until branches.length()).map { branches.getString(it) } else emptyList()
        val branchId = when {
            payload.optString("branch_id").isNotBlank() -> payload.getString("branch_id")
            TechLaneApp.instance.tokenStore.selectedBranchId in allowed -> TechLaneApp.instance.tokenStore.selectedBranchId!!
            allowed.isNotEmpty() -> allowed.first()
            else -> error("No branch available for draft repair")
        }
        payload.put("branch_id", branchId)

        if (payload.optString("customer_id").isBlank() && payload.optString("customer_name").isNotBlank()) {
            val customer = ApiClient.createCustomer(
                payload.getString("customer_name"),
                payload.optString("customer_phone").ifBlank { null },
            )
            payload.put("customer_id", customer.getString("id"))
        }

        if (payload.optString("device_id").isBlank()) {
            val device = ApiClient.createDevice(
                customerId = payload.optString("customer_id").ifBlank { null },
                kind = payload.optString("kind", "phone"),
                brand = payload.optString("brand").ifBlank { null },
                model = payload.optString("model").ifBlank { null },
                imei = payload.optString("imei").ifBlank { null },
            )
            payload.put("device_id", device.getString("id"))
        }
    }
}

object SyncScheduler {
    fun ensurePeriodic(context: Context) {
        val req = PeriodicWorkRequestBuilder<SyncWorker>(15, TimeUnit.MINUTES).build()
        WorkManager.getInstance(context).enqueueUniquePeriodicWork(
            "techlane-sync",
            ExistingPeriodicWorkPolicy.KEEP,
            req,
        )
    }

    fun enqueueNow(context: Context) {
        val req = OneTimeWorkRequestBuilder<SyncWorker>().build()
        WorkManager.getInstance(context).enqueueUniqueWork(
            "techlane-sync-now",
            ExistingWorkPolicy.REPLACE,
            req,
        )
    }
}
