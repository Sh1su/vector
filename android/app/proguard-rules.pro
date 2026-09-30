# kotlinx.serialization: generierte Serializer der DTOs behalten
-keepattributes *Annotation*, InnerClasses
-dontnote kotlinx.serialization.**
-keep,includedescriptorclasses class app.vectra.core.**$$serializer { *; }
-keepclassmembers class app.vectra.core.** { *** Companion; }
-keepclasseswithmembers class app.vectra.core.** { kotlinx.serialization.KSerializer serializer(...); }
# OkHttp
-dontwarn okhttp3.internal.platform.**
-dontwarn org.conscrypt.**
-dontwarn org.bouncycastle.**
-dontwarn org.openjsse.**
