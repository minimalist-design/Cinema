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
	"cinema/internal/settings"
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

	// Settings saved in the UI take precedence over the environment.
	store, err := settings.Open(ctx, database, settings.Values{
		TMDBToken:         cfg.TMDBToken,
		TMDBLanguage:      cfg.TMDBLanguage,
		SubtitleLanguages: cfg.SubtitleLanguages,
		FFmpegPath:        cfg.FFmpegPath,
		FFprobePath:       cfg.FFprobePath,
	})
	if err != nil {
		log.Fatalf("load settings: %v", err)
	}
	if err := store.SeedFolders(ctx, cfg.MoviesDir, cfg.TVDir); err != nil {
		log.Fatalf("add library folders from environment: %v", err)
	}
	conf := store.Values()
	if conf.TMDBToken == "" {
		log.Print("no TMDB token set: add one in Settings before scanning")
	}

	client := tmdb.New(conf.TMDBToken, conf.TMDBLanguage)
	imgCache := images.New(cfg.CacheDir, database, client)
	// HLS transcoding for MKVs and subtitles, when ffmpeg is available. The
	// scanner indexes each file (subtitle tracks, and keyframes for MKVs) and
	// extracts built-in subtitles, so playback can start straight away.
	var media server.Media
	var indexer scanner.Indexer
	var extractor scanner.SubtitleExtractor
	var mediaErr string
	if tc, err := transcode.New(conf.FFmpegPath, conf.FFprobePath); err != nil {
		mediaErr = err.Error()
		log.Printf("transcoding and subtitles disabled, files will be served as-is: %v", err)
	} else {
		media.Store = transcode.NewStore(database, tc)
		indexer = media.Store
		if media.HLS, err = transcode.NewHLS(media.Store, tc, filepath.Join(cfg.CacheDir, "hls")); err != nil {
			log.Fatalf("hls: %v", err)
		}
		defer media.HLS.Close()
		media.Subtitles, err = subtitles.New(tc.FFmpeg(), filepath.Join(cfg.CacheDir, "subtitles"), conf.SubtitleLanguages, media.Store)
		if err != nil {
			log.Fatalf("subtitle languages: %v", err)
		}
		extractor = media.Subtitles
		defer media.Subtitles.Close()
	}
	// Apply changed settings right away; ffmpeg paths need a restart.
	store.OnChange(func(v settings.Values) {
		client.SetToken(v.TMDBToken)
		client.SetLanguage(v.TMDBLanguage)
		if media.Subtitles != nil {
			if err := media.Subtitles.SetPreferences(v.SubtitleLanguages); err != nil {
				log.Printf("subtitle languages: %v", err)
			}
		}
	})

	sc := scanner.New(ctx, database, client, imgCache, indexer, extractor, store)
	srv := server.New(database, sc, imgCache, web.Dist(), media)
	srv.Admin = server.Admin{
		Settings: store,
		TMDB:     client,
		Info: server.Info{
			Port: cfg.Port, DBPath: cfg.DBPath, CacheDir: cfg.CacheDir,
			FFmpegPath: conf.FFmpegPath, FFprobePath: conf.FFprobePath, MediaError: mediaErr,
		},
	}

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		// No WriteTimeout: video streams are long-lived responses.
	}

	go func() {
		log.Printf("cinema listening on http://localhost:%s", cfg.Port)
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
