package app

import (
	"image"
	"io"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"mygo-music/internal/music"
)

// oneFrame is a video of a single frame.
type oneFrame struct{ done bool }

func (d *oneFrame) Next() (ui.VideoFrame, error) {
	if d.done {
		return ui.VideoFrame{}, io.EOF
	}
	d.done = true
	return ui.VideoFrame{Image: image.NewRGBA(image.Rect(0, 0, 2, 2))}, nil
}

func (d *oneFrame) Rewind() error { d.done = false; return nil }

// The song that plays is asked for its moving cover once, and the video at
// the address it answers opens; another song is asked anew and gets a video
// of its own; a source without motionCover is never asked.
func TestMotionCoverFollowsTheSong(t *testing.T) {
	a := newTestApp()
	ch := posts(a)
	var asked, opened []string
	fake(a).motionCover = func(id string) (string, error) {
		asked = append(asked, id)
		return "https://example.com/" + id + ".m3u8", nil
	}
	a.libs[0].caps["motionCover"] = true
	defer func(open func(string) (*ui.VideoPlayer, error)) { openMotion = open }(openMotion)
	openMotion = func(url string) (*ui.VideoPlayer, error) {
		opened = append(opened, url)
		return ui.NewVideoPlayer(&oneFrame{})
	}

	one, two, three := &music.Track{ID: "1", Source: "fake"}, &music.Track{ID: "2", Source: "fake"}, &music.Track{ID: "3", Source: "fake"}
	if a.motion(one) != nil {
		t.Fatal("a video showed before the source answered")
	}
	runPost(t, ch)
	first := a.motion(one)
	if first == nil {
		t.Fatal("the source answered an address, and no video opened")
	}
	if a.motion(one) != first || len(asked) != 1 {
		t.Fatalf("the same song was asked about %d times", len(asked))
	}

	a.motion(two)
	runPost(t, ch)
	if second := a.motion(two); second == nil || second == first {
		t.Fatal("the next song did not get a video of its own")
	}
	if want := []string{"https://example.com/1.m3u8", "https://example.com/2.m3u8"}; !slices.Equal(opened, want) {
		t.Fatalf("opened %v, want %v", opened, want)
	}

	delete(a.libs[0].caps, "motionCover")
	if a.motion(three) != nil || len(asked) != 2 {
		t.Fatal("a source without motionCover was asked for one")
	}
	a.closeMotion()
}

// An answer that comes after the listener went on to another song opens
// nothing: the video would be of a song that no longer plays.
func TestMotionCoverAnswerForASongLeftIsDropped(t *testing.T) {
	a := newTestApp()
	ch := posts(a)
	fake(a).motionCover = func(id string) (string, error) { return "https://example.com/" + id + ".m3u8", nil }
	a.libs[0].caps["motionCover"] = true
	opened := 0
	defer func(open func(string) (*ui.VideoPlayer, error)) { openMotion = open }(openMotion)
	openMotion = func(string) (*ui.VideoPlayer, error) { opened++; return ui.NewVideoPlayer(&oneFrame{}) }

	a.motion(&music.Track{ID: "1", Source: "fake"})
	a.motion(&music.Track{ID: "2", Source: "fake"})
	// The two answers come in either order: only the one for the song that
	// plays opens a video.
	runPost(t, ch)
	runPost(t, ch)
	if opened != 1 {
		t.Fatalf("opened %d videos, want 1, for the song that plays", opened)
	}
	if a.motionCover.track != "fake\x002" || a.motionCover.player == nil {
		t.Fatal("the video is not the playing song's")
	}
	a.closeMotion()
}
