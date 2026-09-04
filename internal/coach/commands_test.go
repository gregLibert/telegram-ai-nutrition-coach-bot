package coach

import "testing"

func TestSplitCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		text     string
		wantCmd  string
		wantArgs string
		wantOK   bool
	}{
		{name: "plain help", text: "/help", wantCmd: "/help", wantOK: true},
		{name: "help with bot suffix", text: "/help@MyNutritionBot", wantCmd: "/help", wantOK: true},
		{name: "portion with args", text: "/portion chicken and rice", wantCmd: "/portion", wantArgs: "chicken and rice", wantOK: true},
		{name: "update_profil legacy", text: "/update_profil", wantCmd: "/update_profil", wantOK: true},
		{name: "update_profile", text: "/update_profile", wantCmd: "/update_profile", wantOK: true},
		{name: "case insensitive", text: "/Help", wantCmd: "/help", wantOK: true},
		{name: "not a command", text: "200g chicken", wantOK: false},
		{name: "empty", text: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd, args, ok := splitCommand(tt.text)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if cmd != tt.wantCmd {
				t.Fatalf("cmd = %q, want %q", cmd, tt.wantCmd)
			}
			if args != tt.wantArgs {
				t.Fatalf("args = %q, want %q", args, tt.wantArgs)
			}
		})
	}
}

func TestRouteNamedCommandRecognizesHelpAndUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		text string
		cmd  string
	}{
		{"/help", cmdHelp},
		{"/help@Bot", cmdHelp},
		{"/update_profile", cmdUpdateProfile},
		{"/update_profil", cmdUpdateProfileFR},
		{"/undo", cmdUndo},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			t.Parallel()
			cmd, _, ok := splitCommand(tt.text)
			if !ok {
				t.Fatal("expected command")
			}
			if cmd != tt.cmd {
				t.Fatalf("got %q, want %q", cmd, tt.cmd)
			}
		})
	}
}
