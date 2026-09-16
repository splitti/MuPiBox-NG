# Dynamisches Navigations- und Inhaltsmodell

> Deutsch. English version: [navigation-model.en.md](navigation-model.en.md)

## Ziel

Die MuPiBox-Oberfläche darf keine feste Hierarchie wie `Kategorie -> Reihe -> lokaler Ordner` voraussetzen.
Stattdessen wird die Navigation als frei konfigurierbarer Baum aus Knoten modelliert. Ein Knoten kann ein reiner
Navigationsordner sein oder direkt auf eine Inhaltsquelle zeigen.

Damit sind unter anderem folgende Varianten möglich:

### Variante A – klassische Unterebene

```text
Hörbücher
└── Die drei ??? Kids
    ├── Album/Folge 001
    ├── Album/Folge 002
    └── Album/Folge 003
```

Ein Tipp auf `Die drei ??? Kids` öffnet eine eigene Ansicht mit den Alben/Folgen als Cover-Kacheln – ähnlich der
alten MuPiBox-Oberfläche.

### Variante B – Inhalt direkt auf der Startseite

```text
Die drei ??? Kids
[Album 001] [Album 002] [Album 003] ...
```

Der Knoten ist selbst eine Hauptkategorie. Seine Quelle kann zum Beispiel ein Spotify-Künstler sein. Die vom
Provider gelieferten Alben werden direkt unter der Überschrift angezeigt.

### Variante C – Playlist direkt als Kategorie

```text
Lieblingslieder
[Titel 1] [Titel 2] [Titel 3] ...
```

Die Kategorie verweist direkt auf eine Spotify-Playlist oder eine manuell gepflegte Titelliste. Statt Alben werden
die enthaltenen Titel angezeigt.

## Navigationsknoten

Jeder Navigationsknoten besitzt mindestens:

- stabile ID,
- optionale Parent-ID,
- Sortierposition,
- aktiv/sichtbar,
- lokalisierte Anzeigenamen,
- Darstellungsmodus,
- Auswahlverhalten,
- optional eine Inhaltsbindung.

Die sichtbare Bezeichnung ist von der technischen Quelle unabhängig. Eine Kategorie kann also `Die drei ??? Kids`
heißen, obwohl sie intern beispielsweise auf eine Spotify-Artist-ID oder einen lokalen Ordner zeigt.

## Auswahlverhalten

Ein Knoten kann mindestens zwei grundlegende Verhaltensweisen haben:

- `drilldown`: Antippen öffnet eine neue Ansicht mit Kindern bzw. aufgelösten Inhalten.
- `inline`: Der Inhalt wird direkt unter der Kategorieüberschrift auf der aktuellen Seite angezeigt.

Später kann zusätzlich `play` sinnvoll sein, um einen Knoten ohne Zwischenansicht sofort abzuspielen, zum Beispiel
einen Radiosender oder eine feste Playlist.

## Inhaltsbindung

Ein Knoten kann optional eine providerneutrale Inhaltsbindung besitzen. Erwartete Felder/Konzepte:

- Provider, z. B. `local`, `spotify`, `radio`, `rss`, `manual`,
- Quelltyp, z. B. `artist`, `playlist`, `album`, `folder`, `feed`, `station`, `collection`,
- Provider-Referenz/ID,
- gewünschter Inhaltsmodus,
- optionale Providerparameter.

Beispiele:

```text
Provider: spotify
Typ: artist
Referenz: <Spotify Artist ID>
Inhaltsmodus: albums
```

```text
Provider: spotify
Typ: playlist
Referenz: <Spotify Playlist ID>
Inhaltsmodus: tracks
```

```text
Provider: local
Typ: folder
Referenz: /Hörbücher/Die drei Fragezeichen Kids
Inhaltsmodus: children
```

Die Weboberfläche erhält keine Spotify-/Dateisystem-Sonderlogik. Der jeweilige Provider löst die Referenz auf und
gibt normalisierte Medienobjekte zurück.

## Inhaltsmodus

Der Adminbereich soll mindestens folgende Modi anbieten:

- `auto` – Provider wählt eine sinnvolle Standarddarstellung,
- `children` – Unterknoten/Unterordner anzeigen,
- `albums` – Alben/Folgen anzeigen,
- `tracks` – Titel direkt anzeigen,
- `items` – generische Medienobjekte anzeigen.

Beispiele für `auto`:

- Spotify-Künstler -> Alben,
- Spotify-Playlist -> Titel,
- Spotify-Album -> Titel,
- lokaler Ordner mit Unterordnern -> Unterordner,
- Radiosender -> direkt abspielbar.

Der Benutzer kann die Automatik im Adminbereich überschreiben, soweit der Provider den gewünschten Modus unterstützt.

## Darstellung

Die Darstellung bleibt vom Provider getrennt. Vorgesehene Darstellungsmodi sind beispielsweise:

- `auto`,
- Cover-Kacheln horizontal,
- kompakte Liste,
- große Cover-Ansicht für eine Drill-down-Seite.

Auf dem 800x480-Zieldisplay gilt weiterhin:

- Kategorien vertikal,
- Medien innerhalb einer Kategorie bevorzugt horizontal,
- große Touchziele,
- kein Hover als Voraussetzung,
- möglichst wenige Navigationsebenen.

## Admin-Interface

Beim Erstellen oder Bearbeiten eines Navigationsknotens soll der Admin später ungefähr folgende Entscheidungen treffen können:

1. Name und Übersetzungen,
2. Position/übergeordnete Kategorie,
3. sichtbar/aktiv,
4. Anzeige direkt (`inline`) oder nach Antippen (`drilldown`),
5. Quelle: manuell, lokal, Spotify, Radio, Podcast usw.,
6. Quelltyp und konkrete Referenz,
7. gewünschte Ausgabe: automatisch, Alben, Titel, Unterelemente usw.,
8. Darstellungsart.

Das Admin-Interface soll bei Providerquellen möglichst suchen und auswählen lassen, statt technische IDs manuell
eintragen zu müssen. Beispiel: Spotify verbinden -> nach `Die drei ??? Kids` suchen -> Künstler auswählen ->
`Alben anzeigen` wählen.

## Providervertrag

Provider liefern normalisierte Objekte und dürfen keine providerabhängigen UI-Komponenten erzwingen. Ein normalisiertes
Objekt soll mindestens eine stabile Referenz, Typ, Titel, optional Untertitel/Cover und mögliche Aktionen enthalten.

Die UI kann deshalb lokale Ordner, Spotify-Alben, Podcastfolgen und andere Quellen mit denselben generischen Karten-
und Listenkomponenten darstellen.

## Persistenz

Das Navigationsmodell wird später in SQLite gespeichert. Die genaue Tabellenstruktur wird migrationsbasiert eingeführt.
Voraussichtliche Kernobjekte sind:

- `navigation_nodes`,
- `navigation_node_labels`,
- `content_bindings`,
- optionale manuelle Zuordnungen/Items,
- globale Darstellungseinstellungen.

Die bisherige `categories`-/`content_rows`-Struktur im Entwicklungsprototyp ist ein Zwischenschritt und darf beim
SQLite-Umbau in dieses allgemeinere Modell migriert werden.
