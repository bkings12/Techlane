package com.techlane.ops.ui

import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Inbox
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import com.techlane.core.theme.Brand
import com.techlane.core.theme.statusPalette
import com.techlane.core.ui.BrandCard
import com.techlane.core.ui.BrandSectionTitle
import com.techlane.core.ui.PillBadge
import androidx.compose.runtime.saveable.Saver
import org.json.JSONObject

/** Lets a JSONObject survive rotation/process death via `rememberSaveable`, by round-tripping
 * through its string form — cheap for the small record-shaped payloads this app deals in
 * (a created job, a payment), not meant for large lists. */
val JsonObjectSaver: Saver<JSONObject?, String> = Saver(
    save = { it?.toString() ?: "" },
    restore = { s -> if (s.isEmpty()) null else JSONObject(s) },
)

/** Lets a POS-style cart (variant/SKU id -> quantity) survive rotation/process death —
 * losing a half-built sale at the counter is exactly the failure mode this guards. */
val StringIntMapSaver: Saver<Map<String, Int>, String> = Saver(
    save = { map -> map.entries.joinToString(";") { "${it.key}=${it.value}" } },
    restore = { s ->
        if (s.isEmpty()) emptyMap() else s.split(";").associate { entry ->
            val (k, v) = entry.split("=", limit = 2)
            k to v.toInt()
        }
    },
)

/** Catalog POS cart line — quantity plus an optional bargained sell price. */
data class PosCartLine(
    val variantId: String,
    val qty: Int,
    val listPrice: Double,
    val overridePrice: Double? = null,
    val overrideReason: String = "",
) {
    fun unitPrice(): Double = overridePrice ?: listPrice

    fun isBargained(): Boolean {
        val ov = overridePrice ?: return false
        return kotlin.math.abs(ov - listPrice) > 0.009
    }

    fun toJson(): JSONObject {
        val o = JSONObject()
            .put("variant_id", variantId)
            .put("qty", qty)
            .put("list_price", listPrice)
            .put("override_reason", overrideReason)
        if (overridePrice != null) o.put("override_price", overridePrice) else o.put("override_price", JSONObject.NULL)
        return o
    }

    companion object {
        fun fromJson(o: JSONObject) = PosCartLine(
            variantId = o.getString("variant_id"),
            qty = o.getInt("qty"),
            listPrice = o.getDouble("list_price"),
            overridePrice = if (o.isNull("override_price")) null else o.optDouble("override_price"),
            overrideReason = o.optString("override_reason"),
        )
    }
}

val PosCartLineListSaver: Saver<List<PosCartLine>, List<String>> = Saver(
    save = { list -> list.map { it.toJson().toString() } },
    restore = { list -> list.map { PosCartLine.fromJson(JSONObject(it)) } },
)

/** Same idea as JsonObjectSaver, for a small list of records (e.g. quick-sale cart
 * lines) — round-trips each element through its string form. */
val JsonListSaver: Saver<List<JSONObject>, List<String>> = Saver(
    save = { list -> list.map { it.toString() } },
    restore = { list -> list.map { JSONObject(it) } },
)

@Composable
fun statusColor(status: String): Color {
    val palette = statusPalette()
    return when (status) {
        "intake" -> palette.intake
        "diagnosed" -> palette.diagnosed
        "waiting_parts" -> palette.waitingParts
        "in_progress" -> palette.inProgress
        "ready_for_pickup", "completed" -> palette.completed
        "collected" -> palette.collected
        "cancelled", "unrepairable" -> Brand.Danger
        else -> palette.collected
    }
}

fun statusLabel(status: String): String = when (status) {
    "ready_for_pickup" -> "QC"
    "in_progress" -> "Repairing"
    "waiting_parts" -> "Waiting parts"
    "completed" -> "Ready"
    else -> status.replace('_', ' ')
}

@Composable
fun StatusChip(status: String) {
    PillBadge(
        text = statusLabel(status).replaceFirstChar { it.uppercase() },
        color = statusColor(status),
    )
}

@Composable
fun SectionLabel(text: String, tileModifier: Modifier = Modifier) {
    Text(
        text = text.uppercase(),
        style = MaterialTheme.typography.labelMedium,
        fontWeight = FontWeight.SemiBold,
        color = Brand.TextMuted,
        modifier = tileModifier.padding(bottom = 4.dp),
    )
}

@Composable
fun MetricTile(
    label: String,
    value: String,
    tileModifier: Modifier = Modifier,
    accent: Color? = null,
) {
    Surface(
        modifier = tileModifier,
        shape = RoundedCornerShape(18.dp),
        color = Brand.Surface,
        border = BorderStroke(1.dp, Brand.Border),
        shadowElevation = 2.dp,
    ) {
        Column(
            Modifier.padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                if (accent != null) {
                    Box(
                        Modifier
                            .size(8.dp)
                            .background(accent, CircleShape),
                    )
                }
                Text(
                    text = label,
                    style = MaterialTheme.typography.labelMedium,
                    color = Brand.TextSecondary,
                )
            }
            Text(
                text = value,
                style = MaterialTheme.typography.headlineSmall,
                fontWeight = FontWeight.SemiBold,
                color = Brand.TextPrimary,
            )
        }
    }
}

@Composable
fun EmptyHint(
    message: String,
    hintModifier: Modifier = Modifier,
    title: String? = null,
    icon: ImageVector = Icons.Default.Inbox,
) {
    Surface(
        modifier = hintModifier.fillMaxWidth(),
        shape = RoundedCornerShape(18.dp),
        color = Brand.Surface,
        border = BorderStroke(1.dp, Brand.Border),
    ) {
        Column(
            Modifier.padding(28.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Surface(
                shape = RoundedCornerShape(14.dp),
                color = Brand.NavyTint,
                modifier = Modifier.size(52.dp),
            ) {
                Box(contentAlignment = Alignment.Center) {
                    Icon(
                        icon,
                        contentDescription = null,
                        tint = Brand.Navy,
                        modifier = Modifier.size(26.dp),
                    )
                }
            }
            if (title != null) {
                Text(
                    title,
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.SemiBold,
                    textAlign = TextAlign.Center,
                    color = Brand.TextPrimary,
                )
            }
            Text(
                text = message,
                style = MaterialTheme.typography.bodyMedium,
                color = Brand.TextSecondary,
                textAlign = TextAlign.Center,
            )
        }
    }
}

@Composable
fun ScreenHeader(
    title: String,
    subtitle: String? = null,
    headerModifier: Modifier = Modifier,
    action: @Composable (() -> Unit)? = null,
) {
    Row(
        headerModifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.Top,
    ) {
        Column(Modifier.weight(1f)) {
            BrandSectionTitle(title)
            if (subtitle != null) {
                Spacer(Modifier.height(6.dp))
                Text(
                    text = subtitle,
                    style = MaterialTheme.typography.bodyMedium,
                    color = Brand.TextSecondary,
                )
            }
        }
        action?.invoke()
    }
}

@Composable
fun FormSection(
    title: String,
    sectionModifier: Modifier = Modifier,
    content: @Composable ColumnScope.() -> Unit,
) {
    BrandCard(modifier = sectionModifier) {
        BrandSectionTitle(title)
        Spacer(Modifier.height(8.dp))
        Column(verticalArrangement = Arrangement.spacedBy(12.dp), content = content)
    }
}

@Composable
fun FeedbackBanner(
    message: String?,
    error: String?,
    modifier: Modifier = Modifier,
) {
    val text = error ?: message
    if (text.isNullOrBlank()) return
    val isError = !error.isNullOrBlank()
    Surface(
        modifier = modifier.fillMaxWidth(),
        shape = RoundedCornerShape(18.dp),
        color = if (isError) {
            MaterialTheme.colorScheme.errorContainer
        } else {
            Brand.Success.copy(alpha = 0.12f)
        },
        border = BorderStroke(
            1.dp,
            if (isError) MaterialTheme.colorScheme.error.copy(alpha = 0.35f)
            else Brand.Success.copy(alpha = 0.35f),
        ),
    ) {
        Text(
            text = text,
            style = MaterialTheme.typography.bodyMedium,
            fontWeight = FontWeight.Medium,
            color = if (isError) MaterialTheme.colorScheme.onErrorContainer
            else Brand.Success,
            modifier = Modifier.padding(horizontal = 14.dp, vertical = 12.dp),
        )
    }
}

fun android.content.Context.showAppToast(message: String, long: Boolean = false) {
    android.widget.Toast.makeText(
        this,
        message,
        if (long) android.widget.Toast.LENGTH_LONG else android.widget.Toast.LENGTH_SHORT,
    ).show()
}

fun Throwable.isLikelyNetworkFailure(): Boolean {
    var cur: Throwable? = this
    while (cur != null) {
        when (cur) {
            is java.io.IOException,
            is java.net.SocketTimeoutException,
            is java.net.UnknownHostException,
            is java.net.ConnectException -> return true
        }
        val msg = cur.message.orEmpty().lowercase()
        if (msg.contains("unable to resolve host") ||
            msg.contains("failed to connect") ||
            msg.contains("timeout") ||
            msg.contains("network")
        ) {
            return true
        }
        cur = cur.cause
    }
    return false
}

/** Shimmering placeholder bar for "content is loading" — replaces a blank screen with
 * something that visibly reads as in-progress rather than broken. */
@Composable
private fun ShimmerBlock(modifier: Modifier = Modifier, height: Dp = 16.dp) {
    val transition = rememberInfiniteTransition(label = "shimmer")
    val alpha by transition.animateFloat(
        initialValue = 0.35f,
        targetValue = 0.85f,
        animationSpec = infiniteRepeatable(
            animation = tween(700),
            repeatMode = RepeatMode.Reverse,
        ),
        label = "shimmerAlpha",
    )
    Box(
        modifier
            .height(height)
            .fillMaxWidth()
            .background(Brand.Subtle.copy(alpha = alpha), RoundedCornerShape(8.dp)),
    )
}

/** Placeholder card matching BrandCard's shape/padding, for lists loading for the first time. */
@Composable
fun SkeletonCard(modifier: Modifier = Modifier) {
    Surface(
        modifier = modifier.fillMaxWidth(),
        shape = RoundedCornerShape(18.dp),
        color = Brand.Surface,
        border = BorderStroke(1.dp, Brand.Border),
    ) {
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            ShimmerBlock(Modifier.fillMaxWidth(0.55f))
            ShimmerBlock(Modifier.fillMaxWidth(0.85f), height = 12.dp)
            ShimmerBlock(Modifier.fillMaxWidth(0.4f), height = 12.dp)
        }
    }
}

/** A handful of SkeletonCards, standing in for a list's first load. */
@Composable
fun SkeletonList(count: Int = 3, modifier: Modifier = Modifier) {
    Column(modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        repeat(count) { SkeletonCard() }
    }
}
