package com.techlane.pos.feature.camera

import android.Manifest
import android.content.Context
import android.content.pm.PackageManager
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.camera.core.CameraSelector
import androidx.camera.core.ImageCapture
import androidx.camera.core.ImageCaptureException
import androidx.camera.core.Preview
import androidx.camera.lifecycle.ProcessCameraProvider
import androidx.camera.view.PreviewView
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.systemBarsPadding
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Close
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.core.content.ContextCompat
import com.techlane.pos.core.designsystem.component.TlButton
import com.techlane.pos.core.designsystem.component.TlSecondaryButton
import com.techlane.pos.core.designsystem.component.TlTextField
import com.techlane.pos.core.designsystem.theme.TlTheme
import com.techlane.pos.domain.model.PhotoKind
import java.io.File

/**
 * Fast device documentation.
 *
 * Deliberately not a gallery picker: a technician photographing a cracked hinge
 * wants viewfinder-then-shutter, and every extra screen between those two is a
 * photo that does not get taken.
 */
@Composable
fun JobCameraScreen(
    kind: PhotoKind,
    onCaptured: (File, String?) -> Unit,
    onCancel: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val context = LocalContext.current
    val lifecycleOwner = LocalLifecycleOwner.current
    var granted by remember { mutableStateOf(context.hasCameraPermission()) }
    var captured by remember { mutableStateOf<File?>(null) }
    var caption by remember { mutableStateOf("") }
    var error by remember { mutableStateOf<String?>(null) }

    val permissionLauncher = rememberLauncherForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) { granted = it }

    LaunchedEffect(Unit) {
        if (!granted) permissionLauncher.launch(Manifest.permission.CAMERA)
    }

    val imageCapture = remember {
        ImageCapture.Builder()
            .setCaptureMode(ImageCapture.CAPTURE_MODE_MINIMIZE_LATENCY)
            .build()
    }

    Surface(modifier = modifier.fillMaxSize(), color = Color.Black) {
        Box(Modifier.fillMaxSize()) {
            val pending = captured
            when {
                !granted -> PermissionPrompt(
                    onGrant = { permissionLauncher.launch(Manifest.permission.CAMERA) },
                    onCancel = onCancel,
                )

                pending != null -> CaptionStep(
                    kind = kind,
                    caption = caption,
                    onCaptionChange = { caption = it },
                    onSave = { onCaptured(pending, caption.takeIf(String::isNotBlank)) },
                    onRetake = {
                        runCatching { pending.delete() }
                        captured = null
                        caption = ""
                    },
                )

                else -> {
                    CameraPreview(imageCapture = imageCapture, lifecycleOwner = lifecycleOwner)
                    ShutterBar(
                        kindLabel = kind.label,
                        error = error,
                        onCancel = onCancel,
                        onShutter = {
                            val file = context.newPhotoFile()
                            imageCapture.takePicture(
                                ImageCapture.OutputFileOptions.Builder(file).build(),
                                ContextCompat.getMainExecutor(context),
                                object : ImageCapture.OnImageSavedCallback {
                                    override fun onImageSaved(output: ImageCapture.OutputFileResults) {
                                        captured = file
                                        error = null
                                    }

                                    override fun onError(exception: ImageCaptureException) {
                                        error = exception.message ?: "Could not take the photo"
                                    }
                                },
                            )
                        },
                    )
                }
            }
        }
    }
}

@Composable
private fun CameraPreview(imageCapture: ImageCapture, lifecycleOwner: androidx.lifecycle.LifecycleOwner) {
    val context = LocalContext.current
    val previewView = remember { PreviewView(context).apply { scaleType = PreviewView.ScaleType.FILL_CENTER } }

    DisposableEffect(lifecycleOwner) {
        var provider: ProcessCameraProvider? = null
        val future = ProcessCameraProvider.getInstance(context)
        future.addListener(
            {
                provider = future.get()
                runCatching {
                    provider?.unbindAll()
                    provider?.bindToLifecycle(
                        lifecycleOwner,
                        CameraSelector.DEFAULT_BACK_CAMERA,
                        Preview.Builder().build().also { it.setSurfaceProvider(previewView.surfaceProvider) },
                        imageCapture,
                    )
                }
            },
            ContextCompat.getMainExecutor(context),
        )
        onDispose { runCatching { provider?.unbindAll() } }
    }

    AndroidView(factory = { previewView }, modifier = Modifier.fillMaxSize())
}

@Composable
private fun ShutterBar(
    kindLabel: String,
    error: String?,
    onCancel: () -> Unit,
    onShutter: () -> Unit,
) {
    Column(Modifier.fillMaxSize().systemBarsPadding()) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(TlTheme.spacing.md),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            IconButton(onClick = onCancel) {
                Icon(Icons.Outlined.Close, contentDescription = "Cancel", tint = Color.White)
            }
            Surface(shape = CircleShape, color = Color.Black.copy(alpha = 0.5f)) {
                Text(
                    kindLabel,
                    style = MaterialTheme.typography.labelMedium,
                    color = Color.White,
                    modifier = Modifier.padding(horizontal = TlTheme.spacing.md, vertical = 6.dp),
                )
            }
        }
        Box(Modifier.weight(1f).fillMaxWidth(), contentAlignment = Alignment.BottomCenter) {
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                if (error != null) {
                    Text(
                        error,
                        style = MaterialTheme.typography.bodySmall,
                        color = Color.White,
                        modifier = Modifier.padding(TlTheme.spacing.md),
                    )
                }
                Surface(
                    onClick = onShutter,
                    shape = CircleShape,
                    color = Color.White,
                    modifier = Modifier.padding(bottom = 40.dp).size(76.dp),
                ) {
                    Box(Modifier.fillMaxSize().padding(6.dp)) {
                        Box(
                            Modifier
                                .fillMaxSize()
                                .background(Color.White, CircleShape),
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun CaptionStep(
    kind: PhotoKind,
    caption: String,
    onCaptionChange: (String) -> Unit,
    onSave: () -> Unit,
    onRetake: () -> Unit,
) {
    Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .systemBarsPadding()
                .padding(TlTheme.spacing.xl),
            verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.lg),
        ) {
            Text("${kind.label} photo", style = MaterialTheme.typography.headlineSmall)
            TlTextField(
                value = caption,
                onValueChange = onCaptionChange,
                label = "Caption (optional)",
                placeholder = "e.g. Hairline crack on the top-right corner",
                singleLine = false,
            )
            Box(Modifier.weight(1f))
            TlButton(text = "Save photo", onClick = onSave, modifier = Modifier.fillMaxWidth())
            TlSecondaryButton(text = "Retake", onClick = onRetake, modifier = Modifier.fillMaxWidth())
        }
    }
}

@Composable
private fun PermissionPrompt(onGrant: () -> Unit, onCancel: () -> Unit) {
    Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(
            modifier = Modifier.fillMaxSize().padding(TlTheme.spacing.xxl),
            verticalArrangement = Arrangement.Center,
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Text(
                "Camera access is needed to photograph devices",
                style = MaterialTheme.typography.titleMedium,
                textAlign = TextAlign.Center,
            )
            Text(
                "Photos stay on this phone until they sync to the job.",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                textAlign = TextAlign.Center,
                modifier = Modifier.padding(vertical = TlTheme.spacing.md),
            )
            TlButton(text = "Allow camera", onClick = onGrant, modifier = Modifier.fillMaxWidth())
            TlSecondaryButton(text = "Not now", onClick = onCancel, modifier = Modifier.fillMaxWidth())
        }
    }
}

internal fun Context.hasCameraPermission(): Boolean =
    ContextCompat.checkSelfPermission(this, Manifest.permission.CAMERA) == PackageManager.PERMISSION_GRANTED

/**
 * Photos live in app-private storage until they upload. Nothing lands in the
 * device gallery — a customer's cracked handset is not the technician's camera roll.
 */
private fun Context.newPhotoFile(): File {
    val dir = File(filesDir, "job-photos").apply { mkdirs() }
    return File(dir, "job-${System.currentTimeMillis()}.jpg")
}
