VERDICT: CHANGES_REQUESTED

## 1. DSGVO / GDPR

### G-1 · Fehlende Obergrenze für die Speicherdauer (Art. 5 Abs. 1 lit. e DSGVO)
**Severity: high**

`POST /pastes` akzeptiert in `create_handler.go` einen beliebig großen positiven `int64`-Wert für `expires_in_seconds`. Die Funktion `resolveTTL` multipliziert diesen ohne weitere Prüfung mit `time.Second`. Dadurch kann ein Paste personenbezogene Inhalte praktisch unbegrenzt speichern; eine Speicherbegrenzung ist nicht festgelegt. Das verletzt den Grundsatz der Speicherbegrenzung.

**Konkrete Abhilfe in `create_handler.go`:**
- Konstante `maxTTL = 30 * 24 * time.Hour` (oder einen dokumentierten, datenschutzrechtlich zulässigen Höchstwert) einführen.
- In `handleCreatePaste` vor dem Aufruf von `s.Create(...)` prüfen:
  ```go
  ttl := resolveTTL(req.ExpiresInSeconds)
  if ttl > maxTTL {
      writeError(w, http.StatusBadRequest, "expires_in_seconds exceeds maximum allowed")
      return
  }
  ```
- Zusätzlich den Höchstwert im README/der API-Beschreibung dokumentieren.

### G-2 · Transportverschlüsselung fehlt (Art. 32 Abs. 1 DSGVO)
**Severity: high**

`main.go` lauscht standardmäßig unverschlüsselt auf `:8080` über `http.ListenAndServe`. Es ist keine TLS-Konfiguration, kein `ListenAndServeTLS` und keine erzwungene HTTPS-Weiterleitung vorhanden. Paste-Inhalte können personenbezogene Daten sein; deren Übertragung im Klartext verletzt Art. 32 DSGVO, sofern der Dienst nicht zwingend hinter einem TLS-terminierenden Reverse Proxy betrieben wird.

**Konkrete Abhilfe in `main.go`:**
- Mindestens verbindlich dokumentieren, dass TLS am Edge terminiert werden muss und der Dienst nicht direkt öffentlich exponiert werden darf.
- Besser: TLS direkt unterstützen, z. B. über `http.Server` mit `TLSConfig` und `ListenAndServeTLS`, sofern Zertifikate vorhanden sind.
- README/Deployment-Anleitung: „Public-Betrieb nur hinter TLS-fähigem Proxy/Load-Balancer“.

### G-3 · Kein `Cache-Control: no-store` auf GET-Endpunkten (Art. 17 DSGVO / Data Protection by Design)
**Severity: medium**

`respond.go` setzt in `writeJSON` nur `Content-Type: application/json; charset=utf-8`. Insbesondere `GET /pastes/{id}` liefert den vollständigen Paste-Inhalt ohne `Cache-Control: no-store`. Browser, Intermediate-Proxies oder CDNs könnten eine gelöschte bzw. abgelaufene Paste weiter ausliefern. Damit wird die Löschung nach Ablauf faktisch unterlaufen.

**Konkrete Abhilfe:**
- In `handleGetPaste` vor dem Schreiben der Antwort setzen:
  ```go
  w.Header().Set("Cache-Control", "no-store")
  ```
- Alternativ `writeJSON` um einen optionalen Parameter erweitern oder eine eigene Funktion `writePrivateJSON` für Inhalte mit personenbezogenem Charakter einführen.
- GET `/pastes` lässt sich aus Konsistenzgründen ebenfalls mit `no-store` versehen, auch wenn dort nur Metadaten ausgeliefert werden.

### G-4 · Datenminimierung bei der Listenansicht (Art. 5 Abs. 1 lit. c DSGVO)
**Severity: low**

`handleListPastes` ruft `Store.List()` auf, das aus der Store-Map eine Slice mit allen `Paste`-Werten **inklusive `content`** erzeugt. Der Handler filtert und projiziert anschließend auf `pasteMeta`; der Inhalt wird aber für keine Ausgabe benötigt. Es werden also unnötig Inhalte in den Arbeitsspeicher geladen.

**Konkrete Abhilfe:**
- Eine Methode `Store.ListMeta() []pasteMeta` implementieren, die ausschließlich ID, Sprache, Ablaufzeit und Erstellzeit zurückgibt.
- In `handleListPastes` `ListMeta()` statt `List()` verwenden.

### G-5 · Positivbefunde DSGVO
Die folgenden Anforderungen sind im sichtbaren Code erfüllt:
- **AC-13:** Abgelaufene Paste werden durch `purgeExpiredLocked` und `scheduleExpiryLocked` vollständig aus der Store-Map entfernt.
- **AC-14:** Default-TTL von 24 Stunden ist definiert und wird angewendet.
- **AC-15:** Fehlerantworten enthalten ausschließlich das Feld `error` und keine Paste-Inhalte.
- **AC-16:** `log.Printf` gibt nur die Listen-Adresse aus; keine Paste-Inhalte oder vollständige Paste-Objekte.
- **AC-10:** IDs werden über `crypto/rand` erzeugt.

---

## 2. EU Cyber Resilience Act (CRA)

### C-1 · HTTP-Server ohne Timeouts bei `http.ListenAndServe`
**Severity: high**

`main.go` verwendet `http.ListenAndServe(addr, newMux(s))` ohne `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout` und `IdleTimeout`. Damit ist der Server anfällig für Slowloris, hängende Verbindungen und Ressourcenerschöpfung. Das widerspricht dem CRA-Grundsatz „security by design/default“.

**Konkrete Abhilfe in `main.go`:**
- Statt `http.ListenAndServe` einen expliziten Server verwenden:
  ```go
  srv := &http.Server{
      Addr:              addr,
      Handler:           newMux(s),
      ReadHeaderTimeout: 5 * time.Second,
      ReadTimeout:       15 * time.Second,
      WriteTimeout:      15 * time.Second,
      IdleTimeout:       120 * time.Second,
      MaxHeaderBytes:    1 << 20,
  }
  if err := srv.ListenAndServe(); err != nil { ... }
  ```
- `ReadTimeout`/`WriteTimeout` so bemessen, dass der maximale Body von 1 MiB weiterhin übertragen werden kann; alternativ konfigurierbar machen.

### C-2 · Keine Begrenzung der Anzahl gespeicherter Pastes / kein Rate-Limit
**Severity: medium**

Der Store akzeptiert unbegrenzt viele Pastes. Ein Angreifer kann mit vielen kleinen Anfragen den verfügbaren Arbeitsspeicher erschöpfen. Das betrifft Verfügbarkeit und Ressourcenintegrität (CRA, Robustheit) sowie mittelbar DSGVO-Datenminimierung.

**Konkrete Abhilfe:**
- In `store.go` einen Höchstwert einführen, z. B.:
  ```go
  const maxPastes = 10_000
  ```
- In `Create` vor dem Einfügen prüfen, ob `len(s.pastes) >= maxPastes`; bei Erreichen des Limits einen Fehler zurückgeben, den der Handler als `503 service unavailable` oder `429 too many requests` beantwortet.
- Optional zusätzlich eine einfache Rate-Limit-Middleware in `main.go` bzw. `newMux` einführen.

### C-3 · Fehlende SBOM / Sicherheits- und Update-Dokumentation
**Severity: medium**

`go.mod` zeigt als Abhängigkeit nur die Go-Standardbibliothek. Es fehlt jedoch eine maschinenlesbare SBOM (z. B. SPDX/CycloneDX) sowie eine dokumentierte Aussage zu Sicherheitseigenschaften, Unterstützungsdauer, Update-/Patch-Prozess und Meldemöglichkeit für Schwachstellen. Diese Informationen werden für Produkte mit digitalen Elementen unter dem CRA erwartet.

**Konkrete Abhilfe:**
- Im vorhandenen `README.md` einen Abschnitt „Security / SBOM / Support“ ergänzen:
  - Framework- und Runtime-Version (`Go 1.22+`),
  - Abhängigkeiten (keine externen Module),
  - Prozess für Sicherheitsupdates,
  - Kontakt für Schwachstellenmeldungen.
- Zusätzlich eine `sbom.spdx.json` oder `sbom.cdx.json` ins Repo aufnehmen, die mindestens die Go-Runtime und das Modul beschreibt.

### C-4 · Kein Graceful Shutdown
**Severity: low**

`main.go` ruft `http.ListenAndServe` auf und beendet den Prozess bei Fehler über `log.Fatal`. `defer s.Close()` wird bei `log.Fatal`/`os.Exit` nicht ausgeführt. Damit wird der Reaper-Goroutine nicht sauber beendet. Für einen sicheren Betrieb und CRA-konforme Wartbarkeit ist ein kontrolliertes Herunterfahren sinnvoll.

**Konkrete Abhilfe in `main.go`:**
- `signal.NotifyContext` verwenden und bei Signal:
  ```go
  ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
  defer stop()
  go func() { <-ctx.Done(); srv.Shutdown(context.Background()) }()
  ```
- `defer s.Close()` wirkt dann zuverlässig.

---

## 3. EU AI Act

**Nicht anwendbar.** Im sichtbaren Code sind keine KI-Funktionen, keine automatisierten Entscheidungen und keine Modelle enthalten. Es bestehen daher keine Kennzeichnungs- oder Transparenzpflichten nach dem AI Act.

---

## 4. Pflichttexte & UI

**Nicht anwendbar für das Produkt als reines `go-backend`.** Die API hat keine Endnutzer-Oberfläche; es entstehen keine Pflichten für Impressum, Cookie-Banner oder barrierefreie Web-Oberfläche. Datenschutzhinweise, sofern der Dienst öffentlich betrieben wird, sind als Betreiber-Dokumentation außerhalb des Backends bereitzustellen; das Backend selbst muss keine Datenschutzerklärung ausliefern.

---

## 5. Barrierefreiheit (WCAG/BITV/EAA)

**Nicht anwendbar.** Es gibt keine öffentliche Web-UI. Eine REST-API als solche unterliegt keinen Anforderungen an visuelle Barrierefreiheit.

---

## Fazit

Das Produkt ist in weiten Teilen sauber umgesetzt: Body-Limit, `crypto/rand`, Fehlerantworten ohne interne Details, Panic-Recovery, Ablauf-Löschung und Log-Disziplin sind vorhanden. Es bestehen jedoch behebbare Lücken insbesondere bei der maximalen Speicherdauer, der Transportverschlüsselung, den Server-Timeouts und der CRA-Dokumentation. Es liegen keine fundamentalen, sofort blockierenden Verstöße vor; daher ist das Votum `CHANGES_REQUESTED`.