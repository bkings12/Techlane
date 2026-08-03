package com.techlane.pos.feature.scan

import android.Manifest
import android.annotation.SuppressLint
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.camera.core.CameraSelector
import androidx.camera.core.ImageAnalysis
import androidx.camera.core.Preview
import androidx.camera.lifecycle.ProcessCameraProvider
import androidx.camera.view.PreviewView
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.systemBarsPadding
import androidx.compose.foundation.shape.RoundedCornerShape
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
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewModelScope
import com.google.mlkit.vision.barcode.BarcodeScanning
import com.google.mlkit.vision.barcode.common.Barcode
import com.google.mlkit.vision.common.InputImage
import com.techlane.pos.core.designsystem.component.TlBanner
import com.techlane.pos.core.designsystem.component.TlButton
import com.techlane.pos.core.designsystem.component.TlSecondaryButton
import com.techlane.pos.core.designsystem.component.TlTextField
import com.techlane.pos.core.designsystem.component.TlTone
import com.techlane.pos.core.designsystem.theme.TlTheme
import com.techlane.pos.data.repository.JobRepository
import com.techlane.pos.feature.camera.hasCameraPermission
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.util.concurrent.Executors
import javax.inject.Inject

data class ScanUiState(
    val resolving: Boolean = false,
    val error: String? = null,
    val manualCode: String = "",
)

@HiltViewModel
class ScanViewModel @Inject constructor(
    private val jobs: JobRepository,
) : ViewModel() {

    private val _state = MutableStateFlow(ScanUiState())
    val state: StateFlow<ScanUiState> = _state.asStateFlow()

    fun setManualCode(value: String) = _state.update { it.copy(manualCode = value, error = null) }

    /**
     * Resolves a scan to a job. A pickup code goes straight to the job; anything
     * else falls back to search, which is honest about not knowing rather than
     * guessing at a format the backend has not defined.
     */
    fun resolve(raw: String, onJob: (String) -> Unit) {
        if (_state.value.resolving) return
        _state.update { it.copy(resolving = true, error = null) }
        viewModelScope.launch {
            when (val parsed = ScanPayloads.parse(raw)) {
                is ScanResult.RepairPickup -> jobs.findByPickupCode(parsed.code)
                    .onSuccess { onJob(it) }
                    .onFailure { error ->
                        _state.update { it.copy(error = error.message ?: "No job matches that code.") }
                    }

                is ScanResult.DeviceIdentifier -> resolveBySearch(parsed.value, onJob)
                is ScanResult.Unknown -> resolveBySearch(parsed.raw, onJob)
            }
            _state.update { it.copy(resolving = false) }
        }
    }

    private suspend fun resolveBySearch(term: String, onJob: (String) -> Unit) {
        jobs.searchJobs(term)
            .onSuccess { matches ->
                when {
                    matches.size == 1 -> onJob(matches.first().id)
                    matches.isEmpty() -> _state.update {
                        it.copy(error = "Nothing matches \"$term\".")
                    }
                    // Several hits is a search result, not a scan result — say so
                    // rather than opening whichever happened to sort first.
                    else -> _state.update {
                        it.copy(error = "${matches.size} jobs match that. Search for it on the Jobs board instead.")
                    }
                }
            }
            .onFailure { error -> _state.update { it.copy(error = error.message) } }
    }

    fun clearError() = _state.update { it.copy(error = null) }
}

/**
 * Scan a TechLane intake-slip QR to open its job.
 *
 * Manual entry sits underneath the viewfinder on purpose: slips get wet, torn
 * and thermal-faded, and a technician who cannot scan still needs the job.
 */
@SuppressLint("UnsafeOptInUsageError")
@Composable
fun ScanScreen(
    onJobResolved: (String) -> Unit,
    modifier: Modifier = Modifier,
    viewModel: ScanViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val context = LocalContext.current
    val lifecycleOwner = LocalLifecycleOwner.current
    var granted by remember { mutableStateOf(context.hasCameraPermission()) }
    var lastScan by remember { mutableStateOf<String?>(null) }

    val permissionLauncher = rememberLauncherForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) { granted = it }

    LaunchedEffect(Unit) { if (!granted) permissionLauncher.launch(Manifest.permission.CAMERA) }

    Surface(modifier = modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .systemBarsPadding()
                .padding(TlTheme.spacing.xl),
            verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.lg),
        ) {
            Text("Scan a job", style = MaterialTheme.typography.headlineSmall)
            Text(
                "Point at the QR code on the intake slip.",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )

            TlBanner(message = state.error, tone = TlTone.Warning)

            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .aspectRatio(1f)
                    .border(2.dp, MaterialTheme.colorScheme.primary, RoundedCornerShape(20.dp)),
                contentAlignment = Alignment.Center,
            ) {
                if (granted) {
                    BarcodeCamera(
                        lifecycleOwner = lifecycleOwner,
                        onBarcode = { value ->
                            // Debounced by value: a QR in frame fires many times a second.
                            if (value != lastScan && !state.resolving) {
                                lastScan = value
                                viewModel.resolve(value, onJobResolved)
                            }
                        },
                    )
                } else {
                    Column(
                        horizontalAlignment = Alignment.CenterHorizontally,
                        modifier = Modifier.padding(TlTheme.spacing.xl),
                    ) {
                        Text(
                            "Camera access is needed to scan job slips.",
                            style = MaterialTheme.typography.bodyMedium,
                            textAlign = TextAlign.Center,
                        )
                        TlButton(
                            text = "Allow camera",
                            onClick = { permissionLauncher.launch(Manifest.permission.CAMERA) },
                            modifier = Modifier.padding(top = TlTheme.spacing.md),
                        )
                    }
                }
            }

            Text(
                "Or type the code",
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            TlTextField(
                value = state.manualCode,
                onValueChange = viewModel::setManualCode,
                label = "Job or pickup code",
                placeholder = "PK-XXXXXX or job number",
                showClear = true,
            )
            TlSecondaryButton(
                text = if (state.resolving) "Looking…" else "Open job",
                onClick = { viewModel.resolve(state.manualCode, onJobResolved) },
                enabled = state.manualCode.isNotBlank() && !state.resolving,
                loading = state.resolving,
                modifier = Modifier.fillMaxWidth(),
            )
        }
    }
}

@SuppressLint("UnsafeOptInUsageError")
@Composable
private fun BarcodeCamera(
    lifecycleOwner: androidx.lifecycle.LifecycleOwner,
    onBarcode: (String) -> Unit,
) {
    val context = LocalContext.current
    val previewView = remember { PreviewView(context).apply { scaleType = PreviewView.ScaleType.FILL_CENTER } }
    val analysisExecutor = remember { Executors.newSingleThreadExecutor() }
    val scanner = remember { BarcodeScanning.getClient() }

    DisposableEffect(lifecycleOwner) {
        var provider: ProcessCameraProvider? = null
        val future = ProcessCameraProvider.getInstance(context)
        future.addListener(
            {
                provider = future.get()
                val analysis = ImageAnalysis.Builder()
                    .setBackpressureStrategy(ImageAnalysis.STRATEGY_KEEP_ONLY_LATEST)
                    .build()
                analysis.setAnalyzer(analysisExecutor) { proxy ->
                    val mediaImage = proxy.image
                    if (mediaImage == null) {
                        proxy.close()
                        return@setAnalyzer
                    }
                    val image = InputImage.fromMediaImage(mediaImage, proxy.imageInfo.rotationDegrees)
                    scanner.process(image)
                        .addOnSuccessListener { barcodes ->
                            barcodes.firstNotNullOfOrNull { it.rawValue ?: it.displayValue }
                                ?.let(onBarcode)
                        }
                        .addOnCompleteListener { proxy.close() }
                }
                runCatching {
                    provider?.unbindAll()
                    provider?.bindToLifecycle(
                        lifecycleOwner,
                        CameraSelector.DEFAULT_BACK_CAMERA,
                        Preview.Builder().build().also { it.setSurfaceProvider(previewView.surfaceProvider) },
                        analysis,
                    )
                }
            },
            ContextCompat.getMainExecutor(context),
        )
        onDispose {
            runCatching { provider?.unbindAll() }
            runCatching { scanner.close() }
            analysisExecutor.shutdown()
        }
    }

    AndroidView(factory = { previewView }, modifier = Modifier.fillMaxSize())
}

/** Formats supported today; kept explicit so adding one is a deliberate change. */
internal val SUPPORTED_FORMATS = intArrayOf(
    Barcode.FORMAT_QR_CODE,
    Barcode.FORMAT_CODE_128,
    Barcode.FORMAT_CODE_39,
    Barcode.FORMAT_EAN_13,
)
