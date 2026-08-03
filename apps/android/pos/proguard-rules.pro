# kotlinx.serialization keeps generated serializers reachable through reflection.
-keepattributes *Annotation*, InnerClasses
-dontnote kotlinx.serialization.**
-keepclassmembers class com.techlane.pos.**$$serializer { *; }
-keepclasseswithmembers class com.techlane.pos.** {
    kotlinx.serialization.KSerializer serializer(...);
}
-keep,includedescriptorclasses class com.techlane.pos.data.remote.dto.** { *; }

# JobAction is a @Serializable sealed interface (the offline outbox payload) —
# polymorphic serialization looks subclasses up by name, so both the sealed
# type and every implementation must survive with their serializer intact.
-keep,includedescriptorclasses class com.techlane.pos.data.repository.JobAction { *; }
-keep,includedescriptorclasses class com.techlane.pos.data.repository.JobAction$* { *; }
-keep,includedescriptorclasses class com.techlane.pos.domain.model.** { *; }

# Retrofit / OkHttp
-dontwarn okhttp3.**
-dontwarn okio.**
-dontwarn retrofit2.**
-keepattributes Signature, Exceptions
-keep,allowobfuscation,allowshrinking interface retrofit2.Call
-keep,allowobfuscation,allowshrinking class retrofit2.Response
-keep,allowobfuscation,allowshrinking class kotlin.coroutines.Continuation

# Room — entities and DAOs are reached by generated code the shrinker cannot
# trace back to; AndroidX ships consumer rules for the framework itself, but
# our own tables and query result classes still need keeping.
-keep class com.techlane.pos.data.local.** { *; }

# Hilt / Dagger generated components and entry points.
-keep class dagger.hilt.internal.** { *; }
-keep class * extends dagger.hilt.android.internal.managers.ViewComponentManager$FragmentContextWrapper
-keep,allowobfuscation,allowshrinking class * extends androidx.lifecycle.ViewModel
-keep @dagger.hilt.android.lifecycle.HiltViewModel class * { *; }

# WorkManager workers are instantiated by class name via HiltWorkerFactory.
-keep class * extends androidx.work.ListenableWorker {
    public <init>(android.content.Context, androidx.work.WorkerParameters);
}
