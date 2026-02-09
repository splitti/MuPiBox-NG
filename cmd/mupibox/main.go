package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"mupibox/internal/catalog"
	"mupibox/internal/state"
)

type HomeItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Type        string `json:"type"` // artist, playlist, podcast, continue
	Image       string `json:"image"`
	CanResume   bool   `json:"can_resume"`
	ResumePos   int    `json:"resume_pos_sec,omitempty"`
	ResumeLabel string `json:"resume_label,omitempty"`
}

type HomeSection struct {
	Title string     `json:"title"`
	Items []HomeItem `json:"items"`
}

func main() {
	cat, err := catalog.LoadCatalog("config/catalog.json")
	if err != nil {
		log.Fatal(err)
	}

	stateStore, err := state.NewStore("data/state.json")
	if err != nil {
		log.Fatal(err)
	}

	// -------- helpers --------
	findCatalogItem := func(id string) *catalog.Item {
		for _, c := range cat.Categories {
			for _, it := range c.Items {
				if it.ID == id {
					tmp := it
					return &tmp
				}
			}
		}
		return nil
	}

	pickCover := func(it *catalog.Item) string {
		if it == nil {
			return "/covers/placeholder.png"
		}
		cover := "/covers/placeholder.png"
		for _, src := range it.Sources {
			if src.CoverPath != "" {
				if _, err := os.Stat("webui/static" + src.CoverPath); err == nil {
					cover = src.CoverPath
					break
				}
			}
		}
		return cover
	}

	// -------------------------------
	// HOME: + "Weiter abspielen" (letzte 10-15 aus state.json)
	// -------------------------------
	http.HandleFunc("/api/home", func(w http.ResponseWriter, r *http.Request) {
		var resp struct {
			Sections []HomeSection `json:"sections"`
		}

		// 1) Continue section aus state (nur wenn vorhanden)
		recent := stateStore.ListRecent(15)
		if len(recent) > 0 {
			sec := HomeSection{Title: "Weiter abspielen"}
			for _, e := range recent {
				// Wir versuchen die Anzeige über ItemID aus dem State aufzulösen
				it := findCatalogItem(e.State.ItemID)
				title := "Weiter"
				if it != nil {
					title = it.DisplayName
				} else if e.State.ItemID != "" {
					title = e.State.ItemID
				}

				cover := pickCover(it)

				sec.Items = append(sec.Items, HomeItem{
					ID:          e.Key, // state-key
					Title:       title,
					Type:        "continue", // Frontend ruft /api/continue/{key}
					Image:       cover,
					CanResume:   true,
					ResumePos:   e.State.PositionSec,
					ResumeLabel: "Weiter",
				})
			}
			resp.Sections = append(resp.Sections, sec)
		}

		// 2) normale Sections aus catalog.json
		for _, c := range cat.Categories {
			sec := HomeSection{Title: c.Title}
			for _, it := range c.Items {
				image := pickCover(&it)

				// Resume-Badge für Catalog-Items (wenn Resume true + state vorhanden)
				canResume := it.Resume
				var resumePos int
				var resumeLabel string
				if canResume {
					// Key-Strategie: wir speichern pro "playable" Key.
					// Für Artists/Playlists nehmen wir erstmal it.ID als Key.
					if st, ok := stateStore.Get(it.ID); ok && st.PositionSec > 0 {
						resumePos = st.PositionSec
						resumeLabel = "Weiter"
					}
				}

				sec.Items = append(sec.Items, HomeItem{
					ID:          it.ID,
					Title:       it.DisplayName,
					Type:        it.Type,
					Image:       image,
					CanResume:   canResume,
					ResumePos:   resumePos,
					ResumeLabel: resumeLabel,
				})
			}
			resp.Sections = append(resp.Sections, sec)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	// -------------------------------
	// CONTINUE DETAILS: liefert Player-Payload inkl. TrackIndex + Position
	// -------------------------------
	http.HandleFunc("/api/continue/", func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/api/continue/")
		st, ok := stateStore.Get(key)
		if !ok {
			http.NotFound(w, r)
			return
		}

		it := findCatalogItem(st.ItemID)
		cover := pickCover(it)
		title := st.ItemID
		if it != nil {
			title = it.DisplayName
		}

		// Für UI: Subtitle zeigt Track-Info
		subtitle := "Weiter"
		if st.TrackIndex > 0 {
			subtitle = "Track " + itoa(st.TrackIndex+1)
		} else if st.TrackID != "" && st.TrackID != "main" {
			subtitle = st.TrackID
		}

		resp := map[string]interface{}{
			"id":           key, // player-key
			"title":        title,
			"subtitle":     subtitle,
			"cover":        cover,
			"duration":     0, // später: echte Dauer
			"item_id":      st.ItemID,
			"album_id":     st.AlbumID,
			"track_id":     st.TrackID,
			"track_index":  st.TrackIndex,
			"position_sec": st.PositionSec,
			"updated_at":   st.UpdatedAt,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	// -------------------------------
	// ARTIST DETAILS (Mock-Alben) – später: aus Amazon/Spotify/Local
	// -------------------------------
	http.HandleFunc("/api/artist/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/artist/")

		it := findCatalogItem(id)
		if it == nil || it.Type != "artist" {
			http.NotFound(w, r)
			return
		}

		resp := map[string]interface{}{
			"id":    it.ID,
			"title": it.DisplayName,
			"cover": pickCover(it),
			"albums": []map[string]interface{}{
				{"id": id + "_folge_1", "title": "Folge 1 – Der Anfang", "cover": "/covers/placeholder.png", "duration": 3600},
				{"id": id + "_folge_2", "title": "Folge 2 – Das Geheimnis", "cover": "/covers/placeholder.png", "duration": 3600},
				{"id": id + "_folge_3", "title": "Folge 3 – Die Lösung", "cover": "/covers/placeholder.png", "duration": 3600},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	// -------------------------------
	// RESUME: GET/POST pro "playable key"
	// (Key ist bewusst frei: album/episode/playlist etc.)
	// -------------------------------
	http.HandleFunc("/api/resume/", func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/api/resume/")
		if key == "" {
			http.NotFound(w, r)
			return
		}

		switch r.Method {
		case http.MethodGet:
			if st, ok := stateStore.Get(key); ok {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(st)
				return
			}
			http.NotFound(w, r)
			return

		case http.MethodPost:
			var st state.ResumeState
			if err := json.NewDecoder(r.Body).Decode(&st); err != nil {
				http.Error(w, "bad json", http.StatusBadRequest)
				return
			}
			if err := stateStore.Set(key, st); err != nil {
				http.Error(w, "persist failed", http.StatusInternalServerError)
				return
			}
			w.Write([]byte("ok"))
			return

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
	})

	// -------------------------------
	// Minimal Player APIs (Mock)
	// -------------------------------
	http.HandleFunc("/api/play", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	http.HandleFunc("/api/pause", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	http.HandleFunc("/api/next", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	http.HandleFunc("/api/prev", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	http.HandleFunc("/api/seek", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	http.HandleFunc("/api/volume", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })

	// Static
	http.Handle("/", http.FileServer(http.Dir("webui/static")))

	log.Println("mupibox läuft auf http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func itoa(n int) string {
	// minimal, ohne strconv import
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var b [32]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + (n % 10))
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
