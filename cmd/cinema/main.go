// Command cinema is a small media server for movies and TV shows.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"cinema/internal/config"
	"cinema/internal/db"
	"cinema/internal/images"
	"cinema/internal/scanner"
	"cinema/internal/server"
	"cinema/internal/subtitles"
	"cinema/internal/tmdb"
	"cinema/internal/transcode"
	"cinema/web"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer database.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client := tmdb.New(cfg.TMDBToken, cfg.TMDBLanguage)
	imgCache := images.New(cfg.CacheDir, database, client)
	// HLS transcoding for MKVs and subtitles, when ffmpeg is available. The
	// scanner indexes each file (subtitle tracks, and keyframes for MKVs) and
	// extracts built-in subtitles, so playback can start straight away.
	var media server.Media
	var indexer scanner.Indexer
	var extractor scanner.SubtitleExtractor
	if tc, err := transcode.New(cfg.FFmpegPath, cfg.FFprobePath); err != nil {
		log.Printf("transcoding and subtitles disabled, files will be served as-is: %v", err)
	} else {
		media.Store = transcode.NewStore(database, tc)
		indexer = media.Store
		if media.HLS, err = transcode.NewHLS(media.Store, tc, filepath.Join(cfg.CacheDir, "hls")); err != nil {
			log.Fatalf("hls: %v", err)
		}
		defer media.HLS.Close()
		media.Subtitles, err = subtitles.New(tc.FFmpeg(), filepath.Join(cfg.CacheDir, "subtitles"), cfg.SubtitleLanguages, media.Store)
		if err != nil {
			log.Fatalf("SUBTITLE_LANGUAGES: %v", err)
		}
		extractor = media.Subtitles
		defer media.Subtitles.Close()
	}
	sc := scanner.New(ctx, database, client, imgCache, indexer, extractor, cfg.MoviesDir, cfg.TVDir)
	srv := server.New(database, sc, imgCache, web.Dist(), media)

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		// No WriteTimeout: video streams are long-lived responses.
	}

	go func() {
		log.Printf("cinema listening on http://localhost:%s (movies: %s, tv: %s)", cfg.Port, cfg.MoviesDir, cfg.TVDir)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	log.Print("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	httpServer.Shutdown(shutdownCtx)
}
