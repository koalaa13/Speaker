package main

import (
	"context"
	"github.com/gordonklaus/portaudio"
	"gocv.io/x/gocv"
	"google.golang.org/grpc"
	"io"
	"log"
	"proto"
	"time"
)

const (
	sampleRate    = 44100
	sampleSeconds = .1
)

type client struct {
	window            *gocv.Window
	audioInputStream  *portaudio.Stream
	audioOutputStream *portaudio.Stream
	deviceId          string

	context context.Context

	server grpc.BidiStreamingClient[proto.Broadcast, proto.Broadcast]

	audioOutputCache [][]int32

	lastInBroadcastTime time.Time

	isReceivingBroadcast bool
	hasMicOn             bool
	isPlayingAudio       bool
	wantToBroadcast      bool
	wantToQuit           bool
}

func createClient(ctx context.Context) client {
	return client{
		context: ctx,
		window:  gocv.NewWindow("capture window"),
	}
}

func (c *client) shutdown() {
	c.window.Close()
}

func (c *client) connectToServer() {
	conn, err := grpc.Dial("localhost:6000", grpc.WithInsecure())
	if err != nil {
		panic(err)
	}

	grpcClient := proto.NewAudioServiceClient(conn)
	c.server, err = grpcClient.Connect(c.context)
	if err != nil {
		panic(err)
	}
}

func (c *client) handleGrpcStreamRec() {
	for {
		resp, err := c.server.Recv()
		if err == io.EOF {
			continue
		}
		if err != nil {
			panic(err)
		}
		c.lastInBroadcastTime = time.Now()

		respAudio := resp.GetAudio()
		if respAudio != nil {
			c.audioOutputCache = append(c.audioOutputCache, respAudio.Samples)
			if !c.isPlayingAudio {
				c.isPlayingAudio = true
				go c.playAudio()
			}
		}
	}
}

func (c *client) playAudio() {
	out := make([]int32, sampleRate*sampleSeconds)
	var err error

	c.audioOutputStream, err = portaudio.OpenDefaultStream(0, 1, sampleRate, len(out), &out)
	if err != nil {
		panic(err)
	}
	defer c.audioOutputStream.Close()

	c.audioOutputStream.Start()
	defer c.audioOutputStream.Stop()

	for {
		cacheLength := len(c.audioOutputCache)
		if cacheLength == 0 {
			c.isPlayingAudio = false
			break
		}

		c.isPlayingAudio = true
		out = c.audioOutputCache[0]
		c.audioOutputCache = c.audioOutputCache[1:]
		err = c.audioOutputStream.Write()

		if err != nil {
			panic(err)
		}
	}
}

func (c *client) startAudioBroadcast() {
	c.hasMicOn = true
	in := make([]int32, sampleRate*sampleSeconds)
	audioInStream, err := portaudio.OpenDefaultStream(1, 0, sampleRate, len(in), &in)
	if err != nil {
		panic(err)
	}
	err = audioInStream.Start()
	if err != nil {
		panic(err)
	}
	for {
		select {
		case <-c.context.Done():
			break
		default:
		}

		if !c.wantToBroadcast {
			break
		}

		err = audioInStream.Read()
		if err != nil {
			panic(err)
		}

		go func(sendSamples []int32) {
			res := proto.Broadcast{Audio: &proto.Audio{Samples: sendSamples}}

			if sendError := c.server.Send(&res); sendError != nil {
				log.Printf("%v", sendError)
				return
			}
		}(in)
	}
	err = audioInStream.Stop()
	if err != nil {
		panic(err)
	}
	c.hasMicOn = false
}

func (c *client) hasIncomingBroadcast() bool {
	if !c.isReceivingBroadcast {
		return false
	}
	if time.Now().After(c.lastInBroadcastTime.Add(300 * time.Millisecond)) {
		c.isReceivingBroadcast = false
		return false
	}
	return true
}

func main() {
	c := createClient(context.Background())

	c.connectToServer()
	err := portaudio.Initialize()
	if err != nil {
		panic(err)
	}
	defer portaudio.Terminate()

	go c.handleGrpcStreamRec()

	for {
		select {
		case <-c.context.Done():
			c.wantToQuit = true
		default:
		}

		switch c.window.WaitKey(1) {
		case 27: // ESC
			c.wantToQuit = true
		case 32: // SPACE
			c.wantToBroadcast = !c.wantToBroadcast
		default:
		}

		if c.wantToQuit {
			c.shutdown()
			break
		}

		if c.wantToBroadcast {
			if !c.hasMicOn {
				go c.startAudioBroadcast()
			}
		} else {
			if c.hasMicOn {
				c.hasMicOn = false
			}
		}
	}
}
