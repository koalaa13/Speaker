package main

import (
	"context"
	"github.com/gordonklaus/portaudio"
	"gocv.io/x/gocv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"io"
	"log"
	"proto"
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

	server grpc.BidiStreamingClient[proto.Audio, proto.Audio]

	audioOutputCache [][]float32

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
	conn, err := grpc.NewClient(":6006", grpc.WithTransportCredentials(insecure.NewCredentials()))
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

		if resp != nil {
			c.audioOutputCache = append(c.audioOutputCache, resp.Samples)
			if !c.isPlayingAudio {
				c.isPlayingAudio = true
				go c.playAudio()
			}
		}
	}
}

func (c *client) playAudio() {
	out := make([]float32, sampleRate*sampleSeconds)

	c.audioOutputStream = openAudioStream(true, &out)
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
		log.Println(out)
		c.audioOutputCache = c.audioOutputCache[1:]
		err := c.audioOutputStream.Write()

		if err != nil {
			panic(err)
		}
	}
}

func openAudioStream(forOutput bool, buffer *[]float32) *portaudio.Stream {
	h, _ := portaudio.DefaultHostApi()
	var p portaudio.StreamParameters
	if forOutput {
		p = portaudio.LowLatencyParameters(nil, h.DefaultOutputDevice)
		p.Input.Channels = 0
		p.Output.Channels = 1
	} else {
		p = portaudio.LowLatencyParameters(h.DefaultInputDevice, nil)
		p.Input.Channels = 1
		p.Output.Channels = 0
	}
	res, _ := portaudio.OpenStream(p, buffer)
	return res
}

func (c *client) startAudioBroadcast() {
	c.hasMicOn = true
	in := make([]float32, sampleRate*sampleSeconds)
	audioInStream := openAudioStream(false, &in)
	err := audioInStream.Start()
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

		res := &proto.Audio{Samples: in}
		log.Println(in)

		if sendError := c.server.Send(res); sendError != nil {
			log.Printf("%v", sendError)
			return
		}
	}
	err = audioInStream.Stop()
	if err != nil {
		panic(err)
	}
	c.hasMicOn = false
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
