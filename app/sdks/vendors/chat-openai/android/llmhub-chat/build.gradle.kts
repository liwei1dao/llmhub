plugins {
    id("com.android.library")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.serialization")
    `maven-publish`
}

group   = providers.gradleProperty("LLMHUB_GROUP").get()
version = providers.gradleProperty("LLMHUB_VERSION").get()

android {
    namespace = "io.llmhub.chat"
    compileSdk = 34

    defaultConfig {
        minSdk = 24
        consumerProguardFiles("consumer-rules.pro")
        ndk {
            // Keep only the ABIs the platform supports. armeabi-v7a is
            // included for older devices; drop it if the host app targets
            // 64-bit-only.
            abiFilters += listOf("arm64-v8a", "armeabi-v7a", "x86_64")
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = true
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro"
            )
        }
        debug {
            isMinifyEnabled = false
        }
    }

    sourceSets["main"].kotlin.srcDir("src/main/kotlin")
    // The native .so files are produced by `scripts/build-android.sh`
    // and copied into src/main/jniLibs/<abi>/ before `assembleRelease`.
    sourceSets["main"].jniLibs.srcDir("src/main/jniLibs")

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions {
        jvmTarget = "17"
    }

    packaging {
        // Strip debug info from .so on packaging — defensive: cargo
        // already strips, but double-stripping costs nothing and saves
        // a few KB if a contributor forgets the release profile.
        jniLibs.useLegacyPackaging = false
    }

    publishing {
        singleVariant("release") {
            withSourcesJar()
        }
    }
}

dependencies {
    implementation("org.jetbrains.kotlin:kotlin-stdlib:2.0.20")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.8.1")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.7.1")
}

publishing {
    publications {
        register<MavenPublication>("release") {
            groupId    = project.group.toString()
            artifactId = "llmhub-chat"
            version    = project.version.toString()
            afterEvaluate { from(components["release"]) }
        }
    }
}

// Convenience task: rebuild the native cdylib for all enabled ABIs
// before packaging, so `./gradlew :llmhub-chat:assembleRelease`
// always ships a fresh .so set. The script is idempotent and skips
// rebuilds when sources haven't changed.
tasks.register<Exec>("buildNativeCore") {
    // rootProject.projectDir = app/sdks/chat-openai/android
    // ../core/scripts         = app/sdks/chat-openai/core/scripts
    workingDir = rootProject.projectDir.resolve("../core").canonicalFile
    commandLine("bash", "scripts/build-android.sh",
        "--out", "${projectDir}/src/main/jniLibs",
        "--profile", "release")
}

tasks.matching { it.name.startsWith("merge") && it.name.contains("JniLibFolders") }.configureEach {
    dependsOn("buildNativeCore")
}
