package gvrepo

import "testing"

func TestFromFullName(t *testing.T) {
	cases := []struct {
		in                        string
		wantOwner, wantName, host string
		wantErr                   bool
	}{
		{in: "veryviolet/gv", wantOwner: "veryviolet", wantName: "gv", host: "gitverse.ru"},
		{in: "veryviolet/gv.git", wantOwner: "veryviolet", wantName: "gv", host: "gitverse.ru"},
		{in: "git.example.com/veryviolet/gv", wantOwner: "veryviolet", wantName: "gv", host: "git.example.com"},
		{in: "https://gitverse.ru/veryviolet/gv.git", wantOwner: "veryviolet", wantName: "gv", host: "gitverse.ru"},
		{in: "git@gitverse.ru:veryviolet/gv.git", wantOwner: "veryviolet", wantName: "gv", host: "gitverse.ru"},
		{in: "gv", wantErr: true},
		{in: "", wantErr: true},
		{in: "veryviolet//gv", wantErr: true},
	}

	for _, tc := range cases {
		got, err := FromFullName(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("FromFullName(%q) = %v, want an error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("FromFullName(%q): %v", tc.in, err)
			continue
		}
		if got.RepoOwner() != tc.wantOwner || got.RepoName() != tc.wantName || got.RepoHost() != tc.host {
			t.Errorf("FromFullName(%q) = %s/%s on %s, want %s/%s on %s",
				tc.in, got.RepoOwner(), got.RepoName(), got.RepoHost(),
				tc.wantOwner, tc.wantName, tc.host)
		}
	}
}

func TestCloneURL(t *testing.T) {
	r := New("veryviolet", "gv")
	if got, want := CloneURL(r, "https"), "https://gitverse.ru/veryviolet/gv.git"; got != want {
		t.Errorf("https clone URL = %q, want %q", got, want)
	}
	if got, want := CloneURL(r, "ssh"), "git@gitverse.ru:veryviolet/gv.git"; got != want {
		t.Errorf("ssh clone URL = %q, want %q", got, want)
	}
}

func TestWebURL(t *testing.T) {
	r := New("veryviolet", "gv")
	if got, want := WebURL(r, ""), "https://gitverse.ru/veryviolet/gv"; got != want {
		t.Errorf("WebURL = %q, want %q", got, want)
	}
	if got, want := WebURL(r, "pulls"), "https://gitverse.ru/veryviolet/gv/pulls"; got != want {
		t.Errorf("WebURL = %q, want %q", got, want)
	}
}

func TestIsSameIgnoresCase(t *testing.T) {
	if !IsSame(New("VeryViolet", "GV"), New("veryviolet", "gv")) {
		t.Error("IsSame should ignore case")
	}
	if IsSame(New("veryviolet", "gv"), NewWithHost("veryviolet", "gv", "git.example.com")) {
		t.Error("repositories on different hosts are not the same")
	}
}
