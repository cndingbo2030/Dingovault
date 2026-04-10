import java.util.Properties

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

val dvProps = Properties().apply {
    rootProject.file("gradle.properties").reader().use { load(it) }
}
val dvVersionName = dvProps.getProperty("dvVersion", "1.4.0")

android {
    namespace = "com.dingovault.mobile"
    compileSdk = 34

    defaultConfig {
        applicationId = "com.dingovault.mobile"
        minSdk = 24
        targetSdk = 34
        versionCode = 10400
        versionName = dvVersionName
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            signingConfig = signingConfigs.getByName("debug")
        }
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
    implementation("androidx.appcompat:appcompat:1.6.1")
    implementation("androidx.core:core-ktx:1.12.0")
    implementation(files("libs/dingovault-mobile.aar"))
}
