package models

import "time"

type User struct {
	ID              int64     `json:"id"`
	Username        string    `json:"username"`
	Nickname        string    `json:"nickname"`
	PasswordHash    string    `json:"-"`
	Balance         int       `json:"balance"`
	XP              int       `json:"xp"`
	Streak          int       `json:"streak"`
	LastLogin       string    `json:"last_login"`
	LastNickChange  string    `json:"last_nick_change"`
	IsAdmin         bool      `json:"is_admin"`
	Badges          []string  `json:"badges"`
	Avatars         []string  `json:"avatars"`
	ActiveAvatar    string    `json:"active_avatar"`
	CompletedTopics []string  `json:"completed_topics"`
	PromoUsed       []string  `json:"promo_used"`
	FavoriteGames   []string  `json:"favorite_games"`
	DailyTasksDate  string    `json:"daily_tasks_date"`
	DailyTasksDone  int       `json:"daily_tasks_done"`
	GamesWonToday   int       `json:"games_won_today"`
	GamesWonTypes   []string  `json:"games_won_types"`
	LastDailyClaim  string    `json:"last_daily_claim"`
	CreatedAt       time.Time `json:"created_at"`

	// КСПОЯ fields (B5)
	KspoяBanUntil     string   `json:"kspoя_ban_until"`
	KspoяViolations   int      `json:"kspoя_violations"`
	KspoяLastAttempt  string   `json:"kspoя_last_attempt"`
	KspoяBadges       []string `json:"kspoя_badges"`
	ActiveKspoяBadge  string   `json:"active_kspoя_badge"`
}

type LeaderboardEntry struct {
	Rank     int    `json:"rank"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	XP       int    `json:"xp"`
	Balance  int    `json:"balance"`
	Badges   int    `json:"badges_count"`
	Streak   int    `json:"streak"`
}

type PromoCode struct {
	Code       string `json:"code"`
	Reward     string `json:"reward"` // "coins", "badge", "avatar", "xp"
	Value      int    `json:"value"`
	BadgeName  string `json:"badge_name"`
	AvatarName string `json:"avatar_name"`
	Uses       int    `json:"uses"`      // -1 = unlimited
	UsedCount  int    `json:"used_count"`
}

type GameResult struct {
	UserID      int64  `json:"user_id"`
	GameType    string `json:"game_type"`
	Score       int    `json:"score"`
	XPEarned    int    `json:"xp_earned"`
	CoinsEarned int    `json:"coins_earned"`
	PlayedAt    string `json:"played_at"`
}

type TestResult struct {
	UserID      int64  `json:"user_id"`
	Score       int    `json:"score"`
	Passed      bool   `json:"passed"`
	Level       string `json:"level"`
	BadgeEarned string `json:"badge_earned"`
	PlayedAt    string `json:"played_at"`
}

type CaseResult struct {
	UserID       int64  `json:"user_id"`
	CaseType     string `json:"case_type"`
	ItemEmoji    string `json:"item_emoji"`
	ItemRarity   string `json:"item_rarity"`
	IsDuplicate  bool   `json:"is_duplicate"`
	Compensation int    `json:"compensation"`
	PlayedAt     string `json:"played_at"`
}

// ── КСПОЯ Models ──────────────────────────────────────────────────────────────

// KspoяSession — активная или завершённая сессия теста
type KspoяSession struct {
	ID          string    `json:"id"`
	UserID      int64     `json:"user_id"`
	Username    string    `json:"username"`
	QuestionIDs []int     `json:"question_ids"` // 40 индексов из пула
	StartedAt   time.Time `json:"started_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	Status      string    `json:"status"` // active | completed | aborted
	Score       int       `json:"score"`
	LevelKey    string    `json:"level_key"`
	FinishedAt  time.Time `json:"finished_at"`
}

// KspoяQuestion — один вопрос из банка (не передаём correct_idx клиенту в сессии)
type KspoяQuestion struct {
	ID         int      `json:"id"`
	Question   string   `json:"question"`
	Options    []string `json:"options"`
	CorrectIdx int      `json:"-"` // скрыто от клиента
	Category   string   `json:"category"`
}

// KspoяQuestionClient — версия без ответа для фронта
type KspoяQuestionClient struct {
	ID       int      `json:"id"`
	Question string   `json:"question"`
	Options  []string `json:"options"`
	Category string   `json:"category"`
}

// KspoяResult — запись в истории
type KspoяResult struct {
	UserID     int64     `json:"user_id"`
	Username   string    `json:"username"`
	Score      int       `json:"score"`
	LevelKey   string    `json:"level_key"`
	CoinsGiven int       `json:"coins_given"`
	XPGiven    int       `json:"xp_given"`
	FinishedAt time.Time `json:"finished_at"`
}

// API request/response types
type RegisterRequest struct {
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type PromoRequest struct {
	Code string `json:"code"`
}

type PromoResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Reward  string `json:"reward"`
	Value   int    `json:"value"`
}

type GameSubmitRequest struct {
	GameType string `json:"game_type"`
	Score    int    `json:"score"`
}

type TestSubmitRequest struct {
	Answers []int `json:"answers"`
}

type CaseOpenRequest struct {
	CaseType   string `json:"case_type"`
	Price      int    `json:"price"`
	ItemEmoji  string `json:"item_emoji"`
	ItemRarity string `json:"item_rarity"`
}

// KspoяSubmitRequest — ответы на 40 вопросов (индексы выбранных вариантов)
type KspoяSubmitRequest struct {
	Answers []int `json:"answers"` // len = 40, значения 0-3 (или -1 = не ответил)
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type SuccessResponse struct {
	Message string `json:"message"`
}
