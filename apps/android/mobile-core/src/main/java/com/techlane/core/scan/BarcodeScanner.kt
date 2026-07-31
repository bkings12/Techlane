package com.techlane.core.scan

import android.Manifest
import android.content.pm.PackageManager
import android.view.ViewGroup
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.camera.core.ImageAnalysis
import androidx.camera.mlkit.vision.MlKitAnalyzer
import androidx.camera.view.CameraController
import androidx.camera.view.LifecycleCameraController
import androidx.camera.view.PreviewView
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Cameraswitch
import androidx.compose.material.icons.filled.FlashOff
import androidx.compose.material.icons.filled.FlashOn
import androidx.compose.material3.Button
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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.LocalLifecycleOwner
import com.google.mlkit.vision.barcode.BarcodeScannerOptions
import com.google.mlkit.vision.barcode.BarcodeScanning
import com.google.mlkit.vision.barcode.common.Barcode
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicReference

@Composable
fun CameraPermissionGate(
    modifier: Modifier = Modifier,
    rationale: String = "Camera access is required to scan QR codes and barcodes.",
    content: @Composable () -> Unit,
) {
    val context = LocalContext.current
    var granted by remember {
        mutableStateOf(
            ContextCompat.checkSelfPermission(context, Manifest.permission.CAMERA) ==
                PackageManager.PERMISSION_GRANTED,
        )
    }
    val launcher = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) {
        granted = it
    }

    if (granted) {
        content()
    } else {
        Column(
            modifier = modifier
                .fillMaxWidth()
                .padding(16.dp)
                .background(Color(0xFFF5F6FB), RoundedCornerShape(12.dp))
                .padding(20.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Text(rationale, style = MaterialTheme.typography.bodyMedium)
            Button(onClick = { launcher.launch(Manifest.permission.CAMERA) }) {
                Text("Enable camera")
            }
        }
    }
}

@Composable
fun LiveBarcodeScanner(
    onCode: (String) -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
) {
    val context = LocalContext.current
    val lifecycleOwner = LocalLifecycleOwner.current
    var torchOn by remember { mutableStateOf(false) }
    var useFront by remember { mutableStateOf(false) }
    var flashLabel by remember { mutableStateOf<String?>(null) }
    var statusHint by remember { mutableStateOf("Point at a QR code") }
    val acceptScans = remember { AtomicBoolean(enabled) }
    val onCodeRef = remember { AtomicReference(onCode) }
    val lastScanRef = remember { AtomicReference<Pair<String, Long>?>(null) }
    onCodeRef.set(onCode)

    val analysisExecutor = remember { Executors.newSingleThreadExecutor() }
    val barcodeClient = remember {
        val options = BarcodeScannerOptions.Builder()
            .setBarcodeFormats(
                Barcode.FORMAT_QR_CODE,
                Barcode.FORMAT_AZTEC,
                Barcode.FORMAT_DATA_MATRIX,
                Barcode.FORMAT_CODE_128,
                Barcode.FORMAT_CODE_39,
                Barcode.FORMAT_CODE_93,
                Barcode.FORMAT_EAN_13,
                Barcode.FORMAT_EAN_8,
                Barcode.FORMAT_UPC_A,
                Barcode.FORMAT_UPC_E,
                Barcode.FORMAT_ITF,
            )
            .build()
        BarcodeScanning.getClient(options)
    }
    val cameraController = remember {
        LifecycleCameraController(context).apply {
            // Preview attaches via PreviewView; analysis-only is enough for QR/barcode.
            setEnabledUseCases(CameraController.IMAGE_ANALYSIS)
            imageAnalysisTargetSize = CameraController.OutputSize(android.util.Size(1280, 720))
        }
    }

    LaunchedEffect(enabled) {
        acceptScans.set(enabled)
        if (!enabled) {
            flashLabel = null
            statusHint = "Looking up…"
        } else {
            statusHint = "Point at a QR code"
        }
    }

    LaunchedEffect(useFront) {
        cameraController.cameraSelector = if (useFront) {
            androidx.camera.core.CameraSelector.DEFAULT_FRONT_CAMERA
        } else {
            androidx.camera.core.CameraSelector.DEFAULT_BACK_CAMERA
        }
    }

    LaunchedEffect(torchOn) {
        runCatching {
            if (cameraController.cameraInfo?.hasFlashUnit() == true) {
                cameraController.enableTorch(torchOn)
            }
        }
    }

    DisposableEffect(Unit) {
        onDispose {
            analysisExecutor.shutdown()
            runCatching { barcodeClient.close() }
        }
    }

    DisposableEffect(lifecycleOwner, cameraController) {
        // LifecycleCameraController + PreviewView is the supported path for reliable QR reads.
        cameraController.setImageAnalysisAnalyzer(
            analysisExecutor,
            MlKitAnalyzer(
                listOf(barcodeClient),
                // ORIGINAL is reliable with CameraController; VIEW_REFERENCED needs extra wiring
                // and often yields empty results on shop-floor devices.
                ImageAnalysis.COORDINATE_SYSTEM_ORIGINAL,
                ContextCompat.getMainExecutor(context),
            ) { result ->
                if (!acceptScans.get()) return@MlKitAnalyzer
                val barcodes = result?.getValue(barcodeClient).orEmpty()
                val value = barcodes.firstNotNullOfOrNull { barcode ->
                    barcode.rawValue?.trim()?.takeIf { it.isNotEmpty() }
                        ?: barcode.displayValue?.trim()?.takeIf { it.isNotEmpty() }
                } ?: return@MlKitAnalyzer
                val now = System.currentTimeMillis()
                val prev = lastScanRef.get()
                if (prev != null && prev.first == value && now - prev.second < 1_800) return@MlKitAnalyzer
                lastScanRef.set(value to now)
                flashLabel = value.take(40)
                statusHint = "Scanned"
                onCodeRef.get()?.invoke(value)
            },
        )
        cameraController.bindToLifecycle(lifecycleOwner)
        onDispose {
            runCatching { cameraController.clearImageAnalysisAnalyzer() }
        }
    }

    Box(
        modifier = modifier
            .fillMaxWidth()
            .height(340.dp)
            .background(Color.Black, RoundedCornerShape(16.dp)),
    ) {
        AndroidView(
            factory = { ctx ->
                PreviewView(ctx).also { view ->
                    view.layoutParams = ViewGroup.LayoutParams(
                        ViewGroup.LayoutParams.MATCH_PARENT,
                        ViewGroup.LayoutParams.MATCH_PARENT,
                    )
                    view.implementationMode = PreviewView.ImplementationMode.COMPATIBLE
                    view.scaleType = PreviewView.ScaleType.FILL_CENTER
                    view.controller = cameraController
                }
            },
            update = { view ->
                if (view.controller !== cameraController) {
                    view.controller = cameraController
                }
            },
            modifier = Modifier.fillMaxSize(),
        )
        Box(
            modifier = Modifier
                .align(Alignment.Center)
                .size(230.dp, 230.dp)
                .border(
                    width = 3.dp,
                    color = if (flashLabel != null) Color(0xFFF2BE2A) else Color.White.copy(alpha = 0.9f),
                    shape = RoundedCornerShape(18.dp),
                ),
        )
        flashLabel?.let { label ->
            Surface(
                color = Color(0xF2060386),
                shape = RoundedCornerShape(10.dp),
                modifier = Modifier
                    .align(Alignment.TopCenter)
                    .padding(top = 14.dp),
            ) {
                Text(
                    "Scanned: $label",
                    color = Color.White,
                    style = MaterialTheme.typography.labelLarge,
                    fontWeight = FontWeight.SemiBold,
                    modifier = Modifier.padding(horizontal = 12.dp, vertical = 8.dp),
                )
            }
        }
        if (!enabled) {
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .background(Color.Black.copy(alpha = 0.45f)),
                contentAlignment = Alignment.Center,
            ) {
                Text("Looking up…", color = Color.White, fontWeight = FontWeight.SemiBold)
            }
        }
        Surface(
            color = Color.Black.copy(alpha = 0.55f),
            shape = RoundedCornerShape(bottomStart = 16.dp, bottomEnd = 16.dp),
            modifier = Modifier
                .align(Alignment.BottomCenter)
                .fillMaxWidth(),
        ) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 8.dp, vertical = 4.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    statusHint,
                    color = Color.White,
                    style = MaterialTheme.typography.labelMedium,
                    modifier = Modifier.padding(start = 8.dp),
                )
                Row {
                    IconButton(onClick = { torchOn = !torchOn }) {
                        Icon(
                            if (torchOn) Icons.Filled.FlashOn else Icons.Filled.FlashOff,
                            contentDescription = "Torch",
                            tint = Color.White,
                        )
                    }
                    IconButton(onClick = { useFront = !useFront }) {
                        Icon(Icons.Filled.Cameraswitch, contentDescription = "Switch camera", tint = Color.White)
                    }
                }
            }
        }
    }
}

@Composable
fun ScanCameraPanel(
    onCode: (String) -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
) {
    CameraPermissionGate(modifier = modifier) {
        LiveBarcodeScanner(onCode = onCode, enabled = enabled)
    }
}
