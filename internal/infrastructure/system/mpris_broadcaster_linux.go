//go:build linux

package system

import (
	"fmt"
	"sync"
	"time"

	"github.com/amurru/gocaster/internal/domain"
	"github.com/godbus/dbus/v5"
)

const (
	mprisBusName     = "org.mpris.MediaPlayer2.gocaster"
	mprisObjectPath  = "/org/mpris/MediaPlayer2"
	mprisInterface   = "org.mpris.MediaPlayer2"
	playerInterface  = "org.mpris.MediaPlayer2.Player"
	propInterface    = "org.freedesktop.DBus.Properties"
	introspectionXML = `
<node>
  <interface name="org.mpris.MediaPlayer2">
    <method name="Raise"/>
    <method name="Quit"/>
    <property name="CanQuit" type="b" access="read"/>
    <property name="CanRaise" type="b" access="read"/>
    <property name="CanSetFullscreen" type="b" access="read"/>
    <property name="HasTrackList" type="b" access="read"/>
    <property name="Identity" type="s" access="read"/>
    <property name="DesktopEntry" type="s" access="read"/>
    <property name="SupportedUriSchemes" type="as" access="read"/>
    <property name="SupportedMimeTypes" type="as" access="read"/>
  </interface>
  <interface name="org.mpris.MediaPlayer2.Player">
    <method name="Play"/>
    <method name="Pause"/>
    <method name="Stop"/>
    <method name="PlayPause"/>
    <method name="Seek"/>
    <method name="SetPosition"/>
    <method name="Next"/>
    <method name="Previous"/>
    <property name="PlaybackStatus" type="s" access="read"/>
    <property name="Rate" type="d" access="readwrite"/>
    <property name="MinimumRate" type="d" access="read"/>
    <property name="MaximumRate" type="d" access="read"/>
    <property name="CanGoNext" type="b" access="read"/>
    <property name="CanGoPrevious" type="b" access="read"/>
    <property name="CanPlay" type="b" access="read"/>
    <property name="CanPause" type="b" access="read"/>
    <property name="CanSeek" type="b" access="read"/>
    <property name="CanControl" type="b" access="read"/>
    <property name="Metadata" type="a{sv}" access="read"/>
    <property name="Position" type="x" access="read"/>
  </interface>
  <interface name="org.freedesktop.DBus.Properties">
    <method name="Get"/>
    <method name="GetAll"/>
    <signal name="PropertiesChanged"/>
  </interface>
  <interface name="org.freedesktop.DBus.Introspectable">
    <method name="Introspect"/>
  </interface>
  <interface name="org.freedesktop.DBus.Peer">
    <method name="Ping"/>
  </interface>
</node>
`
)

type mprisBroadcaster struct {
	mu         sync.Mutex
	conn       *dbus.Conn
	controller domain.PlaybackController

	state    domain.PlaybackState
	metadata domain.PlaybackMetadata
	position float64
	duration float64
	running  bool
}

type noOpBroadcaster struct{}

func NewMPRISBroadcaster() (domain.PlaybackBroadcaster, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return &noOpBroadcaster{}, fmt.Errorf("connect to D-Bus session bus: %w", err)
	}

	b := &mprisBroadcaster{
		conn:    conn,
		running: true,
	}

	if err := b.setup(); err != nil {
		b.conn.Close()
		return &noOpBroadcaster{}, fmt.Errorf("setup MPRIS service: %w", err)
	}

	go b.positionUpdater()

	return b, nil
}

func (b *noOpBroadcaster) PublishState(state domain.PlaybackState, metadata domain.PlaybackMetadata) error {
	return nil
}

func (b *noOpBroadcaster) PublishPosition(positionSec float64, durationSec float64) error {
	return nil
}

func (b *noOpBroadcaster) Close() error {
	return nil
}

func (b *noOpBroadcaster) SetController(controller domain.PlaybackController) {
}

func (b *mprisBroadcaster) setup() error {
	reply, err := b.conn.RequestName(mprisBusName, dbus.NameFlagAllowReplacement|dbus.NameFlagDoNotQueue)
	if err != nil {
		return err
	}
	if reply != dbus.RequestNameReplyPrimaryOwner && reply != dbus.RequestNameReplyAlreadyOwner {
		return fmt.Errorf("failed to own %s (reply=%d)", mprisBusName, reply)
	}

	if err := b.conn.Export(b, mprisObjectPath, mprisInterface); err != nil {
		return err
	}

	if err := b.conn.Export(b, mprisObjectPath, playerInterface); err != nil {
		return err
	}

	if err := b.conn.Export(b, mprisObjectPath, propInterface); err != nil {
		return err
	}

	b.conn.Export(b, mprisObjectPath, "org.freedesktop.DBus.Introspectable")
	b.conn.Export(b, mprisObjectPath, "org.freedesktop.DBus.Peer")

	b.conn.Emit(mprisObjectPath, propInterface+".PropertiesChanged", propInterface, map[string]dbus.Variant{
		"CanQuit":             dbus.MakeVariant(true),
		"CanRaise":            dbus.MakeVariant(false),
		"CanSetFullscreen":    dbus.MakeVariant(false),
		"HasTrackList":        dbus.MakeVariant(false),
		"Identity":            dbus.MakeVariant("Gocaster"),
		"DesktopEntry":        dbus.MakeVariant("gocaster"),
		"SupportedUriSchemes": dbus.MakeVariant([]string{"https", "file"}),
		"SupportedMimeTypes":  dbus.MakeVariant([]string{}),
	}, []string{})

	return nil
}

func (b *mprisBroadcaster) positionUpdater() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C

		b.mu.Lock()
		if !b.running {
			b.mu.Unlock()
			return
		}

		state := b.state
		ctrl := b.controller
		b.mu.Unlock()

		if state != domain.PlaybackStatePlaying || ctrl == nil {
			continue
		}

		status, err := ctrl.Status()
		if err == nil {
			_ = b.PublishPosition(status.PositionSec, status.DurationSec)
		}
	}
}

func (b *mprisBroadcaster) SetController(controller domain.PlaybackController) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.controller = controller
}

func (b *mprisBroadcaster) PublishState(state domain.PlaybackState, metadata domain.PlaybackMetadata) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.state = state
	b.metadata = metadata

	playbackStatus := map[string]dbus.Variant{
		"PlaybackStatus": dbus.MakeVariant(toMPRISPlaybackStatus(state)),
		"Rate":           dbus.MakeVariant(1.0),
		"MinimumRate":    dbus.MakeVariant(1.0),
		"MaximumRate":    dbus.MakeVariant(1.0),
		"CanGoNext":      dbus.MakeVariant(metadata.CanGoNext),
		"CanGoPrevious":  dbus.MakeVariant(metadata.CanGoPrevious),
		"CanPlay":        dbus.MakeVariant(true),
		"CanPause":       dbus.MakeVariant(true),
		"CanSeek":        dbus.MakeVariant(metadata.CanSeek),
		"CanControl":     dbus.MakeVariant(true),
	}

	if metadata.Source != "" {
		playbackStatus["Metadata"] = dbus.MakeVariant(b.buildMetadata(metadata))
	}

	b.conn.Emit(mprisObjectPath, propInterface+".PropertiesChanged", playerInterface, playbackStatus, []string{})

	return nil
}

func (b *mprisBroadcaster) buildMetadata(m domain.PlaybackMetadata) map[string]dbus.Variant {
	meta := make(map[string]dbus.Variant)

	if m.EpisodeTitle != "" {
		meta["xesam:title"] = dbus.MakeVariant(m.EpisodeTitle)
	}
	if m.PodcastTitle != "" {
		meta["xesam:album"] = dbus.MakeVariant(m.PodcastTitle)
	}
	if m.Source != "" {
		meta["xesam:url"] = dbus.MakeVariant(m.Source)
	}
	if m.DurationSec > 0 {
		meta["mpris:length"] = dbus.MakeVariant(int64(m.DurationSec * 1e6))
	}

	return meta
}

func (b *mprisBroadcaster) PublishPosition(positionSec float64, durationSec float64) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.position = positionSec
	b.duration = durationSec

	position := map[string]dbus.Variant{"Position": dbus.MakeVariant(int64(positionSec * 1e6))}
	b.conn.Emit(mprisObjectPath, propInterface+".PropertiesChanged", playerInterface, position, []string{})

	return nil
}

func (b *mprisBroadcaster) Close() error {
	b.mu.Lock()
	b.running = false
	b.mu.Unlock()
	if b.conn != nil {
		b.conn.ReleaseName(mprisBusName)
		return b.conn.Close()
	}
	return nil
}

func (b *mprisBroadcaster) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	props := make(map[string]dbus.Variant)

	switch iface {
	case mprisInterface:
		props["CanQuit"] = dbus.MakeVariant(true)
		props["CanRaise"] = dbus.MakeVariant(false)
		props["CanSetFullscreen"] = dbus.MakeVariant(false)
		props["HasTrackList"] = dbus.MakeVariant(false)
		props["Identity"] = dbus.MakeVariant("Gocaster")
		props["DesktopEntry"] = dbus.MakeVariant("gocaster")
		props["SupportedUriSchemes"] = dbus.MakeVariant([]string{"https", "file"})
		props["SupportedMimeTypes"] = dbus.MakeVariant([]string{})
	case playerInterface:
		props["PlaybackStatus"] = dbus.MakeVariant(toMPRISPlaybackStatus(b.state))
		props["Rate"] = dbus.MakeVariant(1.0)
		props["MinimumRate"] = dbus.MakeVariant(1.0)
		props["MaximumRate"] = dbus.MakeVariant(1.0)
		props["CanGoNext"] = dbus.MakeVariant(b.metadata.CanGoNext)
		props["CanGoPrevious"] = dbus.MakeVariant(b.metadata.CanGoPrevious)
		props["CanPlay"] = dbus.MakeVariant(true)
		props["CanPause"] = dbus.MakeVariant(true)
		props["CanSeek"] = dbus.MakeVariant(b.metadata.CanSeek)
		props["CanControl"] = dbus.MakeVariant(true)
		props["Position"] = dbus.MakeVariant(int64(b.position * 1e6))
		if b.metadata.Source != "" {
			props["Metadata"] = dbus.MakeVariant(b.buildMetadata(b.metadata))
		}
	}

	return props, nil
}

func (b *mprisBroadcaster) Get(iface, prop string) (dbus.Variant, *dbus.Error) {
	props, err := b.GetAll(iface)
	if err != nil {
		return dbus.Variant{}, err
	}
	if v, ok := props[prop]; ok {
		return v, nil
	}
	return dbus.Variant{}, dbus.NewError(fmt.Sprintf("org.freedesktop.DBus.Error.InvalidProperty: %s", prop), nil)
}

func (b *mprisBroadcaster) Open() *dbus.Error {
	return nil
}

func (b *mprisBroadcaster) Quit() *dbus.Error {
	return nil
}

func (b *mprisBroadcaster) Raise() *dbus.Error {
	return nil
}

func (b *mprisBroadcaster) Play() *dbus.Error {
	b.mu.Lock()
	ctrl := b.controller
	b.mu.Unlock()

	if ctrl != nil {
		status, err := ctrl.Status()
		if err == nil && status.State == domain.PlaybackStatePaused {
			if err := ctrl.Resume(); err != nil {
				return dbus.NewError("org.mpris.MediaPlayer2.Error", []interface{}{err.Error()})
			}
			return nil
		}

		if err := ctrl.Play(0); err != nil {
			return dbus.NewError("org.mpris.MediaPlayer2.Error", []interface{}{err.Error()})
		}
	}
	return nil
}

func (b *mprisBroadcaster) Pause() *dbus.Error {
	b.mu.Lock()
	ctrl := b.controller
	b.mu.Unlock()
	if ctrl != nil {
		if err := ctrl.Pause(); err != nil {
			return dbus.NewError("org.mpris.MediaPlayer2.Error", []interface{}{err.Error()})
		}
	}
	return nil
}

func (b *mprisBroadcaster) PlayPause() *dbus.Error {
	b.mu.Lock()
	ctrl := b.controller
	b.mu.Unlock()
	if ctrl != nil {
		if err := ctrl.PlayPause(); err != nil {
			return dbus.NewError("org.mpris.MediaPlayer2.Error", []interface{}{err.Error()})
		}
	}
	return nil
}

func (b *mprisBroadcaster) Stop() *dbus.Error {
	b.mu.Lock()
	ctrl := b.controller
	b.mu.Unlock()
	if ctrl != nil {
		if err := ctrl.Stop(); err != nil {
			return dbus.NewError("org.mpris.MediaPlayer2.Error", []interface{}{err.Error()})
		}
	}
	return nil
}

func (b *mprisBroadcaster) Seek(to int64) (int64, *dbus.Error) {
	b.mu.Lock()
	ctrl := b.controller
	b.mu.Unlock()
	if ctrl != nil {
		positionSec := float64(to) / 1e6
		if err := ctrl.SeekTo(positionSec); err != nil {
			return 0, dbus.NewError("org.mpris.MediaPlayer2.Error", []interface{}{err.Error()})
		}
	}
	return to, nil
}

func (b *mprisBroadcaster) SetPosition(trackId string, position int64) *dbus.Error {
	_, err := b.Seek(position)
	return err
}

func (b *mprisBroadcaster) Next() *dbus.Error {
	return dbus.NewError("org.mpris.MediaPlayer2.Error", []interface{}{"Not supported"})
}

func (b *mprisBroadcaster) Previous() *dbus.Error {
	return dbus.NewError("org.mpris.MediaPlayer2.Error", []interface{}{"Not supported"})
}

func (b *mprisBroadcaster) Introspect() string {
	return introspectionXML
}

func (b *mprisBroadcaster) Ping() {}

func toMPRISPlaybackStatus(state domain.PlaybackState) string {
	switch state {
	case domain.PlaybackStatePlaying:
		return "Playing"
	case domain.PlaybackStatePaused:
		return "Paused"
	default:
		return "Stopped"
	}
}
