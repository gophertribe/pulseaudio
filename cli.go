package pulseaudio

import (
	"bufio"
	"bytes"
	"context"
	errs "errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var ErrSinkNotFound = errs.New("sink not found in output")

type Logger interface {
	Errorf(msg string, args ...interface{})
}

type CliClient struct {
	defaultSink string
	logger      Logger
}

func NewCliClient(defaultSink string, logger Logger) *CliClient {
	return &CliClient{
		defaultSink: defaultSink,
		logger:      logger,
	}
}

func (cli *CliClient) SetVolume(volume float32) error {
	ctx := context.Background()
	sinks, err := runListSinks(ctx, cli.logger)
	if err != nil {
		return fmt.Errorf("could not get sinks info: %w", err)
	}
	for _, s := range sinks {
		if s.Name == cli.defaultSink {
			return runSetVolume(ctx, s.Index, uint32(volume*100))
		}
	}
	return ErrSinkNotFound
}

func (cli *CliClient) SetMute(mute bool) error {
	ctx := context.Background()
	sinks, err := runListSinks(ctx, cli.logger)
	if err != nil {
		return fmt.Errorf("could not get sinks info: %w", err)
	}
	for _, s := range sinks {
		if s.Name == cli.defaultSink {
			return runSetMute(ctx, s.Index, mute)
		}
	}
	return ErrSinkNotFound
}

func (cli *CliClient) Volume() (float32, error) {
	sinks, err := runListSinks(context.Background(), cli.logger)
	if err != nil {
		return 0.0, fmt.Errorf("could not get sinks info: %w", err)
	}
	for _, s := range sinks {
		if s.Name == cli.defaultSink {
			if len(s.CVolume) == 0 {
				return 0.0, nil
			}
			return float32(s.CVolume[0]) / 100, nil
		}
	}
	return 0.0, ErrSinkNotFound
}

func (cli *CliClient) Mute() (bool, error) {
	sinks, err := runListSinks(context.Background(), cli.logger)
	if err != nil {
		return false, fmt.Errorf("could not get sinks info: %w", err)
	}
	for _, s := range sinks {
		if s.Name == cli.defaultSink {
			return s.Muted, nil
		}
	}
	return false, ErrSinkNotFound
}

var beginSinkRegex = regexp.MustCompile(`^Sink #(\d+)`)
var volumeRegex = regexp.MustCompile(`\d+ / +(\d+)% +/ +-?(?:\d+.\d+|inf) dB`)

func runListSinks(ctx context.Context, logger Logger) ([]*Sink, error) {
	cmd := exec.CommandContext(ctx, "/usr/bin/pactl", "list", "sinks")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error executing command: %w", err)
	}
	return parseSinks(bytes.NewBuffer(out), logger)
}

func runSetVolume(ctx context.Context, sink uint32, vol uint32) error {
	args := []string{"set-sink-volume", fmt.Sprintf("%d", sink), fmt.Sprintf("%d%%", vol)}
	fmt.Println(args)
	cmd := exec.CommandContext(ctx, "/usr/bin/pactl", args...)
	_, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("error executing command: %w", err)
	}
	return nil
}

func runSetMute(ctx context.Context, sink uint32, mute bool) error {
	args := []string{"set-sink-mute", fmt.Sprintf("%d", sink), fmt.Sprintf("%v", mute)}
	fmt.Println(args)
	cmd := exec.CommandContext(ctx, "/usr/bin/pactl", args...)
	_, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("error executing command: %w", err)
	}
	return nil
}

func parseSinks(r io.Reader, logger Logger) ([]*Sink, error) {
	scan := bufio.NewScanner(r)
	var sinks []*Sink
	var sink *Sink

ScanLine:
	for scan.Scan() {
		line := scan.Text()

		// read property name
		token, indent, reminder := readToken(line, false)
		switch indent {
		case 0:
			parts := beginSinkRegex.FindStringSubmatch(token)
			if len(parts) != 2 {
				continue
			}
			if sink != nil {
				sinks = append(sinks, sink)
			}
			sink = &Sink{}
			idx, err := strconv.Atoi(parts[1])
			if err != nil {
				logger.Errorf("unexpected sink index format: %s", parts[1])
			}
			sink.Index = uint32(idx)
			continue
		case 1:
			if sink == nil {
				// ignore
				continue
			}
			switch token {
			case "Volume":
				parts := volumeRegex.FindAllStringSubmatch(reminder, -1)
				if len(parts) < 2 {
					return sinks, fmt.Errorf("invalid volume line: %s", reminder)
				}
				for i := 0; i < len(parts); i++ {
					if len(parts[i]) < 2 {
						continue
					}
					vol, err := strconv.Atoi(parts[i][1])
					if err != nil {
						return sinks, fmt.Errorf("invalid base volume format (%s): %w", parts[1], err)
					}
					sink.CVolume = append(sink.CVolume, uint32(vol))
				}
			case "Mute":
				token, _, _ := readToken(reminder, true)
				sink.Muted = token == "yes"
			case "Name":
				token, _, _ := readToken(reminder, true)
				sink.Name = token
			default:
				continue ScanLine
			}
		}
	}
	if sink != nil {
		sinks = append(sinks, sink)
	}
	err := scan.Err()
	if err != nil {
		return nil, fmt.Errorf("sink scanner error: %w", err)
	}
	return sinks, nil
}

func readToken(line string, isText bool) (string, int, string) {
	var token strings.Builder
	indent := 0
	for i, r := range line {
		switch r {
		case '\t':
			indent++
		case ':':
			if isText {
				token.WriteRune(r)
				continue
			}
			return strings.TrimSpace(token.String()), indent, line[i+1:]
		case '=':
			if isText {
				token.WriteRune(r)
				continue
			}
			return strings.TrimSpace(token.String()), indent, line[i+1:]
		default:
			token.WriteRune(r)
		}
	}
	// reached end of line
	return strings.TrimSpace(token.String()), indent, ""
}
