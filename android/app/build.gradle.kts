plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.compose)
    alias(libs.plugins.kotlin.serialization)
}

// Set by scripts/android-build.sh / .ps1 (-Pcarryover.…); the defaults are
// for builds from Android Studio.
val carryoverVersionName = providers.gradleProperty("carryover.versionName").orElse("0.0.1-dev")
val carryoverVersionCode = providers.gradleProperty("carryover.versionCode").map { it.toInt() }.orElse(1)
// The ABIs the Go core was built for (scripts/android-bind.sh TARGETS): one APK each.
val carryoverAbis = providers.gradleProperty("carryover.abis")
    .map { it.split(',').map(String::trim).filter(String::isNotEmpty) }
    .orElse(listOf("arm64-v8a", "x86_64"))
// Release signing, from the environment so no key or password is ever in the repository.
val releaseKeystore = providers.environmentVariable("CARRYOVER_KEYSTORE")

android {
    // Every language in every install: the in-app language setting switches without
    // downloading anything (an App Bundle would otherwise split by language).
    bundle {
        language {
            enableSplit = false
        }
    }
    namespace = "com.akfaiz.carryover"
    compileSdk {
        version = release(36)
    }

    defaultConfig {
        applicationId = "com.akfaiz.carryover"
        minSdk = 26
        targetSdk = 36
        versionCode = carryoverVersionCode.get()
        versionName = carryoverVersionName.get()
    }

    signingConfigs {
        if (releaseKeystore.isPresent) {
            create("release") {
                storeFile = file(releaseKeystore.get())
                storePassword = providers.environmentVariable("CARRYOVER_KEYSTORE_PASSWORD").orNull
                keyAlias = providers.environmentVariable("CARRYOVER_KEY_ALIAS").orNull
                // An empty CARRYOVER_KEY_PASSWORD counts as unset: CI passes it empty when that
                // secret doesn't exist, and the key then has the keystore's password.
                keyPassword = providers.environmentVariable("CARRYOVER_KEY_PASSWORD").orNull?.takeIf { it.isNotEmpty() }
                    ?: providers.environmentVariable("CARRYOVER_KEYSTORE_PASSWORD").orNull
            }
        }
    }

    buildTypes {
        release {
            optimization {
                enable = true
                keepRules {
                    files.add(file("proguard-rules.pro"))
                }
            }
            // Without CARRYOVER_KEYSTORE the release APKs are unsigned (not installable).
            signingConfigs.findByName("release")?.let { signingConfig = it }
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    buildFeatures {
        compose = true
    }
    // Compress the Go core in the APK (~42 MB -> ~23 MB per ABI; it is
    // extracted on install). The default stores it uncompressed.
    packaging {
        jniLibs {
            useLegacyPackaging = true
        }
    }
    // One APK per ABI: the Go core is ~30 MB per ABI.
    splits {
        abi {
            isEnable = true
            reset()
            include(*carryoverAbis.get().toTypedArray())
            isUniversalApk = false
        }
    }
}

dependencies {
    // The Go core, built by scripts/android-bind.sh.
    implementation(files("libs/carryover-core.aar"))

    implementation(platform(libs.androidx.compose.bom))
    implementation(libs.androidx.activity.compose)
    implementation(libs.androidx.compose.material3)
    implementation(libs.androidx.compose.material.icons)
    implementation(libs.androidx.compose.material.icons.extended)
    implementation(libs.androidx.compose.ui)
    implementation(libs.androidx.compose.ui.tooling.preview)
    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.lifecycle.runtime.ktx)
    implementation(libs.androidx.lifecycle.viewmodel.compose)
    implementation(libs.androidx.lifecycle.runtime.compose)
    implementation(libs.androidx.work.runtime.ktx)
    implementation(libs.kotlinx.serialization.json)
    testImplementation(libs.junit)
    testImplementation(libs.kotlinx.coroutines.test)
    debugImplementation(libs.androidx.compose.ui.tooling)
}
