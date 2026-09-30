package app.vectra.core

import app.vectra.core.net.ApiClient
import app.vectra.core.net.Diagnosis
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test
import java.io.IOException
import java.net.UnknownHostException
import java.net.UnknownServiceException
import javax.net.ssl.SSLHandshakeException

class DiagnosisTest {
    @Test
    fun `Ursachen werden erkannt`() {
        assertTrue(Diagnosis.network(UnknownServiceException("CLEARTEXT communication to vector.lan not permitted"), "http://vector.lan").contains("HTTP"))
        assertTrue(Diagnosis.network(UnknownHostException("vector.lan"), "https://vector.lan").contains("vector.lan wurde nicht gefunden"))
        assertTrue(Diagnosis.network(IOException("x", SSLHandshakeException("PKIX")), "https://vector.lan").contains("Zertifikat"))
        assertTrue(Diagnosis.sessionNotSent("http://vector.lan").contains("VECTRA_COOKIE_SECURE=false"))
    }

    @Test
    fun `http bleibt http, ohne Schema wird https ergaenzt`() {
        assertEquals("http://vector.lan/api/v1/", ApiClient.normalizeBase("http://vector.lan").toString())
        assertEquals("https://vector.lan/api/v1/", ApiClient.normalizeBase("vector.lan").toString())
        assertEquals("http://192.168.1.20:8080/api/v1/", ApiClient.normalizeBase("http://192.168.1.20:8080/").toString())
    }
}
