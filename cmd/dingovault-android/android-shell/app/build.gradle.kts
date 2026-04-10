import java.util.Properties

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

val dvProps = Properties().apply {
    rootProject.file("gradle.properties").reader().use { load(it) }
}
val dvVersionName = dvProps.getProperty("dvVersion", "1.4.0")

val frontendDistDir = rootProject.projectDir.resolve("../../../frontend/dist")

val syncWebDist by tasks.registering(Copy::class) {
    from(frontendDistDir) {
        include("**/*")
    }
    into(layout.projectDirectory.dir("src/main/assets/dist"))
    duplicatesStrategy = DuplicatesStrategy.INCLUDE
    onlyIf { frontendDistDir.exists() }
}

val patchWebDistIndex by tasks.registering {
    dependsOn(syncWebDist)
    doLast {
        val html = layout.projectDirectory.file("src/main/assets/dist/index.html").asFile
        if (!html.exists()) {
            throw GradleException(
                "Missing frontend/dist with relative asset paths. From repo root run: " +
                    "cd frontend && npm ci && npm run build:android"
            )
        }
        var text = html.readText(Charsets.UTF_8)
        if (text.contains("android-shim.js")) {
            return@doLast
        }
        val needle = "<script type=\"module\""
        if (!text.contains(needle)) {
            throw GradleException("dist/index.html missing Vite module script tag")
        }
        text = text.replaceFirst(
            needle,
            "<script src=\"../android-shim.js\"></script>\n    $needle"
        )
        html.writeText(text, Charsets.UTF_8)
    }
}

tasks.named("preBuild") {
    dependsOn(patchWebDistIndex)
}

android {
    namespace = "com.dingovault.mobile"
    compileSdk = 34

    defaultConfig {
        applicationId = "com.dingovault.mobile"
        minSdk = 24
        targetSdk = 34
        versionCode = 10403
        versionName = dvVersionName
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            signingConfig = signingConfigs.getByName("debug")
        }
    }

    buildFeatures {
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
    implementation("androidx.appcompat:appcompat:1.6.1")
    implementation("androidx.core:core-ktx:1.12.0")
    implementation(files("libs/dingovault-mobile.aar"))
}
