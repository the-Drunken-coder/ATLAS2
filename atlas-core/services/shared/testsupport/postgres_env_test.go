package testsupport

import "testing"

func TestPostgresUnavailableActionFromEnv(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want postgresUnavailableAction
	}{
		{name: "default fails", env: "", want: postgresUnavailableFail},
		{name: "explicit skip", env: "true", want: postgresUnavailableSkip},
		{name: "other values do not skip", env: "1", want: postgresUnavailableFail},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ATLAS_SKIP_POSTGRES_TESTS", tt.env)
			if got := postgresUnavailableActionFromEnv(); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
