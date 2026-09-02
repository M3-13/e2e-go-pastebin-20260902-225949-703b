# Pastebin REST API

Eine kleine Pastebin-REST-API in Go, die ausschließlich die Standardbibliothek
(`net/http`) nutzt. Pasts werden in einem thread-sicheren In-Memory-Store mit
`sync.Mutex` gehalten und können erstellt, gelesen, gelistet und gelöscht werden.
Ablauf über `expires_in_seconds`, saubere Statuscodes und JSON-Fehler, getestet
mit `httptest`.

## Tech Stack

- **Language**: Go 1.22+
- **Framework**: `net/http` (Standardbibliothek)
- **Storage**: In-Memory mit `sync.Mutex`
- **Module**: `go.mod` (Modul `pastebin`)

## Installation

Keine externen Abhängigkeiten — nur die Go-Standardbibliothek. Voraussetzung ist
eine Go-Installation (1.22 oder neuer).

```sh
go mod download
```

## Entwicklung / Start

```sh
go run .
```

Der Server lauscht auf Port `8080` (konfigurierbar über die Umgebungsvariable
`PORT`):

```sh
PORT=9000 go run .
```

## Build (Produktion)

```sh
go build ./...
```

## Verwendung

REST-API mit JSON-Antworten (Zeitstempel als RFC3339):

| Methode | Pfad              | Beschreibung                                     |
|---------|-------------------|--------------------------------------------------|
| GET     | `/health`         | Health-Check, liefert `200 {"status":"ok"}`      |
| POST    | `/pastes`         | Neuen Paste anlegen                              |
| GET     | `/pastes/{id}`    | Einzelnen Paste inkl. `content` abrufen          |
| GET     | `/pastes`         | Metadaten aller aktiven Pasts (ohne `content`)   |
| DELETE  | `/pastes/{id}`    | Paste löschen                                    |

Fehlerantworten enthalten ausschließlich das Feld `error` mit einer
vordefinierten Meldung, z. B. `{"error":"not found"}`.

## Features

- Erstellen, Lesen, Auflisten und Löschen von Pasts
- Thread-sicherer In-Memory-Store (`sync.Mutex`)
- Ablauf über `expires_in_seconds` (Default 24 h)
- Zufällige IDs über `crypto/rand`
- Panic-Recovery-Middleware (500 als JSON-Fehler)
- Saubere JSON-Fehler ohne interne Details
