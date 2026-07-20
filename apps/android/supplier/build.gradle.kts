plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.compose")
}

android {
    namespace = "com.techlane.supplier"
    compileSdk = 35

    defaultConfig {
        applicationId = "com.techlane.supplier"
        minSdk = 26
        targetSdk = 35
        versionCode = 1
        versionName = "0.1.0"
        buildConfigField("String", "API_BASE", "\"http://10.0.2.2:8080/api/v1\"")
    }

    val apiBaseOverride = (project.findProperty("apiBase") as String?)?.trim().orEmpty()
    buildTypes {
        debug {
            if (apiBaseOverride.isNotEmpty()) {
                buildConfigField("String", "API_BASE", "\"$apiBaseOverride\"")
            }
        }
        release {
            isMinifyEnabled = false
            buildConfigField(
                "String",
                "API_BASE",
                "\"${apiBaseOverride.ifEmpty { "https://api.example.com/api/v1" }}\"",
            )
            signingConfig = signingConfigs.getByName("debug")
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
    }
}

dependencies {
    implementation(project(":mobile-core"))
    val composeBom = platform("androidx.compose:compose-bom:2024.10.01")
    implementation(composeBom)
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.ui:ui-tooling-preview")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.material:material-icons-extended")
    implementation("androidx.activity:activity-compose:1.9.3")
    implementation("androidx.navigation:navigation-compose:2.8.3")
    implementation("androidx.lifecycle:lifecycle-runtime-ktx:2.8.7")
    implementation("androidx.lifecycle:lifecycle-viewmodel-compose:2.8.7")
    debugImplementation("androidx.compose.ui:ui-tooling")
}
