package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/domain"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/state"
)

//go:embed schema.sql
var schemaFS embed.FS

type Store struct {
	db *sql.DB
}

type User struct {
	ID         int64
	TelegramID sql.NullInt64
	ChatID     sql.NullInt64
	Username   string
	State      state.Name
	StateData  state.Data
	Timezone   string
}

type Profile struct {
	UserID              int64
	Age                 int
	HeightCm            float64
	WeightKg            float64
	TargetWeightKg      float64
	Gender              domain.Gender
	ActivityLevel       domain.ActivityLevel
	BMR                 float64
	TDEE                float64
	TargetCalories      float64
	TargetProteinG      float64
	TargetFatG          float64
	TargetCarbsG        float64
	WeightBaselineKg    float64
	DietBreakUntil      sql.NullString
	ForfaitAdjustment   float64
	ExcludedIngredients string
	Region              string
}

type Meal struct {
	ID          int64
	UserID      int64
	Description string
	Calories    float64
	ProteinG    float64
	FatG        float64
	CarbsG      float64
	Source      string
	LoggedAt    time.Time
}

type WeightEntry struct {
	ID         int64
	UserID     int64
	WeightKg   float64
	RecordedAt time.Time
}

type ForfaitEntry struct {
	ID            int64
	UserID        int64
	PresetKey     string
	Calories      float64
	DaysRemaining int
	DailyOffset   float64
	LoggedAt      time.Time
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	s := &Store{db: db}
	if err := s.Migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) Migrate() error {
	schema, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}
	if _, err := s.db.Exec(string(schema)); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return s.migrateColumns()
}

func (s *Store) GetOrCreateUser(ctx context.Context, telegramID, chatID int64, username string) (*User, error) {
	u, err := s.GetUserByTelegramID(ctx, telegramID)
	if err == nil {
		return u, nil
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users (telegram_id, chat_id, username) VALUES (?, ?, ?)`,
		telegramID, chatID, username)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	return s.GetUserByID(ctx, id)
}

func (s *Store) GetOrCreateLocalUser(ctx context.Context, username string) (*User, error) {
	var id int64
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM users WHERE telegram_id IS NULL AND username = ?`, username).Scan(&id)
	if err == nil {
		return s.GetUserByID(ctx, id)
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users (username) VALUES (?)`, username)
	if err != nil {
		return nil, fmt.Errorf("create local user: %w", err)
	}
	id, err = res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	return s.GetUserByID(ctx, id)
}

func (s *Store) GetUserByTelegramID(ctx context.Context, telegramID int64) (*User, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM users WHERE telegram_id = ?`, telegramID).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return s.GetUserByID(ctx, id)
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (*User, error) {
	u := &User{}
	var st, sd string
	var tgID, chatID sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT id, telegram_id, chat_id, username, state, state_data, timezone FROM users WHERE id = ?`, id).
		Scan(&u.ID, &tgID, &chatID, &u.Username, &st, &sd, &u.Timezone)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	u.TelegramID = tgID
	u.ChatID = chatID
	u.State = state.Name(st)
	u.StateData = state.ParseData(sd)
	return u, nil
}

func (s *Store) UpdateUserState(ctx context.Context, userID int64, st state.Name, data state.Data) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET state = ?, state_data = ?, updated_at = datetime('now') WHERE id = ?`,
		string(st), data.JSON(), userID)
	if err != nil {
		return fmt.Errorf("update state: %w", err)
	}
	return nil
}

func (s *Store) SaveProfile(ctx context.Context, userID int64, input domain.ProfileInput, targets domain.MacroTargets) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO profiles (
			user_id, age, height_cm, weight_kg, target_weight_kg, gender, activity_level,
			bmr, tdee, target_calories, target_protein_g, target_fat_g, target_carbs_g,
			weight_baseline_kg, excluded_ingredients, region
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			age=excluded.age, height_cm=excluded.height_cm, weight_kg=excluded.weight_kg,
			target_weight_kg=excluded.target_weight_kg, gender=excluded.gender,
			activity_level=excluded.activity_level, bmr=excluded.bmr, tdee=excluded.tdee,
			target_calories=excluded.target_calories, target_protein_g=excluded.target_protein_g,
			target_fat_g=excluded.target_fat_g, target_carbs_g=excluded.target_carbs_g,
			weight_baseline_kg=excluded.weight_baseline_kg,
			excluded_ingredients=excluded.excluded_ingredients, region=excluded.region,
			updated_at=datetime('now')`,
		userID, input.Age, input.HeightCm, input.WeightKg, input.TargetWeightKg,
		string(input.Gender), string(input.ActivityLevel),
		targets.BMR, targets.TDEE, targets.TargetCalories,
		targets.TargetProteinG, targets.TargetFatG, targets.TargetCarbsG, input.WeightKg,
		input.ExcludedIngredients, input.Region)
	if err != nil {
		return fmt.Errorf("save profile: %w", err)
	}
	return nil
}

func (s *Store) GetProfile(ctx context.Context, userID int64) (*Profile, error) {
	p := &Profile{}
	var gender, activity string
	var dietBreak sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id, age, height_cm, weight_kg, target_weight_kg, gender, activity_level,
			bmr, tdee, target_calories, target_protein_g, target_fat_g, target_carbs_g,
			weight_baseline_kg, diet_break_until, forfait_adjustment,
			excluded_ingredients, region
		FROM profiles WHERE user_id = ?`, userID).
		Scan(&p.UserID, &p.Age, &p.HeightCm, &p.WeightKg, &p.TargetWeightKg,
			&gender, &activity, &p.BMR, &p.TDEE, &p.TargetCalories,
			&p.TargetProteinG, &p.TargetFatG, &p.TargetCarbsG,
			&p.WeightBaselineKg, &dietBreak, &p.ForfaitAdjustment,
			&p.ExcludedIngredients, &p.Region)
	if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}
	p.Gender = domain.Gender(gender)
	p.ActivityLevel = domain.ActivityLevel(activity)
	p.DietBreakUntil = dietBreak
	return p, nil
}

func (s *Store) EffectiveTargets(ctx context.Context, userID int64) (domain.MacroTargets, error) {
	p, err := s.GetProfile(ctx, userID)
	if err != nil {
		return domain.MacroTargets{}, err
	}
	targets := domain.MacroTargets{
		BMR: p.BMR, TDEE: p.TDEE,
		TargetCalories: p.TargetCalories, TargetProteinG: p.TargetProteinG,
		TargetFatG: p.TargetFatG, TargetCarbsG: p.TargetCarbsG,
	}

	if daily, err := s.GetDailyLogForUser(ctx, userID); err == nil && daily.AdjustedTDEE > 0 {
		targets = applyAdjustedTDEE(targets, p, daily.AdjustedTDEE)
	}

	if p.DietBreakUntil.Valid {
		until, err := time.Parse("2006-01-02", p.DietBreakUntil.String)
		if err == nil && time.Now().Before(until) {
			return domain.DietBreakTargets(targets.TDEE, p.TargetProteinG), nil
		}
	}

	offset, err := s.ActiveForfaitOffset(ctx, userID)
	if err != nil {
		return domain.MacroTargets{}, err
	}
	if offset > 0 {
		targets = domain.ApplyForfaitAdjustment(targets, offset)
	}
	return targets, nil
}

func applyAdjustedTDEE(targets domain.MacroTargets, p *Profile, adjustedTDEE float64) domain.MacroTargets {
	deficit := p.TDEE - p.TargetCalories
	if deficit < 0 {
		deficit = 0
	}
	newCalories := adjustedTDEE - deficit
	if newCalories < p.BMR {
		newCalories = p.BMR
	}
	targets.TDEE = adjustedTDEE
	targets.TargetCalories = newCalories

	fatCal := newCalories * domain.FatCalorieFraction
	proteinCal := targets.TargetProteinG * 4
	carbCal := newCalories - proteinCal - fatCal
	if carbCal < 0 {
		carbCal = 0
	}
	targets.TargetFatG = fatCal / 9
	targets.TargetCarbsG = carbCal / 4
	return targets
}

func (s *Store) AddWeightEntry(ctx context.Context, userID int64, weightKg float64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO weight_entries (user_id, weight_kg) VALUES (?, ?)`, userID, weightKg)
	if err != nil {
		return fmt.Errorf("add weight: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE profiles SET weight_kg = ?, updated_at = datetime('now') WHERE user_id = ?`,
		weightKg, userID)
	return err
}

func (s *Store) ListWeightEntries(ctx context.Context, userID int64, limit int) ([]WeightEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, weight_kg, recorded_at FROM weight_entries
		 WHERE user_id = ? ORDER BY recorded_at ASC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list weights: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var entries []WeightEntry
	for rows.Next() {
		var e WeightEntry
		var ts string
		if err := rows.Scan(&e.ID, &e.UserID, &e.WeightKg, &ts); err != nil {
			return nil, err
		}
		e.RecordedAt, _ = time.Parse("2006-01-02 15:04:05", ts)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *Store) UpdateWeightBaseline(ctx context.Context, userID int64, baseline float64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE profiles SET weight_baseline_kg = ? WHERE user_id = ?`, baseline, userID)
	return err
}

func (s *Store) RecalculateProfileTargets(ctx context.Context, userID int64, weightKg float64) error {
	p, err := s.GetProfile(ctx, userID)
	if err != nil {
		return err
	}
	input := domain.ProfileInput{
		Age: p.Age, HeightCm: p.HeightCm, WeightKg: weightKg,
		TargetWeightKg: p.TargetWeightKg, Gender: p.Gender,
		ActivityLevel: p.ActivityLevel, WeightGoal: domain.GoalLose,
	}
	targets := domain.CalculateMacroTargets(input)
	_, err = s.db.ExecContext(ctx, `
		UPDATE profiles SET weight_kg=?, bmr=?, tdee=?, target_calories=?,
			target_protein_g=?, target_fat_g=?, target_carbs_g=?,
			weight_baseline_kg=?, updated_at=datetime('now') WHERE user_id=?`,
		weightKg, targets.BMR, targets.TDEE, targets.TargetCalories,
		targets.TargetProteinG, targets.TargetFatG, targets.TargetCarbsG,
		weightKg, userID)
	return err
}

func (s *Store) AddMeal(ctx context.Context, userID int64, meal domain.MealEstimate, source string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO meals (user_id, description, calories, protein_g, fat_g, carbs_g, source)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		userID, meal.Description, meal.Calories, meal.ProteinG, meal.FatG, meal.CarbsG, source)
	return err
}

func (s *Store) DailyProgress(ctx context.Context, userID int64, day time.Time) (domain.DailyProgress, error) {
	loc := s.userLocation(ctx, userID)
	start, end := domain.DayBounds(loc, day)
	return s.dailyProgressBetween(ctx, userID, start, end, day.In(loc))
}

func (s *Store) DailyProgressNow(ctx context.Context, userID int64) (domain.DailyProgress, error) {
	loc := s.userLocation(ctx, userID)
	now := time.Now().In(loc)
	return s.DailyProgress(ctx, userID, now)
}

func (s *Store) dailyProgressBetween(ctx context.Context, userID int64, start, end, localDay time.Time) (domain.DailyProgress, error) {
	var cal, p, f, c sql.NullFloat64
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(calories),0), COALESCE(SUM(protein_g),0),
			COALESCE(SUM(fat_g),0), COALESCE(SUM(carbs_g),0)
		FROM meals WHERE user_id = ? AND logged_at >= ? AND logged_at < ?`,
		userID, domain.FormatSQLiteUTC(start), domain.FormatSQLiteUTC(end)).
		Scan(&cal, &p, &f, &c)
	if err != nil {
		return domain.DailyProgress{}, err
	}
	return domain.DailyProgress{
		Date: localDay, Calories: cal.Float64, ProteinG: p.Float64,
		FatG: f.Float64, CarbsG: c.Float64,
	}, nil
}

func (s *Store) DeleteLastMeal(ctx context.Context, userID int64) (bool, error) {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM meals WHERE id = (
			SELECT id FROM meals WHERE user_id = ? ORDER BY logged_at DESC, id DESC LIMIT 1
		)`, userID)
	if err != nil {
		return false, fmt.Errorf("delete last meal: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *Store) AddForfait(ctx context.Context, userID int64, preset domain.ForfaitPreset) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO forfait_entries (user_id, preset_key, calories, days_remaining, daily_offset)
		VALUES (?, ?, ?, ?, ?)`,
		userID, preset.Key, preset.Calories, domain.ForfaitSmoothDays, domain.ForfaitDailyOffset)
	if err != nil {
		return fmt.Errorf("add forfait: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO meals (user_id, description, calories, protein_g, fat_g, carbs_g, source)
		VALUES (?, ?, ?, 0, 0, 0, 'text')`,
		userID, preset.Label, preset.Calories)
	return err
}

func (s *Store) ActiveForfaitOffset(ctx context.Context, userID int64) (float64, error) {
	var offset sql.NullFloat64
	err := s.db.QueryRowContext(ctx, `
		SELECT daily_offset FROM forfait_entries
		WHERE user_id = ? AND days_remaining > 0
		ORDER BY logged_at DESC LIMIT 1`, userID).Scan(&offset)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return offset.Float64, nil
}

func (s *Store) DecrementForfaitDays(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE forfait_entries SET days_remaining = days_remaining - 1
		WHERE user_id = ? AND days_remaining > 0`, userID)
	return err
}

func (s *Store) SetDietBreak(ctx context.Context, userID int64, until time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE profiles SET diet_break_until = ? WHERE user_id = ?`,
		until.Format("2006-01-02"), userID)
	return err
}

func (s *Store) ClearDietBreak(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE profiles SET diet_break_until = NULL WHERE user_id = ?`, userID)
	return err
}

func (s *Store) SaveLLMAudit(ctx context.Context, userID *int64, operation, model, prompt, rawResp string, tokIn, tokOut int, latency int64) error {
	var uid sql.NullInt64
	if userID != nil {
		uid = sql.NullInt64{Int64: *userID, Valid: true}
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO llm_audit_log (user_id, operation, model, prompt, raw_response, tokens_prompt, tokens_output, latency_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		uid, operation, model, prompt, rawResp, tokIn, tokOut, latency)
	return err
}

func (s *Store) ListUsersWithTelegram(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, telegram_id, chat_id, username, state, state_data, timezone
		 FROM users WHERE telegram_id IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var users []User
	for rows.Next() {
		u := User{}
		var st, sd string
		var tgID, chatID sql.NullInt64
		if err := rows.Scan(&u.ID, &tgID, &chatID, &u.Username, &st, &sd, &u.Timezone); err != nil {
			return nil, err
		}
		u.TelegramID = tgID
		u.ChatID = chatID
		u.State = state.Name(st)
		u.StateData = state.ParseData(sd)
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) WasSchedulerSentToday(ctx context.Context, userID int64, jobType string) (bool, error) {
	loc := s.userLocation(ctx, userID)
	start, end := domain.DayBounds(loc, time.Now())
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM scheduler_log
		WHERE user_id = ? AND job_type = ? AND sent_at >= ? AND sent_at < ?`,
		userID, jobType, domain.FormatSQLiteUTC(start), domain.FormatSQLiteUTC(end)).Scan(&count)
	return count > 0, err
}

func (s *Store) LogSchedulerSent(ctx context.Context, userID int64, jobType string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO scheduler_log (user_id, job_type) VALUES (?, ?)`, userID, jobType)
	return err
}

func (s *Store) UpsertActivitySync(ctx context.Context, userID int64, provider string, activeCalories float64) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO activity_sync (user_id, provider, active_calories, last_sync_at, synced_date)
		VALUES (?, ?, ?, datetime('now'), date('now'))
		ON CONFLICT(user_id, provider) DO UPDATE SET
			active_calories=excluded.active_calories,
			last_sync_at=datetime('now'),
			synced_date=date('now')`,
		userID, provider, activeCalories)
	return err
}

func (s *Store) GetActivityCalories(ctx context.Context, userID int64) (float64, error) {
	var cal sql.NullFloat64
	err := s.db.QueryRowContext(ctx, `
		SELECT SUM(active_calories) FROM activity_sync
		WHERE user_id = ? AND synced_date = date('now')`, userID).Scan(&cal)
	if err != nil {
		return 0, err
	}
	return cal.Float64, nil
}

func (s *Store) WeeklyMealSummary(ctx context.Context, userID int64) (totalCal, avgCal float64, days int, err error) {
	loc := s.userLocation(ctx, userID)
	start, end := domain.WeekBounds(loc, time.Now())

	rows, err := s.db.QueryContext(ctx, `
		SELECT logged_at, calories FROM meals
		WHERE user_id = ? AND logged_at >= ? AND logged_at < ?`,
		userID, domain.FormatSQLiteUTC(start), domain.FormatSQLiteUTC(end))
	if err != nil {
		return 0, 0, 0, err
	}
	defer func() { _ = rows.Close() }()

	dailyTotals := make(map[string]float64)
	for rows.Next() {
		var ts string
		var cal float64
		if err := rows.Scan(&ts, &cal); err != nil {
			return 0, 0, 0, err
		}
		logged, parseErr := time.Parse("2006-01-02 15:04:05", ts)
		if parseErr != nil {
			continue
		}
		dayKey := logged.In(loc).Format("2006-01-02")
		dailyTotals[dayKey] += cal
	}
	if err := rows.Err(); err != nil {
		return 0, 0, 0, err
	}

	for _, cal := range dailyTotals {
		totalCal += cal
	}
	days = len(dailyTotals)
	if days > 0 {
		avgCal = totalCal / float64(days)
	}
	return totalCal, avgCal, days, nil
}
