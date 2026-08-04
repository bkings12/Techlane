package com.techlane.pos.data.repository

import com.techlane.pos.data.local.SaleCacheDao
import com.techlane.pos.data.local.SaleCacheEntity
import com.techlane.pos.data.remote.OfflineException
import com.techlane.pos.data.remote.TechLaneApi
import com.techlane.pos.data.remote.dto.SaleDto
import com.techlane.pos.data.remote.toAppException
import com.techlane.pos.domain.model.SaleDetail
import com.techlane.pos.domain.model.SaleLineItem
import com.techlane.pos.domain.model.SaleSummary
import com.techlane.pos.domain.model.SalesFilter
import kotlinx.serialization.json.Json
import java.time.LocalDate
import java.time.ZoneOffset
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Sales history is read-only and server-authoritative — unlike jobs, there is
 * no offline queue here, because a sale/payment is never created or changed
 * from this screen. The only offline concession is a small view cache
 * ([SaleCacheDao]) so a recently opened receipt survives a dropped connection
 * (per the "receipt availability must not depend on a foreground session"
 * requirement) — it is refreshed opportunistically, never trusted over a live
 * fetch when one succeeds.
 */
@Singleton
class SalesRepository @Inject constructor(
    private val api: TechLaneApi,
    private val cacheDao: SaleCacheDao,
) {
    private val json = Json { ignoreUnknownKeys = true }

    suspend fun listSales(filter: SalesFilter, branchId: String? = null): Result<List<SaleSummary>> = runCatching {
        val from = filter.fromEpochDay?.let { LocalDate.ofEpochDay(it).toString() }
        val to = filter.toEpochDay?.let { LocalDate.ofEpochDay(it).toString() }
        val res = api.sales(
            branchId = branchId,
            limit = 100,
            query = filter.query.trim().takeIf(String::isNotBlank),
            method = filter.method,
            status = filter.status,
            from = from,
            to = to,
        )
        res.items.map { it.toSummary() }
    }.recoverCatching { throw it.toAppException() }

    /**
     * Network-first, cache-fallback. A sale is immutable once completed (a
     * refund creates its own trail, it doesn't rewrite this record), so a
     * stale cache entry is still a *correct* one — just possibly missing a
     * very recent payment reconciliation.
     */
    suspend fun getSale(saleId: String): Result<SaleDetail> {
        val fresh = runCatching {
            val dto = api.sale(saleId)
            cacheDao.put(SaleCacheEntity(saleId, json.encodeToString(SaleDto.serializer(), dto), System.currentTimeMillis()))
            dto.toDetail(fromCache = false)
        }
        if (fresh.isSuccess) return fresh

        val mapped = fresh.exceptionOrNull()?.toAppException()
        if (mapped is OfflineException) {
            val cached = cacheDao.get(saleId)
            if (cached != null) {
                return runCatching { json.decodeFromString(SaleDto.serializer(), cached.json).toDetail(fromCache = true) }
            }
        }
        return Result.failure(mapped ?: fresh.exceptionOrNull() ?: IllegalStateException("unknown error"))
    }

    /** Rendered receipt HTML — for the "View Receipt" screen. */
    suspend fun receiptHtml(saleId: String): Result<String> = runCatching {
        api.saleReceiptHtml(saleId).use { it.string() }
    }.recoverCatching { throw it.toAppException() }

    /** Rendered receipt PDF bytes — for "Share Receipt". */
    suspend fun receiptPdf(saleId: String): Result<ByteArray> = runCatching {
        api.saleReceiptPdf(saleId).use { it.bytes() }
    }.recoverCatching { throw it.toAppException() }

    private fun SaleDto.toSummary(): SaleSummary {
        val names = items.map { it.description }.filter(String::isNotBlank)
        return SaleSummary(
            id = id,
            reference = reference.ifBlank { "SL-${id.take(8).uppercase()}" },
            customerName = customerName?.takeIf(String::isNotBlank),
            total = total,
            paymentMethod = paymentMethod,
            status = status,
            createdAt = createdAt.toEpochMillis(),
            itemSummary = names.firstOrNull(),
            itemCount = items.size,
        )
    }

    private fun SaleDto.toDetail(fromCache: Boolean): SaleDetail = SaleDetail(
        id = id,
        reference = reference.ifBlank { "SL-${id.take(8).uppercase()}" },
        status = status,
        channel = channel,
        branchName = branchName.takeIf(String::isNotBlank),
        cashierName = cashierName.takeIf(String::isNotBlank),
        customerName = customerName?.takeIf(String::isNotBlank),
        customerPhone = customerPhone?.takeIf(String::isNotBlank),
        createdAt = createdAt.toEpochMillis(),
        items = items.map {
            SaleLineItem(
                description = it.description.ifBlank { "Item" },
                quantity = it.quantity,
                unitPrice = it.unitPrice,
                lineTotal = it.lineTotal,
                unitCost = it.unitCost,
                margin = it.margin,
            )
        },
        subtotal = subtotal,
        taxTotal = taxTotal,
        discountTotal = discountTotal,
        total = total,
        paidTotal = paidTotal,
        balanceDue = balanceDue,
        paymentMethod = paymentMethod,
        paymentStatus = paymentStatus,
        paymentReference = paymentReference.takeIf(String::isNotBlank),
        fromCache = fromCache,
    )
}

internal fun epochDayFor(daysAgo: Long): Long = LocalDate.now(ZoneOffset.UTC).minusDays(daysAgo).toEpochDay()
