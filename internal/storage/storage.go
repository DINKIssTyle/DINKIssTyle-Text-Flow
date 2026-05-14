package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const appDirName = "DKST Text Flow"

type Store struct {
	db *sql.DB
}

const generalSettingsKey = "general.settings"

type GeneralSettings struct {
	TypingTrendEnabled bool `json:"typingTrendEnabled"`
}

type Snippet struct {
	ID            int64  `json:"id"`
	LabelID       int64  `json:"labelId"`
	Shortcut      string `json:"shortcut"`
	Title         string `json:"title"`
	Content       string `json:"content"`
	ContentType   string `json:"contentType"`
	Enabled       bool   `json:"enabled"`
	CaseSensitive bool   `json:"caseSensitive"`
	UsePaste      bool   `json:"usePaste"`
	ExpandMode    string `json:"expandMode"`
	UsageCount    int64  `json:"usageCount"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

type SnippetInput struct {
	LabelID       int64  `json:"labelId"`
	Shortcut      string `json:"shortcut"`
	Title         string `json:"title"`
	Content       string `json:"content"`
	ContentType   string `json:"contentType"`
	Enabled       bool   `json:"enabled"`
	CaseSensitive bool   `json:"caseSensitive"`
	UsePaste      bool   `json:"usePaste"`
	ExpandMode    string `json:"expandMode"`
}

type Label struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Color        string `json:"color"`
	SnippetCount int64  `json:"snippetCount"`
	EnabledCount int64  `json:"enabledCount"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type LabelInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
}

type DashboardStats struct {
	TotalExpansions    int64             `json:"totalExpansions"`
	TodayExpansions    int64             `json:"todayExpansions"`
	SnippetCount       int64             `json:"snippetCount"`
	EnabledCount       int64             `json:"enabledCount"`
	TodayTypingCount   int64             `json:"todayTypingCount"`
	AverageDailyTyping int64             `json:"averageDailyTyping"`
	TypingHistory      []DailyTypingStat `json:"typingHistory"`
	TopSnippets        []Snippet         `json:"topSnippets"`
}

type DailyTypingStat struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

func OpenDefault() (*Store, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve config dir: %w", err)
	}

	dir := filepath.Join(configDir, appDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create app data dir: %w", err)
	}

	return Open(filepath.Join(dir, "text-flow.sqlite3"))
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.seed(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) ListSnippets(query string) ([]Snippet, error) {
	return s.ListSnippetsByLabel(query, 0)
}

func (s *Store) ListSnippetsByLabel(query string, labelID int64) ([]Snippet, error) {
	where := ""
	args := []any{}
	if strings.TrimSpace(query) != "" {
		where = "WHERE (shortcut LIKE ? OR title LIKE ? OR content LIKE ?)"
		term := "%" + strings.TrimSpace(query) + "%"
		args = append(args, term, term, term)
	}
	if labelID > 0 {
		if where == "" {
			where = "WHERE label_id = ?"
		} else {
			where += " AND label_id = ?"
		}
		args = append(args, labelID)
	}

	rows, err := s.db.Query(`
		SELECT id, COALESCE(label_id, 0), shortcut, title, content, content_type, enabled, case_sensitive,
		       use_paste, expand_mode, usage_count, created_at, updated_at
		FROM snippets
		`+where+`
		ORDER BY updated_at DESC, id DESC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSnippets(rows)
}

func (s *Store) CreateSnippet(input SnippetInput) (Snippet, error) {
	normalized, err := normalizeInput(input)
	if err != nil {
		return Snippet{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.Exec(`
		INSERT INTO snippets (
			label_id, shortcut, title, content, content_type, enabled, case_sensitive,
			use_paste, expand_mode, usage_count, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)
	`, labelIDArg(normalized.LabelID), normalized.Shortcut, normalized.Title, normalized.Content, normalized.ContentType,
		normalized.Enabled, normalized.CaseSensitive, normalized.UsePaste, normalized.ExpandMode, now, now)
	if err != nil {
		return Snippet{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Snippet{}, err
	}
	return s.GetSnippet(id)
}

func (s *Store) UpdateSnippet(id int64, input SnippetInput) (Snippet, error) {
	normalized, err := normalizeInput(input)
	if err != nil {
		return Snippet{}, err
	}

	result, err := s.db.Exec(`
		UPDATE snippets
		SET label_id = ?, shortcut = ?, title = ?, content = ?, content_type = ?, enabled = ?,
		    case_sensitive = ?, use_paste = ?, expand_mode = ?, updated_at = ?
		WHERE id = ?
	`, labelIDArg(normalized.LabelID), normalized.Shortcut, normalized.Title, normalized.Content, normalized.ContentType,
		normalized.Enabled, normalized.CaseSensitive, normalized.UsePaste, normalized.ExpandMode,
		time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return Snippet{}, err
	}

	count, err := result.RowsAffected()
	if err != nil {
		return Snippet{}, err
	}
	if count == 0 {
		return Snippet{}, sql.ErrNoRows
	}
	return s.GetSnippet(id)
}

func (s *Store) DeleteSnippet(id int64) error {
	result, err := s.db.Exec("DELETE FROM snippets WHERE id = ?", id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) ToggleSnippet(id int64, enabled bool) (Snippet, error) {
	result, err := s.db.Exec(
		"UPDATE snippets SET enabled = ?, updated_at = ? WHERE id = ?",
		enabled,
		time.Now().UTC().Format(time.RFC3339),
		id,
	)
	if err != nil {
		return Snippet{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return Snippet{}, err
	}
	if count == 0 {
		return Snippet{}, sql.ErrNoRows
	}
	return s.GetSnippet(id)
}

func (s *Store) AssignSnippetLabel(snippetID int64, labelID int64) (Snippet, error) {
	result, err := s.db.Exec(
		"UPDATE snippets SET label_id = ?, updated_at = ? WHERE id = ?",
		labelIDArg(labelID),
		time.Now().UTC().Format(time.RFC3339),
		snippetID,
	)
	if err != nil {
		return Snippet{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return Snippet{}, err
	}
	if count == 0 {
		return Snippet{}, sql.ErrNoRows
	}
	return s.GetSnippet(snippetID)
}

func (s *Store) SetLabelSnippetsEnabled(labelID int64, enabled bool) error {
	where := "label_id = ?"
	args := []any{enabled, time.Now().UTC().Format(time.RFC3339), labelID}
	if labelID == 0 {
		where = "1 = 1"
		args = []any{enabled, time.Now().UTC().Format(time.RFC3339)}
	}
	_, err := s.db.Exec("UPDATE snippets SET enabled = ?, updated_at = ? WHERE "+where, args...)
	return err
}

func (s *Store) GetSnippet(id int64) (Snippet, error) {
	row := s.db.QueryRow(`
		SELECT id, COALESCE(label_id, 0), shortcut, title, content, content_type, enabled, case_sensitive,
		       use_paste, expand_mode, usage_count, created_at, updated_at
		FROM snippets
		WHERE id = ?
	`, id)
	return scanSnippet(row)
}

func (s *Store) Dashboard() (DashboardStats, error) {
	var stats DashboardStats
	var err error
	if err := s.db.QueryRow("SELECT COUNT(*), COALESCE(SUM(usage_count), 0), COALESCE(SUM(CASE WHEN enabled THEN 1 ELSE 0 END), 0) FROM snippets").
		Scan(&stats.SnippetCount, &stats.TotalExpansions, &stats.EnabledCount); err != nil {
		return DashboardStats{}, err
	}

	today := time.Now().Format("2006-01-02")
	if err := s.db.QueryRow("SELECT COUNT(*) FROM usage_logs WHERE date(expanded_at) = date(?)", today).
		Scan(&stats.TodayExpansions); err != nil {
		return DashboardStats{}, err
	}
	if err := s.db.QueryRow("SELECT COALESCE(count, 0) FROM typing_stats WHERE date = ?", today).
		Scan(&stats.TodayTypingCount); errors.Is(err, sql.ErrNoRows) {
		stats.TodayTypingCount = 0
	} else if err != nil {
		return DashboardStats{}, err
	}

	stats.TypingHistory, err = s.typingHistory(90)
	if err != nil {
		return DashboardStats{}, err
	}
	var totalTyping int64
	var activeTypingDays int64
	for _, day := range stats.TypingHistory {
		if day.Count == 0 {
			continue
		}
		totalTyping += day.Count
		activeTypingDays++
	}
	if activeTypingDays > 0 {
		stats.AverageDailyTyping = totalTyping / activeTypingDays
	}

	rows, err := s.db.Query(`
		SELECT id, COALESCE(label_id, 0), shortcut, title, content, content_type, enabled, case_sensitive,
		       use_paste, expand_mode, usage_count, created_at, updated_at
		FROM snippets
		ORDER BY usage_count DESC, updated_at DESC
		LIMIT 5
	`)
	if err != nil {
		return DashboardStats{}, err
	}
	defer rows.Close()

	stats.TopSnippets, err = scanSnippets(rows)
	return stats, err
}

func (s *Store) typingHistory(days int) ([]DailyTypingStat, error) {
	if days <= 0 {
		return []DailyTypingStat{}, nil
	}

	start := time.Now().AddDate(0, 0, -(days - 1))
	rows, err := s.db.Query(`
		SELECT date, count
		FROM typing_stats
		WHERE date >= ?
		ORDER BY date ASC
	`, start.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byDate := map[string]int64{}
	for rows.Next() {
		var stat DailyTypingStat
		if err := rows.Scan(&stat.Date, &stat.Count); err != nil {
			return nil, err
		}
		byDate[stat.Date] = stat.Count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	history := make([]DailyTypingStat, 0, days)
	for i := 0; i < days; i++ {
		date := start.AddDate(0, 0, i).Format("2006-01-02")
		history = append(history, DailyTypingStat{
			Date:  date,
			Count: byDate[date],
		})
	}
	return history, nil
}

func (s *Store) ListLabels() ([]Label, error) {
	rows, err := s.db.Query(`
		SELECT labels.id, labels.name, labels.description, labels.color,
		       COUNT(snippets.id) AS snippet_count,
		       COALESCE(SUM(CASE WHEN snippets.enabled THEN 1 ELSE 0 END), 0) AS enabled_count,
		       labels.created_at, labels.updated_at
		FROM labels
		LEFT JOIN snippets ON snippets.label_id = labels.id
		GROUP BY labels.id
		ORDER BY labels.name COLLATE NOCASE ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	labels := []Label{}
	for rows.Next() {
		var label Label
		if err := rows.Scan(&label.ID, &label.Name, &label.Description, &label.Color, &label.SnippetCount, &label.EnabledCount, &label.CreatedAt, &label.UpdatedAt); err != nil {
			return nil, err
		}
		labels = append(labels, label)
	}
	return labels, rows.Err()
}

func (s *Store) CreateLabel(input LabelInput) (Label, error) {
	normalized, err := normalizeLabelInput(input)
	if err != nil {
		return Label{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.Exec(`
		INSERT INTO labels (name, description, color, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, normalized.Name, normalized.Description, normalized.Color, now, now)
	if err != nil {
		return Label{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Label{}, err
	}
	return s.GetLabel(id)
}

func (s *Store) UpdateLabel(id int64, input LabelInput) (Label, error) {
	normalized, err := normalizeLabelInput(input)
	if err != nil {
		return Label{}, err
	}

	result, err := s.db.Exec(`
		UPDATE labels
		SET name = ?, description = ?, color = ?, updated_at = ?
		WHERE id = ?
	`, normalized.Name, normalized.Description, normalized.Color, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return Label{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return Label{}, err
	}
	if count == 0 {
		return Label{}, sql.ErrNoRows
	}
	return s.GetLabel(id)
}

func (s *Store) DeleteLabel(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec("UPDATE snippets SET label_id = NULL, updated_at = ? WHERE label_id = ?", now, id); err != nil {
		return err
	}
	result, err := tx.Exec("DELETE FROM labels WHERE id = ?", id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

func (s *Store) GetLabel(id int64) (Label, error) {
	row := s.db.QueryRow(`
		SELECT labels.id, labels.name, labels.description, labels.color,
		       COUNT(snippets.id) AS snippet_count,
		       COALESCE(SUM(CASE WHEN snippets.enabled THEN 1 ELSE 0 END), 0) AS enabled_count,
		       labels.created_at, labels.updated_at
		FROM labels
		LEFT JOIN snippets ON snippets.label_id = labels.id
		WHERE labels.id = ?
		GROUP BY labels.id
	`, id)

	var label Label
	err := row.Scan(&label.ID, &label.Name, &label.Description, &label.Color, &label.SnippetCount, &label.EnabledCount, &label.CreatedAt, &label.UpdatedAt)
	return label, err
}

func (s *Store) LogExpansion(snippetID int64, appBundleID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec(
		"INSERT INTO usage_logs (snippet_id, app_bundle_id, expanded_at) VALUES (?, ?, ?)",
		snippetID,
		appBundleID,
		now,
	); err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE snippets SET usage_count = usage_count + 1, updated_at = ? WHERE id = ?", now, snippetID); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) LogTyping(count int64) error {
	if count <= 0 {
		return nil
	}
	enabled, err := s.TypingTrendEnabled()
	if err != nil || !enabled {
		return err
	}

	_, err = s.db.Exec(`
		INSERT INTO typing_stats (date, count, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(date) DO UPDATE SET
			count = count + excluded.count,
			updated_at = excluded.updated_at
	`, time.Now().Format("2006-01-02"), count, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (s *Store) TypingTrendEnabled() (bool, error) {
	settings := GeneralSettings{TypingTrendEnabled: true}
	found, err := s.GetJSONSetting(generalSettingsKey, &settings)
	if err != nil {
		return false, err
	}
	if !found {
		return true, nil
	}
	return settings.TypingTrendEnabled, nil
}

func (s *Store) GetSetting(key string) (string, bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", false, errors.New("setting key is required")
	}

	var value string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func (s *Store) SetSetting(key string, value string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("setting key is required")
	}

	_, err := s.db.Exec(`
		INSERT INTO settings (key, value)
		VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	return err
}

func (s *Store) GetJSONSetting(key string, target any) (bool, error) {
	value, ok, err := s.GetSetting(key)
	if err != nil || !ok {
		return ok, err
	}
	if err := json.Unmarshal([]byte(value), target); err != nil {
		return true, err
	}
	return true, nil
}

func (s *Store) SetJSONSetting(key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.SetSetting(key, string(data))
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS snippets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			label_id INTEGER,
			shortcut TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			content_type TEXT NOT NULL DEFAULT 'plain',
			group_id INTEGER,
			enabled INTEGER NOT NULL DEFAULT 1,
			case_sensitive INTEGER NOT NULL DEFAULT 0,
			use_paste INTEGER NOT NULL DEFAULT 0,
			expand_mode TEXT NOT NULL DEFAULT 'delimiter',
			usage_count INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS labels (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL DEFAULT '',
			color TEXT NOT NULL DEFAULT '#153e75',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS snippet_context_rules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			snippet_id INTEGER NOT NULL,
			app_bundle_id TEXT NOT NULL,
			app_name TEXT NOT NULL,
			transform_mode TEXT NOT NULL DEFAULT 'plain',
			enabled INTEGER NOT NULL DEFAULT 1,
			FOREIGN KEY(snippet_id) REFERENCES snippets(id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS usage_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			snippet_id INTEGER NOT NULL,
			app_bundle_id TEXT,
			expanded_at TEXT NOT NULL,
			FOREIGN KEY(snippet_id) REFERENCES snippets(id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS typing_stats (
			date TEXT PRIMARY KEY,
			count INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);

		UPDATE snippets
		SET content_type = 'plain'
		WHERE content_type NOT IN ('plain', 'rich');
	`)
	if err != nil {
		return err
	}
	if err := s.ensureColumn("snippets", "use_paste", "ALTER TABLE snippets ADD COLUMN use_paste INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	return s.ensureColumn("snippets", "label_id", "ALTER TABLE snippets ADD COLUMN label_id INTEGER")
}

func (s *Store) ensureColumn(table string, column string, statement string) error {
	rows, err := s.db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == column {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec(statement)
	return err
}

func (s *Store) seed() error {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM snippets").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	seeds := []SnippetInput{
		{Shortcut: ";addr", Title: "Office Address", Content: "서울특별시 강남구 테헤란로 DKST Text Flow", ContentType: "plain", Enabled: true, ExpandMode: "delimiter"},
		{Shortcut: ";sig", Title: "Email Signature", Content: "감사합니다.\nDINKIssTyle 드림", ContentType: "plain", Enabled: true, ExpandMode: "delimiter"},
		{Shortcut: ";date", Title: "Today", Content: "{{date:2006-01-02}}", ContentType: "plain", Enabled: true, ExpandMode: "delimiter"},
	}
	for _, seed := range seeds {
		if _, err := s.CreateSnippet(seed); err != nil {
			return err
		}
	}
	return nil
}

func normalizeInput(input SnippetInput) (SnippetInput, error) {
	if input.LabelID < 0 {
		input.LabelID = 0
	}
	input.Shortcut = strings.TrimSpace(input.Shortcut)
	input.Title = strings.TrimSpace(input.Title)
	input.ContentType = strings.TrimSpace(input.ContentType)
	input.ExpandMode = strings.TrimSpace(input.ExpandMode)

	if input.Shortcut == "" {
		return SnippetInput{}, errors.New("shortcut is required")
	}
	if input.Title == "" {
		return SnippetInput{}, errors.New("title is required")
	}
	if input.Content == "" {
		return SnippetInput{}, errors.New("content is required")
	}
	if input.ContentType == "" {
		input.ContentType = "plain"
	}
	if input.ExpandMode == "" {
		input.ExpandMode = "delimiter"
	}
	if input.ContentType != "plain" && input.ContentType != "rich" {
		return SnippetInput{}, errors.New("unsupported content type")
	}
	if input.ExpandMode != "instant" && input.ExpandMode != "delimiter" {
		return SnippetInput{}, errors.New("unsupported expand mode")
	}

	return input, nil
}

func normalizeLabelInput(input LabelInput) (LabelInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Color = strings.TrimSpace(input.Color)

	if input.Name == "" {
		return LabelInput{}, errors.New("label name is required")
	}
	if input.Color == "" {
		input.Color = "#153e75"
	}
	if !isHexColor(input.Color) {
		return LabelInput{}, errors.New("label color must be a hex color")
	}
	return input, nil
}

func isHexColor(value string) bool {
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	for _, char := range value[1:] {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') && (char < 'A' || char > 'F') {
			return false
		}
	}
	return true
}

func labelIDArg(labelID int64) any {
	if labelID <= 0 {
		return nil
	}
	return labelID
}

type scanner interface {
	Scan(dest ...any) error
}

func scanSnippet(row scanner) (Snippet, error) {
	var snippet Snippet
	err := row.Scan(
		&snippet.ID,
		&snippet.LabelID,
		&snippet.Shortcut,
		&snippet.Title,
		&snippet.Content,
		&snippet.ContentType,
		&snippet.Enabled,
		&snippet.CaseSensitive,
		&snippet.UsePaste,
		&snippet.ExpandMode,
		&snippet.UsageCount,
		&snippet.CreatedAt,
		&snippet.UpdatedAt,
	)
	return snippet, err
}

func scanSnippets(rows *sql.Rows) ([]Snippet, error) {
	snippets := []Snippet{}
	for rows.Next() {
		snippet, err := scanSnippet(rows)
		if err != nil {
			return nil, err
		}
		snippets = append(snippets, snippet)
	}
	return snippets, rows.Err()
}
