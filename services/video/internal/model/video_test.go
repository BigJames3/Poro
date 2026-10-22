package model

import "testing"

func TestPartLength(t *testing.T) {
	cases := []struct {
		size int64
		part int
		want int64
	}{
		{size: 1, part: 1, want: 1},
		{size: PartSize, part: 1, want: PartSize},
		{size: PartSize + 1, part: 1, want: PartSize},
		{size: PartSize + 1, part: 2, want: 1},
		{size: 3*PartSize - 5, part: 3, want: PartSize - 5},
	}
	for _, c := range cases {
		if got := PartLength(c.size, c.part); got != c.want {
			t.Errorf("PartLength(%d, %d) = %d, want %d", c.size, c.part, got, c.want)
		}
	}
}

func TestPartCountAndKeys(t *testing.T) {
	if PartCount(0) != 0 {
		t.Fatal()
	}
	if PartCount(1) != 1 {
		t.Fatal(PartCount(1))
	}
	if PartCount(PartSize) != 1 {
		t.Fatal()
	}
	if PartCount(PartSize+1) != 2 {
		t.Fatal()
	}
	if SourceKey("u", "v", "mp4") != "videos/u/v/source.mp4" {
		t.Fatal(SourceKey("u", "v", "mp4"))
	}
	if HLSDir("u", "v") != "videos/u/v/hls" {
		t.Fatal()
	}
	if ThumbKey("u", "v") != "videos/u/v/thumb.jpg" {
		t.Fatal()
	}
}

func TestIsSourceKey(t *testing.T) {
	user := "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b"
	video := "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6c"
	for _, ext := range []string{"mp4", "mov", "webm"} {
		if !IsSourceKey(SourceKey(user, video, ext)) {
			t.Fatal(ext)
		}
	}
	for _, key := range []string{
		ThumbKey(user, video),
		HLSDir(user, video) + "/master.m3u8",
		MediaRoot + user + "/" + video + "/hls/source.mp4",
		MediaRoot + "not-a-uuid/" + video + "/source.mp4",
		MediaRoot + user + "/not-a-uuid/source.mp4",
		MediaRoot + user + "/" + video + "/source.",
		MediaRoot + user + "/" + video + "/source.tar.gz",
		"other/" + user + "/" + video + "/source.mp4",
	} {
		if IsSourceKey(key) {
			t.Fatal(key)
		}
	}
}
