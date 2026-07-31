package com.techlane.core.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.techlane.core.theme.Brand

/** Vertical navy gradient used for hero headers and auth screens. */
fun brandGradient() = Brush.verticalGradient(
    colors = listOf(Brand.Navy, Brand.NavyDark, Brand.NavyDeep),
)

/** The TechLane equalizer mark: navy rounded square with gold/white bars. */
@Composable
fun TechLaneMark(size: Dp, modifier: Modifier = Modifier, onNavy: Boolean = false) {
    Canvas(modifier.size(size)) {
        val s = this.size.minDimension
        val u = s / 32f
        if (!onNavy) {
            drawRoundRect(
                color = Brand.Navy,
                cornerRadius = CornerRadius(6f * u, 6f * u),
            )
        } else {
            drawRoundRect(
                color = Color.White.copy(alpha = 0.10f),
                cornerRadius = CornerRadius(6f * u, 6f * u),
            )
            drawRoundRect(
                color = Color.White.copy(alpha = 0.22f),
                cornerRadius = CornerRadius(6f * u, 6f * u),
                style = androidx.compose.ui.graphics.drawscope.Stroke(width = 1f * u),
            )
        }
        fun bar(x: Float, y: Float, h: Float, color: Color) {
            drawRoundRect(
                color = color,
                topLeft = Offset(x * u, y * u),
                size = Size(3f * u, h * u),
                cornerRadius = CornerRadius(1.5f * u, 1.5f * u),
            )
        }
        bar(5f, 10f, 12f, Brand.Gold)
        bar(11f, 8f, 16f, Color.White)
        bar(17f, 12f, 8f, Brand.Gold)
        bar(23f, 7f, 18f, Color.White)
    }
}

/**
 * Navy gradient hero header with the brand mark, page title, and optional
 * trailing action + bottom slot (search fields, chips, stat strips).
 * Draws behind the status bar; place it as the first element of the screen.
 */
@Composable
fun BrandHero(
    title: String,
    modifier: Modifier = Modifier,
    subtitle: String? = null,
    appLabel: String? = null,
    trailing: @Composable (() -> Unit)? = null,
    bottomContent: @Composable (ColumnScope.() -> Unit)? = null,
) {
    val compact = LocalWindowLayout.current.compactChrome
    val corner = if (compact) 16.dp else 28.dp
    val markSize = if (compact) 22.dp else 30.dp
    val topPad = if (compact) 4.dp else 10.dp
    val bottomPad = if (compact) 12.dp else 20.dp
    val sidePad = if (compact) 14.dp else 20.dp
    val titleGap = if (compact) 8.dp else 18.dp
    Box(
        modifier
            .fillMaxWidth()
            .background(
                brush = brandGradient(),
                shape = RoundedCornerShape(bottomStart = corner, bottomEnd = corner),
            ),
    ) {
        Column(
            Modifier
                .statusBarsPadding()
                .padding(start = sidePad, end = sidePad, top = topPad, bottom = bottomPad),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                TechLaneMark(markSize, onNavy = true)
                Spacer(Modifier.size(if (compact) 8.dp else 10.dp))
                Text(
                    "TechLane",
                    style = if (compact) MaterialTheme.typography.titleSmall else MaterialTheme.typography.titleMedium,
                    color = Color.White,
                    fontWeight = FontWeight.Bold,
                    letterSpacing = 0.2.sp,
                )
                if (appLabel != null) {
                    Spacer(Modifier.size(8.dp))
                    Surface(
                        color = Brand.Gold.copy(alpha = 0.16f),
                        shape = RoundedCornerShape(999.dp),
                        border = BorderStroke(1.dp, Brand.Gold.copy(alpha = 0.45f)),
                    ) {
                        Text(
                            appLabel.uppercase(),
                            style = MaterialTheme.typography.labelSmall,
                            color = Brand.Gold,
                            fontWeight = FontWeight.Bold,
                            letterSpacing = 1.sp,
                            modifier = Modifier.padding(
                                horizontal = if (compact) 6.dp else 8.dp,
                                vertical = if (compact) 2.dp else 3.dp,
                            ),
                        )
                    }
                }
                Spacer(Modifier.weight(1f))
                trailing?.invoke()
            }
            Spacer(Modifier.height(titleGap))
            Text(
                title,
                style = if (compact) MaterialTheme.typography.titleLarge else MaterialTheme.typography.headlineMedium,
                color = Color.White,
                fontWeight = FontWeight.Bold,
            )
            if (subtitle != null) {
                Spacer(Modifier.height(2.dp))
                Text(
                    subtitle,
                    style = if (compact) MaterialTheme.typography.bodySmall else MaterialTheme.typography.bodyMedium,
                    color = Color.White.copy(alpha = 0.72f),
                )
            }
            if (bottomContent != null) {
                Spacer(Modifier.height(if (compact) 10.dp else 16.dp))
                bottomContent()
            }
        }
    }
}

/** Compact hero for detail screens: back arrow slot, title, status slot. */
@Composable
fun BrandDetailHeader(
    title: String,
    modifier: Modifier = Modifier,
    subtitle: String? = null,
    navigation: @Composable (() -> Unit)? = null,
    trailing: @Composable (() -> Unit)? = null,
) {
    val compact = LocalWindowLayout.current.compactChrome
    val corner = if (compact) 14.dp else 24.dp
    Box(
        modifier
            .fillMaxWidth()
            .background(
                brush = brandGradient(),
                shape = RoundedCornerShape(bottomStart = corner, bottomEnd = corner),
            ),
    ) {
        Row(
            Modifier
                .statusBarsPadding()
                .padding(
                    start = 6.dp,
                    end = if (compact) 12.dp else 16.dp,
                    top = if (compact) 2.dp else 4.dp,
                    bottom = if (compact) 10.dp else 16.dp,
                ),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            navigation?.invoke()
            Column(
                Modifier
                    .weight(1f)
                    .padding(start = if (navigation != null) 2.dp else 14.dp),
            ) {
                Text(
                    title,
                    style = if (compact) MaterialTheme.typography.titleMedium else MaterialTheme.typography.titleLarge,
                    color = Color.White,
                    fontWeight = FontWeight.Bold,
                )
                if (subtitle != null) {
                    Text(
                        subtitle,
                        style = MaterialTheme.typography.bodySmall,
                        color = Color.White.copy(alpha = 0.72f),
                    )
                }
            }
            trailing?.invoke()
        }
    }
}

/** High-emphasis gold call-to-action button (navy text on brand gold). */
@Composable
fun GoldButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    loading: Boolean = false,
) {
    val compact = LocalWindowLayout.current.compactChrome
    Button(
        onClick = onClick,
        enabled = enabled && !loading,
        modifier = modifier.height(if (compact) 44.dp else 52.dp),
        shape = RoundedCornerShape(if (compact) 12.dp else 14.dp),
        colors = ButtonDefaults.buttonColors(
            containerColor = Brand.Gold,
            contentColor = Brand.NavyDark,
            disabledContainerColor = Brand.Gold.copy(alpha = 0.4f),
            disabledContentColor = Brand.NavyDark.copy(alpha = 0.5f),
        ),
    ) {
        if (loading) {
            CircularProgressIndicator(
                modifier = Modifier.size(20.dp),
                strokeWidth = 2.dp,
                color = Brand.NavyDark,
            )
            Spacer(Modifier.size(10.dp))
        }
        Text(text, fontWeight = FontWeight.Bold, style = MaterialTheme.typography.titleSmall)
    }
}

/** Standard white card with soft border and shadow for light surfaces. */
@Composable
fun BrandCard(
    modifier: Modifier = Modifier,
    contentPadding: Dp = 16.dp,
    onClick: (() -> Unit)? = null,
    content: @Composable ColumnScope.() -> Unit,
) {
    val shape = RoundedCornerShape(18.dp)
    if (onClick != null) {
        Surface(
            onClick = onClick,
            modifier = modifier.fillMaxWidth(),
            shape = shape,
            color = Brand.Surface,
            border = BorderStroke(1.dp, Brand.Border),
            shadowElevation = 2.dp,
        ) {
            Column(Modifier.padding(contentPadding), content = content)
        }
    } else {
        Surface(
            modifier = modifier.fillMaxWidth(),
            shape = shape,
            color = Brand.Surface,
            border = BorderStroke(1.dp, Brand.Border),
            shadowElevation = 2.dp,
        ) {
            Column(Modifier.padding(contentPadding), content = content)
        }
    }
}

/** Rounded status/label pill with tinted background. */
@Composable
fun PillBadge(text: String, color: Color, modifier: Modifier = Modifier) {
    Surface(
        modifier = modifier,
        shape = RoundedCornerShape(999.dp),
        color = color.copy(alpha = 0.13f),
        border = BorderStroke(1.dp, color.copy(alpha = 0.3f)),
    ) {
        Text(
            text,
            style = MaterialTheme.typography.labelMedium,
            fontWeight = FontWeight.SemiBold,
            color = color,
            modifier = Modifier.padding(horizontal = 10.dp, vertical = 5.dp),
        )
    }
}

/** Small key metric tile used on dashboards ("stat strip"). */
@Composable
fun HeroStat(label: String, value: String, modifier: Modifier = Modifier) {
    val compact = LocalWindowLayout.current.compactChrome
    Surface(
        modifier = modifier,
        shape = RoundedCornerShape(if (compact) 10.dp else 14.dp),
        color = Color.White.copy(alpha = 0.08f),
        border = BorderStroke(1.dp, Color.White.copy(alpha = 0.14f)),
    ) {
        Column(
            Modifier.padding(
                horizontal = if (compact) 10.dp else 14.dp,
                vertical = if (compact) 6.dp else 10.dp,
            ),
        ) {
            Text(
                value,
                style = if (compact) MaterialTheme.typography.titleMedium else MaterialTheme.typography.titleLarge,
                color = Color.White,
                fontWeight = FontWeight.Bold,
            )
            Text(
                label,
                style = MaterialTheme.typography.labelSmall,
                color = Color.White.copy(alpha = 0.68f),
                letterSpacing = 0.4.sp,
            )
        }
    }
}

/**
 * Full-bleed auth screen brand block: large mark, wordmark, and app label.
 * Use at the top of login/OTP screens above a white form card.
 */
@Composable
fun BrandAuthHeader(
    appLabel: String,
    tagline: String,
    modifier: Modifier = Modifier,
) {
    val compact = LocalWindowLayout.current.compactChrome
    Column(
        modifier.fillMaxWidth(),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        TechLaneMark(if (compact) 44.dp else 72.dp, onNavy = true)
        Spacer(Modifier.height(if (compact) 8.dp else 16.dp))
        Text(
            "TechLane",
            style = if (compact) MaterialTheme.typography.headlineSmall else MaterialTheme.typography.headlineLarge,
            color = Color.White,
            fontWeight = FontWeight.Bold,
        )
        Spacer(Modifier.height(if (compact) 4.dp else 6.dp))
        Surface(
            color = Brand.Gold.copy(alpha = 0.16f),
            shape = RoundedCornerShape(999.dp),
            border = BorderStroke(1.dp, Brand.Gold.copy(alpha = 0.45f)),
        ) {
            Text(
                appLabel.uppercase(),
                style = MaterialTheme.typography.labelMedium,
                color = Brand.Gold,
                fontWeight = FontWeight.Bold,
                letterSpacing = 1.5.sp,
                modifier = Modifier.padding(horizontal = 12.dp, vertical = 4.dp),
            )
        }
        Spacer(Modifier.height(if (compact) 6.dp else 10.dp))
        Text(
            tagline,
            style = if (compact) MaterialTheme.typography.bodySmall else MaterialTheme.typography.bodyMedium,
            color = Color.White.copy(alpha = 0.72f),
            textAlign = TextAlign.Center,
        )
    }
}

/**
 * Slim banner shown when a newer app build is available. Non-dismissible
 * when [forceUpdate] is true (the backend has marked this build as below the
 * minimum supported version), otherwise the user can dismiss it for the
 * session via [onDismiss]. Place at the very top of the screen, above nav.
 */
@Composable
fun UpdateAvailableBanner(
    versionName: String,
    forceUpdate: Boolean,
    onUpdateClick: () -> Unit,
    modifier: Modifier = Modifier,
    onDismiss: (() -> Unit)? = null,
) {
    Surface(
        modifier = modifier.fillMaxWidth(),
        color = if (forceUpdate) Brand.Danger.copy(alpha = 0.08f) else Brand.GoldTint,
    ) {
        Row(
            Modifier.padding(horizontal = 16.dp, vertical = 10.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column(Modifier.weight(1f)) {
                Text(
                    if (forceUpdate) "Update required" else "Update available",
                    style = MaterialTheme.typography.labelLarge,
                    fontWeight = FontWeight.Bold,
                    color = if (forceUpdate) Brand.Danger else Brand.NavyDark,
                )
                Text(
                    if (forceUpdate) {
                        "Version $versionName is required to keep using TechLane."
                    } else {
                        "Version $versionName is ready — update for the latest fixes."
                    },
                    style = MaterialTheme.typography.bodySmall,
                    color = Brand.TextSecondary,
                )
            }
            Spacer(Modifier.size(10.dp))
            Surface(
                onClick = onUpdateClick,
                shape = RoundedCornerShape(10.dp),
                color = if (forceUpdate) Brand.Danger else Brand.Navy,
            ) {
                Text(
                    "Update",
                    style = MaterialTheme.typography.labelMedium,
                    fontWeight = FontWeight.Bold,
                    color = Color.White,
                    modifier = Modifier.padding(horizontal = 14.dp, vertical = 8.dp),
                )
            }
            if (!forceUpdate && onDismiss != null) {
                Spacer(Modifier.size(6.dp))
                Surface(onClick = onDismiss, shape = CircleShape, color = Color.Transparent) {
                    Text(
                        "✕",
                        style = MaterialTheme.typography.labelLarge,
                        color = Brand.TextMuted,
                        modifier = Modifier.padding(8.dp),
                    )
                }
            }
        }
    }
}

/** Section title with a short gold underline accent. */
@Composable
fun BrandSectionTitle(text: String, modifier: Modifier = Modifier) {
    Column(modifier) {
        Text(
            text,
            style = MaterialTheme.typography.titleMedium,
            fontWeight = FontWeight.Bold,
            color = Brand.TextPrimary,
        )
        Spacer(Modifier.height(5.dp))
        Box(
            Modifier
                .size(width = 28.dp, height = 3.dp)
                .background(Brand.Gold, CircleShape),
        )
    }
}

/**
 * Blocking "please wait" overlay for long-running POS / payment work.
 * Non-dismissible while [visible] — caller clears it when the job finishes.
 * Place inside a full-screen Box so it covers the UI.
 */
@Composable
fun PleaseWaitOverlay(
    visible: Boolean,
    message: String,
    modifier: Modifier = Modifier,
    detail: String? = null,
) {
    if (!visible) return
    Box(
        modifier
            .fillMaxSize()
            .background(Color.Black.copy(alpha = 0.45f)),
        contentAlignment = Alignment.Center,
    ) {
        Surface(
            shape = RoundedCornerShape(20.dp),
            color = Brand.Surface,
            shadowElevation = 8.dp,
            modifier = Modifier
                .padding(32.dp)
                .fillMaxWidth(),
        ) {
            Column(
                Modifier.padding(horizontal = 24.dp, vertical = 28.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                CircularProgressIndicator(color = Brand.Navy, strokeWidth = 3.dp)
                Text(
                    message,
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.Bold,
                    color = Brand.Navy,
                    textAlign = TextAlign.Center,
                )
                if (!detail.isNullOrBlank()) {
                    Text(
                        detail,
                        style = MaterialTheme.typography.bodySmall,
                        color = Brand.TextSecondary,
                        textAlign = TextAlign.Center,
                    )
                }
            }
        }
    }
}
