import java.util.Base64

plugins {
    id("com.android.application")
    id("kotlin-android")
    // The Flutter Gradle Plugin must be applied after the Android and Kotlin Gradle plugins.
    id("dev.flutter.flutter-gradle-plugin")
}

// Decode --dart-define values that Flutter passes as base64-encoded, comma-separated
// entries in the "dart-defines" Gradle property.
fun dartDefines(): Map<String, String> {
    val raw = (project.findProperty("dart-defines") as? String) ?: return emptyMap()
    if (raw.isBlank()) return emptyMap()
    return raw.split(",").associate { entry ->
        val decoded = String(Base64.getDecoder().decode(entry), Charsets.UTF_8)
        val eq = decoded.indexOf('=')
        if (eq >= 0) decoded.substring(0, eq) to decoded.substring(eq + 1)
        else decoded to ""
    }
}

android {
    namespace = "com.example.realestate_app"
    compileSdk = flutter.compileSdkVersion
    ndkVersion = flutter.ndkVersion

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions {
        jvmTarget = "17"
    }

    defaultConfig {
        applicationId = "com.example.realestate_app"
        minSdk = flutter.minSdkVersion
        targetSdk = flutter.targetSdkVersion
        versionCode = flutter.versionCode
        versionName = flutter.versionName

        // Expose the Maps API key from --dart-define to AndroidManifest.xml.
        manifestPlaceholders["GOOGLE_MAPS_API_KEY"] =
            dartDefines()["GOOGLE_MAPS_API_KEY"] ?: ""
    }

    buildTypes {
        release {
            signingConfig = signingConfigs.getByName("debug")
        }
    }
}

flutter {
    source = "../.."
}
