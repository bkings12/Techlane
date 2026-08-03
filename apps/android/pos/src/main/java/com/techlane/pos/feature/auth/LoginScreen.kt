package com.techlane.pos.feature.auth

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.systemBarsPadding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Fingerprint
import androidx.compose.material.icons.outlined.Lock
import androidx.compose.material.icons.outlined.MailOutline
import androidx.compose.material.icons.outlined.Visibility
import androidx.compose.material.icons.outlined.VisibilityOff
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.fragment.app.FragmentActivity
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.techlane.pos.core.designsystem.component.TlBanner
import com.techlane.pos.core.designsystem.component.TlButton
import com.techlane.pos.core.designsystem.component.TlSecondaryButton
import com.techlane.pos.core.designsystem.component.TlTextButton
import com.techlane.pos.core.designsystem.component.TlTextField
import com.techlane.pos.core.designsystem.component.TlTone
import com.techlane.pos.core.designsystem.theme.PillShape
import com.techlane.pos.core.designsystem.theme.TlTheme

/**
 * Sign-in. Password first, always — biometrics only ever *resume* a session that
 * a password (and, where the account requires it, an MFA code) already created.
 */
@Composable
fun LoginScreen(
    onSignedIn: () -> Unit,
    modifier: Modifier = Modifier,
    viewModel: AuthViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val context = LocalContext.current
    val activity = context.findFragmentActivity()
    var passwordVisible by remember { mutableStateOf(false) }

    LaunchedEffect(state.signedIn, state.offerBiometricEnrolment) {
        if (state.signedIn && !state.offerBiometricEnrolment) onSignedIn()
    }

    // Offer the sensor straight away when it is already set up — the fastest
    // possible path back to the till after the screen locks.
    LaunchedEffect(state.biometricEnrolled) {
        if (state.biometricEnrolled && activity != null && !state.signedIn) {
            viewModel.unlockWithBiometrics(activity)
        }
    }

    Surface(modifier = modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Box(Modifier.fillMaxSize().systemBarsPadding().imePadding()) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .widthIn(max = 460.dp)
                    .align(Alignment.Center)
                    .verticalScroll(rememberScrollState())
                    .padding(horizontal = TlTheme.spacing.xxl, vertical = TlTheme.spacing.xxl),
                verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.lg),
            ) {
                BrandMark()
                Spacer(Modifier.height(TlTheme.spacing.xs))

                Text(
                    text = if (state.stage == LoginStage.Password) "Sign in to the till" else "Verification code",
                    style = MaterialTheme.typography.headlineMedium,
                    color = MaterialTheme.colorScheme.onBackground,
                )
                Text(
                    text = when (state.stage) {
                        LoginStage.Password -> "Use your TechLane staff account."
                        LoginStage.Mfa -> "Enter the 6-digit code from your authenticator app."
                    },
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )

                TlBanner(message = state.error, tone = TlTone.Danger)

                when (state.stage) {
                    LoginStage.Password -> {
                        TlTextField(
                            value = state.email,
                            onValueChange = viewModel::onEmailChange,
                            label = "Email",
                            placeholder = "you@techlane.co.ke",
                            leadingIcon = Icons.Outlined.MailOutline,
                            keyboardType = KeyboardType.Email,
                            enabled = !state.busy,
                        )
                        TlTextField(
                            value = state.password,
                            onValueChange = viewModel::onPasswordChange,
                            label = "Password",
                            leadingIcon = Icons.Outlined.Lock,
                            keyboardType = KeyboardType.Password,
                            imeAction = ImeAction.Done,
                            enabled = !state.busy,
                            visualTransformation = if (passwordVisible) {
                                VisualTransformation.None
                            } else {
                                PasswordVisualTransformation()
                            },
                            trailingIcon = {
                                IconButton(onClick = { passwordVisible = !passwordVisible }) {
                                    Icon(
                                        if (passwordVisible) Icons.Outlined.VisibilityOff else Icons.Outlined.Visibility,
                                        contentDescription = if (passwordVisible) "Hide password" else "Show password",
                                    )
                                }
                            },
                        )
                    }
                    LoginStage.Mfa -> {
                        TlTextField(
                            value = state.mfaCode,
                            onValueChange = viewModel::onMfaCodeChange,
                            label = "6-digit code",
                            placeholder = "000000",
                            keyboardType = KeyboardType.NumberPassword,
                            imeAction = ImeAction.Done,
                            enabled = !state.busy,
                        )
                    }
                }

                TlButton(
                    text = when {
                        state.busy -> "Please wait…"
                        state.stage == LoginStage.Mfa -> "Verify"
                        else -> "Sign in"
                    },
                    onClick = viewModel::submit,
                    enabled = state.canSubmit,
                    loading = state.busy,
                    large = true,
                    modifier = Modifier.fillMaxWidth(),
                )

                if (state.stage == LoginStage.Mfa) {
                    TlTextButton(
                        text = "Use a different account",
                        onClick = viewModel::backToPassword,
                        modifier = Modifier.align(Alignment.CenterHorizontally),
                    )
                }

                if (state.stage == LoginStage.Password && state.biometricEnrolled && activity != null) {
                    TlSecondaryButton(
                        text = "Unlock with fingerprint",
                        onClick = { viewModel.unlockWithBiometrics(activity) },
                        icon = Icons.Outlined.Fingerprint,
                        enabled = !state.busy,
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
            }
        }
    }

    if (state.offerBiometricEnrolment && activity != null) {
        BiometricEnrolmentPrompt(
            onEnable = { viewModel.enableBiometrics(activity, onSignedIn) },
            onSkip = { viewModel.skipBiometricEnrolment(onSignedIn) },
        )
    }
}

@Composable
private fun BrandMark() {
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(TlTheme.spacing.md)) {
        Surface(shape = PillShape, color = MaterialTheme.colorScheme.primary, modifier = Modifier.size(48.dp)) {
            Box(contentAlignment = Alignment.Center) {
                Text(
                    "TL",
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onPrimary,
                )
            }
        }
        Column {
            Text("TechLane", style = MaterialTheme.typography.titleLarge, color = MaterialTheme.colorScheme.onBackground)
            Text(
                "Counter & repairs",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

@Composable
private fun BiometricEnrolmentPrompt(onEnable: () -> Unit, onSkip: () -> Unit) {
    androidx.compose.ui.window.Dialog(onDismissRequest = onSkip) {
        Surface(shape = MaterialTheme.shapes.large, color = MaterialTheme.colorScheme.surface) {
            Column(
                modifier = Modifier.padding(TlTheme.spacing.xxl),
                verticalArrangement = Arrangement.spacedBy(TlTheme.spacing.lg),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Surface(shape = PillShape, color = MaterialTheme.colorScheme.primaryContainer, modifier = Modifier.size(64.dp)) {
                    Box(contentAlignment = Alignment.Center) {
                        Icon(
                            Icons.Outlined.Fingerprint,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.onPrimaryContainer,
                            modifier = Modifier.size(32.dp),
                        )
                    }
                }
                Text(
                    "Turn on fingerprint sign-in?",
                    style = MaterialTheme.typography.titleLarge,
                    textAlign = TextAlign.Center,
                )
                Text(
                    "Next time, one touch opens the till. Your password is never stored on this phone.",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    textAlign = TextAlign.Center,
                )
                TlButton(
                    text = "Turn on",
                    onClick = onEnable,
                    icon = Icons.Outlined.Fingerprint,
                    modifier = Modifier.fillMaxWidth(),
                )
                TlTextButton(text = "Not now", onClick = onSkip)
            }
        }
    }
}

/** BiometricPrompt needs a FragmentActivity; unwrap whatever context we're in. */
internal fun android.content.Context.findFragmentActivity(): FragmentActivity? {
    var context = this
    while (context is android.content.ContextWrapper) {
        if (context is FragmentActivity) return context
        context = context.baseContext
    }
    return null
}
