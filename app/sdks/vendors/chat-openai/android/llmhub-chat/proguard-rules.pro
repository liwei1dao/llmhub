# Library-side R8/ProGuard rules. The consumer app gets the
# corresponding "consumer-rules.pro" automatically — keep these in
# sync (or rather: keep the *consumer* file the authoritative one
# and use this file only for rules that protect the library's own
# release build).

# kotlinx.serialization: keep generated $$serializer companions
-keepattributes RuntimeVisibleAnnotations,AnnotationDefault
-keepclassmembers class io.llmhub.chat.** {
    *** Companion;
}
-keepclasseswithmembers class io.llmhub.chat.** {
    kotlinx.serialization.KSerializer serializer(...);
}

# Kotlin/JDK 17 emits java.lang.invoke.StringConcatFactory.makeConcatWithConstants
# for data-class toString() bodies. AGP desugars it on devices that lack it
# (minSdk 24 < API 26), but R8 still warns about the symbolic reference —
# the warning is noise, not an actual missing class at runtime.
-dontwarn java.lang.invoke.StringConcatFactory
