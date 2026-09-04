package scheduler

import (
	"context"
	"errors"
	"time"

	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/coach"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/db"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/trace"
)

type Notifier func(chatID int64, text string)

type Scheduler struct {
	store    *db.Store
	coach    *coach.Service
	logger   *trace.Logger
	notify   Notifier
	location *time.Location
}

var errSkipReminder = errors.New("skip reminder: meal already logged")

func New(store *db.Store, coachSvc *coach.Service, logger *trace.Logger, notify Notifier, tz string) *Scheduler {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	return &Scheduler{store: store, coach: coachSvc, logger: logger, notify: notify, location: loc}
}

func (sch *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			sch.tick(ctx, now.In(sch.location))
		}
	}
}

func (sch *Scheduler) tick(ctx context.Context, now time.Time) {
	hour, minute := now.Hour(), now.Minute()

	switch {
	case hour == 14 && minute == 0:
		sch.sendMealReminder(ctx, "lunch_reminder", "lunch", 11, 14, now)
	case hour == 21 && minute == 0:
		sch.sendMealReminder(ctx, "dinner_reminder", "dinner", 18, 21, now)
	case hour == 21 && minute == 30:
		sch.sendJob(ctx, "daily_recap", func(uid int64) (string, error) {
			return sch.coach.DailyRecap(ctx, uid)
		})
	case now.Weekday() == time.Sunday && hour == 20 && minute == 0:
		sch.sendJob(ctx, "weekly_report", func(uid int64) (string, error) {
			return sch.coach.WeeklyReport(ctx, uid)
		})
	}

	sch.decrementForfaitDays(ctx)
}

type messageFn func(userID int64) (string, error)

// sendMealReminder skips users who already logged a meal in [windowStart, windowEnd) local time.
func (sch *Scheduler) sendMealReminder(ctx context.Context, jobType, mealType string, windowStart, windowEnd int, now time.Time) {
	sch.sendJob(ctx, jobType, func(uid int64) (string, error) {
		hasMeal, err := sch.store.HasMealInLocalWindow(ctx, uid, now, windowStart, windowEnd)
		if err != nil {
			return "", err
		}
		if hasMeal {
			return "", errSkipReminder
		}
		return sch.coach.InactivityReminder(mealType), nil
	})
}

func (sch *Scheduler) sendJob(ctx context.Context, jobType string, fn messageFn) {
	users, err := sch.store.ListUsersWithTelegram(ctx)
	if err != nil {
		sch.logError(ctx, "list_users", err)
		return
	}

	for _, u := range users {
		if !u.ChatID.Valid {
			continue
		}
		sent, err := sch.store.WasSchedulerSentToday(ctx, u.ID, jobType)
		if err != nil || sent {
			continue
		}

		msg, err := fn(u.ID)
		if err != nil {
			if errors.Is(err, errSkipReminder) {
				_ = sch.store.LogSchedulerSent(ctx, u.ID, jobType)
				continue
			}
			sch.logError(ctx, jobType, err)
			continue
		}

		if sch.notify != nil {
			sch.notify(u.ChatID.Int64, msg)
		}
		_ = sch.store.LogSchedulerSent(ctx, u.ID, jobType)
	}
}

func (sch *Scheduler) decrementForfaitDays(ctx context.Context) {
	users, err := sch.store.ListUsersWithTelegram(ctx)
	if err != nil {
		return
	}
	for _, u := range users {
		_ = sch.store.DecrementForfaitDays(ctx, u.ID)
	}
}

func (sch *Scheduler) logError(ctx context.Context, event string, err error) {
	if sch.logger != nil {
		sch.logger.DomainEvent(ctx, "scheduler_error", map[string]any{
			"job": event, "error": err.Error(),
		})
	}
}
