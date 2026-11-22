# R8 keep rules for the release build (build.gradle.kts, optimization.keepRules).
# Libraries bring their own rules; this file only adds what they miss.

# WorkManager's database is built by Room with reflection: it needs the generated
# class's no-argument constructor. Room's own rule keeps the class but, in R8 full
# mode, not its default constructor, so the app crashed at start-up with
# NoSuchMethodException: androidx.work.impl.WorkDatabase_Impl.<init>.
-keep class * extends androidx.room.RoomDatabase {
    <init>();
}
