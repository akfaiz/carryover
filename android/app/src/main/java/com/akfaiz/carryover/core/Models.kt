package com.akfaiz.carryover.core

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json

/**
 * The JSON the Go core returns (see mobile/ and services/). Go encodes nil
 * maps and slices as null, so every field has a default and [json] coerces
 * nulls to it; fields the app doesn't use are ignored.
 */
val json = Json {
    ignoreUnknownKeys = true
    coerceInputValues = true
    encodeDefaults = true
}

/**
 * What a sync does (or would do), as the core's syncer.Action names it. A value
 * this app doesn't know reads as [None] ([json] coerces it), so a newer core
 * can't crash an older app.
 */
@Serializable
enum class SyncAction {
    @SerialName("none") None,
    @SerialName("push") Push,
    @SerialName("pull") Pull,
    @SerialName("conflict") Conflict,
}

/** The side whose saves win a conflict: what Core.sync takes as a resolution. */
enum class Resolution(val wire: String) {
    Local("local"),
    Cloud("cloud"),
}

/** What the core reports about a session that ended (core.NoticeKind). */
@Serializable
enum class NoticeKind {
    Unknown,
    @SerialName("uploaded") Uploaded,
    @SerialName("conflict") Conflict,
    @SerialName("failed") Failed,
    @SerialName("offline") Offline,
}

/** What a [Prompt] asks (services.Prompt.Kind). */
@Serializable
enum class PromptKind {
    Unknown,
    @SerialName("conflict") Conflict,
    @SerialName("lock") Lock,
    @SerialName("syncError") SyncError,
    @SerialName("emulator") Emulator,
}

/** The answers to a [Prompt] ("emulator" is answered with a choice's id instead); see mobile.Respond. */
enum class Answer(val wire: String) {
    Local("local"),
    Cloud("cloud"),
    Skip("skip"),
    Yes("yes"),
    No("no"),
    Cancel("cancel"),
}

/** The emulator id [Core.beginSession] takes for the game's own saves (no emulator). */
const val NATIVE_PROFILE = "native"

@Serializable
data class Settings(
    val deviceId: String = "",
    val deviceName: String = "",
    val remote: String = "",
    val remoteRoot: String = "",
    /** The game library (games, emulators and their settings) is shared with the other devices through the cloud. */
    val syncLibrary: Boolean = false,
    /** Covers and artwork stay on this device: neither uploaded nor downloaded. */
    val coversLocalOnly: Boolean = false,
    val backupRetention: Int = 0,
    /** "" (follow the system), "light" or "dark", shared with the desktop app's setting. */
    val theme: String = "",
    /** The user's own SteamGridDB API key (optional): needed to find covers and backgrounds. */
    val steamGridDbKey: String = "",
    /** The releases updates come from: "" (finished ones only) or "prerelease". */
    val updateChannel: String = "",
    /** Carryover doesn't look for a newer version by itself. */
    val noUpdateCheck: Boolean = false,
)

/** Where updating the app stands (services.UpdateStatus). A value this app doesn't know reads as [Idle]. */
@Serializable
enum class UpdateStatus {
    @SerialName("idle") Idle,
    @SerialName("checking") Checking,
    @SerialName("current") Current,
    @SerialName("available") Available,
    @SerialName("downloading") Downloading,
    @SerialName("ready") Ready,
    @SerialName("installed") Installed,
}

/** What there is to know about updating the app (services.UpdateState). */
@Serializable
data class UpdateState(
    val status: UpdateStatus = UpdateStatus.Idle,
    /** False for a development build, which never updates. */
    val supported: Boolean = false,
    val current: String = "",
    /** The newer version, when there is one, with its notes and release page. */
    val latest: String = "",
    val prerelease: Boolean = false,
    val notes: String = "",
    val url: String = "",
    /** The release has an APK for this device. */
    val installable: Boolean = false,
    /** Bytes downloaded. */
    val done: Long = 0,
    val total: Long = 0,
    /** The downloaded and verified APK while [status] is [UpdateStatus.Ready]. */
    val file: String = "",
    /** Why the last check or download failed. */
    val error: String = "",
) {
    /** There is a newer version to tell the user about. */
    val newer: Boolean
        get() = status == UpdateStatus.Available || status == UpdateStatus.Downloading || status == UpdateStatus.Ready
}

/** What importing an rclone.conf did (the core's cloud.ImportResult). */
@Serializable
data class ImportResult(
    /** The accounts copied. */
    val imported: List<String> = emptyList(),
    /** "name (type)" of the accounts left out: a kind of cloud this app doesn't support. */
    val skipped: List<String> = emptyList(),
    /** Imported accounts that were there already: their old settings are gone. */
    val replaced: List<String> = emptyList(),
)

@Serializable
data class Remote(
    val name: String = "",
    val type: String = "",
    val supported: Boolean = true,
    /** An account whose sign-in never finished (no token): it can't be used, only deleted. */
    val incomplete: Boolean = false,
)

@Serializable
data class OptionExample(val value: String = "", val help: String = "")

@Serializable
data class ProviderOption(
    val name: String = "",
    val help: String = "",
    val default: String = "",
    val required: Boolean = false,
    val isPassword: Boolean = false,
    val exclusive: Boolean = false,
    val examples: List<OptionExample> = emptyList(),
)

@Serializable
data class Provider(
    val type: String = "",
    val description: String = "",
    val oauth: Boolean = false,
    val options: List<ProviderOption> = emptyList(),
)

/** A step of rclone's remote configuration that needs an answer. */
@Serializable
data class Question(
    val done: Boolean = false,
    val state: String = "",
    val name: String = "",
    val help: String = "",
    val default: String = "",
    val required: Boolean = false,
    val isPassword: Boolean = false,
    val exclusive: Boolean = false,
    val isBool: Boolean = false,
    val examples: List<OptionExample> = emptyList(),
    val error: String = "",
)

@Serializable
data class SaveSlot(
    val key: String = "",
    val label: String = "",
    val include: List<String> = emptyList(),
    val exclude: List<String> = emptyList(),
) {
    val title: String get() = label.ifBlank { key }
}

@Serializable
data class GameStatus(
    val lastSyncAt: String = "",
    val lastAction: String = "",
    val conflict: Boolean = false,
    val lastError: String = "",
    val running: Boolean = false,
    val localChanges: Boolean = false,
)

@Serializable
data class DeviceConfig(
    val deviceName: String = "",
    val paths: Map<String, String> = emptyMap(),
    /** On Android: the package of the game's own app (empty: none). */
    val exe: String = "",
)

@Serializable
data class DevicePaths(val deviceName: String = "", val paths: Map<String, String> = emptyMap())

@Serializable
data class ProfileView(
    val emulatorId: String = "",
    val name: String = "",
    val key: String = "",
    val shareWith: String = "",
    val slots: List<SaveSlot> = emptyList(),
    val setUp: Boolean = false,
    val installed: Boolean = false,
    val rom: String = "",
    val paths: Map<String, String> = emptyMap(),
    val resolvedPaths: Map<String, String> = emptyMap(),
    val pathErrors: Map<String, String> = emptyMap(),
    val status: GameStatus = GameStatus(),
)

@Serializable
data class GameView(
    val id: String = "",
    val name: String = "",
    val slots: List<SaveSlot> = emptyList(),
    val device: DeviceConfig = DeviceConfig(),
    /** This device has an entry for the game (even an empty one): it was set up or recovered here. */
    val hasDevice: Boolean = false,
    val resolvedPaths: Map<String, String> = emptyMap(),
    val pathErrors: Map<String, String> = emptyMap(),
    val otherDevices: List<DevicePaths> = emptyList(),
    val status: GameStatus = GameStatus(),
    val emulators: List<ProfileView> = emptyList(),
    val overall: GameStatus = GameStatus(),
    /** The cover image (portrait), "" when the game has none. */
    val coverPath: String = "",
    /** Changes whenever the cover file does (an image cache key). */
    val coverVersion: Long = 0,
    /** Extra artwork by kind: hero, wide, logo, icon. */
    val art: Map<String, ArtFile> = emptyMap(),
) {
    /**
     * This game as the Android app shows it, or null when nothing of it can be
     * played here: the profiles of emulators that exist on Android (ids in
     * [emulatorsHere]), and its own saves only once this device has a save
     * folder or an Android app for them, or knows the game ([hasDevice]: one
     * recovered from the cloud, still to be set up). A PC game's own saves
     * don't sync with Android, but a game added here or with an Android
     * version does.
     */
    fun forAndroid(emulatorsHere: Set<String>): GameView? {
        val own = slots.isNotEmpty() && (hasDevice || device.paths.values.any { it.isNotBlank() } || device.exe.isNotBlank())
        val emus = emulators.filter { it.emulatorId in emulatorsHere }
        if (!own && emus.isEmpty()) return null
        return copy(slots = if (own) slots else emptyList(), emulators = emus)
    }

    /**
     * The ways this game's saves are kept: its own (when it has save slots)
     * and each emulator. Phase 1 only syncs, so a profile counts as usable
     * here once its save paths are set, whatever the launch setup says.
     */
    val profiles: List<Profile>
        get() = buildList {
            if (slots.isNotEmpty()) {
                add(Profile(id, null, null, slots, device.paths, resolvedPaths, pathErrors, status, app = device.exe))
            }
            for (e in emulators) {
                add(
                    Profile(
                        e.key, e.name, e.emulatorId, e.slots, e.paths, e.resolvedPaths, e.pathErrors, e.status,
                        rom = e.rom, shareWith = e.shareWith,
                    ),
                )
            }
        }
}

@Serializable
data class ArtFile(val path: String = "", val version: Long = 0)

/** One profile of a game: the game's own saves (emulator == null) or an emulator's. */
data class Profile(
    val key: String,
    val emulator: String?,
    val emulatorId: String?,
    val slots: List<SaveSlot>,
    val paths: Map<String, String>,
    val resolvedPaths: Map<String, String>,
    val pathErrors: Map<String, String>,
    val status: GameStatus,
    /** The game file an emulator profile starts with ("" = open the emulator). */
    val rom: String = "",
    /** The package of the game's own Android app (native profile). */
    val app: String = "",
    /** The emulator whose saves (and save locations) this one uses, "" for its own. */
    val shareWith: String = "",
) {
    val canSync: Boolean
        get() = slots.isNotEmpty() && pathErrors.isEmpty() && slots.all { resolvedPaths.containsKey(it.key) }

    /**
     * Ready to play here, as the desktop's ProfileView.Ready: its save folders
     * are set and it has an app to start (the game's own, or its emulator's).
     */
    fun ready(emulators: List<EmulatorView>): Boolean = canSync && if (emulatorId == null) {
        app.isNotBlank()
    } else {
        emulators.any { it.id == emulatorId && it.device.exe.isNotBlank() }
    }
}

/** Ready to play here (the desktop's GameView.Ready): one way of playing it is. */
fun GameView.ready(emulators: List<EmulatorView>): Boolean = profiles.any { it.ready(emulators) }

/**
 * The desktop's GameView.NeedsAttention: not ready to play here yet, or saves
 * with a conflict, an error or changes not uploaded; never while it is played.
 */
fun GameView.needsAttention(emulators: List<EmulatorView>): Boolean {
    if (profiles.any { it.status.running }) return false
    if (!ready(emulators)) return true
    return profiles.any { it.canSync && (it.status.conflict || it.status.lastError.isNotEmpty() || it.status.localChanges) }
}

@Serializable
data class Summary(
    val files: Int = 0,
    val bytes: Long = 0,
    val newest: String = "",
    val deviceName: String = "",
)

/**
 * Why a backup was made, as the core's syncer.Backup.Kind names it: [Local] the saves of a device, before a download
 * replaced them; [Cloud] the saves in the cloud, before an upload replaced them; [Manual] on request ("Back up now");
 * [Safety] a copy a device keeps for itself. A kind this app doesn't know reads as [Cloud] ([json] coerces it).
 */
@Serializable
enum class BackupKind {
    @SerialName("cloud") Cloud,
    @SerialName("local") Local,
    @SerialName("manual") Manual,
    @SerialName("safety") Safety,
}

/** One backup of a game's saves (the core's syncer.Backup). */
@Serializable
data class Backup(
    val name: String = "",
    /** RFC 3339. */
    val time: String = "",
    val deviceName: String = "",
    val kind: BackupKind = BackupKind.Cloud,
    /** A pinned backup is never deleted automatically. */
    val pinned: Boolean = false,
    /** A copy kept only on the device that made it: it can't be pinned, and nothing else sees it. */
    val local: Boolean = false,
)

/**
 * Which of a game's save locations a suggestion is, for emulators whose folder differs per device (Switch): the slot
 * of that name is on every device, and this device only sets its own folder. [None] for any other suggestion. A kind
 * this app doesn't know reads as [None] ([json] coerces it).
 */
@Serializable
enum class SaveKind {
    @SerialName("") None,
    @SerialName("account") Account,
    @SerialName("device") Device,
}

/** One possible save location of a game in an emulator (the core's services.EmuSuggestion). */
@Serializable
data class SaveSuggestion(
    /** The manifest's title for the game. */
    val game: String = "",
    /** The folder; may be a folder granted through the Storage Access Framework. */
    val path: String = "",
    /** Filters for what is inside it (empty = everything). */
    val include: List<String> = emptyList(),
    /** There are files at this location on this device now. */
    val exists: Boolean = false,
    val kind: SaveKind = SaveKind.None,
)

/** What a lookup of a game's save locations found (the core's services.EmuSuggestResult). */
@Serializable
data class SaveSuggestions(
    val suggestions: List<SaveSuggestion> = emptyList(),
    /** False when the locations hold every game's saves: they are only offered, never filled in by themselves. */
    val perGame: Boolean = false,
    /** Something to know about the result. */
    val note: String = "",
)

@Serializable
data class AutofillResult(val applied: Int = 0)

@Serializable
data class PlanView(
    val action: SyncAction = SyncAction.None,
    val local: Summary = Summary(),
    val remote: Summary = Summary(),
    val hasRemote: Boolean = false,
)

@Serializable
data class SyncOutcome(val action: SyncAction = SyncAction.None, val offline: Boolean = false, val cancelled: Boolean = false)

@Serializable
data class SyncResult(val gameId: String = "", val name: String = "", val action: SyncAction = SyncAction.None, val error: String = "")

@Serializable
data class SyncAllOutcome(
    val results: List<SyncResult> = emptyList(),
    val offline: Boolean = false,
    val cancelled: Boolean = false,
)

@Serializable
data class SyncEvent(
    val gameId: String = "",
    val phase: String = "",
    val message: String = "",
    val bytes: Long = 0,
    val totalBytes: Long = 0,
)

@Serializable
data class SyncAllProgress(
    val done: Int = 0,
    val total: Int = 0,
    val name: String = "",
    /** The sync key of the game syncing now: its own SyncEvents carry it as gameId. */
    val key: String = "",
)

@Serializable
data class NewGame(val name: String, val slots: List<SaveSlot>, val device: NewDevice)

@Serializable
data class NewDevice(val paths: Map<String, String>)

/** This device's setup of an emulator. On Android, exe is the app's package
 *  ("package" or "package/activity") and args the optional template that
 *  starts a game directly (am start syntax, {rom}). */
@Serializable
data class EmulatorDevice(
    val exe: String = "",
    val args: String = "",
    val dataDir: String = "",
)

@Serializable
data class EmulatorView(
    val id: String = "",
    val name: String = "",
    val preset: String = "",
    val device: EmulatorDevice = EmulatorDevice(),
    val setUp: Boolean = false,
    val games: Int = 0,
)

@Serializable
data class EmulatorSetup(
    val id: String,
    val name: String,
    val exe: String,
    val args: String,
    val preset: String,
    val dataDir: String,
    /** The platforms it has a version for (see [EmulatorPreset.platforms]); null keeps the ones it has. */
    val platforms: List<String>? = null,
)

@Serializable
data class BeginOutcome(
    val key: String = "",
    val cloudKey: String = "",
    val emulator: String = "",
    val preAction: String = "",
    val preOffline: Boolean = false,
    val synced: Boolean = false,
    val cancelled: Boolean = false,
)

@Serializable
data class Notice(
    val key: String = "",
    val title: String = "",
    val kind: NoticeKind = NoticeKind.Unknown,
    val err: String = "",
)

@Serializable
data class EndOutcome(val notice: Notice? = null, val offline: Boolean = false)

@Serializable
data class OpenSession(
    val cloudKey: String = "",
    val key: String = "",
    val game: String = "",
    val emulator: String = "",
)

@Serializable
data class ConflictInfo(val local: Summary = Summary(), val remote: Summary = Summary())

@Serializable
data class Lock(val deviceName: String = "", val since: String = "")

@Serializable
data class ProfileChoice(val id: String = "", val name: String = "")

/** A question from a play session; answer with Core.respond. */
@Serializable
data class Prompt(
    val id: String = "",
    val kind: PromptKind = PromptKind.Unknown,
    val gameId: String = "",
    val gameName: String = "",
    val beforePlay: Boolean = false,
    val conflict: ConflictInfo? = null,
    val lock: Lock? = null,
    val error: String = "",
    val choices: List<ProfileChoice> = emptyList(),
    val default: String = "",
)

/** Progress of a play session. */
@Serializable
data class SessionStatus(
    val gameId: String = "",
    val key: String = "",
    /** sync, playing, done, error, offline, choose */
    val phase: String = "",
    val message: String = "",
    val running: Boolean = false,
)

/** A deleted game that can still be restored (services.TrashGame). Times are RFC 3339. */
@Serializable
data class TrashGame(
    val id: String = "",
    val name: String = "",
    val deletedAt: String = "",
    /** When its settings are removed for good. */
    val expiresAt: String = "",
    /** How many emulators it has. */
    val emulators: Int = 0,
)

/** A deleted emulator that can still be restored (services.TrashEmulator). */
@Serializable
data class TrashEmulator(
    val id: String = "",
    val name: String = "",
    val deletedAt: String = "",
    val expiresAt: String = "",
)

/**
 * A device the library holds settings of (services.DeviceView). Reinstalling the app or clearing its data makes a
 * device start over under a new id; its earlier entry ([sameName]) can be taken back with reuseDevice.
 */
@Serializable
data class DeviceView(
    val id: String = "",
    /** "" when unknown. */
    val name: String = "",
    /** The GOOS its Carryover runs on: windows, linux, darwin or android ("" if unknown). */
    val platform: String = "",
    /** RFC 3339; when its settings last changed. */
    val lastSeen: String = "",
    val games: Int = 0,
    val emulators: Int = 0,
    /** This device. */
    @SerialName("this") val isThis: Boolean = false,
    /** Another device with this device's name: likely this one before a reinstall. */
    val sameName: Boolean = false,
)

/** What was deleted and can still be restored (mobile.TrashView). */
@Serializable
data class TrashView(
    /** How long a deleted game or emulator stays restorable. */
    val days: Int = 30,
    val games: List<TrashGame> = emptyList(),
    val emulators: List<TrashEmulator> = emptyList(),
)

/** A cloud save folder no game uses (services.LeftoverView). */
@Serializable
data class Leftover(
    val key: String = "",
    val gameId: String = "",
    val gameName: String = "",
    val emulatorId: String = "",
    val emulatorName: String = "",
    val slots: List<String> = emptyList(),
    val files: Int = 0,
    val bytes: Long = 0,
    val time: String = "",
    val backups: Int = 0,
) {
    /** What the user calls it: the game, with its emulator when the saves are an emulator's. */
    val title: String get() = if (emulatorName.isEmpty()) gameName else "$gameName ($emulatorName)"
}

/** The kinds of artwork the app picks from SteamGridDB. */
enum class ArtKind(val wire: String) {
    /** The portrait capsule, a game's cover. */
    Cover("cover"),

    /** The wide image behind a game's page (SteamGridDB calls it a hero). */
    Background("hero"),
}

/** A game on SteamGridDB (steamgrid.Game). */
@Serializable
data class GridGame(
    val id: Int = 0,
    val name: String = "",
    val year: Int = 0,
    val verified: Boolean = false,
)

/** One image of SteamGridDB: a cover or a background (steamgrid.Grid). */
@Serializable
data class GridArt(
    val id: Int = 0,
    /** The full image, downloaded by the core when it is chosen. */
    val url: String = "",
    /** A small copy to show in the list. */
    val thumb: String = "",
    val width: Int = 0,
    val height: Int = 0,
    val style: String = "",
    val score: Int = 0,
    val author: String = "",
)
