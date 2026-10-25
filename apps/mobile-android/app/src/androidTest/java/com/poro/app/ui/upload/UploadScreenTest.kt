package com.poro.app.ui.upload

import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import com.poro.app.R
import com.poro.app.core.media.LocalMedia
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.data.video.UploadState
import com.poro.app.ui.theme.PoroTheme
import java.io.IOException
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

/** Onglet Publier : aperçu de la vidéo choisie, et action proposée selon la cause d'un échec. */
@RunWith(AndroidJUnit4::class)
class UploadScreenTest {

    @get:Rule
    val compose = createComposeRule()

    private val context = InstrumentationRegistry.getInstrumentation().targetContext
    private val clicks = mutableListOf<String>()
    private val video = LocalMedia("content://media/picker/0/video/1", "video/mp4", 5_000_000, durationMs = 30_000)

    private fun show(form: UploadFormState, upload: UploadState) {
        compose.setContent {
            PoroTheme {
                UploadScreen(
                    form = form,
                    upload = upload,
                    onVideoPicked = {},
                    onTitleChange = {},
                    onDescriptionChange = {},
                    onPublish = { clicks += "publish" },
                    onRetry = { clicks += "retry" },
                    onCancelUpload = {},
                    onDismissResult = { clicks += "dismiss" },
                    onOpenVideo = {},
                )
            }
        }
    }

    @Test
    fun theChosenVideoIsPreviewedBeforePublishing() {
        show(UploadFormState(file = video, title = "Match"), UploadState.Idle)

        compose.onNodeWithContentDescription(context.getString(R.string.upload_preview)).assertIsDisplayed()
    }

    @Test
    fun aNetworkFailureOffersRetryWithTheKeptForm() {
        show(UploadFormState(file = video, title = "Match"), UploadState.Failed(ApiResult.NetworkError(IOException("offline"))))

        compose.onNodeWithText(context.getString(R.string.action_retry)).performClick()

        assertEquals(listOf("retry"), clicks)
        compose.onNodeWithText("Match").performScrollTo().assertIsDisplayed()
    }

    @Test
    fun aRefusedContentIsNotRetriedAsIs() {
        show(
            UploadFormState(file = video, title = "Match"),
            UploadState.Failed(ApiResult.Failure(422, ApiErrorCodes.INVALID_REQUEST, "title", null, null)),
        )

        compose.onAllNodesWithText(context.getString(R.string.action_retry)).assertCountEquals(0)
        compose.onNodeWithText(context.getString(R.string.upload_edit)).performClick()
        assertEquals(listOf("dismiss"), clicks)
    }

    @Test
    fun aLostVideoAsksToPickItAgain() {
        show(
            UploadFormState(title = "Match", description = "#foot"),
            UploadState.Failed(ApiResult.Failure(0, ApiErrorCodes.MEDIA_UNREADABLE, "unreadable", null, null)),
        )

        compose.onAllNodesWithText(context.getString(R.string.upload_error_unreadable)).assertCountEquals(1)
        compose.onNodeWithText(context.getString(R.string.upload_repick)).assertIsDisplayed()
        compose.onAllNodesWithText(context.getString(R.string.action_retry)).assertCountEquals(0)
    }
}
