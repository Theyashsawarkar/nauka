package tui

import (
	"testing"

	"github.com/Theyashsawarkar/nauka/internal/dock"
)

func TestShellJoin(t *testing.T) {
	got := shellJoin([]string{"docker", "exec", "-it", "abc", "sh", "-c", "command -v bash && exec bash || exec sh", "it's"})
	want := `docker exec -it abc sh -c 'command -v bash && exec bash || exec sh' 'it'\''s'`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestShortName(t *testing.T) {
	for _, c := range []struct{ name, project, want string }{
		{"onadssuper-postgres-1", "onadssuper", "postgres-1"},
		{"app_web_1", "app", "web_1"},
		{"lonely", "", "lonely"},
		{"other-db-1", "onadssuper", "other-db-1"},
	} {
		if got := shortName(&dock.Container{Name: c.name, Project: c.project}); got != c.want {
			t.Errorf("%s: got %s", c.name, got)
		}
	}
}
