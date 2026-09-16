# Spotify-Integration und Cache

> Deutsch. English version: [spotify.en.md](spotify.en.md)

## Ziel

Spotify wird als eigener Provider integriert. Pro MuPiBox wird ein separates Spotify-Premium-Family-Konto verbunden. Die Administration soll Künstler, Alben und Playlists suchen und ohne manuelle IDs an Navigationsknoten binden können.

## Klare Trennung

Die Spotify Web API liefert Katalog-, Such-, Bibliotheks-, Playlist- und Player-Metadaten. Sie ist nicht selbst der Audio-Decoder der Box. Katalogzugriff und Audioausgabe werden deshalb getrennt:

- `SpotifyCatalogProvider`: OAuth, Suche, Künstler, Alben, Playlists, Titel und Metadaten,
- `SpotifyPlaybackAdapter`: Start/Pause/Seek/Queue/Device und tatsächliche Audioausgabe,
- normalisiertes MuPiBox-Medienmodell zwischen Provider, Player und UI.

Die konkrete Playback-Technik wird erst nach einem vollständigen Pi-3-/DietPi-Test festgelegt. Der offizielle Web Playback SDK erfordert Spotify Premium und läuft in einer Browserumgebung; `librespot` bleibt ein möglicher, aber inoffizieller Kandidat. Die Architektur darf keinen dieser Wege fest verdrahten.

## Einrichtung im Adminbereich

Vorgesehener Ablauf:

1. Spotify-App-/OAuth-Konfiguration prüfen,
2. Konto dieser Box verbinden,
3. Berechtigungen und Tokenstatus anzeigen,
4. Quelle hinzufügen,
5. Künstler, Album oder Playlist suchen,
6. Ergebnis auswählen,
7. Inhaltsmodus wählen, zum Beispiel Alben oder Titel,
8. Navigationsposition und Wiedergabeprofil wählen,
9. speichern und auf 800×480 voranzeigen.

Client Secret, Refresh Token und weitere Zugangsdaten werden nicht an die Playeroberfläche ausgeliefert und nicht in Logs oder Git geschrieben.

## Cache und Rate Limits

Der Cache dient nur der Performance und Reduzierung unnötiger API-Aufrufe. Er ist kein Offline-Musikspeicher.

Gespeichert werden dürfen nur temporär benötigte Metadaten und Cover entsprechend den jeweils aktuellen Spotify-Vorgaben. Spotify-Audiodaten werden nicht als normaler lokaler Cache gespeichert. Jeder Cacheeintrag besitzt Provider, Account, Ressourcentyp, Ressourcenschlüssel, Abrufzeit und Ablaufzeit.

Vorgesehen sind:

- kurze TTLs für Suche und veränderliche Playerzustände,
- längere, aber endliche TTLs für Album-/Künstlermetadaten und Cover,
- Playlist-`snapshot_id` zur gezielten Erkennung von Änderungen,
- Zusammenfassen/Deduplizieren paralleler identischer Abfragen,
- Pagination und bedarfsgerechtes Nachladen,
- Stale-while-revalidate für die Anzeige, sofern zulässig,
- Begrenzung von Cachegröße und Alter,
- Löschen accountbezogener Daten beim Trennen des Kontos.

Bei HTTP 429 wird `Retry-After` beachtet. Wiederholungen verwenden Backoff und niemals enge Schleifen. Abgelaufene Tokens werden serverseitig aktualisiert.

## Wiedergabefortschritt

Für Spotify-Inhalte wird die letzte Position wie bei lokalen Dateien gespeichert. Der Schlüssel besteht mindestens aus Provider `spotify`, Box-/Accountkontext und stabiler Spotify-Medien-ID. Position und Dauer werden in Millisekunden gespeichert. Beim erneuten Start wird fortgesetzt, sofern das Medium nicht als beendet markiert wurde. Live-Radio bleibt von diesem Mechanismus ausgeschlossen.

## Offline-Verhalten

Spotify ist ohne Internet nicht als vollwertige Offlinequelle zugesagt. Bei Netzausfall kann die UI zuletzt erlaubte Metadaten kurzfristig anzeigen und verständlich melden, dass die Quelle derzeit nicht abspielbar ist. Lokale Medien bleiben davon unabhängig nutzbar.

Eine spätere Spotify-Offline-Funktion darf nur umgesetzt werden, wenn sie mit den dann gültigen Spotify-Vorgaben und einer unterstützten Playback-Technik vereinbar ist.

## Persistenz

Erwartete Konzepte:

- `provider_accounts`,
- geschützte Token-/Secret-Ablage,
- `content_bindings` mit Spotify-ID/URI und Typ,
- `provider_cache` mit Ablaufzeit,
- optional Playlist-Snapshots,
- providerneutrale stabile Medien-IDs für Fortschritt und Navigation.

Offizielle Referenzen:

- https://developer.spotify.com/documentation/web-api/
- https://developer.spotify.com/documentation/web-api/concepts/api-calls
- https://developer.spotify.com/documentation/web-api/concepts/scopes
- https://developer.spotify.com/terms
