package app.vectra.core.net

import java.io.IOException
import java.net.ConnectException
import java.net.SocketTimeoutException
import java.net.UnknownHostException
import java.net.UnknownServiceException
import javax.net.ssl.SSLException
import javax.net.ssl.SSLHandshakeException

/** Verständliche Ursache für Verbindungsfehler beim Anmelden (selbst gehostete Server, oft im LAN). */
object Diagnosis {
    fun network(e: IOException, base: String): String {
        val chain = generateSequence<Throwable>(e) { it.cause }.toList()
        val text = chain.joinToString(" ") { it.message.orEmpty() }
        return when {
            chain.any { it is UnknownServiceException } || text.contains("CLEARTEXT", ignoreCase = true) ->
                "Unverschlüsseltes HTTP ist für $base nicht erlaubt. Nutze HTTPS oder einen lokalen Namen (z. B. *.lan, *.local, *.home.arpa)."
            chain.any { it is UnknownHostException } ->
                "Der Name ${host(base)} wurde nicht gefunden. Prüfe den DNS-Eintrag und ob das Handy im selben Netz ist (Privates DNS in Android kann lokale Namen blockieren)."
            chain.any { it is SSLHandshakeException } || text.contains("CertPath", ignoreCase = true) ->
                "Das HTTPS-Zertifikat von ${host(base)} ist nicht vertrauenswürdig. Installiere das Zertifikat deiner Zertifizierungsstelle auf dem Handy oder nutze http:// im Heimnetz."
            chain.any { it is SSLException } ->
                "HTTPS-Verbindung zu ${host(base)} fehlgeschlagen. Läuft der Server evtl. nur mit http://?"
            chain.any { it is ConnectException } ->
                "Keine Verbindung zu ${host(base)}. Läuft der Server und stimmt der Port?"
            chain.any { it is SocketTimeoutException } ->
                "Zeitüberschreitung bei ${host(base)}. Ist der Server im Netz erreichbar?"
            else -> "Server nicht erreichbar: ${e.message ?: e.javaClass.simpleName}"
        }
    }

    /** Anmeldung gelang, aber die Sitzung wird nicht mitgesendet: Cookie mit „Secure“ über HTTP. */
    fun sessionNotSent(base: String): String =
        if (base.startsWith("http://"))
            "Anmeldung erfolgreich, aber die Sitzung wird über HTTP nicht übertragen. Setze auf dem Server VECTRA_COOKIE_SECURE=false (nur im Heimnetz) oder nutze HTTPS."
        else "Anmeldung erfolgreich, aber die Sitzung wurde nicht übernommen. Prüfe den Reverse-Proxy (Cookies und Header müssen durchgereicht werden)."

    private fun host(base: String) = base.substringAfter("://").substringBefore('/').substringBefore(':')
}
