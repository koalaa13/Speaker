package main

import (
	"google.golang.org/grpc"
	"log"
	"net"
	"proto"
	"sync"
)

type server struct {
	proto.UnimplementedAudioServiceServer
	audioMutex                 sync.Mutex
	currentBroadcastAudioCache [][]float32
}

func (s *server) hasToBroadcast() bool {
	return len(s.currentBroadcastAudioCache) > 0
}

func (s *server) Connect(stream grpc.BidiStreamingServer[proto.Audio, proto.Audio]) error {
	log.Println("new stream connection established")
	ctx := stream.Context()

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("stream connection closed: " + ctx.Err().Error())
				return
			default:
			}

			s.audioMutex.Lock()
			if s.hasToBroadcast() {
				data := s.currentBroadcastAudioCache[0]
				audio := proto.Audio{Samples: data}
				s.currentBroadcastAudioCache = s.currentBroadcastAudioCache[1:]

				if err := stream.Send(&audio); err != nil {
					log.Println("failed to send audio: " + err.Error())
				}
			}
			s.audioMutex.Unlock()
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("stream connection closed: " + ctx.Err().Error())
				return
			default:
			}

			audio, err := stream.Recv()
			if err != nil {
				log.Println("stream connection closed: " + ctx.Err().Error())
				break
			}
			log.Println(audio.GetSamples())

			if audio != nil {
				s.audioMutex.Lock()
				s.currentBroadcastAudioCache = append(s.currentBroadcastAudioCache, audio.GetSamples())
				s.audioMutex.Unlock()
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			log.Println("stream closed: " + ctx.Err().Error())
			return nil
		default:
		}
	}
}

func main() {
	l, err := net.Listen("tcp", ":6006")
	if err != nil {
		panic(err)
	}

	grpcServer := grpc.NewServer()
	proto.RegisterAudioServiceServer(grpcServer, &server{})
	if err := grpcServer.Serve(l); err != nil {
		panic(err)
	}
}
