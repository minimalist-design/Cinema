// Command cinema is a small media server for movies and TV shows.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cinema/internal/config"
	"cinema/internal/db"
	"cinema/internal/images"
	"cinema/internal/scanner"
	"cinema/internal/server"
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
	sc := scanner.New(ctx, database, client, imgCache, cfg.MoviesDir, cfg.TVDir)
	tc, err := transcode.New(cfg.FFmpegPath, cfg.FFprobePath)
	if err != nil {
		log.Printf("transcoding disabled, MKV files will be served as-is: %v", err)
		tc = nil
	}
	srv := server.New(database, sc, imgCache, web.Dist(), tc)

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
