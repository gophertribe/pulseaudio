package pulseaudio

import (
	"fmt"
)

const pulseVolumeMax = 0xffff

// Volume returns current audio volume as a number from 0 to 1 (or more than 1 - if volume is boosted).
func (c *Client) Volume() (float32, error) {
	s, err := c.ServerInfo()
	if err != nil {
		return 0, err
	}
	sinks, err := c.Sinks()
	for _, sink := range sinks {
		if sink.Name != s.DefaultSink {
			continue
		}
		return float32(sink.CVolume[0]) / pulseVolumeMax, nil
	}
	return 0, fmt.Errorf("PulseAudio error: couldn't query volume - Sink %s not found", s.DefaultSink)
}

// SetVolume changes the current volume to a specified value from 0 to 1 (or more than 1 - if volume should be boosted).
func (c *Client) SetVolume(volume float32) error {
	s, err := c.ServerInfo()
	if err != nil {
		return err
	}
	return c.setSinkVolume(s.DefaultSink, CVolume{uint32(volume * 0xffff)})
}

func (c *Client) SetSinkVolume(sinkName string, volume float32) error {
	return c.setSinkVolume(sinkName, CVolume{uint32(volume * 0xffff)})
}

func (c *Client) setSinkVolume(sinkName string, cvolume CVolume) error {
	res, err := c.request(commandSetSinkVolume, uint32Tag, uint32(0xffffffff), stringTag, []byte(sinkName), byte(0), cvolume)
	fmt.Println(res.String())
	return err
}

// ToggleMute reverse mute status
func (c *Client) ToggleMute() (bool, error) {
	s, err := c.ServerInfo()
	if err != nil || s == nil {
		return true, err
	}

	muted, err := c.Mute()
	if err != nil {
		return true, err
	}

	err = c.SetMute(!muted)
	return !muted, err
}

// SetMute reverse mute status
func (c *Client) SetMute(mute bool) error {
	s, err := c.ServerInfo()
	if err != nil || s == nil {
		return err
	}
	return c.SetSinkMute(s.DefaultSink, mute)
}

// SetSinkMute reverse mute status
func (c *Client) SetSinkMute(sinkName string, mute bool) error {
	muteCmd := '0'
	if mute {
		muteCmd = '1'
	}
	_, err := c.request(commandSetSinkMute, uint32Tag, uint32(0xffffffff), stringTag, []byte(sinkName), byte(0), uint8(muteCmd))
	return err
}

func (c *Client) Mute() (bool, error) {
	s, err := c.ServerInfo()
	if err != nil || s == nil {
		return false, err
	}

	sinks, err := c.Sinks()
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
