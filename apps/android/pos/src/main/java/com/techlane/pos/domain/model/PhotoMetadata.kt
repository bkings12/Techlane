package com.techlane.pos.domain.model

/**
 * Carries a photo's kind and caption inside the attachment file name.
 *
 * WHY THIS EXISTS — and what should replace it
 * --------------------------------------------
 * `repair.RepairAttachment` (internal/repair/service.go) stores only id,
 * file_name, content_type, size and audit columns. There is no field for what
 * stage a photo belongs to or what the technician wanted to say about it.
 *
 * Rather than invent a parallel photo API, or keep the metadata device-local
 * where it would vanish on reinstall and be invisible to the web console, the
 * two values ride inside the one string the backend already round-trips.
 *
 * The encoding is deliberately narrow and reversible:
 *
 *     tl~<kind>~<url-encoded caption>~<millis>.jpg
 *
 * Anything that does not parse is treated as an ordinary attachment, so photos
 * added by the web console still appear — they simply land under "Progress".
 *
 * MISSING BACKEND SUPPORT: add `kind` and `caption` columns to
 * repair.repair_attachments and expose them on RepairAttachment. When they
 * exist, [encode] becomes a plain file name and [decode] falls back to reading
 * the real fields; nothing above this codec has to change.
 */
object PhotoMetadata {

    private const val PREFIX = "tl"
    private const val SEP = "~"

    data class Decoded(val kind: PhotoKind, val caption: String?)

    fun encode(kind: PhotoKind, caption: String?, at: Long = System.currentTimeMillis()): String {
        val safeCaption = caption?.trim().orEmpty().let { encodeSegment(it) }
        return listOf(PREFIX, kind.wire, safeCaption, at.toString()).joinToString(SEP) + ".jpg"
    }

    fun decode(fileName: String?): Decoded {
        val name = fileName?.substringBeforeLast('.').orEmpty()
        val parts = name.split(SEP)
        if (parts.size < 4 || parts[0] != PREFIX) return Decoded(PhotoKind.Progress, null)
        return Decoded(
            kind = PhotoKind.fromWire(parts[1]),
            caption = decodeSegment(parts[2]).takeIf { it.isNotBlank() },
        )
    }

    /** Keeps the separator and path characters out of the encoded caption. */
    private fun encodeSegment(value: String): String = buildString {
        value.take(120).forEach { char ->
            when {
                char.isLetterOrDigit() || char == ' ' || char == '-' -> append(if (char == ' ') '+' else char)
                else -> append('.')
            }
        }
    }

    private fun decodeSegment(value: String): String = value.replace('+', ' ').trim()
}
