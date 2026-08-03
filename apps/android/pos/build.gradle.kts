import java.util.Properties

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.compose")
    id("org.jetbrains.kotlin.plugin.serialization")
    id("com.google.devtools.ksp")
    id("com.google.dagger.hilt.android")
}

// Push notifications stay dormant until a google-services.json is dropped in —
// applying the plugin without one fails the build for everyone else.
if (file("google-services.json").exists()) {
    apply(plugin = "com.google.gms.google-services")
}

android {
    namespace = "com.techlane.pos"
    compileSdk = 35

    defaultConfig {
        applicationId = "com.techlane.pos"
        minSdk = 26
        targetSdk = 35
        // Release versioning: bump BOTH for every release build.
        //   versionCode  — plain integer, strictly increasing, never reused.
        //   versionName  — semver shown to staff and on the download page.
        // See apps/android/RELEASE.md for the full release checklist.
        versionCode = 4
        versionName = "1.0.3"
        vectorDrawables.useSupportLibrary = true
    }

    // -PapiBase=https://host/api/v1 overrides both variants (LAN testing, staging).
    val apiBaseOverride = (project.findProperty("apiBase") as String?)?.trim().orEmpty()

    // ---------------------------------------------------------------- signing
    //
    // Production signing credentials never live in source. They are read, in
    // order of precedence, from:
    //   1. Gradle properties -PTECHLANE_KEYSTORE_PATH=... (what CI passes)
    //   2. Environment variables TECHLANE_KEYSTORE_PATH, ... (a local shell)
    //   3. apps/android/keystore.properties (gitignored; a developer's own copy)
    //
    // See apps/android/RELEASE.md for how the keystore is generated, backed up,
    // and rotated (it cannot be rotated for an app already in the hands of
    // staff — Android refuses updates signed by a different key).
    fun signingProp(name: String): String? =
        (project.findProperty(name) as String?)?.takeIf { it.isNotBlank() }
            ?: System.getenv(name)?.takeIf { it.isNotBlank() }

    val keystorePropsFile = rootProject.file("keystore.properties")
    val keystoreProps = Properties().apply {
        if (keystorePropsFile.exists()) keystorePropsFile.inputStream().use { stream -> load(stream) }
    }

    fun signingValue(propKey: String, envKey: String): String? =
        signingProp(envKey) ?: keystoreProps.getProperty(propKey)?.takeIf { it.isNotBlank() }

    val releaseStorePath = signingValue("storeFile", "TECHLANE_KEYSTORE_PATH")
    val releaseStorePassword = signingValue("storePassword", "TECHLANE_KEYSTORE_PASSWORD")
    val releaseKeyAlias = signingValue("keyAlias", "TECHLANE_KEY_ALIAS")
    val releaseKeyPassword = signingValue("keyPassword", "TECHLANE_KEY_PASSWORD")

    val hasReleaseSigning = !releaseStorePath.isNullOrBlank() &&
        !releaseStorePassword.isNullOrBlank() &&
        !releaseKeyAlias.isNullOrBlank() &&
        !releaseKeyPassword.isNullOrBlank()

    signingConfigs {
        if (hasReleaseSigning) {
            create("release") {
                storeFile = file(releaseStorePath!!)
                storePassword = releaseStorePassword
                keyAlias = releaseKeyAlias
                keyPassword = releaseKeyPassword
            }
        }
    }

    buildTypes {
        debug {
            applicationIdSuffix = ".debug"
            versionNameSuffix = "-debug"
            buildConfigField(
                "String",
                "API_BASE",
                "\"${apiBaseOverride.ifEmpty { "http://10.0.2.2:8080/api/v1/" }}\"",
            )
        }
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
            buildConfigField(
                "String",
                "API_BASE",
                "\"${apiBaseOverride.ifEmpty { "https://api.techlane.co.ke/api/v1/" }}\"",
            )
            // Falls back to the debug key only when no production credentials are
            // configured, so `./gradlew assembleRelease` still works for local/CI
            // smoke builds without secrets. A real release build (see RELEASE.md)
            // always has these set, and CI verifies the output is release-signed
            // before it is ever attached to a GitHub Release.
            signingConfig = if (hasReleaseSigning) {
                signingConfigs.getByName("release")
            } else {
                logger.warn(
                    "TechLane POS: no release signing credentials found — " +
                        "signing this release build with the DEBUG key. " +
                        "See apps/android/RELEASE.md before distributing it.",
                )
                signingConfigs.getByName("debug")
            }
        }
    }

    buildFeatures {
        compose = true
        buildConfig = true
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions {
        jvmTarget = "17"
        freeCompilerArgs += listOf("-opt-in=kotlin.RequiresOptIn")
    }

    packaging {
        resources.excludes += setOf(
            "/META-INF/{AL2.0,LGPL2.1}",
            "/META-INF/DEPENDENCIES",
        )
    }

}

// "pos-release.apk" tells a technician nothing, and renaming the Gradle output
// in place (via the Variant API) is version-sensitive enough across AGP
// releases that it isn't worth fighting — a small Copy task after assemble is
// simpler and cannot break the actual build. Lands in outputs/apk/named/,
// which is what the release workflow and RELEASE.md both point at.
listOf("debug", "release").forEach { variantName ->
    val capitalised = variantName.replaceFirstChar { it.uppercase() }
    tasks.register<Copy>("renameApk$capitalised") {
        dependsOn("assemble$capitalised")
        from(layout.buildDirectory.dir("outputs/apk/$variantName"))
        include("*.apk")
        into(layout.buildDirectory.dir("outputs/apk/named"))
        rename { "techlane-pos-v${android.defaultConfig.versionName}-$variantName.apk" }
    }
}

dependencies {
    val composeBom = platform("androidx.compose:compose-bom:2024.10.01")
    implementation(composeBom)
    androidTestImplementation(composeBom)

    // Compose + Material 3
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.ui:ui-graphics")
    implementation("androidx.compose.ui:ui-tooling-preview")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.material3:material3-window-size-class")
    implementation("androidx.compose.material:material-icons-extended")
    implementation("androidx.compose.animation:animation")
    debugImplementation("androidx.compose.ui:ui-tooling")

    // App + lifecycle
    implementation("androidx.core:core-ktx:1.15.0")
    implementation("androidx.core:core-splashscreen:1.0.1")
    implementation("androidx.activity:activity-compose:1.9.3")
    implementation("androidx.lifecycle:lifecycle-runtime-ktx:2.8.7")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.8.7")
    implementation("androidx.lifecycle:lifecycle-viewmodel-compose:2.8.7")
    implementation("androidx.navigation:navigation-compose:2.8.3")

    // DI
    implementation("com.google.dagger:hilt-android:2.52")
    ksp("com.google.dagger:hilt-compiler:2.52")
    implementation("androidx.hilt:hilt-navigation-compose:1.2.0")
    implementation("androidx.hilt:hilt-work:1.2.0")
    ksp("androidx.hilt:hilt-compiler:1.2.0")

    // Networking
    implementation("com.squareup.retrofit2:retrofit:2.11.0")
    implementation("com.squareup.retrofit2:converter-scalars:2.11.0")
    implementation("com.jakewharton.retrofit:retrofit2-kotlinx-serialization-converter:1.0.0")
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
    implementation("com.squareup.okhttp3:logging-interceptor:4.12.0")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.7.3")

    // Offline storage
    implementation("androidx.room:room-runtime:2.6.1")
    implementation("androidx.room:room-ktx:2.6.1")
    ksp("androidx.room:room-compiler:2.6.1")
    implementation("androidx.datastore:datastore-preferences:1.1.1")
    implementation("androidx.security:security-crypto:1.1.0-alpha06")

    // Background sync
    implementation("androidx.work:work-runtime-ktx:2.9.1")

    // Auth
    implementation("androidx.biometric:biometric:1.1.0")

    // Capture + scanning
    implementation("androidx.camera:camera-core:1.4.1")
    implementation("androidx.camera:camera-camera2:1.4.1")
    implementation("androidx.camera:camera-lifecycle:1.4.1")
    implementation("androidx.camera:camera-view:1.4.1")
    implementation("com.google.mlkit:barcode-scanning:17.3.0")

    // Images
    implementation("io.coil-kt:coil-compose:2.7.0")

    // Push (inert until google-services.json is present)
    implementation(platform("com.google.firebase:firebase-bom:33.7.0"))
    implementation("com.google.firebase:firebase-messaging-ktx")

    testImplementation("junit:junit:4.13.2")
    testImplementation("org.jetbrains.kotlinx:kotlinx-coroutines-test:1.9.0")
    androidTestImplementation("androidx.compose.ui:ui-test-junit4")
    debugImplementation("androidx.compose.ui:ui-test-manifest")
}
