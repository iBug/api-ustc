package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/iBug/api-ustc/csgo"
	"github.com/iBug/api-ustc/factorio"
	"github.com/iBug/api-ustc/ibugauth"
	"github.com/iBug/api-ustc/minecraft"
	"github.com/iBug/api-ustc/teamspeak"
	"github.com/iBug/api-ustc/ustc"
)

type Config struct {
	Teamspeak  teamspeak.TeamspeakConfig `json:"teamspeak"`
	UstcTokens []string                  `json:"ustc-tokens"`
	WgPubkey   string                    `json:"wg-pubkey"`
}

var (
	listenAddr  string
	csgologAddr string

	config Config
)

func LogRequest(r *http.Request) {
	remoteAddr := r.Header.Get("CF-Connecting-IP")
	if remoteAddr == "" {
		remoteAddr = "(local)"
	}
	log.Printf("%s %q from %s\n", r.Method, r.URL.Path, remoteAddr)
}

func LoadConfig() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configFile := filepath.Join(homeDir, ".config", "api-ustc.json")
	f, err := os.Open(configFile)
	if err != nil {
		return err
	}
	defer f.Close()

	err = json.NewDecoder(f).Decode(&config)
	if err != nil {
		return err
	}
	return nil
}

var mainMux = http.NewServeMux()

func main() {
	flag.StringVar(&listenAddr, "l", ":8000", "listen address")
	flag.StringVar(&csgologAddr, "csgolog", "", "CS:GO log listen address")
	flag.Parse()

	// $JOURNAL_STREAM is set by systemd v231+
	if _, ok := os.LookupEnv("JOURNAL_STREAM"); ok {
		log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))
	}

	if err := LoadConfig(); err != nil {
		log.Fatal(err)
	}

	if csgologAddr != "" {
		csgo.StartCsgoLogServer(csgologAddr)
	}

	// Reload config on SIGHUP
	signalC := make(chan os.Signal, 1)
	signal.Notify(signalC, syscall.SIGHUP)
	go func() {
		for range signalC {
			if err := LoadConfig(); err != nil {
				log.Printf("Error reloading config: %v", err)
			} else {
				log.Printf("Config reloaded!")
			}
		}
	}()

	mainMux.HandleFunc("/csgo", csgo.Handle206Csgo)
	mainMux.HandleFunc("/minecraft", minecraft.Handle206Minecraft)
	mainMux.HandleFunc("/factorio", factorio.Handle206Factorio)
	mainMux.HandleFunc("/teamspeak", teamspeak.HandleTeamspeakOnline)
	mainMux.HandleFunc("/206ip", Handle206IP)
	mainMux.HandleFunc("/ibug-auth", ibugauth.HandleIBugAuth)
	mainMux.HandleFunc("/ustc-id", ustc.HandleUstcId)
	mainMux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "User-Agent: *\nDisallow: /\n")
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		LogRequest(r)
		w.Header().Set("X-Robots-Tag", "noindex")
		mainMux.ServeHTTP(w, r)
	})
	log.Fatal(http.ListenAndServe(listenAddr, mux))
}
