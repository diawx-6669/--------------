package handlers

// ══════════════════════════════════════════════════════════════
//  internal/handlers/kspoя.go
//  КСПОЯ — Казахстанская система проверки оценивания языка
// ══════════════════════════════════════════════════════════════
//
//  Подключи в main.go:
//    mux.HandleFunc("/api/kspoя/start",   middleware.AuthMiddleware(h.KspoяStart))
//    mux.HandleFunc("/api/kspoя/submit",  middleware.AuthMiddleware(h.KspoяSubmit))
//    mux.HandleFunc("/api/kspoя/cancel",  middleware.AuthMiddleware(h.KspoяCancel))
//    mux.HandleFunc("/api/kspoя/status",  middleware.AuthMiddleware(h.KspoяStatus))
// ══════════════════════════════════════════════════════════════

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"time"
)

// ─── уровни ────────────────────────────────────────────────────
type kspoяLevel struct {
	Label  string
	Rarity string
	Coins  int
	XP     int
	Badge  string // код значка в таблице badges
}

var kspoяLevels = map[string]kspoяLevel{
	"C2": {Label: "C2 — Мастерство",    Rarity: "rainbow",   Coins: 5000, XP: 2000, Badge: "kspoя_c2"},
	"C1": {Label: "C1 — Продвинутый",   Rarity: "mythic",    Coins: 3000, XP: 1500, Badge: "kspoя_c1"},
	"B2": {Label: "B2 — Выше среднего", Rarity: "legendary", Coins: 1000, XP: 1000, Badge: "kspoя_b2"},
	"B1": {Label: "B1 — Средний",       Rarity: "epic",      Coins: 500,  XP: 750,  Badge: "kspoя_b1"},
	"A2": {Label: "A2 — Элементарный",  Rarity: "rare",      Coins: 250,  XP: 300,  Badge: "kspoя_a2"},
	"A1": {Label: "A1 — Начальный",     Rarity: "common",    Coins: 100,  XP: 0,    Badge: "kspoя_a1"},
}

func scoreToLevel(score int) string {
	switch {
	case score >= 38:
		return "C2"
	case score >= 32:
		return "C1"
	case score >= 25:
		return "B2"
	case score >= 15:
		return "B1"
	case score >= 5:
		return "A2"
	default:
		return "A1"
	}
}

// ─── KspoяStart — POST /api/kspoя/start ───────────────────────
// Создаёт новую сессию: выбирает 40 случайных вопросов из 100.
func (h *Handler) KspoяStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	username := r.Context().Value("username").(string)

	// Проверяем нет ли уже активной сессии
	var activeID string
	row := h.db.QueryRow(`
		SELECT id FROM kspoя_sessions
		WHERE user_id = $1 AND status = 'active' AND expires_at > now()
	`, username)
	if err := row.Scan(&activeID); err == nil {
		// Активная сессия есть — отдаём её
		sendActiveSession(w, h, username, activeID)
		return
	}

	// Получаем 100 ID вопросов и перемешиваем
	rows, err := h.db.Query(`SELECT id FROM kspoя_questions ORDER BY id`)
	if err != nil {
		jsonError(w, "Ошибка базы данных", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var allIDs []int
	for rows.Next() {
		var id int
		rows.Scan(&id)
		allIDs = append(allIDs, id)
	}
	if len(allIDs) < 40 {
		jsonError(w, "Недостаточно вопросов в базе", http.StatusInternalServerError)
		return
	}

	// Fisher-Yates перемешивание
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	rng.Shuffle(len(allIDs), func(i, j int) { allIDs[i], allIDs[j] = allIDs[j], allIDs[i] })
	chosen := allIDs[:40]

	// Создаём сессию
	expiresAt := time.Now().Add(40 * time.Minute)
	var sessionID string
	err = h.db.QueryRow(`
		INSERT INTO kspoя_sessions (user_id, question_ids, expires_at, status)
		VALUES ($1, $2, $3, 'active')
		ON CONFLICT DO NOTHING
		RETURNING id
	`, username, intSliceToPgArray(chosen), expiresAt).Scan(&sessionID)
	if err != nil {
		jsonError(w, "Не удалось создать сессию", http.StatusInternalServerError)
		return
	}

	// Загружаем вопросы
	questions, err := loadQuestionsByIDs(h, chosen)
	if err != nil {
		jsonError(w, "Ошибка загрузки вопросов", http.StatusInternalServerError)
		return
	}

	jsonOK(w, map[string]interface{}{
		"session_id":  sessionID,
		"expires_at":  expiresAt,
		"questions":   questions,
	})
}

// ─── KspoяSubmit — POST /api/kspoя/submit ─────────────────────
type submitRequest struct {
	SessionID string `json:"session_id"`
	Answers   []int  `json:"answers"` // 40 ответов [0-3], -1 = пропущен
}

func (h *Handler) KspoяSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	username := r.Context().Value("username").(string)

	var req submitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Неверный запрос", http.StatusBadRequest)
		return
	}
	if len(req.Answers) != 40 {
		jsonError(w, "Ожидается 40 ответов", http.StatusBadRequest)
		return
	}

	// Получаем активную сессию
	var questionIDs []int64
	err := h.db.QueryRow(`
		SELECT question_ids FROM kspoя_sessions
		WHERE id = $1 AND user_id = $2 AND status = 'active' AND expires_at > now()
	`, req.SessionID, username).Scan((*pq.Int64Array)(&questionIDs))
	if err != nil {
		jsonError(w, "Сессия не найдена или истекла", http.StatusBadRequest)
		return
	}

	// Загружаем правильные ответы
	ids := make([]int, len(questionIDs))
	for i, id := range questionIDs {
		ids[i] = int(id)
	}
	questions, err := loadQuestionsByIDs(h, ids)
	if err != nil {
		jsonError(w, "Ошибка проверки ответов", http.StatusInternalServerError)
		return
	}

	// Считаем баллы (сервер — источник правды)
	correct := 0
	for i, q := range questions {
		if i < len(req.Answers) && req.Answers[i] == q.CorrectIdx {
			correct++
		}
	}

	levelKey := scoreToLevel(correct)
	lvl := kspoяLevels[levelKey]

	// Закрываем сессию
	h.db.Exec(`
		UPDATE kspoя_sessions
		SET status = 'completed', score = $1, level_key = $2, finished_at = now()
		WHERE id = $3
	`, correct, levelKey, req.SessionID)

	// Сохраняем результат
	h.db.Exec(`
		INSERT INTO kspoя_results (user_id, score, level_key, coins_given)
		VALUES ($1, $2, $3, $4)
	`, username, correct, levelKey, lvl.Coins)

	// Начисляем монеты и XP пользователю
	var newBalance, newXP int
	h.db.QueryRow(`
		UPDATE users
		SET balance = balance + $1, xp = xp + $2
		WHERE username = $3
		RETURNING balance, xp
	`, lvl.Coins, lvl.XP, username).Scan(&newBalance, &newXP)

	// Выдаём значок (если ещё нет)
	badgeEarned := ""
	var exists bool
	h.db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM user_badges WHERE user_id = $1 AND badge_code = $2
		)
	`, username, lvl.Badge).Scan(&exists)
	if !exists {
		h.db.Exec(`
			INSERT INTO user_badges (user_id, badge_code) VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, username, lvl.Badge)
		badgeEarned = lvl.Badge
	}

	jsonOK(w, map[string]interface{}{
		"score":        correct,
		"level_key":    levelKey,
		"level_label":  lvl.Label,
		"rarity":       lvl.Rarity,
		"coins_earned": lvl.Coins,
		"xp_earned":    lvl.XP,
		"badge_earned": badgeEarned,
		"new_balance":  newBalance,
		"new_xp":       newXP,
	})
}

// ─── KspoяCancel — POST /api/kspoя/cancel (античит) ───────────
func (h *Handler) KspoяCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	username := r.Context().Value("username").(string)

	var body struct {
		SessionID string `json:"session_id"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	h.db.Exec(`
		UPDATE kspoя_sessions
		SET status = 'cancelled', finished_at = now()
		WHERE user_id = $1 AND status = 'active'
		  AND ($2 = '' OR id::text = $2)
	`, username, body.SessionID)

	jsonOK(w, map[string]interface{}{"cancelled": true})
}

// ─── KspoяStatus — GET /api/kspoя/status ──────────────────────
// Возвращает последний результат пользователя + все его КСПОЯ-значки
func (h *Handler) KspoяStatus(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value("username").(string)

	// Последний результат
	var score int
	var levelKey string
	var finishedAt time.Time
	err := h.db.QueryRow(`
		SELECT score, level_key, finished_at FROM kspoя_results
		WHERE user_id = $1
		ORDER BY finished_at DESC LIMIT 1
	`, username).Scan(&score, &levelKey, &finishedAt)

	lastResult := map[string]interface{}{}
	if err == nil {
		lastResult = map[string]interface{}{
			"score":       score,
			"level_key":   levelKey,
			"level_label": kspoяLevels[levelKey].Label,
			"finished_at": finishedAt,
		}
	}

	// Все КСПОЯ-значки пользователя
	rows, _ := h.db.Query(`
		SELECT ub.badge_code, b.label, b.rarity, b.image_path, ub.earned_at
		FROM user_badges ub
		JOIN badges b ON b.code = ub.badge_code
		WHERE ub.user_id = $1 AND b.source = 'kspoя'
		ORDER BY ub.earned_at DESC
	`, username)
	defer rows.Close()

	var myBadges []map[string]interface{}
	for rows.Next() {
		var code, label, rarity, imgPath string
		var earnedAt time.Time
		rows.Scan(&code, &label, &rarity, &imgPath, &earnedAt)
		myBadges = append(myBadges, map[string]interface{}{
			"code":       code,
			"label":      label,
			"rarity":     rarity,
			"image_path": imgPath,
			"earned_at":  earnedAt,
		})
	}

	// Есть ли активная сессия?
	var activeSessionID string
	var expiresAt time.Time
	h.db.QueryRow(`
		SELECT id, expires_at FROM kspoя_sessions
		WHERE user_id = $1 AND status = 'active' AND expires_at > now()
	`, username).Scan(&activeSessionID, &expiresAt)

	jsonOK(w, map[string]interface{}{
		"last_result":       lastResult,
		"kspoя_badges":      myBadges,
		"active_session_id": activeSessionID,
		"active_expires_at": expiresAt,
	})
}

// ─── helpers ───────────────────────────────────────────────────

type questionDTO struct {
	ID         int      `json:"id"`
	Question   string   `json:"q"`
	Options    []string `json:"opts"`
	CorrectIdx int      `json:"correct"` // NOTE: сервер отдаёт correct только при /submit проверке
}

func loadQuestionsByIDs(h *Handler, ids []int) ([]questionDTO, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	// Строим IN-запрос
	query := `
		SELECT id, question, option_a, option_b, option_c, option_d, correct_idx
		FROM kspoя_questions WHERE id = ANY($1)
	`
	rows, err := h.db.Query(query, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Строим map для сохранения оригинального порядка ids
	byID := map[int]questionDTO{}
	for rows.Next() {
		var q questionDTO
		var a, b, c, d string
		rows.Scan(&q.ID, &q.Question, &a, &b, &c, &d, &q.CorrectIdx)
		q.Options = []string{a, b, c, d}
		byID[q.ID] = q
	}

	result := make([]questionDTO, 0, len(ids))
	for _, id := range ids {
		if q, ok := byID[id]; ok {
			// При отдаче клиенту НЕ включаем CorrectIdx
			result = append(result, questionDTO{
				ID:       q.ID,
				Question: q.Question,
				Options:  q.Options,
			})
		}
	}
	return result, nil
}

func sendActiveSession(w http.ResponseWriter, h *Handler, username, sessionID string) {
	var questionIDs []int64
	var expiresAt time.Time
	h.db.QueryRow(`
		SELECT question_ids, expires_at FROM kspoя_sessions WHERE id = $1
	`, sessionID).Scan((*pq.Int64Array)(&questionIDs), &expiresAt)

	ids := make([]int, len(questionIDs))
	for i, id := range questionIDs {
		ids[i] = int(id)
	}
	questions, _ := loadQuestionsByIDs(h, ids)
	jsonOK(w, map[string]interface{}{
		"session_id": sessionID,
		"expires_at": expiresAt,
		"questions":  questions,
		"resumed":    true,
	})
}

func intSliceToPgArray(s []int) interface{} {
	arr := make(pq.Int64Array, len(s))
	for i, v := range s {
		arr[i] = int64(v)
	}
	return arr
}

func jsonOK(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}