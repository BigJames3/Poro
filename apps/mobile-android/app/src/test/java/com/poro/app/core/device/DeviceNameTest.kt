package com.poro.app.core.device

import org.junit.Assert.assertEquals
import org.junit.Test

class DeviceNameTest {
    @Test
    fun `joins manufacturer and model without repeating the brand`() {
        assertEquals("TECNO Spark 20", DeviceName.format("TECNO", "Spark 20"))
        assertEquals("Samsung SM-A155F", DeviceName.format("samsung", "SM-A155F"))
        assertEquals("Google Pixel 8", DeviceName.format("Google", "Pixel 8"))
        assertEquals("Infinix HOT 40", DeviceName.format("Infinix", "Infinix HOT 40"))
    }

    @Test
    fun `falls back and truncates to 60 characters`() {
        assertEquals("Android", DeviceName.format(null, null))
        assertEquals("Nokia", DeviceName.format("nokia", ""))
        assertEquals(60, DeviceName.format("Brand", "M".repeat(100)).length)
    }
}
