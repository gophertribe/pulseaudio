package pulseaudio

import (
	"context"
	"fmt"
)

const pulseVolumeMax = 0xffff

// Volume returns current audio volume as a number from 0 to 1 (or more than 1 - if volume is boosted).
func (c *Client) Volume(ctx context.Context) (float32, error) {
	s, err := c.ServerInfo(ctx)
	if err != nil {
		return 0, err
	}
	sinks, err := c.Sinks(ctx)
	for _, sink := range sinks {
		if sink.Name != s.DefaultSink {
			continue
		}
		return float32(sink.CVolume[0]) / pulseVolumeMax, nil
	}
	return 0, fmt.Errorf("PulseAudio error: couldn't query volume - Sink %s not found", s.DefaultSink)
}

// SetVolume changes the current volume to a specified value from 0 to 1 (or more than 1 - if volume should be boosted).
func (c *Client) SetVolume(ctx context.Context, volume float32) error {
	s, err := c.ServerInfo(ctx)
	if err != nil {
		return err
	}
	return c.setSinkVolume(ctx, s.DefaultSink, CVolume{uint32(volume * 0xffff)})
}

func (c *Client) SetSinkVolume(ctx context.Context, sinkName string, volume float32) error {
	return c.setSinkVolume(ctx, sinkName, CVolume{uint32(volume * 0xffff)})
}

func (c *Client) setSinkVolume(ctx context.Context, sinkName string, cvolume CVolume) error {
	res, err := c.request(ctx, commandSetSinkVolume, uint32Tag, uint32(0xffffffff), stringTag, []byte(sinkName), byte(0), cvolume)
	fmt.Println(res.String())
	return err
}

// ToggleMute reverse mute status
func (c *Client) ToggleMute(ctx context.Context) (bool, error) {
	s, err := c.ServerInfo(ctx)
	if err != nil || s == nil {
		return true, err
	}

	muted, err := c.Mute(ctx)
	if err != nil {
		return true, err
	}

	err = c.SetMute(ctx, !muted)
	return !muted, err
}

// SetMute reverse mute status
func (c *Client) SetMute(ctx context.Context, mute bool) error {
	s, err := c.ServerInfo(ctx)
	if err != nil || s == nil {
		return err
	}
	return c.SetSinkMute(ctx, s.DefaultSink, mute)
}

// SetSinkMute reverse mute status
func (c *Client) SetSinkMute(ctx context.Context, sinkName string, mute bool) error {
	muteCmd := '0'
	if mute {
		muteCmd = '1'
	}
	_, err := c.request(ctx, commandSetSinkMute, uint32Tag, uint32(0xffffffff), stringTag, []byte(sinkName), byte(0), uint8(muteCmd))
	return err
}

func (c *Client) Mute(ctx context.Context) (bool, error) {
	s, err := c.ServerInfo(ctx)
	if err != nil || s == nil {
		return false, err
	}

	sinks, err := c.Sinks(ctx)
	if err != nil {
		return false, err
	}
	for _, sink := range sinks {
		if sink.Name != s.DefaultSink {
			continue
		}
		return sink.Muted, nil
	}
	return true, fmt.Errorf("couldn't find Sink")
}
