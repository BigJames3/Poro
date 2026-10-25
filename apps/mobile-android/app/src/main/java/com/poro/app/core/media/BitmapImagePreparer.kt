package com.poro.app.core.media

import android.content.Context
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Matrix
import android.media.ExifInterface
import android.net.Uri
import java.io.File
import java.io.FileNotFoundException
import java.io.IOException
import java.util.UUID
import kotlin.math.max
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

/**
 * Préparation des photos avec les décodeurs d'Android, une image à la fois pour borner la
 * mémoire : l'image est décodée sous-échantillonnée (jamais plus de deux fois [LocalImagePreparer.MAX_SIDE]),
 * tournée selon son EXIF, réduite puis encodée. Les fichiers vont dans `files/photo-uploads/`,
 * qui survit à un arrêt du processus pendant l'envoi.
 */
class BitmapImagePreparer(context: Context) : LocalImagePreparer {
    private val resolver = context.applicationContext.contentResolver
    private val root = File(context.applicationContext.filesDir, "photo-uploads")

    override suspend fun prepare(uris: List<String>): PrepareResult = withContext(Dispatchers.IO) {
        val dir = File(root, UUID.randomUUID().toString()).apply { mkdirs() }
        val out = ArrayList<PreparedImage>(uris.size)
        uris.forEachIndexed { index, uri ->
            val position = index + 1
            val file = File(dir, "$position.jpg")
            val reason = try {
                if (prepareOne(Uri.parse(uri), file)) null else PrepareResult.Reason.Unsupported
            } catch (e: SecurityException) {
                PrepareResult.Reason.Unreadable
            } catch (e: FileNotFoundException) {
                PrepareResult.Reason.Unreadable
            } catch (e: IOException) {
                PrepareResult.Reason.Unsupported
            } catch (e: OutOfMemoryError) {
                PrepareResult.Reason.Unsupported
            }
            if (reason != null || file.length() > LocalImagePreparer.MAX_BYTES) {
                dir.deleteRecursively()
                return@withContext PrepareResult.Failed(position, reason ?: PrepareResult.Reason.Unsupported)
            }
            out += PreparedImage(file.absolutePath, file.length())
        }
        PrepareResult.Ready(out)
    }

    /** False si l'appareil ne sait pas décoder l'image. */
    private fun prepareOne(uri: Uri, target: File): Boolean {
        val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
        open(uri).use { BitmapFactory.decodeStream(it, null, bounds) }
        if (bounds.outWidth <= 0 || bounds.outHeight <= 0) return false
        var sample = 1
        while (max(bounds.outWidth, bounds.outHeight) / (sample * 2) >= LocalImagePreparer.MAX_SIDE) sample *= 2
        val decoded = open(uri).use {
            BitmapFactory.decodeStream(it, null, BitmapFactory.Options().apply { inSampleSize = sample })
        } ?: return false
        // transform() rend une nouvelle image, sauf s'il n'y a rien à faire.
        val oriented = transform(decoded, orientation(uri))
        try {
            target.outputStream().use { stream ->
                if (!oriented.compress(Bitmap.CompressFormat.JPEG, LocalImagePreparer.QUALITY, stream)) return false
            }
        } finally {
            if (oriented !== decoded) oriented.recycle()
            decoded.recycle()
        }
        return true
    }

    private fun open(uri: Uri) = resolver.openInputStream(uri) ?: throw FileNotFoundException(uri.toString())

    private fun orientation(uri: Uri): Int = try {
        open(uri).use { ExifInterface(it).getAttributeInt(ExifInterface.TAG_ORIENTATION, ExifInterface.ORIENTATION_NORMAL) }
    } catch (e: IOException) {
        ExifInterface.ORIENTATION_NORMAL // pas d'EXIF lisible : l'image reste telle quelle
    }

    /** Réduction au côté long de MAX_SIDE et orientation EXIF, en une seule passe. */
    private fun transform(source: Bitmap, orientation: Int): Bitmap {
        val matrix = Matrix()
        val longSide = max(source.width, source.height)
        if (longSide > LocalImagePreparer.MAX_SIDE) {
            val scale = LocalImagePreparer.MAX_SIDE.toFloat() / longSide
            matrix.postScale(scale, scale)
        }
        when (orientation) {
            ExifInterface.ORIENTATION_FLIP_HORIZONTAL -> matrix.postScale(-1f, 1f)
            ExifInterface.ORIENTATION_ROTATE_180 -> matrix.postRotate(180f)
            ExifInterface.ORIENTATION_FLIP_VERTICAL -> matrix.postScale(1f, -1f)
            ExifInterface.ORIENTATION_TRANSPOSE -> {
                matrix.postRotate(90f)
                matrix.postScale(-1f, 1f)
            }
            ExifInterface.ORIENTATION_ROTATE_90 -> matrix.postRotate(90f)
            ExifInterface.ORIENTATION_TRANSVERSE -> {
                matrix.postRotate(-90f)
                matrix.postScale(-1f, 1f)
            }
            ExifInterface.ORIENTATION_ROTATE_270 -> matrix.postRotate(-90f)
        }
        if (matrix.isIdentity) return source
        return Bitmap.createBitmap(source, 0, 0, source.width, source.height, matrix, true)
    }

    override fun read(image: PreparedImage): ByteArray = File(image.path).readBytes()

    override fun discard(images: List<PreparedImage>) {
        images.map { File(it.path).parentFile }.distinct().forEach { it?.deleteRecursively() }
    }
}
