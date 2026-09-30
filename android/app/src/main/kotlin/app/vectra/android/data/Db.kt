package app.vectra.android.data

import android.content.Context
import androidx.room.Dao
import androidx.room.Database
import androidx.room.Entity
import androidx.room.PrimaryKey
import androidx.room.Query
import androidx.room.Room
import androidx.room.RoomDatabase
import androidx.room.Upsert
import app.vectra.core.outbox.OutboxEntry
import app.vectra.core.outbox.OutboxStatus
import app.vectra.core.outbox.OutboxStore
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map

// Lokaler Speicher nach ADR-021: Outbox-Tabelle und ein einfacher Lesecache (JSON je Schlüssel).

@Entity(tableName = "outbox")
data class OutboxRow(
    @PrimaryKey val id: String,
    val operation: String,
    val vehicleId: String,
    val method: String,
    val path: String,
    val body: String,
    val summary: String,
    val capturedAt: Long,
    val baseVersion: String?,
    val idempotencyKey: String?,
    val status: String,
    val attempts: Int,
    val problem: String?,
)

@Entity(tableName = "cache")
data class CacheRow(@PrimaryKey val key: String, val json: String, val updatedAt: Long)

@Dao
interface OutboxDao {
    @Query("SELECT * FROM outbox ORDER BY capturedAt")
    suspend fun all(): List<OutboxRow>

    @Query("SELECT * FROM outbox ORDER BY capturedAt")
    fun observe(): Flow<List<OutboxRow>>

    @Query("SELECT * FROM outbox WHERE id = :id")
    suspend fun get(id: String): OutboxRow?

    @Upsert
    suspend fun upsert(row: OutboxRow)

    @Query("DELETE FROM outbox WHERE id = :id")
    suspend fun delete(id: String)

    @Query("DELETE FROM outbox")
    suspend fun clear()
}

@Dao
interface CacheDao {
    @Query("SELECT * FROM cache WHERE `key` = :key")
    suspend fun get(key: String): CacheRow?

    @Upsert
    suspend fun put(row: CacheRow)

    @Query("DELETE FROM cache")
    suspend fun clear()
}

@Database(entities = [OutboxRow::class, CacheRow::class], version = 1, exportSchema = true)
abstract class VectraDb : RoomDatabase() {
    abstract fun outbox(): OutboxDao
    abstract fun cache(): CacheDao

    companion object {
        fun create(context: Context): VectraDb = Room.databaseBuilder(context, VectraDb::class.java, "vectra.db").build()
    }
}

fun OutboxRow.toEntry() = OutboxEntry(
    id = id, operation = operation, vehicleId = vehicleId, method = method, path = path, body = body, summary = summary,
    capturedAt = capturedAt, baseVersion = baseVersion, idempotencyKey = idempotencyKey,
    status = OutboxStatus.valueOf(status), attempts = attempts, problem = problem,
)

fun OutboxEntry.toRow() = OutboxRow(
    id = id, operation = operation, vehicleId = vehicleId, method = method, path = path, body = body, summary = summary,
    capturedAt = capturedAt, baseVersion = baseVersion, idempotencyKey = idempotencyKey,
    status = status.name, attempts = attempts, problem = problem,
)

class RoomOutboxStore(private val dao: OutboxDao) : OutboxStore {
    override suspend fun all() = dao.all().map { it.toEntry() }
    override suspend fun get(id: String) = dao.get(id)?.toEntry()
    override suspend fun upsert(entry: OutboxEntry) = dao.upsert(entry.toRow())
    override suspend fun remove(id: String) = dao.delete(id)
    fun observe(): Flow<List<OutboxEntry>> = dao.observe().map { rows -> rows.map { it.toEntry() } }
}
