package model

import "testing"

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
