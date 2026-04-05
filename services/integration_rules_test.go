package services

import (
	"testing"
)

func TestIntegrationRules_ApplyRules(t *testing.T) {
	svc := NewIntegrationRulesService(nil)
	baseTargets := &NutritionTargets{
		Calories: 2500,
		Protein:  150,
		Carbs:    250,
		Fat:      70,
	}

	tests := []struct {
		name       string
		ctx        WorkoutNutritionContext
		wantDeltaC int
		wantDeltaP int
		wantRules  []string
	}{
		{
			name: "Leg Day Bonus",
			ctx: WorkoutNutritionContext{
				HasWorkout:  true,
				WorkoutType: "legs",
				Goal:        "maintain",
			},
			wantDeltaC: 200,
			wantDeltaP: 0,
			wantRules:  []string{"leg_day_bonus"},
		},
		{
			name: "Rest Day",
			ctx: WorkoutNutritionContext{
				HasWorkout: false,
				Goal:       "maintain",
			},
			wantDeltaC: -200,
			wantDeltaP: 0,
			wantRules:  []string{"rest_day_adjustment"},
		},
		{
			name: "Cardio Bonus",
			ctx: WorkoutNutritionContext{
				HasWorkout:      true,
				WorkoutType:     "cardio",
				WorkoutDuration: 30,
				Goal:            "maintain",
			},
			wantDeltaC: 300,
			wantDeltaP: 0,
			wantRules:  []string{"cardio_bonus"},
		},
		{
			name: "High Volume Week",
			ctx: WorkoutNutritionContext{
				HasWorkout:     true,
				WorkoutType:    "push", // No daily bonus
				WeeklyWorkouts: 4,
				WeeklyVolume:   11000,
				Goal:           "maintain",
			},
			wantDeltaC: int(2500 * 0.15),
			wantDeltaP: 0,
			wantRules:  []string{"high_volume_week"},
		},
		{
			name: "Recovery Warning",
			ctx: WorkoutNutritionContext{
				HasWorkout:    true,
				WorkoutType:   "push",
				WorkoutVolume: 6000,
				ProteinIntake: 80, // Far below 150
				Goal:          "maintain",
			},
			wantDeltaC: 0,
			wantDeltaP: 0,
			wantRules:  []string{"recovery_warning"},
		},
		{
			name: "Goal Alignment - Muscle Gain",
			ctx: WorkoutNutritionContext{
				HasWorkout:    false,
				Goal:          "build_muscle",
				CalorieIntake: 2000, // Below 2500*0.9 (2250)
				ProteinIntake: 100,  // Below 150*0.8 (120)
			},
			// Rest day reduces target to 2300.
			wantDeltaC: -200,
			wantDeltaP: 0,
			wantRules:  []string{"rest_day_adjustment", "muscle_gain_calorie_warning", "muscle_gain_protein_warning"},
		},
		{
			name: "Combined Precedence (Legs + High Volume)",
			ctx: WorkoutNutritionContext{
				HasWorkout:     true,
				WorkoutType:    "legs",
				WeeklyWorkouts: 4,
				WeeklyVolume:   11000,
				Goal:           "maintain",
			},
			wantDeltaC: 200 + int(2500*0.15),
			wantDeltaP: 0,
			wantRules:  []string{"leg_day_bonus", "high_volume_week"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.ApplyIntegrationRules(tt.ctx, baseTargets)

			if got.CalorieDelta != tt.wantDeltaC {
				t.Errorf("CalorieDelta = %v, want %v", got.CalorieDelta, tt.wantDeltaC)
			}
			if got.ProteinDelta != tt.wantDeltaP {
				t.Errorf("ProteinDelta = %v, want %v", got.ProteinDelta, tt.wantDeltaP)
			}

			// Check if expected rules fired
			var gotRules []string
			for _, r := range got.Rules {
				gotRules = append(gotRules, r.ID)
			}

			if len(gotRules) != len(tt.wantRules) {
				t.Errorf("got rules %v, want %v", gotRules, tt.wantRules)
			} else {
				for i, ruleID := range tt.wantRules {
					if gotRules[i] != ruleID {
						t.Errorf("rule %d = %v, want %v", i, gotRules[i], ruleID)
					}
				}
			}
		})
	}
}
