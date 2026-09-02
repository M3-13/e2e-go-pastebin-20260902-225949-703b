VERDICT: CHANGES_REQUESTED

### Security Report

#### Prüfung der Kernbereiche

- **Secrets:** Keine hartkodierten Schlüssel, Passwörter, Tokens oder internen URLs im sichtbaren Code. Logs in `main.go` geben nur die Listen-Adresse aus, keine Paste-Inhalte.
- **Injection / Eingaben:** Keine SQL-, Command- oder Path-Injection erkennbar. IDs werden streng validiert. JSON wird über die Standardbibliothek geparst. Es fehlt jedoch eine vollständige Ablehnung von ungültigem JSON mit zusätzlichen Daten hinter dem ersten Objekt.
- **AuthN/AuthZ:** Keine Authentifizierung oder Autorisierung vorhanden. Insbesondere kann jede:r über `GET /pastes` alle IDs sehen und anschließend `DELETE /pastes/{id}` für beliebige Pastes aufrufen.
- **Dependencies:** Es werden ausschließlich Go-Standardbibliotheken verwendet. Ein externer Scanner wurde für dieses Projekt nicht ausgeführt; das Fehlen von Scannerbefunden ist keine Garantie, wird aber nicht als Finding gewertet.
- **Konfiguration / Transport:** HTTP läuft unverschlüsselt, ohne Server-Timeout-Konfiguration. Für ein internes/öffentliches Pastebin je nach Deployment relevant.

---

### 1. Mittel — DELETE ohne Besitznachweis erlaubt Löschen beliebiger Pastes

**Betroffene Stelle:** `main.go` (Routen), `delete_handler.go`, `get_handler.go` (Liste veröffentlicht IDs)

**Problem:**  
`GET /pastes` liefert alle aktiven Paste-IDs. `DELETE /pastes/{id}` akzeptiert jede gültige ID ohne Nachweis der Urheberschaft. Jeder Client kann damit sämtliche Pastes löschen — auch solche, die er nicht erstellt hat. Dies ist ein fehlender Zugriffsschutz und führt zu Datenverlust/Denial-of-Service.

**Fix (Härtung):**  
Beim Anlegen eines Pastes ein zufälliges `delete_token` erzeugen, nur in der Create-Antwort zurückgeben und serverseitig z. B. als Hash speichern. `DELETE /pastes/{id}` nur akzeptieren, wenn das Token per Header (`X-Delete-Token`) mitgesendet wird und zum gespeicherten Hash passt. Das `delete_token` darf nicht in `GET /pastes` oder `GET /pastes/{id}` auftauchen.

**Hinweis zur Abnahme:**  
Die aktuellen Tests (`TestDeletePaste`, AC-05) rufen `DELETE` direkt ohne Token auf. Die Einführung eines Tokens erfordert eine bewusste Erweiterung der Acceptance Criteria und Tests. Bis dahin mindestens als dokumentiertes Restrisiko behandeln.

---

### 2. Mittel — Unbegrenzte In-Memory-Speicherung und fehlendes Rate-Limit

**Betroffene Stelle:** `store.go`, `create_handler.go`

**Problem:**  
Jeder Request kann bis zu 1 MiB Body speichern. Es gibt jedoch weder ein globales Limit für die Anzahl der Pastes noch für den gesamten Speicherverbrauch noch ein Rate-Limit. Ein Angreifer kann beliebig viele `POST /pastes` absetzen und so den Prozess-Speicher erschöpfen (OOM).

**Fix:**  
Im `Store` globale Limits einführen, z. B.:
- maximale Anzahl aktiver Pastes,
- maximale Summe der gespeicherten Bytes (über `len(Content)` + `len(Language)`),
- ein maximales TTL-Fenster, damit Pastes nicht unbefristet gehalten werden.

Bei Überschreitung `writeError(w, http.StatusTooManyRequests, "too many pastes")` oder `503` mit vordefinierter Meldung zurückgeben. Zusätzlich eine einfache Rate-Limit-Middleware vorsehen.

---

### 3. Mittel — `expires_in_seconds` kann bei großen Werten überlaufen

**Betroffene Stelle:** `create_handler.go`, Funktion `resolveTTL`

**Problem:**  
`time.Duration(*expiresInSeconds) * time.Second` multipliziert zwei `int64`-Werte. Bei Eingaben oberhalb von etwa `math.MaxInt64 / int64(time.Second)` läuft die Multiplikation über und ergibt einen negativen TTL-Wert. Der Paste wird dann scheinbar erfolgreich erstellt, ist aber sofort abgelaufen. Für Clients ist das irreführend; zusätzlich können viele zeitgesteuerte Aufräum-Timer entstehen.

**Fix:**  
Einen oberen Grenzwert definieren und vor der Multiplikation prüfen, z. B.:

```go
const maxTTLSeconds = int64((1<<63 - 1) / int64(time.Second))

if *expiresInSeconds <= 0 || *expiresInSeconds > maxTTLSeconds {
    writeError(w, http.StatusBadRequest, "invalid expires_in_seconds")
    return
}
```

Alternativ ein produktspezifisches Maximal-TTL setzen (z. B. 30 Tage), um die Lebensdauer in In-Memory-Betrieb sinnvoll zu begrenzen.

---

### 4. Mittel — HTTP-Server ohne Timeouts

**Betroffene Stelle:** `main.go`

**Problem:**  
`http.ListenAndServe(addr, mux)` verwendet den Default-Server ohne `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout` oder `IdleTimeout`. Langsame oder absichtlich offen gehaltene Verbindungen können Ressourcen blockieren (z. B. Slowloris).

**Fix:**

```go
srv := &http.Server{
    Addr:              addr,
    Handler:           newMux(s),
    ReadHeaderTimeout: 5 * time.Second,
    ReadTimeout:       10 * time.Second,
    WriteTimeout:      20 * time.Second,
    IdleTimeout:       60 * time.Second,
}
if err := srv.ListenAndServe(); err != nil {
    log.Fatal(err)
}
```

`time` ist bereits im Projekt vorhanden.

---

### 5. Niedrig — JSON-Decoder akzeptiert ungültige Zusatzdaten nach dem JSON-Objekt

**Betroffene Stelle:** `create_handler.go`, `handleCreatePaste`

**Problem:**  
`json.NewDecoder(r.Body).Decode(&req)` liest nur das erste JSON-Objekt und ignoriert weitere Daten. Ein Body wie

```json
{"content":"hallo"}garbage
```

wird nicht als ungültig abgelehnt, sondern mit `201` beantwortet. Das verletzt AC-06 („Ungültiges JSON … 400“) in Randfällen und kann in Proxys oder API-Gateways zu unterschiedlicher Interpretation führen.

**Fix:**  
Nach der erfolgreichen Decodierung sicherstellen, dass keine weiteren Token folgen:

```go
dec := json.NewDecoder(r.Body)
if err := dec.Decode(&req); err != nil {
    // bestehende Fehlerbehandlung
}
if dec.More() {
    writeError(w, http.StatusBadRequest, "invalid JSON")
    return
}
```

Alternativ prüfen, ob ein zweiter `Decode`-Aufruf `io.EOF` liefert.

---

### 6. Niedrig — Transport unverschlüsselt (siehe Deployment-Kontext)

**Betroffene Stelle:** `main.go`

**Problem:**  
Der Dienst lauscht auf `:PORT` per HTTP ohne TLS. Bei Erreichbarkeit aus einem nicht vertrauenswürdigen Netz sind Paste-Inhalte und IDs unverschlüsselt übertragbar.

**Fix:**  
Sofern der Dienst öffentlich erreichbar ist, TLS über einen Reverse-Proxy oder direkt über `http.ListenAndServeTLS` umsetzen. Für rein lokale oder abgeschottete Netze kann dies bewusst akzeptiert werden.

---

### Bewertung

Die sichtbaren Datenschutz-Anforderungen (kein vollständiger Paste-Content in Fehlern/Logs, Entfernen abgelaufener Pastes aus dem Speicher) sind erfüllt. Die wichtigsten Risiken sind der fehlende Schutz vor unbefugtem Löschen sowie die fehlende Ressourcenlimitierung und fehlende Server-Timeouts. Es wurden keine kritischen Schwachstellen wie hartkodierte Secrets, Injection/RCE oder ausgenutzte CVEs gefunden.