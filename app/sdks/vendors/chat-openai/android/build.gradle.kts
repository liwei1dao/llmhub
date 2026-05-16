// Top-level Gradle file — pins the toolchain versions every module
// uses. Individual modules apply the matching plugin without an
// `apply false` boilerplate per module thanks to `pluginManagement`.
plugins {
    id("com.android.library")            version "8.5.2" apply false
    id("org.jetbrains.kotlin.android")    version "2.0.20" apply false
    id("org.jetbrains.kotlin.plugin.serialization") version "2.0.20" apply false
}
