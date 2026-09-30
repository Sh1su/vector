package app.vectra.android.feature

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.systemBarsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import app.vectra.android.ui.ButtonKind
import app.vectra.android.ui.Note
import app.vectra.android.ui.NoteKind
import app.vectra.android.ui.V
import app.vectra.android.ui.VButton
import app.vectra.android.ui.VField
import app.vectra.android.ui.VIcons
import app.vectra.android.ui.VType

/** Anmeldung nach dem Entwurf „Login“, ergänzt um die Server-Adresse (selbst gehostet). */
@Composable
fun LoginScreen(
    state: LoginState,
    logo: @Composable () -> Unit,
    onServer: (String) -> Unit,
    onEmail: (String) -> Unit,
    onPassword: (String) -> Unit,
    onSubmit: () -> Unit,
) {
    val c = V.colors
    Column(
        Modifier.fillMaxSize().background(c.bg).systemBarsPadding().imePadding()
            .verticalScroll(rememberScrollState()).padding(start = 24.dp, end = 24.dp, top = 40.dp, bottom = 28.dp),
        verticalArrangement = Arrangement.spacedBy(24.dp),
    ) {
        Column(Modifier.fillMaxWidth().padding(top = 12.dp), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(12.dp)) {
            logo()
            Text("Intelligentes Fahrzeugmanagement auf einen Blick.", style = VType.body, color = c.muted, textAlign = TextAlign.Center)
        }
        Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
            VField("Server", state.server, onServer, VIcons.sync, placeholder = "vectra.example.org", keyboardType = KeyboardType.Uri)
            VField("E-Mail", state.email, onEmail, VIcons.mail, placeholder = "name@beispiel.de", keyboardType = KeyboardType.Email)
            VField("Passwort", state.password, onPassword, VIcons.lock, placeholder = "Passwort", password = true)
        }
        if (state.error != null) Note(NoteKind.Bad, state.error)
        VButton(
            "Anmelden", onSubmit, Modifier.fillMaxWidth(), kind = ButtonKind.Navy, loading = state.busy,
            enabled = state.server.isNotBlank() && state.email.isNotBlank() && state.password.isNotEmpty(),
        )
        Note(NoteKind.Soft, "Selbst gehostet: Deine Fahrzeugdaten bleiben auf deinem Server. Konto noch nicht vorhanden? Frag deine Administration nach einer Einladung.")
    }
}
