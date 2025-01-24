package main

import (
	"client/ui"
	"context"
	"github.com/google/uuid"
	"github.com/gordonklaus/portaudio"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"io"
	"log"
	"proto"
	"types"
)

const (
	sampleRate    = 44100
	sampleSeconds = .1
)

type audioPlayer struct {
	isPlaying bool
	cache     types.AudioCache
}

func (player *audioPlayer) addSample(part types.AudioPart) {
	player.cache.Write(part)
}

func (player *audioPlayer) playAudio() {
	out := make([]int32, sampleRate*sampleSeconds)

	audioOutputStream := openAudioStream(true, &out)
	err := audioOutputStream.Start()
	if err != nil {
		panic(err)
	}

	defer func(audioOutputStream *portaudio.Stream) {
		err = audioOutputStream.Close()
		if err != nil {
			panic(err)
		}
	}(audioOutputStream)

	defer func(audioOutputStream *portaudio.Stream) {
		err = audioOutputStream.Stop()
		if err != nil {
			panic(err)
		}
	}(audioOutputStream)

	for {
		cacheLength := player.cache.Len()
		log.Printf("cacheLength is %d", cacheLength)
		if cacheLength == 0 {
			log.Println("isPlayingAudio set to false")
			player.isPlaying = false
			break
		}

		player.isPlaying = true
		out = player.cache.Read()
		err = audioOutputStream.Write()

		if err != nil {
			panic(err)
		}
	}
}

type client struct {
	id      string
	context context.Context

	server grpc.BidiStreamingClient[proto.AudioInfo, proto.AudioInfo]

	players map[string]*audioPlayer

	isReceivingBroadcast bool
	hasMicOn             bool
	wantToBroadcast      bool
	wantToQuit           bool
}

func createClient() *client {
	clientUUID, _ := uuid.NewUUID()
	clientId := clientUUID.String()
	ctx := context.WithValue(context.Background(), "clientId", clientId)
	c := &client{
		id:      clientId,
		context: ctx,
	}
	go ui.CreateWindow(func() {
		log.Println("trying to change microphone status")
		c.wantToBroadcast = !c.wantToBroadcast
	})
	return c
}

func (c *client) shutdown() {
}

func (c *client) connectToServer() {
	log.Println("Connecting to server...")
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
		audioInfo, err := c.server.Recv()
		if err == io.EOF {
			continue
		}
		if err != nil {
			panic(err)
		}

		if audioInfo != nil {
			fromClientId := audioInfo.ClientId
			player, hasPlayer := c.players[fromClientId]
			if !hasPlayer {
				c.players[fromClientId] = &audioPlayer{}
				player = c.players[fromClientId]
			}
			player.addSample(audioInfo.Samples)
			if !player.isPlaying && player.cache.Len() > 2 {
				player.isPlaying = true
				go player.playAudio()
			}
		}
	}
}

func openAudioStream(forOutput bool, buffer *[]int32) *portaudio.Stream {
	log.Printf("Opening audio stream forOutput: %s", forOutput)
	h, err := portaudio.DefaultHostApi()
	if err != nil {
		panic(err)
	}
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
	res, err := portaudio.OpenStream(p, buffer)
	if err != nil {
		panic(err)
	}
	return res
}

func (c *client) startAudioBroadcast() {
	c.hasMicOn = true
	in := make([]int32, sampleRate*sampleSeconds)
	audioInStream := openAudioStream(false, &in)
	err := audioInStream.Start()
	if err != nil {
		panic(err)
	}

	defer func(audioInStream *portaudio.Stream) {
		err = audioInStream.Close()
		if err != nil {
			panic(err)
		}
	}(audioInStream)

	defer func(audioInStream *portaudio.Stream) {
		err = audioInStream.Stop()
		if err != nil {
			panic(err)
		}
	}(audioInStream)

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

		res := &proto.AudioInfo{
			ClientId: c.id,
			Samples:  in,
		}
		if sendError := c.server.Send(res); sendError != nil {
			log.Printf("%v", sendError)
			return
		}
	}
	c.hasMicOn = false
}

func main() {
	c := createClient()

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

		if c.wantToQuit {
			c.shutdown()
			break
		}

		if c.wantToBroadcast {
			if !c.hasMicOn {
				c.hasMicOn = true
				go c.startAudioBroadcast()
			}
		} else {
			if c.hasMicOn {
				c.hasMicOn = false
			}
		}
	}
}
