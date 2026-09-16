package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"mupibox/internal/catalog"
	"mupibox/internal/player"
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
	// --------------------------------------------------
	// Load catalog + state
	// --------------------------------------------------
	cat, err := catalog.LoadCatalog("config/catalog.json")
	if err != nil {
		log.Fatal(err)
	}

	stateStore, err := state.NewStore("data/state.json")
	if err != nil {
		log.Fatal(err)
	}

	// --------------------------------------------------
	// Init player (memory mock)
	// --------------------------------------------------
	p := player.NewMemoryPlayer()

	// --------------------------------------------------
	// Helpers
	// --------------------------------------------------
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
		for _, src := range it.Sources {
			if src.CoverPath != "" {
				if _, err := os.Stat("webui/static" + src.CoverPath); err == nil {
					return src.CoverPath
				}
			}
		}
		return "/covers/placeholder.png"
	}

	// --------------------------------------------------
	// HOME API
	// --------------------------------------------------
	http.HandleFunc("/api/home", func(w http.ResponseWriter, r *http.Request) {
		var resp struct {
			Sections []HomeSection `json:"sections"`
		}

		// --- Continue section ---
		recent := stateStore.ListRecent(15)
		if len(recent) > 0 {
			sec := HomeSection{Title: "Weiter abspielen"}
			for _, e := range recent {
				it := findCatalogItem(e.State.ItemID)

				title := e.State.ItemID
				if it != nil {
					title = it.DisplayName
				}

				sec.Items = append(sec.Items, HomeItem{
					ID:          e.Key,
					Title:       title,
					Type:        "continue",
					Image:       pickCover(it),
					CanResume:   true,
					ResumePos:   e.State.PositionSec,
					ResumeLabel: "Weiter",
				})
			}
			resp.Sections = append(resp.Sections, sec)
		}

		// --- Catalog sections ---
		for _, c := range cat.Categories {
			sec := HomeSection{Title: c.Title}
			for _, it := range c.Items {
				canResume := it.Resume
				var resumePos int
				var resumeLabel string

				if canResume {
					if st, ok := stateStore.Get(it.ID); ok && st.PositionSec > 0 {
						resumePos = st.PositionSec
						resumeLabel = "Weiter"
					}
				}

				sec.Items = append(sec.Items, HomeItem{
					ID:          it.ID,
					Title:       it.DisplayName,
					Type:        it.Type,
					Image:       pickCover(&it),
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

	// --------------------------------------------------
	// CONTINUE DETAILS
	// --------------------------------------------------
	http.HandleFunc("/api/continue/", func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/api/continue/")
		st, ok := stateStore.Get(key)
		if !ok {
			http.NotFound(w, r)
			return
		}

		it := findCatalogItem(st.ItemID)
		title := st.ItemID
		if it != nil {
			title = it.DisplayName
		}

		resp := map[string]interface{}{
			"id":           key,
			"title":        title,
			"cover":        pickCover(it),
			"track_index":  st.TrackIndex,
			"position_sec": st.PositionSec,
			"item_id":      st.ItemID,
			"updated_at":   st.UpdatedAt,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	// --------------------------------------------------
	// ARTIST DETAILS (mock)
	// --------------------------------------------------
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
				{"id": id + "_1", "title": "Folge 1", "duration": 3600},
				{"id": id + "_2", "title": "Folge 2", "duration": 3600},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	// --------------------------------------------------
	// PLAYER API (NEU)
	// --------------------------------------------------
	player.NewAPI(p).Register(http.DefaultServeMux)

	// --------------------------------------------------
	// Static UI
	// --------------------------------------------------
	http.Handle("/", http.FileServer(http.Dir("webui/static")))

	log.Println("MuPiBox running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
