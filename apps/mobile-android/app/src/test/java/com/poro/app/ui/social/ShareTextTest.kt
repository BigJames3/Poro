package com.poro.app.ui.social

import org.junit.Assert.assertEquals
import org.junit.Test

class ShareTextTest {
    @Test
    fun `no link until a public base URL is configured`() {
        assertEquals("« Match du dimanche » sur Poro", shareText("Match du dimanche", "v1", ""))
        assertEquals("« Match » sur Poro\nhttps://poro.example/v/v1", shareText("Match", "v1", "https://poro.example/"))
    }
}
