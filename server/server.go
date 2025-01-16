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
	currentBroadcastAudioCache []proto.Audio
}

func (s *server) isCurrentlyBroadcasting() bool {

	return len(s.currentBroadcastAudioCache) > 0
}

func (s *server) Connect(stream grpc.BidiStreamingServer[proto.Broadcast, proto.Broadcast]) error {
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

			if !s.isCurrentlyBroadcasting() {
				continue
			}

			s.audioMutex.Lock()
			if s.isCurrentlyBroadcasting() {
				log.Println("broadcasting audio")
				data := proto.Broadcast{
					Audio: &s.currentBroadcastAudioCache[0],
				}
				s.currentBroadcastAudioCache = s.currentBroadcastAudioCache[1:]

				if err := stream.Send(&data); err != nil {
					log.Println("failed to send audio: " + err.Error())
				}
			}
			s.audioMutex.Unlock()
			log.Println("finish broadcasting audio")
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

			broadcast, err := stream.Recv()
			if err != nil {
				log.Println("stream connection closed: " + ctx.Err().Error())
				break
			}

			audio := broadcast.GetAudio()
			if audio != nil {
				s.audioMutex.Lock()
				log.Println("new audio from broadcast: " + broadcast.GetName())
				// TODO copy grpc message here is incorrect????
				s.currentBroadcastAudioCache = append(s.currentBroadcastAudioCache, *audio)
				s.audioMutex.Unlock()
				log.Println("added new audio to a cache: " + broadcast.GetName())

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
	l, err := net.Listen("tcp", ":6000")
	if err != nil {
		panic(err)
	}

	grpcServer := grpc.NewServer()
	proto.RegisterAudioServiceServer(grpcServer, &server{})
	if err := grpcServer.Serve(l); err != nil {
		panic(err)
	}
}
