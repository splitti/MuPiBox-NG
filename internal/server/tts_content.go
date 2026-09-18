package server

import (
	"context"

	"mupibox/internal/tts"
)

// RegisterAllSpeakableContent registers every currently known, actually
// displayed touch-UI text for TTS: navigation categories/media (NORMAL --
// there are only ever a handful) and local media library folders (LOW --
// there can be thousands). It is the single place that walks the real
// content providers (see CLAUDE.md: only navigation and the local library
// have real import/sync logic today; Spotify/podcasts/webradio are
// provider strings without sync code and are intentionally not touched
// here so this never invents content that does not exist).
//
// Safe to call at any time and any number of times: EnsureText is
// idempotent (a cache hit or an already-queued job is a fast no-op), so
// this both seeds a fresh generation's background content and serves as
// the "fill missing" admin action against the current one. If TTS is
// unavailable or not configured yet, it is a silent no-op -- registering
// content must never depend on or block on TTS (see docs/tts.md).
func (a *API) RegisterAllSpeakableContent(ctx context.Context) error {
	if a.TTSManager == nil || a.Store == nil {
		return nil
	}
	if _, ok, err := a.Store.CurrentTTSGeneration(); err != nil || !ok {
		return nil
	}
	a.registerNavigationSpeech(ctx)
	a.registerLibrarySpeech(ctx)
	return nil
}

func (a *API) registerNavigationSpeech(ctx context.Context) {
	nav, err := a.Store.LoadNavigation()
	if err != nil {
		return
	}
	for _, category := range nav.Categories {
		if ctx.Err() != nil {
			return
		}
		for _, label := range category.Labels {
			_ = a.TTSManager.EnsureText(ctx, "category", category.ID, label, tts.PriorityNormal)
		}
		for _, row := range category.Rows {
			for _, label := range row.Labels {
				_ = a.TTSManager.EnsureText(ctx, "media", row.ID, label, tts.PriorityNormal)
			}
		}
	}
}

func (a *API) registerLibrarySpeech(ctx context.Context) {
	lib := a.librarySnapshot()
	if lib == nil {
		return
	}
	for _, folder := range lib.Folders {
		// A server shutdown (see API.Shutdown) cancels this so a rescan of
		// a very large library never keeps a goroutine running after the
		// store it writes to has closed.
		if ctx.Err() != nil {
			return
		}
		_ = a.TTSManager.EnsureText(ctx, "library-folder", folder.ID, folder.Name, tts.PriorityLow)
	}
}
