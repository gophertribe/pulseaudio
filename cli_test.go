package pulseaudio

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

func TestParseSinks(t *testing.T) {
	sinks, err := parseSinks(bytes.NewBufferString(testSinks), logger{})
	require.NoError(t, err)
	if assert.Len(t, sinks, 3) {
		assert.Equal(t, "null", sinks[0].Name)
		assert.Equal(t, uint32(74), sinks[0].CVolume[0])
		assert.Equal(t, true, sinks[0].Muted)
		assert.Equal(t, "alsa_output.zone1", sinks[1].Name)
		assert.Equal(t, uint32(70), sinks[1].CVolume[0])
		assert.Equal(t, uint32(70), sinks[1].CVolume[1])
		assert.Equal(t, uint32(70), sinks[1].CVolume[2])
		assert.Equal(t, uint32(70), sinks[1].CVolume[3])
		assert.Equal(t, false, sinks[1].Muted)
		assert.Equal(t, "test", sinks[2].Name)
		assert.Equal(t, uint32(0), sinks[2].CVolume[0])
	}
}

const testSinks = `
Sink #0
	State: IDLE
	Name: null
	Description: Null Output
	Driver: module-null-sink.c
	Sample Specification: s16le 2ch 44100Hz
	Channel Map: front-left,front-right
	Owner Module: 0
	Mute: yes
	Volume: front-left: 65536 / 74% / 0.00 dB,   front-right: 65536 / 74% / 0.00 dB
	        balance 0.00
	Base Volume: 65536 / 100% / 0.00 dB
	Monitor Source: null.monitor
	Latency: 2101486 usec, configured 2000000 usec
	Flags: DECIBEL_VOLUME LATENCY
	Properties:
		device.description = "Null Output"
		device.class = "abstract"
		device.icon_name = "audio-card"
	Formats:
		pcm

Sink #1
	State: RUNNING
	Name: alsa_output.zone1
	Description: PCM2902C Audio CODEC
	Driver: module-alsa-sink.c
	Sample Specification: s16le 4ch 44100Hz
	Channel Map: front-left,front-right,rear-left,rear-right
	Owner Module: 1
	Mute: no
	Volume: front-left: 45875 /  70% / -9.29 dB,   front-right: 45875 /  70% / -9.29 dB,   rear-left: 45875 /  70% / -9.29 dB,   rear-right: 45875 /  70% / -9.29 dB
	        balance 0.00
	Base Volume: 65536 / 100% / 0.00 dB
	Monitor Source: alsa_output.zone1.monitor
	Latency: 15857 usec, configured 25000 usec
	Flags: HARDWARE DECIBEL_VOLUME LATENCY
	Properties:
		alsa.resolution_bits = "16"
		device.api = "alsa"
		device.class = "sound"
		alsa.class = "generic"
		alsa.subclass = "generic-mix"
		alsa.name = "USB Audio"
		alsa.id = "USB Audio"
		alsa.subdevice = "0"
		alsa.subdevice_name = "subdevice #0"
		alsa.device = "0"
		alsa.card = "4"
		alsa.card_name = "USB AUDIO  CODEC"
		alsa.long_card_name = "BurrBrown from Texas Instruments USB AUDIO  CODEC at usb-1c1b000.usb-1.3, full"
		alsa.driver_name = "snd_usb_audio"
		device.bus_path = "platform-1c1b000.usb-usb-0:1.3:1.0"
		sysfs.path = "/devices/platform/soc/1c1b000.usb/usb3/3-1/3-1.3/3-1.3:1.0/sound/card4"
		udev.id = "usb-BurrBrown_from_Texas_Instruments_USB_AUDIO_CODEC-00"
		device.bus = "usb"
		device.vendor.id = "08bb"
		device.vendor.name = "Texas Instruments"
		device.product.id = "29c2"
		device.product.name = "PCM2902C Audio CODEC"
		device.serial = "BurrBrown_from_Texas_Instruments_USB_AUDIO_CODEC"
		device.string = "zone1"
		device.buffering.buffer_size = "282240"
		device.buffering.fragment_size = "35280"
		device.access_mode = "mmap+timer"
		device.description = "PCM2902C Audio CODEC"
		device.icon_name = "audio-card-usb"
	Formats:
		pcm

Sink #2
	State: IDLE
	Name: test
	Description: Null Output
	Driver: module-null-sink.c
	Sample Specification: s16le 2ch 44100Hz
	Channel Map: front-left,front-right
	Owner Module: 0
	Mute: yes
	Volume: front-left: 0 /   0% / -inf dB,   front-right: 0 /   0% / -inf dB,   rear-left: 0 /   0% / -inf dB,   rear-right: 0 /   0% / -inf dB
	        balance 0.00
	Base Volume: 65536 / 100% / 0.00 dB
	Monitor Source: null.monitor
	Latency: 2101486 usec, configured 2000000 usec
	Flags: DECIBEL_VOLUME LATENCY
	Properties:
		device.description = "Null Output"
		device.class = "abstract"
		device.icon_name = "audio-card"
	Formats:
		pcm
`

type logger struct {
}

func (l logger) Errorf(msg string, args ...interface{}) {
	fmt.Printf(msg+"\n", args)
}
