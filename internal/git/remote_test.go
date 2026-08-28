package git

import "testing"

func TestParseRemotes(t *testing.T) {
	lines := []string{
		"origin\thttps://gitverse.ru/veryviolet/gv.git (fetch)",
		"origin\thttps://gitverse.ru/veryviolet/gv.git (push)",
		"upstream\tgit@gitverse.ru:gitverse/gv.git (fetch)",
		"upstream\tgit@gitverse.ru:gitverse/gv.git (push)",
		"backup\t/srv/mirrors/gv.git (fetch)",
	}

	remotes := parseRemotes(lines)
	if len(remotes) != 3 {
		t.Fatalf("got %d remotes, want 3", len(remotes))
	}
	// upstream sorts ahead of origin so that a fork's parent wins.
	if remotes[0].Name != "upstream" || remotes[1].Name != "origin" {
		t.Errorf("order = %s, %s, %s; want upstream, origin, backup",
			remotes[0].Name, remotes[1].Name, remotes[2].Name)
	}
	if got := remotes[0].FetchURL.Hostname(); got != "gitverse.ru" {
		t.Errorf("scp-like URL host = %q, want gitverse.ru", got)
	}
	if got := remotes[0].FetchURL.Path; got != "/gitverse/gv.git" {
		t.Errorf("scp-like URL path = %q, want /gitverse/gv.git", got)
	}
	if got := remotes[1].PushURL.String(); got != "https://gitverse.ru/veryviolet/gv.git" {
		t.Errorf("push URL = %q", got)
	}
}

func TestRemoteSetFindByName(t *testing.T) {
	remotes := RemoteSet{{Name: "origin"}, {Name: "fork"}}

	if r, ok := remotes.FindByName("fork"); !ok || r.Name != "fork" {
		t.Error("FindByName should find an exact match")
	}
	if r, ok := remotes.FindByName("nope", "origin"); !ok || r.Name != "origin" {
		t.Error("FindByName should try names in order")
	}
	if r, ok := remotes.FindByName("nope", "*"); !ok || r.Name != "origin" {
		t.Error(`FindByName("*") should fall back to the first remote`)
	}
	if _, ok := remotes.FindByName("missing"); ok {
		t.Error("FindByName should report a miss")
	}
}

func TestParseURL(t *testing.T) {
	cases := map[string]string{
		"https://gitverse.ru/o/r.git":   "gitverse.ru",
		"git@gitverse.ru:o/r.git":       "gitverse.ru",
		"ssh://git@gitverse.ru/o/r.git": "gitverse.ru",
		"git+ssh://gitverse.ru/o/r":     "gitverse.ru",
	}
	for in, wantHost := range cases {
		u, err := ParseURL(in)
		if err != nil {
			t.Errorf("ParseURL(%q): %v", in, err)
			continue
		}
		if u.Hostname() != wantHost {
			t.Errorf("ParseURL(%q).Hostname() = %q, want %q", in, u.Hostname(), wantHost)
		}
	}
}
