plugins {
    id("com.android.library")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.compose")
}

android {
    namespace = "com.techlane.core"
    compileSdk = 35

    defaultConfig {
        minSdk = 26
        consumerProguardFiles("consumer-rules.pro")
    }

    buildFeatures {
        compose = true
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions {
        jvmTarget = "17"
    }
}

dependencies {
    val composeBom = platform("androidx.compose:compose-bom:2024.10.01")
    api(composeBom)
    api("androidx.compose.ui:ui")
    api("androidx.compose.material3:material3")
    api("androidx.compose.material:material-icons-extended")
    api("androidx.activity:activity-compose:1.9.3")
    api("androidx.lifecycle:lifecycle-runtime-ktx:2.8.7")
    api("androidx.lifecycle:lifecycle-runtime-compose:2.8.7")
    api("androidx.security:security-crypto:1.1.0-alpha06")
    api("com.squareup.okhttp3:okhttp:4.12.0")
    api("androidx.camera:camera-camera2:1.4.1")
    api("androidx.camera:camera-lifecycle:1.4.1")
    api("androidx.camera:camera-view:1.4.1")
    api("androidx.camera:camera-mlkit-vision:1.4.1")
    api("com.google.mlkit:barcode-scanning:17.3.0")
    api("com.google.zxing:core:3.5.3")
}
