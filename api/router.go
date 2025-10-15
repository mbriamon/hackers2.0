package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type GameStatus string

const (
	StatusPre  GameStatus = "PreGame"
	StatusDone GameStatus = "Settled"
)

type Selection string

const (
	SelHome Selection = "home"
	SelAway Selection = "away"
	SelDraw Selection = "draw"
)

type Game struct {
	ID        int64      `json:"id"`
	Sport     string     `json:"sport"`
	Home      string     `json:"home"`
	Away      string     `json:"away"`
	StartTime string     `json:"start_time"`
	Status    GameStatus `json:"status"`
	Result    *Selection `json:"result,omitempty"`

	HomePool int64   `json:"home_pool_tokens"`
	AwayPool int64   `json:"away_pool_tokens"`
	DrawPool int64   `json:"draw_pool_tokens"`
	HomeOdds float64 `json:"home_odds"`
	AwayOdds float64 `json:"away_odds"`
	DrawOdds float64 `json:"draw_odds"`
}

type Bet struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	GameID    int64     `json:"game_id"`
	Selection Selection `json:"selection"`
	Stake     int64     `json:"stake_tokens"`
	PlacedAt  string    `json:"placed_at"`
}

type Wallet struct {
	UserID  int64 `json:"user_id"`
	Balance int64 `json:"tokens_balance"`
}

type store struct {
	mu       sync.Mutex
	games    map[int64]*Game
	bets     map[int64]*Bet
	wallets  map[int64]*Wallet
	nextBet  int64
	adminKey string
}

func newStore() *store {
	s := &store{
		games:    map[int64]*Game{},
		bets:     map[int64]*Bet{},
		wallets:  map[int64]*Wallet{},
		nextBet:  1,
		adminKey: "letmein",
	}
	now := time.Now().Add(30 * time.Minute).Format(time.RFC3339)

	s.wallets[1] = &Wallet{UserID: 1, Balance: 1000}

	s.games[101] = &Game{
		ID:        101,
		Sport:     "Flag Football",
		Home:      "Welsh Fam Whirls",
		Away:      "Lewis Chicks",
		StartTime: now,
		Status:    StatusPre,
		HomePool:  100, AwayPool: 100, DrawPool: 0,
	}
	s.games[102] = &Game{
		ID:        102,
		Sport:     "Soccer",
		Home:      "Alumni",
		Away:      "Dillon",
		StartTime: time.Now().Add(90 * time.Minute).Format(time.RFC3339),
		Status:    StatusPre,
		HomePool:  150, AwayPool: 120, DrawPool: 30,
	}
	s.games[103] = &Game{
		ID:        103,
		Sport:     "Volleyball",
		Home:      "Cat Food",
		Away:      "Kiss My Ace",
		StartTime: time.Now().Add(90 * time.Minute).Format(time.RFC3339),
		Status:    StatusPre,
		HomePool:  150, AwayPool: 120, DrawPool: 30,
	}
	return s
}

func (s *store) listGames() []*Game {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Game, 0, len(s.games))
	for _, g := range s.games {
		copy := *g
		addOdds(&copy)
		out = append(out, &copy)
	}
	return out
}

func (s *store) getGame(id int64) (*Game, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.games[id]
	if !ok {
		return nil, false
	}
	copy := *g
	addOdds(&copy)
	return &copy, true
}

func (s *store) placeBet(userID, gameID int64, sel Selection, stake int64) (*Bet, *Wallet, *Game, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	w, ok := s.wallets[userID]
	if !ok {
		return nil, nil, nil, fmt.Errorf("user_not_found")
	}
	if stake <= 0 {
		return nil, nil, nil, fmt.Errorf("bad_stake")
	}
	if w.Balance < stake {
		return nil, nil, nil, fmt.Errorf("insufficient_balance")
	}
	g, ok := s.games[gameID]
	if !ok {
		return nil, nil, nil, fmt.Errorf("game_not_found")
	}
	if g.Status == StatusDone {
		return nil, nil, nil, fmt.Errorf("game_settled")
	}

	w.Balance -= stake
	switch sel {
	case SelHome:
		g.HomePool += stake
	case SelAway:
		g.AwayPool += stake
	case SelDraw:
		g.DrawPool += stake
	default:
		return nil, nil, nil, fmt.Errorf("bad_selection")
	}

	b := &Bet{
		ID:        s.nextBet,
		UserID:    userID,
		GameID:    gameID,
		Selection: sel,
		Stake:     stake,
		PlacedAt:  time.Now().Format(time.RFC3339),
	}
	s.bets[b.ID] = b
	s.nextBet++

	return b, w, g, nil
}

func (s *store) settle(adminKey string, gameID int64, result Selection) (*Game, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if adminKey != s.adminKey {
		return nil, fmt.Errorf("forbidden")
	}
	g, ok := s.games[gameID]
	if !ok {
		return nil, fmt.Errorf("game_not_found")
	}
	if g.Status == StatusDone {
		return nil, fmt.Errorf("already_settled")
	}

	g.Status = StatusDone
	g.Result = &result

	total := g.HomePool + g.AwayPool + g.DrawPool
	var winnerPool int64
	switch result {
	case SelHome:
		winnerPool = g.HomePool
	case SelAway:
		winnerPool = g.AwayPool
	case SelDraw:
		winnerPool = g.DrawPool
	}
	if winnerPool == 0 {
		return g, nil
	}
	for _, b := range s.bets {
		if b.GameID != gameID {
			continue
		}
		if b.Selection == result {
			share := float64(b.Stake) / float64(winnerPool)
			payout := int64(share * float64(total))
			w := s.wallets[b.UserID]
			w.Balance += payout
		}
	}
	return g, nil
}

func addOdds(g *Game) {
	total := float64(g.HomePool + g.AwayPool + g.DrawPool)
	if total <= 0 {
		g.HomeOdds, g.AwayOdds, g.DrawOdds = 0, 0, 0
		return
	}
	g.HomeOdds = float64(g.HomePool) / total
	g.AwayOdds = float64(g.AwayPool) / total
	g.DrawOdds = float64(g.DrawPool) / total
}

var st = newStore()

// ---------------- Vercel entry (single function) ----------------

func Handler(w http.ResponseWriter, r *http.Request) {
	// CORS + dispatch using the original path passed via rewrite (?path=...)
	allowCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Query().Get("path"), "/") // e.g., "games", "games/101/bets"
		switch {
		case r.Method == http.MethodGet && (rel == "games" || rel == "games/"):
			handleGames(w, r)
			return

		case strings.HasPrefix(rel, "games/"):
			rest := strings.TrimPrefix(rel, "games/")
			// handleGameByID expects URL.Path like /api/games/<rest>
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/api/games/" + rest
			handleGameByID(w, r2)
			return

		default:
			http.NotFound(w, r)
			return
		}
	})).ServeHTTP(w, r)
}

// ---------------- helpers & handlers ----------------

func allowCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Admin-Key")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func handleGames(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, st.listGames())
		return
	}
	http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
}

func handleGameByID(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/games/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "bad_id", http.StatusBadRequest)
		return
	}

	if len(parts) == 1 && r.Method == http.MethodGet {
		g, ok := st.getGame(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, g)
		return
	}

	if len(parts) == 2 && parts[1] == "bets" && r.Method == http.MethodPost {
		var body struct {
			UserID    int64     `json:"user_id"`
			Selection Selection `json:"selection"`
			Stake     int64     `json:"stake"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad_json", http.StatusBadRequest)
			return
		}
		b, wlt, g, err := st.placeBet(body.UserID, id, body.Selection, body.Stake)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// >>> CHANGE #1: compute fresh odds in the response
		gc := *g
		addOdds(&gc)
		writeJSON(w, http.StatusOK, map[string]any{"bet": b, "wallet": wlt, "game": &gc})
		return
	}

	if len(parts) == 2 && parts[1] == "settle" && r.Method == http.MethodPost {
		var body struct {
			Result Selection `json:"result"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad_json", http.StatusBadRequest)
			return
		}
		key := r.Header.Get("X-Admin-Key")
		g, err := st.settle(key, id, body.Result)
		if err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}

		// >>> CHANGE #2: compute fresh odds in the response
		gc := *g
		addOdds(&gc)
		writeJSON(w, http.StatusOK, &gc)
		return
	}

	http.Error(w, "not_found", http.StatusNotFound)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
