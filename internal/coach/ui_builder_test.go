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
			wantAbsent: []string{"Grocery hints", "Streak:", "Top aligned meals", "Bilan Hebdomadaire"},
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

func TestBuildWeeklyAnalysisMessage(t *testing.T) {
	t.Parallel()

	got := BuildWeeklyAnalysisMessage(WeeklyAnalysisView{
		Data: domain.WeeklyAnalysis{
			Overview:        "Déficit global respecté — bonne dynamique de perte de poids.",
			MacroBottleneck: "Difficulté à atteindre les protéines le week-end",
			MealTips: []domain.WeeklyMealTip{
				{MealName: "Crêpes du dimanche", Tip: "Réduisez le nombre de crêpes et augmentez la dose de jambon"},
				{MealName: "Purée du soir", Tip: "Optez pour du blanc de poulet en tranches fines"},
			},
			LegumeFocus: "Remplacez la purée de pommes de terre par des lentilles vertes ou une purée de pois cassés avec du fromage blanc",
			GroceryList: []string{"Lentilles vertes", "Blanc de poulet en tranches fines", "Fromage blanc 0%"},
		},
		Lang:          "fr",
		AvgKcal:       1900,
		TargetKcal:    2100,
		AvgProtein:    110,
		TargetProtein: 140,
		AvgCarbs:      180,
		TargetCarbs:   220,
		AvgFat:        60,
		TargetFat:     70,
	})

	wantContain := []string{
		"📋 Bilan Hebdomadaire",
		"Déficit global respecté",
		"📊 Moyenne sur 7 jours : 1900 / 2100 kcal",
		"🥩 Protéines : 110 / 140 g",
		"⚠️ Point d'attention : Difficulté à atteindre les protéines le week-end",
		"💡 Astuces & Remplacements",
		"Crêpes du dimanche",
		"🍲 Focus Légumineuses",
		"lentilles vertes",
		"🛒 Idées Courses pour la semaine pro",
		"Blanc de poulet en tranches fines",
	}
	for _, want := range wantContain {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	for _, absent := range []string{"Série", "Pour atteindre 100%", "Streak"} {
		if strings.Contains(got, absent) {
			t.Fatalf("unexpected %q in:\n%s", absent, got)
		}
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
