package com.techlane.pos.feature.jobs.components

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.AddAPhoto
import androidx.compose.material.icons.outlined.CheckCircle
import androidx.compose.material.icons.outlined.CloudQueue
import androidx.compose.material.icons.outlined.Close
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.Inventory2
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import coil.compose.AsyncImage
import com.techlane.pos.core.designsystem.component.TlCard
import com.techlane.pos.core.designsystem.component.TlDivider
import com.techlane.pos.core.designsystem.component.TlKeyValue
import com.techlane.pos.core.designsystem.component.TlSecondaryButton
import com.techlane.pos.core.designsystem.component.TlStatusPill
import com.techlane.pos.core.designsystem.component.TlTone
import com.techlane.pos.core.designsystem.theme.PillShape
import com.techlane.pos.core.designsystem.theme.TlTheme
import com.techlane.pos.core.util.formatKes
import com.techlane.pos.domain.model.JobPart
import com.techlane.pos.domain.model.JobPhoto
import com.techlane.pos.domain.model.TimelineEvent
import com.techlane.pos.domain.model.TimelineKind
import com.techlane.pos.domain.model.WorkApproval
import java.text.SimpleDateFormat
import java.util.Calendar
import java.util.Date
import java.util.Locale

/**
 * Customer approval state.
 *
 * When a job is not approved this is not a passive display — it is the thing
 * standing between the technician and the bench, so it carries the two actions
 * that resolve it.
 */
@Composable
fun ApprovalCard(
    approval: WorkApproval,
    pendingEstimateTotal: Double?,
    onSendEstimate: () -> Unit,
    onRecordApproval: () -> Unit,
    modifier: Modifier = Modifier,
) {
    TlCard(modifier = modifier) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text("Customer approval", style = MaterialTheme.typography.titleSmall)
            TlStatusPill(
                text = if (approval.isApproved) "Approved" else "Not approved",
                tone = if (approval.isApproved) TlTone.Success else TlTone.Warning,
            )
        }

        if (approval.isApproved) {
            approval.amount?.let { TlKeyValue("Agreed price", formatKes(it), emphasise = true) }
            approval.approvedAt?.let { TlKeyValue("Approved", fullDateFormat.format(Date(it))) }
            approval.method?.let { TlKeyValue("Method", it.label) }
            approval.approvedByName?.let { TlKeyValue("Recorded by", it) }
        } else {
            Text(
                "Customer approval is required before repair work can begin.",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            pendingEstimateTotal?.let {
                TlKeyValue("Estimate awaiting decision", formatKes(it), emphasise = true)
            }
            Row(horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm)) {
                TlSecondaryButton(
                    text = "Send estimate",
                    onClick = onSendEstimate,
                    modifier = Modifier.weight(1f),
                )
                TlSecondaryButton(
                    text = "Record approval",
                    onClick = onRecordApproval,
                    modifier = Modifier.weight(1f),
                )
            }
        }
    }
}

/**
 * One section of the work order: labour, parts, or products, each their own
 * card so a technician scanning the job sees revenue by category the way the
 * shop's accounting does, not one undifferentiated list. All three read the
 * same underlying line-item rows (`/repairs/{id}/line-items`), filtered by
 * type before this is called — see [JobPart.lineType].
 */
@Composable
fun LineItemsCard(
    title: String,
    parts: List<JobPart>,
    canEdit: Boolean,
    onAdd: () -> Unit,
    addLabel: String,
    emptyLabel: String,
    onRemove: (JobPart) -> Unit,
    modifier: Modifier = Modifier,
    onMarkPartRequired: (() -> Unit)? = null,
) {
    TlCard(modifier = modifier) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(title, style = MaterialTheme.typography.titleSmall)
            if (parts.isNotEmpty()) {
                Text(
                    formatKes(parts.sumOf { it.lineTotal }),
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface,
                )
            }
        }

        if (parts.isEmpty()) {
            Text(
                emptyLabel,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        } else {
            parts.forEach { part ->
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm),
                ) {
                    Column(modifier = Modifier.weight(1f)) {
                        Text(
                            part.name,
                            style = MaterialTheme.typography.bodyMedium,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                        Text(
                            listOfNotNull(
                                part.sku,
                                "× ${part.quantity}",
                                part.partStatus?.replaceFirstChar(Char::uppercase),
                                if (part.partSource == "sourced") "sourced" else null,
                            ).joinToString(" · "),
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                    if (part.pendingSync) {
                        Icon(
                            Icons.Outlined.CloudQueue,
                            contentDescription = "Not synced",
                            tint = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.size(16.dp),
                        )
                    }
                    Text(formatKes(part.lineTotal), style = MaterialTheme.typography.bodyMedium)
                    // Removal is only offered before the job is finished; after
                    // that the line is part of what the customer was billed.
                    if (canEdit) {
                        IconButton(onClick = { onRemove(part) }) {
                            Icon(
                                Icons.Outlined.Delete,
                                contentDescription = "Remove ${part.name}",
                                tint = MaterialTheme.colorScheme.error,
                            )
                        }
                    }
                }
                TlDivider()
            }
        }

        if (canEdit) {
            Row(horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm)) {
                TlSecondaryButton(text = addLabel, onClick = onAdd, modifier = Modifier.weight(1f))
                if (onMarkPartRequired != null) {
                    TlSecondaryButton(
                        text = "Part required",
                        onClick = onMarkPartRequired,
                        modifier = Modifier.weight(1f),
                    )
                }
            }
        }
    }
}

/** Horizontal photo strip, grouped by stage. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun RepairPhotoGallery(
    photos: List<JobPhoto>,
    onAddPhoto: () -> Unit,
    onDeletePhoto: (JobPhoto) -> Unit,
    photoUrl: (JobPhoto) -> String?,
    modifier: Modifier = Modifier,
) {
    TlCard(modifier = modifier) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text("Photos", style = MaterialTheme.typography.titleSmall)
            Text(
                "${photos.size}",
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }

        LazyRow(horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.sm)) {
            item {
                Surface(
                    onClick = onAddPhoto,
                    shape = RoundedCornerShape(14.dp),
                    color = MaterialTheme.colorScheme.surfaceVariant,
                    modifier = Modifier.size(96.dp),
                ) {
                    Column(
                        modifier = Modifier.padding(TlTheme.spacing.sm),
                        horizontalAlignment = Alignment.CenterHorizontally,
                        verticalArrangement = Arrangement.Center,
                    ) {
                        Icon(
                            Icons.Outlined.AddAPhoto,
                            contentDescription = "Add photo",
                            tint = MaterialTheme.colorScheme.primary,
                        )
                        Spacer(Modifier.height(TlTheme.spacing.xs))
                        Text("Add", style = MaterialTheme.typography.labelSmall)
                    }
                }
            }
            items(photos, key = { it.id }) { photo ->
                PhotoTile(photo = photo, url = photoUrl(photo), onDelete = { onDeletePhoto(photo) })
            }
        }

        if (photos.isEmpty()) {
            Text(
                "No photos yet. Document the device before you start — it protects both sides.",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

@Composable
private fun PhotoTile(photo: JobPhoto, url: String?, onDelete: () -> Unit) {
    Box(modifier = Modifier.size(96.dp)) {
        Surface(
            shape = RoundedCornerShape(14.dp),
            color = MaterialTheme.colorScheme.surfaceVariant,
            modifier = Modifier.size(96.dp),
        ) {
            AsyncImage(
                model = photo.localPath ?: url,
                contentDescription = photo.caption ?: photo.kind.label,
                contentScale = ContentScale.Crop,
                modifier = Modifier.size(96.dp),
            )
        }
        // Stage badge, so a strip of similar-looking photos is still readable.
        Surface(
            shape = PillShape,
            color = MaterialTheme.colorScheme.scrim.copy(alpha = 0.65f),
            modifier = Modifier.align(Alignment.BottomStart).padding(4.dp),
        ) {
            Text(
                photo.kind.label,
                style = MaterialTheme.typography.labelSmall,
                color = androidx.compose.ui.graphics.Color.White,
                modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp),
            )
        }
        if (!photo.uploaded) {
            Surface(
                shape = PillShape,
                color = MaterialTheme.colorScheme.scrim.copy(alpha = 0.65f),
                modifier = Modifier.align(Alignment.TopStart).padding(4.dp),
            ) {
                Icon(
                    Icons.Outlined.CloudQueue,
                    contentDescription = "Waiting to upload",
                    tint = androidx.compose.ui.graphics.Color.White,
                    modifier = Modifier.size(14.dp).padding(1.dp),
                )
            }
            // Only an un-uploaded photo can be deleted — see JobRepository.
            IconButton(
                onClick = onDelete,
                modifier = Modifier.align(Alignment.TopEnd).size(28.dp),
            ) {
                Surface(shape = PillShape, color = MaterialTheme.colorScheme.scrim.copy(alpha = 0.65f)) {
                    Icon(
                        Icons.Outlined.Close,
                        contentDescription = "Delete photo",
                        tint = androidx.compose.ui.graphics.Color.White,
                        modifier = Modifier.size(20.dp).padding(3.dp),
                    )
                }
            }
        }
    }
}

/**
 * The repair timeline, grouped by day.
 *
 * Composed on the client from status events, notes, photos, estimates and parts:
 * the API keeps those in separate tables and exposes no single feed, and adding
 * one would have meant a backend concept the web console does not have.
 */
@Composable
fun RepairTimeline(events: List<TimelineEvent>, modifier: Modifier = Modifier) {
    TlCard(modifier = modifier) {
        Text("Timeline", style = MaterialTheme.typography.titleSmall)
        if (events.isEmpty()) {
            Text(
                "Nothing recorded yet.",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            return@TlCard
        }
        events.groupBy { dayLabel(it.at) }.forEach { (day, dayEvents) ->
            Text(
                day.uppercase(),
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.padding(top = TlTheme.spacing.sm),
            )
            dayEvents.forEach { event -> TimelineRow(event) }
        }
    }
}

@Composable
private fun TimelineRow(event: TimelineEvent) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.md),
    ) {
        Text(
            timeFormat.format(Date(event.at)),
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.width(64.dp),
        )
        // The rail: a dot per event, joined by a hairline.
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Box(
                modifier = Modifier
                    .size(9.dp)
                    .background(event.kind.tone(), PillShape),
            )
            Box(
                modifier = Modifier
                    .width(1.dp)
                    .height(28.dp)
                    .background(TlTheme.colors.hairline),
            )
        }
        Column(
            modifier = Modifier.weight(1f).padding(bottom = TlTheme.spacing.sm),
            verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.xxs),
        ) {
            Text(event.title, style = MaterialTheme.typography.bodyMedium)
            event.detail?.let {
                Text(
                    "“$it”",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 4,
                    overflow = TextOverflow.Ellipsis,
                )
            }
            event.actorName?.let {
                Text(
                    it,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }
    }
}

@Composable
private fun TimelineKind.tone(): androidx.compose.ui.graphics.Color = when (this) {
    TimelineKind.Received -> TlTheme.colors.info
    TimelineKind.Approval -> TlTheme.colors.success
    TimelineKind.Collected -> TlTheme.colors.success
    TimelineKind.Diagnosis, TimelineKind.Note -> MaterialTheme.colorScheme.primary
    TimelineKind.Photo -> TlTheme.colors.accent
    TimelineKind.Estimate -> TlTheme.colors.warning
    TimelineKind.Part -> TlTheme.colors.warning
    TimelineKind.CustomerNotified -> TlTheme.colors.info
    TimelineKind.StatusChange, TimelineKind.Assigned -> MaterialTheme.colorScheme.onSurfaceVariant
}

/** Small, always-visible sync state. Never blocks; just tells the truth. */
@Composable
fun SyncStatusIndicator(pendingCount: Int, modifier: Modifier = Modifier) {
    if (pendingCount <= 0) return
    Surface(
        shape = PillShape,
        color = TlTheme.colors.warningContainer,
        modifier = modifier,
    ) {
        Row(
            modifier = Modifier.padding(horizontal = TlTheme.spacing.md, vertical = 6.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(6.dp),
        ) {
            Icon(
                Icons.Outlined.CloudQueue,
                contentDescription = null,
                tint = TlTheme.colors.onWarningContainer,
                modifier = Modifier.size(14.dp),
            )
            Text(
                if (pendingCount == 1) "1 change waiting to sync" else "$pendingCount changes waiting to sync",
                style = MaterialTheme.typography.labelSmall,
                color = TlTheme.colors.onWarningContainer,
            )
        }
    }
}

@Composable
fun JobSectionHeading(text: String, modifier: Modifier = Modifier) {
    Text(
        text.uppercase(),
        style = MaterialTheme.typography.labelSmall,
        color = MaterialTheme.colorScheme.onSurfaceVariant,
        modifier = modifier,
    )
}

@Composable
fun EmptyTick() = Icon(
    Icons.Outlined.CheckCircle,
    contentDescription = null,
    tint = TlTheme.colors.success,
    modifier = Modifier.size(16.dp),
)

private val timeFormat = SimpleDateFormat("h:mm a", Locale.getDefault())
private val fullDateFormat = SimpleDateFormat("d MMM yyyy, h:mm a", Locale.getDefault())
private val dayFormat = SimpleDateFormat("EEEE d MMMM", Locale.getDefault())

internal fun dayLabel(at: Long): String {
    val today = Calendar.getInstance()
    val target = Calendar.getInstance().apply { timeInMillis = at }
    fun sameDay(a: Calendar, b: Calendar) =
        a.get(Calendar.YEAR) == b.get(Calendar.YEAR) && a.get(Calendar.DAY_OF_YEAR) == b.get(Calendar.DAY_OF_YEAR)
    if (sameDay(today, target)) return "Today"
    today.add(Calendar.DAY_OF_YEAR, -1)
    if (sameDay(today, target)) return "Yesterday"
    return dayFormat.format(Date(at))
}

@Composable
fun PartsIcon() = Icon(Icons.Outlined.Inventory2, contentDescription = null)
