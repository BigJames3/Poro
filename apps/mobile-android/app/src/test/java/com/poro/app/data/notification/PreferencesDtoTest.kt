package com.poro.app.data.notification

import com.poro.app.core.network.PoroJson
import org.junit.Assert.assertEquals
import org.junit.Test

class PreferencesDtoTest {

    @Test
    fun `every preference is sent so that turning one back on reaches the server`() {
        assertEquals(
            """{"push_enabled":true,"likes":true,"comments":true,"follows":true,"video_ready":true,"orders":false}""",
            PoroJson.encodeToString(PreferencesDto.serializer(), PreferencesDto(orders = false)),
        )
    }
}
