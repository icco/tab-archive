package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"contrib.go.opencensus.io/exporter/stackdriver"
	"contrib.go.opencensus.io/exporter/stackdriver/monitoredresource"
	"contrib.go.opencensus.io/exporter/stackdriver/propagation"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
	"github.com/icco/tab-archive/lib"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"

	_ "github.com/lib/pq"
)

var (
	log = lib.InitLogging()
)

func main() {
	port := "8080"
	if fromEnv := os.Getenv("PORT"); fromEnv != "" {
		port = fromEnv
	}
	log.Infof("Starting up on http://localhost:%s", port)

	if os.Getenv("ENABLE_STACKDRIVER") != "" {
		labels := &stackdriver.Labels{}
		labels.Set("app", "tab-archive", "The name of the current app.")
		sd, err := stackdriver.NewExporter(stackdriver.Options{
			ProjectID:               "icco-cloud",
			MonitoredResource:       monitoredresource.Autodetect(),
			DefaultMonitoringLabels: labels,
			DefaultTraceAttributes:  map[string]interface{}{"app": "tab-archive"},
		})

		if err != nil {
			log.WithError(err).Fatalf("failed to create the stackdriver exporter")
		}
		defer sd.Flush()

		view.RegisterExporter(sd)
		trace.RegisterExporter(sd)
		trace.ApplyConfig(trace.Config{
			DefaultSampler: trace.AlwaysSample(),
		})
	}

	db, err := lib.InitDB(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("cannot connect to database server: %+v", err)
	}
	defer db.Close()

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(lib.LoggingMiddleware())

	crs := cors.New(cors.Options{
		AllowCredentials:   true,
		OptionsPassthrough: false,
		AllowedOrigins:     []string{"*"},
		AllowedMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:     []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:     []string{"Link"},
		MaxAge:             300, // Maximum value not ignored by any of major browsers
	})
	r.Use(crs.Handler)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.Must(template.ParseFiles("index.tmpl"))
		if err := tmpl.Execute(w, nil); err != nil {
			log.Fatalf("template execution: %s", err)
		}
	})

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hi."))
	})

	r.Post("/hook", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		tok := r.Header.Get("Authorization")
		if tok == "" {
			err := fmt.Errorf("no Authorization header")
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		tok = strings.TrimPrefix(tok, "Bearer ")
		u, err := lib.GetUser(ctx, db, tok)
		if err != nil {
			log.WithError(err).Error("could not get user")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		buf, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.WithError(err).Error("could not read buffer")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		ct := r.Header.Get("content-type")
		if ct != "application/json" {
			http.Error(w, "expected 'application/json' content type", http.StatusBadRequest)
			return
		}

		if err := lib.ParseAndStore(ctx, db, u, buf); err != nil {
			log.WithError(err).Error("could not parse request")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Write([]byte(`{"status": "success"}`))
	})

	r.Get("/archive", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		j := json.NewEncoder(w)

		tok := r.Header.Get("Authorization")
		if tok == "" {
			err := fmt.Errorf("no Authorization header")
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		tok = strings.TrimPrefix(tok, "Bearer ")
		u, err := lib.GetUser(ctx, db, tok)
		if err != nil {
			log.WithError(err).Error("could not get user")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tabs, err := u.GetArchive(ctx, db)
		if err != nil {
			log.WithError(err).Error("could not get user tabs")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := j.Encode(map[string]interface{}{
			"status": "success",
			"tabs":   tabs,
		}); err != nil {
			log.WithError(err).Error("could not marshal data")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	h := &ochttp.Handler{
		Handler:     r,
		Propagation: &propagation.HTTPFormat{},
	}
	if err := view.Register([]*view.View{
		ochttp.ServerRequestCountView,
		ochttp.ServerResponseCountByStatusCode,
	}...); err != nil {
		log.WithError(err).Fatal("Failed to register ochttp views")
	}

	log.Fatal(http.ListenAndServe(":"+port, h))
}
