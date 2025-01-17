package pulseaudio

import (
	"context"
	"io"
)

type Server struct {
	PackageName    string
	PackageVersion string
	User           string
	Hostname       string
	SampleSpec     SampleSpec
	DefaultSink    string
	DefaultSource  string
	Cookie         uint32
	ChannelMap     ChannelMap
}

func (s *Server) ReadFrom(r io.Reader) (int64, error) {
	return 0, bread(r,
		stringTag, &s.PackageName,
		stringTag, &s.PackageVersion,
		stringTag, &s.User,
		stringTag, &s.Hostname,
		&s.SampleSpec,
		stringTag, &s.DefaultSink,
		stringTag, &s.DefaultSource,
		uint32Tag, &s.Cookie,
		&s.ChannelMap)
}

type Module struct {
	Index    uint32
	Name     string
	Argument string
	NUsed    uint32
	PropList map[string]string
}

func (m *Module) ReadFrom(r io.Reader) (int64, error) {
	err := bread(r,
		uint32Tag, &m.Index,
		stringTag, &m.Name,
		stringTag, &m.Argument,
		uint32Tag, &m.NUsed)
	if err != nil {
		return 0, err
	}
	err = bread(r, &m.PropList)
	return 0, nil
}

type Sink struct {
	Index              uint32
	Name               string
	Description        string
	SampleSpec         SampleSpec
	ChannelMap         ChannelMap
	ModuleIndex        uint32
	CVolume            CVolume
	Muted              bool
	MonitorSourceIndex uint32
	MonitorSourceName  string
	Latency            uint64
	Driver             string
	Flags              uint32
	PropList           map[string]string
	RequestedLatency   uint64
	BaseVolume         uint32
	SinkState          uint32
	NVolumeSteps       uint32
	CardIndex          uint32
	Ports              []SinkPort
	ActivePortName     string
	Formats            []FormatInfo
}

func (s *Sink) ReadFrom(r io.Reader) (int64, error) {
	var portCount uint32
	err := bread(r,
		uint32Tag, &s.Index,
		stringTag, &s.Name,
		stringTag, &s.Description,
		&s.SampleSpec,
		&s.ChannelMap,
		uint32Tag, &s.ModuleIndex,
		&s.CVolume,
		&s.Muted,
		uint32Tag, &s.MonitorSourceIndex,
		stringTag, &s.MonitorSourceName,
		usecTag, &s.Latency,
		stringTag, &s.Driver,
		uint32Tag, &s.Flags,
		&s.PropList,
		usecTag, &s.RequestedLatency,
		volumeTag, &s.BaseVolume,
		uint32Tag, &s.SinkState,
		uint32Tag, &s.NVolumeSteps,
		uint32Tag, &s.CardIndex,
		uint32Tag, &portCount)
	if err != nil {
		return 0, err
	}
	s.Ports = make([]SinkPort, portCount)
	for i := uint32(0); i < portCount; i++ {
		err = bread(r, &s.Ports[i])
		if err != nil {
			return 0, err
		}
	}
	if portCount == 0 {
		err = bread(r, stringNullTag)
		if err != nil {
			return 0, err
		}
	} else {
		err = bread(r, stringTag, &s.ActivePortName)
		if err != nil {
			return 0, err
		}
	}

	var formatCount uint8
	err = bread(r,
		uint8Tag, &formatCount)
	if err != nil {
		return 0, err
	}
	s.Formats = make([]FormatInfo, formatCount)
	for i := uint8(0); i < formatCount; i++ {
		err = bread(r, &s.Formats[i])
		if err != nil {
			return 0, err
		}
	}
	return 0, nil
}

type FormatInfo struct {
	Encoding byte
	PropList map[string]string
}

func (i *FormatInfo) ReadFrom(r io.Reader) (int64, error) {
	return 0, bread(r, formatInfoTag, uint8Tag, &i.Encoding, &i.PropList)
}

type SinkPort struct {
	Name, Description string
	Priority          uint32
	Available         uint32
}

func (p *SinkPort) ReadFrom(r io.Reader) (int64, error) {
	return 0, bread(r,
		stringTag, &p.Name,
		stringTag, &p.Description,
		uint32Tag, &p.Priority,
		uint32Tag, &p.Available)
}

type CVolume []uint32

func (v *CVolume) ReadFrom(r io.Reader) (int64, error) {
	var n byte
	err := bread(r, cvolumeTag, &n)
	if err != nil {
		return 0, err
	}
	*v = make([]uint32, n)
	return 0, bread(r, []uint32(*v))
}

type ChannelMap []byte

func (m *ChannelMap) ReadFrom(r io.Reader) (int64, error) {
	var n byte
	err := bread(r, channelMapTag, &n)
	if err != nil {
		return 0, err
	}
	*m = make([]byte, n)
	_, err = r.Read(*m)
	return 0, err
}

type SampleSpec struct {
	Format   byte
	Channels byte
	Rate     uint32
}

func (s *SampleSpec) ReadFrom(r io.Reader) (int64, error) {
	return 0, bread(r, sampleSpecTag, &s.Format, &s.Channels, &s.Rate)
}

type Card struct {
	Index         uint32
	Name          string
	Module        uint32
	Driver        string
	Profiles      map[string]*Profile
	ActiveProfile *Profile
	PropList      map[string]string
	Ports         []Port
}

type Profile struct {
	Name, Description string
	Nsinks, Nsources  uint32
	Priority          uint32
	Available         uint32
}

type Port struct {
	Card              *Card
	Name, Description string
	Pririty           uint32
	Available         uint32
	Direction         byte
	PropList          map[string]string
	Profiles          []*Profile
	LatencyOffset     int64
}

func (p *Port) ReadFrom(r io.Reader) (int64, error) {
	err := bread(r,
		stringTag, &p.Name,
		stringTag, &p.Description,
		uint32Tag, &p.Pririty,
		uint32Tag, &p.Available,
		uint8Tag, &p.Direction,
		&p.PropList)
	if err != nil {
		return 0, err
	}
	var portProfileCount uint32
	err = bread(r, uint32Tag, &portProfileCount)
	if err != nil {
		return 0, err
	}
	for j := uint32(0); j < portProfileCount; j++ {
		var profileName string
		err = bread(r, stringTag, &profileName)
		if err != nil {
			return 0, err
		}
		p.Profiles = append(p.Profiles, p.Card.Profiles[profileName])
	}
	return 0, bread(r, int64Tag, &p.LatencyOffset)
}

func (c *Client) Sinks(ctx context.Context) ([]Sink, error) {
	b, err := c.request(ctx, commandGetSinkInfoList)
	if err != nil {
		return nil, err
	}
	var sinks []Sink
	for b.Len() > 0 {
		var sink Sink
		err = bread(b, &sink)
		if err != nil {
			return nil, err
		}
		sinks = append(sinks, sink)
	}
	return sinks, nil
}

func (c *Client) Modules(ctx context.Context) ([]Module, error) {
	b, err := c.request(ctx, commandGetModuleInfoList)
	if err != nil {
		return nil, err
	}
	var modules []Module
	for b.Len() > 0 {
		var module Module
		err = bread(b, &module)
		if err != nil {
			return nil, err
		}
		modules = append(modules, module)
	}
	return modules, nil
}

func (c *Client) Cards(ctx context.Context) ([]Card, error) {
	b, err := c.request(ctx, commandGetCardInfoList)
	if err != nil {
		return nil, err
	}
	var cards []Card
	for b.Len() > 0 {
		var card Card
		var profileCount uint32
		err := bread(b,
			uint32Tag, &card.Index,
			stringTag, &card.Name,
			uint32Tag, &card.Module,
			stringTag, &card.Driver,
			uint32Tag, &profileCount)
		if err != nil {
			return nil, err
		}
		card.Profiles = make(map[string]*Profile)
		for i := uint32(0); i < profileCount; i++ {
			var profile Profile
			err = bread(b,
				stringTag, &profile.Name,
				stringTag, &profile.Description,
				uint32Tag, &profile.Nsinks,
				uint32Tag, &profile.Nsources,
				uint32Tag, &profile.Priority,
				uint32Tag, &profile.Available)
			if err != nil {
				return nil, err
			}
			card.Profiles[profile.Name] = &profile
		}
		var portCount uint32
		var activeProfileName string
		err = bread(b,
			stringTag, &activeProfileName,
			&card.PropList,
			uint32Tag, &portCount)
		if err != nil {
			return nil, err
		}
		card.ActiveProfile = card.Profiles[activeProfileName]
		card.Ports = make([]Port, portCount)
		for i := uint32(0); i < portCount; i++ {
			card.Ports[i].Card = &card
			err = bread(b, &card.Ports[i])
		}
		cards = append(cards, card)
	}
	return cards, nil
}

func (c *Client) SetCardProfile(ctx context.Context, cardIndex uint32, profileName string) error {
	_, err := c.request(ctx, commandSetCardProfile,
		uint32Tag, cardIndex,
		stringNullTag,
		stringTag, []byte(profileName), byte(0))
	return err
}

func (c *Client) setDefaultSink(ctx context.Context, sinkName string) error {
	_, err := c.request(ctx, commandSetDefaultSink,
		stringTag, []byte(sinkName), byte(0))
	return err
}

func (c *Client) ServerInfo(ctx context.Context) (*Server, error) {
	r, err := c.request(ctx, commandGetServerInfo)
	if err != nil {
		return nil, err
	}
	var s Server
	err = bread(r, &s)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
