package capability

import "testing"

func TestHas(t *testing.T) {
	cases := []struct {
		header string
		want   bool
	}{
		{"", false},
		{"photo-posts", true},
		{"Photo-Posts", true},
		{" reels , photo-posts ", true},
		{"photo-posts-v2", false},
		{"photo", false},
		{"unknown,other", false},
	}
	for _, c := range cases {
		if got := Has(c.header, PhotoPosts); got != c.want {
			t.Errorf("Has(%q) = %v, want %v", c.header, got, c.want)
		}
	}
}

func TestMediaTypesAndScope(t *testing.T) {
	if got := MediaTypes(false); len(got) != 1 || got[0] != MediaVideo {
		t.Fatalf("without photo posts: %v", got)
	}
	if got := MediaTypes(true); len(got) != 2 || got[1] != MediaPhoto {
		t.Fatalf("with photo posts: %v", got)
	}
	if Scope(false) == Scope(true) {
		t.Fatal("scopes must differ")
	}
}
