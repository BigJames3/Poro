package com.poro.app

import android.security.NetworkSecurityPolicy
import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

/**
 * Configuration réseau du build de debug, lue par Android lui-même : HTTP en clair vers la pile
 * locale seulement (émulateur et tunnel adb), HTTPS partout ailleurs.
 */
@RunWith(AndroidJUnit4::class)
class NetworkSecurityTest {

    private val policy = NetworkSecurityPolicy.getInstance()

    @Test
    fun cleartextIsAllowedToTheLocalStackOnly() {
        assertTrue("émulateur → poste", policy.isCleartextTrafficPermitted("10.0.2.2"))
        assertTrue("tunnel adb", policy.isCleartextTrafficPermitted("127.0.0.1"))
        assertFalse("tout autre hôte", policy.isCleartextTrafficPermitted("api.poro.africa"))
        assertFalse("pas de sous-domaine", policy.isCleartextTrafficPermitted("x.10.0.2.2"))
    }
}
