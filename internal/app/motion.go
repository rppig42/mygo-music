package app

import (
	"log"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/video"

	"mygo-music/internal/music"
)

// A cover that moves: some sources have, for some albums, a short video that
// loops where the cover is. A plugin says so with motionCover, which answers
// the video's address for a song, an HLS playlist or a file. Only the song
// that plays is asked about, once, and its video plays muted over the still
// cover in Now Playing and on the radio's card, stopping when the song does.

// motionSource is a source that knows covers that move.
type motionSource interface {
	// MotionCover answers the address of the video that is the cover of the
	// song's album, or none.
	MotionCover(id string) (string, error)
}

// motionCover is the moving cover of the song that plays.
type motionCover struct {
	track  string // the song it is of: its source and ID
	player *ui.VideoPlayer
}

// openMotion opens the video at a URL. Tests swap it.
var openMotion = video.OpenURL

// motion is the video that is tr's cover, or nil while there is none to show:
// the source has none, or was not asked yet. Asking for another song's closes
// the one before, so it is only for the song that plays.
func (a *app) motion(tr *music.Track) *ui.VideoPlayer {
	m := &a.motionCover
	if key := tr.Source + "\x00" + tr.ID; m.track != key {
		if m.player != nil {
			m.player.Close()
		}
		*m = motionCover{track: key}
		lib := a.trackLib(tr)
		if src, ok := lib.src.(motionSource); ok && lib.can("motionCover") {
			id := tr.ID
			go func() {
				url, err := src.MotionCover(id)
				a.post(func() {
					if a.motionCover.track != key || err != nil || url == "" {
						return
					}
					p, err := openMotion(url)
					if err != nil {
						log.Printf("motion cover: %v", err)
						return
					}
					a.motionCover.player = p.SetLoop(true).SetMuted(true)
				})
			}()
		}
	}
	p := m.player
	if p == nil || p.Err() != nil {
		return nil
	}
	// It stops with the song, as the cover's breathing does.
	if playing := a.pl.Playing(); playing != p.Playing() {
		if playing {
			p.Play()
		} else {
			p.Pause()
		}
	}
	return p
}

// motionImage shows the video over the still cover it is of, filling the
// same box, once it has a picture: until then the cover shows through.
func motionImage(c *ui.Context, p *ui.VideoPlayer, radius float32) {
	if p == nil {
		return
	}
	v := ui.Video(c, p).Absolute().Top(0).Left(0).Right(0).Bottom(0).Fit(ui.Cover).Radius(radius)
	if !p.Ready() {
		v.Opacity(0)
	}
}

// closeMotion stops the moving cover, as the app quits.
func (a *app) closeMotion() {
	if p := a.motionCover.player; p != nil {
		p.Close()
	}
	a.motionCover = motionCover{}
}
