package pulseaudio

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"sync"
	"testing"
	"time"
)

func TestExample(t *testing.T) {
	client, err := NewClient(WithLogger(stdoutLogger{}))
	require.NoError(t, err)
	var wg sync.WaitGroup
	client.Connect(context.TODO(), 10*time.Second, &wg)
	client.Close()
	wg.Wait()
	// Use `client` to interact with PulseAudio
}

func TestOutputs(t *testing.T) {
	client, err := NewClient(WithLogger(stdoutLogger{}))
	require.NoError(t, err)
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client.Connect(ctx, 5*time.Second, &wg)

	outs, active, err := client.Outputs(ctx)
	require.NoError(t, err)
	if active < 0 {
		for _, out := range outs {
			if !out.Available {
				continue
			}
			err = out.Activate(ctx)
			assert.NoError(t, err)
			break
		}
	}
	err = client.SetVolume(ctx, 0.5)
	assert.NoError(t, err)
	client.Close()
	wg.Wait()
}

func TestExampleClient_SetVolume(t *testing.T) {
	c, err := NewClient(WithLogger(stdoutLogger{}))
	require.NoError(t, err)
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.Connect(ctx, 5*time.Second, &wg)

	err = c.SetVolume(ctx, 1.5)
	assert.NoError(t, err)

	vol, err := c.Volume(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1.4999, vol, "wrong volume value")

	c.Close()
	wg.Wait()
}

func TestExampleClient_Updates(t *testing.T) {
	c, err := NewClient(WithLogger(stdoutLogger{}))
	require.NoError(t, err)
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.Connect(ctx, 5*time.Second, &wg)

	updates, err := c.Updates(ctx)
	require.NoError(t, err)

	select {
	case _ = <-updates:
		t.Errorf("Got update from PulseAudio")
	case _ = <-time.After(time.Millisecond * 10):
		fmt.Println("No update in 10 ms")
	}

	err = c.SetVolume(ctx, 0.1)
	if err != nil {
		panic(err)
	}
	fmt.Println("Volume set to 0.1")

	select {
	case _ = <-updates:
		fmt.Println("Got update from PulseAudio")
	case _ = <-time.After(time.Millisecond * 10):
		t.Errorf("No update in 10 ms")
	}

	// Output:
	// No update in 10 ms
	// Volume set to 0.1
	// Got update from PulseAudio
	c.Close()
	wg.Wait()
}

func TestExampleClient_SetMute(t *testing.T) {
	c, err := NewClient(WithLogger(stdoutLogger{}))
	require.NoError(t, err)
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.Connect(ctx, 5*time.Second, &wg)

	err = c.SetMute(ctx, true)
	assert.NoError(t, err, "can't mute")
	b, err := c.Mute(ctx)
	assert.NoError(t, err, "can't mute")
	assert.True(t, b)

	err = c.SetMute(ctx, false)
	assert.NoError(t, err, "can't unmute")
	b, err = c.Mute(ctx)
	assert.NoError(t, err, "can't mute")
	assert.False(t, b, "wrong mute value")

	c.Close()
	wg.Wait()

}

func TestExampleClient_ToggleMute(t *testing.T) {
	c, err := NewClient(WithLogger(stdoutLogger{}))
	require.NoError(t, err)
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.Connect(ctx, 30*time.Second, &wg)

	fmt.Println("1")
	b1, err := c.ToggleMute(ctx)
	assert.NoError(t, err, "can't toggle mute")
	b2, err := c.ToggleMute(ctx)
	assert.NoError(t, err, "can't toggle mute")

	assert.NotEqual(t, b1, b2)

	c.Close()
	wg.Wait()
}

type stdoutLogger struct {
}

func (s stdoutLogger) Info(msg string) {
	_, _ = fmt.Fprint(os.Stdout, "INF: "+msg+"\n")
}

func (s stdoutLogger) Infof(msg string, args ...interface{}) {
	_, _ = fmt.Fprintf(os.Stdout, "INF: "+msg+"\n", args...)
}

func (s stdoutLogger) Errorf(msg string, args ...interface{}) {
	_, _ = fmt.Fprintf(os.Stdout, "ERR: "+msg+"\n", args...)
}
