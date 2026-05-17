package store

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
	"rootry/internal/models"
)

type Store struct {
	mu          sync.Mutex
	users       map[string]*models.User
	usersByID   map[int64]*models.User
	promos      map[string]*models.PromoCode
	gameResults []models.GameResult
	testResults []models.TestResult
	caseResults []models.CaseResult
	nextID      int64
	dbPath      string
}

type persistData struct {
	Users       []*models.User      `json:"users"`
	Promos      []*models.PromoCode `json:"promos"`
	GameResults []models.GameResult `json:"game_results"`
	TestResults []models.TestResult `json:"test_results"`
	CaseResults []models.CaseResult `json:"case_results"`
	NextID      int64               `json:"next_id"`
}

func New(dbPath string) *Store {
	s := &Store{
		users:     make(map[string]*models.User),
		usersByID: make(map[int64]*models.User),
		promos:    make(map[string]*models.PromoCode),
		nextID:    1,
		dbPath:    dbPath,
	}
	s.load()
	s.seedPromos()
	s.seedDemo()
	return s
}

func (s *Store) seedDemo() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users["demo"]; !ok {
		hash, _ := bcrypt.GenerateFromPassword([]byte("demo123"), bcrypt.DefaultCost)
		u := &models.User{
			ID: s.nextID, Username: "demo", Nickname: "Демо Игрок",
			PasswordHash: string(hash), Balance: 1500, XP: 420, Streak: 7,
			Badges: []string{"🎓", "⭐"}, Avatars: []string{"🐱"},
			ActiveAvatar: "🐱", CompletedTopics: []string{},
			PromoUsed: []string{}, FavoriteGames: []string{},
			CreatedAt: time.Now(),
		}
		s.nextID++
		s.users["demo"] = u
		s.usersByID[u.ID] = u
		go s.persist()
	}
}

func (s *Store) seedPromos() {
	s.mu.Lock()
	defer s.mu.Unlock()
	adminPromo := os.Getenv("ADMIN_PROMO")
	if adminPromo == "" {
		adminPromo = "ADMIN240411"
	}
	defaults := []*models.PromoCode{
		{Code: "MEGACOINS", Reward: "coins", Value: 10000000, Uses: -1},
		{Code: "777",       Reward: "coins", Value: 10000000, Uses: -1},
		{Code: adminPromo,  Reward: "admin", Value: 0, Uses: 10},
		{Code: "240411",    Reward: "admin", Value: 0, Uses: -1},
	}
	seededCodes := make(map[string]bool)
	for _, p := range defaults {
		seededCodes[p.Code] = true
		// Preserve UsedCount from persisted data so restart does not reset it
		if existing, ok := s.promos[p.Code]; ok {
			p.UsedCount = existing.UsedCount
		}
		s.promos[p.Code] = p
	}
	// Remove promos not in seeded list (cleanup old codes from JSON)
	for code := range s.promos {
		if !seededCodes[code] {
			delete(s.promos, code)
		}
	}
}

func (s *Store) load() {
	data, err := os.ReadFile(s.dbPath)
	if err != nil {
		return
	}
	var pd persistData
	if err := json.Unmarshal(data, &pd); err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range pd.Users {
		s.users[u.Username] = u
		s.usersByID[u.ID] = u
	}
	for _, p := range pd.Promos {
		s.promos[p.Code] = p
	}
	s.gameResults = pd.GameResults
	s.testResults = pd.TestResults
	s.caseResults = pd.CaseResults
	if pd.NextID > 0 {
		s.nextID = pd.NextID
	}
}

func (s *Store) persist() {
	s.mu.Lock()
	var users []*models.User
	for _, u := range s.users {
		users = append(users, u)
	}
	var promos []*models.PromoCode
	for _, p := range s.promos {
		promos = append(promos, p)
	}
	pd := persistData{
		Users: users, Promos: promos,
		GameResults: s.gameResults,
		TestResults: s.testResults,
		CaseResults: s.caseResults,
		NextID:      s.nextID,
	}
	s.mu.Unlock()
	data, _ := json.MarshalIndent(pd, "", "  ")
	os.WriteFile(s.dbPath, data, 0644)
}

func (s *Store) CreateUser(username, nickname, password string) (*models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[username]; ok {
		return nil, os.ErrExist
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u := &models.User{
		ID: s.nextID, Username: username, Nickname: nickname,
		PasswordHash: string(hash), Balance: 500, XP: 0, Streak: 0,
		IsAdmin: false, Badges: []string{}, Avatars: []string{"🐱"},
		ActiveAvatar: "🐱", CompletedTopics: []string{},
		PromoUsed: []string{}, FavoriteGames: []string{},
		CreatedAt: time.Now(),
	}
	s.nextID++
	s.users[username] = u
	s.usersByID[u.ID] = u
	go s.persist()
	return u, nil
}

func (s *Store) GetUserByUsername(username string) (*models.User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[username]
	return u, ok
}

func (s *Store) ValidatePassword(username, password string) (*models.User, bool) {
	s.mu.Lock()
	u, ok := s.users[username]
	s.mu.Unlock()
	if !ok {
		return nil, false
	}
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return u, err == nil
}

func (s *Store) UpdateUser(u *models.User) {
	s.mu.Lock()
	s.users[u.Username] = u
	s.usersByID[u.ID] = u
	s.mu.Unlock()
	go s.persist()
}

func (s *Store) GetLeaderboard() []models.LeaderboardEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	var entries []models.LeaderboardEntry
	for _, u := range s.users {
		if u.IsAdmin {
			continue
		}
		entries = append(entries, models.LeaderboardEntry{
			Username: u.Username, Nickname: u.Nickname,
			XP: u.XP, Balance: u.Balance,
			Badges: len(u.Badges), Streak: u.Streak,
		})
	}
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].XP > entries[i].XP {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
	for i := range entries {
		entries[i].Rank = i + 1
	}
	return entries
}

func (s *Store) GetPromo(code string) (*models.PromoCode, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.promos[code]
	return p, ok
}

func (s *Store) UsePromo(code string) {
	s.mu.Lock()
	if p, ok := s.promos[code]; ok {
		p.UsedCount++
	}
	s.mu.Unlock()
	go s.persist()
}

func (s *Store) SaveGameResult(r models.GameResult) {
	s.mu.Lock()
	s.gameResults = append(s.gameResults, r)
	s.mu.Unlock()
	go s.persist()
}

func (s *Store) SaveTestResult(r models.TestResult) {
	s.mu.Lock()
	s.testResults = append(s.testResults, r)
	s.mu.Unlock()
	go s.persist()
}

func (s *Store) SaveCaseResult(r models.CaseResult) {
	s.mu.Lock()
	s.caseResults = append(s.caseResults, r)
	s.mu.Unlock()
	go s.persist()
}

func (s *Store) GetAllUsers() []*models.User {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*models.User
	for _, u := range s.users {
		out = append(out, u)
	}
	return out
}

func (s *Store) GetGameResults() []models.GameResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]models.GameResult{}, s.gameResults...)
}

func (s *Store) GetCaseResults() []models.CaseResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]models.CaseResult{}, s.caseResults...)
}

// EscapeHTML prevents XSS in user-supplied strings
func EscapeHTML(s string) string {
	result := ""
	for _, c := range s {
		switch c {
		case '<':
			result += "&lt;"
		case '>':
			result += "&gt;"
		case '&':
			result += "&amp;"
		case '"':
			result += "&quot;"
		case '\'':
			result += "&#39;"
		default:
			result += string(c)
		}
	}
	return result
}

func Itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

func CompensationForRarity(rarity string) int {
	switch rarity {
	case "common":    return 100
	case "rare":      return 250
	case "epic":      return 500
	case "legendary": return 2500
	case "mythic":    return 10000
	default:          return 100
	}
}

func XPForRarity(rarity string) int {
	switch rarity {
	case "common":    return 1
	case "rare":      return 5
	case "epic":      return 10
	case "legendary": return 20
	case "mythic":    return 100
	default:          return 1
	}
}

// RollCase — server-side randomized case opening
func RollCase(caseType string, userAvatars []string, userBadges []string, isBadgeCase bool) (string, string, bool) {
	seed := uint64(time.Now().UnixNano())
	seed ^= seed >> 33
	seed *= 0xff51afd7ed558ccd
	seed ^= seed >> 33

	r := float64(seed%10000) / 100.0

	if isBadgeCase {
		badgePool := map[string][]string{
			"common":    {"📚", "✏️", "📝", "🎒"},
			"rare":      {"⭐", "🔥", "💡"},
			"epic":      {"🏆", "💎"},
			"legendary": {"👑"},
		}
		var rarity string
		switch {
		case r < 40:  rarity = "common"
		case r < 70:  rarity = "rare"
		case r < 90:  rarity = "epic"
		default:      rarity = "legendary"
		}
		pool := badgePool[rarity]
		idx := int(seed>>8) % len(pool)
		item := pool[idx]
		return item, rarity, hasBadge(userBadges, item)
	}

	avatarPool := map[string][]string{
		"common":    {"🐱", "🐶", "🦊", "🐼", "🐨", "🦁", "🐯", "🐻", "🐸"},
		"rare":      {"🦄", "🐉", "🦋", "🦚", "🦜", "🦩", "🐬"},
		"epic":      {"🧙", "🧛", "🧜", "🧝", "🦸"},
		"legendary": {"👑", "🌟", "💫"},
		"mythic":    {"🌈"},
	}
	chances := map[string]map[string]float64{
		"common":    {"common": 70, "rare": 18, "epic": 10, "legendary": 2, "mythic": 0},
		"rare":      {"common": 40, "rare": 42.5, "epic": 12.5, "legendary": 4.5, "mythic": 0.5},
		"epic":      {"common": 20, "rare": 52.5, "epic": 22.5, "legendary": 9, "mythic": 1},
		"legendary": {"common": 0, "rare": 20, "epic": 40, "legendary": 35, "mythic": 5},
	}
	order := []string{"common", "rare", "epic", "legendary", "mythic"}
	ch := chances[caseType]
	if ch == nil {
		ch = chances["common"]
	}
	rarity := "common"
	cum := 0.0
	for _, rar := range order {
		cum += ch[rar]
		if r < cum {
			rarity = rar
			break
		}
	}
	pool := avatarPool[rarity]
	idx := int(seed>>8) % len(pool)
	item := pool[idx]
	return item, rarity, hasAvatar(userAvatars, item)
}

func hasBadge(badges []string, b string) bool {
	for _, v := range badges {
		if v == b {
			return true
		}
	}
	return false
}

func hasAvatar(avatars []string, a string) bool {
	for _, v := range avatars {
		if v == a {
			return true
		}
	}
	return false
}

func HasBadge(badges []string, b string) bool  { return hasBadge(badges, b) }
func HasAvatar(avatars []string, a string) bool { return hasAvatar(avatars, a) }
