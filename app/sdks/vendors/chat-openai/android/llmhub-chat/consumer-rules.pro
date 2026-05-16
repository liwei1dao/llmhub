# Consumer rules applied to any app that depends on llmhub-chat.
# Everything an app *must* keep at runtime — the JNI entry points and
# the listener interface — is listed here so R8 / ProGuard don't strip
# names the native side looks up by reflection.

-keep,allowobfuscation,allowshrinking class io.llmhub.chat.LLMHubException { *; }

# JNI surface — the native library calls these by JNI name. Removing
# or renaming any of them breaks the .so at runtime.
-keep class io.llmhub.chat.internal.NativeBridge {
    public static native <methods>;
}

# Listener interface: the native streaming path invokes onChunk /
# onComplete / onError via JNI signatures, so the signatures must
# survive obfuscation.
-keep class io.llmhub.chat.StreamListener {
    public boolean onChunk(java.lang.String);
    public void onComplete();
    public void onError(java.lang.String, java.lang.String);
}

# kotlinx.serialization companions — the JSON DTOs cross JNI as
# strings; the (de)serialisers must keep their reflective hooks.
-keepclassmembers @kotlinx.serialization.Serializable class io.llmhub.chat.** {
    *** Companion;
    *** serializer(...);
}
-keepattributes RuntimeVisibleAnnotations,AnnotationDefault,InnerClasses
