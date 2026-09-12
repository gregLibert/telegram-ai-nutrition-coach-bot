package coach

import (
	"strings"
	"testing"

	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/domain"
)

func TestBuildAnalysisMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		view        AnalysisView
		wantContain []string
		wantAbsent  []string
	}{
		{
			name: "french daily under target with snack",
			view: AnalysisView{
				Data: domain.NutritionAnalysis{
					Congratulations:  "Bravo pour le suivi aujourd'hui !",
					StreakMaintained: true,
					TopAlignedMeals:  []string{"Bowl poulet", "Skyr"},
					EveningSnack:     "150g de skyr et 15g d'amandes — apporte les 25g de protéines manquantes sans exploser les glucides.",
				},
				Lang:           "fr",
				Period:         analysisPeriodDaily,
				CurrentKcal:    1130,
				TargetKcal:     2336,
				CurrentProtein: 80,
				TargetProtein:  140,
				CurrentCarbs:   100,
				TargetCarbs:    220,
				CurrentFat:     40,
				TargetFat:      70,
				Streak:         5,
			},
			wantContain: []string{
				"📋 Analyse Quotidienne",
				"Bravo pour le suivi aujourd'hui !",
				"🔥 Série : 5 jours",
				"✅ Statut : maintenu",
				"📊 Calories : 1130 / 2336 kcal",
				"🟩",
				"🥩 Protéines : 80 / 140 g",
				"🍚 Glucides : 100 / 220 g",
				"🥑 Lipides : 40 / 70 g",
				"✅ Meilleurs repas",
				"Bowl poulet",
				"🎯 Pour atteindre 100% ce soir",
				"150g de skyr",
			},
			wantAbsent: []string{"Grocery hints", "Streak:", "Top aligned meals"},
		},
		{
			name: "english improvements no snack when over target",
			view: AnalysisView{
				Data: domain.NutritionAnalysis{
					Congratulations:  "Calories ran high — trim evening snacks.",
					StreakMaintained: false,
					Improvements: []domain.MealImprovement{
						{
							MealName:    "Dinner pizza",
							Issue:       "Too high in fat/calories",
							Alternative: "Thin-crust + side salad, add whey shake",
						},
					},
					EveningSnack: "should not appear",
				},
				Lang:        "en",
				Period:      analysisPeriodDaily,
				CurrentKcal: 2600,
				TargetKcal:  2000,
				Streak:      0,
			},
			wantContain: []string{
				"📋 Daily Analysis",
				"🔥 Streak : 0 days",
				"⚠️ Status: at risk / broken",
				"🔧 Improvement ideas",
				"Dinner pizza",
				"🟥",
			},
			wantAbsent: []string{"To hit 100% tonight", "Grocery"},
		},
		{
			name: "weekly french title and no evening snack section",
			view: AnalysisView{
				Data: domain.NutritionAnalysis{
					Congratulations:  "Belle semaine de logging.",
					StreakMaintained: true,
					EveningSnack:     "ignored on weekly",
				},
				Lang:        "fr",
				Period:      analysisPeriodWeekly,
				CurrentKcal: 1980,
				TargetKcal:  2000,
				Streak:      1,
			},
			wantContain: []string{
				"📈 Analyse Hebdomadaire",
				"🔥 Série : 1 jour",
				"🟨",
			},
			wantAbsent: []string{"Pour atteindre 100%"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := BuildAnalysisMessage(tt.view)
			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Fatalf("missing %q in:\n%s", want, got)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Fatalf("unexpected %q in:\n%s", absent, got)
				}
			}
		})
	}
}

func TestBuildCalorieProgressBar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		current int
		target  int
		want    string
	}{
		{name: "zero target", current: 0, target: 0, want: "0%"},
		{name: "half", current: 1000, target: 2000, want: "50%"},
		{name: "over", current: 3000, target: 2000, want: "150%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := BuildCalorieProgressBar(tt.current, tt.target)
			if !strings.Contains(got, tt.want) {
				t.Fatalf("got %q, want substring %q", got, tt.want)
			}
			if !strings.HasPrefix(got, "[") || !strings.Contains(got, "]") {
				t.Fatalf("expected bracketed bar, got %q", got)
			}
		})
	}
}
