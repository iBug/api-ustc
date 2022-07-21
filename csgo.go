package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

type CsgoStatus struct {
	Map         string   `json:"map"`
	GameMode    string   `json:"game_mode"`
	PlayerCount int      `json:"player_count"`
	BotCount    int      `json:"bot_count"`
	Players     []string `json:"players"`

	cvar_GameMode int
	cvar_GameType int
}

var re_players = *regexp.MustCompile(`(\d+) humans?, (\d+) bots?`)
var GAME_MODE_S = map[int]string{
	0:   "casual",
	1:   "competitive",
	2:   "scrim competitive",
	100: "arms race",
	101: "demolition",
	102: "deathmatch",
	200: "training",
	300: "custom",
	400: "cooperative",
	500: "skirmish",
}

func (s *CsgoStatus) ParseGameMode() string {
	// Source: https://totalcsgo.com/command/gamemode
	id := s.cvar_GameType*100 + s.cvar_GameMode
	str, ok := GAME_MODE_S[id]
	if ok {
		return str
	}
	return "unknown"
}

func GetCsgoStatus() (CsgoStatus, error) {
	res, err := http.Post("http://10.255.0.9:8001/api/exec/",
		"application/json",
		bytes.NewBufferString(`{"cmd": "cvarlist game_; status"}`))
	if err != nil {
		return CsgoStatus{}, err
	}
	defer res.Body.Close()

	status := CsgoStatus{Players: make([]string, 0, 10)}
	scanner := bufio.NewScanner(res.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 {
			continue
		}
		if !strings.HasPrefix(line, "#") {
			items := strings.SplitN(line, ": ", 3)
			if len(items) < 2 {
				continue
			}
			key := strings.TrimSpace(items[0])
			value := strings.TrimSpace(items[1])
			switch key {
			case "map":
				status.Map = value
			case "players":
				matches := re_players.FindStringSubmatch(value)
				status.PlayerCount, _ = strconv.Atoi(matches[1])
				status.BotCount, _ = strconv.Atoi(matches[2])
			case "game_mode":
				status.cvar_GameMode, _ = strconv.Atoi(value)
			case "game_type":
				status.cvar_GameType, _ = strconv.Atoi(value)
			}
		} else {
			parts := strings.SplitN(line, "\"", 3)
			if len(parts) != 3 {
				continue
			}
			moreInfo := strings.Split(strings.TrimSpace(parts[2]), " ")
			if moreInfo[0] == "BOT" {
				continue
			}
			status.Players = append(status.Players, parts[1])
		}
	}
	status.GameMode = status.ParseGameMode()
	return status, nil
}

func Handle206Csgo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	status, err := GetCsgoStatus()
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `{"status": "internal server error}`)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}
